#!/usr/bin/env python3
"""Every ```sql block in every reference must parse as PostgreSQL.

Uses pglast (libpg_query bindings; the 8.x series carries the Postgres 18 grammar).
Blocks must be complete statements; psql meta-commands (\\d+, \\di) go in ```psql fences,
which are skipped. A block may hold several statements separated by semicolons.
"""
import pathlib, re, sys
try:
    from pglast import parse_sql
    from pglast.parser import ParseError
except ImportError:
    print("FAIL: pglast not installed — pip install -r verify/requirements.txt")
    sys.exit(1)

root = pathlib.Path(__file__).resolve().parent.parent
total = ok = 0
bad = []
for f in sorted((root/"references").rglob("*.md")):
    for i, m in enumerate(re.finditer(r"```sql\n(.*?)```", f.read_text(), re.S), 1):
        total += 1
        try:
            parse_sql(m.group(1))
            ok += 1
        except ParseError as e:
            bad.append(f"{f.relative_to(root)} block #{i}: {e}")

print(f"{ok}/{total} sql blocks parse" + ("" if not bad else "\nFAIL:\n  " + "\n  ".join(bad)))
sys.exit(1 if bad else 0)
