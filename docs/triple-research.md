# The second triple: what `i686-pc-windows-msvc` changes

2026-11-16. Item 4 of [assessment-2026-11-16](assessment-2026-11-16.md) §5: **research first, then
hamza's decisions, then built**. Program first: the suite on the triple, every failure classified,
before anything is built. Nothing here is built. The measurements use a scratch copy of `targets/`,
with each change the experiment makes said where it is made. §6 holds the decisions, each with a
recommendation. **Status** (2026-11-17): decided by ADR 0122. Built so far: the family (step 1) and the
target `windows32` with D3 and the sentinel ([windows32-2026-11-17](../gauntlet/results/windows32-2026-11-17.md):
66 of 74, the file-read failure of §3's item 2 a NUL nobody wrote, on both triples), and D6 (ADR 0123,
[d6-2026-11-17](../gauntlet/results/d6-2026-11-17.md): the triple's runtime in the language, 68 of 74).
D2 (with the rest of D4's `x86.oro`), D1 by header and D5 are next, in that order.

## 1. What a triple is

The windows target is a theory instantiated twice (ADR 0093): T_LLVM[Σ_OS], with Win32 interpreting
Σ_OS, glued to the triple's data. A **triple t** is a model of what the LLVM layer leaves open. Each
of the following is a function of t:

| what | a function of | on x86_64-pc-windows-msvc | on i686-pc-windows-msvc |
|---|---|---|---|
| the C ABI's map A_t from C types to LLVM types (ADR 0094) | the data model | LLP64: `int`, `long` 32 bits, `SIZE_T`, pointers 64 | ILP32: `int`, `long`, `SIZE_T`, pointers all 32 |
| a C call's convention | the triple | one convention, the C one | `__stdcall` is `x86_stdcallcc`, decorated `_F@n` |
| the machine's word, its registers | the ISA | 64-bit | 32-bit; LLVM's i64 is a register pair |
| the double word natively | the ISA | i128 by `mul`/`div` on RDX:RAX | i64 by `mul`/`div` on EDX:EAX |
| the address space | the OS on the ISA | 2⁴⁷ bytes of user space | 2³¹, or 2³² with `/LARGEADDRESSAWARE` under WOW64 |
| the ISA's own operations (`x64.oro`) | the ISA | `divq`, `adc`/`sbb` on 64 bits | `divl`, `adc`/`sbb` on 32 bits |
| the build line | the toolchain | `vcvars64`, clang's default target | `vcvars32`, `--target=i686-pc-windows-msvc` |

The language does not change. ADR 0083 item 3 keeps the word S₆₄ on every triple, so a program's
meaning and its proofs are the same. What changes is ρ, the representations below the word, and
the C ABI's sets.

## 2. Measured first: one program

A scratch copy of `targets/` with `triple "i686-pc-windows-msvc"`, built by `vcvars32`, clang and
`link` (experiments/i686/README.md).

1. **clang ignores the module's triple** unless it is told: `-Wno-override-module` silences the
   warning that its default target, x86_64, overrides it. The object was x64, and the x86
   `kernel32.lib` did not link. So the build line is a function of the triple (`--target=t`), and of
   the toolchain's environment (`vcvars32` against `vcvars64`). This is derived, not a choice.
2. **The data model.** `HeapAlloc(HANDLE, DWORD, SIZE_T)` declared with `n ∈ 0..2^63-1` is an i64
   on i686, so the call is decorated `_HeapAlloc@16`, and kernel32 exports `_HeapAlloc@12`. The link
   fails. ADR 0094 decision 1 already says the declaration's sets *are* its prototype, so the LLP64
   prototype is not the ILP32 one.
3. **The entry point** is decorated `_main` on x86, and `-entry:main` finds it (link decorates).
4. With the data model's sets at ILP32 in the experiment's copy (byte counts 0..2³²−1, a trap above
   them, `max_len` 2³¹ − 9), **`print_int(42)` and a product above 2³² ran on 32-bit Windows**, with
   the same output and exit 0.
5. **LLVM on i686**: `mul i64` and even `mul i128` and `udiv i128` are expanded inline. **`udiv i64`
   calls `__aulldiv`**, MSVC's runtime helper, and the windows target links no C runtime. The x86
   `ntdll.lib` exports none of the helpers. MSVC's static `libcmt.lib` has all ten: `__alldiv`,
   `__allrem`, `__allmul`, `__allshl`, `__allshr`, `__aulldiv`, `__aullrem`, `__aullshr`,
   `__alldvrm`, `__aulldvrm`. Only the x86_64 compiler-rt builtins are installed.

## 3. The suite on the triple

`experiments/i686/run32.go` runs every differential case on the triple, through `mk32.py`'s copy
(2026-11-16, `-j 4`). **74 cases**:

- **63 pass**, each printing its `// expect:` line on 32-bit Windows, exit 0. They cover the word,
  the unsigned word (i64 read unsigned, a pair on i686), the pair representation, F64, strings, maps,
  variants and products.
- **2 are the harness's**: `u128-fact` and `u128-ops` skip windows on x86_64 too (Go's
  `math/bits` is not bound), and with windows the only target nothing runs.
- **5 stop at the address-space bound** (§4.3): `big-fib`, `big-nat-div`, `big-nat-ops`,
  `big-render` (K(ℕ\*)'s tables are sized up to 2³² − 20 by `room`'s 2³¹ digits), and `read-loop`
  (`readfile`'s buffer up to 2⁴⁰ bytes, a literal in `lib/os/windows.oro`).
- **2 stop at x86-64's own operations** (§4.4): `big-kara` and `big-sqr` crash LLVM's i686
  backend, *Do not know how to expand the result of this operator!*, at `@llvm.x86.addcarry.64` in
  `nat.add`. The five above may meet it once past the bound.
- **2 stop at the division helpers** (§4.6): `bits-edges` (`__allrem`) and `word-wide` (`__alldiv`)
  are refused by the link.

**Found in passing**:
1. **`llvm.address` is declared on a false premise for i686**, "on every triple a pointer is the
   word" (`targets/llvm/instructions.oro`). `ptrtoint` of a 32-bit pointer to i64 zero-extends, so
   `(HANDLE)−1` reads as 2³² − 1, and `open`'s test `llvm.address(h) == -1` would take a failed
   `CreateFileA` for a handle. A pointer's width is the data model's, so the sentinel's integer is
   too.
2. **A file read fails at run time on i686**: `crc32` and `fnv`, built through the experiment's copy,
   answer *cannot read that file* for any file. The cause is not isolated: no file case of the suite
   reaches run time on the triple, and a probe printing the error code is refused (3). The
   candidates are the sentinel above and the `os` cell's calls.
3. **Not the triple's, found on x86_64**: a small new program over windows' `os` is refused where
   the corpus's programs are accepted. `case os.ReadFile("…") { … io.print_int(…) }` is refused at
   `ReadFile`'s `lo`: its upper end, `len(b) <= max_len`, is not instantiated, and the interval
   route's residual does not lower a `(0, #any)`. An edit of `crc32`'s error arm is refused at
   `put`'s byte, `0 <= b(i) <= 255`. Both are missing proofs that a program names, so they are owed
   under the analysis rule.

**The cost of the register pairs**, on a loop of 3·10⁸ steps whose values the decision proves
inside 32 bits (`if (i·3 + s) % 7 == 0 { s ← (s + i) % 1000003 }`), five runs each, quiet machine,
medians:

| | ms |
|---|---|
| C, i686, `int` (i32) | 330 |
| C, i686, `long long` (pairs) | 871 |
| C, x86_64, `long long` | 321 |
| ours, i686 (S₆₄ in pairs) | 782 |
| ours, x86_64 | 457 |

On i686 a 32-bit representation is **2.6× the speed** of the pairs (C's own two widths), and ours is
at C's 64-bit parity there. On this loop the x86_64 figure is 1.42× C's, a residue the gauntlet does
not show, owed to the standing measure.

## 4. The algebra of each question

### 4.1 The data model: C type names are sets the triple's layer defines

A declaration's sets are its prototype (ADR 0094 decision 1). A Win32 prototype written once, as
the header writes it (`SIZE_T`, `ULONG_PTR`, `LONG`), is therefore a **term with free set names**,
and a data model is an **interpretation** of those names: LLP64 sends `SIZE_T ↦ 0..2⁶⁴−1` (held as
0..2⁶³−1 where the unsigned word is absent, ADR 0094) and ILP32 sends `SIZE_T ↦ 0..2³²−1`. This is
exactly T_LLVM[Σ_OS] one level down: the declarations are a theory with a parameter, and the triple's
layer instantiates it. ADR 0073 already makes a set's name denote a set, and a target's manifest
set is its definition (`set DWORD = 0..2^32-1`, ADR 0112). So:

- **the names are the header's**, defined once per data model, and **W1 (Win32 by header)** is where
  they live: `windows/basetsd` defines `SIZE_T`, `ULONG_PTR` and `DWORD_PTR` per data model, and
  `windows/minwindef` defines `DWORD`, `BOOL`, `UINT` once, as they agree on both;
- **the triple's layer chooses the data model** (`model ilp32`, or a layer `windows/x86` glued after
  `windows`), and nothing else changes in a declaration.

ADR 0094's "Why not" rejected names *beside* sets ("a second claim that can disagree"). A name
*defined as* a set is not beside it: it is the set, as `DWORD` is now.

### 4.2 K(ℕ\*)'s base is a function of the triple

ADR 0106's theorem: the base is the largest B = 2^w whose B² − 1 the double word holds and whose
multiply-accumulate the decision proves, B = 2⁶⁴ with i128 on x86-64. The double word that is
**native** on i686 is i64 (`mul r/m32` gives EDX:EAX, `div r/m32` divides EDX:EAX), so by the same
theorem **B = 2³² on i686**, digits in 0..2³²−1 held in i32 or i64, each step's t in i64. LLVM's
inline i128 expansion makes B = 2⁶⁴ *correct* on i686, as a 4×4-limb schoolbook per digit product,
and roughly 3–4× the work per digit of B = 2³². That ratio is estimated: the suite did not reach a K(ℕ\*) program at run time on the triple (§3). So the
base is target data, `fact digit_bits`, and K(ℕ\*)'s library is written over it. **Corrected
2026-11-17**: this said ADR 0106 already wrote the library with B a parameter. It did not.
`emit/natnum.oro` writes B = 2⁶⁴ in 26 places: `t >> 64`, `dm = 2⁶⁴ − 1`, D's normalization at 2⁶³.
So D2 is a refactor of that library onto a digit width, each of its proofs re-derived.

### 4.3 `max_len` bounds bytes, not elements

`fact max_len (a) len(a) <= 2^47 - 9` is the address space divided by the smallest element, one
byte, less the table's header. A region of n elements of w bytes takes n·w + 8 + 1 bytes. On x64
the space is so large that the word's arithmetic never meets it. On i686 the space is 2³¹ bytes
(2³² with `/LARGEADDRESSAWARE` under WOW64), and a byte count of an n-element table of words may
pass the 32-bit `SIZE_T`. **The fact the address space gives is about bytes**: n·w + 9 ≤ A_t, with
A_t = 2³¹ (or 2³²). The length bound per table is ⌊(A_t − 9)/w⌋, a consequence. `region`'s request
b ≤ A_t is then a theorem at every allocation, where the experiment had to trap above 2³²−1.

### 4.4 `x64.oro` is the ISA's, so it splits by ISA

`x64.oro`'s operations (`divq` by inline assembly, `llvm.x86.addcarry.64`) are x86-64's. They are
not i686's, which has `divl` and `adc` on 32 bits (`llvm.x86.addcarry.32`). The module is
`llvm/isa`, an interpretation per ISA, so it is a layer the triple selects: `windows/x64.oro` on
x86_64 and `windows/x86.oro` on i686, each defining the same operation symbols at its own width.
The digit base (§4.2) chooses which ones K(ℕ\*) calls.

### 4.5 i32 scalars

ADR 0083 item 2's register rule, already met by `wc`, where the newline count in 64-bit lanes is
1.43× C's work. On i686 every S₆₄ operation is a register pair, so a value the decision proves in
−2³¹..2³¹−1 (or 0..2³²−1) held in an i32 is a representation chosen by place, (H, d) with H the i32
and d the embedding, and its operations are the word's on i32 wherever the decision proves the
result in i32's range. It is the same construction as the double word (ADR 0103) one width down,
so its soundness rule is that rule's: an operation is printed at i32 only where the decision
proves every intermediate in i32. Measured in §3 as the cost of pairs on the suite.

### 4.6 The 64-bit division helpers

`udiv i64` on i686 needs `__aulldiv` (§2 item 5). Three sources:

- **libcmt's helpers**, linked statically: the linker takes only the objects it needs, with no CRT
  initialisation since the entry is ours. A dependency on MSVC's library.
- **compiler-rt's builtins** for i386: not installed with the LLVM release here.
- **the language's own definition**: a quotient of two words on a 32-bit machine is Knuth's D with
  two 32-bit digits, the algorithm K(ℕ\*) already has (ADR 0099, natdiv). A realization of `/` on
  the triple (ADR 0089), a term over `divl`.

## 5. What it costs

Nothing is built, so nothing is paid yet. The experiment cost `experiments/i686/` (`run32.go`, a
copy of `run.go` with two edits; `mk32.py`; a README), two suite runs of about four minutes each at
`-j 4`, and this document. The build each decision implies, estimated:

- **D1, the data model**: the type names in their headers (W1), and kernel32's and the survey's
  declarations restated in them. The survey writes 10,479 prototypes and would need the headers'
  names, not the C ABI's sets alone, so most of D1's lines are W1's.
- **D2, the base**: one target fact, and K(ℕ\*)'s library rewritten over a digit width (not already
  parametric, corrected above); the i686 kernels
  (`adc` on 32 bits, `divl`).
- **D3, the bound on bytes**: one fact per triple, with `region`'s and `readfile`'s guards derived
  from it where the x64 literals are now.
- **D4, the ISA split**: `x64.oro` as is, and `x86.oro` beside it.
- **D5, i32 scalars**: a representation on the LLVM layer, with the double word's soundness rule
  one width down. The largest item, and the one the measurement says is worth 2.6× on i686 loops.
- **D6, the division helpers**: a realization of `/` and `%` on the word for i686, or a link line.

## 6. Decisions for hamza

Each is hamza's to make. Each recommendation follows the algebra, as the working rules ask.

1. **D1, the data model.** C's type names become sets defined per data model, in their headers
   (W1). `windows/basetsd` defines `SIZE_T`, `ULONG_PTR` and `DWORD_PTR` per model, and the names
   the two models agree on (`DWORD`, `BOOL`, `UINT`) are defined once. The triple's layer selects
   the model, so a declaration is written once and interpreted per triple (§4.1). **Recommended**:
   it is the parameterized theory the target already is, one level down. **The alternative**, a copy
   of each declaration per triple, is the "generator per triple" ADR 0094 rejected.
2. **D2, the digit base.** `fact digit_bits` per triple: 64 on x86_64, 32 on i686, by ADR 0106's
   theorem applied to the native double word (§4.2). **Recommended.** **The alternative** is B = 2⁶⁴
   on i686 through LLVM's i128 expansion. It is correct, but each digit product is a schoolbook of
   4×4 limbs, and it needs `adc`/`sbb` without the x86-64 intrinsics.
3. **D3, the address space bounds bytes.** `fact max_bytes`, n·w + 9 ≤ A_t, with A_t = 2⁴⁷ on
   x86_64 and 2³¹ on i686, the length bound a consequence per element width (§4.3).
   **Recommended**: 2³¹, not 2³² with `/LARGEADDRESSAWARE`. It holds on every 32-bit Windows, and
   the larger space is a link flag a program could ask for later.
4. **D4, the ISA's operations split by ISA**: `windows/x64.oro` and `windows/x86.oro`, each defining
   the same symbols at its own width, and the triple's layer glues one (§4.4). **Recommended**: it
   is what `x64.oro`'s own header says ("32-bit Windows is another member, with its own").
5. **D5, i32 scalars.** A representation of the word's values the decision proves inside 32 bits,
   on the LLVM layer for both triples, printed at i32 only where every intermediate is proven there
   (§4.5). Measured: 2.6× on i686 loops, and `wc`'s 1.43× on x86_64 (files-2026-11-16).
   **Recommended**, built after D1–D4, so that the suite runs on the triple first.
6. **D6, `/` and `%` of the word on i686** (§4.6), from one of three sources:
   - (a) libcmt's helpers, on the link line;
   - (b) a realization in the language: Knuth's D on two 32-bit digits over `divl`, as K(ℕ\*)
     divides;
   - (c) compiler-rt's builtins, which are not installed.

   **Recommended: (b)**. The target links no C runtime (llvm.md §2), and the algorithm is already
   proven in the language. (a) is the measured fallback if (b) is slower than MSVC's helper.
7. **D7, the target's name**: `windows32`, a target that glues `windows` and selects the triple,
   model, ISA and toolchain, beside `windows`. **Recommended**: a triple is target data (ADR 0093
   item 5), and two triples are two models, so two targets. **The alternative**, a `-triple` flag
   on one target, makes the build a function of a flag the target does not state.

**Derived, not decided**:
- the build line names the triple (`--target`) and its toolchain's environment (`vcvars32`), since
  clang's default target overrides a module's own (§2 item 1);
- `llvm.address`'s integer is the pointer's width, so a sentinel is compared at that width (§3,
  found 1).

Both follow from the triple being data, and are built with D7.
