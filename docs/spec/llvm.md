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

**Two layers** (ADR 0093). `targets/llvm/` is the LLVM layer, a theory with a parameter,
T_LLVM[Σ_OS], the same on every triple:
- `llvm.oro`: `backend llvm`, the sets, the word, the representations, and **the `realize` map**
  (ADR 0088): which declaration interprets each of the language's names, stated, never found by
  spelling;
- `instructions.oro`: Σ_LLVM, **LLVM's operations as they are**, each with its domain (the inputs
  where it is neither poison nor undefined) as its `where` or its parameters' sets:
  `llvm.shl(a, b ∈ 0..63)`, `llvm.sdiv` where b ≠ 0 and not −2⁶³ by −1. It is declared by hand, as the
  survey ([llvmsurvey-2026-10-27](../../gauntlet/results/llvmsurvey-2026-10-27.md)) classified it, and
  grows family by family;
- `lang.oro`: where no single instruction is a language operation on the whole of its domain, its
  realization as **a term over the host's operations** (ADR 0089): `~`, `<<` and `>>` past 63, `mod`,
  `/` and `%` at a divisor −1, the checked arithmetic (the overflow intrinsics' two results and
  `llvm.trap`, ADR 0090), `trap_if`, `nulfree` (ADR 0091), the crash, and the allocator (ADR 0092);
- `os.oro`: **Σ_OS, the parameter**: `region`, `release` and `zero` (ADR 0096), `write_err` and
  `exit`, five headers with no body,
  what ι's terms need from an operating system and no instruction computes;
- `str.oro`: a string's bytes, the layout this layer prints (§4).

Loaded alone it is refused, naming the three operations. `targets/windows/` is a member of its
family: `glue llvm`, `triple "x86_64-pc-windows-msvc"`, the link line and the build, and
- `os.oro`: the view Σ_OS → Win32, the three definitions over kernel32;
- `kernel32.oro`: the Win32 functions programs call, as they are, each `declare dllimport` in the
  printed module: `WriteFile` writes a table's segment, its domain the segment and a DWORD count. The
  rest of Win32 is the survey's generated layer (`gauntlet/stdlib/win32.go`), written the same way;

**A run is exit(0) ∘ main**: `cmd/build` composes the entry with Σ_OS's `exit`, so the printer's
`main` only calls it.

`lib/win/print.oro` prints integers and strings in the language's own operators over a byte table.

The build compiles with `clang -O2 -c` and links with MSVC's `link`, the libraries of the link line,
and `-entry:main`. **The program's manifest declares UTF-8 its code page**
(`<activeCodePage>UTF-8</activeCodePage>`, Windows 10 1903 and later), embedded by `link
/MANIFEST:EMBED`: Win32's `A` functions read their bytes in the process's code page, so with it a
string's bytes are the text the language means, and without it `CreateFileA("tést.txt")` named a file
`tÃ©st.txt` (code page 1252, mastretire-2026-11-03). The language's strings are UTF-8 (strings.md),
and this makes the identity s ↦ s + 8 the `A` functions' encoding on every string without a NUL. No C runtime is linked unless a primitive names one, and
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

### 3a. The unsigned word, U₆₄ (ADR 0083)

U₆₄ = [0, 2⁶⁴) is `i64` read unsigned: (H, d) with H the 64 bits and d their unsigned reading. `i64`
carries no sign, so the residue map r : ℤ → ℤ/2⁶⁴ is a ring homomorphism under both readings: `add`,
`sub`, `mul`, `and`, `or`, `xor` and `shl` are the same instructions on U as on S₆₄, ℤ's own where the
result is proven inside U. Order, the quotient, the remainder and the right shift do not commute with
r, and they are U's own instructions: `icmp ult ule ugt uge`, `udiv`, `urem`, `lshr`. On [0, 2⁶³) the
two readings agree, so a value moves between S₆₄ and U₆₄ as its bits. A shift past 63 is poison and
ℤ/2⁶⁴'s answer is 0, so `u64<<` and `u64>>` are terms over the instructions, as `<<` is (§5).

### 3b. Doubles, F64 (ADR 0086, numbers.md §6.2)

F64 is `double`, by d: a finite binary64 denotes its real, ±0 both 0. A constant prints as its exact
64-bit pattern, `0x3FB999999999999A`, never a decimal LLVM would round.

**A rounding is an instruction.** LLVM's default floating-point environment rounds to nearest, ties to
even, with traps disabled and the status flags unobserved (LangRef). There, `fadd`, `fsub`, `fmul` and
`fdiv` with no fast-math flag are IEEE 754's correctly rounded operations, so each realizes
round(F64, a ∘ b) by its `ensure` (`instructions.oro`). Without `contract`, no two fuse: two roundings
stay two instructions. `sitofp` of an integer in ±2⁵³ is exact, round(F64, n) = n. The obligations,
each rounding finite and each divisor not zero, are decided where the program writes the rounding, so
no instruction makes ±∞, NaN or −0.

