# llvmsurvey-2026-10-27: Σ_LLVM, every primitive classified by what the language can state

The 10-24 plan's item 6b, its spec-first step. hamza, 2026-10-27: "because llvm ir is just another
target, we will try to support every primitive it has, when you hit a wall or something that can't be
expressed let me know, so we can decide it together" ([ADR 0080](../../docs/decisions/0080-the-native-targets-print-llvm-ir.md)
item 7). This document finds the walls, all at once, before anything is built. Nothing was built for any of them.

Reproduce: `python -I experiments/llvmsurvey/survey.py` against the installed LLVM (23.1.4, the official
release); its output is [survey.txt](../../experiments/llvmsurvey/survey.txt). The semantics quoted are
LangRef's (llvm.org/docs/LangRef.html, read 2026-10-27).

## 1. What is being classified

LLVM IR is a many-sorted signature Σ_LLVM. Its sorts are the first-class types: integers iN (N from 1
to 2²³), six float formats, `ptr` in each address space, vectors `<N x T>` and `<vscale x N x T>`,
structs and arrays, `token`, `metadata` and labels. Its operations are 67 instructions (`Instruction.def`,
less the two pass-internal `UserOp`s) and 16,423 intrinsics (`Intrinsics.td`, dumped by `llvm-tblgen`):
534 target-independent and 15,889 belonging to 19 architectures.

The language's computation has the sorts Int (ℤ), F64, Bool, strings, tables `Array(V)`, maps, products,
variants and host types. Each target realizes a set of Int, its word, as data (`repr`).

**The criterion.** Let s_N, u_N : BV_N → ℤ be the signed and unsigned decodings of an N-bit vector, with
images W_s = [−2^(N−1), 2^(N−1)) and W_u = [0, 2^N). An LLVM operation f is **stateable now** when f
is the image of a function g on the language's sorts on a domain D:

  s ∘ f = g ∘ s^k  on D,  and D is exactly where f is defined (no poison, no undefined behaviour).

D then becomes the declaration's `where`, an obligation the IR discharges or refuses (ADR 0028, 0075).
LLVM's poison flags are exactly such domains: `add nsw` is ℤ's + on D = {(a, b) : a + b ∈ W_s}; `sdiv`
is the language's `/` (truncation, integers.md §3) on D = {b ≠ 0, (a, b) ≠ (−2⁶³, −1)}; `udiv` is the
same on a, b ≥ 0. An operation that is no such image is a **wall**: the language would need a sort, a
form or an effect it lacks.

## 2. The count

**Instructions (67):**

