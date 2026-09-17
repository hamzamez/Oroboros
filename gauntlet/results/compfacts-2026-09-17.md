# Facts about one component of a strided table: tree's last clamps come out

2026-09-17. `emit/component.go` (new), `emit/content.go`, `emit/refine.go`, `emit/content_test.go`,
`examples/json/tree.oro`, `gauntlet/go/gen_jsontree.go`.

## 1. Measured first, and it moved the work to the other layer

The request was per-component cells. Before building, every integer operation still unproven in the corpus was
classified by what would settle it. **None needs a per-component interval cell.** They fall into five groups:
- the programs `examples/int/` exists to refuse (collatz, fib, power);
- increments by a product inside nested loops (`kara/core`);
- a map's value range (`map/dynamic`);
- a pattern-matched counter (`match/runs`);
- `shift-div`'s deliberately unbounded dividends.

After smashfd-2026-09-16, one joint cell per table already bounds every operation in freq, tally and tree.

The demand was in the **refinement** layer, on the last of array-facts.md §1.2's fifteen clamps: `tree.oro`'s
`nslc`, clamping a node index read out of a table, at six sites. With the clamp removed, tree is refused:

```
(nodes (go.+ (go.* 4 n) 3)) is an indexing, and (<= 0 (go.+ (go.* 4 n) 3)) does not follow
```

`n` is read from the worklist's slot 0. The node indices tree stores in its tables are not bounded as a whole
table:
- the node table's slot 1 holds token lengths up to 1023;
- the worklist's slot 1 holds depths.

So `∀s. t[s] < nmax` is false of every table involved. It is true of the **components** that hold indices:
- stack slots 0 and 1;
- worklist slot 0;
- node-table slots 2 and 3.

## 2. The mathematics

**Theorem (currying).** `Π_{s<k·n} V ≅ Π_{c<k} Π_{j<n} V`: a table read at stride k is k tables. So the fact

```
∀s. s ≡ c (mod k) ∧ 0 ≤ s < len t → φ(t[s])
```

is an ordinary F-D₁ fact about component c. The periodic fragment (Habermehl, Iosif & Vojnar 2008) is never
needed, because the modular guard is decided by the **index**, not by the solver. That is array-facts.md §5.3,
now built.

**Deciding the guard is a ring homomorphism.** Reduction mod m, ℤ → ℤ/mℤ, commutes with + and ×, so an index's
residue is computed structurally:

| index | residue (modulus m, residue r) |
|---|---|
| literal `L` | exactly `L` |
| `x ± y` | `(gcd(mₓ, m_y), rₓ ± r_y)` |
| `L·x` | `(|L|·mₓ, L·rₓ)` |
| `if c a b` | `(gcd(m_a, m_b, |r_a − r_b|), r_a)`, since both branches are ≡ r_a mod that |
| a name's linear form `Σ aᵥ·v + b` | `(gcd(aᵥ), b)` |

m = 1 means nothing is known, and m = 0 means the index is exactly r.

- **Consuming** a fact at read `t[i]` (Theorem D): it applies when k | m and r ≡ c (mod k).
- **Establishing** one (Theorem S): McCarthy's second axiom with the disequality proved by the congruence. A store
  at i leaves component c untouched when k | m and r ≢ c (mod k), so the fact survives without checking the
  stored value. Any other store must satisfy φ, including one whose residue is unknown.
- **Inference** (Theorem S′): Houdini over each base template restricted to each component of each stride the
  loop's stores use, beside the uniform templates.

A component fact is written `(#at k c φ)`, and a bare φ is k = 1. So printed-form intersection, fingerprints,
the body-facts cache and Houdini carry it unchanged.

## 3. Witnesses, each failing against a planted bug

`TestAComponentFactHoldsOfItsResidueClass` uses a stride-2 table with slot 2j holding `j < n` and slot 2j+1 holding
10⁶. The read at `2k + 0` is proven. Three controls must not be proven:
- the read at `2k + 1`;
- the read at `k`, whose residue is unknown;
- the read at `(if … 0 1)`, which spans both components.

`TestAStoreOfUnknownResidueTouchesEveryComponent` adds a store of 10⁶ at a clamped `j`, which may be even, and the
component-0 read must then not be proven.

| plant | caught by |
|---|---|
| an undecided residue treated as decided | the `k` control and the conditional control |
| a store of undecided residue skipped | the unknown-residue test |
| the conditional join ignoring its branches' difference | the conditional control |
| component candidates switched off | the positive case, and tree is refused again |

## 4. What it bought

- **tree.oro has no clamps.** `nslc` and `cn` are gone, and the dead `cd` and `cw` with them. `measure` is 101 of
  101 and the walk 424 of 424, with every index proven and no `-checked`. All fifteen value and node-index clamps
  array-facts.md §1.2 counted are now deleted: freq's and tally's in smashfd-2026-09-16, tree's here.
- **Answers:** the clamped and unclamped emissions agree at 1, 7, 40 and the benchmark's record count, and on eight
  walks. The differential and tooling suites pass.
- **Speed**, Go, `-benchtime=20000x`, median of five in one process:

| form | ns/op |
|---|---|
| hand-written flat, unclamped | 4,455 |
| **emitted, unclamped (now)** | **4,726** |
| emitted, clamped (before) | 5,179 |
| hand-written flat, clamped | 5,958 |

- The honest comparison is now like-for-like against hand-written **unclamped** code: **1.06×**. The clamped
  emission it replaces was 1.10× faster than its own clamped reference, a comparison the program no longer needs
  to make.
- The unclamped form's first two runs were outliers at 10,226 and 12,143 ns. The three stable ones set the median,
  and a separate six-run A/B gave 5,215 → 4,819, which is 1.08× and inside the 15% noise floor.
- **Cost:** emission is byte-identical everywhere except tree. Compiling freq, whose Houdini set grows with its
  stride-2 candidates, takes 13.3 s → 13.9 s on three interleaved pairs, about 4.6%. That is inside the noise floor
  but consistent, so it is recorded rather than called free.
- **Size:** `emit/component.go` about 150 code lines, and about 40 changed in `content.go` and `refine.go`.

## 5. Not built, and why

- **Per-component interval cells.** Nothing measured needs them (§1). If a program appears whose tag field and
  unbounded payload share a table, the same residue computation gives the cell partition, with lcm for the join,
  which is exact.
- **F-D₁ declared** (`forall` in `where`/`ensures`), which is array-facts.md §8's next step.
