# P8: a program's own definitions kept as functions

Date: 2026-11-15. hamza: "go ahead with P8, always mathematical and algebraic". P8 of
[specialization-research.md](specialization-research.md) §8, decided in principle by
[ADR 0102](decisions/0102-factoring-never-changes-an-answer-or-a-verdict.md) and waiting on a trigger.
[oswin-2026-11-15](../gauntlet/results/oswin-2026-11-15.md) met it. This document is the algebra of
keeping a program definition as a function, the measurements that size it, the design, and **its
ledger**, written before anything is built (assessment-2026-11-15 §5 item 0).

## 1. The trigger, measured

Windows' `os` is J over kernel32, inlined at every call. Reduced with K = ∅ (every definition
unfolded) and with K = every dynamic definition the program defines (`gauntlet/factoring/depth.go`):

| program (windows) | K = ∅ | K = all dynamic |
|---|---|---|
| `roundtrip` | **fails**: the reducer's depth, 181 ms | 2,757 nodes, depth 38, 49 kept, 6 ms |
| `ledger` | **fails**: depth, 147 ms | 5,752 nodes, depth 30, 53 kept, 24 ms |
| `pe` | **fails**: depth, 270 ms | fails: a kept definition takes a static set (P1) |
| `freq` | 21,654 nodes, depth 80, 187 ms | 3,750 nodes, depth 29, 5 ms |
| `wc` | 4,900, depth 56, 30 ms | 2,600, depth 29, 3 ms |
| `count` | 8,150, depth 56, 70 ms | 2,741, depth 29, 4 ms |
| `text-of` | 18,503, depth 39, 68 ms | 2,500, depth 29, 4 ms |

