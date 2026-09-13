#!/usr/bin/env python3
"""Write docs/releases JSON and copy the latest binaries for GitHub Pages.

URLs after deploy (custom domain):

  https://robld.com/releases/latest/robld-darwin-arm64
  https://robld.com/releases/latest.json
  https://robld.com/releases.json
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any, Callable

DEFAULT_REPO = "aduermael/robld"
DEFAULT_SITE = "https://robld.com"
USER_AGENT = "robld-pages"
ASSET_RE = re.compile(r"^robld-([a-z0-9]+)-([a-z0-9]+)(\.exe)?$", re.I)
Fetch = Callable[[str, bool], bytes]


def parse_asset_name(name: str) -> tuple[str, str] | None:
    m = ASSET_RE.match(name.strip())
    if not m:
        return None
    return m.group(1).lower(), m.group(2).lower()


def sha256_hex(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def digest_hex(digest: str | None) -> str | None:
    if not digest:
        return None
    d = digest.strip()
    if d.lower().startswith("sha256:"):
        d = d.split(":", 1)[1]
    d = d.lower()
    if len(d) != 64 or any(c not in "0123456789abcdef" for c in d):
        return None
    return d


def github_headers(token: str | None, *, json_api: bool) -> dict[str, str]:
    h = {"User-Agent": USER_AGENT}
    if json_api:
        h["Accept"] = "application/vnd.github+json"
    if token:
        h["Authorization"] = f"Bearer {token}"
    return h


def default_fetch(token: str | None) -> Fetch:
    def fetch(url: str, binary: bool) -> bytes:
        req = urllib.request.Request(
            url, headers=github_headers(token, json_api=not binary)
        )
        try:
            with urllib.request.urlopen(req, timeout=120) as resp:
                return resp.read()
        except urllib.error.HTTPError as e:
            raise RuntimeError(f"{url}: HTTP {e.code}") from e

    return fetch


def list_releases(repo: str, fetch: Fetch) -> list[dict[str, Any]]:
    out: list[dict[str, Any]] = []
    url: str | None = (
        f"https://api.github.com/repos/{repo}/releases?per_page=100"
    )
    while url:
        raw = fetch(url, False)
        batch = json.loads(raw.decode("utf-8"))
        if not isinstance(batch, list):
            raise RuntimeError(f"unexpected releases payload from {url}")
        out.extend(batch)
        url = None  # public catalog is small; one page is enough for now
    return out


def asset_entry(
    *,
    name: str,
    os_name: str,
    arch: str,
    size: int,
    sha256: str | None,
    url: str,
    github_url: str,
) -> dict[str, Any]:
    entry: dict[str, Any] = {
        "name": name,
        "os": os_name,
        "arch": arch,
        "size": size,
        "sha256": sha256,
        "url": url,
        "github_url": github_url,
    }
    return entry


def release_object(
    rel: dict[str, Any],
    *,
    repo: str,
    site: str,
    stable_urls: bool,
) -> dict[str, Any] | None:
    if rel.get("draft"):
        return None
    tag = (rel.get("tag_name") or "").strip()
    if not tag:
        return None
    assets: list[dict[str, Any]] = []
    for a in rel.get("assets") or []:
        name = (a.get("name") or "").strip()
        parsed = parse_asset_name(name)
        if not parsed:
            continue
        os_name, arch = parsed
        github_url = (a.get("browser_download_url") or "").strip()
        if not github_url:
            github_url = (
                f"https://github.com/{repo}/releases/download/{tag}/{name}"
            )
        url = f"{site}/releases/latest/{name}" if stable_urls else github_url
        assets.append(
            asset_entry(
                name=name,
                os_name=os_name,
                arch=arch,
                size=int(a.get("size") or 0),
                sha256=digest_hex(a.get("digest")),
                url=url,
                github_url=github_url,
            )
        )
    assets.sort(key=lambda x: (x["os"], x["arch"], x["name"]))
    if not assets:
        return None
    return {
        "name": "robld",
        "version": tag,
        "released": rel.get("published_at") or rel.get("created_at"),
        "prerelease": bool(rel.get("prerelease")),
        "changelog": rel.get("html_url")
        or f"https://github.com/{repo}/releases/tag/{tag}",
        "assets": assets,
    }


def pick_latest(objects: list[dict[str, Any]]) -> dict[str, Any] | None:
    for obj in objects:
        if not obj.get("prerelease"):
            return obj
    return objects[0] if objects else None


def write_json(path: Path, payload: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    text = json.dumps(payload, indent=2) + "\n"
    path.write_text(text, encoding="utf-8")


def download_latest_binaries(
    latest: dict[str, Any],
    dest: Path,
    fetch: Fetch,
) -> None:
    if dest.exists():
        for child in dest.iterdir():
            if child.is_file():
                child.unlink()
    dest.mkdir(parents=True, exist_ok=True)
    sums: list[str] = []
    for a in latest["assets"]:
        data = fetch(a["github_url"], True)
        got = sha256_hex(data)
        expected = a.get("sha256")
        if expected and expected != got:
            raise RuntimeError(
                f"{a['name']}: sha256 mismatch (github {expected}, got {got})"
            )
        a["sha256"] = got
        a["size"] = len(data)
        out = dest / a["name"]
        out.write_bytes(data)
        sums.append(f"{got}  {a['name']}")
    (dest / "SHA256SUMS").write_text("\n".join(sums) + "\n", encoding="utf-8")


def publish(
    docs: Path,
    *,
    site: str,
    repo: str,
    fetch: Fetch,
) -> dict[str, Any]:
    site = site.rstrip("/")
    raw = list_releases(repo, fetch)
    objects: list[dict[str, Any]] = []
    for rel in raw:
        obj = release_object(rel, repo=repo, site=site, stable_urls=False)
        if obj:
            objects.append(obj)
    if not objects:
        raise RuntimeError(f"no published robld releases on {repo}")
    latest_src = pick_latest(objects)
    if latest_src is None:
        raise RuntimeError("no latest release")
    latest = release_object(
        next(r for r in raw if (r.get("tag_name") or "").strip() == latest_src["version"]),
        repo=repo,
        site=site,
        stable_urls=True,
    )
    if latest is None:
        raise RuntimeError("latest release has no binaries")
    download_latest_binaries(latest, docs / "releases" / "latest", fetch)
    write_json(docs / "releases" / "latest.json", latest)
    catalog = {
        "name": "robld",
        "latest": latest["version"],
        "releases": objects,
    }
    write_json(docs / "releases.json", catalog)
    return catalog


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument(
        "--docs",
        type=Path,
        default=None,
        help="docs/ directory (default: <repo>/docs)",
    )
    p.add_argument("--site", default=DEFAULT_SITE)
    p.add_argument("--repo", default=DEFAULT_REPO)
    p.add_argument(
        "--token",
        default=os.environ.get("GITHUB_TOKEN") or os.environ.get("GH_TOKEN") or "",
    )
    args = p.parse_args(argv)
    root = Path(__file__).resolve().parents[2]
    docs = args.docs if args.docs is not None else root / "docs"
    token = args.token.strip() or None
    catalog = publish(
        docs,
        site=args.site,
        repo=args.repo,
        fetch=default_fetch(token),
    )
    n = len(catalog["releases"])
    print(
        f"wrote {docs / 'releases.json'} ({n} release(s), latest {catalog['latest']})"
    )
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as e:
        print(f"ERROR: {e}", file=sys.stderr)
        raise SystemExit(1)
