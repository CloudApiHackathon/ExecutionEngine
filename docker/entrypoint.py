#!/usr/bin/env python3

import http.server
import socketserver
import json
import os
import sys
import socket

class ExitHandler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path == '/exit':
            content_length = int(self.headers['Content-Length'])
            post_data = self.rfile.read(content_length)
            try:
                data = json.loads(post_data)
                exit_code = data.get("code", 0)

                self.send_response(204)
                self.end_headers()
                self.wfile.close()
                os._exit(exit_code)
            except (json.JSONDecodeError, TypeError, KeyError):
                self.send_response(400)
                self.end_headers()
                self.wfile.close()

    def log_message(self, format, *args):
        # Disable logging
        return

def run_unix_socket_server(socket_path):
    # Ensure the socket path is removed if it already exists
    if os.path.exists(socket_path):
        os.remove(socket_path)

    with socketserver.ThreadingUnixStreamServer(socket_path, ExitHandler) as http_server:
        print(f"Listening at {socket_path}")
        http_server.serve_forever()


if __name__ == "__main__":
    socket_path = "/tmp/daemon.socket"
    run_unix_socket_server(socket_path)
