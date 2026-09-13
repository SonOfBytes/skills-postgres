#!/usr/bin/env python3
"""Every reference path named in SKILL.md must exist, and every reference file must be routed.

Paths are recognised as `references/<name>.md` or `references/drivers/<name>.md` inside
backticks. A `<layer>` placeholder is not a path. The reverse check catches a reference
file that nothing routes to, which is a file no session will ever read.
"""
import pathlib, re, sys
root = pathlib.Path(__file__).resolve().parent.parent
refs = root/"references"
skill = (root/"SKILL.md").read_text()

named = set()
missing = []
for m in re.finditer(r"`references/((?:drivers/)?[a-z0-9-]+\.md)`", skill):
    named.add(m.group(1))
    if not (refs/m.group(1)).is_file():
        missing.append(m.group(1))

present = {str(p.relative_to(refs)) for p in refs.rglob("*.md")}
unrouted = sorted(present - named)

problems = []
if missing:
    problems.append("missing: " + ", ".join(sorted(set(missing))))
if unrouted:
    problems.append("not routed from SKILL.md: " + ", ".join(unrouted))
print("FAIL " + "; ".join(problems) if problems else f"all {len(named)} routed references exist")
sys.exit(1 if problems else 0)
