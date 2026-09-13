#!/usr/bin/env python3
"""Every URL cited in references/ must be listed in SOURCES.md and match an allowed origin.

The allowed origins live in SOURCES.md under `## Allowed origins`, one per line:

    T1 | postgresql.org | *
    T1 | github.com | /pgaudit/,/pgvector/,/jackc/
    T3 | brandur.org | * (Brandur Leach)

Tier, host, and either `*` or a comma-separated list of path prefixes. A tier-3 entry must
name its author in parentheses. Hosts match exactly or as a `www.` variant; a subdomain is
not covered by its parent (so a Medium subdomain never rides on a tier-3 blog rule).

Both directions are checked: a cited URL absent from SOURCES.md fails, and a SOURCES.md URL
cited nowhere fails, so the list stays honest.
"""
import pathlib, re, sys
from urllib.parse import urlsplit

root = pathlib.Path(__file__).resolve().parent.parent
sources_text = (root/"SOURCES.md").read_text()

def norm(url: str) -> str:
    u = urlsplit(url.strip().rstrip(").,;"))
    host = u.netloc.lower().removeprefix("www.")
    path = u.path.rstrip("/") or "/"
    return f"{host}{path}" + (f"?{u.query}" if u.query else "")

def host_path(url: str):
    u = urlsplit(url)
    return u.netloc.lower().removeprefix("www."), (u.path or "/")

# allowed origins
origins = []
sec = re.search(r"^## Allowed origins\n(.*?)(?=^## |\Z)", sources_text, re.S | re.M)
if not sec:
    print("FAIL: SOURCES.md has no '## Allowed origins' section"); sys.exit(1)
for line in sec.group(1).splitlines():
    m = re.match(r"^\s*(T[123])\s*\|\s*([a-z0-9.-]+)\s*\|\s*(.+?)\s*$", line)
    if not m:
        continue
    tier, host, rest = m.groups()
    author = re.search(r"\(([^)]+)\)", rest)
    prefixes = rest.split("(")[0].strip()
    if tier == "T3" and not author:
        print(f"FAIL: tier-3 origin {host} must name its author in parentheses"); sys.exit(1)
    origins.append((tier, host.removeprefix("www."), None if prefixes == "*" else [p.strip() for p in prefixes.split(",")]))

def allowed(url: str):
    host, path = host_path(url)
    for tier, h, prefixes in origins:
        if host == h and (prefixes is None or any(path.startswith(p) for p in prefixes)):
            return tier
    return None

URL = re.compile(r"https?://[^\s<>()\]`'\"]+")
cited = {}
for f in sorted((root/"references").rglob("*.md")):
    for m in URL.finditer(f.read_text()):
        cited.setdefault(norm(m.group(0)), (m.group(0), str(f.relative_to(root))))

listed = {norm(m.group(0)): m.group(0) for m in URL.finditer(sources_text)}

fail = []
for key, (url, where) in cited.items():
    if key not in listed:
        fail.append(f"{where}: {url} not in SOURCES.md")
    if allowed(url) is None:
        fail.append(f"{where}: {url} is not from an allowed origin")
for key, url in listed.items():
    if key not in cited:
        fail.append(f"SOURCES.md lists {url} but no reference cites it")

print("FAIL:\n  " + "\n  ".join(fail) if fail else f"{len(cited)} cited URLs, all listed and allowed")
sys.exit(1 if fail else 0)
