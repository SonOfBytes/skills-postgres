#!/usr/bin/env python3
"""Every version marker in the references must name a major that versions.md knows.

Markers are `[pg>=N]`, `[pg<=N]`, `[pg=N]` or ranges `[pg>=N,<=M]`. The known set is read
from the `## Majors` table in references/versions.md (first column), so adding a major is a
one-line change there.
"""
import pathlib, re, sys
root = pathlib.Path(__file__).resolve().parent.parent
refs = root/"references"
vtext = (refs/"versions.md").read_text()
sec = re.search(r"^## Majors\n(.*?)(?=^## |\Z)", vtext, re.S | re.M)
if not sec:
    print("FAIL: references/versions.md has no '## Majors' table"); sys.exit(1)
known = {int(m.group(1)) for m in re.finditer(r"^\|\s*(\d{2})\s*\|", sec.group(1), re.M)}
if not known:
    print("FAIL: no majors parsed from versions.md"); sys.exit(1)

MARK = re.compile(r"\[pg((?:[<>=]=?\d{2})(?:,[<>=]=?\d{2})?)\]")
bad = []; count = 0
for f in sorted(refs.rglob("*.md")):
    for m in MARK.finditer(f.read_text()):
        count += 1
        for n in re.findall(r"\d{2}", m.group(1)):
            if int(n) not in known:
                bad.append(f"{f.relative_to(root)}: {m.group(0)} names unknown major {n}")
print(f"{count} version markers, majors known: {sorted(known)}" + ("" if not bad else "\nFAIL:\n  " + "\n  ".join(bad)))
sys.exit(1 if bad else 0)
