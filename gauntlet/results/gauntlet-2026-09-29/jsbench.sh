#!/usr/bin/env bash
# JS gauntlet: every case in its own node process (V8 carries state across
# benchmarks in one process), old and new trees alternating, order swapped each
# round. Each process prints the median of its own 7 runs. CSV round,tree,case,ns.
S=/c/Users/hamza/AppData/Local/Temp/claude/C--d-oroboros/d5742a4a-ab53-485f-bd4e-df0b95ce1634/scratchpad/gauntlet
NEW=/c/d/oroboros/gauntlet/js
OLD=$S/old/gauntlet/js
ROUNDS=${ROUNDS:-3}
cases=(
  "native.mjs g1-dot-hand" "native.mjs g1-dot-native"
  "native.mjs g2-centroid-hand" "native.mjs g2-centroid-native"
  "native.mjs g3-sum-hand" "native.mjs g3-sum-native"
  "native.mjs g4-map-hand" "native.mjs g4-object-hand" "native.mjs g4-tally-native" "native.mjs g4-generic-native"
  "native.mjs s-early-hand" "native.mjs s-early-native" "native.mjs s-late-hand" "native.mjs s-late-native"
  "native.mjs g7-alloc-hand" "native.mjs g7-alloc-native"
  "native.mjs g7-reuse-hand" "native.mjs g7-reuse-noalias" "native.mjs g7-reuse-native"
  "jsontok.mjs u8" "jsontok.mjs array" "jsontok.mjs native"
  "jsontree.mjs rec" "jsontree.mjs flat" "jsontree.mjs native"
)
for r in $(seq 1 $ROUNDS); do
  if [ $((r % 2)) -eq 1 ]; then order="old new"; else order="new old"; fi
  for c in "${cases[@]}"; do
    set -- $c
    for tree in $order; do
      dir=$NEW; [ $tree = old ] && dir=$OLD
      ns=$(cd $dir && node $1 $2 2>&1 | grep -oE '[0-9.]+ ns/op' | awk '{print $1}')
      echo "$r,$tree,${1%.mjs}:$2,$ns"
    done
  done
done
