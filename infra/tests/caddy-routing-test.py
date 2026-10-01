#!/usr/bin/env python3
"""Real Caddy routing/TLS test. Uses only an ephemeral internal CA, never ACME."""
import http.server
import json
import os
from pathlib import Path
import socket
import ssl
import subprocess
import tempfile
import threading
import time

ROOT = Path(__file__).resolve().parents[2]
CADDY = os.environ.get('CADDY_BINARY', 'caddy')
BASE = 'remote.example.test'

class Backend(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.end_headers()
        self.wfile.write(('BACKEND ' + self.path).encode())
    def log_message(self, *_):
        pass

class Preview(Backend):
    def do_GET(self):
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b'PREVIEW')

servers = [http.server.ThreadingHTTPServer(('127.0.0.1', 0), cls) for cls in (Backend, Preview)]
for server in servers:
    threading.Thread(target=server.serve_forever, daemon=True).start()
with socket.socket() as listener:
    listener.bind(('127.0.0.1', 0))
    port = listener.getsockname()[1]

def visit(host, path='/', authority=None):
    context = ssl._create_unverified_context()
    with socket.create_connection(('127.0.0.1', port), timeout=2) as raw:
        with context.wrap_socket(raw, server_hostname=host) as tls:
            tls.sendall(f'GET {path} HTTP/1.1\r\nHost: {authority or host}\r\nConnection: close\r\n\r\n'.encode())
            data = b''
            while chunk := tls.recv(65536):
                data += chunk
    return data

try:
    with tempfile.TemporaryDirectory(prefix='remote-caddy-test-') as work:
        work = Path(work)
        template = (ROOT / 'infra/templates/Caddyfile.tmpl').read_text()
        for key, value in {'HOSTNAME': BASE, 'HOSTNAME_RE': BASE.replace('.', r'\.'),
                           'SERVICE_PORT': str(servers[0].server_port), 'INSTALL_DIR': str(ROOT)}.items():
            template = template.replace('${' + key + '}', value)
        # Exercise the real template with an internal CA instead of external DNS/ACME.
        template = '{\n local_certs\n skip_install_trust\n}\n' + template.replace('import /etc/caddy/remote-dns.caddy', 'issuer internal')
        caddyfile = work / 'Caddyfile'
        caddyfile.write_text(template)
        adapted = subprocess.run([CADDY, 'adapt', '--config', str(caddyfile), '--adapter', 'caddyfile'], check=True, capture_output=True)
        config = json.loads(adapted.stdout)
        names = set()
        def walk(node):
            if isinstance(node, dict):
                if 'host' in node and isinstance(node['host'], list):
                    names.update(node['host'])
                if 'dial' in node and '.lxd:' in node['dial']:
                    node['dial'] = '127.0.0.1:' + str(servers[1].server_port)
                for value in node.values():
                    walk(value)
            elif isinstance(node, list):
                for value in node:
                    walk(value)
        walk(config)
        assert names == {BASE, '*.' + BASE}, names
        assert 'on_demand' not in json.dumps(config)
        config['admin'] = {'disabled': True}
        config['storage'] = {'module': 'file_system', 'root': str(work / 'storage')}
        for server in config['apps']['http']['servers'].values():
            server['listen'] = ['127.0.0.1:' + str(port)]
            server['automatic_https'] = {'disable_redirects': True}
        path = work / 'config.json'
        path.write_text(json.dumps(config))
        with (work / 'log').open('w+') as log:
            process = subprocess.Popen([CADDY, 'run', '--config', str(path)], stdout=log, stderr=log)
            try:
                for _ in range(100):
                    try:
                        if b'BACKEND' in visit(BASE):
                            break
                    except (OSError, ssl.SSLError):
                        pass
                    if process.poll() is not None:
                        log.seek(0)
                        raise AssertionError(log.read())
                    time.sleep(.1)
                else:
                    raise AssertionError('Caddy did not become ready')
                for host in ['code--gamerhead', 'browser--gamerhead', 's3--gamerhead', 'app--abcdef123456--instance']:
                    assert b'BACKEND /?folder=%2Fworkspace' in visit(host + '.' + BASE, '/?folder=%2Fworkspace'), host
                assert b'PREVIEW' in visit('dev--gamerhead--3000.' + BASE)
                assert b'PREVIEW' in visit('dev--gamerhead--3000.' + BASE, authority='DEV--GAMERHEAD--3000.' + BASE.upper() + ':443')
                assert b'BACKEND /__remote_inspector' in visit('dev--gamerhead--3000.' + BASE, '/__remote_inspector')
                assert b'403' in visit(BASE, '/internal/example').split(b'\r\n', 1)[0]
                for host in ['unrelated.example.test', 'gamerhead--3000.dev.' + BASE, 'abcdef123456.apps.' + BASE]:
                    try:
                        visit(host)
                    except ssl.SSLError:
                        continue
                    raise AssertionError('Unexpected certificate for ' + host)
                certs = list((work / 'storage/certificates').rglob('*.crt'))
                assert len(certs) == 2, certs
                print('Caddy routing passed; only base and wildcard certificates issued locally')
            finally:
                process.terminate()
                process.wait(timeout=10)
finally:
    for server in servers:
        server.shutdown()
        server.server_close()
