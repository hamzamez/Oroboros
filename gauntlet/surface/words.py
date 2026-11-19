"""Count each J keyword's uses across the corpus's .oro files, comments and
string literals removed: the words audit's first measurement (assessment
2026-11-18 §5 item 6)."""
import os
import re
import sys

root = sys.argv[1]
reserved = "fn spec lemma set array map var loop again case if else try expect assert use module provides pub export axiom true false".split()
contextual = ("where ensure invariant pure index variadic inout out linear as host expr stmt call stdcall cdecl "
              "import lib checked jump comma_ok success relevant mod record with").split()
model = ("repr word niche narrow structural backend link build artifact data include fact underlying read write "
         "realize glue triple runtime c pack size len NULL").split()
words = reserved + contextual + model

tok = re.compile(r"[A-Za-z_][A-Za-z0-9_]*")
counts = {w: {"targets": 0, "lib": 0, "examples": 0, "cases": 0, "stdlib": 0} for w in words}
files = {w: set() for w in words}


def area(path):
    p = path.replace("\\", "/")
    if "/targets/" in p:
        return "targets"
    if "/lib/" in p:
        return "lib"
    if "/examples/" in p:
        return "examples"
    if "/gauntlet/differential/" in p:
        return "cases"
    if "/gauntlet/" in p:
        return "stdlib"
    return None


for d in ["targets", "lib", "examples", "gauntlet"]:
    for dp, _, fs in os.walk(os.path.join(root, d)):
        for f in fs:
            if not f.endswith(".oro"):
                continue
            path = os.path.join(dp, f)
            a = area(path)
            if a is None:
                continue
            text = open(path, encoding="utf-8", errors="replace").read()
            text = re.sub(r'"(\\.|[^"\\])*"', '""', text)
            text = re.sub(r"//[^\n]*", "", text)
            for t in tok.findall(text):
                if t in counts:
                    counts[t][a] += 1
                    files[t].add(os.path.relpath(path, root).replace("\\", "/"))

print("| word | class | targets | lib | examples | cases | stdlib | files |")
print("|---|---|---:|---:|---:|---:|---:|---:|")
for w in words:
    c = counts[w]
    cls = "reserved" if w in reserved else "contextual" if w in contextual else "model"
    print(f"| `{w}` | {cls} | {c['targets']} | {c['lib']} | {c['examples']} | {c['cases']} | {c['stdlib']} | {len(files[w])} |")
