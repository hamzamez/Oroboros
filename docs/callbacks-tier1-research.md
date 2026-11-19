# Research: callbacks tier 1, a λ the host calls during the call

2026-11-18. hamza: "shouldn't we implement tier 1 before tier 2? does tier 1 need a research as
well?", then "write the tier 1 research now". **Research, no decision; nothing built.** It answers
what [ADR 0053](decisions/0053-the-arrow-is-a-claim-on-a-host-parameter.md) left "decided when
callbacks tier 1 is built", the markers' vocabulary and the callback hole, and the three questions no
document asked. The measurements in §2 were made on this machine: Go 1.27's API manifest and its
escape analysis, and the Windows SDK 10.0.26100 headers through `gauntlet/stdlib/win32.go`.

**The short version:**
- **Tier 1 is the language's own `loop`, given to a host.** A `loop`'s body is a λ in a structural
  position that the primitive applies and nothing holds (effects.md §3, `pureTerm`'s comment). A
  host's callback parameter is the same position with the host's body opaque. So the λ is a
  **second-class value** (Strachey 1967; Osvald et al. 2016): it may be applied, never stored or
  returned, and that is all tier 1 needs.
- **Measured (§2.1): 105 of Go's 150 callback-taking functions call the callback only during the
  call, on the caller's thread.** 12 call it many times in an order they specify, 77 many times in
  no stated order, and 16 once or at most once (*corrected 2026-11-20*: first counted 64 and 25, the
  52 others stating a result, not the order of the calls; callbacks-derivation-2026-11-20). 41 keep it beyond the call, 3 run it on another goroutine,
  and one uses it as a key without calling it. Go's escape analysis agrees: every callback of the
  first kind measured stays on the stack, including one that writes a captured variable, and
  `time.AfterFunc`'s goes to the heap.
- **Measured (§2.2): 291 of the 349 Win32 functions taking a function pointer both pass a context
  word and hand it to the callback.** That is the ∃E pair of typed closure conversion, so tier 1 on
  windows is closure conversion into the host's own pair, with E in the caller's frame. 33 pass
  neither, so their environment needs a slot.
- **The markers are a usage and an order (§3.3).** The count is an element of abstract.md §8.1's
  usage semiring 𝕌, the interval [m, n] the host promises, and a captured name's usage is
  count · its usage in the body: one line added to 𝒰. The order is whether the host's activations
  are a sequence its arguments determine. **Theorem D**: under no stated order, the call's answer
  is the host's only if the body is pure, which is the rule Go's own `FieldsFunc` documents.
- **What a λ captures (§3.4)** is decided per kind of binder. A value is free. A cell written in the
  body is threaded through the call as an extra result, the state-passing translation. Under no
  stated order that is refused (Theorem D). A buffer can be lent like a cell, and the old refusal of
  a buffer (callbacks.md §4) applies only where the λ escapes. `again` and `try` cannot cross the λ.
- **On the IR (§3.5)** the λ is a region of the call, as a loop's body is a region of the loop.
  Every analysis reads it as **a loop whose trip count is the host's count**: once, at most once, or
  any number of times. Intervals, size-change termination and the usage check reuse the loop's
  rules.
- **Recommended: B (§6)**, the region on the call, printed as the host's λ on Go, JavaScript and
  the JVM, and as closure conversion on LLVM. The program's surface is unchanged, since J already
  writes `fn(r) { … }` at a call (j.md §2.7). A target file gains markers after the arrow, as `pure`
  already follows it. Six decisions for hamza are in §7.

## 0. What was decided before, and what was not

| document | what it says | status |
|---|---|---|
| [callbacks.md](spec/callbacks.md) §2 | tier 1: "the lambda is written at the call site, and the HOST closes over it"; a structural position the backends read positionally; "a callback hole in the template language"; on x86, "emit a separate procedure and pass its address, with captures made explicit". §4: a callback body must not capture a buffer; a host closure allocates, which is the host's idiom | specified, not built |
| [ADR 0053](decisions/0053-the-arrow-is-a-claim-on-a-host-parameter.md), arrow-research §4.4, §5.3 | a callback parameter's set is a claim, a λ's header without its body; fitting is P ⊆ P_f ∧ R_f ⊆ R; a λ at the call site is checked *under* the claim; the markers (call count, escape, no captured buffer) go after the heading; **"Not decided here, and decided when callbacks tier 1 is built: the markers' vocabulary (call count, escape), and how a target file writes a callback hole"** | decided, not built |
| [j.md](spec/j.md) §2.4, §2.7 | the arrow `cmp ∈ fn(a ∈ elems(x), b ∈ elems(x)) (Int) pure`; "a λ `fn(a, b) { e }` written at a call takes its claim from the parameter it is passed to" (ADR 0066 item 5) | the spelling decided; a λ reaching the emitter is refused |
| [abstract.md](spec/abstract.md) §8.1 | usage is an interval in 𝕌, a semiring, `·` "a receiver's usage scales an argument's" | built on trees |
| [callbacks-research.md](callbacks-research.md) | Theorem N (a callback invocation nests in a host call), Theorem T (a state threaded through a dispatching call), the slot as evidence; tier 1 "not here" | research, five decisions with hamza |

## 1. The issue

`strings.IndexFunc(s, fn(r) { r == c })`, `sort.Slice(a, fn(i, j) { key(a[i]) < key(a[j]) })`,
`filepath.WalkDir(root, fn(p, d, err) { … })`, `EnumWindows(fn(h) { … })`. The λ captures `c`, `a`,
a count or a buffer from the scope it is written in. The host calls it some number of times, in some
order, and returns. Nothing holds the λ afterwards.

Today the reducer keeps a λ opaque when it is an argument to a β-redex, and transparent when it is an
argument to a structural primitive (`loop`, `cond`, `let`), which applies it. A λ given to a *host*
primitive is neither: it reaches the emitter and is refused as "an escaping closure"
(`emit/golang_ir_test.go`'s `TestEscapingClosureIsARefusal`). A target file has no way to declare a
callback parameter (`targets/go/strings.oro`: "TrimFunc, IndexFunc, Map, …, which are callbacks,
tiers 1 and 2, not built").

## 2. Measured

### 2.1 What Go's hosts do with a callback

`go run gauntlet/stdlib/callbacks.go -list take` gives the 150 functions and methods (arrow-research
§3). Each one is classified by reading its `go doc`, in `gauntlet/stdlib/callbacks-go-markers.txt`:
- **escape**: L, called only during the call on the caller's goroutine; E, kept and called after the
  call returns or on a goroutine of the host's own; C, called on another goroutine while the call
  blocks; K, used without being called;
- **count**: 1, 0..1 or 0.. (any number);
- **order**: S, the activations a sequence the host's documentation fixes (index order, lexical walk
  order, `Visit`'s sorted order); U, no stated order.

| escape | count | order | functions | examples |
|---|---|---|---:|---|
| L | 0.. | S | **12** | `filepath.WalkDir`, `Walk`, `fs.WalkDir`, `ast.Inspect`, `ast.PreorderStack`, `flag.Visit` (×4), `ring.Do`, `token.FileSet.Iterate`, `slices.EqualFunc` |
| L | 0.. | U | **77** | `sort.Slice`, `slices.SortFunc`, `sort.Search`, `strings.FieldsFunc`, `maps.DeleteFunc`, `sync.Map.Range`, `rand.Shuffle`, `hkdf.Key`; and, stating a result and not the calls' order, `strings.IndexFunc`, `Map`, `TrimFunc`, `slices.IndexFunc`, `regexp.ReplaceAllFunc` |
| L | 1 | | 14 | `slices.Collect`, `maps.Insert` (an iterator, called once), `pprof.Do`, `trace.WithRegion`, `testing.B.Run` |
| L | 0..1 | | 2 | `sync.Once.Do`, `os.Process.WithHandle` |
| E | | | 41 | `time.AfterFunc`, `http.HandleFunc`, `flag.Func`, `bufio.Scanner.Split`, `sync.OnceFunc`, `WaitGroup.Go`, `runtime.AddCleanup`, `iter.Pull` |
| C | | | 3 | `testing.T.Run`, `synctest.Test` (blocking), `B.RunParallel` (concurrent) |
| K | | | 1 | `json/v2.GetOption`, whose setter is an option's identity |

**105 of 150 are L (70%).** Every L function's count is one of three intervals, 1, 0..1 or 0..,
and none needs "at least once". Of the 89 called any number of times, 12 state their order.

*Corrected 2026-11-20* (callbacks-derivation-2026-11-20). The first count, 64 with a stated order,
read a promise about the **result** as one about the **calls**. `IndexFunc` returns "the index of
the first Unicode code point satisfying f(c)", which fixes the answer and not which code points f
sees, or in what order. `Map` returns "a copy of the string s with all its characters modified",
which fixes neither. Only 12 documents state the sequence of calls: "walked in lexical order",
"traverses an AST in depth-first order: it starts by calling f(node)", "in lexicographical order,
calling fn for each", "compared in increasing index order". The difference is observable.
`FieldsFunc`, which disclaims the order, made two passes over s in Go 1.9, a counting loop and a
creating loop, calling f twice per rune. Go 1.27 calls it once. `Map` calls it once per rune in
order today, and nothing promises it will.

Two of Go's own documents state this research's rules:
- `strings.FieldsFunc`: "makes no guarantees about the order in which it calls f(c) and assumes that
  f always returns the same value for a given c". That is Theorem D (§3.3), U with a pure body.
- `testing.T.Run`: "runs f in a separate goroutine and blocks until f returns". That is C, which is
  L for every observation but the goroutine's identity.

**Go's escape analysis** (`go build -gcflags=-m`, a program of my own with each kind):

| call | the closure | what it captures |
|---|---|---|
| `strings.Map(func(r rune) rune { n++; return r }, s)` | does not escape | `n`, written |
| `sort.Slice(a, func(i, j int) bool { return a[i]%k < a[j]%k })` | does not escape | `a`, `k` |
| `slices.SortFunc(a, func(x, y int) int { return x%k - y%k })` | does not escape | `k` |
| `time.AfterFunc(time.Millisecond, func() { fmt.Println(k) })` | **escapes to heap** | `k` |

So an L callback on Go is stack-allocated, a written capture included: callbacks.md §4's "a host
closure allocates" is true of E and not of L.

### 2.2 What Win32 does: the context word

`go run gauntlet/stdlib/win32.go -callbacks` (added for this research; the survey's report is
unchanged, diffed against HEAD's). For every entry point with a function-pointer parameter, it asks
whether the API takes a context word and whether the callback's own typedef receives one. A context
word is a pointer to anything or a pointer-width integer: `LPARAM`, `LPVOID`, `PVOID`, `void*`,
`DWORD_PTR`, `ULONG_PTR`, `UINT_PTR`, `LONG_PTR`, `INT_PTR`.

| | entry points |
|---|---:|
| take a function pointer | 349 |
| **pass a context word and the callback receives one** | **291** |
| pass one, the callback's typedef has none | 13 |
| the callback has one, the API passes none (`DdeInitializeA`, `FlsAlloc`) | 12 |
| **neither** | **33** |

Ten members of the 291 read by hand (`DialogBoxParamA`, `DSA_Sort`, `WlanRegisterNotification`,
`SwDeviceCreate`, `MsiSetExternalUIRecord`, `TreeResetNamedSecurityInfoA`, `WriteEncryptedFileRaw`,
`BluetoothSdpEnumAttributes`, `FwpmNetEventSubscribe2`, `ShowFindFriendsUIForUser`): each is
the environment passed through. The figure is by type, so it over-counts where an `LPVOID` is a
buffer, and it under-counts the `OVERLAPPED` idiom: `ReadFileEx`'s completion routine finds its
environment through the `OVERLAPPED*` it receives, which this count does not treat as a context.
The 33 with neither are the old enumerators (`EnumSystemLocalesA`, `EnumDateFormatsA`,
`EnumCalendarInfoA`, each with an `Ex2` or `Ex` successor that takes an `lParam`), the process-wide
handlers (`SetConsoleCtrlHandler`, `AddVectoredExceptionHandler`), kernel streaming, and a few
others (`RmShutdown`, `CryptCATCDFOpen`, `mciSetYieldProc`; `IsBadCodePtr` takes a code address and
calls nothing, a false member). The C
runtime's `qsort` is the same case, and `qsort_s` its fix.

A callback reached through a struct's field (`WNDCLASSA.lpfnWndProc`) is not a parameter, and is
not counted. It is the window procedure's case, an installed handler (callbacks-research).

## 3. The algebra

### 3.1 A callback during the call is a second-class value

**First- and second-class** (Strachey, "Fundamental concepts in programming languages", 1967): a
second-class value may be passed as an argument but not returned, stored or bound. Algol 60's
procedure parameters are second-class, and that is what made its environments a stack. The
**downward funarg** (Moses, "The function of FUNCTION in LISP", 1970) is a function passed down to a
call that finishes before its creator's frame does, so its environment can be the frame itself. The
upward funarg is the one that outlives it and needs a heap. Osvald, Essertel, Wu, González Alayón
and Rompf ("Gentrification gone too far? Affordable 2nd-class values for fun and (co-)effect",
OOPSLA 2016) type exactly this: a second-class function may capture second-class and stack values,
and the type system guarantees it never escapes. Swift made it the default (SE-0103, 2016:
non-escaping closures unless marked `@escaping`). Kotlin contracts state the count
(`callsInPlace(f, InvocationKind.EXACTLY_ONCE)` and its `AT_MOST_ONCE`, `AT_LEAST_ONCE`,
`UNKNOWN`). Rust's `FnOnce`, `FnMut` and `Fn` state what a closure does to its captures (RFC 114).

**L is the downward funarg.** An L callback is applied only inside the call, so its activations nest
inside the call, which is Theorem N's conclusion, here by the host's specification rather than by
measurement. Its environment is the caller's frame. Nothing about it is a value in V, so tier 3
stays refused by construction (ADR 0053 item 6).

**And the language already has one.** `loop x = z { … }` is a primitive applied to a λ, its body,
which the primitive applies a number of times and nothing holds. effects.md §3 reads it so: the
λ is transparent as an argument to a primitive "because the structural primitives … apply the λ
they are given". Tier 1 is that position given to a host, with two differences: the host's body is
opaque, and the host fixes the count and order rather than the program's guards.

### 3.2 The λ under the claim

ADR 0053 item 4 decided the check: the λ at the call site is checked *under* the parameter's
claim c = (P, R). Its body assumes x̄ ∈ P, which is the host's promise and is assumed as an export's
parameters are (ADR 0028). It must establish r ∈ R, which is our obligation, decided as a declared
result is (ADR 0072). A top-level function passed by name, `strings.IndexFunc(s, is_space)`, is its
η-expansion `fn(r) { is_space(r) }`, fitted by item 4's two inclusions. So **for an L parameter,
tier 2 is tier 1 with an empty environment**. Tier 2 is a separate case only where the callback
escapes, the window procedure's case.

### 3.3 The markers: a usage and an order

**The count is a usage.** abstract.md §8.1's 𝕌 is the interval semiring over ℕ ∪ {∞}, with
[a, b] · [c, d] = [ac, bd] "a receiver's usage scales an argument's". It is quantitative type
theory's grade (McBride 2016; Atkey 2018), and the bounded linear types of Ghica and Smith (ESOP
2014) and Granule's graded modalities (Orchard, Liepelt and Eades, ICFP 2019) are the same algebra.
A host promises the count k ∈ 𝕌 with which it applies the λ, so a name x the λ's body uses scales:

```
𝒰ₓ(app(h; ā, λȳ. e))  =  Σᵢ 𝒰ₓ(aᵢ)  +  k_h · 𝒰ₓ(e)
```

That is one clause of 𝒰. It decides what the λ may consume:
- a **relevant** value (ADR 0043, used on every path) consumed in the body is used on every path
  iff k_h's lower end is at least 1, so only `1` admits it;
- a **linear** buffer used once in the body is used at most once iff k_h's upper end is at most 1;
- under `0..` nothing relevant or linear is consumed in the body.

The measured counts are 1, 0..1 and 0.. (§2.1), three elements of 𝕌. Kotlin's fourth,
`AT_LEAST_ONCE`, is [1, ∞], which no measured function needs and the algebra admits unchanged.

**The order is a commutation question.** An activation of the λ is a computation whose effects are
its body's: a host call, a cell's write. Under S the host's documentation fixes the sequence of
activations as a function of its arguments, so the effects' history is determined. Under U it is
the host's algorithm's, which may differ between Go releases and between targets.

**Theorem D (determinacy under no stated order).** Let a host call apply a λ whose body is pure,
a function of its parameters and its captured values, in any order and any number of times within
its count. Then the call's result is a function of its arguments and that function, whatever the
order.

*Proof.* A pure computation commutes with every computation and may be copied and discarded
without changing the result (Kammar and Plotkin, "Algebraic foundations for effect-dependent
optimisations", POPL 2012: the commutation, discardability and copyability equations hold of the
pure computations). So any two orders of the activations, and any repetition, give the same answers
to every activation. The host's result depends on the λ only through those answers. ∎

So **under U the body must be pure**, and an impure one is refused, saying so. This is Go's
`FieldsFunc` contract made a check, and under S an impure body is admitted. Theorem D is about the
λ, not about the host: `sort.Slice` is unstable, so equal elements may come out in different orders
on different hosts. That is the host function's own disagreement, and it makes the name Tier 2, no
portability claim, as for any host name.

**A law the callback must satisfy is not a set.** `sort.Slice`'s `less` must be a strict weak order.
That is a relation between activations, not a pointwise claim Π(P).R, so ADR 0053's heading cannot
state it. One case is decided by a lemma: **an order pulled back along a function is a strict weak
order**. If key : X → ℤ, then x ≺ y ⇔ key(x) < key(y) is irreflexive, transitive, and its
incomparability is the equivalence ker(key). So `fn(i, j) { key(a[i]) < key(a[j]) }` is lawful by its
shape. A general `less` is not decided. On Go a lawless `less` gives a permutation in an unspecified
order, not a crash, so stating the law waits on a program that needs it (§8).

### 3.4 What a λ captures, by kind of binder

The λ's free names are partitioned by kind, and each kind has its rule.

| captured | rule | why |
|---|---|---|
| a static name (a set, a definition, a constant) | erased by staging | not in the residual |
| a value: scalar, string, table, record, product | read freely | V is immutable; the frame outlives the call (L) |
| a cell, read only | read freely | its value at the call is a value |
| **a cell written in the body** | threaded through the call as an extra result; **refused under U** | state passing (below); Theorem D |
| a buffer | lent for the call, like a cell; read and written as the body says | ADR 0084's Theorem P at the call: a view lent and returned |
| `again`, `try`'s failure, `expect`'s crash | `again` and `try` refused across the λ; `expect` and `abandon` allowed | no unstructured control; ∅ is initial (ADR 0090) |

**A written cell is state passing.** storeorder-research's state-passing translation is how a cell
already becomes loop variables. A cell x written in the λ makes the host call, observably,

```
h_x : X × (Y × S → Z × S) × S  →  W × S
```

the state monad's lifting of h (Moggi 1991), with S the cell's value lent into the call and the last
activation's S returned. This is callbacks-research's Theorem T with the host's activations in
place of messages, and Theorem N's nesting holds by L. **Theorem T₁**: under L and S, the threaded
program has the meaning of the program whose cell is one variable written in place, captured by
reference. The proof is Theorem T's, with the history now the host's specified sequence rather than
a tree of dispatches. Under U the history is the host's, so a written cell is a non-pure body and
Theorem D refuses it. *Corrected 2026-11-20* (callbacks-sketches Part 3, §3.2): this named a write
that commutes with itself, `n := n + 1`, as an exception. It is not one. Its result is the same in
every order, but under U the count of activations is the host's too, so n₀ + count is not a
function of the arguments. A commutative action is independent of order, not of count.

**A buffer under L.** callbacks.md §4 refused a captured buffer because "two goroutines could hold
it". That is E and C's hazard. Under L the buffer is lent for the call, as a cell is (ADR 0048: a
host writes a cell through a reference to a fresh copy; ADR 0070: a host header is the caller's view
of a borrow). The body's reads and writes then happen in the host's sequence, so under S they are
the program order's, and under U writes are refused by Theorem D. R3's "no buffer captured" is
needed exactly where the λ escapes.

**A loop's own control does not cross.** An `again` in the λ would jump from inside the host's
frame to a loop outside it, which is unstructured control. The host's way to stop early is the
callback's result: `IndexFunc`'s `true`, `WalkDir`'s `fs.SkipAll`. A `try` that propagates a
failure out of the λ is the same jump. A failure is returned through R when the host's callback
type can carry one (`WalkDirFunc`'s `error`), and otherwise it is handled inside. `abandon` stays,
since a function into ∅ never returns and needs no continuation.

### 3.5 On the IR: a region, read as a loop whose trip count is the host's

The IR is structured SSA (ADR 0032), and a loop's body is already a region with π-parameters. The
λ becomes **a region of the call**: its parameters the λ's, its free SSA values the captures read
in place, its tails returning to the host. A written cell or a lent buffer becomes an extra
parameter of the region and an extra result of the call, which is §3.4's state passing in SSA form.
MLIR's operations with regions (`scf.for`'s body, `linalg.generic`'s per-element region; Lattner
et al., CGO 2021) are the same construction, a region that the operation calls.

**Every analysis reads the region as a loop whose trip count is k_h:**

| analysis | a loop | a callback region |
|---|---|---|
| intervals (`ir.Decide`) | the body under the guards, widened to a fixpoint | the body under x̄ ∈ P; a written cell's value a fixpoint widened with the loop's thresholds for `0..`, one pass for `1`, joined with its entry value for `0..1` |
| the result | `again`'s arguments, the exit's value | each tail in R, an obligation (ADR 0072) |
| size-change termination | the loop's measure | the region has no `again`, so it terminates when its body does; the host call's own termination is the host's, as for any host call |
| usage (𝒰) | the body's usage times the trip count | k_h · 𝒰(body), §3.3 |
| release (ADR 0096) | a table read in the body lives to the loop's end | a table read in the region lives to the call's end |

The last row is a soundness rule with its fault: a table captured by the region and released before
the call is a read of freed memory on LLVM, and on Go, JavaScript and the JVM it is invisible, since
the host collects it. So it can be caught only on windows, and the planted fault (§5) must fail
there.

### 3.6 On each target: the host's λ, or closure conversion

- **Go**: a `func(…) … { … }` literal at the call, the region printed as its body. Captures are Go
  variables, a written cell a Go variable captured by reference, and the closure stays on the stack
  (§2.1). The parameter types are ρ of P, which the claim states (arrow-research §4.4).
- **JavaScript**: an arrow function. A written cell is a `let` captured by reference.
- **The JVM**: a lambda behind the host's functional interface. Java captures only effectively final
  locals (JLS §15.27.2), so a captured value printed as a reassigned local, such as a loop variable
  or a join's slot, is first copied to a final one. A written cell needs a holder, a one-element
  array, which is what hand-written Java does. Whether ART removes a capturing lambda's allocation
  is unmeasured, and owed when a program runs one.
- **LLVM (windows)**: closure conversion (Minamide, Morrisett and Harper, POPL 1996), ∃E. (E × A → R)
  × E, into the host's own pair.
  - The region is lifted to a function of ours with the C convention the callback typedef declares,
    `CALLBACK` being `x86_stdcallcc` on i686 (ADR 0094). One lifted function per call site, so
    defunctionalization (Reynolds 1972) has one constructor and needs no dispatch.
  - E is a record of the captured values and pointers to the written cells, an `alloca` in the
    caller's frame. This is sound because the λ is L: the frame outlives every activation.
  - **The context word carries E's address**, where the API passes one (291 of 349). No program
    observes it, so it is representation, and it is written where representation is: the API's
    call takes it as an argument term, `ctx(f)`, beside `ref(x)` and `elem(b, lo)` (ADR 0118,
    0130), and the program's call omits it. *Corrected 2026-11-18*: first proposed as a determined
    parameter of the header, `= context(f)`, the fault ADR 0130 removed from `in`.
  - **Where the API passes none** (33 of 349, and `qsort`), E is found through a slot,
    callbacks-research §3.5's evidence. Writing the slot is a dynamically scoped binding: save the
    old value, write E's address, call, restore. That is shallow binding (Baker, "Shallow binding in
    Lisp 1.5", CACM 1978). It is sound under L by nesting, since a nested call of the same API from
    inside the λ saves and restores its own E. One thread has one slot. Threads would need a
    thread-local slot, and wait on ADR 0081.

So the shared piece with the window procedure is **a function of ours printed with the host's C
convention, its address passed as a value**: the lifted function here, the trampoline there.
Tier 1 adds the region, its environment and the context word. The window procedure adds the
installed state and Theorem T's lending to dispatching calls. Neither needs the other beyond that
shared piece.

### 3.7 The callback hole, and the surface

**A program's surface is unchanged.** J writes `fn(r) { r == c }` at a call (j.md §2.7), and the
λ's binders take their sets from the claim (ADR 0066 item 5).

**A target file's declaration** gains its markers after the arrow, where `pure` already goes
(j.md §2.4), and the hole is the existing named splice:

```
fn IndexFunc(s ∈ go.bytestring, f ∈ fn(r ∈ Rune) (Bool) pure calls 0.. ordered) (Int) pure {
    host expr "strings.IndexFunc(${s}, ${f})" import "strings"
}
```

`${f}` receives the region printed as the host's λ, so callbacks.md's "callback hole" is a splice
whose argument is a region, and no new template syntax is needed. The count is written as a set of
naturals, `calls 1`, `calls 0..1`, `calls 0..`, because a usage interval is a range, and ranges are
already sets (ADR 0077). No new vocabulary is needed for it beyond the word `calls`.

For a C declaration (ADR 0094 by prototype, ADR 0126 by header), the callback's typedef is a named
claim, and **the claim is the program's view**: the λ's parameters as the program sees them, with
no context word, which no program observes (ADR 0130's test). Where C carries the environment is
representation, in two places, as a struct's layout and its `&x` are:

```
set WNDENUMPROC = fn(h ∈ HWND) (BOOL) calls 0.. ordered
repr WNDENUMPROC c(h, ctx) stdcall
fn EnumWindows(lpEnumFunc ∈ WNDENUMPROC) (r ∈ BOOL) { host expr call "EnumWindows"(lpEnumFunc, ctx(lpEnumFunc)) stdcall lib "user32" }
```

- `repr WNDENUMPROC c(h, ctx) stdcall` is the callback's C signature: its C parameters in order,
  each a claim parameter or `ctx`, the word where E's address arrives, with the convention. The
  lifted function is printed from it and fills `ctx`'s position from E. A typedef with no `ctx`
  (the 33 with neither, §2.2) finds E through the slot.
- `ctx(f)` is a C argument term, the address of f's environment, where the API passes it.

The program calls `winuser.EnumWindows(fn(h) { … })`, and its λ has the claim's parameters.
*Corrected 2026-11-18*: first written with the context in the header, `l ∈ LPARAM context` and
`lParam ∈ LPARAM = context(lpEnumFunc)`.

## 4. The targets

- **Portability is computed as for any host name.** A tier-1 parameter is a host's, so a program
  using `strings.IndexFunc` is Go's, and the capability graph reports it. A language name taking a
  λ, a portable `index_where(s, p)` in Σ_L, would be spec first, with each target's realization. It
  is not proposed here.
- **Go, JavaScript and the JVM** print the host's λ. **windows and windows32** print the lifted
  function and the environment. **Android** (the JVM) and **the browser** (JavaScript) are their
  backends' cases.

## 5. Planted faults, before any witness is trusted

| rule | the fault | what must fail |
|---|---|---|
| the claim's domain | the λ's parameters assumed beyond P | `fn(r) { t[r] }` with r unproven in t's domain accepted; it must be refused |
| the claim's result | a tail not checked against R | a λ returning outside R accepted |
| Theorem D | the purity check under U off | `sort.Slice` with a `print` in `less` accepted; it must be refused, naming the order |
| 𝒰's clause | the count not scaling the body's usage | a relevant value consumed under `calls 0..` accepted |
| Theorem T₁ | a written cell's extra result dropped | `strings.Map` counting runes prints 0 |
| release | a captured table released before the call (windows) | the λ reads freed memory: a wrong answer or a crash on windows only |
| the slot | the slot not restored after a nested call | a nested context-less enumeration's outer λ sees the inner E |
| the context | E's address passed in the wrong argument | the λ reads garbage and crashes |
| the JVM's capture | a reassigned local captured without a final copy | `javac` refuses the emitted code |
| `again` | an `again` across the region accepted | refused, saying the jump would leave the host's frame |

## 6. Candidates

- **A. A template hole and no IR construct.** callbacks.md's first reading: the λ's body is printed
  as text into the host's λ syntax.
  - *Against*: ADR 0038, the IR's text is the printers' whole input. A body that is not in the IR
    is unseen by `ir.Decide`, size-change termination, the release analysis and the usage check, so
    every rule of §3.4 would be unchecked. A path nothing runs is a path nothing checks.
- **B. A region on the call (recommended).** §3.5's region, read as a loop of trip count k_h, with
  written cells and lent buffers threaded as results. Printed as the host's λ on Go, JavaScript and
  the JVM, and by closure conversion into the context word or a slot on LLVM. Every rule of §3 is
  decided on the IR.
- **C. Defunctionalization** (Reynolds 1972): each λ a constructor, one `apply` dispatching on it.
  - It is B's LLVM realization with one constructor per site, since the residual is monomorphic and
    each call site has its own region. As a separate mechanism it buys a dispatch nothing needs.
- **D. Replace the host function by our loop** (`IndexFunc` written in J).
  - That is always available, and is a program's choice: JavaScript's array methods measured 3.6×
    to 133× slower than a loop (js-toplevel-2026-08-18). As the mechanism it would lower further
    than the target requires, and it gives up the host's own `sort`.

## 7. Decisions for hamza

1. **Tier 1 is L: a callback applied only during the call** (105 of Go's 150). E is not tier 1. E
   splits into a spawn (`go`, `AfterFunc`, `WaitGroup.Go`: a process, ADR 0081), an installed
   handler (`http.HandleFunc`, `flag.Func`: the window procedure's case, callbacks-research B), and a
   host object holding our code (`Scanner.Split`, `ModelFunc`). C's blocking form (`T.Run`) is L by
   every observation but the goroutine's identity. Recommended.
2. **The markers.** `calls K`, with K a set of naturals (`1`, `0..1`, `0..`), and `ordered`, written
   after the arrow as `pure` is. **The default is the conservative one, `calls 0..` and unordered**,
   so a declaration that says nothing admits only a pure body that consumes nothing relevant, and the
   declarer adds what the host's documentation promises. Escape needs no marker yet: a callback
   parameter is L, and an escaping one is declared by decision 1's other forms. The words are for
   hamza.
3. **A written cell**: threaded under `ordered`, refused without it (Theorem D), with commuting
   writes waiting on a program. Recommended.
4. **A buffer under L**: lent for the call like a cell, which narrows callbacks.md §4's refusal to
   escaping callbacks, or kept refused until a program needs it. I recommend deciding it now and
   building it when a program writes one, since the rule is Theorem P's and the refusal's reason was
   concurrency.
5. **The context word on a C declaration** is representation, written where representation is:
   the callback type's C signature as a `repr`, `repr WNDENUMPROC c(h, ctx) stdcall`, and the API's
   context argument as a call term, `ctx(f)`. A slot (shallow binding) serves where the API has
   none. The claim is the program's view, `fn(h ∈ HWND) (BOOL)`. The words `ctx` and `c(…)` for a
   callback are for hamza. (Corrected 2026-11-18, after ADR 0130: this was a determined parameter
   of the header.)
6. **The order of building.** The piece shared with the window procedure comes first: a function of
   ours printed with the host's C convention, its address a value. Then whichever of tier 1 and the
   window procedure the plan reaches first. For plan item 6, TinyRetroPad, that is the window
   procedure, and tier 1's trigger is a Go-library program (`strings.IndexFunc`,
   `strings.FieldsFunc`, `sort.Slice`, `filepath.WalkDir`) under the standing goal. Recommended.

## 8. Not here

- **Escaping callbacks** (E, 41 of 150), each with its own reading (decision 1).
- **A callback's law**, such as `less`'s strict weak order, beyond the pullback lemma of §3.3.
- **Commuting writes under U**: withdrawn 2026-11-20, since the count under U is the host's too
  (callbacks-sketches Part 3, §3.2).
- **Order-2 callbacks**: the 9 iterator consumers take an `iter.Seq`, which a program holds as a host
  value (arrow-research §4.5). A λ written as a push iterator is order 2, waiting on a program.
- **A language name taking a λ** (a portable `sort` or `index_where` in Σ_L).
- **JavaScript's and ART's allocation of a capturing λ**: unmeasured, owed when a program runs one.

## Paid and bought (this research)

- **Compiler, emission, proofs, refusals**: unchanged; nothing built.
- **Tooling**: `gauntlet/stdlib/win32.go` gains `-callbacks` (one report, about 80 lines) and keeps
  each callback typedef's parameter list. The survey's default report is byte-identical to HEAD's.
  `gauntlet/stdlib/callbacks-go-markers.txt` records the 150 functions' classification, made by
  reading `go doc` and spot-checked against five members' documentation.
- **The measurements**: one Go program for the escape analysis and the two surveys, under two hours
  with the documents read first.
