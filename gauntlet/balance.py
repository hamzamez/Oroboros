#!/usr/bin/env python3
"""THE BALANCE OF EFFORT, counted as assessment-2026-10-03 §3.1 defines it.

    python gauntlet/balance.py [--nodes] [REV ...]        default: HEAD

Reads each revision from git, so a past assessment's table can be recounted:
  compiler            raw lines of .go under core/, emit/, ir/ (subdirectories included), tests included
  of which tests      the same, files ending _test.go
  Oroboros written    lines of code of .oro under examples/ and lib/: no blank lines, no comment-only lines
                      (`;` today's comment, `//` J's)
  --nodes             also the language's size in core-term nodes (gauntlet/balance_nodes.go), a
                      cross-check on lines across a change of notation (assessment-2026-10-20 §3.1).
                      Each revision is checked out in a temporary worktree and its own reader counts it.
  largest program     the .oro file under examples/ with the most lines of code
  acceptance          lines of code of gauntlet/stdlib/acceptance/*.oro
  hand declarations   raw lines of every .oro under targets/
  survey tooling      raw lines of gauntlet/stdlib/*.go, gauntlet/stdlib/jdk/*.java, gauntlet/stdlib/jsdump.mjs
  emit : core, (emit + ir) : core   raw lines, tests included
  analysis layer      emit/{interval,refine,linear,monotone}.go
  analysis, extended  the layer and emit/{fact,content,component}.go
  IR analysis         ir/{interval,trip,endpoint,sct,decide,require}.go
"""
import os, shutil, subprocess, sys, tempfile

def files(rev):
    out = subprocess.run(["git", "ls-tree", "-r", "--name-only", rev], capture_output=True, text=True, check=True).stdout
    return out.split("\n")

def blob(rev, path):
    return subprocess.run(["git", "show", f"{rev}:{path}"], capture_output=True, check=True).stdout.decode("utf-8", "replace")

def raw(text):
    return len(text.splitlines())

def code(text):
    n = 0
    for line in text.splitlines():
        s = line.strip()
        if s and not s.startswith(";") and not s.startswith("//"):
            n += 1
    return n

def count(rev):
    fs = files(rev)
    r = {k: 0 for k in ["compiler", "tests", "core", "emit", "ir", "oroboros", "acceptance", "targets", "survey",
                        "layer", "extended", "iranalysis"]}
    largest, lname = 0, ""
    layer = {"emit/interval.go", "emit/refine.go", "emit/linear.go", "emit/monotone.go"}
    extended = layer | {"emit/fact.go", "emit/content.go", "emit/component.go"}
    iran = {"ir/" + n + ".go" for n in ["interval", "trip", "endpoint", "sct", "decide", "require"]}
    for f in fs:
        if not f:
            continue
        top = f.split("/")[0]
        if f.endswith(".go") and top in ("core", "emit", "ir"):
            n = raw(blob(rev, f))
            r["compiler"] += n
            r[top] += n
            if f.endswith("_test.go"):
                r["tests"] += n
            if f in layer:
                r["layer"] += n
            if f in extended:
                r["extended"] += n
            if f in iran:
                r["iranalysis"] += n
        elif f.endswith(".oro") and top in ("examples", "lib"):
            n = code(blob(rev, f))
            r["oroboros"] += n
            if top == "examples" and n > largest:
                largest, lname = n, f
        elif f.endswith(".oro") and top == "targets":
            r["targets"] += raw(blob(rev, f))
        elif f.startswith("gauntlet/stdlib/acceptance/") and f.endswith(".oro"):
            r["acceptance"] += code(blob(rev, f))
        elif (f.startswith("gauntlet/stdlib/") and f.count("/") == 2 and f.endswith(".go")) or \
                (f.startswith("gauntlet/stdlib/jdk/") and f.endswith(".java")) or f == "gauntlet/stdlib/jsdump.mjs":
            r["survey"] += raw(blob(rev, f))
    r["largest"], r["largest_name"] = largest, lname
    return r

def nodes(rev):
    """The core-term node count of examples/ and lib/ at rev, by rev's own reader."""
    tool = os.path.join(os.path.dirname(os.path.abspath(__file__)), "balance_nodes.go")
    tmp = tempfile.mkdtemp(prefix="balance-")
    wt = os.path.join(tmp, "wt")
    subprocess.run(["git", "worktree", "add", "--detach", wt, rev], capture_output=True, check=True)
    try:
        shutil.copy(tool, os.path.join(wt, "gauntlet", "balance_nodes.go"))
        out = subprocess.run(["go", "run", "gauntlet/balance_nodes.go"], cwd=wt, capture_output=True, text=True)
        if out.returncode != 0:
            return "nodes: " + out.stderr.strip().splitlines()[-1]
        return out.stdout.strip()
    finally:
        subprocess.run(["git", "worktree", "remove", "--force", wt], capture_output=True)
        shutil.rmtree(tmp, ignore_errors=True)

args = sys.argv[1:]
with_nodes = "--nodes" in args
args = [a for a in args if a != "--nodes"]
for rev in args or ["HEAD"]:
    r = count(rev)
    print(f"== {rev}")
    print(f"compiler {r['compiler']}  tests {r['tests']}  core {r['core']}  emit {r['emit']}  ir {r['ir']}")
    print(f"emit:core {r['emit']/r['core']:.2f}  (emit+ir):core {(r['emit']+r['ir'])/r['core']:.2f}  overall {r['compiler']/max(r['oroboros'],1):.1f}:1")
    print(f"oroboros {r['oroboros']}  largest {r['largest']} {r['largest_name']}  acceptance {r['acceptance']}")
    print(f"targets {r['targets']}  survey {r['survey']}  layer {r['layer']}  extended {r['extended']}  ir-analysis {r['iranalysis']}")
    if with_nodes:
        print(nodes(rev))
