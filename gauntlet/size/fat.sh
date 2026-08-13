#!/usr/bin/env bash
# Same comparison at varying body size, to find where specialization stops paying.
set -u
cd "$(dirname "$0")"
rm -rf fatwork && mkdir -p fatwork
N=100
gen() {
  local body=$1 mode=$2
  local d=fatwork/${mode}_${body}
  mkdir -p "$d"
  printf 'module sz\n\ngo 1.26\n' > "$d/go.mod"
  {
    echo 'package main'
    echo 'import "os"'
    echo 'var sink float64'
    if [ "$mode" = shared ]; then
      echo 'func work(xs []float64, k float64) float64 {'
      echo '  acc := 0.0'
      echo '  for i := 0; i < len(xs); i++ {'
      for ((b=0;b<body;b++)); do echo "    acc += xs[i]*k + float64(i*$((b+1)))"; done
      echo '  }'
      echo '  return acc'
      echo '}'
    fi
    echo 'func main() {'
    echo '  xs := make([]float64, 8)'
    echo '  sink += float64(len(xs))'
    for ((i=0;i<N;i++)); do
      if [ "$mode" = shared ]; then
        echo "  sink += work(xs, $((i+1)).5)"
      else
        echo "  { acc := 0.0; for i := 0; i < len(xs); i++ {"
        for ((b=0;b<body;b++)); do echo "    acc += xs[i]*$((i+1)).5 + float64(i*$((b+1)))"; done
        echo "  }; sink += acc }"
      fi
    done
    echo '  if sink == 12345.0 { os.Exit(1) }'
    echo '}'
  } > "$d/main.go"
  ( cd "$d" && go build -ldflags="-s -w" -o app . 2>/dev/null ) && wc -c < "$d/app" | tr -d ' '
}
FLOOR=1495552
printf '%-14s | %-11s | %-11s | %s\n' "ops in body" "shared" "specialized" "special/shared, over floor"
printf '%.0s-' {1..76}; echo
for body in 1 2 4 8 16 32; do
  s=$(gen "$body" shared); p=$(gen "$body" special)
  ds=$((s-FLOOR)); dp=$((p-FLOOR))
  printf '%-14s | %11s | %11s | %s\n' "$body" "$s" "$p" "$(awk -v a=$dp -v b=$ds 'BEGIN{printf "%.2fx  (+%d vs +%d bytes)", a/b, a, b}')"
done
