# CLAUDE.md

Guidance for Claude Code working in this repository. **It states where the project is, not how it got
there.** Each result has its document in [gauntlet/results/](gauntlet/results/), each round an
assessment in [docs/](docs/), each decision an ADR in [docs/decisions/](docs/decisions/); follow the
links for history. Update this file where the state changes, and never append a narrative. The long
versions are in git: `73004de` (a 60,000-word changelog) and `b22d553` (17,300 words, before this
rewrite, 2026-10-26).

## What this project is

Oroboros is a small language and build system that **parasitizes** target ecosystems rather than
abstracting over them ([ADR 0001](docs/decisions/0001-parasite-model.md)). Portability is a property a
program may or may not have, and the compiler computes it.
- **One mechanism, a capability graph**: modules declare what they require, targets what they provide,
  and building covers the first with the second ([ADR 0002](docs/decisions/0002-capability-graph.md)).
  **Emit at the highest layer the target natively provides**, and lower only as far as necessary.
- **General-purpose** ([general-purpose.md](docs/general-purpose.md)). **The targets are Go, Windows,
  Android and the browser** ([ADR 0039](docs/decisions/0039-the-targets-are-go-windows-android-and-the-browser.md)):
  the JavaScript backend's host is the browser (Node only tests it as an engine), the Java backend's is
  Android, ART its runtime of record.
- **The atom** ([the-atom.md](docs/the-atom.md)): λ-calculus whose normal form is a parameter. A target
  partitions names into primitive and defined, and reduction runs until only primitives remain.
- **Two levels** ([closures-direction.md](docs/closures-direction.md)): a static level, unrestricted and
  higher-order, erased by staging; a dynamic level of first-order tables and loops.
- **Two languages** ([ADR 0049](docs/decisions/0049-types-are-sets-and-the-logic-never-prints.md)): a
  computation that reduces and prints, and a logic whose types are sets and whose claims are formulas,
  which never prints. The one bridge is ρ, a representation chosen from proven sets.

## Working rules (hamza's)

- **Always be mathematical and algebraic, and reach for the literature.** Before building or declaring
  anything, say what it is: the set, the operation, the law, the theorem that makes the design sound,
  where the literature answered it. A host package is an algebra before it is a list of functions.
  Derive first, measure second, and never substitute a list of cases for a derivation.
- **Follow the derivation where it leads, even into a rewrite.** Propose the better design, whatever it
  costs to build; never present the cheaper option as the recommendation when the algebra says
  otherwise. When the algebra decides a question already in front of hamza, take it.
- **Spec first.** Nothing enters the language without a specification of how it behaves on every
  target; every primitive is classified in [primitives.md](docs/spec/primitives.md), and a Tier 1 name
  needs a conformance case ([gauntlet/conformance/](gauntlet/conformance/)). The test is not "is it
  useful": what does it mean independently of a target; what does each target do; if they disagree, is it
  observable (then Tier 2, no portability claim)?
- **An ADR per significant decision**, from the template in [docs/decisions/README.md](docs/decisions/README.md);
  its "Why not" is the point. A reversal is a new ADR. A rejected alternative raised later joins the
  "Why not".
- **A result document per step**, `gauntlet/results/NAME-YYYY-MM-DD.md`: what was measured, what failed
  first, what it cost, what was not built. **An assessment when a plan finishes**, not an ADR: it scores
  the plan, measures the balance the same way every time, names process failures, and sets the next plan
  with a "deliberately not next" list.
- **README.md's "Current work" is the plan for a reader**: updated when a plan is set and when an item
  finishes, dated, and saying no more than the plan.
- **Emission is byte-identical by default.** `cmd/check` compares every source × target's emitted code,
  outcome, proof counts and refusal text against [gauntlet/check/](gauntlet/check/). A difference is kept
  only when correct and no slower, with `-accept "reason"` (which needs the compiler and differential
  steps passing in the same run, and is logged in `ACCEPTED.md`); otherwise fix the code. **An accept
  that changes a count updates every document quoting it.**
- **Compile time is gated** (gauntlet/check/README.md): 1.5× the baseline and 250 ms slower is a change
  for review, confirmed by a second sweep and a serial pair against `gen` built at the baseline's commit.
  To say how long one program compiles, time its binary serially and warm.
- **Every soundness rule gets a planted fault** that fails before its witness is trusted, the bug that
  shipped included; a refusal test states its reason, is checked for it, and computes its premise; a
  check refuses something false before it is trusted to pass something true.
- **Every experiment gets a time limit.**
- **Keep documents current.** A stale status line is worse than none; fix it when the thing ships.

## Primary documents

Read these before planning anything that touches types, claims or the surface. They stay here until
everything in them is built (hamza, 2026-10-10).

