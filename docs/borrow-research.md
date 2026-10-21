# A borrow that a later call ends

Research, 2026-10-21. This is coverage cause **C** ([gocoverage-2026-10-06](../gauntlet/results/gocoverage-2026-10-06.md)),
the wall the 10-20 plan's item 3 predicted `bufio`'s `Reader` would meet. hamza's instruction:
*"go ahead with bufio's Reader, always mathematical and algebraic."*

`examples/lines/shape.oro` met it ([shape-2026-10-21](../gauntlet/results/shape-2026-10-21.md)).
**Not decided**; six decisions for hamza are in §8. Experiments are in §2 and can be reproduced from
the programs quoted there.

> **Six findings, in the order they change what should be decided.**
>
> 1. **C is a wall of cost, not of expressiveness.** Every lent function in `bufio` but two has a
>    copying twin: `ReadSlice` and `ReadString`, `ReadLine` and `ReadString`, `Bytes` and `Text`.
>    `shape` is written today with `ReadString`. Its 5 operations are proven, it matches its reference,
>    and it runs at parity with the same algorithm written by hand (1.01×). The program a Go
>    programmer writes, with `ReadSlice` and `bytes.Count`, takes **0.74×** the time on 260 MB, with
>    **0 garbage collections against 78**: one allocation per record. The two without a twin are
>    `Peek` (lookahead without consuming) and `Writer.AvailableBuffer` (a buffer lent for writing).
> 2. **Declared naively, a lent result gives wrong answers, including in a program that is correct
>    in source order.** `ReadSlice` declared as an ordinary table compiles. A program that makes its
>    copy, `go.string_of_bytes(first)`, before the next read prints the overwritten bytes: the copy
>    is pure, used once, and call-by-need substituted it past the read. So the wall is in the
>    reducer as well as in a check. ADR 0010 sees effects, but not the state a value depends on. This
>    is effects.md §7c's lesson again: then a buffer's read, now every read of a lent table.
> 3. **A lent value is a value whose representation is borrowed for an epoch.** Its meaning is
>    unchanged: `ReadSlice` returns the next factor's bytes, an element of `Array(0..255)`. Its
>    representation is a view of the receiver's state, and it is faithful exactly while that state
>    is not written. Theorem L (§3): if every read of the view happens before the next write to its
>    extent, the program's meaning with the view equals its meaning with a copy. Outside that
>    condition the program is refused and copies explicitly. A hidden copy would be the hidden
>    allocation the core forbids.
> 4. **It is a known construct under three names.** It is Wadler's `let!` (1990) and Odersky's
>    observers (1992): read-only access to a linear value, which may not escape the access. It is
>    Reynolds' passive alias under syntactic control of interference (1978), the discipline ADR 0052
>    already uses for buffers. It is Rust's shared borrow from `&mut self`: `BufRead::fill_buf` is
>    `ReadSlice`, non-lexical lifetimes (RFC 2094) end it at its last use, and the lending iterator
>    (generic associated types, RFC 1598) is `Scanner.Bytes`.
> 5. **Which writes end the epoch is the whole design.** E0: every impure call, which is sound
>    without knowing anything about aliasing, because a pure call writes nothing. E1: the calls that
>    may reach the receiver, which needs the receiver owned. **A `bufio.Reader` is not fresh in
>    general**: `NewReaderSize` returns its argument itself when that argument is already a large
>    enough `*bufio.Reader`, so two names may hold one Reader. E2: declared effect regions, a claim
>    no manifest justifies (ADR 0023). `shape`'s `ReadSlice` form needs only E0.
> 6. **The standard library lends about 20 times, in two shapes.** A lent slice: `bufio` 9 (with
>    ReadWriter's promoted methods), `bytes.Buffer` 4, `encoding/xml`, `encoding/csv` under
>    `ReuseRecord`, `database/sql`'s `RawBytes`, and `crypto/tls`'s QUIC events. A lent object, a
>    host value valid until the next call: `net/textproto.Reader.DotReader` and
>    `encoding/json.Decoder.Buffered`. Both shapes have one algebra.

---

## 1. The question, stated as algebra

A `bufio.Reader` holds the unread input. Its state is a suffix y of the input x, and every read
consumes a prefix of it (targets/go/bufio.oro). Fix a delimiter d. The words ending at their first d
form a prefix code C_d = (B ∖ {d})*·d. So x factors uniquely as c₁ … c_k u, with cᵢ ∈ C_d and
u ∈ (B ∖ {d})*.

- `ReadString(d)` returns the next factor as **a new string**: a value, lossless, independent of the
  buffer's size.
- `ReadSlice(d)` returns **the same factorization refined by the buffer**. A factor longer than the
  buffer N comes in pieces of N bytes, each with `ErrBufferFull`, and every piece is
  `b.buf[b.r : b.r+i+1]`, **a slice of the Reader's own buffer**.

host-buffers.md wrote a Go slice as its extent ρ(v) = [ptr, ptr + len) and stated ADR 0018's
invariant:

> **(I)** For any two live references u ≠ w: if either is a buffer, ρ(u) ∩ ρ(w) = ∅; and no
> operation writes into ρ(a) for an array a.

`ReadSlice`'s result breaks the second half of (I), not at the call but at the **next** call. Its
extent lies in the Reader's buffer, and the next read writes there: `fill` slides the unread bytes
to the front and reads more, and Go's documentation says *"The bytes stop being valid at the next
read."* None of Aspinall and Hofmann's three usage aspects, which classify what a call does to its
arguments, says this. Here a result aliases **the receiver's private state**, and a later call
writes it. It is aspect 2 turned around: shared, read-only for now, but written later.

So the question is: **when is the value the program means, the bytes at the call, the same as the
view it holds?**

## 2. Measured, before anything was argued

### 2.1 What C would buy

`shape` (examples/lines/shape.oro) folds over the factorization with `ReadString`. The input is a
260 MB TSV file of 2.5 million records, with a record of 100,000 bytes in the middle, every 1,000th
record ragged, and the last unterminated. Each program was run 9 times, serially and interleaved;
the times are medians.

| program | time | GC cycles | output |
|---|---:|---:|---|
| `shape`, emitted (ReadString, strings.Count) | 308 ms | 78 | identical |
| hand-written Go, the same algorithm | 304 ms | 77 | identical |
| hand-written Go, `ReadSlice` and `bytes.Count` | **227 ms** | **0** | identical |

The language is at parity with itself written by hand (1.01×). The gap is the copy: `ReadString`
allocates a string per record, and the lent form allocates nothing. 0.74× is well outside the 15%
noise floor. The hand-written programs are in the result's scratch record (shape-2026-10-21 §4).

### 2.2 What a naive declaration does

`ReadSlice` was declared in a scratch layer with an ordinary table result:
`(Array(0..255) × result.result((), error))`. Two programs read two records, a first record of
4,000 `A`s and a second record of 200 `B`s. The second record crosses the 4,096-byte buffer, so the
second read slides the buffer.

| | program | the language says | printed |
|---|---|---|---|
| **A** | `first`, then `second`, both read; `first` printed after | 4,000 `A`s | **`BBB…`, 3,799 `A`s and 200 `B`s** |
| **B** | `a = go.string_of_bytes(first)` **before** the second read | 4,000 `A`s | **the same wrong bytes** |

**A** is the hazard Go documents. **B** is the finding. In source order the copy precedes the read,
but the emitted Go is

```go
v3, v4 := v1.ReadSlice(uint8(10))
v6, v7 := v1.ReadSlice(uint8(10))
v8 := (string(v3))          // the copy, moved past the read that overwrote v3
```

because `string_of_bytes` is pure, its value is used once, and β substitutes it (ADR 0010 denies
weakening, contraction and exchange only to impure terms). A program written correctly is
miscompiled. Any check that runs after reduction sees B's residual, which is A, and can only refuse
it. **The reducer must not move a read of a lent value past an impure term**, which is §7c's rule
generalized (§6).

## 3. The algebra: a value, a representation, an epoch

**The meaning does not change.** `ReadSlice(d)` denotes the bytes of the next piece, a value in
`Array(0..255)`, exactly as `ReadString` denotes them in `bytestring`. ADR 0049 separates what a
program means from how a value is held, the one bridge being ρ from proven sets; here ρ is a view
instead of a copy, and the proof it needs is temporal.

**Epochs.** Impure terms happen once each, in the position written (ADR 0010), so the impure events
of a run are totally ordered: e₀ < e₁ < …. Say v is lent at the event e_v that produced it, and let
e⁺ be the first impure event after e_v. The **epoch** of v is the open interval (e_v, e⁺).

**Theorem L.** If every read of v, and of every value derived from it that may share its extent,
happens within its epoch, the program's observable behaviour with v held as a view equals its
behaviour with v held as a copy taken at e_v.

*Proof.* Inside the epoch only pure events occur. A pure term has no effect, so it writes no memory
(effects.md §1). The host runs only inside an impure call: there is one thread, and concurrency is
undecided (concurrency-research.md). So the view's extent is unchanged from e_v to e⁺, and every read
inside the epoch reads the bytes the copy holds. Reads are the only way the representation is
observed. ∎

The premise "the host runs only inside a call" is the one that concurrency would break. A goroutine
writing the Reader's buffer between two calls of ours would end the epoch with no event of ours.
This is recorded so that concurrency-research's decisions revisit it.

**Escape is the same condition.** A lent value stored into a buffer, a cell or a map, carried by a
loop past the next read, returned out of the program, or kept by a host is read after its epoch, or
may be. So the rule needs no separate escape clause. It is Theorem L applied to the **alias
closure** A(v): v, every loop or join parameter that receives a member, and every result computed
from a member that may share its extent.

**Which results share an extent is a property of the host representation, not a declaration.**
- A scalar never shares one.
- A Go `string` never shares one: a `[]byte → string` conversion copies, by Go's specification, and a
  string is immutable.
- So on Go the closure stops at `Int`, `Bool`, `F64` and `bytestring`, and it passes through
  `Array(…)` and opaque host values: a slice or a pointer may alias.
- `go.string_of_bytes(v)` is therefore **the explicit copy**, as it should be: it ends the closure,
  so writing it before the next read is how a program keeps a factor.

## 4. The literature, organised by what it answers

**The construct is old and well understood. Only the ending event is specific to a host.**

- **Wadler, "Linear types can change the world!" (1990).** `let! (x) e` gives read-only access to a
  linear x inside e, on the condition that e's result contain no part of x. That condition is Theorem
  L's escape clause, stated for a lexical epoch. **Odersky, "Observers for linear types" (ESOP 1992)**
  makes the observer a type: a value that may be read and not kept.
- **Reynolds, "Syntactic control of interference" (POPL 1978); O'Hearn, Power, Takeyama and
  Tennent, "Syntactic control of interference revisited" (1999).** Passive uses (reads) may share;
  active uses (writes) exclude everything that shares. ADR 0052's R1–R4 are this discipline on
  buffers. A lent value is a passive alias of the receiver's state, and the next read is an active
  use of it.
- **Rust.** `BufRead::fill_buf(&mut self) -> io::Result<&[u8]>` is `ReadSlice`: a shared borrow
  whose lifetime is tied to the exclusive borrow of the reader. **Non-lexical lifetimes (RFC 2094,
  Rust 2018)** end a borrow at its last use, which is the epoch read off liveness instead of scope.
  The **lending iterator**, expressible since generic associated types (RFC 1598, stable in 1.65), is
  `Scanner.Bytes`: each item borrows the iterator until the next `next`.
- **Tofte and Talpin, "Region-based memory management" (1997).** A region per epoch, its lifetime
  inferred. That is more machinery than is needed here: an epoch ends at an event, not at a scope.
- **Osvald, Essertel, Wu, González Alayón and Rompf, "Gentrification gone too far? Affordable
  2nd-class values for fun and (co-)effect" (OOPSLA 2016).** A second-class value may be used and
  passed down, never stored or returned. That is the escape half, as a type discipline with no
  lifetimes at all.
- **Lucassen and Gifford, "Polymorphic effect systems" (POPL 1988); Talpin and Jouvelot, "The type
  and effect discipline" (1992).** Effects indexed by regions would let a call state which state it
  writes, which is E2 below.
- **Clarke, Potter and Noble, "Ownership types for flexible alias protection" (OOPSLA 1998).** The
  owner relation E1 needs: a Reader's buffer is owned by the Reader, and a Reader by whoever holds
  it uniquely.

## 5. Which writes end the epoch

| | the epoch ends at | sound because | needs | shape's ReadSlice form |
|---|---|---|---|---|
| **E0** | the next impure call, any call | a pure call writes nothing (Theorem L as stated) | nothing new about aliasing | **enough** |
| **E1** | the next call that may reach the receiver | only the Reader's methods write its buffer, which is private to it | the receiver **owned**: fresh, and passed to nothing that keeps it | enough |
| **E2** | the next call whose declared effect writes the receiver's region | an effect system | every impure declaration states its regions | enough |

**E0 refuses a program that writes a lent piece out**, as in `os.Stdout.Write(line)`: the write is
an impure call reading the view. Under E1 the write cannot reach the Reader, and the program is
accepted.

**E1's premise is false as stated.** "A `bufio.NewReader` result is fresh" is contradicted by the
source: `NewReaderSize` returns its argument when it is already a `*bufio.Reader` of the requested
size or more. Freshness holds when the argument is not a `*bufio.Reader`, as with `os.Stdin` or an
`*os.File`, so E1 would need a fact about the argument's dynamic type. A generator cannot justify
that claim (ADR 0023), though a hand declaration could state it with that premise. E1 also needs
the Reader to be a place in ADR 0052's sense: R0's freshness, and R1–R4 so that no second name
reaches it. That is the same machinery as a buffer, extended to an opaque host value.

**E2 asks every impure declaration for a region**: 158 of the 397 declarations in `targets/go` are
impure, and every region would be a claim about the host's heap that the manifest does not state.

E0 is what the program needs and what can be proved with nothing new. E1 is named, with its
trigger: **the first program that hands a lent piece to another host call**, such as a `cut` writing
the selected bytes of each record without copying them.

## 6. Where it is checked

**In the reducer, as a side condition on β.** effects.md §7c's rule is *"A term that reads a table
through a bound variable is not substituted into an impure body."* That covers `v[i]`. It does not
cover a host call reading v, such as `bytes.Count(v, sep)` or `string_of_bytes(v)`, which is
experiment B. The generalisation:

> **A term in which a lent binder occurs free is not substituted into an impure body.**

It is decidable at the β site, as §7c is: the body is in hand, and whether a binder is lent is known
from the declaration that bound it. That declaration is a host call whose continuation receives the
result. Like §7c, it costs at most a temporary bound before an impure call.

**On the IR, as an obligation.** Every analysis and representation choice is on the IR (ADR 0032).
The condition, for every lent value v:

> **(L0)** for every impure operation o other than v's definition, no member of A(v) is live on
> entry to o, and no member reaches a function's result.

Liveness is exact on structured SSA, and the x86 printer already computes it (spec/ir.md §9.6). A(v)
is a forward closure over π-parameters and over operations whose result type may share an extent
(§3). (L0) implies Theorem L's condition on every run, because the first impure event after e_v is an
execution of some impure o reachable from v's definition. It is conservative across paths, which is
what a static check is. A violation is a refusal that names the read, the call that ended the epoch,
and the copy to write: `go.string_of_bytes(v)` before that call, or the copying twin.

## 7. What the declaration says

A lent result is a **mode on the result**, not a set. Its set is `Array(0..255)`: the value's meaning
is unchanged, and ADR 0049's types are sets of values. ADR 0050 makes usage a mode on a binder
(`linear`, `inout`, `out`), and a result is a binder (ADR 0062). So:

```
fn ReadSlice(self ∈ Reader, delim ∈ 0..255) (Array(0..255) lent × result.result((), error)) {
    host expr "${self}.ReadSlice(uint8(${delim}))" import "bufio"
}
```

The host check reads `lent` as nothing: Go's manifest says `[]uint8`, and so do we. What it adds is
a claim about time, made by hand from the documentation, which ADR 0022 says a hand declaration is
for. The generator never writes `lent`, because the manifest does not say it (ADR 0023). A lent host
object (`DotReader`, `Decoder.Buffered`) is the same mode on an opaque result.

## 8. Decisions for hamza

1. **Is a lent result a mode on the result binder, `lent`, with its set unchanged?**
   *Recommended*: yes, as `linear` is a mode on a parameter. The alternative is a set of its own,
   which would make "the bytes of the next piece" two different types depending on how they are
   held.
2. **Which writes end the epoch?** *Recommended*: E0, every impure call. It is sound with nothing
   new, and enough for `shape` and for `Peek` followed by `Discard`. E1 waits for its trigger, the
   first program that hands a lent piece to another host call; it needs the receiver as an ADR 0052
   place, and a premise about its argument because `NewReaderSize` may return it.
3. **What happens on a violation?** *Recommended*: refuse, naming the read, the ending call and the
   copy to write. Never insert the copy: it is an allocation the source does not show.
4. **Where is it enforced?** *Recommended*: the reducer's side condition (§6, §7c generalized) and
   (L0) on the IR. Neither alone is enough: the reducer alone does not see escape through a loop, and
   the IR alone sees experiment B only after β has already made it A.
5. **Program first: does `shape` move to `ReadSlice`?** *Recommended*: yes. `shape` becomes the
   lent form, with `bytes.Count` (a new `targets/go/bytes.oro`, one declaration, pure, its parameter
   read and not kept). It is measured against the hand-written `ReadSlice` program for parity.
   `ReadString` stays declared, the copying form, and the refusal tests carry experiments A and B
   as programs that must be refused, each checked for its reason.
6. **Do the other lent names follow?** *Recommended*: `Scanner.Bytes`, `ReadLine` and `Peek` declared
   `lent` when a program calls them, not before. A lent host object, and `Writer.AvailableBuffer`, a
   buffer lent for **writing** (aspect 1 lent, coverage cause J's neighbour), are recorded and wait
   for a program.

## 9. What this does not decide

- Concurrency. Theorem L's premise, that the host runs only inside a call, is the first thing a
  goroutine would break.
- A lent value from the language itself. A view of a live buffer, `b[lo..hi]` without a copy, has
  the same algebra: its epoch ends at the buffer's next write, which ADR 0052's places already order.
  No program asks for it.
- A restriction former as a view. `array { t[k] : k ∈ lo..hi-1 }` over an immutable t could be held
  as `t[lo:hi]` with no copy, since sharing an immutable table is invisible. That is a
  representation choice for the IR, and no program asks for it either.
