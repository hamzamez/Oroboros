# IR step 3, second part: the Java backend is the IR's printer

2026-09-25. ADR 0032's third step, for Java. It follows
[irstep3-2026-09-25](irstep3-2026-09-25.md) (the shared plan and JavaScript). x86 remains on terms.

## 1. What was built

| file | lines | what it is |
|---|---:|---|
| `ir/java/java.go` | 754 | Java's spelling of Σ, on `ir/plan` |
| `ir/final.go` | +30 | the first slice of step 4: **final scalar ranges** in IR_P |
| `ir/types.go`, `ir/verify.go` | +20 | W5 by stage; the limb kernel narrowed |
| `ir/java/java_test.go`, `ir/testdata/java-shapes.oro` | | the measured shapes, run under the JVM |

`-printer` now defaults to `go,js,java`. `-printer terms` keeps every term backend for comparison
until step 4 retires them.

## 2. The algebra

**Java's scalar ρ is a choice among two representations of ℤ.** A value v whose type is
`(int LO HI)` is held as Java's `int` if [LO, HI] ⊆ [−2³¹, 2³¹−1], and as `long` otherwise
(spec §4.2). Before this step, IR_P carried a range only on a table's elements (Theorem D′). A scalar
was the word, and so a Java `long`, and every access needed `(int)`. Finalize now writes each
non-parameter integer's range from the IR's interval fixpoint. A parameter keeps its declared type,
because a signature is a boundary callers were compiled against.

**An operation is computed in the join of its representations.** Let R be the widest of ρ(operands)
and ρ(result). The printer computes in R, widening the first operand when the result is the wider,
and then narrows to ρ(result) with a cast.

*Why it is exact.* Every operand lies in its own type, so in R. The true result lies in its type, so
in ρ(result) ⊆ R. A Java operation on R whose inputs and true result lie in R computes ℤ's result.
The final cast is then the identity on the value. This holds for `div` and `rem` too, where the
cheaper argument, the ring homomorphism ℤ → ℤ/2³², does not: it covers `+ − ·` only.

`n·n` for n ∈ [0, 10⁵] prints `((long) n * n)`. Without the widening it is 1410065408 at n = 10⁵,
and `TestTheScalarRepresentationRuns` runs it under the JVM.

**W5 depends on the stage.** In IR_A a flow edge compares sorts. In IR_P it compares by containment
where both ends are values, since now a value's range can be narrower than a parameter's. Where one
end is *declared* (a call's argument, the function's result), it still compares sorts, because the
declaration is what the host compiled.

**The limb kernel is only finite non-negative ranges.** The unification modulo ρ_T's kernel had
identified every range above the word with `array int` whenever the target's big representation was
limbs. On the go/java limbs differential, a Java `BigInteger` value was typed `long[]`. Limbs hold a
magnitude (ADR 0029), so the identification holds only for a finite range with LO ≥ 0.

## 3. What the measurement found

**A wrong answer in the Java term backend.** The term backend narrowed each loop variable to Java's
`int` on its own (`NarrowByInterval`). A source that is *another loop variable* counted as fitting,
by induction, even when that variable had itself been refused. So in

```
(loop ((i 0) (x 0) (y 0))  …  (again (+ i 1) y (a i)))
```

y reads the table and stays `long`, while x is narrowed to `int` and assigned `(int) y`. With
a[0] = 5·10⁹ and n = 2 it answered **705032704**, which is 5·10⁹ mod 2³².

The induction is sound only over a set of variables that *all* narrow: the greatest fixpoint of "every
source is a literal, a bounded operation, or a member of the set". The term backend took one step of
it. The IR printer never asks the question, because each range is read off the interval fixpoint,
where x's range already contains the element's. The case is `gauntlet/differential/cases/narrow-from-wide.oro`:
it passes on four targets, and the term backend is its planted fault.

**Inlining a boolean was measured, and not adopted.** The first run had one disjoint gap: the early
search at 2.3 ns against the term backend's 2.1. javac does not optimise, so
`final boolean c = (i >= n); if (c)` is a materialised 0 or 1 and a second test. Declaration order
was refuted first (2.3 either way). Writing the conditions in their `if`, which is β for `let` (L1),
recovered 2.1. As a printer rule it did not hold up. An interleaved A/B on the JSON tokeniser,
7 rounds (median ns):