| document | decided by | built |
|---|---|---|
| [settypes-research.md](docs/settypes-research.md): types are sets, a Boolean algebra of static values; the decidability map (Presburger, regular strings, the array property fragment, BAPA, congruence closure), above it assertions and lemmas; own procedures, no SMT | ADR 0049 | the Omega test, `presburger/` ([presburger-2026-10-11](gauntlet/results/presburger-2026-10-11.md): refusals lack facts, not proof strength); types as static values, step 1 (ADR 0063); next: the algebra of sorts, when a program states a set it cannot write |
| [spec/abstract.md](docs/spec/abstract.md): the language as the initial algebra of a binding signature, its meanings homomorphisms (erasure ε, desugaring D, obligations 𝒪, usage 𝒰) | ADR 0050 | the trees and D (`core/abs.go`, `decl.go`, `desugar.go`); usage on trees; ε and 𝒪 wait for claims that carry sets |
| [fit-research.md](docs/fit-research.md): a syntax fits when its abstract syntax is the semantics' initial algebra; misfits are countable | ADR 0050, 0068 | J is the surface |
| [notation-research.md](docs/notation-research.md), [syntax-research.md](docs/syntax-research.md): the mathematical notation and six notations measured | ADR 0068 | J |
| [storeorder-research.md](docs/storeorder-research.md): program-order stores are the state-passing translation's image | ADR 0052, 0058 | built ([storeorder-2026-10-15](gauntlet/results/storeorder-2026-10-15.md)) |
| [arrow-research.md](docs/arrow-research.md): the arrow is a claim on a host's callback parameter | ADR 0053 | not built (callbacks tier 1) |

## Where it stands

**Read first**: the primary documents above, [README.md](README.md), [design-direction.md](docs/design-direction.md)
(the founding argument; its §8 records how its questions were settled, concurrency the one open), the
ADRs, [spec/state.md](docs/spec/state.md), and the current assessment. Measurements in
[gauntlet/results/](gauntlet/results/) are the authority: every design claim here not measured has been
wrong about half the time.

**The current assessment is [assessment-2026-10-24.md](docs/assessment-2026-10-24.md)**; read it
before planning. Previous: [10-21](docs/assessment-2026-10-21.md), [10-20](docs/assessment-2026-10-20.md),
[10-14](docs/assessment-2026-10-14.md), [10-03](docs/assessment-2026-10-03.md),
[09-28](docs/assessment-2026-09-28.md), [09-24](docs/assessment-2026-09-24.md),
[09-23](docs/assessment-2026-09-23.md), [09-17](docs/assessment-2026-09-17.md),
[09-13](docs/assessment-2026-09-13.md), [09-11](docs/assessment-2026-09-11.md),
[09-09](docs/assessment-2026-09-09.md), [09-06](docs/assessment-2026-09-06.md),
[08-20](docs/assessment-2026-08-20.md), [08-19](docs/assessment-2026-08-19.md),
[08-13](docs/assessment-2026-08-13.md).

**The standing goal** (hamza) is **the Go standard library, package by package, program first**: a
program in the language that needs a package, its algebra, and exactly the declarations it calls,
checked. When a package hits a wall that needs language work, stop, research it, design it.

### The 10-24 plan (assessment-2026-10-24 §5)

1. ~~The documents tell the truth~~ **done** ([docs-2026-10-25](gauntlet/results/docs-2026-10-25.md)):
   j.md's status read off the reader; state.md in J; inventory.md in two layers; j.md §1.4 and the
   inventory's counts checked against the code; `cmd/check`'s `gauntlet` step.
