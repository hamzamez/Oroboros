# Integers

**Status, 2026-08-20. Specification. §5 built while writing it; §11 and the rest of §13 are not.** Written before the code that is not yet
written, which is the order [strings.md](strings.md) exists to enforce and the order booleans
followed.

[data-model.md §7](data-model.md) listed eleven questions that must be settled before any integer
is implemented, on the grounds that choosing a representation drags all of them behind it and each
has hosts that disagree. This settles them.

**Everything below is measured on all four targets.** Where the hosts agree, the agreement is a
measurement and not a reading of four specifications — this project has been wrong about that
before ([ADR 0008](../decisions/0008-measurement-over-principle.md)). The x86 column was produced
by our own compiler.

---

## 0a. The operators are the language's now

Everything below was written when `=` was the only integer operator the language
owned and every other one was target-native: `(+ 1 2)` was *"not bound"*, and
each answer here was really an answer about `go.+`. `+ - * / % < <= > >=` are
the language's as of 2026-08-31, found per target the way `=` always was — so
these eleven answers are now the LANGUAGE's guarantees rather than four parallel
host facts.

**One host needed a declaration rather than a rename.** JavaScript's `/` is
float division — `7 / 2` is 3.5 — so `targets/js` declares `idiv` as
`Math.trunc(a / b)` and the language's `/` resolves to it there. `Math.trunc`
and not `| 0`, because the bitwise form coerces to int32 and would silently wrap
every value past 2³¹, well inside the window.

**Bitwise and shifts are NOT promoted**, and §12's third question is why: V8
coerces both operands to int32 for `& | ^ << >>` (uint32 for `>>>`), so
`(2³²) & -1` is **0** there and 4294967296 on Go, Java and x86. That is an
observable disagreement *inside* the window, which is exactly what makes a
construct Tier 2. Promoting them conditionally, when a declared range fits
int32, is a real design and is not made.

## 0. What does not change

[ADR 0012](../decisions/0012-portable-integer-range.md) stands, and this specification is mostly
its consequences worked out:

> An `int` is a mathematical integer. Arithmetic on it is exact. The portable range is
> −(2⁵³−1) … 2⁵³−1. Outside that range the behaviour is the target's, differs between targets, and
> carries no portability claim.

The window is JavaScript's, and §7 shows it pays for itself somewhere unexpected.

## 1. Representation

| | Go | JavaScript | Java | x86-64 |
|---|---|---|---|---|
| | `int` (64-bit) | `number` (binary64) | `long` | `qword` |

**Decision: the target's machine word, and no second representation.** A bignum is a separate
decision with its own ADR, and this specification is deliberately writable without it.

**Why not decide the bignum now.** Because its price is a 3× spread —
[overflow-2026-08-19](../../gauntlet/results/overflow-2026-08-19.md) measured the fixnum check at
1.31× on Java, ~1× on x86, 2.61× on Go and 3.74× on JavaScript — and
[data-model.md §0](data-model.md) says a name whose price differs by that much across targets does
not mean the same thing on all of them. Deciding it needs the *product* type first, because a
usable bignum keeps its small case inline in the value, and `math/big` without one is 38.9× even
used perfectly.

## 2. Overflow

**Decision: outside the window there is no portable meaning, and the compiler can be asked to say
so at run time.**

- **By default**, the behaviour is the target's — wrapping on Go, Java and x86, silent precision
  loss on JavaScript. That is ADR 0012 unchanged.
- **`-checked`** rewrites every operation the compiler cannot prove stays in the window to the
  target's declared checked form, which **traps**
  ([selection-2026-08-19](../../gauntlet/results/selection-2026-08-19.md)).

The compiler can prove a great deal: **100% of integer operations in the corpus once one range is
declared, 54% with nothing declared**
([sct-2026-08-19](../../gauntlet/results/sct-2026-08-19.md)).

**Why not trap by default.** Three reasons, and the first is decisive.
[checkcost-2026-08-19](../../gauntlet/results/checkcost-2026-08-19.md) measures the cost at
**1.23× to 4.54×**, and a 4.54× regression for a program that asked for nothing breaches
requirement 5. Second, **JavaScript declares no checked form**, so trapping by default makes the
same program trap on three targets and go silently wrong on the fourth — a *worse* divergence than
the one it was meant to fix. Third, it reverses a written ADR, and doing that accidentally is what
[assessment-2026-08-20 §2](../assessment-2026-08-20.md) exists to record.

**Why not promote to a bignum.** §1.

## 3. Division: rounding

Measured, `-7 / 2` and `7 / -2`:

| | Go | JavaScript | Java | x86-64 |
|---|---|---|---|---|
| `-7 / 2` | −3 | −3 | −3 | **−3** |
| `7 / -2` | −3 | −3 | −3 | **−3** |

