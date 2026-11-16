# 0117 — A string is the table of its UTF-8, and its operations are the language's

Date: 2026-11-16
Status: Accepted (hamza, 2026-11-16: "Go ahead, always mathematical and algebraic", on the proposal to
replace the LLVM layer's string templates).

## Context

On windows the language's four string operations were LLVM functions written inside strings in
`targets/windows/strings.oro`:
- `string_of`, η : Scalar → Σ\*, the free monoid's generator;
- `++`, its operation;
- `ascii`, ι : (B₇)\* ↪ Σ\* (ADR 0095);
- `utf8_text`, ι_V : V ↪ Σ\* (ADR 0112).

ADR 0089 had decided that a realization is a term over the host's operations, checked for
definedness and agreement, because a template is text the compiler copies and cannot read. These
four stayed text. Measured, that cost (strings-2026-11-16):
- **a wrong answer accepted**: the template declared `a ∈ Int`. Go's realization states the
  scalars' domain, so `string_of(55296)`, a surrogate, and `string_of(2^30)` were refused on Go and
  compiled on windows into bytes outside V;
- **every string leaked**: each helper called `HeapAlloc` directly, past Σ_OS's `region`, so neither
  release nor reuse saw it (R1e);
- **a byte loop clang could not vectorize**: each copy was a `"no-builtins"` loop;
- **x86-64 hard-coded**: `HeapAlloc(ptr, i32, i64)` is the wrong prototype on i686.

## Decision

1. **A string is the table of its UTF-8.** UTF-8 is a monoid isomorphism ρ : Σ\* ≅ V onto the valid
   UTF-8 V ⊂ B\*, and on the LLVM layer ρ is the identity on the representation:
   - `llvm/str.utf8` is ρ, with `ensure utf8_valid(r)`, since ρ's image is V;
   - `llvm/str.text` is ρ⁻¹, with `where utf8_valid(b)`;
   - `llvm/str.text_ascii` is ρ⁻¹ on (B₇)\*, which needs no obligation, because V's automaton loops at
     its start on every byte below 128.
2. **Each string operation is a definition in the language** (`targets/llvm/lang.oro`), a table
   read back by ρ⁻¹, realized on every OS layer:
   - `encode` is Table 3-7's rows, under the scalars' domain, with LLVM's own `sdiv` and `srem`;
   - `cat` is the rule x·y;
   - `ascii_of` and `text_v` are copies.

   Each table is the allocator's, so it is stored, decided, released or escaped, and printed like
   any other.
3. **V is a submonoid of B\*, and the decision says so.** V = φ⁻¹(F) for φ into the transition monoid
   of V's automaton, with F = {q₀} and q₀ initial, so δ(q₀, xy) = δ(δ(q₀, x), y) = q₀. `utf8_valid`
   is decided on a term (`emit/requires.go`):
   - on a literal, by φ;
   - on `alloc(e)`, by e;
   - on the rule `k ↦ if k < len x {x(k)} else {y(k − len x)}` of length len x + len y, by x and y
     (the length is proven, since a prefix could cut a sequence);
   - on a copy of x of length len x, by x;
   - on an `if`, by both arms;
   - on a name, by an identical assumption: a guard, a `let` whose term is in V, or an `ensure`.

   So `++` costs no pass. `encode`'s bytes are computed, and V on computed bytes needs a fact per
   position that the decision does not keep, so a guard of at most four of V's steps decides it.
   Agreement is proven by enumeration over the whole domain: every one of the 1,112,064 scalars
   encodes as Go's `string(rune(c))` (`TestEveryScalarEncodes`), so the guard's other arm is dead.
4. **A string's NUL is the allocator's.** Every table's region holds one zero byte past its elements,
   b(n, w) = n·w + 9 (`lang.bytes`, one definition for allocation, release and reuse). It is written
   by `region`'s zero fill, and in a raw region by one `store8`. So a table of bytes is a C string,
   x·0, and ADR 0091's LPCSTR and ADR 0094's `s + 8` hold unchanged, with no copy for the NUL.
   Σ_OS gains `max_len`, the longest table, which `cat` compares a join's length with: past it is
   exhaustion.
5. **Definedness covers every host operation's `where`.** A realization's instance is now decided by
   the refinement layer as a program is (`ir/realize.go`). It had run only the IR's decision, so a
   term could call a host operation outside its domain and be inlined: ADR 0089's definedness had
   held for the integer operations alone.
6. **An identity coercion is a renaming**, as a π is. `str.text`'s String is its table's own
   storage, so the release pass reads the coercion as a move (an escape), never a borrow.

## Why not

- **Keep the templates and add the `where`**: the domain would be right, and nothing else. The
  storage leaks, the loops stay unreadable, and i686 needs a second copy of the text. ADR 0089 is
  the reason, and its context paragraph names this failure.
- **The NUL at the C boundary, not carried** (a copy x·0 made at each LPCSTR argument): it is sound,
  and ι_V would be the identity with no NUL to place. But the copy then moves to every C call that
  takes a string. Carrying the NUL in the allocator costs one byte a table and no copy anywhere,
  which dominates.
- **Decide V on `encode`'s computed bytes** instead of the guard: it needs a relational fact per
  position (the second byte's range given the lead's), which the refinement layer does not keep. The
  guard costs at most four steps where it runs, and its dead arm is proven dead by enumeration.
- **ι_V as the identity now**, b moved into its string: the coercion is a move, so this is sound for
  a whole table. A segment, though, is a view with no header of its own, so `text_v` stays a copy
  until a view's coercion is a copy and a table's a move. That is the step after this one.
- **A `pub`-fenced coercion** that only realizations may call: target modules carry no `pub`
  (ADR 0078 is for program modules). `str.text`'s `where` is decided at every call, a program's
  included, so no fence is needed for soundness.

## Consequences

- `targets/windows/strings.oro` is deleted, and no LLVM text is left in the targets except a host's
  own instructions.
- `string_of` outside the scalars is refused on windows, as on Go.
- Strings are allocated through `region`. They are still not released (R1e): a realization's tables
  are made after the release pass, and a coerced table escapes.
- Witnesses:
  - `TestATermsHostWhereIsDecided`, with `PlantCatValid` and `PlantTermWhereUnchecked`;
  - `TestAnIdentityCoercionMovesItsTable`, with `PlantCoercionBorrows`;
  - `TestEveryScalarEncodes`, by enumeration.
