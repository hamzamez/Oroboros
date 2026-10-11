# presburger — a complete procedure beside today's entailment, on the whole corpus

2026-10-11. Step 3a of [spec/abstract.md](../../docs/spec/abstract.md) §13, and the first step of
[settypes-research.md](../../docs/settypes-research.md) §8, which ADR 0049 decision 6 put before any
rewrite: *"Every obligation the corpus makes today can be run through a Presburger procedure beside the
current … entailment."* hamza: *"go ahead with step 3"*.

## What was built

**`presburger/`, the Omega test** (Pugh, Supercomputing 1991). It decides whether a conjunction of
linear equalities and inequalities has an integer solution, which is the quantifier-free fragment of
Presburger arithmetic. It is the first of ADR 0049's own complete procedures:
- normalization by gcd, a constant rounded down;
- equalities eliminated exactly, by a unit coefficient or by Pugh's mod-hat step;
- inequalities by the real shadow, the dark shadow and the splinters;
- every arithmetic step checked, so an overflow or the step budget is the answer "unknown", never a
  wrong one.

**`emit/entaillog.go`, the measurement.** Today's `entails` (Fourier–Motzkin over the rationals with
gcd tightening, `entailsFM`) answers as it always has. When `OROBOROS_ENTAIL_LOG` names a directory,
each question (facts ⊢ g ≤ 0), today's answer and the Omega test's are written there, one file per
process. So `cmd/check -only emission` measured all 556 compiles without changing what they emit.

## What the Omega test was checked against

| check | result |
|---|---|
| known systems, among them Pugh's own: rational solutions, no integer one | pass |
| 4,000 random systems over 3–4 variables boxed in [−4, 4], against exhaustive enumeration; equalities, coefficients to 7, constants to 20 | **all agree**, at least 500 sat and 500 unsat |
| eight planted faults: the dark shadow's margin, the splinters, every elimination exact (rational Fourier–Motzkin), the rounding of a constant, an equality's divisibility, the mod-hat residue, a two-sided variable dropped, the splinter range one short | **all eight caught** |

## Measured

Logged by 853 `gen` processes: 556 runs, plus the sweep's second runs and serial pairs, so totals count
some questions twice and **distinct questions are the unit**.

| today \ Omega test | proved | refuted (an integer counterexample) | unknown |
|---|---:|---:|---:|
| **proved** | 16,401 | **0** | 1 |
| **not proved** | **200** | 11,062 | 29 |

27,693 distinct questions; 1,315,663 asked.

**1. Today's entailment is sound on every question the corpus asks.** It never proved a question with
an integer counterexample. Its incompleteness never cost a refused program either, which is the second
finding.

**2. The complete procedure proves 200 more distinct questions (0.7%), and changes no outcome.** They
come from 10 runs, all of programs that already compile (`freq` on three targets, `tokenize`, `tree`,
`lines`, three Windows programs, `lib/win/fmt`). **None of the 256 refused runs asks one.** Of the 200:
- **168 are ex falso on a dead path.** The facts alone have no integer solution, so the goal holds
  vacuously. The contradiction lies in variables the goal does not mention (n ≤ −1 with n ≥ 0, beside
  a goal about `digits`), and `farkas` reasons only in the goal's connected component;
- **32 are entailments proper**, which need integrality (a remainder's or a quotient's bounds) or a
  combination past `farkas`'s limits.

**3. 11,062 distinct questions are not entailed at all.** The Omega test finds an integer
counterexample for each, so no procedure could prove them from those facts. What a refusal lacks is
**facts**: invariants, content facts, disjunctions, sets a program cannot yet state. It is not the
strength of the procedure.

**4. The unknowns** (30 distinct) are all an overflow. Each mentions the word's bounds, ±2⁶³ and
`rem(m, 10)`, whose combinations leave int64. Big integers would answer them.

**5. Cost.** On every question, the Omega test multiplies a compile's time by 3.5–9.5 (`freq` 11.9 →
41.7 s, `jsonfmt` 0.36 → 3.4 s; the sweep flagged eight compiles, all of them this). The time is
dominated by the questions it refutes (286 of 364 CPU-seconds go to questions today does not prove), so
as a fallback after Fourier–Motzkin it would cost almost as much. Median 0 µs, p99 6 ms, maximum
89 ms per question.

## What it decides

settypes-research §8 ordered the work as the Presburger core first, then the Boolean algebra of sorts,
then types as static values. **The measurement reverses the first two.** A complete procedure would
change no program's outcome today, and it would multiply compile time. The refusals are about what the
logic can say and what facts it holds. So the order becomes:
1. **Types as static values and the Boolean algebra of sorts**, demanded by a program: `errors.AsType`
   and `errors.As`, the standard library's waiting item, whose result names a type parameter
   (typevars-research's first instances, settypes §8 step 3);
2. **the Presburger core then joins as the checker of those claims**, where a set expression's
   inclusion needs it. It joins with big integers (finding 4) and with a consistency check of the whole
   fact set (finding 2's 168), which is cheap where Fourier–Motzkin is not;
3. **today's entailment stays**, held sound by the witness below.

## A permanent witness

`TestEntailmentIsSoundAgainstPresburger` puts every question `entails` answers on five corpus programs
(1,000 or more questions) to the Omega test, and fails if `entails` ever proves one with an integer
counterexample. **Planted fault**: the single-fact rule accepting a fact one weaker than the goal
(`fact.konst >= g.konst-1`). Caught, with the counterexamples printed (`lines < words` from
`lines ≤ words`).

## What it cost

| | lines |
|---|---:|
| `presburger/omega.go` (new) | +485 |
| `emit/entaillog.go` (new; the hook is off by default) | +94 |
| `emit/linear.go` (`entails` renamed `entailsFM`) | ±1 |
| tests (`presburger/omega_test.go`, `emit/entaillog_test.go`) | +198 |
| in the language | 0 |

**With the hook off**, `cmd/check` (tooling skipped for the Node pin) passes vet, compiler, emission, ir and differential; emission is **byte-identical**, compile time 1.05×. The tooling tests but the Node pin pass.

## Not built

- **ε and 𝒪 on trees** (abstract.md §13 step 3 proper). Obligations generated from trees with claims
  are worth building when the claims carry sets, which is the reordered first step above. Built
  against today's sig-and-`where` claims, they would regenerate what `DischargeRequires` already
  decides.
- **Big integers in the Omega test**, and the fact-set consistency check, both for when it becomes the
  checker.
