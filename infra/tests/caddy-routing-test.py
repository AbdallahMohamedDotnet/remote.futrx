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
NAMED = ['code--gamerhead.' + BASE, 'code--another-project.' + BASE]

class Backend(http.server.BaseHTTPRequestHandler):
    admissions = []
    def do_GET(self):
        if self.path.startswith('/internal/tls-ask?'):
            domain = parse_qs(urlsplit(self.path).query)['domain'][0]
            self.admissions.append(domain)
            allowed = domain in ['gamerhead--3000.dev.' + BASE] + NAMED
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

def run():
    with tempfile.TemporaryDirectory(prefix='remote-caddy-test-') as work:
        work = Path(work)
        template = (ROOT / 'infra/templates/Caddyfile.tmpl').read_text()
        for key, value in {'HOSTNAME': BASE, 'HOSTNAME_RE': BASE.replace('.', r'\.'),
                           'SERVICE_PORT': str(servers[0].server_port), 'INSTALL_DIR': str(ROOT)}.items():
            template = template.replace('${' + key + '}', value)
        # Exercise the real template with an internal CA instead of external ACME.
        template = template.replace('{\n', '{\n local_certs\n skip_install_trust\n', 1)
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
        required = {BASE, '*.' + BASE, '*.dev.' + BASE}
        assert required <= names <= required | {'code.' + BASE, '*.code.' + BASE}, names
        policies = config['apps']['tls']['automation']['policies']
        wildcard = next(p for p in policies if not p.get('subjects') or '*.' + BASE in p['subjects'])
        assert wildcard.get('on_demand'), wildcard
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
                for host in NAMED:
                    assert b'BACKEND /?folder=%2Fworkspace' in visit(host, '/?folder=%2Fworkspace'), host
                certs = lambda: list((work / 'storage/certificates').rglob('*.crt'))
                # No DNS provider: each admitted host gets its own certificate, and
                # a host the backend does not admit never gets one.
                assert set(Backend.admissions) == set(NAMED), Backend.admissions
                assert {p.stem for p in certs()} == set(NAMED) | {BASE}, certs()
                try:
                    visit('code--missing.' + BASE)
                    raise AssertionError('unadmitted named host got a certificate')
                except (OSError, ssl.SSLError):
                    pass
                Backend.admissions.clear()
                # Existing preview origins still work, with their old admission flow.
                assert b'PREVIEW' in visit('gamerhead--3000.dev.' + BASE)
                assert b'BACKEND /__remote_inspector' in visit('gamerhead--3000.dev.' + BASE, '/__remote_inspector')
                assert set(Backend.admissions) == {'gamerhead--3000.dev.' + BASE}
                assert b'403' in visit(BASE, '/internal/example').split(b'\r\n', 1)[0]
                print('Code Server uses on-demand certificates; original preview routing/TLS preserved')
            finally:
                process.terminate()
                process.wait(timeout=10)

try:
    run()
finally:
    for server in servers:
        server.shutdown()
        server.server_close()
