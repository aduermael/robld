#!/usr/bin/env python3
"""Splice INSTALL-PROMPT.md into docs/index.html (placeholder replacement)."""

import html
import sys
from pathlib import Path

MARKER = "INSTALL_PROMPT_PLACEHOLDER"


def inject(root: Path) -> None:
    prompt = (root / "INSTALL-PROMPT.md").read_text(encoding="utf-8")
    index = root / "docs" / "index.html"
    text = index.read_text(encoding="utf-8")
    if MARKER not in text:
        raise SystemExit(f"placeholder {MARKER!r} not found in {index}")
    index.write_text(
        text.replace(MARKER, html.escape(prompt.strip("\n"))),
        encoding="utf-8",
    )
    print(f"injected {len(prompt)} bytes from INSTALL-PROMPT.md into {index}")


if __name__ == "__main__":
    root = Path(__file__).resolve().parents[2]
    if len(sys.argv) > 1:
        root = Path(sys.argv[1]).resolve()
    inject(root)
