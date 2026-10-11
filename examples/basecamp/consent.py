#!/usr/bin/env python3
"""Get a Basecamp refresh token for rela, once per connection.

    ./consent.py | rela token set basecamp

It opens the Basecamp authorization page in your browser and runs a small
web server on the redirect URI to catch the answer, so you copy nothing by
hand. Register the app at https://launchpad.37signals.com/integrations with
the redirect URI http://localhost:8765/callback, or pass the one you
registered with --redirect-uri. It must point at localhost.

The client id comes from BASECAMP_CLIENT_ID or a prompt. The client secret
comes from BASECAMP_CLIENT_SECRET or a prompt that does not echo. Do not
type the secret on the command line: your shell history would keep it.

The token JSON goes to stdout and everything else to stderr, so a pipe into
`rela token set` carries only the token. Without a pipe it prints the JSON;
paste it into the desktop app (Settings, Connections). It also lists the
Basecamp accounts the token reaches, so you can pick basecamp_account_id.

Needs Python 3.8 or later and nothing else.
"""

import argparse
import getpass
import http.server
import json
import os
import secrets
import sys
import threading
import urllib.error
import urllib.parse
import urllib.request
import webbrowser

LAUNCHPAD = "https://launchpad.37signals.com"
# Basecamp refuses a request whose User-Agent names no contact.
DEFAULT_USER_AGENT = "rela-basecamp-example (ops@example.com)"


def say(text=""):
    print(text, file=sys.stderr)


def fail(text):
    say("consent.py: " + text)
    sys.exit(1)


def ask(prompt):
    sys.stderr.write(prompt)
    sys.stderr.flush()
    return sys.stdin.readline().strip()


def catch_code(redirect_uri, state):
    """Serve the redirect URI until Basecamp sends the code for this state."""
    target = urllib.parse.urlparse(redirect_uri)
    if target.scheme != "http" or target.hostname not in ("localhost", "127.0.0.1"):
        fail("the redirect URI must be http://localhost:<port>/<path>")
    result = {}
    done = threading.Event()

    class Handler(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            url = urllib.parse.urlparse(self.path)
            if url.path != target.path:
                self.send_error(404)
                return
            query = urllib.parse.parse_qs(url.query)
            # The state ties the answer to this run, so another page cannot
            # hand the script a code of its own.
            if query.get("state", [""])[0] != state:
                self.reply(400, "The state does not match. Start consent.py again.")
                return
            if "code" in query:
                result["code"] = query["code"][0]
                self.reply(200, "Connected. You can close this tab and go back to the terminal.")
            else:
                self.reply(400, "Basecamp sent no code: " + query.get("error", ["unknown error"])[0])
            done.set()

        def reply(self, status, text):
            body = ("<!doctype html><title>rela</title><p>%s</p>" % text).encode()
            self.send_response(status)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *args):
            pass

    server = http.server.HTTPServer(("127.0.0.1", target.port or 80), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    try:
        done.wait()
    finally:
        server.shutdown()
    if "code" not in result:
        fail("consent was not given")
    return result["code"]


def request_json(url, user_agent, form=None, token=None):
    headers = {"User-Agent": user_agent}
    if token:
        headers["Authorization"] = "Bearer " + token
    data = urllib.parse.urlencode(form).encode() if form else None
    req = urllib.request.Request(url, data=data, headers=headers)
    with urllib.request.urlopen(req, timeout=30) as resp:
        return json.load(resp)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--redirect-uri", default="http://localhost:8765/callback")
    ap.add_argument("--user-agent", default=os.environ.get("BASECAMP_USER_AGENT", DEFAULT_USER_AGENT))
    args = ap.parse_args()

    client_id = os.environ.get("BASECAMP_CLIENT_ID") or ask("Basecamp client id: ")
    client_secret = os.environ.get("BASECAMP_CLIENT_SECRET") or getpass.getpass("Basecamp client secret: ")
    if not client_id or not client_secret:
        fail("the client id and secret are required")

    state = secrets.token_urlsafe(24)
    auth_url = LAUNCHPAD + "/authorization/new?" + urllib.parse.urlencode(
        {"type": "web_server", "client_id": client_id, "redirect_uri": args.redirect_uri, "state": state}
    )
    say("Opening Basecamp in your browser. If it does not open, visit:")
    say("  " + auth_url)
    webbrowser.open(auth_url)
    code = catch_code(args.redirect_uri, state)

    # The secret and the code travel in the POST body, never in a URL.
    try:
        tokens = request_json(
            LAUNCHPAD + "/authorization/token",
            args.user_agent,
            form={
                "type": "web_server",
                "code": code,
                "client_id": client_id,
                "client_secret": client_secret,
                "redirect_uri": args.redirect_uri,
            },
        )
    except urllib.error.HTTPError as err:
        fail("the token exchange failed (HTTP %d); codes are single-use, so start again" % err.code)
    if not tokens.get("access_token") or not tokens.get("refresh_token"):
        fail("Basecamp's answer held no tokens")

    say("Basecamp accounts this token reaches (use the id as basecamp_account_id):")
    try:
        info = request_json(LAUNCHPAD + "/authorization.json", args.user_agent, token=tokens["access_token"])
        for account in info.get("accounts", []):
            if account.get("product") == "bc3":
                say("  %s  %s" % (account["id"], account["name"]))
    except (urllib.error.URLError, ValueError):
        say("  (could not list accounts)")

    if sys.stdout.isatty():
        say("The next line holds your tokens. Paste it into the desktop app, then clear the screen.")
    print(json.dumps({k: tokens.get(k) for k in ("refresh_token", "access_token", "expires_in")}))


if __name__ == "__main__":
    main()