**Decision: truncation toward zero.** All four hosts agree, so this costs nothing anywhere.

**Why not floor division**, which Python, Haskell's `div` and mathematics generally prefer, and
which has the better property that the remainder is always non-negative? Because **no target does
it**, so every division would need a correction — a comparison and an adjust — on all four hosts, to
buy a convention that only matters for negative operands. Knuth prefers floored division in TAOCP
and he is arguing about mathematics; we are arguing about four instruction sets that all made the
other choice.

If a program wants floored division it is four lines and it says so.

## 4. Division: the sign of the remainder

| | Go | JavaScript | Java | x86-64 |
|---|---|---|---|---|
| `-7 % 2` | −1 | −1 | −1 | **−1** |
| `7 % -2` | 1 | 1 | 1 | **1** |

**Decision: the remainder takes the sign of the dividend**, which is what truncation forces, and
the identity holds on every target:

```
(a / b) * b + a % b  ==  a
```

Verified on Go, Java and x86. It is the same choice as C99, Java, Go and every mainstream
instruction set, and it is not independent of §3 — pick truncation and this follows.

## 5. Division by zero

**This is the sharpest divergence in the whole numeric story, and it does not go away.**

| | Go | JavaScript | Java | x86-64 |
|---|---|---|---|---|
| `1 / 0` | panic | **`Infinity`** | `ArithmeticException` | #DE, process dies |
| `1 % 0` | panic | **`NaN`** | `ArithmeticException` | #DE |

Three targets stop the program. JavaScript **produces a number and keeps going**, and that number
is not an integer.

**Decision: a zero divisor is a precondition, not a behaviour.**

```lisp
(prim / ((a int) (b int)) int expr "%s / %s" pure (where (!= b 0)))
```

An undischarged obligation is **reported, never assumed** — the rule
[refinements.md](refinements.md) already runs on. That means a program dividing by a variable gets
a diagnostic until it proves the divisor non-zero, exactly as one indexing an array does.

**Why not specify a value.** Because to specify one is to emulate it: JavaScript would need a
branch on every division to trap, and the other three would need a branch to produce `Infinity` —
which is not even representable in an `int`. Both directions pay everywhere for a case no correct
program has.

**Why not leave it undefined.** Because it is *checkable*, and the machinery to check it exists.
`d ≠ 0` is a **disjunction** — `d < 0 ∨ d > 0` — and the fragment in `emit/linear.go` is
conjunctions of linear inequalities. It is discharged by **case split**: prove either disjunct
([types-direction.md §6.7](../types-direction.md)). That is cheap and complete for this shape.

**And the JavaScript hazard is named rather than hidden.** On that target a division by zero is
silently wrong, and no check we can afford will find it. A program that divides by a value it has
not bounded is not portable to JavaScript, and covering should eventually say so.

## 5a. Negative zero, which is the divergence that was actually live

§5's hazard turned out to be closed. This one was not, and it arrived with the fix for a different
problem — JavaScript's `/` being float division, so the language's `/` lowers to `Math.trunc(a / b)`
there.

| | Go | JavaScript | Java | x86-64 |
|---|---|---|---|---|
| `-1 / 2` | `0` | **`-0`** | `0` | `0` |
| `-2 % 2` | `0` | **`-0`** | `0` | `0` |

Wherever the true result is zero and the dividend is negative, V8 yields negative zero — from
`Math.trunc` and from `%` alike.

**It hides almost everywhere**, which is what makes it dangerous rather than merely untidy:
`-0 === 0` is true, `String(-0)` is `"0"`, and every arithmetic operation normalises it, so
`-0 + 5` is `5`. The one thing that shows it is printing the value unmodified, where `console.log`
gives `-0`. **The same program printed `0` on three targets and `-0` on one.**

**Decision: normalise at the target, with `+ 0`.** `targets/js` declares

```lisp
(prim idiv ((a any) (b any)) any expr "(Math.trunc(%s / %s) + 0)" pure (where (!= b 0)))
(prim irem ((a any) (b any)) any expr "((%s %% %s) + 0)"          pure (where (!= b 0)))
```

`+ 0` maps `-0` to `0` and is the identity on every other value, negatives included — `-1 + 0` is
`-1`. One addition, no branch, and it is exact for every integer in the window.

The host's own `js./` and `js.%` are left alone: a program that names them has chosen JavaScript and
should get JavaScript. It is the LANGUAGE's `/` and `%` that must agree on four hosts.

**The differential case has to print the value raw**, and that is the part worth keeping: a case
that folded the quotient into a larger answer would pass with the bug present, because the addition
would normalise the sign away. `gauntlet/differential/cases/negative-zero.oro` returns the division
itself, and it fails with the `+ 0` removed.

## 6. Comparing an `int` with an `f64`

