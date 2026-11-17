# A dead region crosses a call: 20000! allocates 22 times, not 20,000

2026-11-17. The second of the two fixes [alloc-2026-11-17](alloc-2026-11-17.md) measured, built at
hamza's word ("Build both fixes first"), by
[ADR 0125](../../docs/decisions/0125-a-dead-region-crosses-a-call-as-the-callees-token.md), after
[filltail](filltail-2026-11-17.md).

**A K(ℕ\*) instance now takes its caller's dead region for its result.** The factorial loop's steady
state is two regions in turn, as the algebra gives. 20000! makes 22 allocations on windows32, 20 on
x86_64, against 20,000. Its page faults fall from 34,567 (after filltail) to 1,233, and the
allocator's time to under a millisecond.

## 1. The algebra

ADR 0100's equation, by ADR 0107's classes:

  release(p, b) ; q ← region(b′) ≡ q ← p   when class(b′) = class(b).

R1d applies it across a loop's back edge with a linear token. Here the release is the caller's and the
allocation the callee's. **A release commutes with a call that does not read the released region.**
After W11 the region has no reference, so it is no argument of the call, and

  release(p) ; f(x) = f′(x, p),

where f′ takes p as the token for its result's region, and the equation holds inside f′. In the loop
`again(acc * i, i + 1)`, the region R1d carries into step i is accᵢ₋₂'s, dead since step i − 1, and
step i's call takes it. So two regions alternate, the in-place z = z·x of Go's `math/big` being
the case of one. This is destination-passing style (Shaikhha et al., FHPC 2017) and Perceus's reuse
token handed to a callee (Reinking et al., PLDI 2021).

## 2. What was built

- **A token variant `f~tok`** (`buildNatCallee`): f's parameters, then `#tok` (a pointer) and `#ktok`
  (its table's count). After storage and before ι, the allocation storing the table every return
  yields, outside any loop, becomes `reuse-fit(tok, ktok, n, w)` (`TakeResultToken`). An instance
  whose returns yield two tables has none. Today `mul-small` and `add` have one, and `mul`, `sqr` and
  `div` do not. Its call is impure.
- **The caller** (`Reuse`): a call of an instance with a variant is a request of the run's size, by
  fit, at its result's width, and R1d hands it the token as it would an allocation: the call becomes
  `f~tok(x, tok, k)`, and the other arms `discard`.
- **W12** (`CheckReuse`): in the caller, the call is the token's one consumer on its path, and the
  pair a fitting token at every source. In the variant, the token is named once, by the `reuse-fit`
  storing the result every return yields (`checkResultToken`).

## 3. The witnesses

- **`TestADeadRegionCrossesACall`** (ir): the factorial loop calls `nat.mul-small~tok`. Three planted
  faults:
  - `PlantNoTokenCall`: the plain instance is called again;
  - `PlantReuseKeepRelease`: a token both handed on and released, refused by W12;
  - `PlantFitCount`: a count not the token's, refused by W12.
- **`TestATokenVariantIsItsResultsStorage`** (ir), on every target K(ℕ\*) serves: `mul-small` and
  `add` have variants, and `div` has none. **The planted fault** (`PlantTokenAnyBuild`) takes the
  first build as the result's, and W12 refuses `div`'s variant: its token is not consumed on every
  return.
- **The counts**, as alloc §2 instrumented them, on 20000!'s product:

  | | before (filltail) | ADR 0125 |
  |---|---|---|
  | windows32 `HeapAlloc` / `HeapFree` | 20,000 / 20,000 | **22 / 22** |
  | windows32 page faults | 34,567 | **1,233** |
  | windows32 time in the allocator | 20.6 ms | 0.1 ms |
  | x86_64 `HeapAlloc` | 20,000 | **20** |

  alloc §3.2's emulation predicted 1,255 faults.

## 4. What failed first

1. **The first accepting check failed in two tests**:
   - P4's retry (ADR 0102, `Entry`) inlines the instance a failing call names. The call named
     `add~tok`, but the plan inlines by the base name `add`, so the retry changed nothing, and
     `TestACallOutsideItsContractIsInlined`'s program was refused. The instance a call names is now
     its base's (`CallSetError.instance`): a variant goes with its base.
   - `TestAnExportsNumeralIsANumeralsTable` pinned the text `ptr @nat.add(`, which fib's file no
     longer calls. It now asks for the variant. A test can pin a stale text (CLAUDE.md's lessons):
     this one was read before it was changed.
2. **The HEAD binaries were first built against the wrong tree.** The worktree sat inside the
   directory of the benchmark sources, and a source's own directory is the nearest target layer.
   So the step's own builds loaded the worktree's `.oro` files as fragments and were refused,
   everything defined twice. The worktree was moved beside the sources.

## Paid and bought

- **Run time**, HEAD (filltail) against this step, built in a worktree, nine interleaved rounds,
  medians, outputs identical:

  | program | triple | before | after | ratio |
  |---|---|---|---|---|
  | 20000!, product only | windows32 | 112.8 ms | 86.8 ms | **0.770** |
  | | x86_64 | 28.7 ms | 28.0 ms | 0.976 |
  | 20000!, computed and printed | windows32 | 197.2 ms | 165.3 ms | 0.839 |
  | | x86_64 | 42.5 ms | 41.0 ms | 0.964 |
  | fib(200000) mod 10⁹ + 7 | x86_64 | 109.0 ms | 89.9 ms | **0.825** |
  | | windows32 | 480.9 ms | 450.0 ms | 0.936 |
  | pair(24) | x86_64 / windows32 | 6.0 / 12.8 ms | 6.3 / 13.1 ms | 1.05 / 1.02, process start, inside the floor |

  With filltail, windows32's 20000! product runs at 0.67 of its time before both fixes (130.4 → about
  87 ms), alloc §3.2's 0.676. It is still 5.5× Go's on 386: the 8-byte digit (alloc §3.3, D5).
- **Emitted text**: three files on windows, `examples/big/fact`, `fib` and `pair`, each calling a
  variant. Every other target is byte-identical.
- **Compile time**: 1.02× the baseline in the accepting check (the median of 60 compiles over
  300 ms), with no suspect kept. The IR is 140,670 operations, from 140,045: each variant's body.
- **Proofs and refusals**: unchanged on the sweep.
- **Compiler lines**:
  - `ir/reuse.go` +230 −16 (the caller's request, the variant's storage, W12's two kinds, three
    plants);
  - `ir/progfunc.go` +26 (the variant, and a failing call's instance its base's), `ir/pipeline.go`
    +10, `ir/ir.go` +4;
  - tests: `ir/tokcall_test.go` 86, and `cmd/gen/numeral_test.go`'s text the variant's.
- **Documents**: ADR 0125; ir.md's W12.
- **Rules made less general**: none.

## What was not built

- **Variants for `mul`, `sqr` and `div`**: their returns yield more than one table (Karatsuba or
  schoolbook; of(0) on some paths). Making each return go through one build would let them take a
  token too. Measured first.
- **Local reuse across a call** (R1b's form, a release before a call in one region): the pass hands
  it, but no program in the corpus has the shape.
- **A program's own kept definition** (ADR 0113) taking a token: ADR 0125's consequence, waiting for a
  program that pays.
