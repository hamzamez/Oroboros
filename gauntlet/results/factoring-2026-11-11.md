# How much code repeats, and how many proofs survive a boundary (M-S1, M-S2, M-S4)

Date: 2026-11-11. The measurements ADR 0102 set before a program's own definitions become functions
(P8): [specialization-research.md](../../docs/specialization-research.md) §9's M-S1 and M-S2, and the
split M-S4 owed of the 2,000-quotient program's compile. The tool is
`go run gauntlet/factoring/measure.go -target T [-v]`. Its one hook in the compiler is `Env.Unfolds`, a
counter of δ by name, nil in every compile.

**The answer.** P8's trigger is not met:
- keeping the definitions that have a header saves **2.5–3%** of the real programs' residual;
- keeping every first-order dynamic definition saves **30–36%**. That is mostly a few programs'
  definitions without a header, which only P3's lifting could keep;
- of the definitions with a header in accepted programs, **87% on Go and 70% on windows are proven
  under their contract alone**. Every one that is not falls into one of three classes the research
  predicted, and **each in the first class has an obligation Theorem 4 can lift**;
- the compile the plan measured as ten minutes was about seven, **70% of it clang's**.

## What was measured

### M-S1: Proposition 2's two normal forms

For each program the tool computes two sizes, in term nodes:
- **R = |NF_∅|**, today's residual, every definition unfolded;
- **R_K = |NF_K| + Σ_{f ∈ K reached} |NF_K(f)|**, with each definition in K never unfolded (marked as
  the reducer marks a recursive name) and each body counted once.

It computes R_K for two sets K:
- **K_hdr**, the dynamic definitions with a header, whose contract P3 can use;
- **K_all**, every dynamic one, the bound.

A definition found applied to a λ is higher-order, static by §3, and leaves K, until K is a fixed
point. The programs are `examples/`, the differential cases and the acceptance programs. A
differential case enters as the harness does: `run` applied to each of its inputs.

| part | target | programs | R | R_K hdr | R_K all |
|---|---|---|---|---|---|
| `examples/` | Go | 70 | 27,755 | 0.975 | 0.696 |
| `examples/` | windows | 70 | 26,528 | 0.984 | 0.640 |
| `examples/`, accepted | Go | 47 | 23,947 | 0.969 | 0.668 |
| `examples/`, accepted | windows | 20 | 1,706 | 0.960 | **1.039** |
| differential cases | Go | 68 | 27,860 | 0.687 | 0.316 |
| acceptance | Go | 18 | 7,249 | 0.761 | 0.683 |
| all | Go | 156 of 166 | 62,864 | 0.822 | 0.526 |
| all | windows | 160 of 166 | 66,931 | 0.832 | 0.440 |

How to read it:
- **The differential cases' savings are the harness's**: it calls `run` nine times on nine constants.
  Their 0.32 is the shape of the harness, not of programs.
- **On the accepted windows examples, keeping everything is larger** (1.039). A kept body is general,
  while each inlined copy was specialized at its constants. That is Theorem 2's precision, measured as
  size: a function decided once is never smaller than its sites.
- **The large savings are a few programs' definitions without a header**, each called from a `case`
  or a sort:
  - `json/tree`: 5,989 → 1,016. `run` calls `report` in each of four arms on a constant array, so
    `parse` (499 nodes) and `walk` (350) are copied four times;
  - `io/freq`: 0.57, from `merge1` and `msort`;
  - `json/tokenize`: 0.59.

  None of these definitions has a header, so none has a contract to be decided under.
- **The largest residual is 5,989 nodes**, and no program's compile is slow: the slow compile was
  the library's, which ADR 0101 already made functions.

### M-S2: each headed definition decided once, under its contract

Each dynamic definition with a header is taken through the pipeline as an entry, its sets and `where`
assumed at its boundary (ADR 0028), beside the program's own verdict.

| verdict as a function | Go: program accepted | Go: refused | windows: accepted | windows: refused |
|---|---|---|---|---|
| proven, bounded contract | 39 | 2 | 22 | 12 |
| proven, unbounded contract | 16 | 0 | 11 | 7 |
| unproven | 2 | 0 | 6 | 7 |
| not decidable as an entry | 6 | 37 | 8 | 23 |

In accepted programs, 55 of 63 definitions are proven on Go and 33 of 47 on windows. The others fall
into three classes:

