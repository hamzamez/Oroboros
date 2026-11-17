# The suite on i686-pc-windows-msvc

The measurement behind [docs/triple-research.md](../../docs/triple-research.md): the differential
suite run on 32-bit Windows before anything is built for it (assessment-2026-11-16 §5 item 4).
Nothing here enters the language or the targets.

```bash
python -I experiments/i686/mk32.py OUT
```

```bash
cd gauntlet/differential && I686_TARGETS=OUT/targets go run ../../experiments/i686/run32.go -j 4
```

- `mk32.py OUT` copies `targets/` to `OUT/targets` and makes the experiment's changes, each named in
  its docstring: the triple, ILP32's sets for the four kernel32 functions whose `SIZE_T` the 64-bit
  declarations state as the word, the address space's bound, a trap for a byte count past 32 bits,
  and a build line through `vcvars32` with `--target=i686-pc-windows-msvc`.
- `run32.go` is `gauntlet/differential/run.go` with windows the only target and every build reading
  `$I686_TARGETS`. Each case's `// expect:` line is the answer; a 32-bit program must print it.

Since ADR 0122 step 2 the triple is a target, and the suite runs on it through the real
declarations: `I686_TARGET=windows32 I686_TARGETS=<repo>/targets go run ../../experiments/i686/run32.go -j 4`
(66 of 74, windows32-2026-11-17).

It needs MSVC 2022's `vcvars32.bat`, the Windows SDK's x86 `kernel32.lib`, and the LLVM release's
clang.
