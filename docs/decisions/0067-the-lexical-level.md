# 0067 — The lexical level: ASCII words, `-` as `_`, backquoted host names, `^` on constants, line comments, and J's logic in TLA+'s symbols

Date: 2026-10-15
Status: Accepted (hamza, 2026-10-15, on surface-design.md §8.7: "I agree with all 9"). Supersedes ADR
0062's J spellings of the logic (`forall`, `exists`, `==>`, `<==>`); amends abstract.md §1.2 (𝒯's holes
are named splices, ADR 0066). Not built: the surface is built when it is chosen.

## Context

surface-design installment 8 designed how names, literals, comments, operators and lines are written,
tested on the whole corpus (`gauntlet/surface/lexical.go`) and on three files rendered in full with
their comments. A rendering's spelling σ_R : 𝒩 → Identifiers_R must be injective per program (abstract.md
§9.3). Measured: 68 bound names with `-`, 14 ending in `?`, none colliding once `-` becomes `_`; 61
target declarations named by operators (`+`, `big+`, `f>`), called 320 times by the native programs; 233
literals of the forms 2ᵏ and 2ᵏ − 1; 4,165 of 4,178 comments on lines of their own.

## Decision

1. **Spelling.** An identifier is UAX #31 (ASCII in practice): a letter or `_`, then letters, digits and
   `_`, optionally ending in one `?`; NFC required and bidirectional controls refused, as today. `-`
   becomes `_`; σ is injective, and a collision is refused naming the pair. **A name outside the
   alphabet is written in backquotes**, the host's own name unchanged: `` go.`>=`(i, len(a)) ``.
2. **Words.** J reserves only the words that open a form: `fn spec lemma set array map var let loop
   again case if else try expect assert use module provides private export axiom true false`. Clause
   words (`where ensure invariant pure inout out linear as import host expr stmt comma_ok success
   relevant` and the model's data words) are keywords only in their clause position. After `.`, any word
   is a member.
3. **Case carries no meaning.** By convention a set's name is UpperCamelCase (`Int`, `Option`,
   `result.Result`, `OsError`), values and functions snake_case, host names the host's spelling. The
   translator renames the corpus's sets.
4. **Literals.** Integers in decimal; **`^` raises an integer constant to a power at compile time**
   (`2^63 - 1`, `10^18`), today's `(pow 2 k)` generalized, both operands static, refused otherwise. A
   float has digits on both sides of its point, so `..` is never part of a number. `-` is a prefix
   operator. No hexadecimal, binary or octal; no raw strings; strings as string-literals.md specifies.
5. **Comments are line comments, `//`**, gaps the translator transports to their abstract position; a
   comment quoting code is carried untranslated. No block comments.
6. **A line break ends a statement unless it cannot** (Scala 3's rule): not inside `( )` or `[ ]`, not
   after a token that cannot end an expression (a binary operator, `,`, `=`, `:=`, `->`, `:`), not before
   one that cannot begin a statement (`where`, `ensure`, `invariant`, `else`, `{`, `)`, `]`, a binary
   operator).
7. **J's logic is written in TLA+'s symbols**: `∀ ∃ ⇒ ⇔`, with TLA+'s ASCII `\A \E => <=>` as input,
   beside ADR 0065's set symbols; `&& || !` stay the computation's connectives, which formulas include.
8. **Precedence**, highest first: application, indexing, prime, member; `^` (right); prefix `-` `!`;
   `* / %`; `+ - ++`; `..`; `×`; `∪ ∩ ∖` (one level, mixed only in parentheses); comparisons and
   memberships `== != < <= > >= ∈ ∉ ⊆ ⊂` (no chains); `&&`; `||`; `⇒` (right); `⇔`; quantifiers, whose
   body extends as far right as it can.
9. **The prime and the tag share `'`**: directly after a name it is a prime and may not be followed by an
   identifier character; anywhere else it begins a tag and must be.

## Why not

- **Renaming host operators** (`go.%` to `go.rem`): it teaches the reader's limits instead of the host's
  names, which is why today's reader admits `%&|^~`; backquotes keep the name.
- **Dropping `?`** or translating it to `is_`: it carries meaning for a reader, and nothing else in J
  uses `?`.
- **Reserving every keyword**: `host` and `out` are bound names in the corpus; contextual words cost
  nothing where no clause can stand.
- **Case as meaning** (Haskell's constructors, Go's exports): host names are the host's, and Go
  capitalizes its functions.
- **Hexadecimal and digit separators**: no bitwise operators, and the long literals measured are powers.
- **Block comments**: a nesting rule and a second lexer mode for 13 comments in 4,178 that do not stand
  on their own line.
- **Go's semicolon rule**: it forbids a header's clauses on lines of their own and dictates `} else {`.
- **`forall` and `==>` beside `∈` and `⊆`**: two notations for one logic, after ADR 0065.

## Consequences

- The translator maps 68 names to snake_case, keeps 14 `?`, backquotes 61 declarations and 320 calls,
  writes 233 literals as powers where they are, and carries every comment.
- Tooling wants an editor input method for the symbols; it is tooling, decided with the reader.
