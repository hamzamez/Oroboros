# 0068 — J is the surface

Date: 2026-10-15
Status: Accepted (hamza, 2026-10-15: "no need for an experiment. J is our syntax"). Supersedes item 1 of
[ADR 0056](0056-the-surface-is-d-or-j-and-its-choice-and-iteration.md) ("the source notation is D or J")
and syntax-research.md §6's proposal of one source with two renderings. D's spellings in ADRs 0056–0067
and in surface-design.md stay as the record of how the design was reached; none binds.

## Context

surface-design.md designed the surface in eight installments over abstract.md's signature (ADR 0050),
carrying two candidates from installment 3 on: D, TLA+'s notation with layout and capitalized reserved
words, and J, braces and lower-case keywords. The assessment of 2026-10-14 proposed deciding between them
by an experiment: the same real programs written in both, by hamza and by a model, counting the errors
the reader catches and the ones it does not.

By installment 8 the two differed less than when the experiment was proposed:
- **They write the logic identically**: sets in TLA+'s symbols (ADR 0065), quantifiers and implication
  too (ADR 0067), headers as membership statements (ADR 0062), definitions over sets as nested λs (ADR
  0066).
- **They differ only in the computation**: J's braces and Scala 3's line rule against D's layout,
  `IN`, `[]` and capitals.
- **Their size is level**: within 1–7% of each other in every installment from 5 on; D shallower, J
  longer in lines.
- **J's worst error is a reader error** (installment 1 §1.7): a misplaced brace fails at once, where D's
  `:` and layout needed two grammar rules (§1.11) to reach the same failure class.

## Decision

1. **J is the language's surface**: the one notation programs, libraries and target files are written
   in, with the forms of ADRs 0050–0067.
2. **One rendering, not two.** syntax-research's "one source, two renderings" is not taken; abstract.md
   §9's rendering laws apply to J, and a second rendering may be added later as a printer only.
3. **Today's notation T stays readable through μ** until the corpus is translated (abstract.md §10), and
   S-expressions stay abstract.md's T′, available to tools.
4. **The experiment is not run.** Its question, which notation is written more correctly, is left
   unmeasured, by hamza's choice.

## Why not

- **D**: its logic is now J's, so what it would have kept is TLA+'s layout for the computation, which
  needs parenthesization rules where J's braces fail at once, and capitals that read as shouting to
  most programmers. Its strengths, TLA+'s set and logic notation, are in J.
- **Running the experiment first**: the remaining difference is the computation's delimiters, and
  hamza decided it; the measurement would cost a round of programs written twice.
- **Two renderings**: two parsers, two printers and two sets of documentation, for one tree, with the
  translator between them as a permanent obligation.

## Consequences

- The next step is J's specification, consolidating ADRs 0050–0067 into one grammar, and then its printer,
  reader and translator, behind the byte-identical gate.
- surface-design.md's D columns become history; new examples are written in J only.
- `gauntlet/surface/tokens.go` keeps counting D for the installments that have it, so the record can be
  recounted.
