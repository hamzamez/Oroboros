# The LLVM printer: IR_P into LLVM IR

Status: **specified 2026-11-02 (ADR 0080, step 6b)**; §9's steps 1, 2 and 4 built: every differential
case MASM ran agreed, the sieve and merge sort ran at 0.65 and 0.36 of MASM
([llvm-2026-11-02](../../gauntlet/results/llvm-2026-11-02.md)), and MASM is retired: this printer is
the windows target's ([mastretire-2026-11-03](../../gauntlet/results/mastretire-2026-11-03.md)). This file says what each IR_P construct prints as, and why the printed program means what
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

`targets/windows/` is the windows target, `backend llvm`, triple `x86_64-pc-windows-msvc` (until
2026-11-03 `targets/windows-llvm/`, beside MASM, which retired at parity, ADR 0080 item 5). Its files
are the host's, under the host's own names:
- `windows.oro`: the sets, the word, the representations, the link line, and **the `realize`
  map** (ADR 0088): which declaration interprets each of the language's names, stated, never found by
  spelling;
- `llvm.oro`: Σ_LLVM, **LLVM's operations as they are**, each with its domain (the inputs where it is
  neither poison nor undefined) as its `where` or its parameters' sets: `llvm.shl(a, b ∈ 0..63)`,
  `llvm.sdiv` where b ≠ 0 and not −2⁶³ by −1. It is declared by hand, as the survey
  ([llvmsurvey-2026-10-27](../../gauntlet/results/llvmsurvey-2026-10-27.md)) classified it, and grows
  family by family;
- `lang.oro`: where no single instruction is a language operation on the whole of its domain, its
  realization as **a term over the host's operations** (ADR 0089), a definition in the language:
  `~`, `<<` and `>>` past 63, `mod`, `/` and `%` at a divisor −1, the checked arithmetic (the overflow
  intrinsics' two results and `llvm.trap`, ADR 0090), `trap_if`, and the crash. One template remains,
  `build`'s allocator (a pointer as a value, ADR 0084's LLVM part);
- `kernel32.oro`: the Win32 functions programs call, as they are, each `declare dllimport` in the
  printed module: `WriteFile` writes a table's segment, its domain the segment and a DWORD count. The
  rest of Win32 is the survey's generated layer (`gauntlet/stdlib/win32.go`), written the same way:
  an integer of w < 64 bits passed by `trunc` from its C range and returned by `sext` or `zext` into
  it, a pointer as `ptr`, a C string as a string's bytes (§4);
- `strings.oro`: the free monoid, `string_of` and `++`, each a helper function the module defines, and
  `utf8`, a string's bytes.

`lib/win/print.oro` prints integers and strings in the language's own operators over a byte table.

