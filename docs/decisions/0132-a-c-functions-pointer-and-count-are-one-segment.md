# 0132 — A C function's pointer and count are one segment

Date: 2026-11-18
Status: Accepted (hamza, 2026-11-18: "go ahead with the segment for item 2", on the audit after
[ADR 0130](0130-const-t-star-is-the-calls-ref-x.md)). **Not built**; to be built with
[ADR 0131](0131-an-out-parameter-is-a-result.md), whose notation it uses and whose declarations it
shares. Amends [ADR 0118](0118-a-c-calls-arguments-are-a-linear-term-over-its-parameters.md)'s
linearity, and extends [ADR 0074](0074-a-segment-is-its-function-restricted.md)'s segment from
declared variants to derived ones for C functions.

## Context

kernel32's byte functions take a region as three parameters, its table, offset and count:

```
fn WriteFile(h ∈ winnt.HANDLE, b ∈ Array(0..255), lo ∈ 0..2^63-1, n ∈ minwindef.DWORD, …) where lo + n <= len(b) {
    host expr call "WriteFile"(h, elem(b, lo), n, put, null) …
```

`(&b[lo], n)` is C's representation of a segment, and the language has segments: `b[lo:hi]` is the
rule k ↦ b(lo + k) on [0, hi − lo), formed where 0 ≤ lo ≤ hi ≤ len b (ADR 0084). By ADR 0130's
test, the offset and the count are representation: a program means one region, and it observes the
bytes in it, not how C was told where they are.

ADR 0118 chose three parameters because "a segment is two parameters, b and lo, so it is not the
image of one parameter under one conversion". That conflates the segment with its representation:
a segment is one value, and its representation is a pair (address, length), the product's two
projections.

What it gains, against mirroring the API in the header:
- **The mirror stays where ADR 0131 put it**: `call WriteFile(h, s, len(s), &put, NULL)` is the
  prototype argument for argument, and the prototype check compares it.
- **One obligation rule instead of one per declaration.** Today each declaration states
  `where lo + n <= len(b)` by hand, and the loader checks only that a bounding conjunct exists. A
  segment's domain is the language's, decided where the program writes `b[lo:hi]`, so no
  declaration can state a bound against the wrong table.
- **One meaning across hosts.** Go's `Write(p []byte)` takes a segment, the JVM's
  `write(b, off, len)` and JavaScript's `(buffer, offset, length)` its parts. The program's value is
  the segment on every host, and each target's call states its own representation.

What it does **not** gain: precision. That a writer leaves the rest of its buffer as it was is the
host's documentation, and the language states no frame law (ADR 0074 item 5). This ADR keeps that.

## Decision

1. **A C function's (pointer, count) to a byte region is one parameter**, a table:
   `s ∈ Array(0..255)` for one the host reads, `s ∈ Array(0..255) linear` for one it writes.
2. **The call writes its two projections in C's notation (ADR 0131)**:
   - `s`, a byte table decayed to its first element's address, as C decays an array;
   - `len(s)`, its length as the C integer of its position, the one form beyond C's own syntax,
     since a C array argument carries no length.

   ```
   fn WriteFile(h ∈ winnt.HANDLE, s ∈ Array(0..255)) (put ∈ minwindef.DWORD, r ∈ minwindef.BOOL) where len(s) <= 2^32-1 {
       host expr call WriteFile(h, s, len(s), &put, NULL) stdcall lib "kernel32"
   }
   ```
3. **The length's set is the declaration's `where`**, `len(s) <= 2^32-1` for a DWORD, decided at
   each call as any `where` is (ADR 0028).
4. **A program passes a table or a segment**: `WriteFile(h, b)` or `WriteFile(h, b[lo:hi])`.
   - At a C call a segment is not formed as a table. Its projections are the restriction's:
     `s` is `&b[lo]`, ADR 0118's `elem(b, lo)`, and `len(s)` is `hi − lo`, with no copy.
   - The variant is derived for every C declaration with a table parameter, as ADR 0074's
     `segment.F` is declared by hand for Go: the parameters (b, lo, hi) in s's place, the domain
     0 ≤ lo ≤ hi ≤ len b, and the declaration's `where` with len(s) read as hi − lo.
   - For a writer the host writes into the segment and hands back b, its length kept, by ADR 0074
     item 3's cancellation, len b′ − lo = len b − lo ⟹ len b′ = len b.
5. **ADR 0118's linearity, amended**: a table parameter's address appears exactly once, and its
   length at most once, together its one occurrence, the product (address, length) taken apart.
   Every other parameter appears exactly once, as before.
6. **`&b[lo]` stays** for a C function that takes an offset into a table apart from its length.
   None does after this.

## Why not

- **Mirror the API in the header** (three parameters). The mirror is the call's argument list,
  which reads as the prototype under ADR 0131. In the header the offset and count are
  representation, and their bound is a hand-written `where` per declaration.
- **A segment variant declared by hand per C function**, as Go's `segment.Read`. A C call's terms
  make the variant mechanical, so a hand copy is a second statement that can disagree with the first.
- **Materialize the segment as a table at the call**: a copy of the bytes per call, where the host
  takes an address into the program's own buffer.
- **State the frame law now** (a writer leaves b outside [lo, hi) unchanged). It is the host's claim,
  and ADR 0074 item 5 leaves it to the host's documentation. It waits on a program whose refusal
  names it.

## Consequences

- `WriteFile` and `ReadFile` take one table each, and their callers (`lib/win/print.oro`,
  `lib/os/windows.oro`, `lib/io/windows.oro`) pass `b[lo:hi]` or `b` where they passed `b, lo, n`.
- The loader derives a C declaration's segment variant, and the reducer chooses it where a C call's
  table argument is a segment, before the segment is formed as a rule.
- ADR 0118's checks are re-read: elem's domain is now the segment's formation obligation, decided in
  the program. The per-declaration bound `lo + n <= len(b)` disappears.
- Go's `segment.Read` stays as declared: its template is Go text, the slice in the hole, ADR 0074's.
- **Spec first**, with ADR 0131's: local.md, llvm.md §6a's term table, j.md's host clause, tables.md
  on segments.
