# callbacks — a λ a host applies during the call, built on every target (ADR 0143), 2026-11-20

hamza, 2026-11-20: "now build everything agreed to in callbacks, you come up with program or
programs for them. always mathematical and algebraic."

Go first: stages A (a pure λ), B (a cell threaded under `ordered`) and D (the evidence check),
then C, windows' closure conversion into the context word and the slot, then E, JavaScript's
arrow function and the JVM's lambda.

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

- **`examples/callbacks/allwindows.oro`: ∀ over the windows is a semilattice homomorphism.**
  EnumWindows answers the conjunction of a pure λ over the windows W, a fold of (∧, 1), so
  ∀h. (p ∧ q) = (∀h. p) ∧ (∀h. q). Checked for every constant pair (a, b), captured from the loop's
  iteration through E in the context word: `0 0 0 11 4` on x86-64 and on i686. The book's §18.7.

- **`examples/callbacks/quantifiers.oro`, on JavaScript: ∀, ∃ and the least witness.** For each
  p_k(x) = x > k, k from 0 to 10, De Morgan (¬∀p ⇔ ∃¬p), the witness law (∃p ⇔ findIndex(p) ≥ 0)
  and findIndex as the least witness, against a loop of the program's own: 11 of 11.
- **`examples/callbacks/semilattice.oro`, on the JVM: ∀ over ∧ and ∃ over ∨.** allMatch is the
  meet of the p(x), anyMatch their join, and each distributes over its own operation: 100 of the
  100 pairs (k, m).

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
| closure conversion (windows) | E a record of the region's captures, an `alloca` in the entry block, its stores at the `fn`; the lifted function takes the λ's parameters and `ptr %ctx` and loads E at entry under the region's own names; the window procedure's `CallbackWrapper` at the claim's convention; `ctx(f)` E's address | the law on both triples; E read from the window's handle instead breaks it (`llvm.PlantCtxSwapped`) |
| the slot (windows) | for a claim with no `ctx`: E's address in a global, saved, written and restored around the call | `EnumSystemGeoID` runs on both triples; the slot written before the call, in the text, and the fault leaves it unwritten (`llvm.PlantSlotUnwritten`) |
| release | a λ's region is placed at the call that applies it, a loop's body for the rule | a captured table released after `EnumWindows` returns; the fault places the region at its `fn`, and the release falls before the call (`ir.PlantFnReleasedEarly`, `TestATableACallbackReadsLivesToItsCall`) |
| JavaScript | an arrow function bound to the λ's value; `every`, `some`, `findIndex` typed by a claim over a table (no marker: ECMAScript's order is not kept as evidence) | the two programs, run |
| the JVM | a lambda printed where its one use is, so the host's interface types it; each captured value copied to a final local first, and one copy per λ | a loop variable captured as it stands is refused by javac (`java.PlantJavaCaptureUnfinal`) |
| the book's checker | runs a JVM program, `java -cp DIR Main` | §17.6's two examples |
| declarations (Win32) | `winuser.EnumWindows` by its context word; `winnls.EnumSystemGeoID`, a new header, by its slot | exempt from the prototype check: the survey refuses a function pointer |
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

**On windows:**
- **The lifted function's parameter was the word where the region read an i32**, on i686 only: a
  parameter proven in S₃₂ is narrowed at a function's entry (ADR 0127), and the lifted function
  skipped it. clang refused the module.
- **The slot's run could not witness its fault.** EnumSystemGeoID answers 1 whether its callback
  stops it at the first nation or runs through all 250, and EnumSystemLocalesA,
  EnumSystemCodePagesA and EnumDateFormatsA the same (measured with C), so no answer shows a wrong
  E; and a load through null is undefined, which clang folded away rather than crash. The witness
  is the text: the slot written before the call. The restore after it has no planted fault, since
  nesting the same API needs an impure host call inside the λ, which Theorem D refuses.
- **A captured table could be freed before the call**: the release analysis placed the region at
  its `fn` statement. Freed memory keeps its bytes, so this too is witnessed by the order.
- **No Win32 host is `ordered`**, so a cell written in a λ is refused on LLVM, explicitly, as `fn`
  is on JavaScript and the JVM until they are built: a statement a printer skipped would be a wrong
  answer.

**The Win32 survey's model** counted a function pointer uncallable, and the target now calls two.
It now counts one callable where the callback's parameters are words, handles or pointers,
`callbackPassable`: 4,468 entry points callable (36.1%, from 35.2%), 3,832 callable and linkable
(31.0%, from 30.2%), 5,878 if every struct were declared (47.5%, from 46.3%). The 116 are an upper
bound: whether an API keeps its pointer past the call, which tier 1 excludes, is its
documentation's, and the header does not say.

**On JavaScript and the JVM:**
- **Two λs capturing one loop variable** declared its final copy twice in one scope, and javac
  refused it; the copy is now named per λ.
- **A loop over (k, m) pairs was not proven to end**: size-change termination does not see a
  lexicographic measure where m resets as k grows. One counter over the pairs, k = i / 10 and
  m = i mod 10, is the same enumeration with a measure it sees.
- **The book's checker could not run a JVM program**, so a JVM example would have been printed
  unrun; it runs them now.

## Paid and bought

| | |
|---|---|
| compile time | 0.95× the baseline on the corpus (`cmd/check`'s gate); no program compiles slower |
| run time | not measured: no program had a callback before; Go keeps a non-escaping closure on the stack (research §2.1), unmeasured here |
| code size | the five new programs only; every existing file byte-identical on every target |
| proofs | 11,859 of 11,913 operations and 1,367 of 1,660 loops, from 11,812 of 11,866 and 1,361 of 1,654: the five programs' 47 operations and six loops, all proven; windows32 6,685 of 6,695 and 741 of 820 |
| refusals changed | none in the corpus; Go's two programs refused on JavaScript, the JVM and windows, which declare neither `strings.Map` nor an `ordered` host; a written cell refused there by name |
| compiler lines | `core/cell.go` +190, `core/reduce.go` +40, `core/jreadtarget.go` +20, `emit/check.go` +40, `emit/target.go` +75, `emit/layout.go` +20, `ir/callback.go` +200, `ir/interval.go` +130, `ir/verify.go` +95, `ir/lower.go` +45, `ir/golang/golang.go` +65, `ir/plan`, `ir/print.go`, `ir/read.go`, `ir/typing.go`, `ir/final.go`, `ir/ir.go` a few each; then windows, JavaScript and the JVM: `ir/llvm/fn.go` +183, `ir/llvm/llvm.go` +41, `ir/java/java.go` +119, `ir/js/js.go` +31, `ir/release.go` +28, `ir/callback.go` +20, `core/jreadtarget.go` +15, `emit/target.go` +12, `gauntlet/stdlib/win32.go` +35; tests +330, then +175 |
| language lines | `targets/go/slices.oro` new, `targets/go/strings.oro` +30, `targets/win32/winnls.oro` new (24), `winuser.oro` +14, `targets/js/methods.oro` +16, `targets/java/util.oro` +8; five programs, 231 lines |
| documents | ADR 0143's "as built"; inventory.md four words; j.md §1.4; the book's §14.2, §17.6 (Go, JavaScript, the JVM) and §18.7, its checker running the JVM; research §2.1's figure, corrected earlier today |
| rule made less general | none |
| not built | a written cell on LLVM, JavaScript and the JVM, each refused explicitly, since no host there is `ordered` (the JVM's holder or slot, measured, waits on one); a buffer in the λ (T1-4, decided, waits on a program that writes one); the middle strength; a law a callback obeys |