**A comparison is N's order at the sort F64.** The language's `= < <= > >=` are N's, used at two
sorts, the word and F64. An interpretation of a many-sorted signature maps each operation at each
sort, so `=` at F64 is its own realization, a later one in `realize` (`llvm.f_eq`, …). A realization's
parameter set is its sort before its range: F64 holds of exactly the operands the IR types as doubles,
and an integer set of none of them (`sortsHold`, ir/realize.go). So `fcmp` is chosen where the operands
are doubles, and `icmp` where they are integers. An operation at F64 that no realization's sort holds
is refused, never printed as the word's. IEEE compares finite values as reals (§5.11), and `fcmp oeq`
reads −0 = +0, as d does. NaN is not in F64, so an ordered predicate (`oeq olt ole ogt oge`) and its
unordered twin agree on every pair; the ordered one is chosen because it claims nothing of NaN.

### 3c. The double word, an intermediate in i128 (ADR 0103)

`repr double 128` declares it. The residue map r: ℤ → ℤ/2¹²⁸ is a ring homomorphism, injective on any
interval of 2¹²⁸ integers. So a term of +, −, · and neg whose every intermediate the decision proves in
one of i128's two readings, (−2¹²⁷, 2¹²⁷) signed or [0, 2¹²⁸) unsigned (ADR 0105), is computed exactly
in i128: each step's residue is its value's, and the lift of a residue in its reading is unique. Each
value is read by its own fact: `nsw` only inside the signed range, `nuw` on naturals, a comparison of
naturals unsigned. E itself is (−2¹⁹⁰, 2¹⁹⁰) since ADR 0105.

An intermediate is a value outside the word whose every use reads it back inside the term:
- + − · neg, its result again an intermediate or in the word;
- a quotient, remainder, `mod` or right shift of which it is the dividend, by a positive constant;
- a comparison;
- a π, the same integer refined, whose uses are its own.

It is never a table's element, a call's argument, a loop variable, a branch's condition or a region's
exit (W14), so no value of the program leaves the word and no claim above the word is needed
(ADR 0079). `ir.SelectDouble` chooses them, a greatest fixed point, and marks each `double` in IR_P,
typed by its proven range.

| IR_P | LLVM |
|---|---|
| `add double a b`, `sub`, `mul`, `neg` | `add nsw i128`, … ; `nuw` on `add` and `mul` of naturals |
| a word operand of one | `sext i64 a to i128` |
| a quotient, remainder, `mod` or shift reading one | `udiv`, `urem`, `lshr` i128 of a natural, else `sdiv`, `srem`, `ashr`; `mod` of an integer `srem` lifted into [0, m); then `trunc i128 … to i64` |
| a comparison reading one | `icmp` on i128 |

A divisor is a constant, or a positive word where the decision proves the quotient a digit (ADR 0104).
By a constant, clang multiplies by its reciprocal, inline; by a word, LLVM would call `__udivti3`, so
the triple's 2-by-1 division is used (x86-64's `divq`, `llvm/isa.udiv-2by1`, its domain hi < d),
on the dividend's two halves. The quotient's digit bound is the two-digit lemma, a transfer of the
decision: t = u·C + w with w < C and u a π of u < d has ⌊t/d⌋ < C. The double word also reads back
by `&` with a constant mask (ℤ's & is 2-adic, so t & (2ᵏ − 1) = t mod 2ᵏ) and by a right shift by a
word count in [0, 127].

### 3d. The full adder, the triple's add-with-carry (ADR 0108)

φ reads an integer by its two halves: t ↦ (t mod B, ⌊t/B⌋) on [0, 2B), and t ↦ (t mod B, [t < 0]) on
[−B, B), B = 2⁶⁴. Each is a bijection onto [0, B) × {0, 1}. The machine's add-with-carry is φ ∘ (+)
on [0, B)² × {0, 1}: adc(c, x, y) = φ(x + y + c) and sbb(b, x, y) = φ(x − y − b). The printer prints a
term of the double word as the triple's adder (`llvm/isa.add-carry`, `sub-borrow`,
`targets/windows/x64.oro`) when IR_P's types prove the premise:

- it is x + y + c, x + c, x − y − b or x − b, each inner + or − used by it alone, with x, y in [0, B)
  and c, b in [0, 1];
- its every use reads it by φ: t & m for a word m, which is (t mod B) & m since & is 2-adic; t >> 64
  of a sum; t < 0 of a difference.