2. ~~Two surface questions~~ **done** ([surface-2026-10-25](gauntlet/results/surface-2026-10-25.md)):
   an unbounded end is omitted, `lo..` (ADR 0077; hamza's `inf` weighed and rejected there); `pub` marks
   the interface, private the default, `export` the entry points (ADR 0078).
3. ~~Three defects the audit found~~ **done** ([defects-2026-10-26](gauntlet/results/defects-2026-10-26.md)):
   no module defines a name of Σ_L (a nested `fn if` had captured every `if`); the recursion refusal names
   `loop`; a set operator in a term is refused saying what it is.
4. ~~A declared set is a claim, and the facts choose the representation~~ **done** (ADR 0079,
   [claimset-2026-10-26](gauntlet/results/claimset-2026-10-26.md)); then `Nat`.
5. ~~CLAUDE.md rewritten as state~~ **done** (2026-10-26, this file).
6. **The Windows track, in steps** (hamza's):
   - 6a. **MASM or LLVM IR**, researched and decided by an ADR. LLVM reaches ARM, Linux and macOS from one
     backend; against it, duplicated work and checks the type system made unnecessary. Weights: ADR
     0001/0008 (LLVM as an ecosystem parasitized, like the JVM), what our IR already decides (chordal
     registers, proven ranges; `nsw`/`nuw`/poison as the way a proof drops a check), the 2,794-byte size
     standard, the toolchain, and a measurement of the sieve and one gauntlet program on each path.
   - 6b. **TinyRetroPad's Win32 surface, measured**: downloaded 2026-10-24 at `42f4c6c` as data, never
     built or run; 2,686 lines of 32-bit `stdcall` MASM, 30 procedures, 67 Win32 functions over kernel32,
     user32, gdi32, comdlg32 and shell32, RichEdit loaded at run time.
   - 6c. **Its walls, spec first, an ADR each**: the window procedure (callbacks tier 2), its structs
     (records), 32-bit against x86-64 (decided by 6a).
7. **A π is justified by its branch, and the verifier checks it**, before item 8 grows lowering. W6
   checks a π's types, not its relation; a π on the wrong table passed it (work-2026-10-22 §5). The rule
   is Theorem E as a W-rule, with faults planted for the wrong table, a flipped relation, a foreign
   operand and a missing negation.
8. **An `again` after a nested loop's join point** (tables.md §2.5): 16% of `wc` against idiomatic Go
   ([wc-2026-10-24](gauntlet/results/wc-2026-10-24.md)). Research it as a join point that may jump, build
   it, measure `wc` nested.
9. **The Go library, program first**: `cmp`, two files compared chunk by chunk.
10. **The analysis grows only for a refusal a program names**, each rule's planted fault with it.

**Deliberately not next**: ARM, Linux and macOS targets themselves; loop-arm patterns and `try`'s
handlers; a result claim per instance; the clause chain's copying, `e2`, `lent`; optimizing the compile;
a join across representations; hoisting or CSE on the IR (it would reach two premises witnessed only on
hand-built IR); a limb instance emitted once as a function; per-result enforcement above the word and
signed limbs (ADR 0029); the higher-order contract gap (ADR 0028); records, `with` and named views beyond
what TinyRetroPad's walls need.

**Waiting on a trigger** (researched, built when a program needs it):
- **One integer rendering**, render : ℤ → Σ*, per representation, replacing `big-str` (declared on three
  targets, and a hidden allocation on a value that fits the word). Trigger: printing's P3.
- **A representation as an instance of a type's signature** (data refinement, Hoare 1972; Haftmann et
  al. 2013), replacing integers' four mechanisms. Trigger: a fifth integer representation (ℤ/2ⁿ for
  `hash/fnv`) or a second string encoding (Win32's `W` functions).
- **Validity through functions that preserve it**: refinement sorts (Freeman and Pfenning 1991) for
  `strings` over V ⊂ B*. Trigger: a program paying for `go.text` right after a `strings` call.
- Six decisions each on **concurrency** ([concurrency-research.md](docs/concurrency-research.md)) and
  **Android's toolkit** ([android-research.md](docs/android-research.md)); **printing**'s three algebras
  ([printing-research.md](docs/printing-research.md)); a borrow a later call ends (`Scanner.Bytes`,
  [borrow-research.md](docs/borrow-research.md)); §1.4's reservation of J's words.

### What is measured

- **The gauntlet** ([docs/gauntlet.md](docs/gauntlet.md)), the one fixed commitment: seven programs at
  parity with hand-written code on Go, JavaScript and Java. Last full benchmark
  [gauntlet-2026-09-29](gauntlet/results/gauntlet-2026-09-29.md): geometric means 0.990, 0.977, 0.987,
  three residues above 1.10× each explained. The 26 files its harnesses call are listed in
  `gauntlet/regen.txt`, written by `bash gauntlet/regen.sh` and regenerated and compared by `cmd/check`'s
  `gauntlet` step. The Windows sieve is 0.98× hand-written MASM.
- **Provability**: **2,103 of 2,148** integer operations proven inside their target's word, the 45 left
  mostly meant to be refused; **353 of 376** loops proven to terminate (size-change termination). Each
  program's share is in its result document; both counts are pinned by `cmd/check`.
- **Host APIs** (surveys, declarable / usable): Go 87.8% / 60.5%, the JVM 81.4% / 60.9%, Win32 90.5%
  declarable and 31.0% callable and linkable; `go run gauntlet/stdlib/coverage.go` gives the declared
  share of the Go packages with a file, with a cause for every missing name
  ([gocoverage-2026-10-06](gauntlet/results/gocoverage-2026-10-06.md)). **Supported** (declared by hand,
  checked against the host, exercised by a program, ADR 0022): `unicode/utf8`, `encoding/hex`, `strconv`,
  `encoding/binary`'s varints, `math/bits`, `io`'s interfaces; **partial and checked**: `os`, `strings`,
  `io`, `fmt`, `errors`, `io/fs`, `path/filepath`, `time`, `bufio` and its `Reader`, `unicode`. The
  portable `os` (`lib/os.oro`) runs on Go, JavaScript and Java with one output.
- **The balance**, named in every assessment: the compiler grows faster than the code written in the
  language. `python gauntlet/balance.py [--nodes] [REV…]` reproduces every assessment's table. At
  `b22d553`: compiler 73,854 lines (24,461 of them tests; `core` 20,257, `emit` 33,442, `ir` 20,155), the
  language 2,086 lines; the 10-24 round's margin 20.6 : 1. The largest program is still `freq`, 194
  lines. **Write programs, and let them demand the analysis.**

## The compiler

`go run ./cmd/check` passes every step: vet, compiler, emission, gauntlet, ir, differential, tooling.

**The pipeline.** J's reader (`core/jlex.go`, `jparse.go`, `jreadtarget.go`) builds abstract Oroboros's
trees, and D (`core/desugar.go`) makes the core term; `Load` resolves modules, expands `case`, `try` and
`expect`, and translates cells and buffers (S, the state translation, `core/cell.go`, `place.go`). The
β/δ reducer, call-by-need with an effect discipline, gives the residual: closed, first-order,
monomorphic. The type checker and the refinement layer run on it; then it is lowered to **the IR**
(ADR 0032, [spec/ir.md](docs/spec/ir.md)), structured SSA with π-parameters, which every analysis and
every backend reads.

**On the IR**: `ir.Decide` proves each operation inside its set (intervals over ℤ̄, thresholds, trip
bounds, the relational difference) and refuses the program or, under `-checked`, traps; size-change
termination (`ir/sct.go`); the representations, the unsigned word (`ir/words.go`, ADR 0033), the rung
above the word as a least fixed point of supply and demand (`ir/big.go`, ADR 0034, 0079), fixed limbs as
instances of a theory checked once (`ir/limbs.go`, ADR 0035); postconditions and a definition's claim
(`ir.CheckClaim`, ADR 0072). Each rule is checked against a concrete interpreter over ℤ
(`ir/interp_test.go`) and has a planted fault.

**The IR's text is the printers' whole input** (ADR 0038): `gen -ir` writes IR_P, and `cmd/check`
reprints every program from that text alone, byte for byte. Four printers, `ir/golang`, `ir/js`,
`ir/java` and `ir/x86` (MASM, a value's place a colouring of exact live sets, optimal by chordality),
sharing `ir/plan`. The term backends and the term interval analysis are deleted (irstep4a, irstep4j).

## The language today

**The surface is J** ([spec/j.md](docs/spec/j.md), ADR 0068; its status says what is built), and every
`.oro` file is J. [spec/state.md](docs/spec/state.md) is the language read off the code, checkable by
grep; [inventory.md](docs/spec/inventory.md) is every word the compiler knows, pinned by a test. A
parenthesized s-expression is a core term, which only the tests and the IR's text write. The other
specs' examples are in the notation J replaced, and each moves to J when it is next touched (hamza's
option (b), jswitch5b-2026-10-16).

### Core

- **Seven core term kinds**: name, integer, float, string, boolean, `fn`, application. A residual's
  binders are named apart along the scope chain (ADR 0036), so a context keyed by name is sound.
- **A declaration is a header and one body** (ADR 0051, 0062, 0064): `fn f(x ∈ S) (R) where φ ensure ψ
  { e }`, D's `def` and `sig`. A constant is a value, `cap = 65536`. **A comment is `//`, a gap, never a
  token** (ADR 0024, 0067); a tool that rewrites source keeps every gap or refuses the edit.
- **A block binds sequentially**, `{ x = e; (a, b) = p; body }`, and a statement is a binding whose name
  is discarded (ADR 0054). `if c { a } else { b }`, `&&`, `||`, `!`, `!=` and `case { φ -> a; else -> b }`
  are D's (ADR 0017); a guard chain costs nothing ([cond-2026-09-19](gauntlet/results/cond-2026-09-19.md)).
- **Effects are a side condition on β** (ADR 0010, [effects.md](docs/spec/effects.md)): a purity bit per
  primitive, impure by default; an impure argument is let-bound, never substituted. **β counts
  evaluations in {0, 1, ω}** (ADR 0071): a pure argument is substituted when evaluated at most once.
- **Iteration is `loop x = z { φ -> again(…); else -> r }`** (ADR 0015). `again` is a jump in a tail: an
  arm's body, a block's end, a host call's continuation (ADR 0027), a `try` or `expect` success arm
  (ADR 0041), each arm of a `case` in a tail (ADR 0076); never under an `if`. In the reader the rule is a
  sort. Every walker of a clause chain walks all four forms, or it is unsound. **No recursion**
  (ADR 0014); a balanced recursion is a loop over levels.
- **Relevance** (ADR 0043): a value of a `relevant` variant is used on every path, `ignore(e)` the
  discard. **Failures are values of a marked sum** (ADR 0040): `lib/result`, `try`, `expect`, the host's
  encoding a niche of its error type.
- **Closures** may not survive staging; host callbacks' tiers 1 and 2 are specified
  ([callbacks.md](docs/spec/callbacks.md)) and not built.
- **Modules**: a file is a theory named by its path (ADR 0064); `use p as a`; **`pub` marks the
  interface, private the default, `export` an entry point** (ADR 0078); **no module defines a name of
  Σ_L**, the language's signature (`core.LanguageSignature`, defects-2026-10-26).

### Data

- **Every data form is a function whose domain differs** ([data.md](docs/spec/data.md)). `Array(V)` on
  `[0, len)`, indexing `t[i]` application ([tables.md](docs/spec/tables.md)); a graph `array { 2, 3 }`
  and a rule `array { e : k ∈ 0..n-1 }`; `Map(Int, V)` with a declared capacity, a read an `Option`
  ([maps.md](docs/spec/maps.md)).
- **State is a binder of a block** (ADR 0047, 0052, 0058): `var b := array {…}` a buffer, `var m :=
  map(cap)` a map buffer, anything else a cell, written in program order (`b[i] := v`, `x := v`) with
  R0–R4 decided on each scope; S gives the core's threaded `build`; a cell becomes loop variables. A host
  writes a cell through a reference to a fresh copy (ADR 0048). A buffer parameter is `b ∈ Array(V)
  linear`, borrowed by reference; a host header is the caller's view of a borrow (ADR 0070); a pure host
  call observes a live buffer (ADR 0069), a segment `b[lo:hi]` its function restricted (ADR 0074).
- **A scope's value may be a product** of frozen buffers and values (ADR 0031), taken apart at a join
  point; an `again` after it is not built. Tuples `(a, b)` and the unit `()`; a table of tuples is
  flattened by currying ([products.md](docs/spec/products.md)).
- **Variants**: `set S = 'a ∪ 'c(T)`, closed, finite, non-recursive, with type arguments; at a boundary its
  tag and one slot per payload type (ADR 0042, 0059).
- **Strings** are the free monoid over Unicode scalars, `++` and `string_of`; a host's string is the
  host's (`go.bytestring`), entered by one total decode, `go.text` (ADR 0030,
  [strings.md](docs/spec/strings.md)). **Not built**: records, symbols, `with`.

