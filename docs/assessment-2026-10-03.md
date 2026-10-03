# Assessment: the proofs that leaned on a name

2026-10-03. Deliberately **not** an ADR. The previous eleven are
[2026-09-28](assessment-2026-09-28.md), [2026-09-24](assessment-2026-09-24.md),
[2026-09-23](assessment-2026-09-23.md), [2026-09-17](assessment-2026-09-17.md),
[2026-09-13](assessment-2026-09-13.md), [2026-09-11](assessment-2026-09-11.md),
[2026-09-09](assessment-2026-09-09.md), [2026-09-06](assessment-2026-09-06.md),
[2026-08-20](assessment-2026-08-20.md), [2026-08-19](assessment-2026-08-19.md) and
[2026-08-13](assessment-2026-08-13.md).

Written when the 09-28 plan finished: its first two items are done, and the third waits on hamza.
Six days, **19 commits**, 11 result documents and **two ADRs** (0036, 0037).

> **Short answer: the plan was kept, the Go target was made honest, and the first program of the
> round found that the refinement layer's proofs had been leaning on a coincidence of names.**
>
> **Kept.** The gauntlet ran in full on the IR's printers, at parity on all three targets (Go 0.990,
> JavaScript 0.977, Java 0.987). `bufio` was done program first: a line tool over standard input,
> proven, and checked against the line split written from its definition.
>
> **Made honest.** hamza called the files in `targets/go` misleading, and they were. Every file now
> states its algebra and names its parameters, and every host name in them is checked against the
> host: partial files included, variadics as restrictions f|Aⁿ, interface methods against the manifest. The audit found wrong
> claims a reader could not have seen: `bits.Len32(uint32(x))` wrapping a range the declaration
> stated and nothing checked; reads of mutable host state declared pure, which reduction reordered
> past stores; and a portable `WriteFile` that compiled on one host of three.
>
> **Found.** Probing one rule before extending it, a loop variable `n` under a parameter `n` declared
> in [0, 5] indexed a table of 6 at 101, **accepted on all four targets**. Fixing it made the gate
> refuse half the corpus: `(loop ((c c) …))` had been giving the loop variable its buffer's length
> through the shared name. The invariant is established now, by induction, and every program is proven
> again with the counts it had. Then the obligations themselves were found to exist only where a
> driver asked for them.
>
> **The balance.** +2,498 lines of compiler. The language's code fell by 45: +75 written, −120
> deleted with the portable layer and the threaded sieves. Counting what was written, **33 : 1**, the
> best margin of the five rounds recorded, and 16 : 1 in the round's last phase.

---

## 1. What the last assessment asked for, scored

| | asked | happened |
|---|---|---|
| 1 | The gauntlet, in full, on the IR's printers | **Done** ([gauntlet-2026-09-29](../gauntlet/results/gauntlet-2026-09-29.md)). Geometric mean of generated over hand-written: Go 0.990, JavaScript 0.977, Java 0.987; three residues above 1.10×, each explained. The first run was contaminated by a stopped background job still timing beside it, and was redone |
| 2 | `bufio`, program first | **Done** ([bufio-2026-10-01](../gauntlet/results/bufio-2026-10-01.md)). [examples/lines/lines.oro](../examples/lines/lines.oro): 46 lines of code, 5 of 5 operations and its loop proven, no `-checked`. It asked for three things and found two faults (§3.3) |
| 3 | The Windows application: measurement, then spec | **Waiting.** hamza has not handed it over |
| 4 | The analysis grows only for a refusal a program names; every rule gets a soundness witness | **Kept.** Each rule the round added was demanded by a program or by the corpus after a fix: the conditional's join, a value's tails, a threaded buffer's length, the solved form of equalities. Each has an execution or a refusal it must produce, and a planted fault that fails it |

**One item the plan did not have.** Before `bufio`, hamza asked for the state of every file in the Go
target, and then for all of them to be made current. That was the round's middle phase.

The round in three phases, counted with the method of §3.1:

| phase | commits | compiler | of which tests | Oroboros | hand declarations | survey tooling |
|---|---:|---:|---:|---:|---:|---:|
| **A. The migration's tail**: η for products, the portable layer retired, the table type, the gauntlet | 7 | +1,033 | +345 | −67 | −654 | 0 |
| **B. The Go target made honest**: the audit, the partial-file checker, every file brought current | 8 | +439 | +243 | −41 | +353 | +617 |
| **C. Programs, and what they found**: the portable `os`, `bufio`, the obligations, the equalities | 4 | +1,026 | +446 | +63 | +92 | +27 |

