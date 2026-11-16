# The file programs, as processes

`examples/io`'s five programs read a real file and print a result: `wc`, `count`, `freq`,
`jsonfmt` and `roundtrip`. The gauntlet (`gauntlet/windows`, `gauntlet/go`, …) times kernels on
tables in memory; this times whole programs, so reading the file, decoding the text and
printing are in the measure ([files-2026-11-16](../results/files-2026-11-16.md)).

**References**, each written from the program's meaning, its output checked equal to the
program's byte for byte before anything is timed:
- [`c/`](c/): what a person writes in C, the C runtime's stdio, the whole file read at once as
  `os.ReadFile` does, built by the clang the windows target uses at its `-O2`;
- [`../go/files/`](../go/files/): what a person writes in Go, `os.ReadFile`, `bytes.Count`, a map
  for `freq`, a `bufio.Writer`;
- [`nop.oro`](nop.oro), `c/nop.c` and `../go/files/nop`: the floor, each toolchain's process
  that does nothing. A program's **work** is its time less its own toolchain's floor.

**Inputs** ([inputs.go](inputs.go)), deterministic, each under its program's cap: 8 MiB of words in
lines for `wc` and `count` (cap 2²⁴), 60 KiB of mixed-case words for `freq` and 60 KiB of minified
JSON for `jsonfmt` (cap 2¹⁶).

```bash
go run gauntlet/files/inputs.go IN
```

```bash
go run gauntlet/files/bench.go BIN IN 101 > runs.csv
```

`BIN` holds `PROGRAM-VARIANT.exe`, `.mjs` (run by node) or `.classes` (run by java as the
differential runs it). The runner first checks every variant against the windows build's output,
then runs every (program, variant) once a round, in an order rotated each round, each a fresh
process timed alone.
