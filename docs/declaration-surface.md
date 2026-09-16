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
  - it separates packages from types by capitalisation, a host convention a future host may lack
    (§1.6 removes this cost);
  - a rename across the generator, both surveys' output, the pinned reports, the tests and every
    `(use …)`;
  - *(an earlier version listed "the target loader reads one directory level, so it needs a
    recursive walk". That is wrong: a target file declares its module path inside the file,
    `(module go/encoding-hex …)`, and the file name is not read. A library's `(use …)` is resolved
    as a file path with `filepath.FromSlash` (`cmd/build/main.go:298`), which already descends
    directories.)*
  - nesting suggests containment and there is none: `go/unicode` does not contain
    `go/unicode/utf8` (true of Go too).

**C. Give the second meaning its own operation**, e.g. `go/unicode/utf8:File` for a type's methods.
- Buys: unambiguous on every host without appeal to case, and the package path is the host's.
- Costs: a second separator in the reader; the qualifier separator is already `.`, so a name would
  read `go/os:File.Read`, three separators in one identifier.

### 1.5 Leaning

B, because the alias cost is paid in every program and the ambiguity cost is paid by a host we do
not have. C is the answer if that host arrives. §1.6 says why C may never be needed.

### 1.6 Modules inside modules (hamza, 2026-09-14)

The question: if a module may contain modules, is the problem solved, as
`go/encoding-hex/InvalidByteError` seems to do already?

**What the code does today.** Modules do not nest. `(module PATH …)` may contain only `(prim …)`
(`emit/target.go:1488`). `go/encoding-hex/InvalidByteError` is a separate module whose *name* has
`go/encoding-hex` as a prefix. Nothing relates the two except that string prefix.

**Nesting adds less structure than it seems, because the tree is already there.** `Seg*` ordered by
prefix *is* a tree, the trie of all paths. So a set of module paths already forms a tree in which a
node may have both members and children. `go/encoding-hex` has prims and a child today. What nesting
adds is only this:
- **a way to write it** — `(module go/os … (module File …))`, where a child's path is its parent's
  path, then `/`, then the child's name. This is reader sugar, pure resolution, so ADR 0011 holds.
- **a rule saying what a child is.**

Only the second touches the ambiguity in §1.1. Nesting alone does not settle it: `go/unicode/utf8`
and `go/os/File` are still both "a child", and something must still say which child is a package and
which is a type's method set.

**What settles it is types living in modules (§4).** Once a module owns its types, the two meanings
separate by a checked fact instead of by letter case:
- a child module that has the same name as a type in its parent is that type's **companion**, the
  type's method set;
- any other child is a nested package.

For this to work, a type name and a child module name must not collide, and the loader can refuse a
collision. The JVM already forbids one: JLS §7.1 makes a package containing both a subpackage and a
top-level type of the same name a compile-time error. Go avoids it by convention: exported types are
capitalised and package paths are not. Win32 has no nesting.

So §1.3's appeal to letter case becomes a load-time check. Option C's second separator is needed
only by a host that breaks the rule on purpose, and such a host would be refused with a message
rather than misread.

**Is another container needed? No.** Option B with types in modules and the companion rule keeps one
container, the module, with three kinds of member: operations, types and child modules. OCaml has
exactly these three separate namespaces in a structure.

OCaml's other convention puts the type *inside* its own module, as `File.t`. It would also work, and
it would remove the companion rule. But it spells `os.File.t` where the host says `os.File`, which is
the cost this section set out to remove.

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

## 4. Where a type lives

### 4.1 What it is

