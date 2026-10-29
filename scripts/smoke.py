#!/usr/bin/env python3
"""Functional smoke check against a running API; exits nonzero on mismatch."""
import json
import sys
from urllib.error import HTTPError
from urllib.request import Request, urlopen

base = (sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:18080").rstrip("/")


def request(path, status=200, method="GET"):
    try:
        response = urlopen(Request(base + path, method=method, headers={"Content-Type": "application/json"}), timeout=5)
    except HTTPError as error:
        response = error
    with response:
        assert response.status == status, (path, response.status, status)
        assert "application/json" in response.headers.get("Content-Type", ""), path
        return json.load(response)


assert request("/health")["status"] == "ok"
assert request("/")["name"] == "FUTRYVA"
variants = request("/v1/sports")
expected = {
    ("football", "Futebol 11", 11), ("football", "Futebol 9", 9),
    ("football", "Futebol 7", 7), ("futsal", "Futsal", 5),
    ("volleyball", "Voleibol de pavilhão", 6),
    ("basketball", "Basquetebol 5x5", 5), ("handball", "Andebol de pavilhão", 7),
}
assert len(variants) == 7, variants
assert {(v["sportCode"], v["variant"], v["playersOnField"]) for v in variants} == expected
request("/v1/not-found", 404)
request("/v1/sports", 405, "POST")
print("Smoke passed: health, API index, all 7 variants / 5 sports, 404 and 405.")