| verdict | n | instructions |
|---|---|---|
| stateable | 26 | `fneg fadd fsub fmul fdiv frem`; `add sub mul shl` with `nsw`; `sdiv srem ashr and or xor`; `alloca load store getelementptr` (the printing of tables); `fptosi sitofp icmp fcmp select freeze` |
| stateable on ↑0 | 5 | `udiv urem lshr fptoui uitofp` |
| structure | 7 | `ret br` (both), `switch unreachable phi call`: the printer's own |
| exceptions | 8 | `invoke resume landingpad cleanuppad catchpad cleanupret catchret catchswitch` |
| pointers as values | 4 | `ptrtoint ptrtoaddr inttoptr addrspacecast` |
| atomics | 3 | `fence cmpxchg atomicrmw` |
| widths | 3 | `trunc zext sext` |
| vectors | 3 | `extractelement insertelement shufflevector` |
| unstructured | 2 | `indirectbr callbr` |
| float formats | 2 | `fptrunc fpext` |
| records | 2 | `extractvalue insertvalue` (a literal struct is a tuple; a laid-out struct is 6d's records) |
| bit-vectors | 1 | `bitcast` (an f64's bit pattern) |
| frame | 1 | `va_arg` |

The 26 count each instruction once. Without its flags, `add`, `sub`, `mul` and `shl` are ℤ/2ⁿ (§4.1).

**Target-independent intrinsics (534):**

| verdict | n | |
|---|---|---|
| vectors | 158 | `vector.*`, `vp.*`, `masked.*`, `matrix.*`, `vscale`, `stepvector` |
| **stateable** | **72** | below |
| the printer's or the build's | 57 | `assume`, `expect`, `lifetime.*`, `invariant.*`, `stacksave`/`stackrestore`, `dbg.*`, `instrprof.*`, sanitizers, CFI |
| float environment | 57 | 49 `experimental.constrained.*`, `get`/`set.rounding`, `fpenv`, `fpmode` |
| another runtime's | 54 | GC statepoints, deoptimization, patchpoints, 32 `objc.*`, 8 `ptrauth.*` |
| coroutines | 38 | `coro.*` |
| the frame | 25 | frame and return addresses, `localescape`, registers, trampolines, `va_*`, `call.preallocated.*` |
| exceptions | 18 | `eh.*`, `seh.*` |
| another target's, or the backend's | 15 | GPU convergence, SPIR-V's `structured.*`, BPF's `preserve.*`, hardware loops |
| **stateable on ↑0** | **11** | `ctlz ctpop umax umin ucmp uadd/usub/umul.with.overflow uadd/usub/ushl.sat` |
| relations | 10 | `fmuladd powi`, the eight fixed-point `[su](mul|div).fix(.sat)` |
| bit-vectors | 7 | `bswap bitreverse fshl fshr clmul pdep pext` |
| atomics, thread-locals | 5 | `mem*.element.unordered.atomic`, `threadlocal.address`, `thread.pointer` |
| float formats | 3 | `fptrunc.round`, `convert.to/from.arbitrary.fp` |
| pointers as values | 3 | `objectsize load.relative ptrmask` |
| unstructured | 1 | `callbr.landingpad` |

The 72: `abs smax smin scmp cttz`; `sadd/ssub/smul.with.overflow`; `sadd/ssub/sshl.sat`; 47 on F64
(`sqrt fabs copysign floor ceil trunc rint nearbyint round roundeven fma minnum maxnum minimum maximum
minimumnum maximumnum sin cos tan asin acos atan atan2 sinh cosh tanh exp exp2 exp10 log log2 log10 pow
ldexp frexp modf sincos sincospi lrint llrint lround llround is.fpclass fptosi.sat fptoui.sat
canonicalize`); `memcpy memmove memset` (and `.inline`, `memset.pattern`) over buffers; `prefetch
readcyclecounter readsteadycounter`; `trap debugtrap ubsantrap looptrap cond.loop`. Their signatures need
only the word, F64 and Bool, once an immediate argument (a constant, erased by staging) is set aside, except
the six memory ones and `prefetch`, whose pointer is a buffer's.

**Every intrinsic, by what its signature needs** (mechanical; an overloaded type counts as its cheapest
instance):

| needs | n | share |
|---|---|---|
| a vector | 10,101 | 61.5% |
| i8, i16 or i32 | 2,129 | 13.0% |
| only the word, F64, Bool | 1,583 | 9.6% |
| a pointer | 1,225 | 7.5% |
| f16, bf16 or f32 | 1,033 | 6.3% |
| a target's special type (x86_amx, x86_mmx, svcount, wasm references) | 189 | 1.2% |
| a token or metadata | 127 | 0.8% |
| i128 | 21 | 0.1% |
| f128 or ppc_fp128 | 15 | 0.1% |

x86 has 1,685, of which 1,392 need vectors. Its 53 that need only the word are a list, not an
algebra: `rdtsc` and `pause` (impure, stateable); `bmi.bextr`, `bmi.bzhi` and `tbm.bextri`
(⌊x/2^s⌋ mod 2^l, stateable on ↑0); `sse42.crc32.64.64` (𝔽₂[x] arithmetic, §4.1);
`avx512.sitofp.round`, `uitofp.round` and `vfmadd.f64`, which take a rounding mode encoded in the
instruction (§4.5); fences and TSX's `xend`/`xabort` (atomics); AMX's tile operations (state in tile
registers); and system instructions a user-mode program never issues (`wrmsr`, `wbinvd`, the
FS and GS bases, CET's shadow stack, user interrupts).

## 3. What is offered now

The criterion's derivations, one per family:

- **`and`, `or`, `xor` are total on ℤ.** Read an integer as its infinite two's-complement string (the
  2-adic integers restricted to ℤ); bits above N − 1 of a word value equal its sign bit, and ∧, ∨, ⊕
  preserve that, so W_s is closed and the operation is ℤ's. `or disjoint` is `+` on D = {a ∧ b = 0}.
- **Shifts.** `ashr a, b` is ⌊a / 2^b⌋ on D = {0 ≤ b < N}, and `exact` adds 2^b | a; `shl nsw a, b` is
  a · 2^b on D = {0 ≤ b < N, a · 2^b ∈ W_s}; `lshr` is ⌊u(a) / 2^b⌋, ℤ's only for a ≥ 0.
- **Counting bits.** `cttz` is ν₂, the 2-adic valuation, on ℤ ∖ {0}, independent of N (two's complement
  preserves trailing zeros, ν₂(−a) = ν₂(a)). `ctpop` is ℕ's popcount on ↑0 only: a negative integer has
  infinitely many ones. `ctlz` is N − bitlen(a) on ↑0.
- **Overflow and saturation.** `sadd.with.overflow` is the partial + on W totalized: a variant
  `'ok(a + b) ∪ 'overflow`, dropping the wrapped value the overflow case carries (which is ℤ/2ⁿ).
  `sadd.sat` is clamp_W ∘ +. `usub.sat` is ℕ's monus a ∸ b.
- **Floats.** `fadd`…`frem` and the 47 F64 intrinsics are F64's own operations; their libm-backed members
  (`sin`, `pow`) are the host's, Tier 2 as `math.Sin` is on Go. `minnum`/`maxnum` are functions in LLVM
  23: "-0.0 is considered to be less than +0.0 for this intrinsic", except with `nsz` or a signalling NaN.
  `fptosi` is truncation on D = {trunc x ∈ W}; `fptosi.sat` is total.
- **`freeze`** is the identity on every value the language produces: no operation the printer emits is
  poison, because every domain D was proven.
- **Memory.** `alloca`, `load`, `store`, `getelementptr inbounds` print tables, as `ir/x86` prints them
  today; `memcpy`/`memset` are a segment copied into a buffer and a buffer's rule of a constant.

Also offered are the flags that state the IR's proofs (ADR 0080 item 2), each a domain D: `nsw`, `nuw`,
`exact`, `disjoint`, `nneg` ("if the zext argument is negative, the result is a poison value"),
`samesign`, `trunc nuw`/`nsw`, `inbounds`. So is the `range(<ty> a, b)` attribute ("If the value is not
in the specified range, it is converted to poison"), which states an interval the IR proved exactly as
the decision has it.

## 4. The walls

Each wall below is a sort, a form or an effect the language lacks. Each has its algebra and a
recommendation, for hamza.

### 4.1 Bit-vectors: ℤ/2ⁿ and 𝔽₂ⁿ

**What.** `add`, `sub`, `mul` and `shl` without `nsw`; `lshr`, `udiv` and the unsigned compares on a
negative operand; `trunc` without flags; `bitcast`; `bswap bitreverse fshl fshr clmul pdep pext`; the
wrapped value of `with.overflow`; x86's `crc32`.

**The algebra.** LLVM's iN is one carrier, BV_N, with two structures: the ring ℤ/2^N (+, ·) and the
Boolean ring 𝔽₂^N (⊕, ∧), with shifts and rotations between them. This is SMT-LIB's QF_BV. The language's Int
is ℤ, and s and u relate the two. A wrapping `add` is a total function on W_s, so it could be declared today
as a host function over `-2^63..2^63-1`. But its laws are ℤ/2^N's, not ℤ's: the analysis would hold an
Int whose arithmetic does not obey ℤ's laws, a misfit in fit-research's sense. `clmul` is
multiplication in 𝔽₂[x] mod x^N, a third ring (CRC is its quotient by the CRC's polynomial).

**Options.** (a) A sort `Bits(N)` = BV_N: ℤ/2^N's ring and 𝔽₂^N's Boolean ring on one carrier, entered
by `bits(N, x)` = x mod 2^N and left by s and u. This is the "representation as an instance of a type's
signature" CLAUDE.md holds for "ℤ/2ⁿ for hash/fnv"; LLVM, Win32's flag words and CRC are the same
algebra. Its linear fragment stays decidable by the procedures the language owns: u(a + b) = u(a) + u(b) − 2^N·c
with c ∈ {0, 1} is Presburger. (b) Host functions over the word: cheap, and a misfit.

**Recommended: (a)**, designed now, built when a program needs one (`hash/fnv`, `hash/crc32`).

**Revised the same day** by [bits-research.md](../../docs/bits-research.md). The derivation there finds
that no sort is needed: π_n(x) = x mod 2ⁿ is at once a ring and a Boolean homomorphism, so wrapping is
π_n applied to ℤ's own operations, computed in n-bit arithmetic by a fusion theorem. (a), the sort, is
its candidate C and not the recommendation.

### 4.2 Widths: i1 to i2²³

**What.** `trunc`, `zext`, `sext`; 2,129 intrinsics over i8/i16/i32; every Win32 `DWORD`, `WORD` and
`BYTE`; and i128 and wider.

**The algebra.** This is not a gap in what the language states. A range already says which widths can
hold a value: S ⊆ W_s(N) or S ⊆ W_u(N). Under ADR 0079 the facts choose ρ among the word, the unsigned
word and the rung above. LLVM's family {iN} makes ρ's codomain a chain ordered by inclusion, and the
least member containing the proven fact is the representation. `trunc nuw`, `zext nneg` and `sext` are
then the identity on ℤ between two representations, inserted by the printer and never written. Two
consequences:

- At a host boundary the width is the declaration's: `DWORD` is `host "i32"` over 0..2^32-1 (ADR 0075's
  bounds).
- **Above the word, LLVM provides iN natively** (division past 128 bits is expanded by LLVM's own
  pass), so on LLVM targets the limbs library (`repr big limbs`, ADR 0035) is lowering further than the
  target requires. A bounded set above the word prints as i(bitlen + 1).

**Recommended:** an ADR amending 0079. ρ ranges over every width; the narrowest is chosen where it is
observable or measurably cheaper (a host's type, a record's layout, a table's density); the rung above
is native iN on LLVM, measured against the limbs before the limbs leave that target.

### 4.3 Float formats

**What.** `fptrunc`, `fpext`, `fptrunc.round`; 1,033 intrinsics over f16, bf16 or f32; 15 over f128
or ppc_fp128; x86_fp80.

**The algebra.** binary32's values are a subset of binary64's, and so are binary16's and bfloat16's.
For ∘ ∈ {+, −, ×, ÷, √}, rounding the exact result to binary32 equals rounding it first to binary64 and
then to binary32, because 53 ≥ 2·24 + 2. Double rounding is innocuous when p' ≥ 2p + 2 (Figueroa,
"When is double rounding innocuous?", 1995). binary16 needs 24 ≤ 53, bfloat16 18 ≤ 53. **So F32 is a set
of F64, round₃₂(F64), and its operations are F64's followed by round₃₂**: no new sort, a set and a
function. `fma` and the libm functions are not covered (fused rounding, and the host's accuracy), and are
declared per format. f128, x86_fp80 and ppc_fp128 are supersets of F64 (ppc_fp128 is double-double,
not IEEE), and no target of ours has them: MSVC's `long double` is binary64.

**Recommended:** F32, F16 and BF16 as subsets of F64 with their rounding functions, ρ printing `float`
where the fact allows; the wider three not offered until a program asks.

### 4.4 Relations

**What.** Ten intrinsics specify a set of results rather than one. For `fmuladd`, "Fusion is not
guaranteed". For `powi`, "The order of evaluation of multiplications is not defined". For the eight
fixed-point operations, "The rounding direction is unspecified". To these add the fast-math flags
`nsz arcp contract afn reassoc`, and the NaN rule: a NaN result "has a non-deterministic sign" and
payload, observable through `bitcast`.

**The algebra.** Each is a relation R ⊆ V × V, not a function. The computation is functional, and
staging never changes an answer (ADR 0009); a program's output would become a member of a set chosen
by the backend's build. Each relation has a deterministic refinement the language already states: `fma`
(fused) or `fmul` then `fadd`; ⌊a·b / 2^s⌋ in ℤ; a loop of multiplications in the written order.

**Options.** (a) Offer only the refinements; the relations are not offered. (b) Offer a relation as
Tier 2, its declared result the hull of R(x), reported like portability. **Recommended: (a)**, and a
`bitcast` of an F64 to its bits is guarded `where x is not NaN` (or Tier 2).

### 4.5 The float environment

**What.** The 49 constrained intrinsics; `get`/`set.rounding`, `fpenv`, `fpmode`; AVX-512's rounding
encoded in the instruction.

**The algebra.** The rounded operations ∘_r, r ∈ {nearest-even, down, up, toward zero, nearest-away},
are five pure functions per operation, and interval arithmetic needs them. LLVM does not provide them as
functions. The constrained intrinsic's argument is an assumption about state ("Using a specific
non-dynamic rounding mode which does not match the actual rounding mode at runtime results in undefined
behavior"). The mode is set by `set.rounding`, and every function containing one must be `strictfp`
throughout. The status flags are a second piece of state that every operation writes.

**Options.** (a) ∘_r offered as functions of a static r (erased by staging): the printer brackets them
with `set.rounding` inside a `strictfp` function. The dynamic mode and the flags are not offered, since
they would make every float operation impure. (b) Nothing until a program asks. **Recommended: (a) as
the design, (b) as the schedule.** No program asks.

### 4.6 Pointers as values

**What.** `ptrtoint`, `inttoptr`, `ptrtoaddr`, `addrspacecast`, `ptrmask`, `objectsize`, `load.relative`.

**The algebra.** An LLVM pointer is a provenance (an allocation) and an address. A language table is a
function on [0, len), and every pointer the printer needs is a table and an offset, a segment
(ADR 0074): `getelementptr inbounds` is b[i:] and `load` is application. `objectsize` is `len`. An
address as an integer forgets provenance, and nothing in the language's semantics denotes it. A Win32
`HANDLE` or `HWND` is opaque: a host type with equality and nothing else, which the language has
(ADR 0045). An out-parameter is ADR 0048's reference to a fresh cell.

**Recommended:** never an address as a value; an opaque host pointer is a host type; every other pointer
is a segment. The four casts are not offered.

*Researched since: [pointers-research.md](../../docs/pointers-research.md), whose Theorem P derives this
recommendation and finds the one wall it leaves, a pointer the host keeps past the call.*

### 4.7 Vectors

**What.** 158 target-independent intrinsics, 10,101 in all (61.5% of Σ_LLVM), and `extractelement`,
`insertelement` and `shufflevector`.

**The algebra.** `<N x T>` is T^N: a function on [0, N), a table of static length (data.md). Its
lane-wise operations are T's lifted pointwise, the direct power of T's algebra, which satisfies every
equation T does (Birkhoff). Its reductions are folds of a monoid over [0, N). A shuffle is precomposition
with an index map σ : [0, M) → [0, 2N). A mask is a restriction to a subset, and `vp`'s explicit vector
length a segment [0, evl). A scalable vector has length vscale·N, fixed per machine and known at run time.
**So a vector is a representation of a table, not a new sort**: `array { a[k] + b[k] : k ∈ 0..3 }` is
`add <4 x i64>`, and ρ chooses the register form for a static-length table whose operations are
pointwise.

**Options.** (a) A representation, ρ's, with a target intrinsic over vectors declared over `Array(T)`
with `where len(a) = N`, so `aesenc` takes two tables of two words. (b) A host type `Vec(N, T)`.
**Recommended: (a)**, the fit; (b) is a second form of one function.

### 4.8 Records

`extractvalue`/`insertvalue` on a literal struct are tuple projection (products, ADR 0031, stateable). A
laid-out struct is 6d's wall, already planned.

## 5. Decided, or another plan's

- **Atomics, fences, thread-locals and coroutines** belong to concurrency (ADR 0081). Isolation is a
  typing property, so an atomic is the scheduler's lowering of a mailbox, never a program's primitive.
  LLVM's switched-resume coroutines are a candidate lowering of a process's step function, measured
  against ours (M2–M6) when processes are built.
- **Exceptions**: ADR 0040. A failure is a value, the language is `nounwind`, and a throwing host's
  exception becomes a value at its boundary.
- **Unstructured control**, `indirectbr` and `callbr`: never (CLAUDE.md).
- **The frame** (return addresses, `localescape`, registers, trampolines, defining a C variadic): not a
  value of the language. A trampoline is a closure surviving staging, which the language forbids.
- **Another runtime's** (GC statepoints, deoptimization, Objective-C ARC, arm64e pointer authentication)
  and **another target's** (GPU convergence, SPIR-V, BPF, hardware loops): not offered.
- **The printer's vocabulary**: `assume` states a proof no flag carries, `invariant.start` a frozen
  buffer (ADR 0031), `noalias.scope.decl` R0–R4's disjointness, `lifetime.*` and `stacksave` a buffer's
  scope. **The build's**: debug info, profiling, sanitizers, CFI.

## 6. What failed first, what it cost, what was not done

- **The classification fails on a name it does not know.** Every rule set is closed by an assertion. It
  fired twice in development: on `UserOp1`/`UserOp2`, which LangRef does not list, and on
  `ptr.annotation`, a regular expression missing a dot.
- **`ImmArg`'s index was misread** as counting results first. It is LLVM's attribute index (0 the return,
  parameters from 1). `ubsantrap`, still classed as needing i8 after the first fix, showed it.
- **Memory was wrong twice**, and LangRef corrected both: `minnum`/`maxnum` are functions without `nsz`
  in LLVM 23, and a constrained intrinsic's rounding mode is an assumption, not a request.
- **Cost**: one script of 226 lines, `llvm-tblgen`'s 24.7 MB dump, and about two hours against a three-hour
  limit.
- **Not done**: the per-architecture intrinsics are classified by signature only, not by meaning. 15,889
  names need the vector decision (§4.7) before a reading by meaning is worth it. No declaration was
  written and no `ir/llvm` code exists; both wait on §4's decisions.
