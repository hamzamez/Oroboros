# callbacks — a λ a host applies during the call, built on Go (ADR 0143), 2026-11-20

hamza, 2026-11-20: "now build everything agreed to in callbacks, you come up with program or
programs for them. always mathematical and algebraic."

This is the first step: Go, stages A (a pure λ), B (a cell threaded under `ordered`) and D (the
evidence check). Windows' closure conversion (C) and JavaScript and the JVM (E) are the next
steps, each its own result.

## The programs

- **`examples/callbacks/caesar.oro`: ℤ/26 acting on text.** σ_k is a rotation of each
  alphabet, k ↦ σ_k a homomorphism ℤ/26 → Sym(Σ), and `strings.Map` lifts f : Σ → Σ to the monoid
  homomorphism Σ* → Σ*, functorially: Map(f) ∘ Map(g) = Map(f ∘ g). The program encrypts by a key it
  computes (6, the a's in the text), decrypts by 26 − k, applies ROT13 twice, and finds the comma,
  which every σ fixes, at the same index in both:
  `Gzzgiq gz jgct, ygoj Igkygx.` / `Attack at dawn, said Caesar.` / the same / 14 / 14.
- **`examples/callbacks/prefix.oro`: the longest common prefix, counted by the host's own
  comparisons.** `slices.EqualFunc` documents its calls, eq(a[0], b[0]), … up to the first false,
  so their number is min(lcp + 1, n). The λ prints each pair and counts in a cell, and the count
  (6) equals lcp + 1 by the program's own loop (lcp = 5).
- **The book's §17.6**: ROT13 twice, a print refused under `IndexFunc`, a count refused under
  `Map`, and the comparisons counted under `EqualFunc`.

## What was built

| piece | what it is | its rule and its planted fault |
|---|---|---|
| the claim's markers | `set Q = fn(…) (r ∈ R) calls K ordered`, read after the result | the default ⊤, `calls 0..` unordered |
| the checker | a λ at a callback parameter typed under its claim | ADR 0053 item 4 |
| `fn` | the λ as a statement of the IR, `(val %f fn (callback Q) REGION)`, its value the call's argument | W16, second-class: used once, by the call (`PlantSecondClassUnchecked`) |
| the domain | the region's parameters the claim's sets, assumed | `r * r * r` on an int32 refused; the fault leaves them unvisited, `[0, 0]` (`PlantCallbackParamUndeclared`) |
| the result | each yield proven in the claim's result | `r + 1` refused under `RuneMap` (`PlantCallbackResultUnchecked`) |
| Theorem D | an effect in the λ only under `ordered` or a count ≤ 1 | a print under `IndexFunc` refused (`PlantCallbackEffectUnchecked`) |
| 𝒰's clause | a use in the λ counts only where k ≥ 1 | a relevant value used only in the λ refused (`core.PlantCallbackUseCounted`) |
| the cell | read only: its value at the call; written, under `ordered`: `r = #ref(x); v = h(…, λ′, …); x := #deref(r)`, λ′ loading and storing r at its boundary (Theorem T₁'s λ_S) | a count under `Map` refused, naming the cell (`core.PlantCallbackCellOrderUnchecked`); the translation off, `prefix` refused (`core.PlantCallbackCellRefused`) |
| the join | a reference the program alone writes reads as the join of its initial value and its stores, a fixed point over the host's count | `k · 2⁶²` after the call refused, k ∈ [0, 10⁶]; the fault drops the stores, k reads 0 (`ir.PlantCellReadUnjoined`) |
| `cell-set` | a write through a reference, `(do (cell-set %r %v))`, an effect to the plan | — |
| the Go printer | `v14 := func(v15 int, v16 int) bool { … }`, a narrow parameter converted on entry, a narrow result at each return, an unread one `_` | — |
| the evidence | the markers file's fifth column, the sentence of `go doc` stating the order; checked both ways | a sentence Go's documentation does not contain refused (`TestEveryOrderedCallbackIsItsHostsSentence`) |
| declarations | `strings`: IndexFunc, LastIndexFunc, ContainsFunc, Map, TrimFunc; `slices.EqualFunc`, `ordered` | Map's result ours, witnessed by an adversary over the mappings (`TestStringsResultsDeclaredOursLandInUTF8`) |

## What failed first

- **The region's parameter was typed `(int 0 0)`.** Nothing visited the `fn` region, so its
  parameter kept the zero fact, which reads as [0, 0], and Finalize typed it so: a proof from no
  premise. It became the domain rule's planted fault.
- **A region beside the call's arguments shifted every position.** Nine places in six passes, and
  every printer's template, pair argument j with parameter j. The λ became a statement of its own,
  its value the argument, which ADR 0143's "as built" records.
- **The guard did not bound the write.** Each read of the cell was its own `#deref`, so the guard
  `k < 10⁶` and the `k + 1` after it read separately. λ′ now loads the cell once at entry and
  stores it once at exit, the state monad's λ_S, and the guard is a φ again.
- **The read after the call was all of ℤ**, the element set of `int`. It is now the join of what
  was stored, which a reference no host receives makes exact.
- **The Go printer dropped the store.** The plan's three hand-written effect lists did not know
  `cell-set`, so the λ's count printed 0 before it was added. A wrong answer, caught by running
  the program.
- **A static slot, ADR 0133's, was rejected for Go**: an export may be called from several
  goroutines, and a slot shared by every call would be a race. The reference lives in the frame.
- **A refusal printed `#_ = io.print_int(r)`**, which I took for a defect and "fixed" in `jshow`.
  The book's check refused the fix: chapters 1, 4 and 5 teach a statement as a binding nobody
  reads, printed `#_`, so the display is the convention. The change was reverted.
- **An unread parameter's conversion was refused by Go** as an unused variable; it is now `_`.

## Paid and bought

| | |
|---|---|
| compile time | 0.95× the baseline on the corpus (`cmd/check`'s gate); no program compiles slower |
| run time | not measured: no program had a callback before; Go keeps a non-escaping closure on the stack (research §2.1), unmeasured here |
| code size | the two new programs only; every existing file byte-identical |
| proofs | 11,840 of 11,894 operations and 1,362 of 1,655 loops, from 11,812 of 11,866 and 1,361 of 1,654: the new programs' 28 operations and one loop, all proven |
| refusals changed | none in the corpus; the two programs refused on JavaScript, the JVM and windows, which declare no such host |
| compiler lines | `core/cell.go` +190, `core/reduce.go` +40, `core/jreadtarget.go` +20, `emit/check.go` +40, `emit/target.go` +75, `emit/layout.go` +20, `ir/callback.go` +200, `ir/interval.go` +130, `ir/verify.go` +95, `ir/lower.go` +45, `ir/golang/golang.go` +65, `ir/plan`, `ir/print.go`, `ir/read.go`, `ir/typing.go`, `ir/final.go`, `ir/ir.go` a few each; tests +330 |
| language lines | `targets/go/slices.oro` new, `targets/go/strings.oro` +30; two programs, 110 lines |
| documents | ADR 0143's "as built"; inventory.md four words; j.md §1.4; the book's §17.6 (8 blocks); research §2.1's figure, corrected earlier today |
| rule made less general | none |
| not built | windows (C), JavaScript and the JVM (E); a buffer in the λ (T1-4, decided, waits on a program that writes one); the middle strength; a law a callback obeys |
