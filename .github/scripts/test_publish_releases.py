#!/usr/bin/env python3
"""Tests for publish_releases.py (no network)."""

from __future__ import annotations

import json
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import publish_releases as pr  # noqa: E402


def sample_release(tag="v1.2.3", prerelease=False, digest=True):
    def asset(name, body: bytes):
        a = {
            "name": name,
            "size": len(body),
            "browser_download_url": f"https://github.example/download/{tag}/{name}",
        }
        if digest:
            a["digest"] = "sha256:" + pr.sha256_hex(body)
        return a

    bodies = {
        "robld-darwin-arm64": b"arm64-bin",
        "robld-darwin-amd64": b"amd64-bin",
        "robld-windows-amd64.exe": b"win-bin",
        "notes.md": b"ignore me",
    }
    return {
        "tag_name": tag,
        "draft": False,
        "prerelease": prerelease,
        "published_at": "2026-09-13T00:26:03Z",
        "html_url": f"https://github.com/aduermael/robld/releases/tag/{tag}",
        "assets": [asset(n, b) for n, b in bodies.items()],
        "_bodies": bodies,
    }


class ParseTests(unittest.TestCase):
    def test_asset_names(self):
        self.assertEqual(pr.parse_asset_name("robld-darwin-arm64"), ("darwin", "arm64"))
        self.assertEqual(
            pr.parse_asset_name("robld-windows-amd64.exe"), ("windows", "amd64")
        )
        self.assertIsNone(pr.parse_asset_name("notes.md"))
        self.assertIsNone(pr.parse_asset_name("robld.exe"))

    def test_digest_hex(self):
        h = "a" * 64
        self.assertEqual(pr.digest_hex("sha256:" + h), h)
        self.assertEqual(pr.digest_hex(h.upper()), h)
        self.assertIsNone(pr.digest_hex("md5:abcd"))
        self.assertIsNone(pr.digest_hex(""))


class PublishTests(unittest.TestCase):
    def fake_fetch(self, releases, bodies_by_url):
        def fetch(url: str, binary: bool) -> bytes:
            if "api.github.com" in url and "/releases" in url:
                payload = [{k: v for k, v in r.items() if k != "_bodies"} for r in releases]
                return json.dumps(payload).encode()
            if binary:
                if url not in bodies_by_url:
                    raise RuntimeError(f"unexpected download {url}")
                return bodies_by_url[url]
            raise RuntimeError(f"unexpected fetch {url} binary={binary}")

        return fetch

    def test_writes_json_and_latest_binaries(self):
        rel = sample_release()
        bodies = {
            a["browser_download_url"]: rel["_bodies"][a["name"]]
            for a in rel["assets"]
            if a["name"] in rel["_bodies"]
        }
        import tempfile

        with tempfile.TemporaryDirectory() as td:
            docs = Path(td) / "docs"
            docs.mkdir()
            catalog = pr.publish(
                docs,
                site="https://robld.com",
                repo="aduermael/robld",
                fetch=self.fake_fetch([rel], bodies),
            )
            self.assertEqual(catalog["latest"], "v1.2.3")
            self.assertEqual(len(catalog["releases"]), 1)

            latest = json.loads((docs / "releases" / "latest.json").read_text())
            self.assertEqual(latest["version"], "v1.2.3")
            self.assertEqual(latest["released"], "2026-09-13T00:26:03Z")
            names = [a["name"] for a in latest["assets"]]
            self.assertEqual(
                names,
                [
                    "robld-darwin-amd64",
                    "robld-darwin-arm64",
                    "robld-windows-amd64.exe",
                ],
            )
            arm = next(a for a in latest["assets"] if a["name"] == "robld-darwin-arm64")
            self.assertEqual(arm["os"], "darwin")
            self.assertEqual(arm["arch"], "arm64")
            self.assertEqual(arm["url"], "https://robld.com/releases/latest/robld-darwin-arm64")
            self.assertTrue(arm["github_url"].endswith("/robld-darwin-arm64"))
            self.assertEqual(arm["sha256"], pr.sha256_hex(b"arm64-bin"))
            self.assertEqual(arm["size"], len(b"arm64-bin"))

            got = (docs / "releases" / "latest" / "robld-darwin-arm64").read_bytes()
            self.assertEqual(got, b"arm64-bin")
            exe = (docs / "releases" / "latest" / "robld-windows-amd64.exe").read_bytes()
            self.assertEqual(exe, b"win-bin")
            self.assertFalse((docs / "releases" / "latest" / "notes.md").exists())

            all_rel = json.loads((docs / "releases.json").read_text())
            self.assertEqual(all_rel["latest"], "v1.2.3")
            hist = all_rel["releases"][0]["assets"][0]
            self.assertTrue(hist["url"].startswith("https://github.example/download/"))

            sums = (docs / "releases" / "latest" / "SHA256SUMS").read_text()
            self.assertIn(arm["sha256"], sums)

    def test_latest_skips_prerelease(self):
        pre = sample_release("v2.0.0-rc.1", prerelease=True)
        stable = sample_release("v1.9.0")
        bodies = {}
        for rel in (pre, stable):
            for a in rel["assets"]:
                if a["name"] in rel["_bodies"]:
                    bodies[a["browser_download_url"]] = rel["_bodies"][a["name"]]
        import tempfile

        with tempfile.TemporaryDirectory() as td:
            docs = Path(td) / "docs"
            docs.mkdir()
            catalog = pr.publish(
                docs,
                site="https://robld.com",
                repo="aduermael/robld",
                fetch=self.fake_fetch([pre, stable], bodies),
            )
            self.assertEqual(catalog["latest"], "v1.9.0")
            self.assertEqual(len(catalog["releases"]), 2)

    def test_sha256_mismatch_fails(self):
        rel = sample_release()
        bodies = {
            a["browser_download_url"]: b"TAMPERED"
            for a in rel["assets"]
            if a["name"] in rel["_bodies"]
        }
        import tempfile

        with tempfile.TemporaryDirectory() as td:
            docs = Path(td) / "docs"
            docs.mkdir()
            with self.assertRaises(RuntimeError) as ctx:
                pr.publish(
                    docs,
                    site="https://robld.com",
                    repo="aduermael/robld",
                    fetch=self.fake_fetch([rel], bodies),
                )
            self.assertIn("sha256 mismatch", str(ctx.exception))


if __name__ == "__main__":
    unittest.main()
