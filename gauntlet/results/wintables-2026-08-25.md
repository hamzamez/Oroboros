# Tables on windows — 2026-08-25

The language's data structure now works on all four targets. That closes a rule violation:
`array`, `table`, `len`, `alloc`, `build` and `set` are the language's, and CLAUDE.md says a
construct promoted to the language works on every target — but
[tables-write-2026-08-25](tables-write-2026-08-25.md) shipped with windows refusing all of them.

Two decisions were owed and are made here. Both are the **target's**, not the language's.

**And the portable sieve ends up at 0.88× of hand-written assembly** — after starting at 3.7×. What
closed the gap was two things this host makes visible and the other three hide: the element size,
and a threaded buffer costing a register (§4).

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

## 4. It cost 3×, and the 3× is gone

The first version was **3× slower** than hand-written assembly. Two changes removed it, and the
second was not the one that was predicted.

| 100 rounds of a 200,000 sieve | min | median | vs hand-written |
|---|---|---|---|
| hand-written assembly | 51 | 52 | — |
| emitted, `examples/native/sieve-win.oro` — a byte array via `x64.movb` | 47 | 48 | 0.92× |
| **table, eight bytes per element** | 187 | 194 | **3.7×** |
| **table, one byte per element** | 84 | 89 | **1.71×** |
| **table, + invariant loop variables** | **45** | **46** | **0.88×** |

Three independent rounds of eleven runs each; the ordering held in all three.

### 4a. Element size is part of the type

Eight bytes per element against one. A 200,000-element boolean sieve was 1.6 MB instead of 200 KB,
so the marking loop moved eight times the memory.

This host has no types, so the width has to be **carried**: one byte for a bool, eight for
everything else, read off the value a `set` stores or the value a rule produces. It is keyed by
NAME, because a table crosses binders — a `build` buffer is threaded through two loops and re-bound
by a `let` — and losing it at any one of them reads a byte array as qwords, which is a wrong answer
rather than a slow one. That is exactly what the first version did to the sieve's counting loop:
`c` there is bound by a `let` whose value is a `build` **term**, not a name, so the width has to be
readable off the term too.

Go never showed any of this, because Go has a `bool` and `[]bool` is one byte per element. Three
hosts were sizing our elements for us through their own type systems.

### 4b. And then a threaded buffer cost a register

One byte per element left 1.71×, and the emitted code said why:

```asm
Ltop10:
        mov r10, qword ptr [rsp+48]     ; reload j
        cmp r10, 200000
        jge Lnext11
        mov r10, qword ptr [rsp+48]     ; reload j
        mov byte ptr [r15+r10+8], 1
        mov r10, qword ptr [rsp+48]     ; reload j
        mov r14, r10
        add r14, r12
        mov qword ptr [rsp+48], r14     ; store j
        jmp Ltop10
```

Five memory accesses per element, for a loop that needs none. The sieve threads its buffer through
both loops, and giving it a place of its own cost a register plus a copy in and out — which pushed
the **index** to a spill slot.

**A threaded buffer looks like it changes and does not.** `(again (set c j v) …)` hands back what
it was given, because `set` consumes its argument and returns it — and
[ADR 0018](../../docs/decisions/0018-immutable-values-linear-buffers.md)'s linearity is what makes
that reliable rather than merely usual: nothing else can be holding the buffer, so nothing else can
have replaced it. So the variable keeps the place its initial value is already in, with no register
of its own and no copy.

The place is **aliased, not taken** — it usually belongs to an enclosing binder, since a nested loop
threading the same buffer is the common case — so it is not released at the end.

```asm
Ltop10:
        cmp r15, 200000
        jge Lnext11
        mov byte ptr [rdi+r15+8], 1
        add r15, r12
        jmp Ltop10
```

### 4c. What this says

**The portable program is now faster than the hand-written assembly it was 3× behind**, and faster
than the target-native version that reaches for `x64.movb` directly. 12% is at the edge of the
noise floor, so the honest claim is *at parity, consistently slightly ahead* rather than a win —
but the direction held in every one of 33 runs per program.

The result that matters is not the 12%. It is that **both costs were in the same place**: this host
has no type system to size our elements and no register allocator to spill for us, so both showed
up as our problem. ADR 0016 said x86 was worth having for exactly this reason — *the optimisations
you were parasitizing only become visible on a host that has none* — and both fixes are ours to
keep on every target that ever needs them.

### 4d. One thing built and reverted

A byte-table read used as a **guard** can fuse into the compare — `cmp byte ptr [c+i+8], 0` instead
of `movzx` + `cmp` — which is three instructions where x86 does one, and is what `x64.test-byte`
exists for. It was built and measured: **90/90 and 74/78 ms** against the unfused form across
repeats, indistinguishable. The loop is memory-bound and the extra instructions hide behind the
cache traffic, which is [bce-2026-08-15](bce-2026-08-15.md)'s finding arriving again — a saving on
a compute-bound loop is nothing on a memory-bound one.

Reverted, because nothing measured supports it. The comment in `branchUnless` says so, so the next
person to notice the pattern finds the measurement rather than repeating it.

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
