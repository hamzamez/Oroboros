# j.md's two promises: `invariant`, Hoare's rule, and `try` with several binds

2026-11-16. Item 2 of [assessment-2026-11-16](../../docs/assessment-2026-11-16.md) §5, the book's
findings 60 and 77. hamza: "Go ahead, always mathematical and algebraic".

**`loop … invariant I { … }` is read and decided by Hoare's rule, and `try b₁, …, bₙ { e }` is the
nested bind.** readfile's clamp on windows is gone. It turned out that what readinto owed was two
facts, not a stronger rule. One is the out cell's set: ReadFile's count is a DWORD. The other is the
store law for a buffer's version: len t = len z. With both, Houdini's own template proves
lo ≤ len(b), and the invariant readinto states is checked rather than needed. What the rule adds is
a relation outside Houdini's templates, such as s ≤ 2·i, whose coefficient no template has. A
program indexing with it was refused before and is accepted now.

## 1. The algebra

- **Hoare's while rule.** {I ∧ g} B {I} for each arm whose tail is `again`, and I(z⃗) at entry,
  give I at every iteration (Hoare 1969; Floyd 1967). The loop is decided as Houdini's greatest
  inductive subset of a finite candidate set (Flanagan and Leino 2001), so a stated I is given to
  Houdini as **required** candidates, its conjuncts. A conjunct Houdini keeps is preserved by every
  back edge assuming the kept set, which is the induction step. One that is dropped refuses the
  program, and so does an I(z⃗) not proven at entry.
- **I is a claim, never run** (ADR 0049). It travels as ADR 0028's where-mark,
  `(#reqw "a loop's invariant" (fn (x⃗) I) loop)`, whose value is its body's. Every pass between D
  and lowering therefore reads the mark as its body: tailArity, R3's place check, S, spliceTails.
  The program-order theorem (S after erasure ε) is unchanged.
- **A proven claim is a fact.** DischargeRequires decides the mark and then strips the marks, but
  Hoare's rule also assumes I in the body, and the refinement layer's walk comes after the strip. So
  a decided invariant stays as `"a loop's invariant, proven"`. The refinement walk hands it to
  Houdini again and owes nothing. Lowering reads it as its body, and it never prints.
- **The invariant's variables are the program's.** S threads a buffer's version (`#vb`) and a
  scope's cells beside the program's own variables, so I's variables are matched to the loop's by
  name. A buffer or cell in scope may appear in I only as `len(b)`, which no store changes
  (len ∘ set = len ∘ π₁). An element would need the loop head's version, which is not built.
- **len t = len z for a threaded version** is the store law carried round the loop. It is a pair of
  Houdini candidates for each loop variable t whose initial value is a name z and whose length the
  body reads, and Houdini checks it like any other candidate.
- **A cell's value is in its cell's set** (ADR 0048, 0094). The host writes a value of τ through
  `(#fresh "τ")`, so `(#deref r)` ∈ τ, and DWORD unfolds to 0..2^32-1.
- **A rule (table n λ) is n long** (tables.md §2), read as a count is. `alloc(rule)` passes the
  length through (ADR 0115), and had none.
- **Bind is associative**: try b₁, …, bₙ { e } = try b₁ { try b₂, …, bₙ { e } }. The reader folds
  the comma list to the right, so each bᵢ sees the names bound before it.

## 2. What failed first

1. **The mark hid the loop's result arity** from tailArity. Then R3 read the invariant's λ as a
   closure over the buffer, S refused it as a closure over a cell, and spliceTails met "a tail that
   is no tuple". Each now reads a mark as its body.
2. **readinto stopped being kept** (P8). Its invariant failed on entry as `0 <= b && b <= len(b)`,
   because I's variables were mapped to the loop's by position, and the translated loop begins with
   the buffer's version. P4 then inlined everything, and roundtrip hit the reducer's step limit.
   Matching by name fixed the entry.
3. **The induction step failed on two missing facts**: `0 <= lo + k` lacked g ≥ 0, since an out
   cell's read had no set, and `lo + k <= len(b)` lacked len(#vb) = len(b). My first filter for the
   length candidate asked for `len`, which the fragment spells `alen`, and matched nothing.
