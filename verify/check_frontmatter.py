#!/usr/bin/env python3
"""SKILL.md must have valid YAML frontmatter, and `name` must match the install directory.

The skill installs to ~/.claude/skills/<name>, so `name` must equal the directory
holding SKILL.md. In CI the checkout is named after the repo (skills-<name>), so a
leading "skills-" is stripped before comparing.
"""
import pathlib, re, sys

root = pathlib.Path(__file__).resolve().parent.parent
expected = root.name.removeprefix("skills-")
lines = (root/"SKILL.md").read_text().split("\n")

fail = []
if lines[0].strip() != "---":
    fail.append("SKILL.md does not open with ---")
close = next((i for i, l in enumerate(lines[1:], 1) if l.strip() == "---"), None)
if not close:
    fail.append("frontmatter not closed")

fm = "\n".join(lines[1:close]) if close else ""
name = re.search(r"^name:\s*(.+)$", fm, re.M)
desc = re.search(r"^description:\s*(.+)$", fm, re.M)

if not name:
    fail.append("missing name:")
elif name.group(1).strip() != expected:
    fail.append(f"name {name.group(1).strip()!r} != expected {expected!r}")
if not desc:
    fail.append("missing description:")
elif len(desc.group(1).strip()) < 40:
    fail.append("description too short to route on")

print("FAIL: " + "; ".join(fail) if fail else f"frontmatter OK (name={expected})")
sys.exit(1 if fail else 0)
