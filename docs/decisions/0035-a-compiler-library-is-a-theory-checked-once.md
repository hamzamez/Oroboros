# 0035 — A compiler library is a theory checked once; a use is an instance, inlined on the IR

Date: 2026-09-28
Status: Accepted. Realized for the fixed-limb bignum (`emit/bignum.oro`) in irstep4i.

## Context

The fixed-limb rung (ADR 0019, 0029) holds a value above the word as a table of n limbs in base
B = 2²⁴, presented by ι(o) = Σ oᵢ·Bⁱ. Its operations are a library written in the language,
`emit/bignum.oro`: schoolbook addition, subtraction and multiplication, short multiplication and
division by a word, the remainder by a word, lexicographic comparison from the top limb, and the
conversion to a host bignum. A `guard` enforces the program's one set by trapping on the final carry or
on the top limb reaching its ceiling.

The library reached a program by **splicing on terms**. `PromoteBig` rewrote the residual into the
host's bignum shape, and `LowerLimbs` replaced each operation by an application of a library
definition, reduced at its site. Each spliced copy was then type-checked, refinement-checked and
analysed as part of the program. That kept two things alive that ADR 0032's migration retires:
- the term interval analysis, which ran the promotion;
- a representation chosen on terms.

It also meant the library was re-proven at every site. Its index obligations (`(t i)` under `at`'s
guards) and its divisors were discharged once per call, although neither depends on the call.

ADR 0034 moved the choice of values held exactly onto the IR, for the host's bignum. The limb rung
needs the same selection and then a lowering.

## Decision

**The library is a theory.** Each definition f is a lemma about Vₙ = [0, Bⁿ) with a precondition P:
under P, ι(f(ā)) is the operation on ι(ā) when the result lies in the program's set, and otherwise f
traps. Its typing, its linearity and its refinement obligations are properties of the definition
under P. **So they are checked once per specialisation (n, lim)** (`emit.LimbFunc`): the
definition is η-expanded at the literals n and lim, normalised, and put through the type checker, the
signature check, linearity and the refinement layer. The only precondition is k ≠ 0 for `div-small`
and `rem-small`, stated as the instance's `where`.

**A use is an instance.** Its obligation is P at its arguments. For short division and remainder
that is exactly the program's own division obligation, which the refinement layer discharged on the
residual before the rung was chosen. The instance is lowered to IR once and **inlined at each use**
(`ir.LowerLimbs`). Inlining is substitution into a checked body:
- a body ending in a yield hands its values over by renaming;
- one ending in a branch becomes an `if` with the branch's arms.

What depends on the site, the word arithmetic under the site's facts, is decided at the site by the
decision taken again, like any code.

**The rung is chosen on the IR** (`ir.SelectRung`). `SelectBig` writes the host's shape, with no
`big-fit` and no destinations, since `guard` enforces the set and a table is not written into. Then
each operation becomes its instance. Two shapes are chosen by facts:
- **one pass.** A product or quotient by a widened word k with k ∈ [0, 2²⁸) is `mul-small` or
  `div-small`: limb·k + carry stays under 2⁵³. A widening only a one-pass instance reads is dead, not
  an instance;
- **a magnitude.** `big-of v` is `of`, whose limbs represent v iff v ≥ 0, so a widened value that may
  be negative is refused. Limbs hold a magnitude (ADR 0029).

An operation with no instance, a quotient or remainder by a bignum, takes the host's bignum for the
program where the target has one, and is refused by name where it has none.

## Why not

**Keep splicing on terms.** It needs the term promotion, and so the term interval analysis. It also
re-proves a verified body at every site.

**Trust the library unchecked.** It is code, and its obligations are real. With `at`'s upper guard
planted away, the theory check refuses `add`, `sub` and `mul` by name. A library the compiler inlines
without checking is a proof nobody made.

**Check each inlined copy on the IR.** The refinement layer, which discharges index and divisor
obligations, works on terms. And a site-independent obligation checked at every site is the same
proof repeated.

**Emit each instance once as a host function and call it.** That keeps code size down, but it costs
a call per operation and loses the specialisation the literals give, where every loop bound and
length is a constant. bigarith-2026-08-28 measured the inlined form. A change of that kind is its own
measurement (ADR 0008), not part of this move.

## Consequences

- No emitting path uses `PromoteBig` or `LowerLimbs`. The term interval analysis is left only in
  `CheckSignatures`' claim check, in the refinement layer's loop invariants, and in the `-irproof`
  shadow.
- A compiler library written in the language has a place to be checked: its own specialisation.
  `emit/winmap.oro` is spliced before reduction, where the program's checks already see it, so this
  does not apply to it.
- The limb rung inherits ADR 0034's selection. A counter the facts bound stays a word, where the term
  pass had made render's factorial counter a limb table.
