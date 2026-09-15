# 0022 — Host declarations are written by hand; the generator is their checker

Date: 2026-09-15 (decided 2026-09-14)
Status: Accepted, and built for two packages — `unicode/utf8`
([handdecl-2026-09-14](../../gauntlet/results/handdecl-2026-09-14.md)) and `encoding/hex`
([hex-2026-09-14](../../gauntlet/results/hex-2026-09-14.md)).

Research: [host-buffers.md](../host-buffers.md). Rests on [0023](0023-a-generator-does-not-make-a-claim-it-cannot-justify.md).

## Context

The goal is to support the Go standard library package by package (gostd-utf8-2026-09-13). The first
package, `unicode/utf8`, was declared entirely by the generator and hit a wall that was not about types:

- **`EncodeRune` writes into a buffer and returns a count.** A linear buffer handed to a primitive was
  consumed, so a program got the count or the bytes, never both. That is the shape of
  `io.Reader.Read`, `hex.Encode` and `binary.PutUvarint`.
- **`AppendRune` twice on one immutable value aliased.** `x` changed when `y` was appended: a silent
  wrong answer, reachable through a generated declaration.

host-buffers.md then found what the wall was made of:

- **A soundness hole.** `CheckLinear` was seeded by *binders*, so a buffer that came from a primitive's
  *result* was never checked. Handed to a second call and then read, it printed `195`: the second write,
  through a dead name.
- **What a host call does to storage is not in its type.** Of the **166** Go standard-library slice
  parameters whose behaviour a body-reading classifier could determine, **81 (49%) are destructive**.
  **79.9%** could not be read at all.

A declaration that says only a host's types is therefore incomplete in exactly the ways that matter:
- whether an argument is consumed, borrowed or shared with the result;
- what the call requires (`2·len src ≤ len dst` for `hex.Encode`);
- what range its result lies in.

Under [0023](0023-a-generator-does-not-make-a-claim-it-cannot-justify.md) a generator may not claim any
of those, because no host artifact states them. hamza, after host-buffers.md: *"shouldn't we do it by
hand, all of it?"*

## Decision

**The declarations for a supported host package are written by hand, by someone who read the host's
source. The generator is reduced to checking them and suggesting where to start.**

1. **A hand file declares the whole package** — every exported function, method, type and constant —
   and states, beyond the host's types, everything the host's source justifies:
   - **buffer usage**: a write-borrow `B ⊸ B ⊗ R` (the template hands the buffer back), or
     consume-and-return `B ⊸ B`;
   - **preconditions**, exactly where they are linear (`hex.Decode`'s `len src ≤ 2·len dst + 1`);
   - **result ranges** (`utf8.RuneLen` is `(int -1 4)`);
   - **purity**, where the source shows no effect;
   - **the algebra first**, per CLAUDE.md's working convention: the file opens with the laws the package
     obeys, and the declarations are written to say as much of them as the language can check.
2. **`TestHandDeclarationsAgreeWithTheHost` checks every hand file against the generator's output for
   the same package**, and requires:
   - the host's own types, with exactly **three justified refinements** allowed:
     1. a slice the host writes may be declared a buffer;
     2. a write-borrow's results may begin with the buffer it hands back;
     3. an integer result may be a narrower range;
   - **coverage in both directions**: no host name missing from the hand file, and no hand name the host
     lacks;
   - that **planted mistakes are caught**: six per package, each one of the kinds above, so the checker
     is shown able to fail.
3. **Linearity is seeded by type, not by binder.** Every term whose type is `(buffer V)` is linear,
   including a primitive's result. An immutable array may not reach a parameter declared a buffer.
4. **Each hand-declared package has an acceptance program** whose output equals hand-written Go calling
   the real package: 63 lines for `unicode/utf8`, 31 for `encoding/hex`.

## Why not

**The generator declares buffer usage, with "unknown ⇒ buffer" as the default.** Sound, by ADR 0010's
reasoning, and useless: 79.9% of slice parameters are unknown, so nearly every call would consume its
argument and no program could read a buffer after using it. It also says nothing about preconditions or
result ranges, which the two built packages needed as much as buffer usage.

**Clip a slice's capacity at every boundary** (`a[:len:len]`, the host's own revocation). This closes
aliasing through `append` at the cost of an allocation on every call that grows, and it closes nothing
else: no precondition, no range, no count-and-bytes result.

**Generate, then let a person override individual names** through a project layer (target-system.md
§7.2). Layers support it mechanically, but a package half-generated and half-read mixes justified and
unjustified claims under one module. Coverage in both directions is what makes the checker complete, and
a mixed file has nothing to check the generated half against.

**Keep generated declarations and add claims only when a program fails.** That is how the soundness hole
was found, which is the argument against it: the failure was a wrong answer at run time, not a refusal.

## Consequences

**Supporting a package is human work, and that is the stated cost.** It has not been measured per
package. Both packages so far also uncovered compiler work: `encoding/hex` found that the refinement layer had never
looked inside a host call with several results, which meant every file-reading program's body had never
been checked.

**Every claim beyond types has a reader and a check.** A wrong type fails the checker; a missing name
fails coverage; a wrong claim about behaviour fails the acceptance program against the host.

**What the hand declarations buy is measured.** `unicode/utf8`'s declared result ranges took the
acceptance program from **3 of 9** integer operations proven to **8 of 9**. `hex.Encode` with a buffer one
byte short **no longer builds**, where before it panicked inside the host. Both changes were
**178 of 178** emitted files byte-identical, or explained file by file where not.

**The surveys keep their job.** They measure what fraction of each host can be declared, generate the
declarations the checker compares against, and produce every acceptance program's declarations for
packages not yet written by hand.

**It commits the checker to tracking the host version.** A hand file is pinned to the Go toolchain it
was read against, and the tooling suite fails rather than skips on a new version (tooling-2026-09-11).

**Trigger to revisit:**
- the per-package cost measured as what blocks the standard-library goal;
- a host that publishes usage, preconditions or ranges as artifacts, so a generator could justify them
  under 0023. Win32's SAL annotations are the nearest case, for buffers and nullability.