A module in a target is meant to be a **signature**, and a many-sorted signature is a pair
`Σ = (S, Ω)`: the **sorts** and the **operation symbols** typed over them (Goguen and Burstall's
institutions; OBJ's modules, which declare `sort` beside `op`). Today a target module holds only
`Ω`. Its sorts come from one pool shared by the whole target, `tg.Types`, a flat map from name to
spelling (`emit/target.go:125`), glued across files. `go/encoding-hex` is therefore not a signature
on its own: `InvalidByteError`, which is its own sort, is declared outside it.

In ML's terms, a host type is an **abstract type component** of a structure: `type t` in a signature
whose representation is hidden, here because the host holds it (MacQueen 1984; manifest and abstract
types in Leroy 1994 and Harper and Lillibridge 1994). An opaque host token is exactly that.

### 4.2 What the flat pool costs today, with instances

- **The qualifier is mangled into the name by hand.** `io-Reader` and `hex-InvalidByteError` are
  `io.Reader` and `hex.InvalidByteError` with the `.` turned into `-` because the pool has no
  structure. The generator does the same mechanically (`gauntlet/stdlib/survey.go:1745`, `spell(qual(…))`).
- **The mangling uses the base name, not the path, so distinct host types can collide.**
  `qual`'s own comment says so (`survey.go:1058`: *"two packages with the same base name and the
  same type name still collide"*). **Measured against Go's API manifest: 3 of 1,270 exported type
  names collide.** `template.Template` (`html/` and `text/template`) and `scanner.Scanner`
  (`go/` and `text/scanner`) are distinct structs that the pool makes one. `template.FuncMap` is
  one type, because `html/template` declares `type FuncMap = template.FuncMap`, so the pool is right
  about it by accident. Go refuses a file that confuses the first two, so no answer is silently
  wrong. It is still a false fact in our checker, the kind the coercion work was careful never to
  hold. [theories.md §2.2](theories.md) reads the three as two abstract types and one alias.
- **A sort is re-declared by whoever needs it.** `targets/go/encoding-hex.oro` declares
  `io-Reader`, `io-Writer` and `io-WriteCloser`, which belong to `io`. Glue accepts the repeat
  because the spelling matches. Nothing records that hex *depends* on io, and nothing stops a third
  file from declaring `io-Writer` with a different meaning in a layer that overrides.

### 4.3 The options

**A. Keep the flat pool (today).**
- Buys: no resolution of type names at all. A type in a signature is a string compared by equality.
- Costs: §4.2, all three.

**B. Types are members of modules.** `(module go/io (type Writer "io.Writer") …)`, named
`go/io.Writer` everywhere, the way a prim is.
- Buys:
  - no hand prefixes;
  - the base-name collision disappears, because the name carries the whole path;
  - a type has one owner, so another module referring to it states a dependency rather than making
    a second declaration, and a declaration of a type in a module one does not own can be refused;
  - it is what makes §1.6's companion rule possible.
- Costs:
  - **type names enter name resolution.** A prim in `go/encoding-hex` taking an `io.Writer` must
    reach `go/io`, and the target format has no `use`. So either signatures in target files write
    the full path, `(w go/io.Writer)`, or target modules gain `(use …)`;
  - **program signatures that name a host type resolve through aliases**, as values do. The reader
    qualifies value names today and not type names;
  - a rename across the generator, the pinned reports and every hand file.

### 4.4 Where an `implements` edge lives

Once types have owners, an edge has a place to belong. For edges within one module the answer is
obvious: `io.WriteCloser ≤ io.Writer` belongs in `go/io`. For an edge between modules there is
Haskell's **orphan instance** problem to consider, and it does not arise here.

An orphan instance is dangerous in Haskell because an instance is *evidence*, a dictionary. Two
orphans for one class and type are two different dictionaries, and coherence breaks. Our edge
carries no evidence: `⟦coerce⟧ = id` (target-files.md §2a), and the relation is a set, so declaring
an edge twice is `R ∪ R = R`. An edge declared anywhere cannot be incoherent. Its placement is about
**ownership and checking**, not meaning.

The natural owner is the **subject's** module:
- Go declares a type's methods in the type's package, so `*os.File ≤ io.Reader` belongs to `go/os`.
- The JVM writes `implements` on the class.

### 4.5 Leaning

B, together with §1's B and §1.6's companion rule. The three belong together: a module with its
sorts, its operations and its child modules is one container that answers all three questions. The
decision that remains is the syntax for referring to another module's type inside a target file:
the full path, or a `use`.

---

## 5. What is left for the decision

- **§1** is a rename with a known list of places; B, with the companion rule of §1.6 once §4 is B.
- **§2** is either reader sugar in the target format (A) or a name-resolution rule keyed on purity (B).
- **§3** needs no change to take C; an alias (B) is a reader desugaring; D is a type-system decision.
- **§4** changes what a type name *is*, a string or a resolved path, and so is the one question
  here that is not purely spelling. It touches the reader, the checker's type comparison, the target
  format and the generator.

§1, §2 and §3 are spelling. §4 is not, and it decides the shape of every hand file still to be
written, so it is the one to settle before the next package.

> **Carried forward 2026-09-15.** All four questions are answered in the specifications that followed
> the research in [theories.md](theories.md) and [theories-b-or-c.md](theories-b-or-c.md):
> - §1: host paths, nesting and companions — [spec/theories.md](spec/theories.md) §3;
> - §2: `const` as the conditional cell — spec/theories.md §8.3;
> - §3: manifest types with host-qualified names, `(type rune int32)` in module `go` —
>   spec/theories.md §1–§2;
> - §4: types resolved like terms, owned by modules — spec/theories.md §3.4.
>
> Specified, and being built in the order theories-b-or-c.md §9 fixes. **§4 is built, 2026-09-16**
> ([ownedtypes-2026-09-16](../gauntlet/results/ownedtypes-2026-09-16.md)): a type declared inside
> `(module PATH …)` is named `PATH.NAME`, `targets/go/io.oro` owns `io`'s three sorts, and the
> hand-declaration checker compares a declaration to the host by the SPELLING it realizes rather than
> by the key that names it — so hand and generated files may name one host type differently and still
> be checked against each other. The generator still writes types at target level: naming them by
> module needs a base-name-to-path index the api manifest does not carry, which is the half that
> makes the two `Template` types distinct.
