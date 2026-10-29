#!/usr/bin/env python3
"""Exercise Sprint 1 against the real API and a local Mailpit inbox.

Creates isolated synthetic accounts/clubs and retains their records and emails.
Use a development/test installation, never a production or shared real-data API.
Secrets, session cookies and email contents are never printed.
"""

import argparse
import copy
import http.cookiejar
import json
import re
import secrets
import sys
import time
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode, urlsplit
from urllib.request import HTTPCookieProcessor, Request, build_opener


class SmokeFailure(Exception):
    """A diagnostic intentionally excluding response bodies and credentials."""


def require(condition, message):
    if not condition:
        raise SmokeFailure(message)


class Client:
    def __init__(self, base, cookies=None):
        self.base = base.rstrip("/")
        self.cookies = cookies if cookies is not None else http.cookiejar.CookieJar()
        self.opener = build_opener(HTTPCookieProcessor(self.cookies))

    def clone(self):
        cookies = http.cookiejar.CookieJar()
        for cookie in self.cookies:
            cookies.set_cookie(copy.copy(cookie))
        return Client(self.base, cookies)

    def request(self, method, path, data=None, expected=200):
        headers = {"Accept": "application/json", "Origin": self.base}
        payload = None
        if data is not None:
            headers["Content-Type"] = "application/json"
            payload = json.dumps(data).encode("utf-8")
        request = Request(self.base + path, data=payload, headers=headers, method=method)
        try:
            response = self.opener.open(request, timeout=15)
        except HTTPError as error:
            response = error
        except URLError:
            raise SmokeFailure(f"{method} {path}: service unavailable") from None
        statuses = (expected,) if isinstance(expected, int) else expected
        with response:
            if response.status not in statuses:
                raise SmokeFailure(
                    f"{method} {path}: status {response.status}; expected {statuses}"
                )
            body = response.read()
            if not body:
                require(response.status == 204, f"{method} {path}: missing JSON response")
                return None
            require(
                "application/json" in response.headers.get("Content-Type", ""),
                f"{method} {path}: response is not JSON",
            )
            try:
                return json.loads(body)
            except (ValueError, UnicodeError):
                raise SmokeFailure(f"{method} {path}: invalid JSON") from None

    def get(self, path, expected=200):
        return self.request("GET", path, expected=expected)

    def post(self, path, data=None, expected=200):
        return self.request("POST", path, {} if data is None else data, expected)


class Mailbox:
    token_pattern = re.compile(r"(?<![A-Za-z0-9_-])[A-Za-z0-9_-]{43}(?![A-Za-z0-9_-])")

    def __init__(self, base):
        self.client = Client(base)
        self.seen = set()

    def token(self, recipient):
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            listing = self.client.get("/api/v1/search?" + urlencode({"query": "to:" + recipient}))
            messages = listing.get("messages", [])
            for message in messages:
                identifier = message.get("ID", "")
                if not identifier or identifier in self.seen:
                    continue
                detail = self.client.get("/api/v1/message/" + identifier)
                matches = self.token_pattern.findall(detail.get("Text", ""))
                self.seen.add(identifier)
                if matches:
                    require(len(set(matches)) == 1, "Email contains ambiguous verification codes")
                    return matches[0]
            time.sleep(0.2)
        raise SmokeFailure("Expected development email did not arrive within 30 seconds")


def theme(primary, secondary, accent):
    return {
        "primaryColor": primary,
        "secondaryColor": secondary,
        "accentColor": accent,
        "surfaceColor": "#ffffff",
        "textColor": "#111827",
        "logoUrl": "",
        "coverUrl": "",
        "sponsors": [],
        "kitColors": [primary, secondary],
    }


