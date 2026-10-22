# 0070 — A header is the caller's view of a borrow, and a write-borrow hands back its own storage

Date: 2026-10-22
Status: Accepted (hamza, 2026-10-21/22: "should we say so in the declaration?", "should we drop the func
wrapper too?", "go ahead"). Amends [ADR 0052](0052-a-buffer-is-a-state-binder-holding-a-table.md) item 4
("the write-borrow wrapper stays"), whose stated condition for removing it, that the IR know a
write-borrow returns the same storage, this ADR meets.

## Context

Since ADR 0052 a call that borrows a buffer is written in program order, `n = hex.Encode(buf, src)`, and
so is a definition that borrows one: `fn mark(s linear, i, n)` returns the unit, not `s` (item 3:
borrowed by reference). But a host declaration's header still stated the threaded result, the core's,
with the buffer handed back first: `(buffer(0..255) × 0..2^63-2)` for `hex.Encode`. So one mode,
`linear`, meant two things in two kinds of header, a synonym in the abstract syntax (fit-research.md).

To manufacture that pair, every host call that writes a buffer in place carried a closure,
`func(dst, src []byte) ([]byte, int) { return dst, hex.Encode(dst, src) }(…)`: twelve declarations,
`hex.Encode` and `Decode`, the varint puts, `utf8.EncodeRune` and six `Read`s. Go inlines it, but it is
not what a Go programmer writes.

host-buffers.md §1 had already split aspect 1, a host that writes its argument, into two arrows:
- a **write-borrow** writes the buffer and returns something else, so the buffer after the call is
  **the same storage**;
- a **consume-and-return** may grow it, so the buffer after the call is **storage the host returns**,
  possibly new: every Go `Append*`.

Both are B ⊸ B ⊗ R in the core. The difference is representation, not value.

## Decision

1. **A host declaration's header is the caller's view.** Its result is what the call gives. A `linear`
   parameter is borrowed by reference, exactly as on a definition, and handed back first in the core's
   threaded result, which J's target reader **derives**: each borrowed buffer, in order, then the
   declared results, a unit dropped (B × 1 ≅ B). That is the state translation S's own reading of a
   borrow (G7).
2. **A primed result names storage the host hands back**: `fn AppendInt(dst ∈ Array(0..255) linear, …)
   (dst' ∈ buffer(0..255))`. A `linear` parameter with **no** `p'` among the results is written **in
   place**, and its handed-back component is the argument itself. The reader records that as
   `(inplace j i)`, component j being argument i, carried by the host body through the abstract
   syntax. `x'` is ADR 0066 item 7's "a binder's value on exit", here on a `linear` binder; spec/j.md's
   J2 widens to it.
3. **The law that tells the two apart is stated in `ensure`.** A write-borrow keeps its length,
   `len(dst') == len(dst)`. An append grows, `len(dst') > len(dst)` or `>=`. `p'` in an `ensure` is
   the handed-back component's position.
4. **The template is the host's own call.** For an in-place buffer the target derives a **host
   primitive** with the host's own results, the threaded ones without the in-place components. The IR
   calls it and binds each in-place component to its argument's value (`ir/lower.go`, `threadedCall`).
   So the core, the type checker, the refinement layer and linearity see the threaded shape they always
   saw, and Go gets `hex.Encode(v8, v3)`. The wrapper is gone from every declaration.
5. **The hand check compares the host primitive's results with Go's manifest exactly.** Its old
   allowance, "a write-borrow's result list may begin with the buffer it hands back", is removed. So
   `AppendInt` declared in place (it returns the grown slice, which would be lost) and `Encode` declared
   as handing back storage (it returns only `int`) are each refused on the result count, each with a
   planted mistake.
6. **The old spelling is refused with its reason**: a buffer among the results of a declaration that
   borrows one, unnamed, is "name it `p'` when the host hands back storage, or leave it out when the
   host writes p in place".

## Why not

- **A splice `${dst'}` in the template marking new storage**, the first proposal. It turns every append
  into a statement assigning its destination (`var v5 []byte; v5 = strconv.AppendInt(…)`), which is
  worse Go than the expression it is today, and it hides in the host text a fact the caller needs: that
  the call may hand back different storage. The header's primed result says it where the reader of the
  declaration looks, and leaves both templates the host's plain expressions.
- **The mode `inout` for a write-borrow.** Ada's `in out` is the right intuition, but `inout` already
  means ADR 0048's cell, a reference to a fresh copy, and on a table type it would mean copy-in and
  copy-out, the opposite of in place.
- **`ensure dst' == dst` to mark the same storage.** After a write-borrow the contents differ, so the
  values are not equal. Same storage is a fact about representation, not a claim of the logic (ADR
  0049), and the primed result's absence is where the declaration says it.
- **Keeping the wrapper.** Go inlines the closure, so it cost no time. But the emitted source is held
  to what a good programmer writes, and the header contradicted the program-order call.

## Consequences

- Every `linear` host declaration reads the way it is called, and states its length law.
- A write-borrow whose host returns nothing at all (a `void` writer) is refused for now. There is no
  host primitive with no results, and none of Go's thirty borrowing declarations needs one.
- The JavaScript, Java and Windows targets declare no borrowing host call today. A first one follows
  the same rule, and its template is its host's own call.
- The core is unchanged: the derived threaded primitive is today's, so analyses and proofs see the same
  shapes.
