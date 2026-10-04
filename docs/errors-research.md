# Research: errors

2026-10-04. **Research, not a decision.** hamza: "we have been using them with no research and no
spec, we kind of adopted Go's model. Let's do it properly, the math, the algebra, the literature and
the candidates. If a better error model surfaces we shouldn't settle for a lesser one." It is related
to [concurrency-research.md](concurrency-research.md), whose failure algebra (§2.4 there) is half of
this one. It follows ADR 0007: candidates with what would kill each. Three measurements are taken
([errors-2026-10-04](../gauntlet/results/errors-2026-10-04.md)); the rest are proposed in §7.

The short version: **"error" names three different things, and each has its own algebra.**
1. **The program's own partial operations**: an index, a divisor, a contract. They are *domain
   conditions*, and the language already proves them or refuses the program.
2. **Abandonment**: a state the program cannot continue from. It is the exception effect with no
   handler inside the process, and the process boundary is where a supervisor takes over.
3. **The environment's failures**: a file absent, a socket reset. They are *values*, the right
   summand of a coproduct A + E, and the type says which shape each host call has.

Go's model conflates (3)'s algebra with an encoding, the product. Measured, the encoding leaks: one
program, accepted on three hosts, does three different things. The sum that should replace it costs
nothing once inlined, also measured.

---

## 0. What there is, measured

From [errors-2026-10-04](../gauntlet/results/errors-2026-10-04.md):

- **A fallible host call is declared `(tuple T error)`** with an opaque host `error` and the
  discriminant `err-nil`. That covers 77 of 381 Go `sig`s and the portable `os`'s 2 on JavaScript and
  Java. **Windows declares 0 of 87 fallible:** `ReadFile` and `WriteFile` fail by sentinel and
  `GetLastError`, and are declared total.
- **The corpus has 16 fallible call sites in 7 programs.** Every one tests only `err-nil`, and **no
  program reads an error's content**. Five errors are discarded silently, all defensibly, and nothing
  tells them from a forgotten check.
- **The product leaks.** `(let (tuple src err) (os.ReadFile "missing") (len src))` compiles on Go,
  JavaScript and Java:
  - Go prints `0`;
  - JavaScript throws `TypeError`;
  - Java throws `NullPointerException`.

  ADR 0026's promise, the same answer wherever accepted, is broken by an encoding.
- **A partial host call declared `pure` makes its failure depend on reduction.** `Long.parseLong` is
  declared `pure` and total, and throws. Unused, reduction drops it, and the program prints `7`. Used,
  it throws. That breaks ADR 0009 through a declaration.
- **A sum costs nothing inside a program.** A `(variant (result T) …)` propagated through three levels
  reduces to one branch per error path, with no tag or constructor left. That is the commuting
  conversion, and it gives the code a hand-written nested `if` gives.

`sums-research.md` §3 chose "Result + explicit propagation, `try` as reader sugar" on 2026-08-22.
Variants were built and Result never was: host calls kept the product, and design-direction §8 records
the error model as "settled in part".

---

## 1. Three kinds of error

| | what it is | who can prevent it | the algebra | in the language today |
|---|---|---|---|---|
| **1. a domain condition** | an operation applied outside its domain: `(a i)` with i ≥ len a, `(/ x 0)`, a `where` false at a call | the program; it is a **bug** | a partial function f : A ⇀ B whose domain is decidable on the program's facts; an **obligation** | proven, or the program is refused (refinements.md §3a, ADR 0028); arithmetic outside the word is refused, or trapped under `-checked` (ADR 0019, 0026) |
| **2. abandonment** | a state the program has no way on from: an assertion it chose not to prove, a trap it asked for, an exhausted resource | nobody, locally; the process is lost | the exception effect crash : E → 0, unhandled inside the process; the **fault boundary** is the process | a trap ends the whole program: one process, no supervisor (concurrency-research.md §3) |
| **3. the environment's failure** | the world did not cooperate: no such file, permission denied, connection reset, malformed input from outside | nobody; it is **not a bug**, and no proof can exclude it | a total map into a coproduct, A → B + E | the product `(tuple v err)` and `err-nil` (§0) |

