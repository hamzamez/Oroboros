# 0025 — A module path is the host's path, and the file is the path

Date: 2026-09-20
Status: Accepted

## Context

A module path is a word in the free monoid `Seg*`, with `/` for its operation
([modules.md §3](../spec/modules.md)). A host package path is a word in the same kind of monoid:
`encoding/hex` on Go, `java.util` on the JVM, `fs/promises` on Node.

The generator flattened the second into one segment of the first — `encoding/hex` became the single
segment `encoding-hex`, so `go/encoding-hex` — and the two packages declared by hand kept that name
because the checker compares module by module. [declaration-surface.md §1](../declaration-surface.md)
(2026-09-14) asked why, laid out three options and **leaned B, the host's own path**, but it was
research with no decision and nothing was built.

Two things have happened since. The prerequisite §1.6 named is built — **types are owned by their
module, and companions exist** ([ownedtypes-2026-09-16](../../gauntlet/results/ownedtypes-2026-09-16.md),
[companions-2026-09-16](../../gauntlet/results/companions-2026-09-16.md)) — so the ambiguity that
flattening was avoiding can be settled by a checked fact instead of by letter case. And hamza asked
whether the rename had been done. It had not.

## Decision

**A host module's path is the host's own path, prefixed by the host.** `go/encoding/hex`,
`go/unicode/utf8`, `java/java/util/regex`, `js/fs/promises`. The flattening is removed from all three
generators and from the two hand-written declarations.

**A type's companion is a child of its package**, as it already was: `go/encoding/hex/InvalidByteError`,
`java/java/util/Random`. A child module whose name matches a type in its parent is that type's method
set; any other child is a nested package.

**The file is the path.** A generated or hand-written declaration of `encoding/hex` lives at
`encoding/hex.oro` under the target directory, and `loadTargetDir` walks instead of reading one
level.

## Why not

**Keep the flat segment.** It buys one thing: `/` has a single meaning after the host prefix without
appealing to any host convention. It costs the host's own name in every program — every `(use …)`
carried an `as` to get `hex` back from `encoding-hex` — and it is not even injective. `flat` maps
`Seg*` onto one segment by joining with `-`, and Go import paths may contain `-`, so a package
`a/b` and a package `a-b` collide. The collision is not hypothetical for third-party paths; it is
merely absent from the standard library.

The structural argument is the one that settles it. `ι(p) = host ++ p` is a **monoid homomorphism
and injective**, and `lastSegment ∘ ι = lastSegment`, so the default alias of an import is the
host's own last segment — `hex`, `utf8`, `regex` — with no `as` at all. Flattening broke that
equation, and every `as` in the corpus was the repair.

**Option C, a second separator** (`go/encoding/hex:InvalidByteError`). Unambiguous without appeal to
any host convention, but it adds a third separator to a name that already has `/` and `.`, and the
companion rule now gives the same answer with a load-time check. Kept in reserve for a host that
puts a type and a subpackage under one name on purpose; such a host is refused with a message rather
than misread.

**Keep flat FILES with nested module paths.** Sound — the loader never reads a file's name, and a
target file declares its own module path inside — but it leaves two spellings of one tree: a trie of
module paths, and a directory of flattened names beside it. Since `Seg*` ordered by prefix *is* the
directory tree, letting the file system be that tree costs one `filepath.Walk` and buys a reader the
ability to find `go/encoding/hex` at `targets/go/encoding/hex.oro`. `loadProvides` already walked the
library layer, so this also makes the two halves of target loading agree.

**Rename the acceptance PROGRAMS too** (`acceptance/encoding-hex.oro`). No: a program is not a
module. It has no path in `Seg*` — it is a file, named by whoever wrote it — so a hyphen there is not
a second spelling of anything. The rule is *the file is the path* **where a path exists**.

## Consequences

**Easy.** A program says `(use go/encoding/hex)` and calls `hex.Encode`; the name it writes is the
name the host's own documentation uses. A generated target tree is a picture of the host's package
tree. A future nested package costs nothing.

**Hard.** The JVM's prefix doubles: `java.util` is `java/java/util`, because the first segment is the
*host* and the second is the package root. It reads oddly and it is honest — `javax.swing` is
`java/javax/swing`, and a Kotlin package would be `java/kotlin/…`.

**Committed to.** The companion rule now carries the weight that letter case carried before: a child
module named like a type in its parent is that type's method set. If a host ever breaks that, option
C is the answer and this ADR is the one it supersedes.
