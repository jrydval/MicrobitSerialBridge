#!/usr/bin/env python3
"""Simple local webhook receiver for testing MicrobitSerialBridge.

Usage:
    python3 receiver.py [port]

Default port is 8080. Then start the bridge with:
    -webhook-url http://localhost:8080
"""

import http.server
import json
import sys


class Handler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length)

        print("-" * 80, flush=True)
        try:
            print(json.dumps(json.loads(body), indent=2, ensure_ascii=False), flush=True)
        except json.JSONDecodeError:
            print(f"(invalid JSON) {body!r}", flush=True)

        self.send_response(200)
        self.end_headers()

    def log_message(self, format, *args):
        pass  # suppress default access logs


if __name__ == "__main__":
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 8080
    print(f"Listening on http://localhost:{port} ...")
    http.server.HTTPServer(("localhost", port), Handler).serve_forever()
