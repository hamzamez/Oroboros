# 0065 — J writes sets in TLA+'s symbols, with TLA+'s names as their ASCII; a set former keeps its keyword

Date: 2026-10-15
Status: Accepted (hamza, 2026-10-15, on surface-design.md §6.8–6.9: "11. yes to TLA symbols", "12. no.
set {} former always has a keyword. block is headless", and on the ASCII spelling, "agree").
Supersedes item 2 of [ADR 0060](0060-j-set-operators-and-qualified-tags.md); its items 1, 3 and 4
(membership `∈`, qualified tags, no `exposing`) stand. Not built: the surface is built when it is chosen.

## Context

ADR 0060 gave J Python's set operators, `| & - ^ <= < *`, beside `∈`, on the argument that a set is its
characteristic function, so join, meet and order are the Boolean operations lifted pointwise. Writing
installment 6's declarations in J (`axiom NumError <= error`, `'not_found | 'exists`) showed what the
argument missed (surface-design §6.8):
- **our sets are static** (Law S): a set operator appears only in the logic, in binder sets, claims, set
  definitions and axioms, where mathematics reads best (syntax-research);
- **`-`, `*`, `<=` and `<` are homonyms by sort**: subtraction, multiplication and ℤ's order on integers,
  difference, product and inclusion on sets, and of the four only the order is one operation lifted;
  `|`/`||` and `&`/`&&` are one keystroke apart across sorts;
- **J was already mixed**: `∈` beside `<=`.

## Decision

1. **J writes the set algebra as D does, in TLA+'s symbols**:

   | | symbol | ASCII the reader accepts |
   |---|---|---|
   | membership | `∈` `∉` | `\in` `\notin` |
   | union, intersection, difference | `∪` `∩` `∖` | `\cup` `\cap` `\` |
   | inclusion, strict | `⊆` `⊂` | `\subseteq` `\subset` |
   | product | `×` | `\X` |

   Precedence as D's, which is TLA+'s.
2. **The file stores the symbol.** The reader also accepts TLA+'s name, and the printer writes the
   symbol: one tree, the canonical spelling printed (ADR 0064, item 3's rule).
3. **Short input is the editor's, never the language's**: a trigger an editor expands may be as short as
   wanted, a unique prefix of the name or a shape mnemonic, but never a string escape (`\t \n \r \u`).
4. **A set former keeps its keyword**: `set { … }`, `array { … }` and `map { … }` are formers, and braces
   with no head are a block, everywhere. A binder's enumeration is `method ∈ set {"GET", "POST"}`.
   ADR 0061's formers stand. D, which has no blocks, writes TLA+'s bare braces.
5. **Symmetric difference has no operator**: TLA+ has none, no program writes one, and it is
   (A ∖ B) ∪ (B ∖ A).

## Why not

- **Python's operators (ADR 0060's item 2).** Homonyms by sort, borrowed from a language whose sets are
  data at run time, for sets that exist only at compile time.
- **The short ASCII forms `\u`, `\n`, `\c`, `\c=`** (hamza's question). `\n` and `\u` are two of the
  language's six string escapes (string-literals.md): a reader sees a newline in `A \n B`, and an editor
  expanding `\n` turns `"line\n"` into `"line∩"` unless it parses. They mix mnemonics of the shape with
  `\in`'s mnemonic of the name, and `\c=` meets the language's `=`. Since the file stores the symbol,
  ASCII is met only where someone typed without the editor, and there `\subseteq` explains itself to
  anyone who has read LaTeX or TLA+.
- **TLA+'s bare braces in a set position** (`method ∈ {"GET", "POST"}`, surface-design §6.8 item 12). It
  would make braces a set in some positions and a block in others; a keyword keeps the rule one token
  long.
- **Unicode without an ASCII spelling.** A file must be writable from any keyboard and readable in any
  terminal; TLA+'s names cost nothing to accept.

## Consequences

- J and D write the logic identically, in TLA+'s notation, and differ only in the computation: braces
  against layout, lower-case keywords against capitals. The D-or-J experiment decides less.
- Installment 6's J renderings (§6.5) are rewritten by the translator: `|` becomes `∪`, `<=` becomes `⊆`.
- `|`, `&` and `^` are free in J again; `-`, `*`, `<=` and `<` are the integers' alone.
- An editor input method is wanted with the reader; it is tooling, decided then.
