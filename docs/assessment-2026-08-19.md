# Assessment: four targets in, what should go into the language next

Dated snapshot after the windows target. **Deliberately not an ADR**, for the same reason
[assessment-2026-08-13](assessment-2026-08-13.md) is not one: naming a design as a decision before
measurement is the predecessor project's failure in ADR form. This is evidence, a ranking, and a
recommendation, and it stays falsifiable.

Everything in §1 is read off the code and checkable by grep.

---

## 1. Where the language actually is

**Six term kinds. Three special forms in the reader. Two reduction rules. Two parameters.**
Unchanged since [state.md](spec/state.md) — the windows target added nothing to the language.

**Three structural primitives**, on all four native targets: `let`, `if`, `loop`. Everything else
in every target file is a template.

| | |
|---|---|
| targets | 4 native (`go/`, `js/`, `java/`, `windows/`) + 3 portable + blas + 3 tutorial |
| backends | Go, JavaScript, Java, x86-64 |
| `core/` | 3,319 lines including tests |
| tests | 91 passing across `core/` and `emit/` |
| examples | 21 |
| measurements | 18 in `gauntlet/results/` |
| parity | 7 gauntlet programs on 3 hosts; sieve on 4; 2 byte-identical on Go; asm at 0.97× |

**What the language still does not have:** data structures of any kind, a product, an error model,
concurrency, pattern matching, extensionality, effect types, symbols, recursion. `string`,
`vec-f64` and `dict` are opaque handles only primitives touch.

**What is new since the last assessment:** unbounded iteration ([ADR 0015](decisions/0015-loop-and-again.md)),
a type checker on the residual, refinements with a real decision procedure, target-native
directories with no portability claim, and a fourth target proving the target-file format does not
require the host to have expressions ([ADR 0016](decisions/0016-targets-need-not-have-expressions.md)).

## 2. Are we on the right track?

**Yes on the thesis. Yes on the method. There is one place where the process has drifted, and it
is the one the project's own rules say matters most.**

### What the evidence supports

The parasite thesis has now survived the strongest test available to it. Four hosts, one of them
with no expressions, no runtime, no allocator, no formatting, and no optimiser — and the structural
set is still three, the format lost nothing, and the output is at parity with what a person writes
in that host's own assembly. That is not a small result. The likeliest failure mode for a
"parasite" design is that the abstraction quietly assumes a rich host; this one did assume one, the
assumption was found, and it cost three holes in a data file.

The method is working better than the design. Every significant thing learned in the last week was
found by **building the thing and watching it fail**, not by arguing:

- the loop primitive's "1117× cost" was the naive encoding, and the retraction produced the rule
  that found everything since — *write the thing you believe cannot be written and watch it fail*;
- the checker had no case for `loop` at all, found by writing a fourth target file;
- `cmd/build` could not build **any** program containing a loop — ADR 0015's central feature — and
  no test noticed, because no test ever built a binary;
- the language reserves four type names, found by JavaScript spelling its boolean `boolean`.

None of those was findable by reading. All were found in a week of building.

### Where it has drifted

**The gauntlet is the one fixed commitment, and it is measuring a layer we have declared retired.**
All seven gauntlet programs still run against `targets/portable-*.oro`. The native directories —
the thing the last two milestones were for — are exercised by four sieves and a hello. So the bar
and the language have come apart: new work is measured against new hand-written references written
alongside it, which is exactly the comparison [CLAUDE.md](../CLAUDE.md) warns against
("compare against best hand-written, not code shaped like our output").

That is not fatal and it is not subtle. It is debt, it is named here, and §3.4 is the repayment.

### The honest risk

The language cannot construct data. Every program is one function over opaque handles, and the
"programs" in `examples/` are the seven gauntlet shapes plus sieves. **We have not yet written
anything that a person would want to write.** The measurements are real and the parity is real,
but they are measurements of a language that has not been asked to do anything awkward. The next
genuinely informative experiment may not be a language feature at all — it may be a program big
enough to be annoying.

## 3. Candidates, ranked

Ranked by *what would be learned*, not by what is missing.

---

### 3.1 Booleans and control flow — **do this first**

`and`, `or`, `not`, `if`, `cond`, `true`, `false`, into the language rather than into each target.

**This is not a gap, it is an inconsistency that already exists.** Four pieces of evidence, all
checkable:

**(a) The reader already hardcodes two names.** `core/read.go:628` emits `Name("if")` and
`:630` emits `Name("loop")` when desugaring a `loop`'s clause chain. So `if` is *de facto* in the
language while the spec says it is a target primitive. A target that named its conditional
anything else would produce loops that do not resolve.