4. **With readinto's invariant removed, readfile still built without its clamp.** The two facts
   alone let Houdini keep lo ≤ len(#vb), its template from the guard `lo >= len(b)`. So the clamp
   was a missing fact, not a missing rule, and the invariant is a checked statement there.
5. **The witness's first program was refused for another reason**: `alloc(array { … : k ∈ 0..200
   })` had no length, so even `t[i]` with i < 100 was unproven. The rule's count is its length now.
6. **A proven invariant was not assumed where the index is decided.** The decision of marks proved
   s ≤ 2·i, and Houdini kept and assumed it, but the refinement walk runs after the marks are
   stripped and saw a bare loop. The decided mark now stays.
7. **A reducer rule I added proved unneeded** and made a test fail. It pushed a context into a mark
   in operator position, `(mark c B) k = mark c (B k)`. Removing it changed nothing in the corpus or
   the witness, and its presence alone enlarged normalizeStep's frame, so Ω
   (`TestDiagnosticsUnderABinderKeepTheirNames`) overflowed Go's 1 GB stack before its fuel ran
   out. The rule is deleted. That test depends on frame size, which is a fragility of its own.
8. **`PlantInvariantUnchecked` was planted inverted** (`all && !plant`), and the witness caught it.

## 3. The witness

`TestALoopsInvariantIsHoaresRule` (cmd/build, Go and windows):
- **built, with its output**: `walk`, where s rises by 1 or 2 a step and `invariant 0 <= s && s <=
  2 * i` proves `t[s]` in a table of 201 (`142 9`). Without the invariant it is refused, so the
  invariant is what proves it. Also an invariant over a lent buffer, `lo <= len(b)` (`100`), and
  `os.ReadFile` on windows through readinto (`12`);
- **refused, each saying why**: an invariant not kept (`at every iteration`), one false on entry
  (`on entry`), and one over a buffer's element (`other than through its length`);
- **planted faults**:
  - `PlantInvariantUnchecked` accepts s ≤ i with s rising by 2;
  - `PlantInvariantErased` (the decided mark erased) and `PlantRuleNoLength` refuse `walk` again;
  - `PlantMarkIsAClosure` refuses the lent buffer;
  - `PlantNoVersionLength` and `PlantCellValueUnbounded` refuse the windows reader.

`TestTryWithSeveralBindsIsNested` (core): the comma form and its nesting reduce to one normal form,
for two independent binds and for a chain. `PlantTryCommaReversed` nests them last-first and must
differ.

## Paid and bought

- **Compile time**: 1.01× the baseline in the first sweep. Five suspects were paired serially
  against `gen` at 18f2f5d, and none was confirmed slower.
- **Run time**: not timed, because the only change is a deletion outside every loop. readfile's
  clamp was two comparisons and three stores once per file read, 19 lines of LLVM IR per inlined
  instance.
- **Code size**: eight windows files shrink, and every change is a deletion once SSA numbers are
  normalized: −38 lines each for crc32, fnv, count, freq, jsonfmt, wc and ledger (two readfile
  instances each), −19 for roundtrip. Go, JavaScript and the JVM are byte-identical. Accepted with
  `-accept`.
- **Proofs**: unchanged, 8,852 of 8,906 operations and 924 of 962 loops.
- **Refusals changed**:
  - a stated invariant that is false on entry or not kept is refused, where before `invariant` was
    not read at all;
  - an index proven only by a stated invariant, or below an allocated rule's count, is accepted;
  - `try` with several binds is read.
- **Compiler lines**: non-test +451 / −26:
  - `emit/refine.go` +153 / −3: the required candidates, the version's length, the rule's length;
  - `emit/requires.go` +94: the invariant's decision, the name matching, the message;
  - `core/cell.go` +53 / −2, `core/jparse.go` +47 / −15, `emit/content.go` +34 (the cell's set),
    `core/desugar.go` +30 / −1, `core/reduce.go` +15 / −5 (the proven mark kept),
    `core/place.go` +14, `ir/lower.go` +7, `core/abs.go` +4.

  Tests: +164 (`cmd/build/invariant_test.go` 123, `core/capture_test.go` +41), and one inventory
  entry deleted.
- **Language lines**: `lib/os/windows.oro` +7 / −9, the clamp gone and the invariant stated.
- **Rules made less general**: none. An invariant may name a buffer or cell in scope only through
  its `len`, which is a restriction of the new construct, not of an existing one.

## What was not built

- **An invariant over a buffer's elements or a cell's value**: it needs the loop head's version
  under its own name in I, which is S's to give.
- **Chapter 11's `p <= N`** is read and decided, and refused, as it should be. p·i ≤ N does not
  follow from p ≤ N, because the invariant that carries it is p = (i − 1)!, outside the linear
  fragment. The guard `p <= 4 * 10^17` stays.
- **`try`'s handlers** (`else`, ADR 0057), still not built.
