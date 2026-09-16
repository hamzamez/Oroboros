# A type is a member of its module: sorts get an owner, and the base-name collision goes

2026-09-16. [spec/theories.md §3.2, §3.4](../../docs/spec/theories.md),
[target-files.md §2](../../docs/spec/target-files.md),
[declaration-surface.md §4](../../docs/declaration-surface.md). Step 3 of
[theories-b-or-c.md §9](../../docs/theories-b-or-c.md)'s build order, and the question that document
says decides the shape of every hand file still to be written.

## 1. What a module in a target is

A many-sorted signature is a pair `Σ = (S, Ω)`: the sorts, and the operation symbols typed over them
(Goguen and Burstall's institutions; OBJ declares `sort` beside `op`). A target's module held `Ω`
alone. Its sorts came from one flat pool per target, `tg.Types`, keyed by a name mangled by hand:
`io-Reader`, `hex-InvalidByteError`, `ptr-os-File`.

**So a module was not a signature**, and the pool's key was not injective. Three costs, all measured
or exhibited:

- **The qualifier was mangled into the name by hand**, `.` written as `-`.
- **The key is the BASE name, so distinct host types collide.** Measured against Go's api manifest:
  **3 of 1,270 exported type names**, of which `text/template.Template` and `html/template.Template`
  are distinct structs the pool made one. Go refuses a file that confuses them, so nothing was
  silently wrong — it was a false fact in our checker.
- **A sort was re-declared by whoever needed it.** `targets/go/encoding-hex.oro` declared `io.Reader`,
  `io.Writer` and `io.WriteCloser`, which belong to `io`. Nothing recorded that hex *depends* on io,
  and nothing stopped a third file from declaring `io.Writer` to mean something else.

**Now a type declared inside `(module PATH …)` is named `PATH.NAME`.** `go/io.Writer` is the name
everywhere, a signature in another module writes that path, and resolution on types is injective
because the key carries the whole path — the same law that made two modules' `result` two types
yesterday (qualvariant-2026-09-15).

## 2. What it needed

- **The loader**: a `type` form inside a module, qualified by the module's path. A **type constructor**
  is refused there: `(type (array A) …)` and `(type (map K V) …)` realize `lang`'s own constructors,
  once per target, outside any module (§5.6).
- **`targets/go/io.oro`**, a new file: `io`'s three sorts, with their owner, and the
  `WriteCloser ≤ Writer` edge that hex had been carrying. Its `Ω` is empty on purpose — nothing has
  read a method of `io` off the host, and a declaration is a claim (ADR 0022).
- **`encoding/hex`** keeps one sort, `InvalidByteError`, inside its own module, and names io's by path.
- **No resolution machinery.** A full path is a name, and the pool is keyed by it, so
  `(w go/io.Writer)` finds the declaration with nothing new in the reader. That is
  declaration-surface.md §4.3's option B, taking the first of its two spellings; target-level `use`
  can come when a file wants it.

## 3. A TYPE'S IDENTITY IS ITS REALIZATION — and that had never been asked

Splitting the naming broke two things, and both are one finding.

**The compiler.** The hex acceptance program hands a generated `*os.File` to the hand-declared
`hex.NewEncoder`, and it was refused: *"f is ptr-os-File, but go/io.Writer is required here"*. The
`implements` edge in the generated target names `io-Writer`; the hand file wants `go/io.Writer`. Two
keys, one host type.

**The rule.** The pool maps a name to the host's own spelling, and for an opaque host type that
spelling IS the type. So two names realizing `"io.Writer"` name one thing, however each is keyed.
`Target.SameHostType` says it, the checker asks it where `compatible` fails, and `Subsumes` reads an
edge through it on BOTH ends — the subject and the interface — so an edge declared with one key holds
when asked with the other.

**Nothing had to ask before.** One flat pool per target gave every host type exactly one name, so
name equality and type identity coincided by construction. They are different questions, and this is
the first build where the difference is observable.

The control keeps it from identifying everything: two different spellings are two types, and
subsumption still has a direction.

## 3b. The checker compares REALIZATIONS, not keys

`TestHandDeclarationsAgreeWithTheHost` compared a hand declaration's type names with the generated
survey's, string by string. The generator still writes `io-Writer` at target level, so a hand file
saying `go/io.Writer` would have failed against it — not because the claim differs but because the
key does.

**A declaration's claim is about the host's type**, and our name is the key that names it. So the
comparison resolves each side's names through its own target's `Types` and compares the host
spellings. The hand side is loaded as its whole target DIRECTORY rather than the one file, because a
declaration's meaning is relative to the target it lives in — hex names `go/io.Writer`, whose
declaration is in `io.oro`: `go/io.Writer` and `io-Writer` both realize `"io.Writer"`, and the check passes or fails on
what the host will see. The planted mistakes still fail.

## 4. Witnesses

- `TestATypeIsAMemberOfItsModule`: two modules declaring `Template` are two declarations with two
  owners; neither lands in the flat pool; a signature in a third module names another module's type by
  its path.
- `TestATypeConstructorIsNotAModuleMember`: `(type (array A) …)` inside a module is refused, naming the
  rule.
- `TestOneHostTypeMayHaveTwoKeys`: two keys realizing `io.Writer` are one type; an edge declared with
  one key holds when asked with the other; and the controls — two spellings are two types, and
  subsumption has a direction.
- The hex **acceptance program** builds and runs against generated `os` declarations and hand-written
  `hex` ones, which is the mixed case exhibited rather than argued.
- The hand-declaration checker still catches its six planted mistakes, now through realizations.

## 5. Cost

CHECK

## 6. Not built

- **The generator still writes types at target level.** Naming them by module needs a base-name → package-path
  index: the manifest qualifies a foreign type as `template.Template`, and which `template` that is
  cannot be read off the name. That is the half that makes the two `Template` types distinct in
  generated declarations, and it is the next thing here.
- `use` inside a target file, so a signature may write `Writer` rather than `go/io.Writer`.
- Companions (§3.3) and `include`, which is what gives an interface its method set — the wall
  `encoding/hex` recorded.
