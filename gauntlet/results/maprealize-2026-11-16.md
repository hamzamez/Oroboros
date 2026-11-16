# A map on the LLVM layer is realized after the decision

2026-11-16. [ADR 0120](../../docs/decisions/0120-a-map-on-the-llvm-layer-is-realized-after-the-decision.md).
hamza, after the capacity's design (B′) met windows: "Go ahead, always mathematical and algebraic".

**On windows a map now reaches the decision as a map**, the same as on Go, JavaScript and the JVM, and
is realized after it: each operation becomes a term over the table of its cells, written in J on the
LLVM layer and substituted on the IR (`ir/maps.go`). The pre-reduction rewrite (`emit/winmap.go`), its
binder tracker and `repr map library` are deleted. Two of the book's findings close with them:
- **71's first half**: the tracker was what lost a `var` buffer, so `len` read the table's length;
- **72**: a map read inside a function, or of a map built in a block, did not build on windows.

Emission is byte-identical, compile time is unchanged, and the compiler is 433 lines shorter.

## 1. The algebra

- **ρ_M : Map(K, V) → Table(ℤ).** A map of capacity c is the table t of 3n cells, n = slots(c) =
  min{2ʲ ≥ 2c}: slot s holds the used flag t[3s], the key t[3s + 1] and the value t[3s + 2]. The
  abstraction is d(t) = {(t[3s + 1], t[3s + 2]) : t[3s] = 1}.
- **Each operation is a homomorphism h on the IR**, d(h(op(m))) = op(d(m)) on op's domain:
  - build-map(c){m. B} ↦ build(cells(c)){t. h(B)};
  - insert ↦ `map_put`, the table lent and written in place;
  - read ↦ `map_get`, the option's (tag, payload), so (1, 0) for an absent key and (0, v) for a
    present one, as the sum's encoding has it (ADR 0042);
  - keys ↦ `map_keys`, by rank, since a map's keys are distinct;
  - len ↦ `map_size`;
  - a literal ↦ `map_of`.
- **The order is ADR 0089's**: decide, then realize. Every statement about a map is about the map,
  not its representation, since the logic never prints (ADR 0049). The capacity (B′) can now be
  decided once, on every target.
- **The terms are over the host's operations**, T(Σ_H), so none uses `/`, `%` or `abandon`, which
  are themselves realized by terms:
  - the slot count is LLVM's `sdiv(len(t), 3)`, whose domain a divisor of 3 decides;
  - the hash is LLVM's `and`.

## 2. What failed first

1. **`realize build_map`**: the loader knew only the integer and string names. The map's six names
   joined the realizable set.
2. **`udiv_nat(len(t), 3)`** needs len(t) ≤ 2⁶³ − 1, which the instance cannot see, because a
   table's length has no upper bound in the refinement layer. `sdiv`'s domain needs nothing of the
   dividend.
3. **`map_slot(n, k, i)`** declared n ≤ 2⁶², likewise unprovable. The probe loops state n ≤ 2⁴⁶ in
   their guards (map_slots makes at most 2⁴⁵), so the hash's arithmetic stays in the word.
4. **The witness program was wrong first**: it stored `look(m, k) + i`, which grows without bound
   because a map's value has no set (the book's finding 74). It stores i.
5. **Four documents and three tests quoted `repr map`**: the refusal now names ADR 0120, the pair of
   disagreeing declarations is gone, and the inventory lost `library` (144 → 143 specified).

## 3. The witness

`TestAMapIsRealizedAfterTheDecision` (cmd/build) runs the book's three keys with one written twice
(`len` 3), and five keys stored in descending order read back by `keys` ascending with their values,
on Go and windows alike. `PlantMapsUnrealized` leaves the operations for the LLVM printer, which has
none, so the build fails. The differential's four map cases (`map-dynamic`, `map-keys`, `map-len`,
`map-static`) agree on all four targets.

## 4. Measured

- **Old against new**: no program the old route built runs differently. Its map built only in the
  differential cases' shapes, which still agree, and two programs written for this measure did not
  build on the old route at all (finding 72).
- **Against Go, the standing measure**: 2,000 rounds of 1,000 inserts into a `map(1024)`, 1,000
  reads and a `len`, 15 runs. Windows takes 110 ms (min 101.5) against Go's native map's 90.2 ms
  (min 82.0), **1.22×**. The probe is the old one, now inlined at each site. The gap is work owed:
  the windows map is 22% slower than Go's.

## Paid and bought

- **Compile time**: 0.99× the baseline, no compile confirmed slower.
- **Run time**: §4; the corpus's emitted text is byte-identical.
- **Code size**: unchanged for the corpus.
- **Proofs**: unchanged, 8,852 of 8,906 operations and 924 of 962 loops.
- **Refusals changed**: map programs that did not build on windows now do (finding 72's shapes);
  `repr map` is refused, naming ADR 0120.
- **Compiler lines**: non-test −607 + 24 + 150 (`ir/maps.go`) = **−433**: `emit/winmap.go` deleted,
  `MapRepr` and its parsing removed. Tests −169 + 34.
- **Language lines**: −254 (`emit/winmap.oro`) + 127 (`targets/llvm/lang.oro`'s map) + 17 (realize
  lines and comments).
- **Rules made more general**: a map's statements are decided on the map on every target. **Less
  general**: none.

## What was not built

- **The capacity's obligation** (B′), which this step made decidable on windows: the next step.
- **The map's speed against Go** (1.22×): owed under the standing measure.
- **A borrowed map parameter** (`m ∈ Tally linear`) as a realization's argument: no program has one.
