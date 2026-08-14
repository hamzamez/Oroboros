# The duplicated read costs nothing — and the clock lied about it

[concerns.md §1.1](../../docs/spec/concerns.md) claimed that β substituting unconditionally
leaves `(aindex a i)` computed twice per element in the filter residual, and called it
**"a silent 2× on the hot loop."**

That was an unmeasured prediction. It is now measured.

**It is wrong twice over.** The duplicated read costs nothing, because Go's common-subexpression
elimination removes it. And the benchmark that appeared to show a 1.45× penalty was measuring
**code alignment**, not the duplication — three functions with byte-identical machine code
measured 45% apart, stably, across every run.

---

## What was compared

The filter residual, emitted:

```go
func GenFilterSum(a []float64) float64 {
	acc := 0.0
	n1 := (len(a))
	for i := 0; i < n1; i++ {
		var t2 float64
		if ((a[i]) > 0.0) {
			t2 = (acc + (a[i]))      // a[i] read twice in source
		} else {
			t2 = acc
		}
		acc = t2
	}
	return acc
}
```

against two hand-written forms — `FilterSumRef`, which binds the element once (`x := a[i]`), and
`FilterSumDup`, which reads it twice as the generated code does.

## The clock

n=1024, L1-resident, medians, and **stable to under 2% across repeated runs**:

| | ns/op |
|---|---|
| `FilterSumRef` — binds once | 556 |
| **`GenFilterSum` — generated, two reads** | **551** |
| `FilterSumDup` — hand-written, two reads | **810** |

Read naively: the generated code is at parity with the *optimal* form, and the naive two-read
form is 1.45× slower. Tempting conclusion — our ANF shape somehow avoids the penalty.

**That conclusion is false.**

## The assembly

Disassembling all three benchmarks, normalising addresses:

```
diff FilterDup.norm GenFilter.norm
41c41
< JMP oroboros/gauntlet.BenchmarkSmallFilterDup(SB)
---
> JMP oroboros/gauntlet.BenchmarkSmallGenFilter(SB)
```

**One line differs, and it is each function's jump to its own name** in the stack-growth
epilogue. Against `FilterSumRef` the diff is the same single line.

All three are **byte-identical machine code**. The hot loop in every one of them:

```
MOVSD_XMM 0(CX)(BX*8), X1     ; ONE load
XORPS     X2, X2
UCOMISD   X2, X1
JBE       …
ADDSD     X1, X0
```

One `MOVSD`. **Go's CSE eliminated the duplicate read before it ever reached the machine.**

## So where did 1.45× come from

Loop addresses:

| | loop start | offset mod 64 | ns/op |
|---|---|---|---|
| `FilterSumRef` | `0x14013cd2e` | **46** | 556 |
| `GenFilterSum` | `0x14013ce6e` | **46** | 551 |
| `FilterSumDup` | `0x14013cdce` | **14** | 810 |

The two functions that share a cache-line offset share a runtime. The one that does not is 45%
slower. **Identical instructions, different alignment.**

This is the classic measurement-bias effect — Mytkowicz et al., *"Producing Wrong Data Without
Doing Anything Obviously Wrong!"* — and it is worth noting how convincing the wrong answer
looked: the spread was large, consistent to under 2% across runs, reproduced on a second run, and
had a plausible mechanism ready to explain it.

## What this changes

**1. The claim in concerns.md §1.1 is withdrawn.** The duplicated read costs nothing on Go. Call-by-need
is still worth having — for effects ([g5](../../docs/derivations/g5-bindings.md)), where
duplication is a *correctness* problem, and for hosts whose optimiser is weaker — but not for
this, and not on this evidence.

It is also [ADR 0008](../../docs/decisions/0008-measurement-over-principle.md) again: **do not
implement what the host already does.** [g2](../../docs/derivations/g2-structs.md) found Go
performs SROA itself; this finds Go performs CSE itself. Twice now, an optimisation the design
planned to own turned out to be the host's job.

**2. The noise floor is not 15%.** The project has been treating ~15% as the threshold below
which differences are not real. That is wrong in a way that matters: **alignment alone produced a
stable, reproducible 45% difference between identical code.** A large, consistent, repeatable
gap is not evidence of a real difference.

This retroactively weakens any conclusion that rested on a margin under roughly 50% between
*similarly shaped* code. Scanning what that touches:

| Result | Margin | Status |
|---|---|---|
| JS `Map` vs `Object` | 3.25× | safe |
| Java `merge` vs `getOrDefault` | 2.6× | safe |
| In-place stencil penalty | 4.6–6.2× | safe |
| Uniqueness false negative | 40–1540× | safe |
| Index vs pointer graph | 1.88× | probably safe |
| **Go `m[k]++` vs get-then-set** | **1.23×** | **now suspect** |
| **Yesterday's dot-product parity** | **≤5%** | **conclusion stands, reasoning does not** |

The parity conclusion survives because the measurement cannot *distinguish* at that resolution —
which is what "parity" means — but it was reported as though 5% were meaningful, and it is not.

**3. The rule that saved this: check the assembly before believing the clock.**
[CLAUDE.md](../../CLAUDE.md) already says to check the compiler's own decisions rather than the
timings. It has now paid off in the strongest possible way: the timings were consistent,
reproducible, and completely misleading, and only the disassembly showed it.

## Method for future parity measurements

1. **Diff the machine code first.** If two implementations compile to identical instructions,
   they are identical and the clock is measuring layout.
2. Only when the code genuinely differs does the timing mean anything.
3. Treat sub-50% differences between similarly shaped code as unresolved without an assembly
   diff or a layout-perturbation study.

## Reproducing

```bash
go run ./cmd/gen examples/filter.oro go gauntlet/go/generated_filter.go gen-filter-sum
```

```bash
cd gauntlet/go && go test -bench='SmallFilterRef|SmallFilterDup|SmallGenFilter' -benchtime=3s -count=5
```

```bash
cd gauntlet/go && go test -c -o b.exe . && go tool objdump -s 'BenchmarkSmallGenFilter$' b.exe
```
