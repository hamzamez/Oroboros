# 0034 — The rung above the word is a least fixed point on the IR, and a bignum destination needs ownership

Date: 2026-09-27
Status: Accepted. Realized for the host's bignum (Go, JavaScript, Java) in irstep4h. The fixed-limb
rung is still selected on terms by `PromoteBig`.

## Context

ADR 0033 moved the choice of the unsigned word onto the IR and named the rung above the word as the
next to move. That rung is ADR 0019's third escape: a value the program declares above the word is held
exactly, in the host's bignum or in fixed limbs (ADR 0029).

`PromoteBig` chose it on terms, as the least set of NAMES closed under three rules (emit/bigrep.go):
- **supply**, a name bound to a big expression;
- **demand**, a name returned where a big result is declared, or passed to a big loop variable;
- **pressure**, a name a big operation reads whose interval leaves the word.

Two of its distinctions are syntactic, and both matter on SSA, where there are no names:
- **Demand promoted a NAME but widened an EXPRESSION**: `(again acc …)` made acc exact, and
  `(again (+ n 7) …)` computed n + 7 in the word and widened it.
- **Its destination rule R** wrote `acc.Mul(acc, i)` when acc occurs only in its own update in the
  `again`, and its initialiser allocates. Occurrence in one term is a stand-in for liveness. It
  forbade `power`'s squaring, where x is read by the accumulator first and by its own square second.
  It also missed a continue that hands a loop variable an object from outside the loop, which the
  next iteration would write into (ir/big_test.go).

## Decision

**B, the values held exactly, is the least set closed under supply and demand on the IR's values.**
- **Supply.** v ∈ B when v is:
  - a parameter declared above the word, or a primitive's result declared `big`;
  - a language operation + − · / with an operand in B;
  - a remainder by a divisor in B;
  - a join with an incoming value in B;
  - a closed constant term outside S.
- **Demand.** These positions demand an exact value:
  - a result declared above the word;
  - a `big` argument;
  - an ascription above the word, ℤ included;
  - every incoming value of a join in B;
  - every operand of an operation in B;
  - both operands of a comparison with one in B.

  A demanded value is in B **iff its fact is not inside S**. Otherwise it is computed in the word and
  widened where it is used, by `big-of`, the inclusion S ↪ ℤ.

Both rules are monotone in B over a finite lattice, so B is a least fixed point (Tarski). Pressure is
demand at an operand position, so it is not a third rule.

**A remainder by a word stays a word** (`big%-small`): |a mod b| < |b|. **A value of B reaching a
position declared in the word is refused**, because the promotion is a widening. **A finite declared
bound is enforced** on every growing operation by `big-fit` or `big-fit-signed` (ADR 0029).

**A destination needs ownership and liveness** (ir/bigreuse.go).
- **Ownership.** A loop parameter is owned when every value bound to it is either a fresh allocation
  read exactly once (on a continue, allocated in this iteration) or another owned parameter's object,
  and no continue binds one object twice. Owned is the greatest such set.
- **Liveness.** An arithmetic call reaching a continue may compute into an owned parameter's object
  when that object is its operand, and nothing that may hold the object is read after the call on any
  path to the end of the iteration. The holders are the parameter, its π's, a `big-fit` or a
  destination of one, and a join one reaches.

By induction on iterations, an owned object is held by nothing else live.

**A printer's inlining respects it.** On a target with destination forms, a pure call reading a
bignum is central only across no effect, as a buffer's read is (ir/plan).

## Why not

**Keep `PromoteBig`'s name/expression distinction.** It has no meaning on SSA. Where it differs from
the fact, it either computes exactly a value the facts put in the word, which costs an allocation per
operation, or it widens an expression the facts cannot bound, which leaves an unproven word operation
and a refusal.

**Promote every demanded value, whatever its fact.** Also sound, and slower: a factorial's counter,
read by the accumulator's multiply, would become a bignum. The fact is exactly what separates a counter
from an accumulator.

**Keep pressure as its own rule.** It is demand at one kind of position. As a separate rule it was
applied only inside loops, only to loop variables, and only by a separate fixpoint.

**Translate rule R's conditions.** "Every occurrence of v in the `again` is inside aₖ" is a liveness
condition read off the syntax. On SSA the liveness is available, and so is the aliasing the syntax
cannot see: a join that may hold the object, or an object bound twice.

**Move the limb rung in the same step.** It splices the limb library into the program. On the IR that
is inlining a module of IR, which is its own piece of work. The host rung does not need it.

## Consequences

- The term checker's integer sort includes the rung above the word (ADR 0033's parenthesis is
  closed). The refusal of an exact value where a word is declared is the IR's, and W5 treats a word and
  `big` as different representations.
- A declared result of ℤ (`(int 0 +inf)`) on an inlined definition is carried as an ascription, as a
  finite range above the word was. A program whose helper declares ℤ and whose export is a word was
  refused and is now accepted.
- `power` and `pair` take one more destination each on Go, where rule R's syntactic condition
  declined.
- `PromoteBig` remains for the fixed-limb rung (windows, and a target choosing `(big-repr limbs)`),
  and with it the term interval analysis. Moving it is the next step.
