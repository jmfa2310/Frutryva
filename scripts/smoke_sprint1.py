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
import subprocess
from pathlib import Path
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


def bootstrap_owner(email, binary=None):
    """Use only the documented, explicit local administrative CLI."""
    root = Path(__file__).resolve().parents[1]
    command = (
        [str(binary), "owner", "set", email]
        if binary
        else ["docker", "compose", "exec", "-T", "api", "/futryva", "owner", "set", email]
    )
    try:
        subprocess.run(command, cwd=root, check=True, capture_output=True, timeout=30)
    except (OSError, subprocess.SubprocessError):
        raise SmokeFailure("Explicit owner bootstrap failed; check the local administrative CLI and database configuration") from None


def run(base, mailpit, owner_binary=None):
    passed = []

    def checkpoint(name):
        passed.append(name)
        print("PASS:", name, flush=True)

    anonymous = Client(base)
    owner, first, second, coach = [Client(base) for _ in range(4)]
    inbox = Mailbox(mailpit)
    nonce = secrets.token_hex(8)
    emails = [f"sprint1-{nonce}-{n}@example.invalid" for n in range(4)]
    password = secrets.token_urlsafe(24)
    next_password = secrets.token_urlsafe(24)

    require(anonymous.get("/health")["status"] == "ok", "API is not ready")
    anonymous.get("/v1/me", expected=401)
    anonymous.post("/v1/clubs", {"name": "Anonymous club"}, expected=401)
    anonymous.get("/v1/admin/clubs", expected=401)
    checkpoint("readiness and anonymous access denied")

    for index, client in enumerate((owner, first, second, coach)):
        registration = client.post(
            "/v1/auth/register",
            {"email": emails[index], "displayName": "Synthetic account", "password": password},
            expected=201,
        )
        require(not registration["user"]["emailVerified"], "New email was already verified")
        require(not registration["user"]["isPlatformOwner"], "Registration granted platform ownership")
        require(not registration["user"].get("assignedClubId"), "Registration assigned a club without the owner")
        require(bool(list(client.cookies)), "Registration did not create a session")
        client.post("/v1/clubs", {"name": "Unverified club"}, expected=403)
        code = inbox.token(emails[index])
        client.post("/v1/auth/verify-email", {"token": code}, expected=204)
        client.post("/v1/auth/verify-email", {"token": code}, expected=400)
        require(client.get("/v1/me")["user"]["emailVerified"], "Email verification not retained")
        require(client.get("/v1/clubs")["clubs"] == [], "Unassigned account can access a club")
    checkpoint("registration, verification and accounts initially without owner privileges or club")

    bootstrap_owner(emails[0], owner_binary)
    require(owner.get("/v1/me")["user"]["isPlatformOwner"], "Explicit bootstrap did not grant platform ownership")
    for client in (first, second, coach):
        client.get("/v1/admin/clubs", expected=403)
        client.post("/v1/admin/clubs", {"name": "Forbidden club", "managerEmail": emails[1]}, expected=403)
        client.post("/v1/clubs", {"name": "Forbidden self-service club"}, expected=403)
    owner.post("/v1/clubs", {"name": "Wrong creation route"}, expected=403)
    club_a = owner.post(
        "/v1/admin/clubs", {"name": "Synthetic club A", "shortName": "SYN-A", "managerEmail": emails[1]}, 201
    )["club"]
    club_b = owner.post(
        "/v1/admin/clubs", {"name": "Synthetic club B", "shortName": "SYN-B", "managerEmail": emails[2]}, 201
    )["club"]
    a, b = "/v1/clubs/" + club_a["id"], "/v1/clubs/" + club_b["id"]
    admin_a, admin_b = "/v1/admin/clubs/" + club_a["id"], "/v1/admin/clubs/" + club_b["id"]
    public_a, public_b = "/v1/public/clubs/" + club_a["id"], "/v1/public/clubs/" + club_b["id"]
    duplicate = owner.post(
        "/v1/admin/clubs", {"name": "Rejected duplicate assignment", "managerEmail": emails[1]}, expected=409
    )
    require(duplicate["error"]["code"] == "user_has_club", "Duplicate user assignment was not explicitly rejected")
    owner.post(admin_b + "/assign", {"email": emails[1]}, expected=409)
    owner.get(a, expected=(403, 404))
    require(owner.get("/v1/clubs")["clubs"] == [], "Platform owner received implicit private membership")
    for client, club in ((first, club_a), (second, club_b)):
        require(client.get("/v1/me")["user"]["assignedClubId"] == club["id"], "Assignment is missing from the current user")
        require(
            {item["id"] for item in client.get("/v1/clubs")["clubs"]} == {club["id"]},
            "Club listing differs from the sole assigned club",
        )
    checkpoint("owner-only provisioning and one assigned club per account")

    first.get(b, expected=(403, 404))
    first.request("PATCH", b, {"name": "Forbidden rename"}, expected=(403, 404))
    first.get(b + "/branding/draft", expected=(403, 404))
    first.post(b + "/branding/publish", expected=(403, 404))
    second.get(a, expected=(403, 404))
    first.request("DELETE", a, expected=403)
    owner.request("DELETE", a, expected=403)
    checkpoint("assigned-club reads, writes and legacy archive route remain restricted")

    theme_a = theme("#134e4a", "#052e2b", "#d9f99d")
    theme_b = theme("#4338ca", "#1e1b4b", "#e9d5ff")
    first.request("PUT", a + "/branding/draft", {"theme": theme_a})
    second.request("PUT", b + "/branding/draft", {"theme": theme_b})
    require(first.get(a + "/branding/draft")["theme"] == theme_a, "Club A draft did not persist")
    require(second.get(b + "/branding/draft")["theme"] == theme_b, "Club B draft did not persist")
    anonymous.get(public_a, expected=404)
    anonymous.get(public_b, expected=404)
    checkpoint("independent branding drafts remain private before publication")

    version_one = first.post(a + "/branding/publish")["version"]
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

    conflicting = first.post(a + "/invitations", {"email": emails[2], "role": "coach"}, 201)["invitation"]
    conflicting_code = inbox.token(emails[2])
    conflict = second.post("/v1/invitations/accept", {"token": conflicting_code}, expected=409)
    require(conflict["error"]["code"] == "user_has_club", "Cross-club invitation did not enforce one assigned club")
    persisted = next(item for item in first.get(a + "/invitations")["invitations"] if item["id"] == conflicting["id"])
    require(persisted["acceptedAt"] is None, "Rejected assignment consumed its invitation")
    require(second.get("/v1/me")["user"]["assignedClubId"] == club_b["id"], "Rejected invitation changed the account's club")
    first.request("DELETE", a + "/invitations/" + conflicting["id"], expected=204)
    first.post(a + "/invitations", {"email": emails[3], "role": "coach"}, 201)
    invitation_code = inbox.token(emails[3])
    invited = coach.post("/v1/invitations/accept", {"token": invitation_code})["club"]
    require(invited["id"] == club_a["id"], "Invitation accepted into the wrong club")
    coach.post("/v1/invitations/accept", {"token": invitation_code}, expected=400)
    require(coach.get(a)["club"]["role"] == "coach", "Invitation did not assign the requested role")
    require(coach.get("/v1/me")["user"]["assignedClubId"] == club_a["id"], "Invitation did not assign the account's sole club")
    require(len(coach.get("/v1/clubs")["clubs"]) == 1, "Coach acquired more than one active club")
    coach.request("PUT", a + "/branding/draft", {"theme": theme_b}, expected=403)
    coach.post(a + "/branding/publish", expected=403)
    coach.get(a + "/members", expected=403)
    coach.get(a + "/invitations", expected=403)
    checkpoint("conflicting invitations preserve tokens and fresh coaches receive exactly one club")

    previous_session = coach.clone()
    coach.post("/v1/auth/refresh")
    previous_session.get("/v1/me", expected=401)
    require(coach.get("/v1/me")["user"]["emailVerified"], "Rotated session did not authenticate")
    revoked_session = coach.clone()
    coach.post("/v1/auth/logout", expected=204)
    revoked_session.get("/v1/me", expected=401)
    coach.get("/v1/me", expected=401)
    coach.post("/v1/auth/login", {"email": emails[3], "password": password})
    checkpoint("session rotation and server-side logout revocation")

    known = anonymous.post("/v1/auth/forgot-password", {"email": emails[1]}, expected=202)
    unknown = anonymous.post(
        "/v1/auth/forgot-password", {"email": f"missing-{nonce}@example.invalid"}, expected=202
    )
    require(known == unknown, "Password recovery reveals whether an account exists")
    reset_code = inbox.token(emails[1])
    reset_session = first.clone()
    anonymous.post("/v1/auth/reset-password", {"token": reset_code, "password": next_password}, expected=204)
    first.get("/v1/me", expected=401)
    reset_session.get("/v1/me", expected=401)
    first.post("/v1/auth/login", {"email": emails[1], "password": password}, expected=401)
    first.post("/v1/auth/login", {"email": emails[1], "password": next_password})
    require(first.get("/v1/me")["user"]["assignedClubId"] == club_a["id"], "Password reset changed club assignment")
    anonymous.post("/v1/auth/reset-password", {"token": reset_code, "password": password}, expected=400)
    checkpoint("generic recovery response, one-use reset and revocation of existing sessions")

    first.request("DELETE", admin_a, expected=403)
    owner.request("DELETE", admin_a, expected=204)
    anonymous.get(public_a, expected=404)
    coach.get(a, expected=(403, 404))
    require(second.get(b)["club"]["id"] == club_b["id"], "Archiving one club affected another club")
    checkpoint("platform-owner-only soft archive preserves isolation")
    print(f"Sprint 1 smoke passed: {len(passed)} functional checkpoints; synthetic data and emails retained.")

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default="http://127.0.0.1:18081", help="API or Manager proxy base URL")
    parser.add_argument("--mailpit-url", default="http://127.0.0.1:18025", help="Development Mailpit HTTP API")
    bootstrap = parser.add_mutually_exclusive_group(required=True)
    bootstrap.add_argument(
        "--bootstrap-owner", action="store_true",
        help="Explicitly bootstrap the synthetic verified owner through local docker compose exec",
    )
    bootstrap.add_argument(
        "--owner-binary", type=Path,
        help="Absolute trusted API binary path; invokes owner set using inherited PostgreSQL configuration (CI)",
    )
    args = parser.parse_args()
    if args.owner_binary and (not args.owner_binary.is_absolute() or not args.owner_binary.is_file()):
        parser.error("--owner-binary must point to an existing trusted absolute executable path")
    for label, value in (("base URL", args.base_url), ("Mailpit URL", args.mailpit_url)):
        parsed = urlsplit(value)
        if parsed.scheme not in ("http", "https") or not parsed.netloc or parsed.username or parsed.password:
            parser.error(f"Invalid {label}; use an HTTP(S) URL without credentials")
    try:
        run(args.base_url.rstrip("/"), args.mailpit_url.rstrip("/"), args.owner_binary)
    except (SmokeFailure, KeyError, TypeError, StopIteration) as error:
        # Key names are safe; never emit server response payloads or token values.
        if isinstance(error, SmokeFailure):
            print("FAIL:", error, file=sys.stderr)
        else:
            print("FAIL: API response did not match the documented contract", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
