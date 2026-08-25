# `match`

**Status**: built, 2026-08-22. Reader sugar. Zero reduction rules, zero term kinds.

`match` is `loop`. It desugars to one in the reader, and that is the entire implementation —
`core/read.go`'s `readMatch`, about 120 lines, no change to the reducer and no change to any
backend. It joins `let`, `seq`, `and`, `or`, `not` and `cond` as sugar that erases before
anything else sees it.

The argument is in [type-algebra.md §5](../type-algebra.md); this is what got built.

---

## 1. Form

```lisp
(match (e₁ … eₙ)
  p₁₁ … p₁ₙ            body₁
  p₂₁ … p₂ₙ (when c)   body₂
  …
  else                 bodyₖ)
```

*n* scrutinees, so every clause is *n* patterns, an optional `(when c)`, and a body. `else` is a
clause of its own — one word and a body — and must be last.

A **list** of scrutinees, not one, for the same reason `loop` has n variables and no product
([ADR 0015](../decisions/0015-loop-and-again.md)): a clause needs no tuple built and then taken
apart.

`(again a₁ … aₙ)` in a clause body jumps back with new scrutinees. It is the loop's `again`,
so [ADR 0014](../decisions/0014-recursion-is-not-in-the-language.md) is untouched — a jump, not
a call.

## 2. Patterns

| | |
|---|---|
| `_` | wildcard — matches, binds nothing |
| *name* | binds the scrutinee under that name |
| `true` / `false` | tests the scrutinee, which is already a `bool` |
| an integer | tests it with `=` |

**A name pattern is a rename, not a `let`.** The pattern variable is another name for the loop
variable, and renaming needs no binder. It is also what lets `(when …)` see the bound names — a
`let` wrapping only the body could not.

The rename is safe without capture analysis, and the reason is worth stating: by the time
`readMatch` runs, every inner `fn` has been through `Fn`, which **closed its body**, so an
occurrence bound by a nested binder is a `KBound`, not a `KName`. Only genuinely free names are
left to rename.

**A `bool` pattern needs no equality at all** — the scrutinee *is* the test, because `if` is
`bool`'s eliminator ([ADR 0017](../decisions/0017-booleans-are-in-the-language.md)).

**Flat patterns**, because our data is flat. No nesting, no constructor patterns yet — those
arrive with the sum, and the shape they will take is one more pattern kind, not a new construct.

### What is refused, and why

**Float and string patterns.** The language has no portable equality: `==` is target-native on
all four hosts, `length` fails the agreement test for strings, and a float pattern would inherit
IEEE's NaN — which is not the equivalence relation a pattern needs. `=` is integer equality
and nothing else.

**A repeated name in one clause.** `x x` would be an equality test written as a pattern. Erlang
allows it; ML does not; we do not, because it would smuggle in an equality the language has no
portable spelling for.

**`again` under an `if` in a clause body.** ADR 0015's rule reaches through the sugar. See §3.

## 3. `when`, and why building this produced it

`when` was not in the design. It exists because the first non-trivial test program did not
compile:

```lisp
(match (s i)
  _ k  (if (go.>= k n) k (again s (go.+ k 1)))     ; refused
  else 0)
```

> `again` may be a clause body, or sit under a `let`, but not under an `if` — write another
> clause instead, so the clause list stays the loop's whole control flow.

ADR 0015 is right and the program was wrong. But it exposes something: **without `when`, `match`
is strictly weaker than the `loop` it desugars to.** A `loop` clause takes an arbitrary boolean;
a `match` clause took only patterns, and a condition patterns cannot express had nowhere to go —
not the guard, because it is not a pattern, and not the body, because ADR 0015 forbids it there.

`(when c)` is that condition, and `c` sees the names the patterns bound. With patterns that also
test, the guard is the **conjunction**, spelled the way `and` desugars — `(if pattern c false)` —
so nothing new reaches the reducer. With patterns that test nothing, the `when` **is** the guard,
rather than `(if true c false)`, which is the same thing spelled worse.

## 4. A bare-name scrutinee becomes the loop variable

This is the subtlest thing in the desugaring and it is not cosmetic.

`(match (s i) …)` initialises the loop from `s` and `i`. With **fresh** loop variables, a clause
body reading `i` would see the *outer* `i` — the value the loop started from — while `again`
advanced a hidden `#m1`. Every iteration after the first would read a stale value, and the
program would look right.

So a scrutinee that is a bare name becomes the loop variable **under that same name**, shadowing
the outer binding. That is also what the state-machine reading means: `s` and `i` *are* the
state. Anything else — a literal, a call, a repeated name — gets a fresh `#m` name, which no
source term can contain because `#` is not `isIdentStart`.

**This found a five-month-old bug in the JavaScript backend.** `(loop ((n n)) …)` inside
`function f(n)` emitted `let n = n;` — a `SyntaxError`, so the module did not parse at all. Go
and Java seeded their fresh-name set from the parameters; JS did not, and no program had ever
written a loop variable shadowing a parameter. `match` makes it the common case. x86 needed no
fix, because registers are allocated positionally and locals have no names at all.

## 5. What it emits

