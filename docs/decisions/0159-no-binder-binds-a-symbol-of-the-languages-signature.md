# 0159 — No binder binds a symbol of the language's signature

Date: 2026-11-23
Status: Accepted (hamza, 2026-11-23: "yes, build it now. always mathematical and algebraic", on
chapter 1 §1.6's `fn(`+`) { 1 + 2 }`). It extends defects-2026-10-26's rule (no module defines a
name of Σ_L) from definitions to binders. It amends [def.md §11](../spec/def.md), whose "Not reported:
a binder shadowing a name" named `(fn (+) …)`, by the measurement §11 itself asked for. Built the same
day ([nobinder](../../gauntlet/results/nobinder-2026-11-23.md)).

## Context

Chapter 1 §1.6 showed `fn(`+`) { 1 + 2 }` reducing to itself and called it "legal, and never what you
meant". The backquotes are J's quoted name (ADR 0067), so `` `+` `` is the name `+`, the one J's
reader writes `1 + 2` with: app(+; 1, 2). Under that binder, `1 + 2` applies the parameter.

Measured on HEAD (877433f9), on Go:

- `fn(`if`) { if c { 1 } else { 2 } }(fn(x, a, b) { 7 })` compiled to `return 7`, whatever c was;
- `fn(`+`) { x + 2 }(fn(a, b) { a * b })` compiled to `x * 2`;
- a case arm `'some(`if`) -> if k > 3 { 1 } else { 2 }` expanded to `(#p (> k 3) 1 2)`, the payload
  applied as if it were the choice.

This is the wrong answer defects-2026-10-26 refused for a module's `if`, reached through a binder.
That rule's reason is a definitional extension: a definition introduces a fresh symbol (Shoenfield
1967 §4.6), and a definition of a symbol of Σ_L redefines the symbol J's syntax is read as. It was
built in `Load` for definitions only.

def.md §11 (2026-08-16) left every shadowing binder unreported, for three reasons:

1. no meaning is target-dependent;
2. the consequences are caught further along;
3. the unambiguous half (a repeated parameter) is already an error.

It named what would reopen it: "a real shadowing bug that survived both the type checker and the
refinement checker". The first case above is one.

## Decision

1. **Σ_L's names are operation symbols, not variables.** In the abstract syntax, a binding signature
   (ADR 0050, abstract.md §1.1), `+` and `if` are operators. A binder binds a variable, which α
   renames together with its occurrences. `1 + 2` is no occurrence of a variable: renaming a binder
   `+` to `p` would have to turn `1 + 2` into `p(1, 2)`, so the infix would not be notation for an
   operation. The core spells operators and variables alike as names, which is why the capture was
   possible at all.
2. **The condition is bv(t) ∩ Σ_L = ∅** for every declaration t. bv is the bound variables, the
   function the signature's arities define by structural recursion. It has one clause per binding
   operator, each naming the positions it binds:
   - a λ's parameters (a quantifier's and a table rule's binder among them);
   - a let's pattern;
   - a loop's variables and its match arms' patterns;
   - a case arm's pattern;
   - a state binder (`var` cell, buffer, map buffer);
   - a binder's claim `x ∈ S = e`;
   - the pattern of `try` and `expect`;
   - a separate `sig`'s parameter names.
3. **It is decided on the reader's tree** (`core.bv`, run from `placed` before D), where every binder
   stands as written. D makes a block's binder and a quantifier's into a λ's, but a match loop's arm
   renames its pattern away, and `Load` substitutes a case arm's pattern. So no later image holds
   every binder as a λ's.
4. **The refusal reads like the module rule's**: *a binder in f names `+`, which is the language's
   own: … A binder introduces a fresh name; choose another*.
5. **Shadowing any other name is still not reported**: a definition (`double`), a target's name
   (`h`), an outer binder (`n`). def.md §11's reasons hold for a variable; they fail only for an
   operation symbol.

## Why not

- **Keep it legal and taught** (def.md §11's instrument): the measurement §11 asked for is in hand.
  A binder of a symbol of Σ_L is not a legibility problem: it changes what the construct means, and
  a program answered 7 whatever its argument.
- **A note instead of a refusal**: a binder of `+` has no reading under which `1 + 2` means addition.
  The rule's other half, a definition of `+`, is already a refusal, and the two must agree.
- **Rename the binder apart, so the construct keeps its meaning** (treat `` `+` `` the parameter as
  fresh, `+` the operator untouched): that makes `` `+` `` and `+` two names written alike. The
  spelling would stop being injective (ADR 0067's σ), and the parameter could never be applied by
  its own name.
- **Check the core's λ parameters after `Load`'s expansions**, one clause instead of many: measured
  wrong. A case arm's binder is substituted, not bound by a λ, and the capture there passed that
  check. The clauses are not a list of cases, since each is the arity the signature already declares.
- **Refuse only the operators and keep `len`, `keys`, `map`, `set` bindable**: these are symbols of
  Σ_L too. `len = 3; len + 1` binds `len`, and the reader writes `t`'s length with `len`, so a
  table's length read under it would be captured as well. One corpus program bound `keys`
  (`experiments/java-toplevel/tally.oro`, renamed `slots`); none bound an operator but the book's
  own example.

## Consequences

- Chapter 1 §1.6 shows the binder of `+` refused, then the working form, a parameter of its own name
  beside `+`. Chapter 2 §2.4's reference to it is updated.
- def.md §11 and state.md state the rule, and §11 says why it was reopened.
- The corpus test reads tracked files only. A book example the reader refuses is `.oro.txt`, so
  `no-claimed-cell.oro` from ADR 0158 is renamed: it had passed the check untracked, and failed it
  once committed.
