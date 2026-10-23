# resultclaim-2026-10-23: a declared result is a claim, decided at its definition

ADR 0072, the 10-21 plan's item 2 (assessment-2026-10-21 §5). Found in observer-2026-10-21 §5:
`fn bad(x ∈ 0..100) (0..5) { x + 10 }` compiled and printed 11.

## 1. What it is

A header is the membership statement f ∈ Π(x ∈ P). R (ADR 0062). Its obligation is the contract
C(f) ≜ ∀x ∈ P. φ(x) ⟹ ⟦B⟧(x) ∈ R ∧ ψ (postconditions.md §1), a statement about the definition. The
reader already made a result's range the conjunct `lo <= result && result <= hi` of ψ. An export
decided ψ against its body (`ir.CheckEnsures`). An internal definition's ψ was classed "redundant"
(postconditions.md §2) and dropped. §3's theorem shows it redundant **as an assumption**, since
inlining re-derives it at every site. That never made it true.

Measured before the change, on Go:

| `bad` | before | after |
|---|---|---|
| exported | refused: "does not establish 0 <= result && result <= 5; its result is [10, 110]" | refused, the same, with ADR 0072's remedies |
| internal, called with 1 | **compiled, printed 11** | refused |
| internal, never called | **compiled** | refused |

## 2. What was built

| where | what |
|---|---|
| `ir/limbs.go` `CheckClaim` → `checkResultClaim` | the definition lowered alone, each parameter at its set, the `where` assumed (`assumeWhere`); decided; ψ by `CheckEnsures`; each set factor by `resultWithin` |
| `ir/decide.go` | `resultWithin(f, j, lo, hi)`: R ⊆ S for the j-th yielded value, factored out of `CheckEnsures` unchanged |
| products | a tuple's factors are `sig.Results`, with no postcondition; each is a claim in the word |
| the unsigned word | a result in U, which the reader makes no postcondition of (its ends pass int64), is decided where the target holds it in a word (Go) |
| a definition over a function | refused with its reason: its contract quantifies over the function, which has no set (ADR 0053); per-instance decision not built |
| `core/read.go`, postconditions.md §2–3 | the trichotomy amended: "redundant" as an assumption, an obligation as a claim |

It runs in `emit.CheckSignatures`, which both drivers that emit code (`gen`, `build`) run once per
program, and which already normalized every definition with a header for its sort and its rung.

## 3. The corpus

`cmd/check`: emission, outcomes and refusal text **byte-identical**. No program in the corpus states a
false result. The assessment allowed that some might ("each such result is a finding"); none does.

22 claims are now decided at their definitions on Go, where none was before. They cover 10 definitions:

| definition | programs | claim |
|---|---|---|
| `num/u128.add`, `.mulw`, `.divw` | 4 each | three factors each, `0..2^64-1` and the carry or remainder |
| `sat` | `shape`, `du`, `ls` | `0..2^62-1`, saturating addition |
| `cube` | `word-wide`, `word-wide-native` | `0..2^63-1` |
| `next`, `mix`, `one`, `digitsum` | `u64-values` | the generator's step in `0..2^32-1`; a sum modulo 10⁹ + 7 and its step, in `0..1000000006`; a digit sum in `0..180` |
| `near_top` | `u64-digits` | `0..2^64-1`, in the unsigned word |

Anti-vacuity: falsifying `sat`'s set to `0..2^61` in a copy of `shape` is refused with `[0,
4611686018427387903]`. Falsifying `num/u128.add`'s carry to `0..0` in a copy of `lib/` is refused:
"component 3 of its result is in 0..0; it is [0, 1]". **The first version missed the second.** A
tuple's factors make no postcondition, so the check passed it, and the factor rule (item 3 of the ADR)
came from that failure.

## 4. Planted faults

`TestAResultClaimIsAnObligation` (`emit/claim_ir_test.go`) has 14 cases, true and false. Each fault was
planted in a copy and restored byte for byte:

| fault | caught by |
|---|---|
| no claim check | called, never called, exported, no set |
| the `where` not assumed | "under its where", a true claim refused |
| tuple factors skipped | "tuple, a false factor" |
| the unsigned word skipped | "unsigned, false" |

The true cases (saturating, under its `where`, unsigned, a table's read, a tuple) guard against a check
that refuses everything.

## 5. What failed first

- The tuple factors (§3).
- A definition over a function first came out as "its result is [-inf, +inf]" with the hint to narrow
  a parameter's set: lowering takes `f(x)` for an opaque call. `f(x)` and `a[i]` are one term by design
  (tables.md §3.1), and only the parameter's set tells them apart. So a parameter applied to a λ or to
  several arguments is refused as a function, and an unclaimed one applied to one value gets a refusal
  that names both readings. An unclaimed binder's set is spelled `any` in the core (shape-2026-10-21),
  which the first test for "unclaimed" missed.

## 6. Not built

- Deciding a result claim per instance, for a definition over a function.
- Assuming a proven claim at the sites that inline it: inlining re-derives it, and no refusal asks.
- A sort claim's per-site check is unchanged: `CheckAgainstSig` already types every definition's
  residual against its header.
- `cmd/intervals`, a reporting tool, does not run the program-level check.
