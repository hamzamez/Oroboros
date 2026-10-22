# 0071 — Work stays where it was written: β counts evaluations, not occurrences

Date: 2026-10-22
Status: Accepted (hamza, 2026-10-22: "go ahead with item 1, always mathematical and algebraic", the
10-21 plan's first item, assessment-2026-10-21 §5). Refines [ADR 0010](0010-effects-as-structural-rules.md)'s
side condition on β; changes no answer, only where a pure value is computed.

## Context

β is call-by-need: `let x = e in E[x]` substitutes e for x when x occurs at most once, and binds e
otherwise (unless e is a value, which it copies). For an impure e the rule is withheld (ADR 0010,
effects.md §4). For a pure e it counted **occurrences in the text**. One occurrence inside a loop's
body is one occurrence, so e was substituted there and ran on every iteration, where the program had
written it once, before the loop.

effects.md §2 named the hazard in 2026-08 ("the argument moves into a loop body … `occurrences`
counts syntactically, with no notion of execution context") and closed it for effects only. The 10-21
round found it again, by reading `shape`'s Go. `shape.oro` works around it in its source: it binds the
separator's bytes and reads them twice "so computed once".

## Decision

1. **β counts evaluations.** The count of x in E, N_x(E), is in the semiring M = {0, 1, ω}. A use is 1;
   sequence and branches add (1 + 1 = ω; a branch's copy is code β declines to duplicate, as before);
   and a λ multiplies its body by its **shot** σ:
   - σ = 1 for the operator of a β-redex; the continuation of `let`, `build` and `build-map`; and the
     continuation of an elimination `(e (fn x̄ b))`, e data or a host call in progress: a product's or a
     sum's eliminator (data.md §3.2), a host call's continuation (ADR 0027), a cell's marked elimination
     (ADR 0047), each applying its continuation once;
   - σ = ω for `loop`'s body (once per iteration), `table`'s rule (once per element read), and a λ
     handed to a definition, a primitive, or a parameter that holds a λ, which runs it as often as it
     chooses.

   β substitutes a pure argument when N ≤ 1 (`core/work.go`). Otherwise it takes the many-occurrence
   path: it substitutes a value (a literal, a variable, a λ, a rule table) and binds anything else at
   the redex, where the program wrote it.
2. **The theorem.** For a pure e, `let x = e in E[x]` and `E[e]` have the same meaning (ADR 0010).
   Their costs differ by e times the number of times E evaluates its hole. So substitution is an
   improvement exactly when that number is at most 1 on every path, or e is a value (Moran and Sands
   1999, improvement theory for call-by-need; Peyton Jones and Marlow 2002 §2.2, GHC's OnceInLam).
   M is the none–one–many semiring of usage (Turner, Wadler and Mossin 1995; Atkey 2018). 𝒰 on trees
   (abstract.md §8.1, `core/usage.go`) is a different count over the same signature: it measures uses
   along a path up to a back edge, which relevance needs, not evaluations.
3. **A let around a value is floated to expose it** (Peyton Jones, Partain and Santos 1996, §3.1). When
   the many-occurrence path normalizes e to `(let a (fn (y) V))` with V a value, it binds `y := a` at
   the redex and substitutes V. A static pair with a computed length, `vec(len(a), f)`, is a λ again
   and fuses, and its length is computed once.
4. **A residual let applies the same rule.** A binder β made for an ω use is pure. When a later
   reduction, a static λ applied once, leaves it with N ≤ 1, `reduceLet` substitutes it, or drops it
   unread. The table-read guard of effects.md §7c holds there, on any spelling of the table's variable
   (`readsTable`).
5. **The IR reads a measured length as the length.** A value is immutable, so `n = len a`, bound before
   a loop, is the same term as `len a` measured in it. A guard on n is a guard on a's length (a length
   π, `ir/lower.go`), and §9.4's re-slicing takes the guard's own n as its bound (`ir/restrict.go`).
6. **A let's several values are refused, not truncated.** Lowering bound a residual let's value with
   `vs[:1]`, keeping a tuple's or a variant's first value silently. This decision made that path
   reachable: a variant's payload bound outside its constructor. Lowering refuses there with `single`'s
   reason.

## Why not

- **Hoisting on the IR (loop-invariant code motion).** It would move work out of loops after the fact,
  including work the program wrote inside one, which is a different promise. It needs an
  invariance and a speculation argument per operation (a pure host call may still panic on an input
  the loop never reaches). It also makes reachable the two premises CLAUDE.md lists as witnessed only on
  hand-built IR. The defect is that β *moves* work the program placed outside; the fix is not to move
  it.
- **σ = 1 for a λ in value position.** A static λ is erased by staging, so its "applications" happen at
  compile time. But each one evaluates its body where that application lands, and an elimination
  landing in a loop runs the body there. So the number of applications is unknown at β, and ω is the
  sound answer. Let-floating (item 3) recovers the fusion that σ = 1 bought.
- **Counting branches as a join (max), as work would allow.** A branch's copy duplicates code. β
  already declines that, for size, and that rule is not this decision's to change.
- **A cheapness classification of primitives (GHC's exprIsCheap).** GHC's work-free set, the one it
  inlines inside a λ, is variables, literals and partial applications. Cheap primitive operations are
  not in it, and neither are they here: a multiplication moved into a loop is still work per iteration.
  The values item 1 copies are the same set.

## Consequences

- No answer changes; staging still preserves results (ADR 0009). Where a value is computed changes.
- In the corpus, 26 emitted files change. Proof counts are 2,064 of 2,109 operations and 353 of 374
  loops: one operation disappears because `smooth` computes its length minus two once. `freq` loses a
  loop nested inside the loop that used its result. The gauntlet is unchanged within noise on Go, JS
  and Java ([work-2026-10-22](../../gauntlet/results/work-2026-10-22.md)).
- `shape.oro`'s workaround is unnecessary.
- A parameter holding a λ that is applied to a λ is known only for the redex's own parameters. A
  static function reached another way, `f(x)(fn …)`, is taken for an eliminator: σ = 1, the old
  behaviour. This is a precision gap, not a soundness one, since e is pure either way.