### Integers

- **An `Int` is an integer** (ADR 0026), and a range `lo..hi` is a set; an end may be omitted, `lo..` and
  `..hi` (ADR 0077), and **`Nat`** is ↑0. Each target declares its word as data, `repr LO..HI word`:
  int64 on Go, the JVM and windows, ±(2⁵³−1) on JavaScript; Go also realizes 0..2^64-1 as `uint64`.
  Legality is per (program, target), portability computed (`go run ./cmd/portable`).
- **Bounded by default** (ADR 0019): an operation not proven inside the word is refused, cleared by
  narrowing a range, a claim above the word, or `-checked`.
- **A declared set is a claim, and the facts choose the representation** (ADR 0079,
  [integers.md §1a](docs/spec/integers.md)): ρ of a value is its proven fact's, the word, the unsigned
  word, or the rung above; a claim above the word is the consent to that rung, and the representation
  itself only at an entry point's parameter or result. Above the word a target chooses `repr big host`
  or `repr big limbs`, and the program's one set is enforced on both, by sign and bit length (ADR 0029).
- **Staging never changes an answer** (ADR 0009): integers fold only inside the word, checked, never
  dividing by zero; floats fold through explicit `float64`.

### Types, claims and facts

- **The type checker** runs on the residual ([types.md](docs/spec/types.md)); the integer sort is ℤ, its
  realization the IR's (ADR 0033); `Table(σ)` has its introduction, elimination and store rules.