This is Duffy's split for Midori (2016), which he reached after Midori tried most of the
alternatives: **bugs are abandonment, and recoverable errors are typed and explicit.** It is
Armstrong's too. Erlang returns `{ok, V} | {error, Reason}` for expected failures (a sum) and crashes
on the unexpected (abandonment), and the pattern `{ok, Bin} = file:read_file(F)` is the partial
elimination that turns the one into the other. Gray's taxonomy (1985) explains why the split is right:
- a bug (a Bohrbug) recurs on retry, so it is either removed or abandoned;
- the environment's failures (mostly Heisenbugs) are why a program has an error path at all.

**What the language adds to Duffy's model is the first row.** Midori checked contracts at run time
and abandoned on violation. This language proves them at compile time, or refuses. So the domain
conditions that are bugs never reach run time unless the program asks (`-checked`), and the run-time
error model has only rows 2 and 3 to cover.

---

## 2. The algebra

### 2.1 A fallible call is a map into a coproduct, and the host's documentation says which one

A partial map A ⇀ B is a total map A → B + 1, and with information about the failure it is
A → B + E (Moggi 1991, the exception monad). **B + E is the coproduct**: an element is exactly one
of a B or an E, and its eliminator is `case`.

Go's `(T, error)` is the **product** T × E′, where E′ = E + 1 (nil is the 1), with a convention: when
the error is non-nil the T is meaningless. As sets:

  {(t, e) ∈ T_⊥ × E′ | e = nil ⇔ t ≠ ⊥} ≅ T + E,

so the product *restricted by the invariant* is the coproduct, encoded. The encoding admits the
states the invariant excludes, and reading t when e ≠ nil reads ⊥, which each host spells
differently: Go `nil`, JavaScript `null`, Java `null`. That is the measured leak, as algebra.

**But not every Go pair is a sum, and that is a fact about the host, not a flaw in it.**
- `io.Reader.Read` returns n > 0 bytes **and** an error, and its documentation says to process the n
  bytes first. Its honest type is ℕ × (1 + E): a partial success.
- `strconv.ParseInt` on an out-of-range input returns the **saturated** value with `ErrRange`
  (strconv.oro's header). On a syntax error it returns 0, which carries no information. So its type
  is T + E_syntax + (T × E_range), a sum of products. That is the "these" or `Ior` type
  (A + E + A × E), which the language's variants can already spell:
  `(variant parsed (ok int) (syntax) (range int))`.
- `os.ReadFile` is a pure sum, B* + E.

So **the declaration states which**, from the host's documentation, as every other declaration
states its algebra (ADR 0022). "Go's model" is not one algebra. It is one encoding of several.

### 2.2 Propagation is the exception monad's bind, and the laws are the refactorings

On T X = X + E:
- `return x = inl x`;
- `m >>= f = case m of inl x → f x | inr e → inr e`, a case whose error arm is the identity.

The monad laws are the refactorings a programmer relies on: inlining a helper, splitting one, and
reassociating a chain of fallible steps change nothing:
- left identity, `return x >>= f = f x`;
- right identity, `m >>= return = m`;
- associativity.

**Syntax for bind** is what the languages differ on: Haskell's `do`, Rust's postfix `?`, Swift's
`try`, Zig's `try`, and Benton and Kennedy's "exceptional syntax" (2001), which repairs the scoping
of handlers. Each is reader sugar for the `case`, and sums-research §3.1 gave the language's:
`(try e (fn (v) rest))`.

**Cost.** The argument for exceptions has always been that explicit propagation costs a test and a
branch at every level (Goodenough 1975). Inside a program it does not, because every call is inlined
and the commuting conversion fuses the cases. The error path is one branch to the handler (measured,
§0). Only a result crossing an export's boundary pays a tag. This is where Herb Sutter's
"zero-overhead deterministic exceptions" (P0709, 2019) and Swift's error ABI (a dedicated register,
2015) arrived from the other direction: **exceptions implemented as return values**.

### 2.3 What an error carries: E

The choices, in increasing information:

1. **E = 1**, an option. Enough when the only question is whether it worked. Every program in the
   corpus asks only that (§0).
2. **E opaque**, the host's error. That is what there is: printable on its host, and comparable only
   by the host's own functions (`os.IsNotExist`).