Nothing new. `match` reaches all four backends as a `loop`, which they already emit at parity —
`for {}` on Go and Java, `for (;;)` with a tail `return` on JavaScript
([native-js-2026-08-20](../../gauntlet/results/native-js-2026-08-20.md)), a label and a
conditional jump on x86.

A two-state machine counting 1-runs in an integer's bits:

```lisp
(def runs (fn (n)
  (match (0 n 0)
    _ 0 c                            c
    0 v c (when (= (go.% v 2) 1)) (again 1 (go./ v 2) (go.+ c 1))
    _ v c (when (= (go.% v 2) 1)) (again 1 (go./ v 2) c)
    _ v c                            (again 0 (go./ v 2) c)
    else                             0)))
```

```go
func RRuns(n int) int {
	var _m0 int = 0
	var n2 int = n
	var _m2 int = 0
	for {
		if n2 == 0 {
			break
		}
		if (_m0 == 0) && ((n2 % 2) == 1) {
			_m0, n2, _m2 = 1, (n2 / 2), (_m2 + 1)
			continue
		}
		if (n2 % 2) == 1 {
			_m0, n2 = 1, (n2 / 2)
			continue
		}
		_m0, n2 = 0, (n2 / 2)
		continue
	}
	return _m2
}
```

Note what the emitter did without being asked: a clause that does not change `_m2` does not
assign it.

## 5b. A `loop` that never repeats is not a loop

`match` desugars to `loop`, so a `match` used as a plain conditional was emitting a **loop that
always breaks** — `for { … break }` plus a result variable, on every backend, where the
hand-written form is an `if`:

```go
var r1 int
for {
	if (t2 == 0) { r1 = (p2 + 1); break }
	r1 = 0
	break
}
return r1
```

So `match` was charging for iteration it did not use. The fix is one observation: **when no clause
body contains an `again`, the `loop` name is dropped and what is left is a β-redex** — which is
exactly what `let` is, since `(let e k)` reads as `(k e)`. `(match (t) 0 1 else 2)` now reduces to
`(if (= t 0) 1 2)`.

It applies to a hand-written `loop` as well, because the rule is `loop`'s rather than `match`'s.
Measured across **188 residuals** — every example on all four targets — it changes **nothing**,
because no existing program had written a loop that never repeats.

**This is what makes "is `case` needed, isn't `match` enough?" a surface question rather than a cost
question**: for the same test the two now produce the *same* residual. What still separates them is
[sums.md §7](sums.md).

## 6. `=`, and why it is not `==` and not `tag=`

`=` is the language's equality, injected into every target like `if`, `let` and `loop`. It is
**not declarable** — the same rule as booleans
([ADR 0017](../decisions/0017-booleans-are-in-the-language.md)).

Each backend finds the host's own and reuses it: `==` on Go and Java, `===` on JavaScript,
`sete` on x86. Nothing is lowered further than the target requires, and a target author cannot
spell it differently or forget it.

**It is integer equality only.** Floats are excluded because NaN is not an equivalence relation;
strings because no two of the four targets agree on comparing them. For a host's own, name it —
`go.==`, `js.===` — which is target-native and carries no portability claim.

**Not `==`** — and the reason first recorded here was wrong.

> **Correction, 2026-08-25.** This section said `==` was rejected because *"on JavaScript that name
> is already taken"*. It is not. `tg.Prims` is keyed by the **qualified** name, so `js.==` and a
> bare `==` are different keys and coexist exactly as `=` and `go.==` do today. The claim was
> asserted from the shape of the code rather than checked against it, which is the failure this
> repository's measurements exist to catch. There was no collision.

What survives is weaker, and is stated as weaker: **legibility**. A program holding both `==`
(strict, the language's) and `js.==` (loose, the host's) spells two different operations almost
identically, and `=` cannot be misread that way. `=` is also equality in Scheme, Clojure, SQL and
mathematics.

**Not `tag=`, which is what it was first built as.** Two arguments killed it. A name should say
what an operation **is**, not what it is for: `(when (= (go.% v 2) 1))` is not comparing a tag,
and this project's own naming rule already says so — `values` not `multi-return`, and `alloc`
beat `materialize` for saying it in a word everyone has. And the honesty a narrow name was
buying is better bought by **the refusal**, which can explain itself where a name cannot:

```
in argument 1 of `=`: a is f64, but int is required here.
`=` is the language's equality and it is integer equality only. Floats are excluded
because NaN is not an equivalence relation, and strings because no two of the four
targets agree on comparing them. For a host's own equality, name it: `go.==`,
`js.===`, `java.==`, `x64.sete` — that is target-native and carries no portability claim
```

The one argument the other way — that a tag comparison and an integer comparison will look
identical once sums land — is the type checker's job, and ours runs on the residual.

## 7. What it is not

It is **not** ML's `case`. ML's is an expression with no way back to the top; ours has `again`,
which makes it the state machine a parser, an event loop and a protocol handler already are.
It is Erlang's clause-head shape with a jump instead of a tail call.

It is **not** a new construct in the core. [state.md](state.md)'s counts are unchanged: seven
term kinds, three reduction rules.