**(b) A boolean literal cannot be written portably.** All four targets declare `true` and `false`
under exactly that name — the only names besides the structural three that all four agree on — and
a program must still write `go.true`, `js.true`, `java.true`, `x64.true`. The universal agreement
is *evidence the name belongs to the language*, and the qualification is friction with no
information in it.

**(c) `&&` means three different things across four targets:**

| target | declared | short-circuits? |
|---|---|---|
| Go, Java | `(bool bool) bool` | yes |
| JavaScript | `(any any) any` — returns an *operand*, not a boolean | yes |
| windows (`andb`) | `(bool bool) bool` | **in a guard, not in a value** |

**(d) And on windows it means two different things *within one target*.** Measured, not argued:

```lisp
(fn (a b) (x64.andb (x64.setl a b) (x64.setg a 0)))          ; value position
(fn (a b) (if (x64.andb (x64.setl a b) (x64.setg a 0)) 1 0)) ; guard position
```

```asm
; value: both operands computed                ; guard: short-circuit
cmp rdi, rsi / setl dil / movzx edi, dil       cmp rbx, rsi / jge Lelse1
cmp r12, 0   / setg r12b / movzx r12d, r12b    cmp rbx, 0   / jle Lelse1
and r13, r12
```

That is a defect introduced by `(jump "and")`, and it is not fixable inside the windows target —
either the value form allocates a branch, or the language says which one `and` means. **A
connective whose meaning depends on where it appears is the clearest possible argument for taking
it out of the target files.**

**What it would mean, per [state.md §6](spec/state.md)'s three questions:**

1. *Independently of any target:* `and` and `or` are short-circuiting — the second operand is
   evaluated only if needed. `if`'s condition is a `bool`, not a truthy value. `true`/`false` are
   the two inhabitants of `bool`. `cond` is the clause chain that `loop` already has, standing
   alone.
2. *Per target:* Go, Java and JS already short-circuit natively. windows emits a branch, which it
   already does in guard position and would now do everywhere.
3. *Observable disagreement:* today, **yes** — a trapping second operand (`x64.idiv` by zero)
   crashes on windows and does not on Go. Which is why this is a correctness item and not a
   convenience one.

**Cost:** small. The structural set goes from 3 to 3 — `if` is already structural; `and`/`or`
become structural (they branch) and `not` stays a template. `cond` is reader sugar over `if`, and
the reader already contains the code for it.

**What would kill it:** finding that a short-circuiting `and` in *value* position costs measurably
more than `and`-the-instruction on any host. Measure before deciding, on windows, where it is a
branch versus one ALU op.

---

### 3.2 `pure` is doing two jobs, and one of them is wrong

**A correctness hole in the reduction relation**, not a feature request.

`go./`, `java./` and `x64.idiv` are all declared `pure`. All three **trap on a zero divisor** —
Go panics, Java throws, x86 raises #DE and kills the process. [ADR 0010](decisions/0010-effects-as-structural-rules.md)
says purity is the licence to use the structural rules: a pure term may be **copied, dropped and
moved**. Dropping a term that would have trapped changes what the program does.

So `pure` currently means *no side effect*, and β is treating it as *total*. Those are different
properties and only one of them licenses weakening.

**The obvious repair is to reuse machinery that already exists — and it does not fit.** `aindex`
carries `(where (and (<= 0 i) (< i (alen v))))` and the obligation is discharged at every call site
([refinements.md](spec/refinements.md)). Division carries nothing, and giving it
`(where (!= d 0))` runs straight into the fragment's shape.

`obligation` in `emit/linear.go` recognises exactly six predicates: `and`, `lt`, `le`, `gt`, `ge`,
`eq`. Not `or`, not `not`, not `ne` — and that is not an oversight. **The fragment is conjunctions
of linear inequalities, and `d ≠ 0` is a disjunction** (`d < 0 ∨ d > 0`). It is the first
obligation in this project that is not a conjunction of `≤` goals, and there is no way to write it
in what exists.

So the item is more interesting than "add a `where`", and it splits cleanly:

- **The narrow question.** Does the fragment grow to handle disjunction? That is a decision
  procedure change, and it should be argued against the deliberately incomplete one already there
  — an undischarged obligation is *reported, never assumed*, and reporting `d ≠ 0` unproven at
  every division would be noise on programs that never divide by a variable.
- **The wide question.** Is `pure` two bits? Indexing and division are two instances of *pure but
  partial*. A third would settle it.

Either way the correctness statement stands on its own and should go into
[effects.md](spec/effects.md) now: **`pure` currently licenses weakening for terms that can trap**,
and that contradicts [ADR 0010](decisions/0010-effects-as-structural-rules.md) as written.

