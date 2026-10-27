# 0083 — A representation is an abstraction function, chosen by place

Date: 2026-10-27
Status: Accepted (hamza, 2026-10-27: "agree to all recommendations. Go ahead with the ADR and the
build"), on [widths-research.md](../widths-research.md) §8's decisions W1–W6. The second of the seven
walls of [llvmsurvey-2026-10-27](../../gauntlet/results/llvmsurvey-2026-10-27.md) §4. Amends
[ADR 0033](0033-an-integers-representation-is-chosen-on-the-ir.md),
[0034](0034-the-rung-above-the-word-is-a-least-fixed-point-on-the-ir.md),
[0035](0035-a-compiler-library-is-a-theory-checked-once.md) and
[0079](0079-a-declared-set-is-a-claim-and-the-facts-choose-the-representation.md); fulfils CLAUDE.md's
waiting item "a representation as an instance of a type's signature". **Built** with
[ADR 0082](0082-wrapping-is-the-residue-map-and-the-bits-are-the-integers.md), program first
([bits-2026-10-27](../../gauntlet/results/bits-2026-10-27.md)): the JVM's unsigned word and its byte
read unsigned, JavaScript's typed arrays and int32 shifts. Not built: JavaScript's split pair, a 32-bit
scalar representation, the old mechanisms re-declared as (H, d), and items 3, 4 and 6's LLVM part,
which come with `ir/llvm`.

## Context

An `Int` is an integer and a set is a claim (ADR 0026, 0079); ρ chooses how each value is held from its
proven fact. LLVM has a type per width, Win32 dictates widths at its boundary and in its structs, the JVM's
`byte` could hold 0..255 but does not (so `lib/os/java.oro` copies every file into a `short[]` and back),
JavaScript has typed arrays it does not use, and above the word LLVM has `i128` where the windows target
has a limb library. The language had built six mechanisms for holding an integer: narrow table elements
(elemwidth), the word, the unsigned word (ADR 0033), the JVM's `int` index (indextype), limbs (ADR 0035)
and the host bignum (ADR 0034). Each new width would have been a seventh.

widths-research.md §2 showed they are one thing. A representation is r = (H, d): a host type H and an
injective abstraction function d : H → ℤ (Hoare, "Proof of correctness of data representations", 1972),
holding the image I(r) = d(H). A host operation computes ℤ's operation on r when it satisfies the
commuting square, d(f_H(x, y)) = f(d x, d y) wherever the result lies in I(r). A move between two
representations is d_{r′}⁻¹ ∘ d_r, the identity on ℤ. So:

> **Representation independence.** If every value's fact lies in its representation's image, every
> operation satisfies the square with its result inside its image, and every move is the identity, the
> program computes the same integers whatever representations are chosen.

Correctness comes from facts, and the choice is cost alone.

## Decision

1. **A representation is an abstraction function** (W1). A target declares each of its integer
   representations as data: a host type, the image it holds, and how to read and write a value where the
   reading is not the host's own (Java's `byte` holds 0..255 read `b & 0xFF`; a `long` holds U read
   unsigned). The word, the unsigned word, narrow table elements, the JVM's index, limbs and the host
   bignum are declarations of this one form, and their conversion rules are the one rule: a move is the
   identity on ℤ, which the printer realizes and the program never writes.
2. **ρ chooses by place** (W2):
   - at a **host boundary or a record field**, the host's type, stated by the declaration, never chosen;
   - in a **table**, the cheapest representation, by size, whose image holds the element set, including
     a second reading of a host type where it is denser;
   - for a **scalar**, the target's preferred register width for the value's fact (the word; the JVM's
     `int` for an index; V8's int32 where a bitwise operation needs it), narrower only where measured
     faster;
   - **above the word**, item 3.
3. **Above the word, LLVM holds a finite set in native iN** (W3), N the set's bit length rounded up to a
   multiple of 64. The limb library is retired on LLVM targets once measured against it on the u128
   factorial and the Karatsuba programs. An unbounded set has no representation on LLVM, which has no
   bignum, until a program asks for one.
4. **The word is S₆₄ = [−2⁶³, 2⁶³) on every LLVM triple, i686 included** (W4): the word is where
   arithmetic is exact, and `i64` is exact on i686. The preferred register width there is 32 bits, so a
   value proven in S₃₂ is computed in `i32`. Pointer-sized host types take the triple's width from the
   triple's target layer.
5. **A table's element representation is Theorem D′'s** (W5; [ir.md](../spec/ir.md) §4.3, built
   since irstep4a): ρ of the hull of its flow class's proven element ranges, and a declared
   representation in the class fixes it, the class's range an obligation against it. This decision
   leaves that rule as it is and widens ρ's codomain (Java's `byte` read as U₈, JavaScript's typed
   arrays). widths-research.md had said inference waited for a program; Theorem D had built it, which
   was found while building this decision.
6. **IR_P carries each value's representation** (W6): every printer reads the width and the reading from
   the IR's text alone (ADR 0038), and `cmd/check` reprints them byte for byte.

## Why not

- **Extend case by case** (widths-research.md's A): a seventh, eighth and ninth mechanism, each with its
  own conversions and faults; the theorem above would have to be argued once per mechanism.
- **Width types in the language** (`u8`, `i32`): a type that chooses a representation is a set naming
  one, which ADR 0079 removed; rejected before by bitwise.md (candidate E) on WebAssembly's precedent.
- **The narrowest representation everywhere**: in a register the narrowest is never cheaper and costs
  extensions; "narrowest" is not even defined, since U₈ and S₁₆ both minimally hold 0..255.
- **The word S₃₂ on i686**: the same program would compile for x64 and be refused for i686, and the
  facts already put most values in S₃₂, where they are computed in `i32` under item 4.
- **Keep the limb library on LLVM**: LLVM legalizes iN itself; a limb library compiled to LLVM IR is
  lowering further than the target requires. It stays where the host has no wide integer (Go, the JVM).

## Consequences

- **Built in stages with ADR 0082, program first**: `hash/fnv` and `hash/crc32` over a file, at parity
  with Go's own on Go, Java and JavaScript. Their first needs from this decision are Java's `byte` read as
  U₈ (the file's bytes, no copy), U₃₂ tables (CRC's), and the unsigned word on the JVM and V8, which
  FNV-1a 64 holds. Each stage passes `cmd/check`, byte-identical except where a representation changes
  and is measured.
- **LLVM's widths, Win32's types and the rung above as iN** are built with `ir/llvm`.
- **The existing mechanisms are re-declared in the new form** when this build reaches them, each move
  byte-identical, so the old and the new never coexist as two systems for long.
