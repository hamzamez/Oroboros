#!/usr/bin/env bash
# Windows gauntlet: every case in its own process, old and new harnesses
# alternating, order swapped each round. "old" is the compiler at e07ffc9,
# before a build's dead buffer was released (gauntlet-2026-11-16 §4), on the
# same ported sources; "new" the committed gauntlet/windows/*.ll.
# Each process checks every case against Go's answers, then prints the median
# of its 9 batches. CSV round,tree,case,ns,peakKB.
S=/c/Users/hamza/AppData/Local/Temp/claude/a27858592609c5af/d57e0ed4-2363-4c51-a8f2-f1d9ee6673b6/scratchpad/g6
ROUNDS=${ROUNDS:-5}
cases="g1-dot1024-hand g1-dot1024-gen g1-dot65536-hand g1-dot65536-gen g2-centroid-hand g2-centroid-gen
g3-sum-hand g3-sum-gen g6-alloc-hand g6-alloc-gen g6-into-hand g6-into-restrict g6-into-gen
s-early-hand s-early-gen s-late-hand s-late-gen g7-tok-hand g7-tok-gen tree-stack-hand tree-calloc-hand tree-gen"
for r in $(seq 1 $ROUNDS); do
  if [ $((r % 2)) -eq 1 ]; then order="old new"; else order="new old"; fi
  for c in $cases; do
    for tree in $order; do
      dir=$S/wnew; [ $tree = old ] && dir=$S/wold
      $dir/bench.exe "$(cygpath -w $dir)" $c | awk -v r=$r -v t=$tree '/ns\/op/ { print r "," t "," $1 "," $2 "," $(NF-1) }'
    done
  done
done
