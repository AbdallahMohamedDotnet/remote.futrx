#!/usr/bin/env python3
"""Real Caddy routing/TLS test. Uses only an ephemeral internal CA, never ACME."""
import http.server
import json
import os
from pathlib import Path
from urllib.parse import parse_qs, urlsplit
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
    admissions = []
    def do_GET(self):
        if self.path.startswith('/internal/tls-ask?'):
            domain = parse_qs(urlsplit(self.path).query)['domain'][0]
            self.admissions.append(domain)
            allowed = domain in ['gamerhead--3000.dev.' + BASE, 'abcdef123456.apps.' + BASE]
            self.send_response(200 if allowed else 404)
            self.end_headers()
            return
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
        template = template.replace('{\n', '{\n local_certs\n skip_install_trust\n', 1).replace('import /etc/caddy/remote-dns.caddy', 'issuer internal')
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
        required = {BASE, '*.' + BASE, '*.dev.' + BASE, '*.apps.' + BASE}
        assert required <= names <= required | {'code.' + BASE, '*.code.' + BASE}, names
        policies = config['apps']['tls']['automation']['policies']
        wildcard = next(p for p in policies if not p.get('subjects') or '*.' + BASE in p['subjects'])
        assert not wildcard.get('on_demand'), wildcard
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
                for host in ['code--gamerhead', 'code--another-project']:
                    assert b'BACKEND /?folder=%2Fworkspace' in visit(host + '.' + BASE, '/?folder=%2Fworkspace'), host
                assert not Backend.admissions, Backend.admissions
                assert b'404' in visit('dev.' + BASE).split(b'\r\n', 1)[0]
                certs = list((work / 'storage/certificates').rglob('*.crt'))
                assert len([p for p in certs if p.name.startswith('wildcard')]) == 1, certs
                assert not any('code--' in p.name for p in certs), certs
                # Existing preview and unnamed app origins still work, with their old admission flow.
                assert b'PREVIEW' in visit('gamerhead--3000.dev.' + BASE)
                assert b'BACKEND /__remote_inspector' in visit('gamerhead--3000.dev.' + BASE, '/__remote_inspector')
                assert b'BACKEND' in visit('abcdef123456.apps.' + BASE)
                assert set(Backend.admissions) == {'gamerhead--3000.dev.' + BASE, 'abcdef123456.apps.' + BASE}
                assert b'403' in visit(BASE, '/internal/example').split(b'\r\n', 1)[0]
                print('Code Server reuses one wildcard; original preview and unnamed app routing/TLS preserved')
            finally:
                process.terminate()
                process.wait(timeout=10)
finally:
    for server in servers:
        server.shutdown()
        server.server_close()
