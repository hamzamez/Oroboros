# Two checking gaps closed

2026-09-23. Plan item 2 of [assessment-2026-09-23](../../docs/assessment-2026-09-23.md), whose §3.6
named them:
1. the unsigned word's strongest evidence lived in a scratchpad;
2. a target layer named on the command line that does not exist was skipped silently.

## 1. The unsigned word's value check, where the check runs it

[word-2026-09-22](word-2026-09-22.md) checked the `u64` rung on 100,010 values against `math/big`
with a program in a session scratchpad, which nothing would run again. It is now
`gauntlet/differential/cases/u64-values.oro`. The differential step builds it through the real
pipeline on every run and compares it with an answer computed independently.

**The generator stays inside what the language proves.** It uses two 32-bit linear congruential
streams, s′ = (1664525·s + 1013904223) mod 2³². Every intermediate is under 2⁵³, so no modulus 2⁶⁴ is
needed; that would be the modular-arithmetic wall the assessment named. From them it builds 4,000
values x = hi·2³² + lo per input, four inputs, **16,000 values**. About half lie past 2⁶³ (1,965 of
4,000 for seed 1), so both sides of the boundary between the signed word and U are met. Each x feeds
one operation class of the unsigned word:

| class | what | licensed by |
|---|---|---|
| product into U | hi·lo, up to (2³²−1)² | the homomorphism ℤ → ℤ/2⁶⁴ |
| difference in U | 2⁶⁴−1−x | the homomorphism |
| division, remainder | x/3, x mod 1000003, the digit sum | both operands in U |
| order across 2⁶³ | x < 2⁶³, against the literal's Horner spine | both operands in U |
| Euclid | x div 2³² = hi and x mod 2³² = lo | both operands in U |

The answers fold into a checksum mod 1,000,000,007, which keeps it provably bounded. The reference
computes x and every operation on it with `math/big`, sharing nothing with the compiler or with Go's
`uint64`.

**Planted, each caught:**
- **Division on signed residues.** Every target agrees and all are wrong: `230428631 -902536496 …`
  where `19426835 239748576 …` is expected.
- **Order on signed residues.** Go refuses the emitted conversion of 2⁶³, so it is caught one step
  earlier, at the host compiler.

**What the first spelling found.** The Euclid line was first written `x − hi·2³² = lo`, which is
true, and the compiler refused it. The interval analysis is not relational: it sees x ∈ [0, 2⁶⁴) and
hi·2³² apart, so the difference may be negative, and then it is in neither realization. That refusal
is the right answer for what intervals know, and Euclid says the same thing in a form they can prove.
It is recorded in the case, because a relational fact about U values is where the analysis would next
be asked to grow.

It is Go-only, like `u64-digits` and `u64-square`. U is not realized natively on the other three
targets yet, so the case skips them, and each skip line says why.

## 2. A layer named on the command line must exist

The target loader treats an absent layer as the identity of the override chain ▷, which is the right
algebra, and `TestAnAbsentLayerIsTheIdentity` keeps it so. But a directory a person *typed* that does
not exist is a mistake, and the loader cannot tell typed from implied. Git Bash converted only part of
`-targets a;b`, the first layer named nothing, and a missing binding was then diagnosed as an override
bug that was not there.

`emit.SearchPath` replaces three identical `targetDirs` in `cmd/gen`, `cmd/build` and `cmd/oro`. It
refuses a `-targets` entry that is not a directory, naming it:

```
gen: -targets names "does-not-exist", which is not a directory; a layer named on the command line
must exist (an absent one would be skipped silently)
```

A layer that exists and lacks the target still contributes nothing. The acceptance runner now creates
the layer it names even for programs that supply no generated file into it.
`TestANamedLayerThatDoesNotExistIsRefused` fails when the check is removed.

All five steps of `cmd/check` pass; the only emission changes are the new case's four outcomes.
