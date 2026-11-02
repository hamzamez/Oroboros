# pijustified-2026-11-02: a π is justified by its branch, and the verifier checks it

Item 7 of the 10-24 plan (assessment-2026-10-24 §5), ahead of `ir/llvm`. A π-parameter is its source
renamed on an arm, with a relation the arm's guard establishes (spec/ir.md §6, Theorem E). W6 checked a
π's types and scope only, so a π on the wrong table passed it (work-2026-10-22 §5). On Go, the JVM and
JavaScript a false π costs a refusal or a missed proof. In LLVM it would be a false `nsw` or `inbounds`,
undefined behaviour the optimizer is entitled to use (ADR 0080: LLVM adds no check).

## The algebra

An arm is entered only when its guard c has the value b, so what it knows is I(c, b), the set of
literals x REL y that ⟦c⟧ = b implies. Lowering builds a guard from comparisons and `if`s: a ∧ b is
`if a { b } else { false }`, a ∨ b `if a { true } else { b }`, ¬a `if a { false } else { true }`, and
a chain nests branches in an arm. So I is computed over that structure:

    I(x REL y, b)                 = {x REL y} if b, {x ¬REL y} if not
    I(if a {arm₀} else {arm₁}, b) = ⋂ { I(a, true) ∪ R : R a route of arm₀ to b }
                                    ∩ ⋂ { I(a, false) ∪ R : R a route of arm₁ to b }

A route is a path through an arm's branch tree to a `yield`, implying the guards taken and I(yield, b).
A constant yield is a route only when it is b, and an arm with no route is unreachable, so every
literal holds of it vacuously. This is the meet over the disjunctive normal form's terms that reach b:
- on a ∧ b's false side the two routes ¬a and a ∧ ¬b share no literal, and nothing atomic is implied;
- on a ∨ b's false side the single route gives both negations;
- the ∧, ∨ and ¬ cases are the instances, and a chain of any depth is the same rule.

**The rule, W6**: each π `(V Of REL Other)` of an arm states a literal of I(guard, side), as written or
flipped (x REL y is y flip(REL) x). A length π states `(len Of) REL Other`, so its literal's side is a
value defined as the length of a table equal to Of. Values are equal modulo π and ascription, a π being
its source renamed (L9), and modulo a conversion between representations, the identity on the integer
it holds (ADR 0083). Constants are equal by their literal. A comparison call is read by the target's
classification, as lowering read it when it placed the π: the unsigned word's are ℤ's on the integers
it holds. A π on a region no branch enters is refused.

## What was measured

- **The corpus** (`cmd/check`): every π of the 366 programs the sweep emits is justified by its
  branch, at IR_A and at IR_P, and the differential suite's on four targets. Emission is byte-identical
  to the baseline, since the rule refuses and never rewrites; compile time 1.08×, the guards' literals
  memoized per guard and side.
- **Planted faults, each refused under W6** (`TestAPiIsJustifiedByItsBranch`, on dot's IR, which
  verifies before each):
  - the wrong table: the length π credited to `b`, the fault that shipped in work-2026-10-22;
  - a flipped relation, `lt` made `gt`;
  - a foreign operand, the index compared with `a` in place of the measured length;
  - a missing negation, the else arm given the guard as written;
  - a π moved to the loop's body, which no branch enters.
- **`TestTheFalseSideOfAConjunctionImpliesNoConjunct`**: a π `a ≥ n` planted on the else arm of
  `a < n && b < n` is refused, and the program verifies before it.

## What failed first, what it cost

- **The first rule read a connective only as a single yield**, `if a { t } else { false }`, and
  refused `tree.oro`'s `measure`. Its guard is the disjunction of eight comparisons, lowered as a chain
  of branches in the else arm, and the π-parameters were right. The meet over routes is the general
  form, and a chain needs no case of its own.
- **After the unsigned word's selection a guard is a call**, `u64<` on converted operands, while its
  π-parameters name the original integers. Both are ℤ's: the conversion is the identity on a value in
  U ∩ S, the only kind the selection converts, and `u64<` is ℤ's `<` there.
- **A host comparison that stays a call**, `js.>=` over `any` or `go.==` on two errors, guards
  π-parameters lowering placed by the target's classification. The verifier reads it by the same
  classification. On a value no analysis reads as an integer the π is idle.
- **A constant was compared by its type**: the ledger's guard compares with 57 typed `int`, and its π
  with 57 typed `(int 57 57)`, the range the decision gave it at IR_P. A constant's type is a fact
  about one integer, so two integer constants of one literal are one value.
- **Three heredocs ate a backslash** in test sources; the Edit tool repaired each.

## What was not built

- **A π whose relation is weaker than a literal the guard implies**, x ≤ c from x < c. Lowering never
  places one, so the rule asks for the literal itself, and a pass that weakens a π would be refused,
  which is the side to err on.
- **Theorem E for `assume`**: an `assume`'s facts are the analyses', read at entry. A π is the one
  construct that carries a relation into an arm.