## 2. What this round established

**A table type has its three kinds of rule** ([tabletype](../gauntlet/results/tabletype-2026-09-29.md),
[tableelim](../gauntlet/results/tableelim-2026-09-29.md)). Table(σ) is introduced by its constructor,
eliminated by application at σ and `len` at `int`, and stored into at σ. An element not yet known is a
unification variable, `?` or `?int`, the sort ℤ with its realization left to the IR. The IR checks
each yield against the declared result by sort (`ir/claim.go`).

**A host call with m results is a tuple, by η for products** ([eta](../gauntlet/results/eta-2026-09-28.md)):
p = ⟨π₁ p, …, πₘ p⟩. So a scope's exit may be any term of the product's type, and each name in the
pattern knows what its projection Pⱼ would tell a `let`, because ⟦Pⱼ⟧ = πⱼ⟦P⟧.

**A declaration file is a theory, checked as a partial function**
([gotarget-2026-09-30](../gauntlet/results/gotarget-2026-09-30.md)). With H the hand file and G the
host's surface: dom H ⊆ dom G ∪ E, the exemptions each with a reason, and H agrees with G on their
intersection. An exemption the host declares is stale; one the file no longer uses is dead. A variadic
f : A* → B is declared as its restrictions f|Aⁿ and checked at the arity its name gives. An interface's
method set is its companion, checked against the manifest's interface lines. A host function's declared
parameter range is an **obligation** at every call, so a template's conversion can no longer truncate
silently.

**Purity is a claim about the host function, not about one reducer.** A read of state some declared
name writes is impure: `at-map`, the per-type slice reads, `Getenv`. Declared pure, each was shown
reordered past its store; each has a witness.

**A portable name is the intersection of the hosts** ([libos](../gauntlet/results/libos-2026-09-30.md)).
Σ is what every cell agrees on. `ReadFile` and `WriteFile` are get and put on a store Path ⇀ B\*, with
put-get, and the mode is not in Σ, because Java's `Files.write` takes none. One program calls every
name of Σ and runs on three hosts, and its guard fails when a name goes uncalled.

**A residual's binders are named apart along the scope chain**
([ADR 0036](decisions/0036-a-residuals-binders-are-named-apart.md)). The old invariant kept a *term*
from capturing. A *context* keyed by name, Γ ⊢ φ, is captured by the name alone. The variable
convention (Barendregt 1984, 2.1.13) along the scope chain is what makes weakening Γ by a binder sound.

**A loop variable that threads a buffer has the buffer's length, by induction**
([refinements.md §5b](spec/refinements.md)). `set` preserves length. Each variable's equation
len v = len z is verified at every back edge, under the facts at the loop's entry, and the greatest
inductive subset is kept (Houdini; Flanagan & Leino 2001). It replaced the coincidence the corpus had
been proven by.

**A name is bounded by its value's tails** ([§5a](spec/refinements.md)): through `if`, `let`, a host
call's continuation and a loop's exits. Exactly one tail is evaluated, on a path whose guards hold.

**A conditional is a join point** ([types.md §3.2](spec/types.md)): ty(a) ≤ ty(if) and ty(b) ≤ ty(if),
so its type is the larger, where the two are strictly comparable and of one representation. The
checker and the IR's typing read one relation, `Target.JoinSameRepr`.

**A program's obligations are a function of the program and the target**
([ADR 0037](decisions/0037-a-programs-obligations-are-in-its-residual.md)). O(P, T) lives in the
residual: every environment installs the contracts, a failed literal stays as its mark, deciding is one
procedure, and a consumer transparent to a mark requires the marks decided.

**Equations are a solved form** ([lengtheq](../gauntlet/results/lengtheq-2026-10-03.md)). A conjunction
of linear equations kept as a substitution is Gaussian elimination: s = t is added by rewriting
σ(s − t) and solving for a unit coefficient, never assigned.

## 3. Are we on the right track

### 3.1 The balance of effort

Counted exactly as before, and the definitions are now written down, because the script was not kept.
They were recovered by reproducing every figure of the 09-28 assessment at its revision `0d8591e`:
- **compiler**: raw lines of `.go` in `core/`, `emit/` and `ir/`, tests included;
- **Oroboros written**: lines of code in `examples/` and `lib/`, without blank lines and comments;
- **largest program** and **acceptance programs**: lines of code, the same way;
- **hand declarations**: raw lines of every `.oro` under `targets/`;
- **survey tooling**: raw lines of `gauntlet/stdlib/*.go`, `jdk/*.java` and `jsdump.mjs`;
- the analysis layers: raw lines of the files named in the table.