- **A header's sets are claims**: a parameter's set and `where` are obligations at every call a
  definition is inlined into, assumed at an export's boundary (ADR 0028); **a declared result is a claim
  decided at its definition**, exported or not (ADR 0072); every name used as a set must denote one
  (ADR 0073); a host `where`'s constant bounds are its parameter's range (ADR 0075). **A set has no
  representation**: a set operator in a computation or a claim's formula is refused saying what it is.
- **An obligation is discharged or the program is refused** ([refinements.md §3a](docs/spec/refinements.md)),
  by a literal, the IR's intervals, then the refinement layer (Fourier–Motzkin over the linear fragment,
  facts as a solved form). Every pipeline carries its obligations in the residual (ADR 0037).
- **Facts** are guarded boundedness axioms ([facts.md](docs/facts.md)); content facts are derived, never
  declared (Houdini invariants, store chains, strided components, record facts, translation invariants,
  a peeled first iteration). A clamp in a program is a missing proof.

### Declarations and targets

- **Declarations are theories** (ADR 0021, [theories.md](docs/spec/theories.md)): a module a theory, a
  target a model, `use`, `pub`, `export` and `provides` morphisms. **A target is a chain of layers**,
  glue within, override between ([target-system.md](docs/spec/target-system.md)).
- **A host module's path is the host's** (`go/encoding/hex`, ADR 0025), and the file is the path; a
  companion is a child module; a bare type name resolves lexically.
- **Host declarations are written by hand**, by someone who read the host's source, and checked against
  it by the generator, which never claims what it cannot justify (ADR 0022, 0023;
  `gauntlet/stdlib/tooling_test.go`). `fn f(x ∈ S) (R) pure { host expr "F(${x})" import "p" }`; a
  variadic is one declaration over a table (ADR 0046); a host's empty interface is a type (ADR 0045).

## Constraints that override normal instincts

- **Anything promoted to the language works on every target, and the compiler finds the
  implementation.** A target may neither decline nor declare a language construct; the capability graph
  is for target-native names, where "this target cannot do it" is a true answer.
- **Never lower further than the target requires**: no hand-rolled hash table into Go, which has `map`.
- **Never assert which host construct is fastest: measure it** (ADR 0008). Treat host compilers as black
  boxes; `go build -gcflags="-m -m"` and `-gcflags="-d=ssa/check_bce/debug=1"` have each been decisive.
- **Never make the core a superset of one host.** JavaScript has no integers or structs; the JVM no
  unsigned types, `goto` or tail calls; Go restricts `goto`.
- **Never introduce boxing or hidden allocation into the core**: that killed the predecessor
  ([design-direction.md §2](docs/design-direction.md)).
- **Never add unstructured control flow.**
- **Performance claims are measured against hand-written code**, never on less than the ~15% noise floor
  of the hybrid P/E-core laptop; carry the form expected to lose as well as the one expected to win.
- **Primitives are declared in `targets/*.oro`, not in Go.** Only structural constructs live in code.
- **Prefer deriving a fact over clamping around it or declaring it.**
- **The core is deliberately unspecified** ([core-candidates.md](docs/core-candidates.md)); the
  predecessor stalled on a fixed language. Beware the minimality trap: the core is minimal subject to
  lowering natively to every target at zero cost. Candidates are killed by measurement, not argument,
  and the derivations are in [docs/derivations/](docs/derivations/).

## Lessons that recur

Each has bitten more than once; the instances are in the results named.
- **A path nothing runs is a path nothing checks** (a `; skip:`, a backend no target used, an escape
  taken everywhere, a kind nothing has, a file regenerated by hand; bounds-2026-09-24, bufio-2026-10-01,
  docs-2026-10-25).
- **A harness that cannot fail proves nothing**: plant the fault first; refusal-shaped properties need an
  anti-vacuity guard; random search cannot test what over-approximation makes sound.
- **A relation relaxed for one check weakens every check that reads it** (W5 after ADR 0033). Audit
  every consumer.
- **"Not refused" is not "proven"**, and a check that runs only where a caller remembers to ask is no
  property of the program (noprop-2026-09-25, hazard-2026-10-02). State a precondition on the consumer.
