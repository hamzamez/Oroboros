# 0136 — A struct's field is a value, and a pointer in it is the layout's, written as C writes it

Date: 2026-11-19
Status: Accepted (hamza, 2026-11-19: "let's try to derive it from our memory model, see where it
lands"; the derivation decides it, CLAUDE.md's rule). Settles TinyRetroPad's four remaining struct
walls (assessment-2026-11-18 §4), extends [ADR 0128](0128-a-record-is-built-whole-and-a-host-struct-is-its-layout.md)'s
layout and [ADR 0131](0131-an-out-parameter-is-a-result.md)'s C notation to it. **Not built**; built
with TinyRetroPad's modules (plan item 1).

## Context

Four struct shapes were open: a buffer and its length in a struct (OPENFILENAMEA's `lpstrFile`,
`nMaxFile`), a nested record by pointer (CHOOSEFONTW's `lpLogFont`), an inline array (PAINTSTRUCT's
`BYTE rgbReserved[32]`), and an optional C string (DOCINFOA's `lpszOutput`, NULL or a string).

## The derivation, from the memory model

- **A record is a value, and a place is not** (ADR 0049, 0084, 0135): a set ranges over values, and a
  cell or a buffer is a place. So no record field holds a linear buffer. A **table**, though, is a
  value, and a record may have a table field.
- **A host writes a struct through a reference to a fresh copy of its encoding** (ADR 0048, 0130): an
  `_Inout_` struct is a cell, `o ∈ T inout`, its whole encoding copied out and read back, every leaf
  and every region its pointers name included.
- **A pointer is representation** (ADR 0130's test): the program observes the value it points to,
  never the address. So where C's struct holds a pointer, the record holds the value and the layout
  holds the pointer, written as C writes it (ADR 0131).
- **NULL is the absent value** (pointers-research §15: no null address; `Option`, with a niche).

## Decision

1. **A buffer in a struct is a table field.** OPENFILENAMEA's record has
   `lpstrFile ∈ Array(0..255)`, and its layout `nMaxFile = len(lpstrFile)`, Theorem F's decided
   determined length. The call passes the struct `inout`, and the host's writes into the copy's table
   come back with the struct. The question "how is a record field holding a linear buffer spelled"
   dissolves: it holds a table, a value, copied as the struct's encoding is.
2. **A nested record by pointer is the nested record's value**, its pointer the layout's:
   `repr CHOOSEFONTW c(…, &lpLogFont, …)`, `&` as ADR 0131 writes an address. The layout points the
   field at a fresh copy of the nested record's encoding, which comes back with the struct when it is
   `inout`.
3. **An inline array is a table field of its length**, the layout writing C's declarator:
   `repr PAINTSTRUCT c(…, rgbReserved[32])`, and the record `rgbReserved ∈ {t ∈ Array(0..255) : len(t) = 32}`.
   Its 32 bytes are the struct's own leaves, carried by `BeginPaint`'s result to `EndPaint`'s `&ps`.
4. **An optional C string is `Option(String)`**, `'none` the null pointer, a niche, and `'some(s)`
   s's bytes under `nulfree(s)` (ADR 0091): DOCINFOA's `lpszOutput ∈ Option(String)`.

## Why not

- **A linear buffer as a record field** (`lpstrFile ∈ Array(0..255) linear` in a record): a place
  inside a value, which the memory model has no meaning for; ADR 0135's reason one level down.
- **The buffer as a separate parameter, the field filled by the call**: possible, and it avoids the
  copy. But the copy is the struct's own semantics (ADR 0048), MAX_PATH bytes, and this keeps one
  declaration form for every struct field. Measured if a program pays for the copy.
- **An opaque field for PAINTSTRUCT's bytes**: a new determined kind, where a table field is already
  a value the layout can carry.
- **`= 0` for an optional string**: makes the declaration less general.

## Consequences

- Two layout notations from C, `&f` and `f[N]`, beside `= size`, `= 0` and `= len(m)`.
- FINDREPLACEA is not settled here: FindTextA keeps the struct past the call, which is P4's protocol
  of views (pointers-research §14.18), and waits on its own decision.