| IR_P | LLVM |
|---|---|
| the top `add double` or `sub double` | `%c = trunc i64 c to i8`; `call { i8, i64 } @llvm.x86.addcarry.64(i8 %c, i64 x, i64 y)` (`subborrow`) |
| an inner one | nothing |
| `and t m` | `extractvalue …, 1`, then `and i64 …, m` |
| `shr t 64` | `extractvalue …, 0`, then `zext i8 … to i64` |
| `lt t 0` | `extractvalue …, 0`, then `icmp ne i8 …, 0` |

The IR is unchanged, and the choice is a function of IR_P's text. It pays only where the fold has no
clause testing the index at every digit: K(ℕ\*)'s A and S are two loops in sequence for that reason
(natnum.oro). LLVM's target-independent `uadd.with.overflow` is not combined into `adc`
(carry-2026-11-13), so the triple's own operation is used. The planted faults are `PlantAdderFault` (a
bit leaf in [0, 2]) and `PlantAdderReadFault` (any shift read as the carry).

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
- **A table's storage** is an operation of the IR (ADR 0092, `ir.Storage`): before each `build`,
  `tabulate` and array literal of n elements of w bytes (`ir.ElemBytes`, the layout every access reads
  by), `p = alloc(n, w)`, which the statement takes as its storage (`store %p`). **w is the table's
  final layout**: its class's representation by Theorem D′ (`ir/final.go`), computed before the
  storage and written onto every table but a parameter, so the allocation, every release, every
  token and the printer read one width, in a function and across a kept one's boundary. Taken from
  the type before the classes, a byte table was allocated 8 bytes an element and released as 1, and
  past 8 191 bytes the two chose different allocators (files-2026-11-16). ι substitutes
  `lang.allocate`, alloc : ℕ × {1, 8} → Region, whose guard 0 ≤ n ≤ ⌊(2⁶³ − 9)/8⌋ makes n·w + 8 a
  proven `mul nsw` and turns a larger request away as exhaustion, the trap. Its region is zero on
  creation, which is a table's semantics (tables.md), so a `build`'s fill of zeros prints nothing:
  `VirtualAlloc` from 64 KiB, whose pages the system has zeroed, and the process heap with
  `HEAP_ZERO_MEMORY` below, since `VirtualAlloc` reserves 64 KiB a call (measured, llvm-2026-11-02).
  A null answer is a trap, never a pointer LLVM may assume.
- **A table's release** is an operation of the IR too (ADR 0096, `ir.Release`): after an owned table's
  last use on each path where it is not moved, `release(t, len t, w)`. ι substitutes `lang.release`,
  whose b = n·w + 8 is proven from the fact `len(t) ≤ 2⁴⁷ − 9`, x64's user space (windows.oro), and
  which calls Σ_OS's `release(p, b)`. windows defines that by the rule `region` chooses by,
  `VirtualFree(p, 0, MEM_RELEASE)` from 64 KiB and `HeapFree` below, so release ∘ region is the
  identity on the heap. A table that is not owned, being shared, inside another value, a host's or a
  constant, is not released. The verifier's W11 re-derives every release (ir.md §3).
- **A dead table's region is reused** by the next table of its size (ADR 0096, `ir.Reuse`):
  release(p, b) followed by region(b) is p zeroed, up to its address, since nothing reaches p after its
  release. Locally, a release followed in its region by an allocation of its size becomes that
  allocation's token. Across a loop's back edge, the body takes a token parameter, null on entry, which
  every path consumes once: `reuse` (lang.reuse: the token zeroed by Σ_OS's `zero`, windows'
  `RtlZeroMemory`, or `allocate` when it is null), or `discard` (its release unless null) at the start
  of an arm that makes no table of its size. Each continuing arm passes a released table's storage, or
  null. Sizes are static. W12 re-derives every token (ir.md §3).
- **A table born full takes a region with no zero fill** (ADR 0097, `ir/fill.go`): region = zero ∘
  region_raw, and the zero is a dead store when every element is written before any is read. A rule and
  an array literal are born full by construction, and a build by the written prefix: its loop writes
  b[i] at a counter from 0 up to its length, or down from n − 1, on every continuing path, without
  reading b. Such a table is allocated by `alloc-raw` over Σ_OS's `region_raw` (`HeapAlloc` without
  `HEAP_ZERO_MEMORY`) and reused by `reuse-raw`, which does not zero its token.
- **`restrict t n`** is not printed in the first step, as on MASM: a restriction's length must survive
  a join, which a header-held length does not.
