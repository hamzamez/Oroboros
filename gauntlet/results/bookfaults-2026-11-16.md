# The book's other faults: a map's size, a pair's projections, a double's comparisons, a crash, a race

2026-11-16. Plan item 1 of [assessment-2026-11-16](../../docs/assessment-2026-11-16.md), after
[capture](capture-2026-11-16.md) (53/76) and [store](store-2026-11-16.md) (70). hamza: "Go ahead,
always mathematical and algebraic".

Five of the book's findings, each with its algebra, its fix at the cause and a planted fault. One
half of finding 71 is not built: the capacity, a step of its own (§6).

## 71. A map's `len` is the size of its domain

**12 for three keys at capacity 2 on windows**, where it is 3 on every other target.
- On windows a map is a table of 3 cells per slot, at the least power of two ≥ 2c slots. So `len`
  must be rewritten to the count of used slots while the map's type is still visible
  (`emit/winmap.go`). The rewrite decides "is a map" by following binders.
- A `var m := map(c)` buffer is handed on by the state translation through a tuple's elimination: a
  borrow answers (result, buffer), and its continuation binds the buffer again,
  (λk. k () m′)(λ(_, m). M). The tracker followed a plain β-redex and not this one, so after the
  first store `len(m)` was the backing table's length, 3·4 = 12.
- **β of a tuple** is M[x⃗ := e⃗], so xⱼ holds a map exactly when eⱼ does. That is now the tracker's
  rule.
- Witness: `TestAMapsLenIsItsDomainsSize`, three keys and one key written twice, 3 on Go and windows;
  `PlantMapTupleLost` gives the table's length again.

## 64. A tuple pattern on a table's element is its projections

**`(x, y) = p[i]` on a table of pairs emitted a map's comma-ok read of a slice** on Go, and on
windows a read nothing prints.
- A table of products is flattened by currying (products.md), Π_{(i,j)} V ≅ Π_i Π_j V, so element i's
  component j is at k·i + j. The flattening pass had the projection ((t i) j) and the field store,
  and not the **eliminator**.
- By η for products, p = ⟨π₀ p, …, πₖ₋₁ p⟩, so ((t i) (λx⃗. M)) = (λx⃗. M)(t(k·i + 0), …,
  t(k·i + k − 1)). The index is pure after β (ADR 0010 let-binds anything impure), so it may be
  repeated (`emit/product.go`).
- Missing, the application reached lowering in the shape of a map read under its option's
  eliminator, ((m k) (λ(#t, #p). b)), and was read as one. A guard on the operand's type in lowering
  was written first and removed: the types are not known there, and with the pass fixed the shape
  no longer reaches it.
- Witness: `TestATuplePatternOnAnElementIsItsProjections`, Σ 10x + y over (k, k + 1) for k < 4,
  which is 70 on Go and windows; `PlantNoProductElim` fails the build again. The table is filled by
  stores, since a table of pairs by a rule or a literal is finding 66, not built.

## 39. One double compared three times verifies

**`x = round(F64, 0.1)` compared by `>`, `<` and `==` with 0.1 failed the IR's own verifier**, W6,
Theorem E: a π not implied by its arm's guard.
- For x ∈ F64 and q ∉ F64 (0.1 lies strictly between the doubles ⌊q⌋ = 0.09999999999999999 and
  ⌈q⌉), no double lies between ⌊q⌋ and ⌈q⌉. The Galois connection rewrote x > q to x ≥ ⌈q⌉, and,
  separately, the else arm's π x ≤ q to x ≤ ⌊q⌋. Both are true, but the second is no literal of
  the guard, whose negation is x < ⌈q⌉.
- **One constant for all four relations**: x < q ⟺ x ≤ q ⟺ x < ⌈q⌉, and x > q ⟺ x ≥ q ⟺ x ≥ ⌈q⌉.
  Then the rewrite commutes with negation, galois(¬ρ, q) = ¬galois(ρ, q), and a π rewritten apart
  from its guard is the guard's literal or its negation (`ir/doubles.go`).
- A cache of one name per constant was written too and removed: W6 compares constants by value,
  and no program needed it.
- The answers are gt, not lt, not eq, since the double nearest 0.1 lies above it (big.Rat:
  Cmp = 1). They agree on Go, windows and JavaScript.
- Witness: `TestOneDoubleComparedThriceVerifies`; `PlantGaloisTwoEnds` fails W6 again.

## 40. A double beside a rounded decimal is refused, saying why

**`round(F64, 1 ÷ 3) < round(D(18, 0..1), 1 ÷ 3)` crashed the compiler.** numbers.md §6.2 refuses
it, and the refusal's message showed the comparison's source term, which lowering had not attached.
`srcOf` names the operation where the term is absent ("a comparison by <"). All three messages in
the file that showed `s.Src` use it. Witness: `TestADoubleBesideARoundedDecimalIsRefused`, refused
for its reason on Go and windows; `PlantSrcUnguarded` crashes again, caught by `recover`.

## 78. The corpus is the repository's files

**`go test ./core/ ./cmd/gen/` failed about one run in five.** `TestEveryCorpusFileReadsInJ`
walked the working tree. `cmd/gen`'s tests write temporary programs under the repository, where
`lib/` is, and remove them, so a walk listed a file that had vanished by the read. It would also
have read a test's deliberately invalid program as corpus.
- The corpus is a set the repository defines, its tracked files, not the tree at an instant. It is
  read from git's index (`git ls-files`); a staged file is in it.
- Witness: `TestTheCorpusIsTheRepositorysFiles`, an untracked invalid program in the tree is not
  read, and with `plantCorpusWalk` it is.

## 6. Not built: a map's capacity

maps.md §6 decided that a map has a declared capacity, and that "inserting beyond it is a condition
the program must handle", so the four targets agree. None enforces it today:
- Go, JavaScript and the JVM grow;
- windows drops a key once its ≥ 2c slots are full, which is an observable disagreement beyond
  2c keys.

The algebra is ADR 0119's: a map of capacity c is m : K ⇀ V with |dom m| ≤ c, so an insert
m[k ↦ v] is defined iff k ∈ dom m or |dom m| < c, a partial operation whose domain is an obligation.
Discharging it needs facts the refinement layer does not have:
- |dom m| = 0 at `map(c)`;
- an insert adds at most one key;
- a loop's invariant |dom m| ≤ i.

A program guarding it needs `len(m)` in O(1) on windows, which means a count cell. That is the next
step.

## Paid and bought

- **Compile time**: 0.98× the baseline on the check of all five, no compile confirmed slower.
- **Run time, code size**: no corpus program's emitted text changed, on any target: the fixes are
  reached only by the book's programs and the witnesses, so there is nothing to time. Chapter 8's
  comparison with a decimal is now written with ⌈q⌉ (`x < 0.1` for `x < 0.1`), and its passage now
  says what that means and where C's reading differs (`<=`).
- **Proofs**: unchanged, 8,852 of 8,906 operations and 924 of 962 loops.
- **Refusals changed**: the double's comparison is refused instead of crashing; finding 39's program
  and 64's build.
- **Compiler lines**: non-test +101 / −11 (`emit/winmap.go` +27, `emit/product.go` +25,
  `ir/doubles.go` +45 / −10, `ir/lower.go` +4 / −1, a comment). Tests +216 / −13: three new files
  (148 lines) and `core/jcorpus_test.go`.
- **Rules made less general**: none.