- **An empty case read as "everything"**: an interface with no names showed every name (ADR 0078); a
  `where` over an unbound name was assumed at an entry point (defects-2026-10-26).
- **A comment stating what code does is not a review of whether it should**; a rule's comment can state
  a premise that is no longer true (`WiderThanWord`, claimset-2026-10-26). Only a planted fault shows
  whether a rule's conclusions are true.
- **A context keyed by name is captured by the name alone**, and `Body()` and `openFresh` rebuild terms:
  a map keyed by a term pointer answers about a copy (bufio-2026-10-01, localtab-2026-10-14).
- **The emitter must be a function of its input**: run it twice; it is reprinted from the IR's text.
- **A survey's first number describes the measurer**, and a count is a measurement too
  (matchguard-2026-09-24). Read ten members of a figure before it becomes a plan item.
- **Benchmark-method errors have many species**: a closure in the reference, a fixed iteration count,
  shared processes, eliminated work, two things changed at once, a modified input, comparing against
  HEAD after a regression, timing among parallel jobs, a stopped run still running, one megamorphic call
  site, a baseline built from `git stash`. Time a baseline in a worktree. Make a suspicious result explain
  itself before recording it.
- **A refusal can hide a wrong answer**, and **a program finds what a construct suite cannot**.
- **A test can pin a stale text**: `TestScopeCheck` required "fold-range" (defects-2026-10-26). Read
  what a test asserts before trusting it to guard a change.

## Decisions already made

Read the ADR, and its "Why not", before relitigating one.