At `0d8591e` these give 53,563, 1,575, 164, 3,743, 409, 7,553, 2.48, 4.36, 3,530, 4,617, 3,328 and
3,031, every figure of the 09-28 table.

| | 2026-09-24 | 2026-09-28 | **2026-10-03** |
|---|---:|---:|---:|
| compiler, `core` + `emit` (the old definition) | 51,546 | 34,787 | **36,962** |
| **compiler, with `ir`** | 51,546 | 53,563 | **56,061** |
| of which tests | 17,764 | 19,000 | **20,034** |
| Oroboros written | 1,571 | 1,575 | **1,530** |
| **at the margin** | 66 : 1 | 504 : 1 | **33 : 1** on what was written; the net is −45 |
| overall | 32.8 : 1 | 34.0 : 1 | **36.6 : 1** |
| `emit` : `core` | 4.28 : 1 | 2.48 : 1 | **2.68 : 1** |
| (`emit` + `ir`) : `core` | 4.28 : 1 | 4.36 : 1 | **4.58 : 1** |
| analysis layer (`interval`, `refine`, `linear`, `monotone`) | 7,626 | 3,530 | **3,877** |
| analysis layer, extended | 9,432 | 4,617 | **4,964** |
| the IR's analysis (`interval`, `trip`, `endpoint`, `sct`, `decide`, `require`) | | 3,328 | **3,336** |
| the IR's representation choices | | 3,031 | **3,031** |
| largest program | 164 | 164 | **164** |
| survey tooling | 7,533 | 7,553 | **8,197** |
| hand declarations (`targets/*.oro`) | 3,697 | 3,743 | **3,534** |
| acceptance programs | 407 | 409 | **414** |

**Read the Oroboros figure for what it is.** Written: `lines.oro` 46, `roundtrip.oro` 17,
`filter-go.oro` 12. Deleted: the portable layer's ten programs (79), retired with their targets, and
the two `sieve-go` variants (41), which `examples/table/sieve.oro` had already replaced. The net, −45,
measures the deletion. The margin on what was written is 2,498 : 75 = **33 : 1**, against 504 : 1 and
66 : 1 the two rounds before. In phase C alone it is 1,026 : 63 = **16 : 1**, or 6.6 : 1 counting the
declarations the programs needed.

**Where the compiler grew.** +2,498 lines, 1,034 of them tests. The refinement layer grew +347: the
tails rule, the length threading and the solved form, each demanded by a program or by the corpus
after a fix. The IR's analysis grew by 8. The partial-file checker is survey tooling, +644. The hand
declarations fell by 209 because the per-type table names were deleted, while every file in the Go
target was rewritten around its algebra.

**The largest program still has not grown.** It has been `freq` at 164 lines for three assessments. The
round's program is the second line tool, not a larger one.

### 3.2 Precision and performance

**Provability is unchanged in what is left.** 1,970 of 2,015 operations and 347 of 365 loops: **45 and
18 unproven, the same 45 and 18 in the same programs** as at 09-28 (§4). The denominators fell because
deleted programs took their operations with them, and rose by `lines.oro`'s 5 and 1.

**And no proof was lost to the repairs.** Naming binders apart withdrew the proofs that rested on a
shared name; the length invariant gave every one back. The obligations now in every pipeline, and the
solved form of equalities, changed no emitted byte.

**The gauntlet is at parity** on the IR's printers, measured once against hand-written code in one run,
which closed the chain of relative measurements the migration had left.

**Compile time is flat.** Paired, serial, against each step's predecessor: the tails rule first cost
1.24× on `tree`, found by a pair when the sweep flagged nothing, and was cut to 1.04×. The obligations
change measured 1.00× on `freq` and `tree` and 1.05× on `jsonfmt`.

### 3.3 Soundness: the round's findings

Each has a witness that fails without its fix:

| found | lived since | how it was found |
|---|---|---|
| **A loop variable inherited a parameter's facts**: an index of 101 into a table of 6, accepted on Go, JavaScript, Java and x86 | the refinement layer's name-keyed facts | a probe before extending the tails rule (bufio §4) |
| **The corpus's buffer proofs rested on that leak**: a threaded buffer's length reached its loop variable through the shared name | the same | the gate, once names were apart: `freq`, the JSON pair, both sieves, `tally`, four differential cases |
| **Obligations existed only where a driver installed them**: `(digit 12)` against (int 0 9) compiled through `ir.Entry` and every `FromResidual` | ADR 0028's mechanism | a reverted change made thirteen tests accept what they must refuse (hazard) |
| **A host parameter's range was a type and nothing else**: `bits.Len32(uint32(x))` answered 32 for −1 | the host declarations | the Go target audit (gotarget §7) |
| **Reads of mutable host state were declared pure** and reduction moved them past stores: the per-type slice reads, `at-map`, the portable `Getenv` | the target-native tables | the audit, and then the portable `os` |
| **The portable `WriteFile` never compiled on JavaScript or Java** | `lib/os` | the first program to call every portable name on three hosts (libos) |
| **The product pass's loop branches never ran**: it tested for a kind nothing has | the pass | naming binders apart took away the coincidence it worked by |
| **A second equation under one key overwrote the first**: the orientation of a true precondition decided a proof | the fact layer | an elementwise add written for a reply about IR formats (lengtheq) |
| **Process-wide import sets** leaked one build's imports into the next | the printers | a test that built two programs in one process |

Three of the first four are the same failure at different scales: **a check that holds only under a
condition nobody states**. Facts were sound only while no binder shadowed; obligations existed only
where a driver installed them; a range was checked only where someone read the declaration as a
contract. Each fix made the condition structural: an invariant of the residual, a precondition a
consumer enforces, an obligation at every call.

### 3.4 Process: five things

**1. THE PROGRAM FOUND WHAT THE AUDIT DID NOT.** Phase B read every file in the Go target, one by one,
and repaired what it found. The worst fault of the round was found in phase C by a 46-line program and a
probe of the rule it needed. `tally`, `jsonfmt`, `freq`, `tree` and `u128` each found bugs in this way
before; `lines` is the sixth. **Write the program, and before extending a rule a program asks for, ask
what would make its conclusion false.** The probe that found the leak was one renamed variable.

**2. CLAIMS WRITTEN BEFORE THEY WERE CHECKED.** A header said a program's own `go.u64+` was counted, and
it was not. A program's header claimed 2²⁴ · 10¹² < 2⁶³, and the compiler refused the sum. A test was
named for "harmless shadowing" and pinned the unsound behaviour. Each statement described the code
exactly and was wrong. The lesson from 09-24, a comment is not a review, was met three more times.

