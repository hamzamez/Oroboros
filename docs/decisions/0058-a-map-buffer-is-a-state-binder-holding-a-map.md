# 0058 — A map buffer is a state binder holding a map, written in program order

Date: 2026-10-12
Status: Accepted (hamza, 2026-10-12, on surface-design.md §4.7, item 4: "Agree. a map buffer is state
binder holding a map"). **Extends [ADR 0052](0052-a-buffer-is-a-state-binder-holding-a-table.md)** from
tables to maps. Not built: it is built with the surface, behind the byte-identical gate.

## Context

ADR 0052 made a buffer a state binder whose value is a table, written in program order, its
linearity following from R0–R4. A map buffer has the same discipline (maps.md §3.3: "the discipline is
about aliasing, and aliasing does not care what the index set is"): `build-map` scopes it, `insert`
consumes it and returns it, and the scope's value freezes it. Today it is threaded by hand:

```lisp
(build-map m 8
  (loop ((m m) (i 0))
    (>= i n)  m
    else      (again (insert m i (* i 10)) (+ i 1))))
```

The corpus has 4 map buffers and 4 stores (`gauntlet/surface/data.go`).

## Decision

**A map buffer is a state binder whose value is a map.** `m := map(cap)` introduces it (`var m :=
map(cap)` in J; in a `LET` group in D), `m[k] := v` stores (today's `insert`), `m[k]` reads and gives an
`option`, and the scope's value freezes it. R0–R4 hold as for a table:
- **R0**: its value is fresh, an empty map of a declared capacity or a call that borrows it and hands it
  back;
- **R1**: it is named only by its binder;
- **R2**: a call borrows it for its duration, and no call is handed it twice;
- **R3**: no closure over it survives its scope;
- **R4**: its scope's value freezes it.

The state-passing translation turns each store into today's threaded `insert`, so the residual, and
emission, should not change.

## Why not

- **Keep map buffers threaded while tables are in program order**: two disciplines for one property,
  the one maps.md §3.3 says does not depend on the index set.
- **A map buffer as a definitional binder with a mode**: rejected for tables in ADR 0052, for the same
  reasons.

## Consequences

- The capacity stays declared (maps.md §6): an empty map buffer is written with it, `map(8)`.
- `insert` leaves the surface, as `set` did; it stays in Λ⁺ as D's image.
- maps.md §3.3's threaded example becomes `m[i] := i * 10 ; AGAIN(i + 1)`, as surface-design §4.5's E4
  writes it.