| Decision | ADR |
|---|---|
| Targets are ecosystems; portability is a program property | [0001](docs/decisions/0001-parasite-model.md) |
| A capability graph, not a fixed layer tower | [0002](docs/decisions/0002-capability-graph.md) |
| Range-typed integers: mathematical semantics, machine representation | [0003](docs/decisions/0003-range-typed-integers.md) |
| ~~Go, JavaScript, Java first~~ (superseded by 0039) | [0004](docs/decisions/0004-first-targets.md) |
| The compiler is written in Go | [0005](docs/decisions/0005-implementation-language.md) |
| The backend interface is a file format | [0006](docs/decisions/0006-ir-file-format.md) |
| Explore candidates against a fixed test; do not specify the core first | [0007](docs/decisions/0007-exploration-over-specification.md) |
| Parasite decisions are per-target measurements | [0008](docs/decisions/0008-measurement-over-principle.md) |
| Staging must not change results | [0009](docs/decisions/0009-staging-preserves-results.md) |
| Effects are a side condition on β | [0010](docs/decisions/0010-effects-as-structural-rules.md) |
| Modules are resolution, not reduction | [0011](docs/decisions/0011-modules-add-nothing-to-the-reducer.md) |
| ~~`int` within ±(2⁵³−1)~~ (window superseded by 0026) | [0012](docs/decisions/0012-portable-integer-range.md) |
| ~~Accept the allocation price~~ (superseded by 0020) | [0013](docs/decisions/0013-accept-the-allocation-price.md) |
| Recursion is not in the language | [0014](docs/decisions/0014-recursion-is-not-in-the-language.md) |
| `loop`/`again`, guarded clauses over n variables | [0015](docs/decisions/0015-loop-and-again.md) |
| A target need not be an expression language | [0016](docs/decisions/0016-targets-need-not-have-expressions.md) |
| Booleans and control flow are in the language | [0017](docs/decisions/0017-booleans-are-in-the-language.md) |
| Immutable values, one scoped linear buffer | [0018](docs/decisions/0018-immutable-values-linear-buffers.md) |
| Precision by declaration, provisionally (items 3–4 amended by 0079) | [0019](docs/decisions/0019-precision-by-declaration.md) |
| A buffer is a nameable type: uniqueness on parameters | [0020](docs/decisions/0020-uniqueness-on-parameters.md) |
| Declarations are theories | [0021](docs/decisions/0021-declarations-are-theories.md) |
| Host declarations are written by hand; the generator checks them | [0022](docs/decisions/0022-host-declarations-are-written-by-hand.md) |
| A generator does not make a claim it cannot justify | [0023](docs/decisions/0023-a-generator-does-not-make-a-claim-it-cannot-justify.md) |
| A comment never carries meaning | [0024](docs/decisions/0024-comments-are-erased.md) |
| A module path is the host's, and the file is the path | [0025](docs/decisions/0025-a-module-path-is-the-hosts.md) |
| An `int` is an integer; each target realizes what it can; portability is reported | [0026](docs/decisions/0026-an-int-is-an-integer.md) |
| A host call's continuation is a tail | [0027](docs/decisions/0027-a-host-calls-continuation-is-a-tail.md) |
| A definition's parameter range and `where` are obligations at its calls | [0028](docs/decisions/0028-a-definitions-contract-is-checked-at-its-calls.md) |
| Above the word, one set decided by sign and bit length, enforced on every representation | [0029](docs/decisions/0029-above-the-word-one-set-on-every-representation.md) |
| A host's string is the host's; ours is entered by one total decode | [0030](docs/decisions/0030-a-hosts-string-is-the-hosts.md) |
| A `build`'s result is a product of frozen buffers and values | [0031](docs/decisions/0031-a-builds-result-is-a-product.md) |
| The IR is structured SSA with π-parameters; every backend prints from it | [0032](docs/decisions/0032-the-ir-is-structured-ssa.md) |
| An integer's representation is chosen on the IR; the checker's sort is ℤ | [0033](docs/decisions/0033-an-integers-representation-is-chosen-on-the-ir.md) |
| The rung above the word is a least fixed point of supply and demand (supply amended by 0079) | [0034](docs/decisions/0034-the-rung-above-the-word-is-a-least-fixed-point-on-the-ir.md) |
| A compiler library is a theory checked once; a use is an inlined instance | [0035](docs/decisions/0035-a-compiler-library-is-a-theory-checked-once.md) |
| A residual's binders are named apart along the scope chain | [0036](docs/decisions/0036-a-residuals-binders-are-named-apart.md) |
| A program's obligations are in its residual, decided by one procedure | [0037](docs/decisions/0037-a-programs-obligations-are-in-its-residual.md) |
| The IR's text is the printers' whole input; read by others, written by nobody else | [0038](docs/decisions/0038-the-irs-text-is-read-not-written.md) |
| The targets are Go, Windows, Android and the browser | [0039](docs/decisions/0039-the-targets-are-go-windows-android-and-the-browser.md) |
| A failure is a value of a marked sum; `try` is bind, `expect` the one way to give up | [0040](docs/decisions/0040-a-failure-is-a-value-of-a-marked-sum.md) |
| A success arm is a tail; a loop yields a sum | [0041](docs/decisions/0041-a-success-arm-is-a-tail.md) |
| A variant's value is its tag and one slot per payload type | [0042](docs/decisions/0042-a-variants-value-is-its-tag-and-its-slots.md) |
| Relevance is checked at β on every path; `ignore` is !_A | [0043](docs/decisions/0043-relevance-is-checked-where-a-binder-meets-its-value.md) |
| A partial success's error is relevant | [0044](docs/decisions/0044-a-partial-successs-error-is-relevant.md) |
| A host's empty interface is a type | [0045](docs/decisions/0045-a-hosts-empty-interface-is-a-type.md) |
| A variadic is one declaration over a table | [0046](docs/decisions/0046-a-variadic-call-is-its-declaration-applied-to-a-word.md) |
| Local state is a scope; a cell is a loop variable | [0047](docs/decisions/0047-local-state-is-a-scope-and-a-cell-is-a-loop-variable.md) |
| A host writes a cell through a reference to a fresh copy | [0048](docs/decisions/0048-a-host-writes-a-cell-through-a-reference.md) |
| Types are sets and the logic never prints; ρ from proven sets; own procedures | [0049](docs/decisions/0049-types-are-sets-and-the-logic-never-prints.md) |
| The abstract syntax is the semantics' initial algebra | [0050](docs/decisions/0050-the-abstract-syntax-fits-the-semantics.md) |
| A definition is a named λ carrying its claims | [0051](docs/decisions/0051-a-definition-is-a-named-lambda.md) |
| A buffer is a state binder holding a table, in program order; R0–R4 | [0052](docs/decisions/0052-a-buffer-is-a-state-binder-holding-a-table.md) |
| The arrow is a claim on a host's callback parameter (not built) | [0053](docs/decisions/0053-the-arrow-is-a-claim-on-a-host-parameter.md) |
| `let` and `local` are one scope | [0054](docs/decisions/0054-let-and-local-are-one-scope.md) |
| A loop variable is bound by application | [0055](docs/decisions/0055-a-loop-variable-is-bound-by-application.md) |
| One arm grammar for choice and iteration (the surface: 0068) | [0056](docs/decisions/0056-the-surface-is-d-or-j-and-its-choice-and-iteration.md) |
| The handled bind is a derived operator; its success arm a tail | [0057](docs/decisions/0057-the-handled-bind-and-its-tail.md) |
| A map buffer is a state binder holding a map | [0058](docs/decisions/0058-a-map-buffer-is-a-state-binder-holding-a-map.md) |
| A tag is a quoted symbol; collections have one former | [0059](docs/decisions/0059-a-tag-is-a-quoted-symbol-and-collections-have-one-former.md) |
| Membership `∈`; an imported tag is qualified | [0060](docs/decisions/0060-j-set-operators-and-qualified-tags.md) |
| Separation and replacement by the binder's place (not built) | [0061](docs/decisions/0061-separation-and-replacement-by-the-binders-place.md) |
| A header is a membership statement; `fn`, `spec`, `lemma` (the logic not built) | [0062](docs/decisions/0062-a-header-is-a-membership-statement-and-the-logic-has-three-definitions.md) |
| A type is a static value; instantiation is application | [0063](docs/decisions/0063-a-type-is-a-static-value-and-instantiation-is-application.md) |
| A file is a theory; a declaration a header and one body (item 4 superseded by 0078) | [0064](docs/decisions/0064-a-file-is-a-theory-and-a-declaration-is-a-header-and-one-body.md) |
| J writes sets in TLA+'s symbols | [0065](docs/decisions/0065-j-writes-sets-in-tla-symbols.md) |
| A definition over sets is a λ returning a λ; named splices `${x}` | [0066](docs/decisions/0066-a-definition-over-sets-is-a-lambda-returning-a-lambda.md) |
| The lexical level | [0067](docs/decisions/0067-the-lexical-level.md) |
| **J is the surface** | [0068](docs/decisions/0068-j-is-the-surface.md) |
| An observer at a host parameter | [0069](docs/decisions/0069-an-observer-at-a-host-parameter.md) |
| A host header is the caller's view of a borrow | [0070](docs/decisions/0070-a-header-is-the-callers-view-of-a-borrow.md) |
| Work stays where it was written: β counts evaluations | [0071](docs/decisions/0071-work-stays-where-it-was-written.md) |
| A declared result is a claim decided at its definition | [0072](docs/decisions/0072-a-declared-result-is-a-claim-decided-at-its-definition.md) |
| A set's name denotes a set | [0073](docs/decisions/0073-a-sets-name-denotes-a-set.md) |
| A segment is its function restricted wherever its table is | [0074](docs/decisions/0074-a-segment-is-its-function-restricted-wherever-its-table-is.md) |
| A host `where`'s constant bounds are its parameter's range | [0075](docs/decisions/0075-a-wheres-constant-bounds-are-the-parameters-range.md) |
| A case in a tail is a tail when each arm is | [0076](docs/decisions/0076-a-case-in-a-tail-is-a-tail-when-its-arms-are.md) |
| An unbounded end is omitted, `lo..`; `Nat` is ↑0 | [0077](docs/decisions/0077-an-unbounded-end-is-omitted.md) |
| A module's interface is what it marks `pub`; `export` an entry point | [0078](docs/decisions/0078-a-modules-interface-is-what-it-marks-pub.md) |
| A declared set is a claim, and the facts choose the representation | [0079](docs/decisions/0079-a-declared-set-is-a-claim-and-the-facts-choose-the-representation.md) |

