# The gauntlet on windows

The windows target prints LLVM IR ([docs/spec/llvm.md](../../docs/spec/llvm.md)), so its reference
is hand-written C compiled by the same clang at the same `-O2` the target's own build uses
(`emit/target.go`'s `build.bat`). One compiler on both sides measures the language's code
generation, not two compilers'.

- [gauntlet.c](gauntlet.c): the references and the harness. The references are the Go ones
  (`gauntlet/go`), written as a person writes C: a pointer and a length, `int` indices, stack
  arrays where a size is fixed, `malloc` where Go allocates, and `restrict` for the stencil, which C
  can say and the other three hosts cannot. A generated function takes the language's table, a
  pointer to an `i64` length and then the elements.
- `gen_*.ll`: the windows target's code for the gauntlet's programs, listed in
  [regen.txt](../regen.txt), written by `bash gauntlet/regen.sh` and compared by `cmd/check`'s
  gauntlet step like every other host's.
- [bench-build.sh](bench-build.sh): builds `bench.exe` into a directory, with the inputs. Go's
  `math/rand` draws them, so `gauntlet/go/wininputs` writes them with Go's own answers, and every
  case is checked against those answers, bit for bit, before anything is timed.

```bash
bash gauntlet/windows/bench-build.sh OUT
```

```bash
OUT/bench.exe OUT g7-tok-gen
```

Each case runs in its own process, as on the other hosts: a calibration to batches of at least
20 ms, then the median of 9 batches. The process prints its peak working set too. That is the
allocation criterion here, since a table the program does not release grows it with every call.
Every timed function is called through a volatile pointer and is not inlined into the timing loop
(the generated ones live in other objects), so a call costs the same on both sides and no work is
hoisted out of the loop.

**What windows does not run**, and why (gauntlet-2026-11-16): word count and generic's second
instantiation need a dictionary keyed by strings, which the language's `Map(Int, V)` is not and no
windows host provides; the report prints an F64, and F64's rendering is not built on windows. Both
are walls, named in the result.

The MASM files beside these (`sieve.asm`, `bigarith.asm`, `karatsuba.asm`) are earlier
references, each with its own `build-*.bat`.
