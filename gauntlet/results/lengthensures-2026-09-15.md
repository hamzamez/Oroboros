# A length is a postcondition: `(length N)` and `(length-of N)` respelled as `ensures`

2026-09-15. [spec/theories.md §8.4](../../docs/spec/theories.md),
[target-files.md §4 `length`](../../docs/spec/target-files.md). Unblocked by
[purecontract-2026-09-15](purecontract-2026-09-15.md).

## 1. The algebra

A host call that builds or threads a container makes a claim about its result's length. There are two
shapes, both equations with one extension term, the length of the result:

- a **count**: `len(f(x̄)) = x_N`. `make([]bool, n)` is `n` long.
- a **pass-through**: `len(f(x̄)) = len(x_N)`. `c[i] = true` is as long as `c`.

Those are postconditions, `∀x̄. Q(x̄, f(x̄))`, and the language already has the clause for one. They
were written instead as two positional attributes, `(length N)` and `(length-of N)`, stored in
`Prim.Length` and `Prim.LengthOf`. theories.md §8.4 respelled them and gated the respelling on
postconditions for pure calls, which became available that day.

**Now** they are `(ensures (= (len result) n))` and `(ensures (= (len result) (len c)))`. The old
spellings are refused, naming the new one, and the two `Prim` fields are deleted. An `ensures` names
what it constrains, so the 25 declarations' positional parameters are now named.

## 2. The translation is checked as a commuting square

The fields change, so a structural comparison of the loaded targets is impossible. The square is taken
on what the compiler *reads*.

`lengthContract(p)` is the inverse of the respelling on these two shapes. It scans `p.Ensures`'s
conjuncts, in either orientation of `=`, and returns `(count | pass-through, argument position)`. The
property is `lengthContract(T(d)) = (Length, LengthOf)(d)` for every declaration `d`.

**Measured**: loading all five targets gives **40 contracts**:
- the **25 declarations** in HEAD's target files (Go 9, Java 8, JavaScript 4, the two portable layers
  2 each), each with the same kind and the same argument position as HEAD's attribute;
- the compiler's own `build`, `alloc` and `set` on each of the five targets (15). They carried the
  attributes as Go literals and state the same postconditions, built as terms by `lenEquals`.

**No other primitive claims a length.** `set-map` and JavaScript's `set` still claim none, which the
existing tests pin through `lengthContract`.

## 3. Witnesses

- `TestLengthFromACount`, `TestLengthThroughAThreadedStore` and `TestAnUndeclaredLengthProvesNothing`
  pass unchanged: the sieve's proof, the threaded store's loop invariant, and silence where nothing is
  declared.
- `TestAMapStoreClaimsNoLength` and `TestAJavaScriptArrayStoreClaimsNoLength` read contracts: an array
  store passes its length through, and a map insert and a JavaScript store claim nothing.
- `TestLengthAttributesAreRespelledAsEnsures`: both old spellings are refused, naming the postcondition.

## 4. Cost

- **Code**: `target.go` 2,322 → 2,322, the deleted fields and parse case balancing `lenEquals` and the
  refusal. `refine.go` 809 → 854, **+45**, which is `lengthContract`.
- **`go run ./cmd/check`, every step, passes**: emission 194 of 194 files byte-identical, no compiler
  note changed, proof counts identical (2,307 of 2,413 integer operations, 345 of 382 loops),
  differential on four targets, tooling.
- **The one effect it could have had did not appear.** An impure allocation's `ensures` is also assumed at
  its `let` binder, so `len(c) = n` now arrives in the fragment's own spelling, where HEAD recorded it
  only under the target's `alen`/`slen` names. It decides nothing new in this corpus.

## 5. Not built

- A length postcondition in any shape other than these two (a sum of lengths, say) is read as an
  ordinary `ensures` and not by `valueLength`.
- Checking that an `ensures` names only parameters the `sig` declares.
