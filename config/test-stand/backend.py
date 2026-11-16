import base64, json, sys
from http.server import BaseHTTPRequestHandler, HTTPServer

def claims(auth):
    """Payload of the bearer token, unverified: what the backend got."""
    try:
        payload = auth.split()[1].split(".")[1]
        return json.loads(base64.urlsafe_b64decode(payload + "=" * (-len(payload) % 4)))
    except Exception:
        return None

class H(BaseHTTPRequestHandler):
    def do_GET(self):
        auth = self.headers.get("Authorization", "")
        body = json.dumps({
            "path": self.path,
            "authorization": (auth[:24] + "…") if auth else "",
            "token": claims(auth),
            "cookie": self.headers.get("Cookie", ""),
            "msg": "hello from backend",
        }).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)
    def log_message(self, fmt, *args):
        sys.stderr.write("backend: " + (fmt % args) + "\n")

HTTPServer(("127.0.0.1", 18081), H).serve_forever()
