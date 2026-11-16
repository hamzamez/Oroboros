#!/usr/bin/env bash
# The JVM tokenizer's generated case under its own name in each tree: byte[]
# since ADR 0083 (new), short[] before it (old, 525ab98). javacases.txt is
# 09-29's list, which names only the old one. CSV round,tree,case,ns.
S=/c/Users/hamza/AppData/Local/Temp/claude/a27858592609c5af/d57e0ed4-2363-4c51-a8f2-f1d9ee6673b6/scratchpad/g6
for r in 1 2 3; do
  if [ $((r % 2)) -eq 1 ]; then order="old new"; else order="new old"; fi
  for tree in $order; do
    if [ $tree = old ]; then cp=$(cygpath -w $S/jold); name="J  tokenize short[]    GEN "; else cp=$(cygpath -w $S/jnew); name="J  tokenize byte[]     GEN "; fi
    ns=$(java -cp "$cp" JsonTokBench "$name" </dev/null 2>&1 | grep -E ' ns' | sed -E 's/^.*[^ ] +([0-9.]+) ns.*/\1/' | head -1)
    echo "$r,$tree,JsonTokBench:J tokenize GEN,$ns"
  done
done
