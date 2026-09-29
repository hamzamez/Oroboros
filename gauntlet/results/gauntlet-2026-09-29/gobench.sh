#!/usr/bin/env bash
# Go gauntlet: EACH BENCHMARK in its own process — a pair in one process let
# the hand-written map benchmark's heap and GC state bias the generated one that
# ran after it (1.3x on identical code). Old and new trees alternate, and the
# order is swapped each round. Output: CSV round,tree,bench,ns.
S=/c/Users/hamza/AppData/Local/Temp/claude/C--d-oroboros/d5742a4a-ab53-485f-bd4e-df0b95ce1634/scratchpad/gauntlet
NEW=/c/d/oroboros/gauntlet/go
OLD=$S/old/gauntlet/go
ROUNDS=${ROUNDS:-5}
pairs=(
  "SmallDotNative|SmallDotHand"
  "G2CentroidNative|G2Centroid"
  "G3SumF64Native|G3SumF64"
  "G3WordTallyNative"
  "G4WordCountNativeInc|G4WordCountIncr"
  "G4WordCountNative|G4WordCountReadWrite"
  "G5NativeReport|G5NativeReportHand"
  "StencilFreshNative|StencilFreshRef"
  "StencilIntoNative|StencilIntoRef"
  "SearchNative|SearchRef"
  "SearchNativeLate|SearchRefLate"
  "TokGen|TokHandBytes"
  "TreeGen|TreeFlatClamped|TreeFlat|TreeRec"
)
for r in $(seq 1 $ROUNDS); do
  if [ $((r % 2)) -eq 1 ]; then order="old new"; else order="new old"; fi
  for p in "${pairs[@]}"; do
    for b in ${p//|/ }; do
      for tree in $order; do
        # each tree's test binary, built once (go test -c): relinking the package
        # for every process cost ~15 s a run and timed nothing
        dir=$NEW; [ $tree = old ] && dir=$OLD
        (cd $dir && $S/$tree.test.exe -test.run '^$' -test.bench "^Benchmark$b\$" -test.benchtime=2s -test.count=1 2>&1) |
          awk -v r=$r -v t=$tree '/^Benchmark/ { n=$1; sub(/^Benchmark/,"",n); sub(/-[0-9]+$/,"",n); print r "," t "," n "," $3 }'
      done
    done
  done
done
