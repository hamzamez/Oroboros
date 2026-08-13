#!/usr/bin/env bash
# Output size measurement — requirement 6, and the price of specialization.
#
# Two questions:
#
#   1. What is the floor? Every target inherits a runtime we do not control, and
#      if that dominates, "small binaries" means something different per target.
#   2. What does specialization cost per call site? g3 left specialization-versus-
#      size unresolved. Since a non-recursive definition is a rewrite rule, every
#      call site gets its own copy, which is the Rust/C++ monomorphization
#      problem. This prices it.
#
# Shared   = one outlined function, called from N sites.
# Special  = N specialized copies, as rewriting would emit.
set -u
cd "$(dirname "$0")"
rm -rf work && mkdir -p work
SIZES="0 1 10 50 100 200"

gen_go() {  # $1 = n, $2 = mode
  local n=$1
  local mode=$2
  local d=work/go_${mode}_${n}
  mkdir -p "$d"
  cat > "$d/go.mod" <<< $'module sz\n\ngo 1.26\n'
  {
    echo 'package main'
    echo 'import "os"'
    echo 'var sink float64'
    if [ "$mode" = shared ]; then
      echo 'func fold(xs []float64, k float64) float64 {'
      echo '  acc := 0.0'
      echo '  for i := 0; i < len(xs); i++ { acc += xs[i]*k + float64(i) }'
      echo '  return acc'
      echo '}'
    fi
    echo 'func main() {'
    echo '  xs := make([]float64, 8)'
    echo '  sink += float64(len(xs))'
    for ((i=0;i<n;i++)); do
      if [ "$mode" = shared ]; then
        echo "  sink += fold(xs, $((i+1)).5)"
      else
        # A distinct specialized copy, as rewriting would emit at each site.
        echo "  { acc := 0.0; for i := 0; i < len(xs); i++ { acc += xs[i]*$((i+1)).5 + float64(i) }; sink += acc }"
      fi
    done
    echo '  if sink == 12345.0 { os.Exit(1) }'
    echo '}'
  } > "$d/main.go"
  ( cd "$d" && go build -ldflags="-s -w" -o app . 2>/dev/null ) && wc -c < "$d/app" | tr -d ' '
}

gen_js() {  # $1 = n, $2 = mode
  local n=$1
  local mode=$2
  local f=work/js_${mode}_${n}.mjs
  {
    echo 'let sink = 0;'
    echo 'const xs = new Float64Array(8); sink += xs.length;'
    if [ "$mode" = shared ]; then
      echo 'function fold(xs, k) { let acc = 0; for (let i = 0; i < xs.length; i++) acc += xs[i]*k + i; return acc; }'
    fi
    for ((i=0;i<n;i++)); do
      if [ "$mode" = shared ]; then
        echo "sink += fold(xs, $((i+1)).5);"
      else
        echo "{ let acc = 0; for (let i = 0; i < xs.length; i++) acc += xs[i]*$((i+1)).5 + i; sink += acc; }"
      fi
    done
    echo 'if (sink === 12345) process.exit(1);'
  } > "$f"
  gzip -9 -c "$f" > "$f.gz"
  echo "$(wc -c < "$f" | tr -d ' ') $(wc -c < "$f.gz" | tr -d ' ')"
}

printf '%-8s | %-21s | %-21s | %s\n' "N" "GO shared / special" "JS raw shared/special" "JS gzip shared/special"
printf '%.0s-' {1..92}; echo
for n in $SIZES; do
  gs=$(gen_go "$n" shared);  gp=$(gen_go "$n" special)
  read -r jsr jsz <<< "$(gen_js "$n" shared)"
  read -r jpr jpz <<< "$(gen_js "$n" special)"
  printf '%-8s | %9s %11s | %9s %11s | %9s %11s\n' "$n" "$gs" "$gp" "$jsr" "$jpr" "$jsz" "$jpz"
done
