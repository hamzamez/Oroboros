# Companions: an interface gets a method set, and the subtyping edge becomes a theorem

2026-09-16. [spec/theories.md §3.3, §6.1, §6.2](../../docs/spec/theories.md). Step 6 of
[theories-b-or-c.md §9](../../docs/theories-b-or-c.md)'s build order, and the wall
[hex-2026-09-14 §6](hex-2026-09-14.md) recorded: *"`Close` on an `io.WriteCloser` is unreachable. No
declaration, generated or by hand, gives an interface its methods."*

## 1. The three things, and the one law

**A companion is a child module sharing a type's name** (§3.3). The operations on `go/io.Writer` are
declared in module `go/io/Writer`. That is how a host type's methods are written — and the only way
an *interface* gets any, since nothing returns one that a method could be declared on.

**`include` is theory inclusion between companions** (§6.2). `io.WriteCloser` is *defined* as
`interface { Writer; Closer }` in io.go, so its companion includes theirs, and every declaration of
the included companion becomes one of the includer's with the receiver retyped. The includer's own
declaration wins, which is override rather than glue.

**The subtyping edge is then DERIVED, not declared.** `T ≤ I` iff `methods(I) ⊆ methods(T)` — the
powerset lattice under reverse inclusion, Cardelli's record subtyping with method sets
(interfaces.md §2) — and an inclusion *is* that containment. So `targets/go/io.oro`'s hand-written
`(implements go/io.WriteCloser go/io.Writer)` is deleted and is a theorem about the declarations
beside it.

**And an edge that is written is - **Code**: `emit/companion.go` is new at **151** lines, `emit/target.go` 2,360 → 2,398 (the field, the
  parse, the two calls), **+189**.
- **`go run ./cmd/check`, every step, passes**: emission 194 of 194 files byte-identical, no compiler
  note changed, proof counts identical (2,307 of 2,413 integer operations, 345 of 382 loops),
  differential on four targets, and tooling — every survey twice, the hand-declaration checker with its
  planted mistakes, and twelve acceptance programs including the closed dumper.ED** (§6.1): an `implements` is a view from the interface's
companion to the subject's, so for every term the interface declares, the subject's companion must
declare one at a type at least as specific. It was declared and believed.

Resolution runs once on the merged target, because an included companion may live in another file or
another layer — `addCore`'s reason (layers-2026-09-07) — and a cycle is refused, the well-founded
order of declarations being §1.3.

## 2. The payoff, measured

`gauntlet/stdlib/acceptance/encoding-hex.oro` wrote into `hex.Dumper` and could not close it. Its own
header said so: *"the dumper without Close is missing exactly what Close would write"*.

**What Close writes, measured** with the same 21 bytes the program uses, against the real package:

| | bytes | lines |
|---|---|---|
| never closed | 104 | 1 |
| closed | 147 | 2 |

The 43 bytes are the final partial line, `|t._..|`. The program now closes the dumper — `Close`
reaching an `io.WriteCloser` through the companion it includes — and prints it, and the oracle is
hand-written Go closing it too.

## 3. Witnesses

- `TestACompanionIncludesAnotherAndTheEdgeFollows`: the included declarations arrive with the receiver
  retyped and the template untouched; both edges are derived; and the control, that subsumption has a
  direction — a `Writer` is not a `WriteCloser`.
- `TestAFalseImplementsEdgeIsRefused`: an edge whose interface declares a method the subject does not
  is refused, naming both and the method. **Control**: declare the method and the same edge is accepted.
- `TestIncludeIsRefusedOffACompanion`: `include` in a module that names no type, an include of a
  companion that does not exist, and a cycle.
- The acceptance program, which is the mixed case end to end: generated `os` declarations, hand-written
  `hex` and `io`, and the host as the oracle.

## 4. Cost

CHECK

## 5. Not built

- **`io`'s Ω is two methods**, `Write` and `Close`. `Read` is a write-borrow into a buffer and wants
  the shape hex's `Decode` has; nothing has read the rest of the package off the host, and a
  declaration is a claim (ADR 0022).
- **A generated survey declares no interface methods**, so the view check is vacuous on its thousands
  of edges. Giving an interface its method set from the manifest is the generator's half.
- **The companion relation is checked only where `include` or a view asks it.** `go/os/File` and
  `go/encoding-hex/InvalidByteError` are companions by naming convention as before; §3.3's other rule —
  a child module sharing a name with a parent's TERM is an error — is not implemented.