| | Go | JavaScript | Java | x86-64 |
|---|---|---|---|---|
| `1 == 1.0` | does not compile | **`true`** | does not compile (needs a cast) | different registers |

**Decision: refused. There is no mixed comparison and no implicit conversion.**

The checker already enforces it — `int` and `f64` are distinct type names and `(== 1 1.0)` is a
type error — and this specification is confirming that rather than changing it.

**Why not allow it, since JavaScript does?** Because JavaScript is the *only* target that can, and
allowing it would mean a portable name whose meaning on three of four targets is "will not compile".
That is not a portability claim, it is a compile error waiting for a change of target.

**Why not insert a conversion.** Because the conversion is §7 or §8, and §8 has a precondition. An
implicit conversion would hide a precondition, which is the one thing
[data-model.md §0](data-model.md) forbids.

## 7. `int` → `f64`

| | Go | JavaScript | Java | x86-64 |
|---|---|---|---|---|
| `2⁵³−1` round-trips exactly | **yes** | **yes** | **yes** | yes |

**Decision: total and exact for every portable `int`. It needs no precondition.**

This is the window paying for itself somewhere nobody chose it for. 2⁵³ is where a binary64 stops
representing consecutive integers, and the portable range was set to JavaScript's number type —
so **the range of a portable `int` is exactly the range in which `int → f64` is lossless.** The
constraint that looked like a concession to the weakest host turns out to make the one conversion
everybody needs free and total on all four.

## 8. `f64` → `int`

Measured, and this is where the hosts fall apart:

| | Go | JavaScript | Java | x86-64 |
|---|---|---|---|---|
| `1.9` | 1 | 1 | 1 | 1 |
| `-1.9` | −1 | −1 | −1 | −1 |
| `NaN` | **−9223372036854775808** | **`NaN`** | **0** | 0x8000…0 |
| `1e300` | **−9223372036854775808** | **`1e300`** | **9223372036854775807** | 0x8000…0 |

Three hosts, three answers, and JavaScript does not return an integer at all.

**Decision: truncation toward zero, with a precondition that the value is finite and inside the
window.**

```lisp
(prim int-of-f64 ((x f64)) int expr "int64(%s)" pure
  (where (and (<= -9007199254740991.0 x) (<= x 9007199254740991.0))))
```

In the domain, all four agree exactly. Outside it, they disagree three ways and the specification
claims nothing.

**Why not clamp**, as Java does? Because Go and x86 do not, so clamping costs two comparisons and a
select on the two targets that are fastest at everything else — to make a wrong answer uniform
rather than to prevent one.

**Why not define NaN as zero**, as Java does? Same reason, and worse: it makes a *type error*
silently produce a plausible number.

## 9. Equality

**Decision:**

- On `int`, equality is mathematical equality. All four hosts agree inside the window; outside it
  the values are not portable anyway (§2).
- On `f64`, equality is **IEEE-754** — because every host's `==` is. So `NaN == NaN` is false, float
  equality is not reflexive and not an equivalence relation, and any refinement mentioning it falls
  outside the decidable fragment and is propagated as an opaque atom. That is already recorded in
  [ADR 0012](../decisions/0012-portable-integer-range.md) and is restated here because it is a
  consequence people forget.
- **Never between the two** (§6).

## 10. Ordering

**Decision: `int` is totally ordered; `f64` is not.**

IEEE-754 comparison is a partial order — `NaN` is unordered with everything including itself, so
exactly one of `a < b`, `a == b`, `a > b` is *not* guaranteed to hold. Every host is like this and
nothing can be done about it cheaply.

The practical consequence, and the reason this is worth writing down: **a sort or a `min`/`max`
over `f64` has no portable meaning in the presence of NaN**, and any such library must say what it
does. On `int` there is no such problem and none of this applies.

## 11. Constant folding

Not built, and this is the specification for building it.

**Decision: fold when every operand is a literal AND the result is inside the window. Otherwise
leave the term alone.**

The condition is what makes it sound, and it is exactly
[ADR 0009](../decisions/0009-staging-preserves-results.md)'s rule applied to integers:

> Staging must not change an answer.

Inside the window, arbitrary-precision arithmetic and every target's machine arithmetic agree
exactly — that is what the window *means*. Outside it they do not: folding `2⁶² + 2⁶²` at arbitrary
precision gives 2⁶³ where an `int64` wraps, which is `0.1 + 0.2` in integer clothing and is the bug
ADR 0009 exists to prevent.

**The target must declare which primitive is which operation.** The language has no arithmetic;
`go.+` means *Go's* `+`, and only the target knows that. So folding is opt-in per primitive, the
way `checked` is:

```lisp
(prim + (int int) int expr "%s + %s" pure (fold add) (checked add-exact))
```

