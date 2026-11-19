#!/usr/bin/env bash
# THE GAUNTLET'S PAIRS, INTERLEAVED, ON FOUR HOSTS (pairs-2026-11-19): each
# generated form against its hand-written reference, timed back to back in one
# process, the order alternating round by round (ABBA), and the median of the
# rounds' ratios printed with their range. Each pair is one process, so a case's
# state does not reach the next pair's.
#
#   bash gauntlet/pairs.sh OUT [go js java windows]   OUT receives the Java
#                                                       classes and bench.exe
#   ROUNDS=9 bash gauntlet/pairs.sh OUT go
#
# Run it from a foreground terminal, with nothing else running, and say which it
# was: a background process may be placed on the efficiency cores
# (jsontok-2026-11-19 §2).
set -euo pipefail
cd "$(dirname "$0")/.."
out=$1
shift
hosts=${*:-go js java windows}
rounds=${ROUNDS:-7}
mkdir -p "$out"
w() { cygpath -w "$1" 2>/dev/null || echo "$1"; }

for h in $hosts; do
  case $h in
  go)
    echo "── go"
    (cd gauntlet/go && go test -c -o "$(w "$out/gauntlet.test.exe")" . >/dev/null)
    for p in SmallDotNative,SmallDotHand G2CentroidNative,G2Centroid G3SumF64Native,G3SumF64 \
      G4WordCountNativeInc,G4WordCountIncr G4WordCountNative,G4WordCountReadWrite \
      G5NativeReport,G5NativeReportHand StencilFreshNative,StencilFreshRef StencilIntoNative,StencilIntoRef \
      SearchNative,SearchRef SearchNativeLate,SearchRefLate TokGen,TokHandBytes TreeGen,TreeFlat; do
      (cd gauntlet/go && "$out/gauntlet.test.exe" -test.run '^TestInterleavedPairs$' -pairs="$p" -rounds="$rounds") |
        grep -v '^generated\|^PASS'
    done
    ;;
  js)
    echo "── js"
    while read -r file gen hand; do
      (cd gauntlet/js && node "$file" --pair "$gen" "$hand" --rounds "$rounds")
    done <<EOF
native.mjs g1-dot-native g1-dot-hand
native.mjs g2-centroid-native g2-centroid-hand
native.mjs g3-sum-native g3-sum-hand
native.mjs g4-tally-native g4-map-hand
native.mjs s-early-native s-early-hand
native.mjs s-late-native s-late-hand
native.mjs g7-alloc-native g7-alloc-hand
native.mjs g7-reuse-native g7-reuse-hand
jsontok.mjs native array
jsontree.mjs native flat
EOF
    ;;
  java)
    echo "── java"
    rm -rf "$out/java" && mkdir -p "$out/java"
    # One.java is left out: it calls JsonTreeBench.shortsOf, gone since ADR 0083
    (cd gauntlet/java && javac -encoding UTF-8 -nowarn -d "$(w "$out/java")" \
      $(ls *.java | grep -v '^One.java$') gen/*.java 2>&1 | grep -v '^Note' || true)
    while IFS='|' read -r cls gen hand; do
      java -cp "$(w "$out/java")" "$cls" --pair "$gen" "$hand" --rounds "$rounds" </dev/null | grep ' pair$'
    done <<EOF
NativeBench|dot  n=65536  GENERATED|dot  n=65536  hand-written
NativeBench|dot  n=1024   GENERATED|dot  n=1024   hand-written
NativeBench|centroid      GENERATED|centroid      hand-written
NativeBench|wordcount     GENERATED|wordcount     hand, FUSED merge
NativeBench|search early  GENERATED|search early  hand-written
NativeBench|search late   GENERATED|search late   hand-written
NativeBench2|sum-of      GENERATED|sum-of      hand-written
NativeBench2|alloc       GENERATED|alloc       hand-written
NativeBench2|into (reuse) GENERATED|into (reuse) hand-written
JsonTokBench|J  tokenize byte[]     GEN |J  tokenize byte[]     hand
JsonTreeBench|T  tree flat          GEN |T  tree flat          hand
EOF
    ;;
  windows)
    echo "── windows"
    bash gauntlet/windows/bench-build.sh "$out/win" >/dev/null
    for p in "g1-dot1024-gen g1-dot1024-hand" "g1-dot65536-gen g1-dot65536-hand" "g2-centroid-gen g2-centroid-hand" \
      "g3-sum-gen g3-sum-hand" "g6-alloc-gen g6-alloc-hand" "g6-into-gen g6-into-hand" \
      "s-early-gen s-early-hand" "s-late-gen s-late-hand" "g7-tok-gen g7-tok-hand" "tree-gen tree-stack-hand"; do
      set -- $p
      "$out/win/bench.exe" "$(w "$out/win")" pair "$1" "$2" "$rounds"
    done
    ;;
  esac
done
