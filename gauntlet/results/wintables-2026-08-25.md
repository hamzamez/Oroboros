# Tables on windows — 2026-08-25

The language's data structure now works on all four targets. That closes a rule violation:
`array`, `table`, `len`, `alloc`, `build` and `set` are the language's, and CLAUDE.md says a
construct promoted to the language works on every target — but
[tables-write-2026-08-25](tables-write-2026-08-25.md) shipped with windows refusing all of them.

Two decisions were owed and are made here. Both are the **target's**, not the language's.

---

## 1. The representation: one pointer, length at offset 0

A table on this host is a single register — a pointer whose first eight bytes hold the length,
with element 0 at offset 8.

| rejected | why |
|---|---|
| fat pointer (ptr, len) | needs two registers; this convention passes one value per register, so a table would stop being a **value** |
| length as a separate argument | `(len a)` could not be computed from `a`, so `len` would not be the language's `len` |
| **length at offset 0** | **one register, and the header is free to skip** |

The header costs nothing at every access, because the displacement is part of x86's addressing
mode: `[rbx+rcx*8+8]` is the same instruction as `[rbx+rcx*8]`. `len` is one load. The only price
is the eight bytes themselves.

## 2. The allocator: the target's, and it declared one already

`alloc` and `build` are the language's; **where the bytes come from is the target's**. Go,
JavaScript and Java have allocation as syntax and their backends emit it directly. x86 has a call.

Rather than new declaration syntax, the allocator is found the way `findEq` finds equality — by the
last segment of a declared name, against `VirtualAlloc`, `malloc`, `HeapAlloc`. So
`targets/windows/` needed **no new declaration**: it already declared `VirtualAlloc`. A target that
declares none is told so, and told what to declare.

**Reclamation is neither of ADR 0018's two suggestions.** It is one `VirtualAlloc` per `alloc`,
never freed. That is the crude answer, its cost is a syscall per allocation which a program
allocating in a loop will feel, and it is the target's to change **without touching the compiler**
— which is the point of the division above.

---

## 3. It runs

```lisp
(def sieve (fn (n)
  (build n (fn (c)
    (loop ((c c) (i 2))
      (x64.setge (x64.imul i i) n)  c
      (c i)                         (again c (x64.add i 1))
      else                          (again (cross c i n) (x64.add i 1)))))))
```

`examples/table/sieve-win.oro` builds and runs, printing **1798400** — the same number
[windows-2026-08-19](windows-2026-08-19.md)'s hand-written assembly prints. **17 of 17 integer
operations bounded, 5 of 5 loops proven terminating.**

---

## 4. And it costs 3×, for a reason worth having in writing

| 100 rounds of a 200,000 sieve | min | median |
|---|---|---|
| hand-written assembly | 50 ms | 52 ms |
| emitted, `examples/native/sieve-win.oro` — a **byte** array via `x64.movb` | 46 ms | 47 ms |
| **emitted, `examples/table/sieve-win.oro` — the language's table** | **168 ms** | **188 ms** |

**Eight bytes per element against one.** A 200,000-element boolean sieve is 1.6 MB here and
200 KB there, so the marking loop moves eight times the memory and the cache does the rest.

This is not a compiler gap and it is not the loop shape — the native windows sieve, same backend,
same loops, is at **0.92×** of hand-written. It is that **the element size is not part of the
type**, so this backend uses eight bytes for everything because that is what `int` and `f64` need.

Go does not show it because Go has a `bool` and `[]bool` is one byte per element; the backend gets
the right size from the host's own type. x86 has no types at all, so the choice is ours and we made
the uniform one.

**Which is exactly what ADR 0016 said this host is for**: *the optimisations you were parasitizing
only become visible on a host that has none.* Three targets hid this because their own type systems
were sizing our elements for us.

### What follows from it

The fix is **element size in the type**, and it is named rather than guessed at: `(array bool)`
should be one byte on x86, with `movzx` on the read side. That needs the asm backend to carry an
element type per table, which it does not today — it has `isFloat` and nothing else.

Until then the honest statement is the one ADR 0001 already makes: portability is a property with a
price, and here the price is 3× on booleans. `examples/native/sieve-win.oro` remains, uses
`x64.movb`, and is at parity — which is the target-native escape hatch working as designed rather
than a workaround.

---

## 5. Three compiler gaps this found, all pre-existing

Making indexing structural on this target exposed three things that had never been reachable.

**The refinement layer understood none of x86's ordering comparisons.** Only `sete` and `setne`
were in `opAlias`; `setl`, `setle`, `setg` and `setge` were absent. Nothing had noticed because
this target's own indexing — `x64.mov` — declares no bounds precondition, so no obligation had ever
needed to read an x86 guard. With indexing as application the obligation is generated from the
form, and a windows program could not prove a single one of its own indices. Only the **signed**
forms were added: `setb`/`seta` are unsigned comparisons, a different relation on a signed value.

**`imul` was not multiplication.** The sieve's `(setl (imul i i) n)` guard was an opaque atom, so
the `x ≤ x·x` rule could not fire. This is the same bug the division line in `opAlias` already
records — *"their absence here meant `go./` was not recognised as division AT ALL"* — arriving on a
third operator.

**The table operations built an addressing mode without materialising.** A spilled operand lives in
a stack slot, and x86 cannot use memory as a base or index register, so the emitted store was
`mov qword ptr [r15+qword ptr [rsp+48]*8+8], 1` and MASM rejected it with *"constant expected"*.
`emitPrim` has always called `materialize` for templates; the table operations are the first code in
this backend that builds an addressing mode itself, and they had to learn the same discipline.

All three were found by writing one program and running it.
