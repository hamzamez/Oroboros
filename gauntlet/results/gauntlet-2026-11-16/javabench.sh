#!/usr/bin/env bash
# Java gauntlet: EACH CASE in its own JVM (the harnesses share one timing call
# site, megamorphic after a few lambdas), old and new classes alternating, the
# order swapped each round. A harness checks agreement before timing, and then
# times only the named case with its own statistic (NativeBench: median of 9;
# NativeBench2, JsonTok, JsonTree: best of 9). CSV round,tree,case,ns.
S=/c/Users/hamza/AppData/Local/Temp/claude/a27858592609c5af/d57e0ed4-2363-4c51-a8f2-f1d9ee6673b6/scratchpad/g6
ROUNDS=${ROUNDS:-3}
for r in $(seq 1 $ROUNDS); do
  if [ $((r % 2)) -eq 1 ]; then order="old new"; else order="new old"; fi
  while IFS='|' read -r prog name; do
    for tree in $order; do
      cp=$(cygpath -w $S/jnew); [ $tree = old ] && cp=$(cygpath -w $S/jold)
      ns=$(java -cp "$cp" "$prog" "$name" </dev/null 2>&1 | grep -E ' ns' | sed -E 's/^.*[^ ] +([0-9.]+) ns.*/\1/' | head -1)
      echo "$r,$tree,$prog:$(echo "$name" | tr -s ' '),$ns"
    done
  done < "$(dirname "$0")/javacases.txt"
done
