# How a declaration reads: module paths, constants, and names for ranges

Research, **no decision**. 2026-09-14, on hamza's three questions about how a hand-written host
declaration looks (after [handdecl-2026-09-14](../gauntlet/results/handdecl-2026-09-14.md)):

1. why `go/unicode-utf8` and not `go/unicode/utf8`;
2. why a constant is a primitive of no arguments, `(prim MaxRune (none) (int 1114111 1114111) …)`,
   and whether it wants sugar;
3. whether a parameter should say `(r (int -2147483648 2147483647))` or `(r rune)`.

Nothing here is built and nothing is changed. Each section says what the thing **is**, what the
code does today and why, and what each alternative buys and costs.

---

## 1. A module path

### 1.1 What it is

A module path is a word in the free monoid over segments, `Seg*`, and `/` is its operation. Two
different things are currently written with that one operation:

- **package nesting** on the host — `unicode/utf8`, `encoding/hex`, `net/http`;
- **a type's method set inside a package** — the survey puts `(*os.File).Read` in the module
  `go/os/File` (`gauntlet/stdlib/survey.go:1784`), and the JVM survey does the same with
  `java/<pkg>/<Class>`.

A single operation carrying two meanings is ambiguous unless something else separates them. The
generator separated them by flattening the first: package `unicode/utf8` becomes the one segment
`unicode-utf8`, so every `/` after `go/` means "a type in this package". The hand-written file kept
the generator's name because the checker compares module by module
(`TestHandDeclarationsAgreeWithTheHost`).

### 1.2 The alias falls out of the path

The default alias of an import is its last segment (`core/read.go:982`, `lastSegment`). So the
path decides what a program calls the module:

| path | default alias | the host's own name |
|---|---|---|
| `go/unicode-utf8` | `unicode-utf8` | `utf8` |
| `go/unicode/utf8` | `utf8` | `utf8` |

This is why `unicode-utf8.oro` has to write `(use go/unicode-utf8 as u)`.

### 1.3 Is the ambiguity real?

The two meanings can be told apart by a fact about the host rather than by syntax:

- **Go:** an exported type begins with an upper-case letter, and a package path segment is
  conventionally lower-case. `go/unicode/utf8` (a package) and `go/os/File` (a type) cannot
  collide.
- **The JVM:** classes are capitalised and packages are not, by a convention strong enough that
  the JDK's own module system relies on it.
- **Win32** has no nesting; its modules are headers (`win/fileapi`).

So on the hosts we have, the case of the first letter is an injective tag on the two meanings. It
is a **host convention**, not a property of our module system.

### 1.4 The options

**A. Keep the flat segment (today).**
- Buys: `/` has one meaning after the host prefix, on every host, with no appeal to case.
- Costs: the name is not the host's, and every program carries an `as` to get it back.

**B. Use the host's path, `go/unicode/utf8`.**
- Buys: the module is named the way the host names it; the default alias is right; the file maps
  onto a directory the way the import path does.
- Costs:
  - it separates packages from types by capitalisation, a host convention a future host may lack;
  - the target loader reads one directory level (`emit/target.go:1130`, `os.ReadDir`), so
    `targets/go/unicode/utf8.oro` needs a recursive walk;
  - a rename across the generator, both surveys' output, the pinned reports, the tests and every
    `(use …)`;
  - nesting suggests containment and there is none: `go/unicode` does not contain
    `go/unicode/utf8` (true of Go too).

**C. Give the second meaning its own operation**, e.g. `go/unicode/utf8:File` for a type's methods.
- Buys: unambiguous on every host without appeal to case, and the package path is the host's.
- Costs: a second separator in the reader; the qualifier separator is already `.`, so a name would
  read `go/os:File.Read`, three separators in one identifier.

### 1.5 Leaning

B, because the alias cost is paid in every program and the ambiguity cost is paid by a host we do
not have. C is the answer if that host arrives.

---

## 2. A constant

### 2.1 What it is

In a cartesian category a constant of type `A` is a **global element**, a morphism `1 → A`.
Declaring one as a primitive of no arguments says exactly that, which is why nothing new was needed:
`(prim MaxRune (none) (int 1114111 1114111) …)` is a morphism from the unit, and `(u.MaxRune)`
applies it to the only element of `1`.

What makes it a constant rather than a computation is **purity**. With effects, a nullary operation
is a Kleisli arrow `1 → T A`, and two applications are two effects: `GetLastError()` and
`os.Getpid()` take no arguments and are not constants. By ADR 0010 purity is the one declared bit
that separates the two.

The exact range `(int v v)` is then the whole value, since `γ(int v v) = {v}`. The declaration states
the value once as a type and again inside the template's host name.

### 2.2 Why it is written this way

`prim` is the only declaration a target has, and a primitive is applied. The survey added constants
as primitives of no arguments (gostd-utf8-2026-09-13, 509 of 2,989) so that no new form was needed.

### 2.3 The options