1. **An obligation over the parameters**, which Theorem 4 lifts, four definitions:
   - `crc32(tab, src)` indexes `tab[(crc ⊕ src(i)) & 255]`. Since x & 255 ≤ 255, the obligation's
     weakest precondition is len(tab) ≥ 256: linear in a parameter, and true at the one site, whose
     table is built with 256 entries;
   - `num.decimal`'s inner call requires a / p ≤ 2⁶³ − 1, a formula over its parameters, decided by
     the site's intervals;
   - `win/print.write`'s call of `WriteFile` requires lo ≤ 2⁶³ − 1 of its unbounded parameter `lo`, a
     range;
   - `win/print.print_int(n ∈ Int)`'s division is unproven under ℤ: lifting gives n ∈ S₆₄, and a site
     holding n above the word takes a clone (P5).
2. **A representation at the boundary**, one definition in two cases: `calc` in `big-nat-div` and
   `big-nat-ops` computes above the word. Inlined, the caller's `big_str` demands the exact integer, so
   the rung is chosen. As a function nothing demands it at the boundary. The supply-and-demand fixed
   point (ADR 0034) would have to cross the call. That is §5.6 and P5, not lifting.
3. **Higher-order by its result**, two definitions:
   - `data` (three cases) returns a table rule, never allocated when inlined because its consumer reads
     it as a function of its index;
   - `os.ReadAt` returns a sum in its continuation-passing encoding.

   Both results are λs. **By §3 these definitions are static, and the tool's classification by
   parameters alone was wrong.** Keeping `data` would allocate a table that inlining never makes, a
   hidden allocation (Wadler 1990's deforestation, lost at a boundary). The research's "dynamic" must
   mean first-order in the whole type, result included.

**No witness of widening's non-monotonicity** (§5.2). Every program refused while it contains a
function proven under its contract is refused at a call's obligation, or by lowering, outside that
function's body. **No Theorem 3 loss**: the declared contracts are tight. Theorem 3 is about a
contract inferred by a join, and none was inferred here.

### M-S4: the 2,000-quotient program, split

Measured with `gen` built twice from one commit, once with every K(ℕ\*) instance inlined
(`ir.PlantInlineAll`). Serial and alone; `gen` twice per form, clang `-O2` as the windows build runs
it.

| | kept (ADR 0101) | inlined |
|---|---|---|
| `gen`, our pipeline | 9.8, 9.7 s | 111, 139 s |
| clang `-O2 -c` | 8.5 s | **317 s** |
| the module | 155,056 lines | 682,660 lines |
| the object | 78 KB | 513 KB |

Inlined, the compile is 7.1–7.6 minutes, 70–74% of it clang's. Text 4.4× longer took us 11–14× and
clang 37× as long. That is superlinear on both sides, as §4 says, clang's more so. These are two
points, not a fit.

## What failed first

- **Counting unfoldings over-stated duplication.** The first estimate, D = Σ|f|·(π(f) − 1), came out at
  2.8× the whole residual. Most δ steps happen at the static level, or unfold a small definition that β
  then folds away. Proposition 2's |f|·π(f) holds for the unfolded terms, not for the residual. So the
  residual's size with K is measured directly, as its normal form.
- **49 programs were skipped as having no entry**: the differential cases enter through an unexported
  `run`. They now enter as the harness enters them.
- **A count of "refused" on windows was mostly host programs**: `os`, `go/bytes`, and declarations
  not yet callable there. Their refusals are recorded and read before a row counts as a candidate.

## What it cost, and its limits

M-S1 and M-S2 together took about an hour and a half against their five; M-S4 about twelve minutes of
machine time. R counts term nodes, not IR operations or host code. The kept residual drops the
constants a site would have specialized, which is why it can exceed R. M-S2 decides a definition as an
entry, so its representation at the boundary is an entry's (class 2 is partly that).

## What was not built

- **P3's lifting**: four definitions in the corpus would use it, and each saves only that
  definition's copies.
- **P8**: its trigger is a compile time or size measured large from repetition. The corpus's largest
  residual is 5,989 nodes, and its heavy definitions have no header.
- **The correction to the research's binding time**: a definition is dynamic only if its whole type is
  first-order. This is recorded in specialization-research.md §3.
- **M-S3**: run time with the program's own definitions kept. It is owed with P8's first program,
  `json/tree` the likeliest: a gauntlet program, so measured against hand-written code.

## References

- Wadler, P. 1990. Deforestation: transforming programs to eliminate trees. *TCS* 73(2).
