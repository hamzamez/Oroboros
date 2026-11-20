# 0143 — A callback during the call is a region of the call; its markers are the host's promises

Date: 2026-11-20
Status: Accepted (hamza, 2026-11-18 and 2026-11-20, on [callbacks-sketches.md](../callbacks-sketches.md)
Part 2, derived in Part 3: T1-1 "L"; T1-3 "agree to option a"; T1-4 "agree"; T1-2 "agree to calls K
with K a range, ordered", then "sure, I agree" to the default and the evidence; T1-5 "agree to k").
Built on Go 2026-11-20, a pure λ and a cell threaded under `ordered`
([callbacks](../../gauntlet/results/callbacks-2026-11-20.md)); windows' closure conversion, the
evidence check, JavaScript and the JVM to follow.

## Context

A host function that takes a λ and applies it only during the call (`strings.IndexFunc`,
`filepath.WalkDir`, `EnumWindows`) is 105 of Go's 150 callback-taking functions and 291 of Win32's
349 function-pointer APIs ([callbacks-tier1-research.md](../callbacks-tier1-research.md) §2). Today
a λ reaching a host is refused as an escaping closure. ADR 0053 decided that a callback parameter's
set is a claim, the arrow, and left "the markers' vocabulary … and how a target file writes a
callback hole" to this decision.

## Decision

1. **Tier 1 is L**: a λ the host applies only during the call, on the caller's thread. It is a
   second-class value (Strachey 1967), never stored or returned, so its environment is the caller's
   frame. On the IR it is **a region of the call**, read by every analysis as a loop whose trip
   count is the host's (research §3.5). An escaping callback is not tier 1.
2. **A program's surface is unchanged.** `fn(…) { … }` is written at the call (j.md §2.7), its
   binders' sets taken from the claim. `again` and `try` do not cross the λ; `expect` and `abandon`
   do.
3. **A target file states two markers after the arrow, where `pure` goes**: `calls K`, with K a
   range (`1`, `0..1`, `0..`), the host's count as an element of the usage semiring 𝕌; and
   `ordered`, that the host's activations are a sequence its documentation fixes. A count of at
   most one implies `ordered`. **The default is the top**, `calls 0..` and unordered.
   - `fn WalkDir(root ∈ go.bytestring, f ∈ fn(p ∈ go.bytestring, d ∈ fs.DirEntry, e ∈ error) (error) ordered) (error) { host expr "filepath.WalkDir(${root}, ${f})" import "path/filepath" }`
4. **What the λ may do follows from the markers, by derivation:**
   - a pure body that consumes nothing relevant or linear, under any declaration (Theorem D);
   - a name used in the body is used k times its use there: 𝒰ₓ(h(ā, λȳ.e)) = Σᵢ 𝒰ₓ(aᵢ) +
     k · 𝒰ₓ(e). A relevant value needs k ≥ 1, a linear one k ≤ 1;
   - **an effect** (a cell written, a buffer written, a host's IO) **only under `ordered`.** The
     cell or buffer is threaded through the call as an extra result: the state monad's lift of a
     traversal (`mapAccumL`), Theorem T₁. Elsewhere it is refused, naming the missing promise. A
     buffer is lent for the call as a cell is, so R3's "no closure over a buffer" holds only for
     escaping callbacks.
5. **A marker is a promise of the host's documentation, not a reading of its implementation.** Its
   evidence, the documentation's sentence, is kept with the tooling, in
   `gauntlet/stdlib/callbacks-go-markers.txt`, not in the declaration. The tooling test checks it
   both ways: every marker in `targets/` has a row whose sentence is in the host's document, and
   every row marked ordered is declared so.
6. **On C, the λ is closure-converted into the API's context word** (Minamide, Morrisett and
   Harper 1996). E, the captured values and the addresses of threaded cells and buffers, is an
   `alloca` in the caller's frame. Its address travels in the API's context word where it has one.
   The callback's C signature names where E arrives, `repr WNDENUMPROC c(h, ctx) stdcall`, and the
   API's call names the argument that carries it, `call EnumWindows(f, ctx(f))`. Where the API has
   none (33 of 349), E is found through a slot, saved, written and restored around the call
   (shallow binding, Baker 1978). On Go, JavaScript and the JVM the λ is the host's own closure.

## Why not

- **E, every callback a heap closure** (T1-1): it copies the captures, lends nothing, writes no cell,
  and allocates where Go's own escape analysis keeps an L closure on the stack (research §2.1).
- **(c) no effect in any λ** (T1-3): a sound subset of 4, and no simpler for the targets. It makes
  the visitors (`WalkDir`, `ast.Inspect`, `flag.Visit`) useless, since their purpose is the effect.
  A cell written in a loop body, a λ applied by a primitive in order, is already accepted. (a) is
  the same translation given to a host that promises an order.
- **(b) commuting writes under no order** (T1-3), as first stated: a commutative monoid's action is
  independent of the order and not of the count. Under no order the count is the host's too: Go
  1.9's `FieldsFunc` called f twice per rune, Go 1.27's once. It is sound only where a host
  documents each element once (`sync.Map.Range`, `expvar.Do`, `EnumWindows`'s "each window, in
  turn"): the middle strength, which waits on its word and a program.
- **A marker read from the implementation**: the survey's first reading marked 64 of 89 functions
  ordered, and 12 document it. `strings.Map` calls f once per rune in order today and promises
  neither, so a declaration from its source would tie the program's meaning to one Go release.
- **The sentence in the declaration, `ordered "…"`**: the compiler never reads it, and a token the
  semantics ignores is a misfit (ADR 0050). **In a comment**: ADR 0024, a comment carries no
  meaning. **No evidence at all**: the claim's measured error rate, 52 of 64, would stay
  undetectable.
- **No default, or a permissive default** (T1-2): the first means the same thing and refuses a
  silent declaration. The second claims an order the host never promised, which is unsound.
- **Kotlin's `callsInPlace` kinds, or `× K in order`**: four points of 𝕌 and no order, or the same
  algebra in other letters.
- **The slot always** (T1-5): it is dynamic scope emulating lexical scope, correct only because the
  activations nest on one thread. It reads the wrong thread's slot for a callback the host runs on
  another thread during the call, and it needs thread-local storage under ADR 0081. The context
  word is C's own closure, an argument, correct by construction.
- **A thunk generated at run time**: executable memory written per call, and an allocation.
- **`&f` for the context, or a determined header parameter**: `&x` means a fresh copy's address
  (ADR 0130, 0131), and a header parameter puts representation in the meaning's place (ADR 0130).
- **A template hole with no IR construct** (research §6 A): a body outside the IR is unseen by the
  decision, termination, release and usage, and every rule of 4 would go unchecked.

## Consequences

- One construct, the region, serves Go, JavaScript, the JVM and windows. Printed as the host's λ on
  three, and by closure conversion on LLVM, sharing the window procedure's lifted function with its
  C convention (ADR 0133).
- The λ's body is a loop to the analyses. A counter in it is proven inside the word only by a
  guard or a bound, as in any loop of unknown trip count.
- **Owed**, each named in callbacks-sketches §3.8 and §3.12:
  - the JVM's written cell, a static slot or a one-element holder, both measured, the holder
    expected to lose;
  - Win32's pages kept as data for the evidence check, fetched with permission, when the first
    Win32 marker is written;
  - ten of Win32's 291 enumerators read against their pages before any is marked;
  - the middle strength's word, at a program;
  - our own higher-order definitions, which still refuse the λ a host's `ordered` admits.
- **Planted faults** (research §5): the claim's domain and result; Theorem D's purity check; 𝒰's
  clause; Theorem T₁'s dropped result, witnessed on `slices.EqualFunc`; a captured table released
  before the call (windows only); the slot not restored; E in the wrong argument; the JVM's
  capture; `again` across the region; a marker without evidence.
- The book gains the construct in chapter 17 (a declaration's markers) and chapter 14 (a cell in a
  λ given to an ordered host) when it is built.

## As built (2026-11-20)

- **The region is an `fn` statement whose one use is the call** (`(val %f fn (callback Q) REGION)`),
  its value the call's argument at the callback's position. So a call's arguments stay its
  declaration's parameters, one for one, which nine places in six passes and every printer's
  template assume. W16 keeps it second-class: an `fn` value is used exactly once, as a call's
  argument at a callback parameter.
- **A written cell is a reference to a fresh copy in the caller's frame**, ADR 0048's form with the
  λ the writer: `r = #ref(x); v = h(…, λ′, …); x := #deref(r); v`. λ′ loads the cell into one of its
  own at entry and stores it back at exit, `#store-ref(r, x)`, Theorem T₁'s λ_S : S × A → S × R, so
  within one activation a write under a guard is a φ the guard bounds. ADR 0133's static slot was
  rejected for Go, where an export may be called from several goroutines at once. The interval
  analysis reads such a reference, one no host call receives, as the join of its initial value and
  every store, a fixed point over the host's unknown count (research §3.5).
- **The target hands the loader `(callbacks i o …)`** per host operation: each callback parameter's
  index, and 1 where its host promises the order. The state translation threads a written cell only
  under it, and refuses one elsewhere naming the cell (Theorem D). A cell the λ only reads is
  captured as its value at the call.