M-S1 on windows (`measure.go -target windows`):
- R_K / R is 0.05–0.42 for the programs using `os` (Go's corpus measured 0.64–0.70 on 11-11);
- the most-copied definitions are `os/host.put` (π up to 684), `win/print.write` (π up to 171) and
  `text_of` with `repair` (π up to 18, at 753 nodes a copy).

## 2. The algebra

**What is settled** (ADR 0102):
- **Theorem 1**: the set K of kept definitions does not change what a program computes, since keeping
  one is an extension by definitions.
- **Theorem 2** (Hoare's procedure rule): a body proven once under its contract C = (P, φ ⟹ R, ψ) is
  correct at every call meeting P ∧ φ, and the call may assume R ∧ ψ of its result.
- **Theorem 3**: a contract proves less than its sites, since interval analysis is not distributive.
- **Theorem 4**: an obligation the body cannot prove under C, if it is a formula over the parameters,
  is lifted to the callers as a sufficient precondition and decided at each site, as inlining decides
  it.

**New here**, the two costs of inlining the trigger exposed:

**Proposition A (depth adds under inlining, and takes a maximum under calls).** Let a block be the
sequence s₁; …; sₙ, each sᵢ a call of fᵢ whose normal form nests to depth dᵢ. Inlined, β puts the
continuation sᵢ₊₁; …; sₙ at each tail of fᵢ's body, so the residual's depth is Σᵢ dᵢ, and so is the
depth of the reduction that builds it. Kept, each call is a leaf: the program's depth is n plus its
own, and each fᵢ's is dᵢ, normalized once. So D_∅ = Σ dᵢ against D_K = max(n + d₀, maxᵢ dᵢ).
`roundtrip`'s nine calls sum past `core.MaxDepth`, and their maximum is 38.

**Proposition B (case-of-case multiplies under inlining).** If f returns a sum built at c(f)
constructor sites, case-of-case copies the consumer k of its call into each site: |k|·c(f). A chain
of n such calls, each consumed by the next, costs ∏ c(fᵢ) · |k| in the worst case. Kept, the sum
crosses the boundary as its tag and slots (ADR 0042), and k is written once after a branch on the tag:
Σ c(fᵢ) + |k|. The windows cell's "build each sum once, at its tail" and its one-iteration join loops
were hand-written join points for exactly this. Under P8 they are not needed for a kept function.

**Proposition C (the boundary is the export's).** A kept function's parameters and results are
represented as at an entry point, each case already printed on every target:
- a scalar is the word, the unsigned word, or the double;
- a table or a string is a reference;
- a product is its factors (ADR 0031);
- a sum is its tag and one slot per payload type (ADR 0042);
- a `linear` parameter is borrowed by reference and written in place (ADR 0070), not handed back;
- a table it returns is born in it (W13, the frame rule of ADR 0101);
- a call is impure iff the body is (ADR 0010), so the call keeps its place among the caller's effects.

Ownership is ADR 0101's frame rule: arguments borrowed, result a birth.

**Proposition D (a contract for a parameter with no declared set).** ADR 0079 says a value's
representation is chosen from its proven fact. Every call site holds a parameter's argument in some
representation ρ. Instantiating f once per vector of argument representations (P5) gives each
parameter the set ρ's codomain, the word's range for an `Int` held in the word. A contract over ρ's
codomain is a claim true at every site of that class, so Theorem 2 holds of the instance.
`win/print.write(b, lo, n)` then has lo, n ∈ S₆₄. The instance key is (f, ρ⃗, target), as ADR 0101's
is (instance, target).

## 3. Proofs under a contract, measured on the cell (M-S2, windows)

Each definition decided once under its contract, beside the program's verdict, which inlines it:

| definition | π | under its contract | what recovers it |
|---|---:|---|---|
| `ReadFile`, `Args`, `not_found?`, `exists?`, `permission?`, `at`, `put`, `print_str`, `print_line`, `print` | 1–684 | **proven** | kept |
| `text_of`, `repair` | 1–18 | every operation proven; 1–3 loops' termination unproven, which no program is refused for | kept |
| `slice(o, a, b)`, `print_int(n)`, `write(b, lo, n)` | 1–171 | **unproven: a parameter with no set**, ℤ under the contract: `b − a`, `n / 10`, `lo + n` | Proposition D (ρ's codomain), then Theorem 4: `lo + n ∈ S₆₄` lifted to the sites |
| `readinto`, `readall`, `readfile` | 1–15 | **unproven**: kernel32.ReadFile's `lo` past 2⁶³ − 1 under the contract; proven inlined, where len(b) is a site's constant | **falls back** (P4): the relation lo ≤ len(b) is a loop's, not a formula over the parameters, so Theorem 4 does not lift it |
| `open`, `argv` | 1 | not measured: a product with no header | a product header (`(ptr × error)`), which oswin needed for `readinto` |
| `failed` | 10–50 | the measurement read `error` unresolved, an artifact of the tool's path | proven in the build (its header is checked there) |

Of the 21 definitions the measurement met (the cell, `io` and `win/print`):
- 12 are proven under their contracts;
- 3 more by Proposition D and Theorem 4;
- 3 fall back to inlining (the read loops, π ≤ 15);
- 2 need a product header;
- 1 was an artifact of the tool.

So under P2 and P4, 15–17 would be kept.

**The price, Theorem 3 here.** `readinto` is the measured case of a contract proving less than its
sites. Inlined, len(b) is the site's allocation, a constant up to 2⁴⁰. Kept, it is any length up to
max_len, and lo's widening loses the relation. P4 keeps the verdict: it is inlined. The price is its
copies and its depth, small at π ≤ 15.

## 4. The design

1. **K is chosen per target** (Theorem 1 makes the choice the target's). The first build is on windows,
   where the trigger is. On Go, JavaScript and the JVM, K stays ∅ until a measurement names a
   definition, so their emitted text does not change.
2. **Which definitions** (P2): every parameter and the result first-order, with no static argument at
   any kept call (a type, a set, a function: P1, `pe`'s case). Every parameter's set declared, or by
   Proposition D. Proven under the contract, else inlined (P4). And keeping must pay:
   |f| · (π(f) − 1) ≥ T, T measured. The proposal is T = 64 nodes, below every definition of the
   trigger worth keeping (`put`'s 66 · 683).
3. **Where**: at reduction. A kept name is not δ-unfolded at a call whose arguments are all dynamic
   (`env.Rec`, as `measure.go` does). Its body is normalized once per instance (f, ρ⃗), lowered as its
   own IR function, and decided once (`Decide` under its sets, as `buildNatCallee` decides K(ℕ\*)'s).
   - A call is `call f ā`. Its obligation is the arguments in P and φ (ADR 0028's, already at calls,
     and `CheckCallSets`).
   - Its result's facts are R and ψ.
   - The printers print a function and a call; LLVM's `define internal` and its call exist since ADR
     0101.
4. **The contract** (P3): the declared sets and `where`, Proposition D's ρ-codomains for the rest, and
   the unproven obligations over parameters lifted to the callers (Theorem 4). The result is read
   through its declared set and `ensure`. A J definition's named results with `ensure`
   (`(n ∈ 0..2^62, r ∈ …) ensure n <= len(b')`) must parse, as the host cells' do.
5. **The fallback is P4's, at two levels**: a definition its contract does not prove is inlined
   everywhere, and a call whose arguments fail the contract is inlined there (ADR 0102 as built, now
   for every kept definition).

## 5. The ledger, registered before building

**To buy** (each measured against the K = ∅ build of the same commit):
- programs: `roundtrip`, `ledger`, `pe`, `freq` on windows, refused today;
- reduction: depth (D_∅ = Σ against D_K = max), steps and time, per program (§1: 30–50× on `os`'s
  programs);
- residual and emitted size: one copy per instance, against π(f) (§1: 0.05–0.42 of the residual);
- compile time: the decision and clang, on the windows corpus;
- the cell's hand-written join points (its sums built once, its one-iteration loops), removable once
  their definitions are kept. Removing them is a later step, with its own ledger.

**To pay** (each measured, or named as owed):
- **proofs and verdicts**: by P4, no verdict changes. The price is the count of definitions kept
  against those that fall back, and why (§3 predicts 15–17 of 21 kept, 3 falling back by Theorem 3).
  A witness is that the proof counts are unchanged on every program accepted today.
- **facts at the boundary**: a caller sees R and ψ, not the body's facts. Obligations that inlining
  discharged from a body's facts may now need an `ensure`, counted per program.
- **run time**: a call, its argument passing, and a constant argument no longer folded into the body.
  LLVM's inliner may recover them (P7). Measured on the windows programs that run today (`wc`,
  `count`, `jsonfmt`, `fnv`, `crc32`, the two cases), kept against inlined, against the noise floor.
- **a sum across the boundary**: tag and slots, then a branch, where inlining eliminated the sum.
- **the compiler**: lines added (reduction's kept calls, instances per ρ⃗, lifting, the J header's named
  results), and **a second mode every pass meets**: a call of a program function where only a host
  call was. Each pass that walks calls is audited, with a planted fault where a rule must hold of both.
- **emitted text**: none on Go, JavaScript and the JVM (K = ∅ there). On windows every program that
  calls a kept definition changes, each accepted with its reason.
- **rules made less general**: none intended. If P1's static test or Proposition D's instance key
  refuses a program inlining accepts, that is P4 broken, and a defect.

## 6. Why not

- **Raise `core.MaxDepth` and the fuel**: Proposition A's sum grows with every call a program adds. It
  hides the trigger and keeps the size, and `freq` took 50 s to reach a refusal.
- **Hand-written join points in every library** (the cell's one-iteration loops): they cure
  Proposition B only, a pattern each author must remember, and one diverged inside a rule (oswin).
- **Join points in the reducer** (GHC's answer to case-of-case): they would cure Proposition B for code
  that stays inlined, and not Proposition A or the body's size. They are worth having for un-kept
  code, and not this step.
- **Keep every definition, on every target, at once**: Go's, JavaScript's and the JVM's text would
  change with no program asking. Theorem 3's price would be paid where nothing bought it.
- **Separate compilation, never inline**: ADR 0102's Why not (Theorem 3), unchanged.
- **A contract inferred as the sites' join**: Theorem 3's `t * k`, unchanged.

## 7. Questions for hamza

- **Q1. K per target, windows first, K = ∅ elsewhere until measured?** *Recommended: yes.* Theorem 1
  makes it the target's choice, and it leaves three targets' text unchanged.
- **Q2. P3, lifting, built with P8?** *Recommended: yes.* Three of the most-copied definitions
  (`write`, π 171; `print_int`, π 39; `slice`) need it, and it is ADR 0037 at a function's boundary.
  Without it they stay inlined, and the ledger buys less.
- **Q3. Proposition D, an unannotated parameter's set as its representation's codomain, one instance
  per ρ⃗?** *Recommended: yes.* It is ADR 0079 read at a boundary. The alternative is to keep only
  fully annotated definitions and annotate the library by hand.
- **Q4. T = 64 nodes as the first threshold**, re-measured on the corpus with the ledger.
  *Recommended: yes, as a measured starting point, not a constant of the design.*

## 8. Steps, each with its witness

1. **Kept calls at reduction, one instance per (f, ρ⃗)**, lowered, decided once, and called on LLVM.
   - Witness: `roundtrip` on windows prints what Go prints (`TestEveryPortableNameRunsOnEveryHost`'s
     windows run).
   - Planted faults: a kept call's arguments unchecked (`PlantNoCallSets`, exists); a result read as
     stronger than declared (`PlantResultUnproven`, exists); a kept impure call moved past another
     effect.
2. **P4's fallback for every kept definition**: one its contract does not prove is inlined. Witness:
   `readinto` inlined, with its reason recorded.
3. **Proposition D and Theorem 4** (P3): `write`, `print_int`, `slice` kept. Witness: a lifted
   precondition a site fails, which must refuse or inline that site, never assume it.
4. **The ledger closed** in `p8-2026-MM-DD.md`, with emitted text compared, proof counts unchanged,
   and run time on windows' running programs.