def run(base, mailpit):
    passed = []

    def checkpoint(name):
        passed.append(name)
        print("PASS:", name, flush=True)

    anonymous = Client(base)
    first, second = Client(base), Client(base)
    inbox = Mailbox(mailpit)
    nonce = secrets.token_hex(8)
    emails = [f"sprint1-{nonce}-{n}@example.invalid" for n in (1, 2)]
    password = secrets.token_urlsafe(24)
    next_password = secrets.token_urlsafe(24)

    require(anonymous.get("/health")["status"] == "ok", "API is not ready")
    anonymous.get("/v1/me", expected=401)
    anonymous.post("/v1/clubs", {"name": "Anonymous club"}, expected=401)
    checkpoint("readiness and anonymous access denied")

    for index, client in enumerate((first, second)):
        registration = client.post(
            "/v1/auth/register",
            {"email": emails[index], "displayName": "Synthetic manager", "password": password},
            expected=201,
        )
        require(not registration["user"]["emailVerified"], "New email was already verified")
        require(bool(list(client.cookies)), "Registration did not create a session")
        client.post("/v1/clubs", {"name": "Unverified club"}, expected=403)
        code = inbox.token(emails[index])
        client.post("/v1/auth/verify-email", {"token": code}, expected=204)
        client.post("/v1/auth/verify-email", {"token": code}, expected=400)
        require(client.get("/v1/me")["user"]["emailVerified"], "Email verification not retained")
    checkpoint("registration, verification, single-use email tokens and verified-write gate")

    club_a = first.post("/v1/clubs", {"name": "Synthetic club A", "shortName": "SYN-A"}, 201)["club"]
    club_b = second.post("/v1/clubs", {"name": "Synthetic club B", "shortName": "SYN-B"}, 201)["club"]
    a, b = "/v1/clubs/" + club_a["id"], "/v1/clubs/" + club_b["id"]
    public_a, public_b = "/v1/public/clubs/" + club_a["id"], "/v1/public/clubs/" + club_b["id"]
    require(
        {club["id"] for club in first.get("/v1/clubs")["clubs"]} == {club_a["id"]},
        "Club listing exposed another manager's club",
    )
    require(
        {club["id"] for club in second.get("/v1/clubs")["clubs"]} == {club_b["id"]},
        "Club listing exposed another manager's club",
    )
    first.get(b, expected=(403, 404))
    first.request("PATCH", b, {"name": "Forbidden rename"}, expected=(403, 404))
    first.get(b + "/branding/draft", expected=(403, 404))
    first.post(b + "/branding/publish", expected=(403, 404))
    second.get(a, expected=(403, 404))
    checkpoint("two-club listing, reads and writes isolated by membership")

    theme_a = theme("#134e4a", "#052e2b", "#d9f99d")
    theme_b = theme("#4338ca", "#1e1b4b", "#e9d5ff")
    first.request("PUT", a + "/branding/draft", {"theme": theme_a})
    second.request("PUT", b + "/branding/draft", {"theme": theme_b})
    require(first.get(a + "/branding/draft")["theme"] == theme_a, "Club A draft did not persist")
    require(second.get(b + "/branding/draft")["theme"] == theme_b, "Club B draft did not persist")
    anonymous.get(public_a, expected=404)
    anonymous.get(public_b, expected=404)
    checkpoint("independent branding drafts remain private before publication")

    first_publish = first.post(a + "/branding/publish")
    version_one = first_publish["version"]
    second.post(b + "/branding/publish")
    require(anonymous.get(public_a)["club"]["branding"] == theme_a, "Published theme differs from draft")
    require(anonymous.get(public_b)["club"]["branding"] == theme_b, "Club B published theme is incorrect")
    changed = dict(theme_a, primaryColor="#b45309")
    first.request("PUT", a + "/branding/draft", {"theme": changed})
    require(anonymous.get(public_a)["club"]["branding"] == theme_a, "Unpublished changes became public")
    second_publish = first.post(a + "/branding/publish")
    require(second_publish["version"] > version_one, "Publication version did not advance")
    require(anonymous.get(public_a)["club"]["branding"] == changed, "Second publication not visible")
    restored = first.post(a + "/branding/rollback", {"version": version_one})
    require(restored["version"] > second_publish["version"], "Rollback reused an old version")
    require(anonymous.get(public_a)["club"]["branding"] == theme_a, "Rollback did not restore the theme")
    require(len(first.get(a + "/branding/versions")["versions"]) == 3, "Publication history was not retained")
    checkpoint("explicit publication, draft isolation and versioned rollback")

    first.post(a + "/invitations", {"email": emails[1], "role": "coach"}, 201)
    invitation_code = inbox.token(emails[1])
    invited = second.post("/v1/invitations/accept", {"token": invitation_code})["club"]
    require(invited["id"] == club_a["id"], "Invitation accepted into the wrong club")
    second.post("/v1/invitations/accept", {"token": invitation_code}, expected=400)
    require(second.get(a)["club"]["role"] == "coach", "Invitation did not assign the requested role")
    require(
        {club["id"] for club in second.get("/v1/clubs")["clubs"]} == {club_a["id"], club_b["id"]},
        "One user cannot select both memberships",
    )
    second.request("PUT", a + "/branding/draft", {"theme": theme_b}, expected=403)
    second.post(a + "/branding/publish", expected=403)
    second.get(a + "/members", expected=403)
    second.get(a + "/invitations", expected=403)
    checkpoint("email invitation, single-use acceptance and coach permissions")

    previous_session = second.clone()
    second.post("/v1/auth/refresh")
    previous_session.get("/v1/me", expected=401)
    require(second.get("/v1/me")["user"]["emailVerified"], "Rotated session did not authenticate")
    revoked_session = second.clone()
    second.post("/v1/auth/logout", expected=204)
    revoked_session.get("/v1/me", expected=401)
    second.get("/v1/me", expected=401)
    second.post("/v1/auth/login", {"email": emails[1], "password": password})
    checkpoint("session rotation and server-side logout revocation")

    known = anonymous.post("/v1/auth/forgot-password", {"email": emails[0]}, expected=202)
    unknown = anonymous.post(
        "/v1/auth/forgot-password", {"email": f"missing-{nonce}@example.invalid"}, expected=202
    )
    require(known == unknown, "Password recovery reveals whether an account exists")
    reset_code = inbox.token(emails[0])
    reset_session = first.clone()
    anonymous.post("/v1/auth/reset-password", {"token": reset_code, "password": next_password}, expected=204)
    first.get("/v1/me", expected=401)
    reset_session.get("/v1/me", expected=401)
    first.post("/v1/auth/login", {"email": emails[0], "password": password}, expected=401)
    first.post("/v1/auth/login", {"email": emails[0], "password": next_password})
    anonymous.post("/v1/auth/reset-password", {"token": reset_code, "password": password}, expected=400)
    checkpoint("generic recovery response, one-use reset and revocation of existing sessions")

    second.request("DELETE", a, expected=403)
    first.request("DELETE", a, expected=204)
    anonymous.get(public_a, expected=404)
    second.get(a, expected=(403, 404))
    require(second.get(b)["club"]["id"] == club_b["id"], "Archiving one club affected another club")
    checkpoint("president-only soft archive preserves isolation")
    print(f"Sprint 1 smoke passed: {len(passed)} functional checkpoints; synthetic data and emails retained.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default="http://127.0.0.1:18081", help="API or Manager proxy base URL")
    parser.add_argument("--mailpit-url", default="http://127.0.0.1:18025", help="Development Mailpit HTTP API")
    args = parser.parse_args()
    for label, value in (("base URL", args.base_url), ("Mailpit URL", args.mailpit_url)):
        parsed = urlsplit(value)
        if parsed.scheme not in ("http", "https") or not parsed.netloc or parsed.username or parsed.password:
            parser.error(f"Invalid {label}; use an HTTP(S) URL without credentials")
    try:
        run(args.base_url.rstrip("/"), args.mailpit_url.rstrip("/"))
    except (SmokeFailure, KeyError, TypeError) as error:
        # Key names are safe; never emit server response payloads or token values.
        if isinstance(error, SmokeFailure):
            print("FAIL:", error, file=sys.stderr)
        else:
            print("FAIL: API response did not match the documented contract", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