- **A string is the table of its UTF-8** (ADR 0117): UTF-8 is a monoid isomorphism ρ : Σ\* ≅ V onto
  the valid UTF-8 V ⊂ B\*, and here ρ is the identity on the representation, `llvm/str.utf8` (whose
  `ensure` states ρ's image is V) and its inverse `llvm/str.text` (whose `where` is V). Every table's
  region holds one zero byte past its elements, b(n, w) = n·w + 9 (`lang.bytes`), written by the
  allocator: so a string's bytes, at s + 8, are the C string Win32's `LPCSTR` reads, exactly when the
  string holds no NUL itself (B\*₀; ADR 0091), and nothing the language computes reads the zero. A
  literal is `{ i64 len, [len+1 x i8] }`. `string_of`, `++`, ι and ι_V are definitions in the language
  (`targets/llvm/lang.oro`: `encode`, `cat`, `ascii_of`, `text_v`), each a table read back by ρ⁻¹;
  ρ⁻¹ is the identity, so its table becomes the string, and the release pass reads it as a move.

## 5. Operations

| IR_P | LLVM | the flag's domain, proven by |
|---|---|---|
| `add exact a b` | `add nsw` | the result inside the word (the decision) |
| … both operands proven ≥ 0 | `add nsw nuw` | the operands' types |
| `sub exact` | `sub nsw`; `nuw` when b ≥ 0 and the result ≥ 0 | |
| `mul exact` | `mul nsw`; `nuw` when both ≥ 0 | |
| `neg exact a` | `sub nsw i64 0, a` | |
| `div a b`, b ≠ −1 or a ≠ −2⁶³ proven | `sdiv` | its domain, decided on the operands' intervals |
| `div a b`, `rem a b`, a ≥ 0 and b ≥ 1 proven | `udiv`, `urem`, by 2ᵏ LLVM's `lshr` and `and` | on ℕ ∩ S₆₄ truncation is the floor and the bits read alike signed or unsigned; the narrowest realization the facts prove (llvm-udiv-2026-11-11) |
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
asks for. Inside a term, a host operation that is a stated realization of g is g. A declaration in `instructions.oro` is LLVM's operation as it is and means only that; a program naming
`llvm.shl` owes `shl`'s domain, whatever `<<` is realized by.

A `where` is an obligation (ADR 0080 item 7). A primitive the language cannot state is a wall
(llvmsurvey-2026-10-27 §4) and is not declared.

## 6a. A C function: its call derived from its prototype (ADR 0094)

`fn F(a ∈ S₁, …) (r ∈ S) { host expr call "F" [stdcall | cdecl] }` declares a C function by its
symbol, and the sets are its prototype: over C's integer types the full range determines the type,
so a C integer's set is its type's full range, and a narrower domain is a `where`.

| set | C type, LLVM type | into the call | out of it |
|---|---|---|---|
| −2^(w−1)..2^(w−1)−1, w ∈ {8, 16, 32} | signed, `iw` | `trunc` from i64 | `sext` to i64 |
| 0..2^w−1, w ∈ {8, 16, 32} | unsigned, `iw` | `trunc` from i64 | `zext` to i64 |
| −2⁶³..2⁶³−1 | 64-bit, `i64` | as it is | as it is |
| 0..2⁶³−1 | 64-bit unsigned, in the word, `i64` | as it is | as it is |
| `ptr` | a pointer, `ptr` | as it is | as it is |
| `String`, `where nulfree(a)` | `const char *`, `ptr` | its bytes, s + 8 (§4) | — |
| ∅, or `host stmt` | `void` | — | — |

`trunc` is exact on the set it is applied to, and `sext` and `zext` are the embedding back, so the
call is ℤ's: out ∘ F ∘ in. The `declare` is the prototype's image, `dllimport` on a Windows triple,
and the convention is the triple's: `stdcall` is `x86_stdcallcc` on i686 and the C convention on
x86-64. A set that is no C type is refused when the target loads, and so is a C-string parameter
without `where nulfree`.

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
3. **Σ_LLVM's stateable families declared** (`instructions.oro`), each with a differential case: **after step 4,
   and by demand** (hamza agreed the order, 2026-11-03): a family is declared when a program needs it,
   TinyRetroPad's first (CLAUDE.md 6c), each with its case.
4. ~~**MASM retired**~~ **built** (ADR 0080 item 5, mastretire-2026-11-03): `ir/x86`, `x64.oro`,
   `msvcrt.oro` and `lib/win/fmt.oro` deleted, `windows-llvm` renamed `windows`, the programs naming
   x86 instructions ported to the language's operators, and the Win32 survey's writer to `call` and
   `declare`.

**Not in these steps**: the narrow scalar widths (ADR 0083's register rule), `noalias` on buffers
(ADR 0080 item 2's next flag, measured when printed), 32-bit (`i686`, ADR 0080 item 3), the rung above
the word on LLVM (native `iN`, ADR 0083), `range` attributes.
