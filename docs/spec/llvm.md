# The LLVM printer: IR_P into LLVM IR

Status: **specified 2026-11-02 (ADR 0080, step 6b)**; §9's first two steps built
([llvm-2026-11-02](../../gauntlet/results/llvm-2026-11-02.md)): every differential case MASM runs agrees
on `windows-llvm`, and the sieve and merge sort run at 0.65 and 0.36 of MASM. This file says what each IR_P construct prints as, and why the printed program means what
the IR means. Where this file and the code disagree, this file is the specification.

## 1. What the printer is

`ir/llvm` is a map h_L : IR_P → LLVM IR text, read from the IR's text alone (ADR 0038), like the other
printers. It is correct when it preserves meaning, ⟦h_L(p)⟧ = ⟦p⟧ for every p the IR accepts (Leroy's
semantic preservation).

**LLVM adds no check** (ADR 0080, backend-2026-10-26 §1). An LLVM operation has undefined behaviour or
yields poison outside a domain D. Each flag the printer writes asserts a domain, and it writes one only
where the IR proved the operands inside it: an `exact` operation is proven inside the word, an index is
proven in bounds, a divisor is proven not zero. So h_L(p) has no undefined behaviour when p's
obligations are all discharged, and that is every p the decision accepts (Lee et al., "Taming Undefined
Behavior in LLVM", PLDI 2017, for poison's semantics). A flag stated without its proof would be a
miscompilation the optimizer is entitled to make. That is why W6 justifies every π (pijustified-
2026-11-02): π-parameters are where many of those proofs come from.

**The printer states the IR's proofs, and only them** (ADR 0080 item 2), and a build with
`-llvm-noflags` prints none of them, for diagnosing a proof.

## 2. The target

`targets/windows-llvm/` is the windows target with this backend, `backend llvm`, triple
`x86_64-pc-windows-msvc`. It is a separate directory only until MASM retires at parity (ADR 0080
item 5); then it becomes `targets/windows`. Its files are the host's, under the host's own names:
- `windows-llvm.oro`: the sets, the word, the representations, the link line;
- `kernel32.oro`: the Win32 functions programs call, each `declare dllimport` in the printed module,
  and `build`'s allocator, `HeapAlloc` with `HEAP_ZERO_MEMORY`;
- `llvm.oro`: Σ_LLVM, the instructions and intrinsics offered under their own names (`llvm.sdiv`),
  each operation's undefined behaviour or poison its `where`. It is declared by hand, as the survey
  ([llvmsurvey-2026-10-27](../../gauntlet/results/llvmsurvey-2026-10-27.md)) classified it, and grows
  family by family. The language's operators are found in it by spelling, as on every target: `+` is
  `add`, `/` is `sdiv`, `<` is `slt` (emit/target.go's `langOps`);
- `strings.oro`: the free monoid, `string_of` and `++`, each a helper function the module defines, and
  `utf8`, a string's bytes.

`lib/win/print.oro` prints integers and strings in the language's own operators over a byte table,
where `lib/win/fmt.oro` names x86's mnemonics; it takes that name when MASM retires.

The build compiles with `clang -O2 -c` and links with MSVC's `link`, the libraries of the link line,
and `-entry:main`, as the MASM target links. No C runtime is linked unless a primitive names one, and
every function is `"no-builtins"`, so no loop becomes a call to the runtime's `memset` or `memcpy`.

## 3. Types

A value's IR_P type is its representation ρ (ADR 0083), and h_L maps it to an LLVM type:

| IR_P | LLVM | |
|---|---|---|
| `int`, `int LO HI` | `i64` | the word S₆₄, ADR 0083's choice on every triple |
| `u64` | `i64` | the unsigned word: its operations are the unsigned ones (§5) |
| `bool` | `i1` in a value, `i8` in a table | |
| `f64` | `double` | F64's representation (numbers.md §6.2) |
| `array σ`, `buffer σ` | `ptr` | §4 |
| `string` | `ptr` | a table of bytes, §4 |

The narrower widths (`i8`, `i16`, `i32` scalars, ADR 0083's register rule) are not printed in the first
step: every integer value is `i64`, and a table's element is the width §4 gives it.

## 4. Tables

A table is a pointer to a header and its elements: an `i64` length, then the elements, each of its
element type's width. An element is held as `ir/x86` holds it, in the representation the target
declares for its range: `repr 0..255 host "i8"` is the one this target declares, so 0..255 is an `i8`
and every other integer an `i64`; a `bool` is an `i8`; a `f64` a `double`; a table or a string a `ptr`.
The width is read off the table's representative's type, so every alias of a table reads it alike.
- **`len t`** is the `i64` at t.
- **`index t i`** is `getelementptr inbounds` of the element at i, loaded and widened: `zext` for a
  byte, `icmp ne` for a Boolean. `inbounds` is the index obligation 0 ≤ i < len t, which the IR proved,
  and `-checked` never clears (refinements.md §3a).
- **`set b i x`** stores x, narrowed with `trunc` (a Boolean with `zext`), which is exact because x is
  proven inside the element's range. A buffer is linear (ADR 0018, R0–R4), so the store is in place and
  the result is b.
- **A table's storage** is 8 + n·w bytes from kernel32, zero on creation, which is a table's semantics
  (tables.md), so a `build`'s fill of zeros prints nothing: `VirtualAlloc` from 64 KiB, whose pages the
  system has zeroed, and the process heap with `HEAP_ZERO_MEMORY` below, since `VirtualAlloc` reserves
  64 KiB a call (measured, llvm-2026-11-02). A null answer is a trap, never a pointer LLVM may assume.
- **`restrict t n`** is not printed in the first step, as on MASM: a restriction's length must survive
  a join, which a header-held length does not.
- **A string literal** is a private constant `{ i64 len, [len x i8] }`, its bytes UTF-8.

## 5. Operations

| IR_P | LLVM | the flag's domain, proven by |
|---|---|---|
| `add exact a b` | `add nsw` | the result inside the word (the decision) |
| … both operands proven ≥ 0 | `add nsw nuw` | the operands' types |
| `sub exact` | `sub nsw`; `nuw` when b ≥ 0 and the result ≥ 0 | |
| `mul exact` | `mul nsw`; `nuw` when both ≥ 0 | |
| `neg exact a` | `sub nsw i64 0, a` | |
| `div exact a b` | `sdiv` | b ≠ 0 and not (a = −2⁶³ ∧ b = −1): the division's obligations |
| `rem exact a b` | `srem` | the same |
| `mod a m` | `srem`, then `select` of it + m where it is negative | m > 0, its `where` |
| `and`, `or`, `xor`, `not` | `and`, `or`, `xor`, `xor −1` | total on ℤ (ADR 0082) |
| `shl a k` | `shl`, and `select` 0 where k > 63 | k ≥ 0, its `where`; a·2ᵏ inside the word |
| `shr a k` | `ashr`, and `select` `ashr a, 63` where k > 63 | k ≥ 0, its `where` |
| an operation in `wrap` mode | the same instruction with no flag | ADR 0082's fusion: the residue is the answer |
| `eq` … `ge` on integers | `icmp eq/ne/slt/sle/sgt/sge` | |
| `u64` operations | `add`/`sub`/`mul` with `nuw`, `udiv`, `urem`, `icmp ult…` | ADR 0033's word selection |
| `eq` … `ge` on doubles | `fcmp oeq/une/olt/ole/ogt/oge` | a double is finite (F64), so ordered |
| `neg` on a double | `fneg` | |
| `const` | the literal: `i64 N`, `i1`, `double` in hexadecimal | exact |
| `call f …` | f's template (§6) | f's `where`, an obligation the IR discharged |
| `assume c` | nothing | an `assume` is the analyses', read at entry |
| `the`, `require` | do not reach IR_P (W9) | |

A shift by 64 or more is poison in LLVM, where in ℤ a·2ᵏ is 0 (a is 0, by the obligation) and
⌊a/2ᵏ⌋ is 0 or −1; so each shift is the instruction below 64 and its limit past it, chosen by `select`,
which takes no poison from the operand it does not choose. `>>` past a count of 63, refused on MASM, is
total here. `shl` carries no `nsw` in the first step: its template is several instructions (§6).

A `trap`-mode operation (`-checked`) prints LLVM's overflow intrinsic, `llvm.sadd.with.overflow.i64`
and kin, and a branch to `llvm.trap` on overflow: the trap is the answer `-checked` asks for, and the
value is the intrinsic's, which is the exact one where there was no trap.

## 6. Calls: a declaration's template

A target declaration's `host expr` is an LLVM instruction's right-hand side: `${x}` is the operand x as
a typed value (`i64 %v3`), `${x:v}` x's value alone (`%v3`), and the printer writes `%vN = ` before it.
A template that names its result, `${r}`, is instead the instructions themselves, so it may be several,
with blocks of its own; `${#}` is `.N`, a number unique to the expansion, for its registers and labels
(`%o${#}`, `trap${#}:`). The printer's own names begin with `.`, and an IR value's is `%vN`, so none of
the three collide. A `host stmt` is one or more instructions, and its value is its first argument. A
declaration's `import` strings are lines of the module, each written once: a `declare`, a global, or a
helper's whole `define`. So a Win32 function is

    fn GetStdHandle(n ∈ -2^31..2^31-1) (r ∈ ptr) {
        host expr "%n${#} = trunc ${n} to i32\n${r} = call ptr @GetStdHandle(i32 %n${#})"
            import "declare dllimport ptr @GetStdHandle(i32)"
    }

and an instruction is

    fn sdiv(a ∈ Int, b ∈ Int) (Int) pure where ne(b, 0) { host expr "sdiv ${a}, ${b:v}" }

**The flags are the printer's** (§5), since they are proofs about one operation's operands, which the IR
holds and a declaration cannot: where an `add`, `sub` or `mul` is proven, the printer writes them after
the template's first word, `add nuw nsw i64 …`. A template of several instructions takes none.

A `where` is an obligation (ADR 0080 item 7). A primitive the language cannot state is a wall
(llvmsurvey-2026-10-27 §4) and is not declared.

## 7. Control: regions into blocks

Structured SSA (ADR 0032) prints as basic blocks, and a region's parameters and an operation's results
are **slots**: an `alloca` per value in the entry block, stored where control passes to it and loaded
where it is read. `clang -O2`'s mem2reg makes them φ-nodes, the front-end idiom clang uses for C. A
statement's value is an SSA register `%vN`, its IR_P number.
- **`if c {A} else {B}`**: `br i1 c, %then, %else`; each arm's `yield` stores the results and branches
  to the join.
- **`loop (init z…) body`**: the parameters are stored with z and the body's block entered; `continue`
  stores the new values and branches back to the body's head, and `break` stores the loop's results and
  branches to the exit.
- **A branch both of whose arms continue, computing only operations with no undefined behaviour**
  (constants, `+ − · neg`, the Boolean operations, comparisons; never a division, a load, a call or a
  trap) is one back edge whose arguments are chosen: both arms computed, then `select` per argument
  (ADR 0080 item 2). It is sound under poison, since `select` takes none from the operand it does not
  choose (LangRef). It is printed by hand because clang `-O2` does not make it: the sieve's count of
  unmarked bytes ran at 0.99 of MASM as a branch and 0.63 as selects (backend-2026-10-26 §4).
- **`branch c`** at a region's end is `br i1`.
- **`unreachable`** is LLVM's `unreachable`, after the call to `abandon`.
- **A π-parameter** is its source renamed (L9): it prints nothing, and reads of it read its source.

## 8. The function and the entry point

An exported function prints as `define internal` with its parameters and its result, its result type
LLVM's for a single result and an anonymous struct for several. The entry point `oro-main` is
`define void @main()`, ending in `ExitProcess(0)`; every function is `nounwind`. A function's name is
its IR name with `-` made `_`.

## 9. Built in order, each with a time limit

1. **The printer and the target, for the differential suite's windows cases** (one session): §3–§8 for
   the operations those cases use. `targets/windows-llvm/` with `kernel32.oro`, and `lib/win/fmt`'s
   printing written in the language's operators over a byte table, as ADR 0080 item 5 says programs
   naming x86 instructions move. The differential runner gains the target, and every case that runs on
   windows runs on it with one answer.
2. **The gauntlet's sieve at parity** with hand-written MASM and with `ir/x86` (one session).
3. **Σ_LLVM's stateable families declared** (`llvm.oro`), each with a differential case.
4. **MASM retired** (ADR 0080 item 5): `ir/x86` and `x64.oro` deleted, `windows-llvm` renamed
   `windows`, the programs naming x86 instructions ported.

**Not in these steps**: the narrow scalar widths (ADR 0083's register rule), `noalias` on buffers
(ADR 0080 item 2's next flag, measured when printed), 32-bit (`i686`, ADR 0080 item 3), the rung above
the word on LLVM (native `iN`, ADR 0083), `range` attributes.