**A. Sugar in the target file**: `(const MaxRune 1114111 "utf8.MaxRune")`, expanding to the pure,
nullary prim with the exact range.
- Buys: the value is written once; `pure`, `(none)` and the range cannot be written wrong because
  they are derived; nothing downstream changes, the same shape as `let` and `seq`; the value is a
  claim the host can check (`const _ = utf8.MaxRune - 1114111` fails to compile when it is wrong).
- Costs: a second declaration form in the format a third party writes; and it does not cover the
  50 constants whose value differs by platform, which have no single value to write.

**B. A pure nullary is a NAME at the use site**: `u.MaxRune` without parentheses.
- Buys: this is the honest reading — a global element of a pure theory is a value, not a call.
- Costs: the rule must be drawn by purity, since an impure nullary must stay an application. So
  whether a program writes parentheses depends on a bit it cannot see where it writes them. This is
  a change to name resolution, not to the target format.

**C. Fold the constant to a literal** in the emitted code.
- Buys: nothing for the analysis, which already has the exact range.
- Costs: the host's own name disappears from the emitted program, and platform-dependent constants
  cannot be folded at all.

### 2.4 Leaning

A: small, and it removes two ways of being wrong. B is a real language question whose cost is the
invisible purity dependency. C is not worth it.

---

## 3. A name for a range

### 3.1 What it is

A range `(int lo hi)` denotes the set `[lo, hi] ∩ ℤ`. The type language is a lattice of such subsets
under inclusion, and representation is a monotone map out of it (ADR 0003, unbounded-rung.md §1).
A name for a range is one of two different things:

- an **abbreviation** — a definition at the type level, `rune ≝ (int -2³¹ 2³¹−1)`, unfolded before
  anything reads it. It is δ, one level up, and it adds no types: `rune` and the range are **equal**;
- a **new type** — a fresh generator `rune` with an isomorphism to the range. It adds a type:
  `rune` and the range are **isomorphic and not equal**, and crossing between them is a conversion.

### 3.2 The fact that decides much of it: `rune` names two different sets

Go's `rune` is a representation, `int32`, the set `[−2³¹, 2³¹−1]`. The **meaning** of a rune is a
Unicode scalar value, the set `[0, 0x10FFFF] \ [0xD800, 0xDFFF]`. These are different sets, and
the second is not an interval, so no range can denote it. `utf8.RuneLen` accepts the first and
answers −1 outside the second.

So an abbreviation `rune` would do one of two wrong things:
- mean `int32`, and read like a meaning while stating a storage width — the confusion ADR 0003 was
  written to prevent;
- mean the scalar values, and be wrong for `RuneLen`'s parameter, and not be an interval anyway.

### 3.3 The options

**A. Ranges only (today).**
- Buys: every signature states exactly its set; the interval and refinement layers read it directly;
  no name to resolve and no scope to own it; the hand-declaration checker compares canonical
  spellings; diagnostics print ranges, so the declaration and the error say the same thing.
- Costs: long literals that are easy to mistype, repeated in every signature, and the intent — "this
  is a Go `int32`" — is not written anywhere.

**B. Abbreviations**, e.g. `(alias int32 (int -2147483648 2147483647))`, unfolded in the reader the
way a range already desugars into its `where`.
- Buys: short; one definition, fixed in one place; nothing after the reader learns aliases exist.
- Costs: the endpoints, which are the useful part, are hidden at the use site; diagnostics print the
  unfolded range, so a user meets two spellings of one set; and the name needs an owner — a bare
  `rune` invites §3.2's confusion, while a host-qualified `go.int32` is honest and names storage.

**C. Spell the endpoints by their reason.** An endpoint is already a compile-time expression
(unbounded-rung.md: literals, unary `-`, `+ - *`, `pow`).
- Today, with no change: `(int (- 0 (pow 2 31)) (- (pow 2 31) 1))` — both ends visible, and why
  they are what they are.
- With one change, a constant's name as an endpoint: `(int 0 MaxRune)`.
- Buys: nothing is hidden, and the arithmetic is checkable at a glance.
- Costs: still longer than a name; a name as an endpoint needs its value when the reader runs, and a
  host constant's value is known only from its declaration.

**D. New types (nominal).**
- Buys: a count cannot be passed where a character is wanted.
- Costs: conversions everywhere a value crosses; it pulls toward the subtyping type-algebra.md
  refuses; it needs the strongest argument of the four and no program has made it.

### 3.4 Leaning

Keep the range as the meaning, and make it readable with C, which works today. If names come, they
should be **abbreviations, never new types**, and **host-qualified** (`go.int32`), so a name only
ever abbreviates a storage fact and cannot pose as a meaning.

---

## 4. What is left for the decision

- **§1** is a rename with a known list of places; B or C.
- **§2** is either reader sugar in the target format (A) or a name-resolution rule keyed on purity (B).
- **§3** needs no change to take C; an alias (B) is a reader desugaring; D is a type-system decision.

None blocks the next package, and the next package can be written in whichever surface is chosen,
since all three questions are spelling rather than meaning.