**Why not fold by name.** `emit/linear.go` already maps `+` → `add` for the decision procedure, and
that is a heuristic which is fine for an *analysis* — being wrong makes it prove less. Folding is
not an analysis; being wrong changes the answer. A heuristic must not be load-bearing for
correctness.

**Why bother.** Because it is visible in emitted code today. A three-layer byte-accessor library
fuses to one expression and leaves this behind:

```go
var i int = (0 + 2)
return (((((b[0]) << 8) | (b[(0 + 1)])) << 16) | (((b[i]) << 8) | (b[(i + 1)])))
```

Go's compiler folds `0 + 1`; the windows target has nothing that will. And the unfolded `(0 + 2)`
is not *duplicable*, so call-by-need bound it to a name — a `let` that exists only because a
constant was not a constant.

**And it does not contradict "no primitive is ever evaluated"** as much as it looks. That rule
([state.md §3](state.md)) is about the *language* not knowing arithmetic, and it still will not:
folding happens only where a target has declared what its primitive means, which is the same
mechanism as every other thing a target declares.

---

## 12. The three questions

[state.md §6](state.md) demands these of every addition.

**1. What does it mean, independently of any target?** An `int` is a mathematical integer.
Arithmetic is exact within ±(2⁵³−1). Division truncates toward zero and the remainder takes the
dividend's sign. `int → f64` is exact; `f64 → int` truncates within a precondition. Equality and
ordering on `int` are mathematical and total; on `f64` they are IEEE's.

**2. What does each target do with it, and do they agree?** Measured throughout. They agree on
everything in the window: arithmetic, division rounding, remainder sign, `int → f64`, and
truncation of an in-domain float.

**3. If they disagree, is the disagreement observable?** Yes, in exactly three places, and each is
handled by refusing to claim rather than by emulating:

| | handled by |
|---|---|
| outside the window | no portable claim; `-checked` reports it at run time (§2) |
| division by zero | a precondition, and a named JavaScript hazard (§5) |
| `f64 → int` out of domain | a precondition (§8) |

Strings passed this test "only by having almost no operations". Integers pass it by having a
**window**, and by turning the three edges into obligations the compiler can check.

## 13. What this specification asks to be built

In order, and none of it is a language change:

1. ~~**The `!=` discharge**~~ — **built**. A disequality is a two-case split against the fragment
   that already existed. Writing it found two further gaps, both of which had been silently costing
   the checker everything on the native targets: **a plain `if` did not assume its guard at all**
   (only a loop's clause chain did), and **`negate` did not resolve a target's own spelling**, so
   `go.<` negated to nothing and the second half of Hoare logic never fired where programs live.
2. ~~**`(where …)` on division**~~ — **built**, on all four native targets. `(if (== b 0) 0 (/ a b))`
   discharges; `(/ a b)` with nothing known is refused. **`f64 → int` is not yet declared.**
3. ~~**`(fold OP)`**, per §11~~ — **built**, and it needed the operators to be
   the LANGUAGE's first. Folding `go.+` would have meant assuming one host's
   semantics; `+` has semantics this document verified on all four. Two side
   conditions, both ADR 0009: a result outside the portable window is **not**
   folded, because compile time is Go's `int64` and run time on JavaScript is a
   binary64 exact only to ±(2⁵³−1) — and leaving the operation alone is what
   lets the overflow analysis report it against what the programmer wrote.
   Division by zero is **not** folded either: it is a precondition, so the
   refinement layer reports it with the call site rather than the compiler
   panicking. Multiplication is checked by dividing back, since two in-window
   operands can produce a product `int64` itself cannot hold.
4. ~~**Covering for the JavaScript division hazard**~~ — **already closed, and more strongly than
   this asked for.** It wanted a program dividing by an unbounded value to be *told it is not
   portable* on JavaScript. What actually happens is that the program is **refused on every
   target**, because the precondition is discharged at the call site and an unproven one is an
   error rather than a note. Verified across the boundary:

   | what is known about `n` in `(/ a n)` | |
   |---|---|
   | nothing | **refused** — *"`/` requires `(!= n 0)`, which does not follow"* |
   | `(where (<= 0 n))` | **refused** — `n` may still be 0, which is the right answer |
   | `(where (< 0 n))` | accepted |
   | `(where (!= n 0))` | accepted |
   | `(if (= n 0) 0 (/ a n))` | accepted — the guard discharges it |

   So `1 / 0` cannot reach a backend, and JavaScript's `Infinity` is unreachable rather than
   merely reported. Portability does not need computing here because the divergent state is one
   the program has already been refused for being able to enter.

5. ~~**Negative zero on JavaScript**~~ — **found and closed 2026-08-31**, §5a.

What it does **not** ask for: a bignum, a second integer type, fixed-width types, or any change to
the reader, the reducer, or the term language.