| variant | tokeniser | tree | `GenTokens` bytecode |
|---|---:|---:|---:|
| terms | 8,014 | 6,299 | 791 B |
| IR, nothing inlined | **7,639** | 5,784 | 1,366 B |
| IR, atomic booleans inlined, connectives bound | 8,416 | 5,878 | 1,042 B |
| IR, every single-use boolean inlined | 8,446 | 5,797 | 922 B |

The fastest tokeniser has the *largest* method, so this is not HotSpot's size thresholds. C2 profiles
each bytecode branch, and a materialised boolean gives it two sites to profile where the inlined form
gives one. That changes its layout, and the host compiler is a black box (ADR 0008). So the two
programs disagree by about 10% in opposite directions. Both effects are under the ~15% floor, and one
is a 2 ns function where call overhead dominates. So the printer keeps its plain form, and there is no
rule.

**An unobliged bound (not fixed here).** tables.md §2.3.1 lets Java declare `len a ≤ 2³¹−1`, and the
analyses use it as an axiom. But a `build`'s size is not obliged to meet it, and both printers emit
`new T[(int) n]`, so `(len (build b 4294967297 …))` is 1 on Java. It predates the IR, and it is queued
as its own task: the size must become an obligation (refinements.md §3a).

## 4. Gates

Differential suite: **all cases agree on four targets**, with Go, JavaScript and Java printed
through the IR (46 cases, `narrow-from-wide` included). The Java printer's tests run the shapes under
the JVM. With the widening removed they fail, and the JVM prints 1410065408.

The Java gauntlet: one JVM per case (`gauntlet/java/One.java`, which gained a case for every program),
5 interleaved rounds, the best of 9 timed batches after 2 s of warm-up, medians in ns:

| program | hand | terms | IR | IR / hand | **IR / terms** |
|---|---:|---:|---:|---:|---:|
| dot, n = 65,536 | 30,662 | 30,393 | 30,603 | 1.00 | **1.01** |
| dot, n = 1,024 | 445 | 449 | 448 | 1.01 | **1.00** |
| centroid | 30,315 | 30,500 | 30,611 | 1.01 | **1.00** |
| search, early | 2.0 | 2.0 | 2.3 | 1.15 | **1.15** |
| search, late | 16,933 | 17,021 | 17,120 | 1.01 | **1.01** |
| tally | 3.29 ms | 3.40 | 3.43 | 1.04 | **1.01** |
| generic tally | | 3.09 ms | 3.09 | | **1.00** |
| sum | 30,262 | 30,347 | 30,729 | 1.02 | **1.01** |
| stencil, allocating | 93,270 | 93,573 | 93,600 | 1.00 | **1.00** |
| stencil, build | | 93,795 | 94,084 | | **1.00** |
| stencil, reusing | 231,010 | 233,629 | 220,281 | 0.95 | **0.94** |
| json tree | 6,468 | 6,544 | 5,617 | 0.87 | **0.86** |
| json tokeniser | 7,454 | 8,192 | 7,807 | 1.05 | **0.95** |

The early search is §3's 2 ns function. The tree's 0.86 is real: disjoint spreads, and it reproduced
in both later A/B runs (0.91, 0.92).

Emission: every Java output was re-printed, and Go, JavaScript and x86 are byte-identical. No outcome
or proof count changed. Two refusals are reworded by the IR's untyped-parameter message. javac
compiles 58 of 61 emitted programs. The other 3 read `oroArgs`, which `cmd/build` supplies, and built
that way `wc`, `freq` and `jsonfmt` print identical output on the JVM under both printers. **freq's
Java compile fell from 44.0 s to 14.5 s** in the sweep (0.33×). That is a time among 16 parallel
compiles, not a serial one, and `-accept` recorded it as the new baseline.

The generated files were regenerated for both columns from the same sources, and the method names
normalised identically. The committed `gauntlet/java/gen/*.java` are stale against the compiler: the
names are now lower camel case, and GenJsonTok's source table is `byte[]`. That is check/README's
"owed" header, met again.

## 5. Not built

- The x86 printer: the rest of step 3.
- **The term analysis as a factor of scalar ranges.** In the JSON tokeniser, the counters `nt` and
  `mx` are `long` through the IR and `int` through terms. The term analysis bounds them relationally
  (nt ≤ i ≤ len src), and the IR's interval domain cannot. A reduced product with the term factor
  would recover them, as Theorem D′'s hull already does for tables. No measurement asks for it: the
  tokeniser is 0.95× the term backend. It belongs to step 4, when the analyses move to the IR.
- Deleting `emit/java.go`: it remains as `-printer terms`.