3. **E a finite set of kinds K**, closed, which a portable layer can define:
   `(variant fs-error not-found permission is-directory (other host-error))`. Each host's cell maps
   its errors into K by a **classification** h : E_host → K, a surjection onto the kinds, with
   `other` carrying the residue. That is the parasite principle again: the host's idiom at the
   boundary, one meaning inside.
4. **E structured**: kinds, plus the host's detail, plus context (what was being done). Context is an
   action of the free monoid of frames on E: each layer prepends a frame. That is Go's `%w` wrapping
   (Go 1.13, `errors.Is`/`As`), Rust's `anyhow::Context` and Java's cause chain.

**Error sets form a lattice.** Zig infers a function's error set as the **union** of the sets of the
calls it propagates from, and a set coerces to any superset. Finite sets of names under inclusion,
union as join, and inference as the least upper bound: a join-semilattice. The language's
subsumption already joins at the larger type (types.md §3.2), so a smaller error variant flowing into
a larger one is the same rule. Rows (Rémy; Koka's effect rows, Leijen 2014) and OCaml's polymorphic
variants (Garrigue 1998) are the open versions. A closed variant is the closed version.
**Exhaustiveness** of a `case` over K is Maranget's (2007), and sums.md has it.

### 2.4 Must an error be used? Relevance

Five errors in the corpus are dropped silently (§0). The type-theoretic question is the structural
rule of **weakening**:
- an **affine** value may be dropped;
- a **relevant** value must be used at least once;
- a **linear** value exactly once.

Rust's `#[must_use]` on `Result` is a warning; Go's `errcheck` is an external linter. The language
already denies weakening for impure terms (ADR 0010) and checks a buffer's linearity by counting
occurrences on the residual (ADR 0018). **An error value can be made relevant by the same counting.**
Dropping one is then a refusal, and a deliberate discard is written as one: `(ignore e)`.

**The host's own algebra can make an error redundant, and the declaration should say so.** A
`bufio.Writer`'s errors are sticky: the first error is kept and every later write and `Flush` returns
it. The errors form a monoid in which the first non-nil absorbs the rest (`first-error`, ⊕ with nil
as unit). So a write's error is subsumed by `Flush`'s, and a declaration that states the stickiness
can move the obligation to `Flush`. That is exactly why `lines.oro`'s three discards are right.

### 2.5 Accumulating errors is applicative, not monadic

Validating ten fields should report every invalid field, not the first. That is **Validation**
(McBride and Paterson 2008): when E is a semigroup, errors combine under `<*>`. Validation is an
applicative functor and **not a monad**, because bind must stop at the first error to have a value
to continue with. So "fail fast" and "collect all" are different structures, and both are ordinary
functions over a variant. A library, not a language feature.

### 2.6 Termination or resumption: who decides what happens next

Goodenough (1975) distinguished two models:
- **termination**: the handler replaces the failed computation;
- **resumption**: the handler supplies a value and the computation continues.

Mesa had resumption, and Stroustrup reports that its users found it almost never needed and
error-prone (*Design and Evolution of C++*, 1994, §16.6). Every mainstream language chose
termination.

Common Lisp's **condition system** (Pitman 2001) is resumption done well. Low-level code signals a
condition and offers *restarts* (`use-value`, `skip-entry`), and a high-level handler chooses one
*without unwinding first*. Policy is separated from mechanism. Algebraic effect handlers (Plotkin and
Pretnar 2009) generalise it: an operation is raised, and the handler receives its continuation.

**In this language a restart is a static function parameter.** The language is two-level: the static
level is unrestricted higher-order and is erased by staging (closures-direction.md). A library
function that would offer restarts takes the policy as a static argument, `(parse-all rows
on-bad-row)`. Staging inlines the caller's choice at the failure site, so it costs nothing at run
time and needs no continuation. Effekt compiles handlers by passing them as capabilities
(Brachthäuser et al. 2020); here staging does it. A handler that only aborts is the exception monad
again (the standard translation), so termination is a sum and resumption is a static function. Both
are already in the language.

### 2.7 Explicit paths are what linearity needs

Weimer and Necula (2004, 2008) measured where error-handling code goes wrong: **on the error paths**,
which leak resources, because an exception adds an invisible edge from every call to the handler.
Exceptions make the error path implicit, and a sum makes it explicit. This language checks linearity
on every path of the residual (ADR 0018). A buffer, and later a file handle that must be closed
exactly once (`defer` on Go, `errdefer` on Zig, `try`-with-resources on Java), can only be checked if
the error path is a path the checker sees. The IR's exits are single-level (spec/ir.md §1.3). An
exception would be a multi-level exit from every call, which the IR does not have and every walker
would have to learn ("every walker of a clause chain walks four forms").

### 2.8 Abandonment is a visible, partial eliminator

Rust's `unwrap`/`expect`, Erlang's `{ok, V} = …` and Swift's `try!` eliminate a sum *partially*: the
missing arm is abandonment. In this language, a partial eliminator `(ok! r)` is an obligation of
exactly the kind refinements.md already handles: "r is `ok`".
- **Discharged** (the refinement layer knows r is `ok`), it is a projection, and free.
- **Undischarged**, the language's rule says refuse, unless the program asked for the trap. That is
  `-checked` for arithmetic, and here it is the site-local request the `!` spells.

So **abandonment is never implicit**. It happens where the source says `!`, where `-checked` was
asked for, or where a host failure was declared as abandoning. Under concurrency it ends one process
and hands the failure to its supervisor (concurrency-research.md §2.4). Without concurrency it ends
the program, as a trap does now.

---

## 3. The literature, by system

| system | recoverable errors | bugs | propagation | what it teaches |
|---|---|---|---|---|
| **CLU** (Liskov and Snyder 1979) | `signals` in the signature, termination model | `failure` | explicit `resignal`, or automatic to `failure` | the ancestor of checked exceptions, and already a typed sum in all but name |
| **Ada** (1983) | exceptions, unchecked | the same | automatic | |
| **Java** | checked exceptions (`throws`) | unchecked (`RuntimeException`, `Error`) | automatic | checked exceptions failed on **versioning and scale** (Hejlsberg, "The trouble with checked exceptions", 2003): a new error changes every signature above it, and they compose badly with lambdas. The type information was right; the propagation default and the lack of inference were not |
| **C#**, **Kotlin** | unchecked exceptions | the same | automatic | |
| **C++** | exceptions; `std::expected` (C++23) | `noexcept`, terminate | automatic | P0709 (Sutter 2019): exceptions as values, statically typed |
| **Go** | `(T, error)` | `panic`, rarely `recover` | manual `if err != nil` | errors are values (Pike 2015); the product encoding (§2.1); `%w` context chains |
| **Rust** | `Result<T, E>` | `panic` (abandonment, often process-wide) | `?`, with `From` coercing E into a larger error | the sum, the bind, `#[must_use]`, and coercion of error types |
| **Swift** | `throws`, typed since SE-0413 (2024) | `fatalError`, overflow traps | **marked**: a `try` at each call that can throw | the Error Handling Rationale (2015) argues for marked propagation: automatic propagation that each call site still marks. Implemented as a return value in a register |
| **Zig** | error unions `E!T`, **inferred error sets** | `unreachable`, safety checks | `try`, `catch` | error sets as a join-semilattice; `errdefer`; error return traces |
| **Haskell** | `Either`, `ExceptT` | `error`, plus exceptions in `IO` | `do` | the monad; also a cautionary mixture of three mechanisms |
| **OCaml** | `result` and exceptions | exceptions | `let*` | effect handlers in OCaml 5 (Sivaramakrishnan et al. 2021) |
| **Koka** | the `exn` effect, in effect rows | — | automatic, tracked in the type | rows (Leijen 2014): error sets that compose by union |
| **Common Lisp** | conditions and restarts | the same | handlers without unwinding | resumption as policy over mechanism (Pitman 2001) |
| **Eiffel** | — | contracts; `rescue`/`retry` | — | contracts are a bug's specification (Meyer 1992) |
| **Erlang** | `{ok, V} \| {error, R}` | crash, to the supervisor | `case`, or a partial match that crashes | sums for the expected, abandonment for the rest, at a process boundary |
| **Midori** (Duffy 2016) | checked exceptions with a marked `try` | **abandonment**: contracts, overflow, out-of-range; not catchable | marked | the clearest statement of the split (§1), from a team that measured the alternatives |
| **Elm** | `Result`, `Maybe` | none at run time | `andThen` | a language with no runtime exceptions at all, by having no partial operations |

---

## 4. What each host provides

| host | recoverable | abandonment | cost when it fails | totalised today by |
|---|---|---|---|---|
| **Go** | `(T, error)`: some calls a sum, some a partial success (§2.1) | `panic`, ends the program unless recovered in its goroutine | a value; cheap | nothing: the pair is Go's |
| **JavaScript** | `throw` of anything; a promise's rejection | an uncaught throw ends the program (Node, exit 1) | a throw builds a stack trace | a `try`/`catch` template (2 `sig`s) |
| **Java** | checked and unchecked exceptions | uncaught, ends the thread; on `main`, the program | a throw fills in a stack trace: expensive when failures are frequent (M5) | a `try`/`catch` template (2 `sig`s); everything else declared total, some `pure` (§0) |
| **Win32 / x86** | `BOOL`, `NULL` or `INVALID_HANDLE_VALUE`, then `GetLastError`; COM's `HRESULT`, whose sign bit is failure | structured exception handling for faults; `ExitProcess` | a value | **nothing: failures are not declared** |

**No host gives the coproduct** (lib/os/js.oro's header said it first). Each gives an encoding: a
product (Go), an escape (JavaScript, Java) or a niche, a sentinel plus a side channel (Win32). The
language's job is the one it already does for strings and integers: **the host's encoding at the
boundary, one meaning inside.** Here the meaning is a sum (or a sum of products, §2.1), constructed by
the declaration's template, so that nothing past the boundary sees the encoding.

---

## 5. Candidates

### E1. Go's product, as now

`(tuple v err)` and `err-nil`, unchecked.
- **Killed by measurement:**
  - one program, three behaviours (§0);
  - silent discards indistinguishable from forgotten checks;
  - no content ever used, because there is no portable content.
- Its one virtue, emitting Go's own idiom on Go, survives in every candidate below, because a sum
  *lowers* to Go's pair (sums.md §1).

### E2. The product with a refinement: occurrence typing

Keep the spelling. Make reading `v` an **obligation**, "`err-nil err` holds here", discharged by the
test that guards it, as TypeScript narrows and Typed Racket's occurrence typing does
(Tobin-Hochstadt and Felleisen 2008). Make `err` relevant (§2.4).
- **Gain:** no new surface; the refinement layer already discharges path conditions.
- **Cost:**
  - two names for one value;
  - a partial success (§2.1) needs a different invariant per call;
  - every consumer must know the invariant, and the fact is keyed by name, the hazard ADR 0036
    closed once already.
- **Not killed by a measurement.** It is the coproduct encoded in the facts rather than in the type,
  which a refactoring will one day break, as a context keyed by name did.

### E3. A result sum, `try` as bind, relevance

Host calls return `(result T E)` (or the declared sum of products, §2.1), constructed by the
declaration. Propagation is `try`, reader sugar for `case` (sums-research §3.1). An error value is
relevant (§2.4).
- **Gain:** the type is the algebra; the leak is unrepresentable; propagation is free (measured).
- **Cost:**
  - a migration of 77 Go `sig`s and the portable layer;
  - E is still opaque.
- **Would be killed by:**
  - a sum carried through a loop variable costing a measurable split (M6);
  - a host whose sum cannot be built without an allocation on the success path (M7).

### E4. E3 with error sets: finite kinds, union and subsumption

As E3, with E a closed variant of kinds per operation or per portable module (§2.3). Inference is by
union along propagation, and a smaller set coerces to a larger. `case` over kinds is checked
exhaustive.
- **Gain:** portable content. A program can tell not-found from permission on every host, by
  classification at the boundary.
- **Cost:**
  - each portable error set is a claim about several hosts agreeing, checked as `os` is now;
  - the classification must be total on each host's errors, with `other` the residue.
- **Would be killed by:** hosts whose errors do not classify consistently. Go reports the system's
  `errno` (`os.IsNotExist`, `os.IsPermission`), Node an `e.code` string (`ENOENT`, `EACCES`), and Java
  an exception class (`NoSuchFileException`, `AccessDeniedException`), or a bare `IOException` for some
  failures. Whether the same failure lands in the same kind on all three is M3's question.

### E5. Unchecked exceptions

JavaScript, C# and Kotlin.
- **Killed by the constraints:**
  - non-local exits from every call, which the IR does not have (§2.7);
  - unwinding on x86, which needs a runtime;
  - implicit error paths, which linearity cannot check (Weimer and Necula);
  - design-direction §8 records "no exceptions".

### E6. Checked exceptions

CLU, Java and Midori. A signature `A → B throws E` denotes A → B + E, so E6 is E4 with automatic
propagation and a different syntax.
- **Killed in Java's form** by versioning and the lack of inference (Hejlsberg).
- **Survives in two places.** Midori's and Swift's *marked* propagation, `try` at each call, is
  exactly E4's `try`. Inferred error sets (Zig) remove the versioning cost.
- So E6, done right, *is* E4.

### E7. Effect handlers or conditions with resumption

Policy chosen by the caller, without unwinding. In this language a restart is a static function
parameter, erased by staging, and an abort-only handler is the exception monad (§2.6). So E7 needs no
construct: it is E4 plus higher-order static functions, which the language has.
- **Killed as a language feature; kept as an idiom.** Whether any program needs the idiom is M2's
  question.

### E8. Abandonment for everything: crash and let the supervisor decide

- **Killed for libraries.** A file that does not exist is not a bug, and a caller that can handle it
  must be able to. Armstrong's Erlang itself returns `{error, R}` for expected failures.
- **Kept as row 2 of §1**, at a fault boundary, with abandonment visible in the source (§2.8).

### Where this points

**Duffy's split with the proofs in front, E4 at the boundary, and abandonment visible:**

1. **Domain conditions are proven or refused** (as now, ADR 0028), never run-time errors.
2. **Abandonment is explicit**: a partial eliminator `(ok! r)` that is an obligation (free when
   discharged; refused, or a trap by request, when not), `-checked`, or a host failure *declared*
   abandoning. It ends the process: the program today, a supervised process when concurrency comes.
3. **The environment's failures are values**: each host call's declared sum (or sum of products), with
   E a closed set of kinds and the host's detail as `other`. Propagation is `try`; an error is
   relevant; context frames and validation are libraries; restarts are static functions.
4. **Declarations tell the truth about failure.**
   - A partial host call is declared fallible, or declared abandoning. It is never `pure` and total
     while it throws (§0).
   - Windows' sentinels become sums, through the same templates.

Against Go's model, item by item:
- the leak is gone;
- discards are refused unless written;
- content is portable;
- propagation is as cheap as Go's `if err != nil` and as short as Rust's `?`;
- abandonment is a process boundary that the concurrency research already needs.

This is the hypothesis. Program first.

---

## 6. What this changes elsewhere

- **ADR 0027** is unchanged. A host call's continuation is still a tail. It now eliminates a sum, by
  `case`, rather than binding a product.
- **design-direction §8, item 2** ("settled in part") would become an ADR.
- **The concurrency research** gets its crash: abandonment is crash : E → 0, and §2.8's `!` is where
  a process meets its supervisor.
- **`pure`** gains a precondition: a call declared `pure` must be total on its declared domain. A
  checker can enforce part of it: on JavaScript and Java a template containing no `try` is assumed not
  to throw only if the host's documentation says so. That is ADR 0022's job, by hand.

---

## 7. Measurements and programs that decide

| | what | decides | status |
|---|---|---|---|
| M1 | the product's leak: a failed call's value, read without testing, on three hosts | whether E1 is sound | **done**: three behaviours |
| M1′ | a `pure` partial declaration under reduction | whether purity may hide a throw | **done**: the throw vanishes when unused |
| M2 | a sum propagated through three levels, reduced | whether E3's `try` costs anything | **done**: one branch per error path, no tag |
| M3 | **a program that uses an error's content**: a tool that reads files named on its command line, reports which were missing and which were not permitted, and continues. Hand-written on Go, JavaScript and Java as the bar | whether kinds classify consistently across hosts (E4's kill condition); the first portable error set | proposed |
| M4 | Windows' `ReadFile` and `WriteFile` declared as sums through `GetLastError` | whether a niche encoding totalises through a template as a throw does | proposed |
| M5 | a loop parsing mostly invalid numbers: Java's `parseLong` throwing, against a pre-validating template, against Go's error value | the cost of totalising a throwing host where failures are frequent | proposed |
| M6 | `(again (err 3))`: a sum as a loop variable | E3's kill condition 1 | proposed |
| M7 | a result crossing an export on each host: Go's pair, a Java record (scalar-replaced?), JavaScript's two values | E3's kill condition 2; sums-research §2's table, measured | proposed |
| M8 | relevance on the corpus: how many sites the check refuses, and whether each refusal is a real dropped error | whether must-use is a burden or a finding | proposed: 5 known, all in `lines.oro` |

---

## 8. Questions for hamza

To be decided when a program meets them, as with concurrency:

1. **Sums for host failures:** migrate the 77 Go `sig`s and the portable layer from the product to
   declared sums (E3)?
2. **Error content:** opaque, kinds, or kinds with the host's detail as `other` (E4)?
3. **Relevance:** refuse a silently dropped error, or only warn?
4. **The partial eliminator `ok!`:** refused unless proven, as an index is, or abandonment by request?
5. **Throwing declarations:** audit every JavaScript and Java `sig` for throws, as the Go target's
   ranges were audited in gotarget-2026-09-30?
6. **The first program:** M3, or one of yours?

---

## 9. Not here

- **Resource management** (`defer`, `errdefer`, RAII). It needs handles the language does not have
  yet. §2.7 is why the error model must leave error paths explicit for it.
- **Asynchronous errors**: a promise's rejection, a goroutine's panic. These belong to the
  concurrency model, where they are messages to a supervisor.
- **Stack traces and error return traces** (Zig). They are diagnostics, a host's business, and the
  host gives them.
- **User-defined effect handlers as a feature.** Static functions cover the restart idiom (§2.6).

---

## References

- Benton, N., Kennedy, A. "Exceptional syntax". *JFP* 11(4), 2001.
- Brachthäuser, J. I., Schuster, P., Ostermann, K. "Effects as capabilities: effect handlers and
  lightweight effect polymorphism". OOPSLA 2020.
- Duffy, J. "The error model". 2016.
- Garrigue, J. "Programming with polymorphic variants". ML Workshop, 1998.
- Goodenough, J. B. "Exception handling: issues and a proposed notation". *CACM* 18(12), 1975.
- Gray, J. "Why do computers stop and what can be done about it?". Tandem TR 85.7, 1985.
- Groff, J., McCall, J. "Error handling rationale and proposal". The Swift project, 2015; SE-0413,
  "Typed throws", 2024.
- Hejlsberg, A., with Venners, B., Eckel, B. "The trouble with checked exceptions". Artima, 2003.
- Leijen, D. "Koka: programming with row polymorphic effect types". MSFP 2014.
- Liskov, B. H., Snyder, A. "Exception handling in CLU". *IEEE TSE* 5(6), 1979.
- Maranget, L. "Warnings for pattern matching". *JFP* 17(3), 2007.
- McBride, C., Paterson, R. "Applicative programming with effects". *JFP* 18(1), 2008.
- Meyer, B. *Eiffel: The Language*. Prentice Hall, 1992.
- Moggi, E. "Notions of computation and monads". *Information and Computation* 93(1), 1991.
- Pike, R. "Errors are values". The Go Blog, 2015.
- Pitman, K. M. "Condition handling in the Lisp language family". *Advances in Exception Handling
  Techniques*, LNCS 2022, 2001.
- Plotkin, G., Pretnar, M. "Handlers of algebraic effects". ESOP 2009.
- Sivaramakrishnan, K. C., Dolan, S., White, L., Kelly, T., Jaffer, S., Madhavapeddy, A. "Retrofitting
  effect handlers onto OCaml". PLDI 2021.
- Stroustrup, B. *The Design and Evolution of C++*. Addison-Wesley, 1994.
- Sutter, H. "Zero-overhead deterministic exceptions: throwing values". WG21 P0709R4, 2019.
- Tobin-Hochstadt, S., Felleisen, M. "The design and implementation of Typed Scheme". POPL 2008.
- Wadler, P. "The essence of functional programming". POPL 1992.
- Weimer, W., Necula, G. C. "Finding and preventing run-time error handling mistakes". OOPSLA 2004;
  "Exceptional situations and program reliability". *TOPLAS* 30(2), 2008.