**What it would prove:** whether the refinement machinery, built for bounds, is the general answer
to partiality — or whether bounds were the easy case *because* they are conjunctive.

---

### 3.3 A product type — **measure, do not design**

**Five independent demands, and the fifth came from hardware:**

1. `fold-range2`'s pair of accumulators (the original).
2. Go's `(int, error)` — `fmt.Fprintf` and `fmt.Sscan` are declared for their first result only,
   so the error is unreachable (`targets/go/fmt.oro`).
3. Java's `Map.Entry`, and every iteration protocol built on it.
4. JavaScript destructuring, which every modern JS API returns into.
5. **One `idiv` produces quotient *and* remainder.** We declare two primitives and divide twice.

There is a sixth in disguise: a string on the windows target is a bare pointer, so `x64.strlen`
runs a loop at runtime over a literal whose length the compiler wrote into the data section three
lines earlier. A string as `(pointer, length)` is a product.

**And this is exactly where the project is most likely to kill itself.** CLAUDE.md's hardest
constraint is *never introduce boxing or hidden allocation into the core* — it is what killed the
predecessor. A product must lower to zero cost on **all four**: a Go struct is free, two registers
in assembly are free, and on **Java and JavaScript an object is an allocation** unless the host's
escape analysis removes it. That is the whole question, and it is a measurement, not an argument.

**Recommended next step is not a design.** It is: write the two-return case by hand in Go, JS,
Java and asm, both boxed and unboxed, and measure. If the JVM and V8 scalar-replace it reliably,
the product is cheap and five demands get paid at once. If they do not, we have learned the price
and the answer stays no — and *that* is worth knowing before anything is built on top of it.

This is the same move [ADR 0008](decisions/0008-measurement-over-principle.md) exists for, and the
same move that refuted four inferences in the first baseline run.

---

### 3.4 Move the gauntlet off the portable layer — **the debt**

Not a language change. The largest outstanding risk in the project.

`portable-{go,js,java}.oro` were declared shelved. All seven gauntlet programs still run on them.
Until they move, the **one fixed commitment** is measuring a layer we no longer intend to build,
and `targets/go/`, `targets/js/`, `targets/java/` have exactly one program between them.

Doing it would also answer a question nothing has asked yet: whether the seven gauntlet shapes are
expressible in the native targets *at all*. Word count needs a dictionary; the native Go target
declares `make-map`, `at-map`, `set-map`, `delete` — one instantiation, `map[string]int`. That is
the type-constructor limitation from [target-native.md](spec/target-native.md) meeting the actual
bar for the first time, and it will either hold or it will not.

**This is the cheapest way to be surprised**, and it should probably happen before 3.3 rather than
after.

---

### 3.5 Below the line, with reasons

**Common-subexpression elimination.** New information from windows: the sieve computes `i*i`
twice, and on the first three hosts the host compiler removed it and we never knew. Real, and
**unpriced** — the measured cost is inside the noise floor. Do not build it until something
measures. Worth recording that it is now a *compiler* responsibility on at least one target.

**An error model** (open question 2). Four hosts, four idioms, and windows sharpens it:
`GetLastError` is a global, `ReadFile` returns its count through an out-parameter, and
`CreateFileA` could not be declared at all. Blocked on 3.3 — a result value is a product.

**The integer-range refinement hole** ([arithmetic.md §4](spec/arithmetic.md)). Named in CLAUDE.md,
still open, and the second of the two holes shaped like a refinement. Cheaper after 3.2, since
both want the same fragment extended.

**A frame-size declaration for targets.** `CreateFileA` takes seven arguments; the emitter reserves
room for six; a target file cannot ask for more. One target, one primitive, no program blocked.
Fix it when a second instance appears.

**Concurrency, memory model, module format.** Unchanged and correctly deferred. Note only that
"deferred" for concurrency cannot mean "forever", since goroutines are a main reason to target Go.

## 4. Recommendation

In order:

1. **Booleans and control flow into the language** (3.1). It closes a defect that exists today,
   it removes friction with no information in it, and the reader is already half-written.
2. **Record the `pure`-versus-total hole in effects.md** (3.2). The repair is not cheap — `d ≠ 0`
   is a disjunction and the fragment is conjunctive — but the statement is true today and belongs
   written down before anything is built on it.
3. **Move the gauntlet onto the native targets** (3.4). The cheapest available surprise, and it
   repays the one piece of process debt.
4. **Measure a product before designing one** (3.3). Hand-written, four hosts, boxed and unboxed.

And one thing that is not on the list and should be considered: **write a program somebody would
want to run.** Everything measured so far is a benchmark shape. The language has never been asked
to do anything awkward, and that is the least-tested claim in the project.