The build compiles with `clang -O2 -c` and links with MSVC's `link`, the libraries of the link line,
and `-entry:main`. No C runtime is linked unless a primitive names one, and
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
element type's width. An element is held in the representation the target declares for its range: `repr 0..255 host "i8"` is the one this target declares, so 0..255 is an `i8`
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
- **A string** is its length, its UTF-8 bytes and a NUL after them: `{ i64 len, [len+1 x i8] }` for a
  literal, and the same layout from `string_of` and `++`, the only other operations that make one
  (`targets/windows/strings.oro`). The NUL is outside the length, so the abstraction function
  (ADR 0083's d) ignores it and nothing the language computes reads it; it is there so that a
  string's bytes, at s + 8, are the C string Win32's `LPCSTR` reads, exactly when the string holds no
  NUL itself (B*₀, the free submonoid on the non-NUL scalars; mastretire-2026-11-03).

## 5. Operations

| IR_P | LLVM | the flag's domain, proven by |
|---|---|---|
| `add exact a b` | `add nsw` | the result inside the word (the decision) |
| … both operands proven ≥ 0 | `add nsw nuw` | the operands' types |
| `sub exact` | `sub nsw`; `nuw` when b ≥ 0 and the result ≥ 0 | |
| `mul exact` | `mul nsw`; `nuw` when both ≥ 0 | |
| `neg exact a` | `sub nsw i64 0, a` | |
| `div a b`, b ≠ −1 or a ≠ −2⁶³ proven | `sdiv` | its domain, decided on the operands' intervals |
| `div a b` elsewhere | the term `if b == -1 { 0 - a } else { llvm.sdiv(a, b) }` | b ≠ 0; `0 − a` decided at the site |
| `rem a b`, the same proven | `srem` | |
| `rem a b` elsewhere | the term `if b == -1 { 0 } else { llvm.srem(a, b) }` | b ≠ 0 |
| `mod a m` | the term `r = llvm.srem(a, m); if r < 0 { r + m } else { r }` | m > 0, its `where` |
| `and`, `or`, `xor`, `not` | `and`, `or`, `xor`, `xor −1` | total on ℤ (ADR 0082) |
| `shl a k`, k proven < 64 | `shl nsw`, `nuw` when a ≥ 0 | a·2ᵏ inside the word; the realization chosen by k's proven set |
| `shl a k` elsewhere | the term `if k <= 63 { llvm.shl(a, k) } else { 0 }`, the arm's `shl` with its flags | k ≥ 0, its `where`; a·2ᵏ inside the word |
| `shr a k`, k proven < 64 | `ashr` | |
| `shr a k` elsewhere | the term `if k <= 63 { llvm.ashr(a, k) } else { llvm.ashr(a, 63) }` | k ≥ 0, its `where` |
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
⌊a/2ᵏ⌋ is 0 or −1; so each shift is the instruction below 64 and its limit past it, a branch in the
term, which LLVM speculates into a `select` where cheaper (sound: each arm is defined where taken). `>>` past a count of 63, refused on MASM, is
total here. Where the IR proves the count below 64, the realization is the instruction itself, chosen
per instance by its parameters' sets (ADR 0088, ir/final.go), and `shl` takes its flags.

`sdiv` and `srem` are undefined for −2⁶³ by −1 (LangRef), where ℤ's remainder is 0 and its quotient
2⁶³ is outside the word. The terms give −1 its own arm: `%` gives ℤ's 0, and `/` gives `0 − a`, the
language's `−`, decided at the site: 2⁶³ is refused, or trapped under `-checked`, as any result outside
the word is (ADR 0019). The other targets do not yet decide it (llvm-2026-11-02).

A `trap`-mode operation (`-checked`) is its checked form's term (ADR 0089, 0090), `(s, o) =
llvm.sadd_with_overflow(a, b); if o { llvm.trap() } else { s }` and kin: the trap is the answer
`-checked` asks for, and the value is the intrinsic's, the exact one where there was no trap.

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

**A host operation into a product** B₁ × … × Bₙ (`llvm.sadd_with_overflow(a, b) (Int × Bool)`)
answers LLVM's aggregate `{ ρB₁, …, ρBₙ }`: the printer names it and binds each result to its
projection, `extractvalue j` (ADR 0090). **A host operation into ∅** (`llvm.trap() (∅)`,
`kernel32.ExitProcess(n) (∅)`) never returns: its region ends `unreachable` after it.

**The flags are the printer's** (§5), since they are proofs about one operation's operands, which the IR
holds and a declaration cannot: where an `add`, `sub`, `mul` or `shl` is proven, the printer writes them
after the template's first word, `add nuw nsw i64 …`. A template of several instructions takes none.

**A realization is stated** (ADR 0088): `realize OP = A, B … [checked C]` in the target file names the
declarations that interpret the language's name OP: the first everywhere, a host operation or **a term
over host operations** (ADR 0089, inlined after the decision, its header OP's domain); each later one a
host operation, where its domain (parameters' sets and `where`) holds of the operands' proven facts,
the narrowest such, recorded on the operation as `via` in the IR's text; C is the trap `-checked`
asks for. Inside a term, a host operation that is a stated realization of g is g. A declaration in `llvm.oro` is LLVM's operation as it is and means only that; a program naming
`llvm.shl` owes `shl`'s domain, whatever `<<` is realized by.

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

1. ~~**The printer and the target, for the differential suite's windows cases**~~ **built** (llvm-2026-11-02): §3–§8 for
   the operations those cases use. `targets/windows-llvm/` with `kernel32.oro`, and `lib/win/fmt`'s
   printing written in the language's operators over a byte table, as ADR 0080 item 5 says programs
   naming x86 instructions move. The differential runner gains the target, and every case that runs on
   windows runs on it with one answer.
2. ~~**The gauntlet's sieve at parity**~~ **built** (llvm-2026-11-02): 0.65 of `ir/x86`, 0.64 of
   hand-written MASM.
3. **Σ_LLVM's stateable families declared** (`llvm.oro`), each with a differential case: **after step 4,
   and by demand** (hamza agreed the order, 2026-11-03): a family is declared when a program needs it,
   TinyRetroPad's first (CLAUDE.md 6c), each with its case.
4. ~~**MASM retired**~~ **built** (ADR 0080 item 5, mastretire-2026-11-03): `ir/x86`, `x64.oro`,
   `msvcrt.oro` and `lib/win/fmt.oro` deleted, `windows-llvm` renamed `windows`, the programs naming
   x86 instructions ported to the language's operators, and the Win32 survey's writer to `call` and
   `declare`.

**Not in these steps**: the narrow scalar widths (ADR 0083's register rule), `noalias` on buffers
(ADR 0080 item 2's next flag, measured when printed), 32-bit (`i686`, ADR 0080 item 3), the rung above
the word on LLVM (native `iN`, ADR 0083), `range` attributes.
