# 0089 — A realization is a term over the host's operations, substituted after the decision

Date: 2026-11-02
Status: Accepted (hamza, 2026-11-02: "why isn't it defined just like another function in terms of
llvm's instruction instead of host template … go ahead. always mathematical and algebraic").
**Amends [ADR 0088](0088-a-target-states-its-realizations-and-a-host-operation-is-declared-as-it-is.md)**
in what a realization may be. Built for `windows-llvm`
([llvm-2026-11-02](../../gauntlet/results/llvm-2026-11-02.md)).

## Context

ADR 0088 made a target state its interpretation of the language's names, `realize OP = A, B …`, and
declared each host operation as it is. Where no single LLVM instruction is the language's operation on
its whole domain (`<<` past a count of 63, `/` and `%` at −2⁶³ by −1, `~`, `mod`), the realization was a
**template**: LLVM text, several instructions, that the compiler copies and cannot read. Its agreement
with the language was trusted, its LLVM operations were invisible to the analyses, and a mistake in it
(a `select` the wrong way round, a guard at 64) would print a wrong program with nothing to refuse it.

The forms are not text by nature. Each is a composition of LLVM's own operations, which the language
can already call: a **derived operation** of the host's signature.

## Decision

1. **A target is an interpretation ι : Σ_L → T(Σ_H)**: each of the language's names is mapped to a
   term over the host's operations (a theory interpretation, Goguen and Burstall; a derived signature
   morphism). A realization is either a host operation (a term of depth one) or a **definition the
   target makes in the language** (D_T, target-system.md §6.2), whose body calls host operations:
   `fn shl(a ∈ Int, b ∈ Nat) (Int) { if b <= 63 { llvm.shl(a, b) } else { 0 } }`.
2. **Its header is the operation's domain**: the definition's `where` and parameters' sets are the
   language operation's obligations at every site, as a host declaration's `where` is (`/`'s b ≠ 0).
3. **Soundness is two conditions**, each checked:
   - **definedness**: every host operation in the term is used inside its own domain. Decided once, when
     the term's instance is built, as a program's obligations are (`llvm.shl`'s count in 0..63 on the
     arm whose guard proves it);
   - **agreement**: ⟦ι(f)⟧(x) = f(x) for x in f's domain. Checked on the IR's interpreter against ℤ's
     definition at the domain's edges, every operation of the term in the word and every host
     operation inside its domain (`ir/realize_test.go`, with the guard at 64 planted).
4. **The term is substituted after the decision**, inlined for the operation it realizes, and the
   decision is taken again (ADR 0035's shape: a theory checked once, a use an inlined instance). The
   site's own obligations are decided before, so a term may rely on them: `<<`'s answers 0 past 63
   because a·2ᵏ in the word forces a = 0.
5. **Inside a term, a host operation that is a stated realization of g is read as g**, by the
   statement, not its name: `llvm.shl` in `<<`'s list is `<<`, so it keeps its interval transfer, its
   exactness and its flags; the statement records it (`via llvm.shl`, printed in the IR's text).
6. **A term is over host operations only**: an operation inside it that is itself realized by a term is
   refused, so ι maps into T(Σ_H) and substitution is one step.
7. **Per instance, the narrowest later realization whose domain the facts prove** is chosen instead:
   its parameters' sets, and its `where` decided on the operands' intervals. `a << k` with k proven
   below 64 is LLVM's `shl nuw nsw` and no term; `a / 10` is `sdiv`.
8. **Templates remain only for a host's own operations**: one instruction or one call. Those that are
   more today each wait on a stated reason: a call with two results (the overflow intrinsics of the
   checked arithmetic), a call that never returns (`llvm.trap`, `ExitProcess`), pointers as values (the
   allocator, `WriteFile`'s buffer and cell, ADR 0084's LLVM part), UTF-8 validity as a claim (the
   string operations).

## Why not

- **Keep templates for the composite forms** (ADR 0088 as built). The compiler cannot read them: their
  agreement is trusted, and their operations are opaque to the analyses. A template is the same
  interpretation written as text; a term is it written in the algebra the compiler checks.
- **Substitute the term before the decision**, as an ordinary definition δ unfolds. The decision would
  then see `if b <= 63 { … } else { 0 }` where it sees `<<`, and lose `<<`'s obligation (a·2ᵏ in the
  word), which is exactly what makes `else 0` right: as an ordinary function the term is wrong for
  a ≠ 0. The term is an interpretation of an operation, applied once the operation is decided.
- **Compute the instruction and discard it, as LLVM's `select` does** (`c = llvm.shl(a, b)` for every
  b, then choose). The language has no poison: a call outside its domain is refused, not a value to
  discard. The branch is the language's form; LLVM speculates it into a `select` where that is cheaper,
  sound because each arm is defined where it is taken.
- **Prove agreement in the logic** rather than check it at samples. That is a lemma per operation,
  `∀ x ∈ dom f. ⟦ι(f)⟧(x) = f(x)`, over bit-vector semantics; the logic is not built (ADR 0062). The
  samples are the domain's edges, and the planted fault shows the check sees a term wrong at one.
- **Read a host operation inside a term by its name** (`sdiv` as `/`). ADR 0088's reason holds: a name
  is not a meaning. Its stated place in a realization list is.

## Consequences

- `targets/windows-llvm/lang.oro` holds `~`, `<<`, `>>`, `mod`, `/` and `%` as definitions over
  `llvm.oro`; `realize` names them first and the bare instruction after (`realize `<<` = lang.shl,
  llvm.shl`).
- The IR gains `via`, the host operation an integer operation is computed by, in its text
  (`(shl exact via llvm.shl %1 %2)`); `ir/realize.go` chooses and substitutes (`RealizeOps`, in
  `ir.Entry` after the decision).
- **−2⁶³ / −1 is decided on this target**: `/`'s term computes it as `0 − a`, the language's `−`,
  whose result 2⁶³ is outside the word, so the site is refused, or traps under `-checked`, as ℤ says.
  The other targets still answer −2⁶³ (plan item 8).
- The target reader keeps a definition's header with its body (it kept only the first of the two forms),
  and a header with no host body is the target's own library's.