## Build commands

Module path is `oroboros` (local).

```bash
go run ./cmd/check                 # every check (gauntlet/check/README.md)
go run ./cmd/check -skip tooling   # everything but the host surveys
go run ./cmd/check -accept "why"   # keep this run's emission as the baseline, and log why
go test ./core/ ./emit/ ./ir/...   # the compiler
go run ./cmd/build -target=go -o hello.exe examples/hello.oro   # a real binary
go run ./cmd/oro -target=go examples/native/dot-go.oro          # reduce to normal form
go run ./cmd/gen -ir dot.ir -name native examples/native/dot-go.oro go dot.go   # emit, and write the IR
go run ./cmd/intervals -v examples/table/sieve.oro go           # what the decision proves, and what not
go run ./cmd/portable examples/io/wc.oro                        # which targets accept a program, and why
bash gauntlet/regen.sh                                          # regenerate the gauntlet's 26 files
cd gauntlet/go && go test -bench='TreeGen|TreeFlat$' -benchtime=20000x -count=5   # generated vs hand-written
```

- `gauntlet/go`, `gauntlet/js`, `gauntlet/java` and `experiments/legibility` are separate modules: `cd`
  into them first. `gauntlet/fmt/*.go` are `//go:build ignore` scripts run with `go run`.
- On Windows, Git Bash's `/tmp` is not visible to native tools: use the session scratchpad. A heredoc
  eats backslashes: write a script containing `\n` with the Write tool, not through bash.
- **Line endings are LF** (`.gitattributes`); the four Windows scripts are CRLF and the emission baseline
  is `-text`, so `gofmt -l` means what it says.

## What exists

| | |
|---|---|
| `core/` | J's reader (`jlex`, `jparse`, `jreadtarget`), σ_J (`jspell`), J in messages (`jshow`), the raw reader of core terms and the IR's text (`raw`, `read`); the trees (`abs`, `decl`) and D (`desugar`); terms, the β/δ reducer and module loading (`reduce`), variants (`sum`), hygiene, the state translation (`cell`, `place`), β's work count (`work`), Σ_L |
| `ir/` | lowering, typing, the verifier (W1–W10), the printer and reader; the decision (`decide`, `interval`, `trip`, `sct`, `require`), the representations (`words`, `big`, `limbs`, `shift`), the claim (`claim`), the pipeline (`ir.Entry`); `ir/plan` and the four printers `ir/golang`, `ir/js`, `ir/java`, `ir/x86` |
| `presburger/` | the Omega test (Pugh 1991), complete over ℤ, a soundness witness for the refinement layer |
| `emit/` | the type checker, the refinement layer (`refine`, `linear`, `fact`, `content`), contracts (`requires`), the target loader (`target`, `companion`, `alias`, `constend`, `hostdef`), linearity, the limb library as a theory (`bigrep`, `biglimb`, `bignum.oro`), the inventory's tests |
| `targets/` | target declarations, data and not Go: `go/`, `js/`, `java/`, `windows/`; `blas` and the `tutorial*` targets parameterize the normal form for the book |
| `lib/` | library modules: `os`, `io` (portable names over each host), `result`, `num`, `win` |
| `cmd/` | `check`, `build`, `gen`, `oro`, `intervals`, `portable` |
| `examples/` | the programs: `io/` (`wc`, `count`, `jsonfmt`, `roundtrip`, `freq`), `os/` (`du`, `ls`, `wc`), `lines/` (`lines`, `ages`, `shape`), `json/`, `big/`, `int/` (meant to be refused), `kara/`, `tally/`, `u128/`, `native/` (the gauntlet's sources) |
| `gauntlet/` | hand-written references, `results/`, `check/` (the baseline), `differential/` (cases on four targets, proving their arithmetic unless `; checked:` says why), `conformance/`, `regen.txt`, `balance.py`, `surface/` (the surface's measurements) |
| `gauntlet/stdlib/` | the four host surveys (writing J through `jdecl`), `coverage.go`, `callbacks.go`, twenty acceptance programs, and `tooling_test.go`, which checks every hand declaration against its host |