**3. WITNESSES THAT COULD NOT FAIL, CAUGHT BY THEIR PLANTS.** Six times a test passed with its fault
planted: the `u64` witness twice (an `if` on `u64<` prunes nothing; the divisor is the refinement
layer's, which never reads the IR's fact), the loop-variable witness three times (a tail the rule
refuses anyway, a pure loop substituted away, an `=` guard that adds no fact), and the where-mark count
once (reduction hoists the mark out of an operator's argument). Planting first is what made each visible.

**4. THE PAIR DECIDED BOTH WAYS.** The sweep flagged nothing when the tails rule cost 1.24× on `tree`,
and it read 1.39× when the machine, not the compiler, was slower. Only the serial pair against a
worktree told each case apart. The rule stands as written in CLAUDE.md.

**5. TOOLING, AGAIN.** Heredocs collapsed `\n` in Go test strings twice more; the command checker gave
no verdict through most of a step, and the step stopped rather than spend the turn's budget; the
balance script was not kept, and its definitions had to be recovered by reproducing a published table.
They are written into §3.1 now.

## 4. What is left, read off the outcomes

**45 unproven operations, 18 unproven loops**, unchanged:

| where | operations | loops | what it is |
|---|---:|---:|---|
| `shift-div`, `limb-subdiv` (differential) | 32 | | the sweep compiles the case's exported `run`, which has no signature; the harness, which builds `main`, proves them |
| `examples/int/` (collatz, power, fib), `kara/core` | 11 | 1 | meant to be refused |
| the two wordcounts | 2 | | genuine: a count bounded by the input's length |
| `freq` | | 12 | three targets × four loops |
| `sieve-js` | | 3 | V8's word |
| `u128` (library and example) | | 2 | `decimal`'s loop: a component of `Div64`'s tuple, whose law cannot be stated |

**Named and not built, from this round's documents:**
- **A borrow that a later call ends**: `Scanner.Bytes`, `Reader.ReadSlice`, `ReadLine`, `Peek`. The
  language has write-borrows and consumes, and no borrow with that lifetime (bufio).
- **A host function's interval `where` on the IR's intervals.** ADR 0037 removed what stopped it; no
  program refuses for want of it.
- **A join across two representations**, a concrete type with its interface (types.md §3.2).
- **A boolean's definition as a fact on its arms**: `lines.oro` writes its guard at each use (bufio §3).
- **An unsatisfiable precondition is not reported**; the body is vacuously provable (lengtheq §5).
- **On Go the limb instances store limbs as `[]int`** where the class hull gives `[]uint32`, on a path
  no target takes (gotarget §14).
- **Carried from 09-28:** host callbacks, tiers 1 and 2; an `again` inside a tuple pattern's body and a
  variant out of a scope; a limb instance emitted once; a bignum operation's transfer as its declared
  type; a declared big result narrower than the program's widest; signed limbs; geometric
  accumulation; the higher-order contract gap; printing −2⁶³ on windows.

## 5. What is next

**1. A ROUND OF QUESTIONS.** hamza's, answered as before: the six type questions (08-15), the eleven
integer questions, the five questions on tables. Each answer says what the thing is (the set, the
operation, the law), where the literature settled it, what a measurement decides, and the decision it
leads to, with an ADR where it settles one. The round raised these, as a starting list for hamza to
replace or extend:
- **Is the IR's text a format others should read and write?** ADR 0006 promised the backend interface
  as a file format; the canonical printer and reader exist and round-trip exactly. Which stage is the
  contract, IR_A or IR_P? Does a printer outside the repository get the facts and the decided modes?
  What is its grammar, and how is it versioned? tinygrad asked the same question of theirs this week.
- **Concurrency**, the one question [design-direction.md §8](design-direction.md) left open. Its
  algebra first (Kahn networks, CSP, fork-join over immutable tables), then what each host gives:
  goroutines, JavaScript's event loop and workers, the JVM's threads, Win32's.
- **A borrow that a later call ends.** `bufio` named four functions that return one. What is its
  type? Regions, fractional permissions, or a scope the language already has?
- **Callbacks, tiers 1 and 2.** `Scanner.Split`, `strings.FieldsFunc`, `sort.Slice`: the next packages
  ask for them. Tier 1 is specified and not built.
- **ℤ/2ⁿ as a type**, the wall `hash/fnv`, `crc32` and `math/rand` will meet. A program can name
  `go.u64+` and get the modular sum now; the language cannot.
- **One prover or two?** The refinement layer reasons over terms with facts keyed by name; the IR's
  analysis reasons over SSA values. Three of this round's faults lived between them: the facts' leak
  across a binder, the `where` the IR could not see, and a second decider with different verdicts.
  Should the linear fragment move onto the IR?
- **What should an unsatisfiable contract do**: be vacuously true, as it is, or be refused?
- **Which Go package next, and in what order?** `bytes`, `sort`, `path`, `unicode`? The order could follow
  the standard library's own dependency graph.

**2. THE GO STANDARD LIBRARY, PACKAGE BY PACKAGE, PROGRAM FIRST.** The next package chosen in item 1, its
algebra first, then a program written in the language that needs it, then exactly the declarations that
program calls, checked against the host. `bufio` showed the order works: the program found what an audit
did not.

**3. THE WINDOWS APPLICATION — hamza's**, when he hands it over. First the measurement of what it needs
from Win32 against what is declared, callable and linkable (31.0% callable and linkable), then spec,
with an ADR per wall.

**4. THE ANALYSIS GROWS ONLY FOR A REFUSAL ONE OF THOSE PROGRAMS NAMES**, and every rule it relies on
gets a witness that it is sound and a planted fault that fails, planted before the witness is trusted.

### Deliberately not next

- **Optimising the compile.** Flat this round, measured by pairs.
- **A host `where` on the IR's intervals; a join across representations; the boolean-carried guard.**
  Each is ready to build and no program asks.
- **Reporting an unsatisfiable contract**, until item 1 says what it should do.
- **Hoisting or CSE on the IR**, which would make reachable two premises witnessed only on hand-built IR.
- **Carried from 09-28 with their reasons:** host callbacks (until item 1); a limb instance as a
  function; the element-bound and geometric-accumulation limits; per-result enforcement above the word
  and signed limbs; the higher-order contract gap; a phase-wise trip count; the windows, V8 and JVM
  unsigned rungs; dependent result ranges; `ByteOrder`'s values; records, `with` and named views.

## 6. The one sentence

The round kept its plan and wrote a program, and the program found that proofs the corpus relied on
were true by a coincidence of names; they are proven now by an induction, the obligations
exist in every pipeline, and the next round begins by asking what to build before building it.
