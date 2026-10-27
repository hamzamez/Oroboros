# Research: widths, or how an integer is held

2026-10-27. **Research, not a decision.** The second of the seven walls
[llvmsurvey-2026-10-27](../gauntlet/results/llvmsurvey-2026-10-27.md) found (§4.2), and the plan's 6b,
step 2. hamza: "research on wall 2, explain the issue, the solutions, what does it mean in different
targets, the type system, and plenty of examples. and recommendations."
[ADR 0082](decisions/0082-wrapping-is-the-residue-map-and-the-bits-are-the-integers.md) (wrapping and the
bits) is built together with this step, so the two documents share their algebra.

**The short version:**
- **A width is never part of what a value is.** An `Int` is an integer; 200 is 200 in a byte, in a
  `long` or in a 128-bit register. A width is how a target **holds** the integer, and the language
  already chooses that from what it proves (ADR 0079).
- **The algebra**: a representation is a pair (H, d) of a host type H and an injective **abstraction
  function** d : H → ℤ (Hoare, "Proof of correctness of data representations", 1972). A set S can be held
  in it when S ⊆ d(H). Because every conversion between representations is the identity on ℤ, **no
  choice of representation can change an answer**; the choice is cost alone.
- **What is new is how many representations there are.** LLVM has iN for every N. The JVM's `byte` can
  hold 0..255 under a second abstraction function. JavaScript has typed arrays. Win32 dictates exact
  widths at its boundary and in its structs. The language's four integer mechanisms (word, unsigned word,
  limbs, host bignum) and its narrow table elements are already five instances of (H, d) built
  separately.
- **A width is observable in exactly four places**: a table's memory, a host's signature, a record's
  layout, and above the word. In a register it is invisible, and costs at most an extension.
- **Recommended** (§8): one mechanism, the representation as an abstraction function, with ρ choosing
  by **place**: forced by the host at a boundary or in a record; the smallest that holds the element set
  in a table; the target's register width for a scalar; native iN above the word on LLVM. This fulfils
  CLAUDE.md's waiting item "a representation as an instance of a type's signature", whose trigger this
  wall meets.

---

## 1. The issue

### 1.1 What the language says today

- **An `Int` is an integer** (ADR 0026), and a range is a set, which is a **claim** (ADR 0079). The
  representation of each value is ρ of its proven fact: the word S = [−2⁶³, 2⁶³), the unsigned word
  U = [0, 2⁶⁴) on Go (ADR 0033), limbs for a finite set above both (ADR 0035), or the host's bignum
  (ADR 0034).
- **A table's elements are held narrow** (elemwidth-2026-08-27). Each target lists `repr` lines,
  narrowest first, and a table's declared element set picks the first that contains it:

  ```
  // targets/go/go.oro                    // targets/java/java.oro
  repr 0..255 host "byte"                 repr -128..127 host "byte"
  repr -128..127 host "int8"              repr -32768..32767 host "short"
  repr 0..65535 host "uint16"             repr -2^31..2^31-1 host "int"
  …                                       repr -2^63..2^63-1 word
  ```

  "A table is the only place the width is consulted — a value read out of a byte array is an integer
  everywhere it is used" (go.oro).
- **A JVM loop counter is held in an `int`** when the interval analysis proves it fits, because a Java
  array index is 32-bit (indextype-2026-08-25, indexnarrow-2026-08-27: the sieve went from 1.16× to 0.99×
  of hand-written Java).

### 1.2 What LLVM, Win32 and the hosts ask that it cannot answer

1. **LLVM has a type per width, and Win32 is typed in them.** `GetLastError` returns a `DWORD`, an
   `i32`; `MessageBoxW` returns an `int`; `SendMessageW` takes a `WPARAM` and an `LPARAM`, 64 bits on
   x64 and 32 on i686. The windows target today holds everything in a qword, which MASM tolerated and
   LLVM will not: a `declare` states each parameter's type.
2. **Records have layouts.** `RECT` is four 32-bit `LONG`s, 16 bytes; `MSG` is 48 bytes on x64 and 28 on
   i686 (§3.3). A field's width is the host's, exactly.
3. **The JVM's 0..255 costs a copy of every file.** Because `byte` is signed, `Array(0..255)` is a
   `short[]`, so `lib/os/java.oro`'s `ReadFile` copies `Files.readAllBytes`' `byte[]` into a new `short[]`
   element by element, and `WriteFile` copies it back. A file's bytes take twice the memory and two passes,
   though a `byte[]` holds exactly the same information (§2.3).
4. **JavaScript declares no representations**, so a table of 0..255 the program builds is a JS `Array`,
   4 to 8 bytes per element, where a `Uint8Array` is 1.
5. **Above the word, LLVM provides iN natively**: `mul i128` is one instruction pair on x86-64. The
   windows target's rung above is the limbs library (`repr big limbs`), which on LLVM would be lowering
   further than the target requires.
6. **TinyRetroPad is 32-bit.** On i686 the pointer-sized types are 32 bits, and 64-bit division is a
   library call. What is the word there?
7. **Five mechanisms where one law would do.** Narrow table elements, the word, the unsigned word,
   limbs and the host bignum are each built separately (`emit/target.go`'s `reprFor`, `ir/words.go`,
   `ir/limbs.go`, `ir/big.go`), each with its own conversion rules. Each new width would be a sixth.

---

## 2. The algebra

### 2.1 A representation is an abstraction function

A target's integer representations are pairs **r = (H, d)**: H is a host type, a finite set of host
values, and d : H → ℤ is injective. d is Hoare's abstraction function (1972), and its image I(r) = d(H)
is the set of integers r can hold.

| r | H | d | I(r) |
|---|---|---|---|
| Go `byte` | 8 bits | unsigned reading σ_u | [0, 2⁸) |
| Go `int8` | 8 bits | signed reading σ_s | [−2⁷, 2⁷) |
| Java `byte`, as Java reads it | 8 bits | σ_s | [−2⁷, 2⁷) |
| **Java `byte`, read `b & 0xFF`** | 8 bits | **σ_u** | **[0, 2⁸)** |
| Java `char` | 16 bits | σ_u | [0, 2¹⁶) |
| Java `long` holding a residue | 64 bits | σ_u (u64repr-2026-09-22) | U = [0, 2⁶⁴) |
| JS Number | IEEE double | the integer it is, on integers | [−(2⁵³−1), 2⁵³−1] |
| JS split pair (hi, lo) | two Numbers | hi·2³² + lo | U |
| JS `Uint8Array` element | 8 bits | σ_u | [0, 2⁸) |
| LLVM `iN`, read signed | N bits | σ_s | [−2ᴺ⁻¹, 2ᴺ⁻¹) |
| LLVM `iN`, read unsigned | N bits | σ_u | [0, 2ᴺ) |
| limbs, k words | (ℤ/2⁶⁴)ᵏ | Σ lᵢ·2⁶⁴ⁱ | [0, 2⁶⁴ᵏ) |
| Go `*big.Int`, Java `BigInteger`, JS `BigInt` | the host's | its value | ℤ (up to memory) |

**A set S can be held in r when S ⊆ I(r).** That is all a representation needs from the logic, and it
is the fact the IR already proves for every value.

### 2.2 Operations: the commuting square

A host operation f_H **computes** ℤ's operation f on r when

  d(f_H(x, y)) = f(d x, d y)  whenever f(d x, d y) ∈ I(r)

the simulation, or commuting-square, condition of data refinement (Hoare 1972; de Roever and Engelhardt,
*Data Refinement*, 1998). For the N-bit representations, ADR 0082 already decided which operations
satisfy it for both readings at once:

| ℤ's operation | on (iN, σ_s) | on (iN, σ_u) | why |
|---|---|---|---|
| + − · & \| ⊕ ~ << | one instruction for both | | they descend to ℤ/2ᴺ (bits-research.md §1.3) |
| / % | `sdiv` `srem` | `udiv` `urem` | they read a residue |
| < ≤ | `icmp slt` | `icmp ult` | |
| >> | `ashr` | `lshr` | |
| the condition "result ∈ I(r)" | `nsw` | `nuw` | LLVM's flags *are* this condition |

So a width brings no new arithmetic. It brings a choice of d, and the instruction follows from d.

### 2.3 Conversions are the identity, so a choice never changes an answer

Moving a value from r to r′ is d_{r′}⁻¹ ∘ d_r on I(r) ∩ I(r′): **the identity on ℤ**, realized by a host
operation. The printer inserts it; the program never writes it.

| from → to | on the integer | the host writes |
|---|---|---|
| Go `byte` → `int` | identity | `int(b)` |
| Java `byte` (σ_u) → `long` | identity | `(long) (b & 0xFF)` |
| Java `long` → `int`, for a value in [−2³¹, 2³¹) | identity | `(int) x` |
| LLVM i8 (σ_u) → i64 | identity | `zext i8 %b to i64` |
| LLVM i32 (σ_s) → i64 | identity | `sext i32 %x to i64` |
| LLVM i64 → i32, for a value in [−2³¹, 2³¹) | identity | `trunc nsw i64 %x to i32` |
| JS Number → `Uint8Array` element, for 0..255 | identity | the store itself |

> **Theorem (representation independence).** If every value v of a program is held in a representation
> r_v with fact(v) ⊆ I(r_v), every operation is computed by a host operation satisfying §2.2's square on
> its operands' representations with its result's fact inside its result's image (the overflow
> obligation every operation already carries), and every move between representations is §2.3's
> identity, then the program computes the same integers whatever representations are chosen.
>
> *Proof sketch*: by induction on the evaluation, d of every host value equals the value ℤ computes,
> using the square at each operation and d_{r′}⁻¹ ∘ d_r = id at each move. ∎

This is ADR 0009's "staging never changes an answer" for representations. Its consequence for design:
**correctness comes from facts, and choice comes from cost.** ρ may choose differently by target, by
place, even by measurement, and no program's output can differ.

### 2.4 The order on representations, and why "narrowest" is not one rule

Images are ordered by inclusion: S₈ ⊂ S₁₆ ⊂ S₃₂ ⊂ S₆₄, U₈ ⊂ U₁₆ ⊂ ⋯, and U_N ⊂ S_{N+1}. That is a
partial order, not a chain: U₈ = [0, 256) and S₈ = [−128, 128) are incomparable. For a set S, the
representations that can hold it form an up-set, and **its minimal elements can be several**:

| S | the minimal representations holding it |
|---|---|
| 0..255 | U₈ (Go `byte`, Java `byte` read σ_u, `Uint8Array`), and S₁₆ (Java `short`) |
| −1..255 (`InputStream.read`) | S₁₆ only |
| 0..2³²−1 | U₃₂, and S₃₃, which no host has, so S₆₄ |
| −2⁶³..2⁶⁴−1 | none of 64 bits: S₆₅, so i128 on LLVM, the rung above elsewhere |

So "the narrowest" is ambiguous, and what breaks the tie is cost. Cost depends on **where** the value is:
a byte in a table costs one byte, a byte in a register costs a whole register and an extension. §3 is
those places.

### 2.5 The existing mechanisms, re-read

| mechanism | as (H, d) | built in |
|---|---|---|
| the word | (int64, σ_s) on Go, the JVM, LLVM; (double, id) on JS | ADR 0026 |
| the unsigned word | (uint64, id) on Go; (long, σ_u) on the JVM, measured | ADR 0033, u64repr |
| narrow table elements | (byte, σ_u), (short, σ_s), … | elemwidth |
| the JVM's `int` index | (int, σ_s) | indextype, indexnarrow |
| limbs | ((ℤ/2⁶⁴)ᵏ, Σ lᵢ·2⁶⁴ⁱ) | ADR 0035 |
| the host bignum | (big.Int, value) | ADR 0034 |

Six built mechanisms, one definition. CLAUDE.md's waiting list holds exactly this: "A representation as
an instance of a type's signature (data refinement, Hoare 1972; Haftmann et al. 2013), replacing
integers' four mechanisms. Trigger: a fifth integer representation". The widths of LLVM, Java's σ_u
`byte`, JavaScript's typed arrays and JavaScript's split pair are four more.

---

## 3. The four places a width is observable

### 3.1 Tables: memory

A table of n elements in r costs n·size(r) bytes. Smaller is denser, and on a memory-bound loop the
memory traffic is the cost (bitwise.md §3.6, from the stencil measurement).

**A file's bytes, `Array(0..255)`:**

| target | today | with σ_u on a signed host byte |
|---|---|---|
| Go | `[]byte`, 1 byte each | unchanged |
| Java | `short[]`, 2 bytes each, **copied** from `byte[]` on read and back on write | `byte[]`, 1 byte each, no copy; each read is `b[i] & 0xFF` |
| JavaScript | the host's `Buffer` (a `Uint8Array`) when `ReadFile` returns it; a JS `Array` when the program builds one | `Uint8Array` either way |
| LLVM | (not built) | `[n x i8]`, each read a `zext` |

```
// Count each byte value in a file: a table of 256 counters.
fn histogram(data ∈ Array(0..255)) (Array(0..2^32-1)) where len(data) < 2^32 {
    var h := array { 0 : _ ∈ 0..255 }
    loop i = 0 {
        i < len(data) -> { h[data[i]] := h[data[i]] + 1; again(i + 1) }
        else -> h
    }
}
```

Two tables, two element sets. `data`'s elements are 0..255 (U₈); `h`'s are counts below 2³² (U₃₂, by the
`where`):

| | `data` | `h` |
|---|---|---|
| Go | `[]byte` | `[]uint32`, which Go's `repr 0..2^32-1 host "uint32"` already gives a table declared `Array(0..2^32-1)` |
| Java | `byte[]`, read `& 0xFF` | `int[]`, read σ_u: `Integer.toUnsignedLong(h[k])`; or `long[]` |
| JavaScript | `Uint8Array` | `Uint32Array` |
| LLVM | `i8`, `zext` on read | `i32`, `zext` on read |

**A sieve's flags, `Array(Bool)`**: Go `[]bool`, Java `boolean[]`, JS `Uint8Array`, LLVM `i8` (LLVM's
`i1` occupies a byte in memory). One bit each is a bit set, ADR 0082's §2.4, written with `&` and `|`.

**Pixels, `Array(0..2^32-1)`** (a 32-bit ARGB bitmap a Win32 program hands to `SetDIBitsToDevice`):
`i32` per element. The width here is also the host's, because the bitmap's memory is the host's input.
This is the boundary case of §3.2 arriving through a table.

### 3.2 Host boundaries: the host's types

A host function's parameter and result types are the host's, and the declaration states them. Today a
declaration states a set, and the target spells the set; at a boundary the **host's type is given**, and
the set is its image (ADR 0075: a host `where`'s constant bounds are its parameter's range).

**Win32** (LLP64: `LONG` is 32 bits even on x64):

| Win32 type | width x64 / i686 | its set | example |
|---|---|---|---|
| `BYTE` | 8 | 0..255 | `LOBYTE` |
| `WORD`, `ATOM` | 16 | 0..2¹⁶−1 | `RegisterClassExW` returns an `ATOM` |
| `DWORD`, `UINT`, `COLORREF` | 32 | 0..2³²−1 | `GetLastError`, `MSG.message` |
| `LONG`, `int` | 32 | −2³¹..2³¹−1 | `RECT.left`, `MessageBoxW`'s result |
| `BOOL` | 32 | **−2³¹..2³¹−1**, not a Boolean | `GetMessageW` returns −1 on error, 0 on `WM_QUIT` |
| `WPARAM` | 64 / 32 | 0..2⁶⁴−1 / 0..2³²−1 | `SendMessageW` |
| `LPARAM`, `LRESULT` | 64 / 32 | signed | `SendMessageW` |
| `HWND`, `HANDLE` | 64 / 32 | an opaque pointer (the pointers wall) | |

In J, with the host's type stated and the set its image:

```
fn GetLastError() (0..2^32-1) { host "GetLastError" import "kernel32" }           // declare i32 @GetLastError()
// MSG and HWND wait on the records and pointers walls; the integers are this step's.
fn GetMessageW(m ∈ MSG linear, h ∈ HWND, lo ∈ 0..2^32-1, hi ∈ 0..2^32-1)
    (-1..2^31-1) { host "GetMessageW" import "user32" }                            // BOOL: −1, 0, or a positive
```

The printer reads the result as σ_u for `DWORD` (`zext i32 to i64`) and σ_s for `BOOL` (`sext`), and the
program sees integers. `GetMessageW`'s set says what its documentation says and its type name hides: a
loop `while GetMessage(...)` in C runs forever on −1, and here `case { r == -1 -> …; r == 0 -> …; else -> … }`
is a three-way case on an integer, which the program cannot forget.

**Go**: almost every API takes `int`. The narrow ones: `io.Reader.Read([]byte)`, `utf8.RuneLen(rune)` with
`rune` an `int32`, `strconv.ParseInt(…, bitSize int)`. Conversions are `int(r)` and `rune(x)`.

**Java and ART**: `String.charAt` returns a `char` (U₁₆); `InputStream.read()` an `int` in −1..255;
array lengths and indices are `int` (S₃₂). Since Java has no unsigned types, a `DWORD`-like host value
arrives as an `int` holding σ_u, and is read `Integer.toUnsignedLong`.

**JavaScript**: every API takes Numbers; the widths appear only in typed arrays and in `DataView`'s
`getUint32(offset, littleEndian)`.

### 3.3 Records: layout

A Win32 struct is laid out by the platform's data layout: each field at its natural alignment, the size
rounded to the largest alignment. **Its widths are exact, and they change with the triple** (each size
below checked by `_Static_assert` with clang 23.1.4 against the Windows SDK's headers, on both triples):

| struct | fields | x64 | i686 |
|---|---|---|---|
| `POINT` | `LONG x, y` | 8 bytes | 8 |
| `RECT` | `LONG left, top, right, bottom` | 16 | 16 |
| `MSG` | `HWND hwnd; UINT message; WPARAM wParam; LPARAM lParam; DWORD time; POINT pt` | **48** (`message` padded to 8, the size rounded) | **28** |
| `WNDCLASSEXW` | `UINT cbSize, style; WNDPROC; int cbClsExtra, cbWndExtra; HINSTANCE; HICON; HCURSOR; HBRUSH; LPCWSTR ×2; HICON` | **80** | **48** |

`WNDCLASSEXW.cbSize` must be set to that size, so a program that writes 80 on i686 fails at run time.
This is 6d's records wall, and it needs from this step only that **a field's representation is the
host's, stated, never chosen**. LLVM computes the layout from the target's data layout:

```llvm
%MSG = type { ptr, i32, i64, i64, i32, %POINT }   ; x86_64-pc-windows-msvc: 48 bytes
%MSG = type { ptr, i32, i32, i32, i32, %POINT }   ; i686-pc-windows-msvc:   28 bytes
```

### 3.4 Registers: invisible, and only a cost

A scalar's width is unobservable: §2.3's theorem. It matters only through cost:

| host | narrower than the word in a register |
|---|---|
| x86-64 (LLVM, MASM) | 32-bit operations cost what 64-bit ones do and zero the upper half for free; 8- and 16-bit ones risk partial-register stalls. Division is the exception: on Intel cores before Ice Lake a 64-bit `idiv` is two to three times slower than a 32-bit one, and LLVM's x86 backend already checks at run time whether the operands fit 32 bits on CPUs tuned for it (`idivq-to-divl`) |
| ARM64 | 32-bit `w` registers cost what `x` registers do |
| the JVM | `int` against `long` matters at array indices (indextype: 1.54× on a late-exit search before the fix) |
| V8 | TurboFan computes values it types as int32 in 32-bit registers; above that range they are doubles. ADR 0082's bitwise operations are native only inside int32 |
| Go | `int32` against `int` is the same speed on amd64 and arm64; Go's indices are `int` |

So a scalar's width should be the **target's preferred register width for its fact**, not the narrowest
that holds it: `int` on the JVM for an index, int32 on V8 where a bitwise operation needs it, `i64` on
x86-64 LLVM (or `i32` where a division is measured to gain).

### 3.5 Above the word

A finite set above both words today takes limbs or the host's bignum. **The u128 factorial**
(`examples/u128`, u128-2026-09-23) computes n! for n ≤ 34 in [0, 2¹²⁸):

| target | 128-bit multiply |
|---|---|
| Go | limbs over `bits.Mul64` (hi, lo) and `bits.Add64`, `lib/num/u128.oro` |
| Java | `Math.unsignedMultiplyHigh` (JDK 18) and `*`, or `BigInteger` |
| JavaScript | `BigInt`, or four 32-bit limbs |
| LLVM | `mul i128` (three `mul`s on x86-64); `udiv i128` is a call into compiler-rt (`__udivti3`), whose availability on the MSVC triple must be checked when it is built |

On LLVM, i128 (and any iN) is the target's own: a limb library compiled into LLVM IR re-does what LLVM's
legalizer does, and "never lower further than the target requires" says not to. An unbounded set (`Nat`
above the word) has no LLVM form: Windows has no host bignum, so it has no representation there.

### 3.6 32-bit Windows

On `i686-pc-windows-msvc` the pointer-sized types (`WPARAM`, `LPARAM`, `SIZE_T`, every handle) are 32
bits, and `i64` is two registers: addition is `add`/`adc`, multiplication three `mul`s, division a library
call (`_alldiv`). Two readings of "the word":

- **The word stays S₆₄ on every LLVM triple.** It is where arithmetic is *exact*, and `i64` is exact on
  i686, only slower. The register width preferred for a scalar is 32 bits there, so a value proven in S₃₂
  is computed in `i32`.
- **The word becomes S₃₂ on i686.** Every operation not proven in S₃₂ is then refused or held above the
  word, and the same program compiles for x64 and not for i686.

The first keeps one program compiling on both; the facts make most values S₃₂ anyway.

---

## 4. The targets, side by side

| | Go | Java / ART | JavaScript | LLVM |
|---|---|---|---|---|
| signed widths | 8, 16, 32, 64 | 8, 16, 32, 64 | none (int32 inside V8) | every N |
| unsigned widths | 8, 16, 32, 64 | 16 (`char`) | typed arrays: 8, 16, 32 | every N (σ_u in the instruction) |
| U₈ in a table | `[]byte` | `byte[]` read `& 0xFF` (proposed); `short[]` today | `Uint8Array` | `i8` |
| U₃₂ in a table | `[]uint32` | `int[]` read σ_u | `Uint32Array` | `i32` |
| U₆₄ scalar | `uint64` | `long`, σ_u | split pair (not built) | `i64` |
| 128 bits | limbs over `math/bits` | `multiplyHigh` + limbs | `BigInt` or limbs | `i128`, native |
| unbounded | `big.Int` | `BigInteger` | `BigInt` | none |
| the word | S₆₄ | S₆₄ | ±(2⁵³−1) | S₆₄ (proposed on every triple) |
| an index | `int` | `int` (S₃₂) | Number | `i64` (x64), `i32` (i686) |

---

## 5. The type system

### 5.1 Sets stay claims; ρ grows

Nothing here touches the logic. A declared set is a claim (ADR 0079), a type is a set (ADR 0049), and
`Array(0..255)` says what each element is. The representation is ρ's, chosen on the IR (ADR 0033). This
document only changes **ρ's codomain**, to every (H, d) a target declares, and **ρ's rule**, to choose by
place and cost (§2.4).

**No width types.** `u8`, `i32`, `u64` in the language were rejected by bitwise.md (candidate E) and
WebAssembly's precedent, and ADR 0079 now excludes them outright: a type that chooses a representation is
a set naming a representation.

### 5.2 A table's element set is a claim that is the fact

The one place a declared set already picks a width is a table's elements (elemwidth). That fits ADR 0079
because the element set is **checked at every store**: `h[k] := h[k] + 1` is an obligation that the new
count lies in 0..2³²−1, so the declared set is a fact about every element the table can hold. For a table
a host supplies (`ReadFile`), the host declaration's set is the fact, ADR 0079's supply. An undeclared
table's element set could be inferred as the join of its stores' facts, a least fixed point as ADR 0034's
supply and demand is; that is deferred until a program needs it (§8, W5).

### 5.3 What the analysis must prove

Nothing new in kind:

- placing v in r: fact(v) ⊆ I(r), an interval inclusion;
- an operation computed on r: its result in I(r), the overflow obligation it already is;
- under `mod 2ⁿ` (ADR 0082), the result is in U_n by π_n's definition, so a fused term's natural home is
  an n-bit representation: `((h ⊕ b) * p) mod 2^32` computes in `i32` with no extension.

### 5.4 The IR's text

The printers read IR_P alone (ADR 0038), so each value's representation must be in the text: N and the
reading, or a name for the (H, d) the target declared. Today IR_P carries `int`, `u64` and `big`; a width
adds members of the same field. Emission stays byte-identical until a width changes a host type, and each
change is an `-accept` with a measurement.

### 5.5 Portability

A representation never changes an answer (§2.3), so widths add no Tier 2 behaviour. They add cost, which
differs by target and is reported, as ADR 0082's lowering cost is.

---

## 6. The candidates

### A. As today, extended case by case

Narrow table elements by the declared set and identity readings only; the four mechanisms kept separate;
the LLVM printer given widths at boundaries by a rule of its own; Java keeps `short[]`.
*For*: least work now. *Against*: a sixth, seventh and eighth mechanism, each with its own conversion
rules and its own faults; Java keeps copying files.

### B. One mechanism: the representation as an abstraction function (recommended)

Every target declares its representations as (H, d) data, including how to read and write a value:

```
// targets/java/java.oro: the same host byte, two readings
repr -128..127 host "byte"
repr 0..255 host "byte" read "(${x} & 0xFF)" write "(byte) ${x}"
repr 0..65535 host "char"
repr -2^31..2^31-1 host "int"
repr 0..2^32-1 host "int" read "Integer.toUnsignedLong(${x})" write "(int) ${x}"
repr -2^63..2^63-1 word
repr 0..2^64-1 host "long" unsigned                 // u64repr's residue, compareUnsigned and the rest

// targets/js/js.oro: a table's representation by its element set
repr Array(0..255) host "Uint8Array"
repr Array(0..2^32-1) host "Uint32Array"

// targets/windows (LLVM): every width, both readings, from the instruction set
repr iN signed  for N ∈ {8, 16, 32, 64, 128}
repr iN unsigned for N ∈ {8, 16, 32, 64, 128}
```

ρ chooses per place (W2), and the existing mechanisms become declarations of this one form; their
conversion rules become §2.3's single rule. *For*: §2.3's theorem covers every choice at once; a new
width is a line in a target file; Java's copies disappear; LLVM's widths, Win32's types and JavaScript's
typed arrays need no new mechanism. *Against*: it is a rewrite of how `emit/target.go` and `ir/words.go`,
`ir/big.go` and `ir/limbs.go` name representations, and every printer reads widths from IR_P.

### C. Width types in the language

`u8`, `i32`, `u64` as types, Rust's or Go's model. **Excluded** by ADR 0079 (a set does not choose a
representation) and rejected twice before (bitwise.md E, WebAssembly). Listed for the "Why not".

### D. Narrowest everywhere

Hold every value, scalars included, in the narrowest image containing its fact. *Against*: in a register
the narrowest is never cheaper and often costs extensions (§3.4); ambiguous between U₈ and S₁₆ (§2.4);
and it would change every emitted program for no measured gain. ρ should choose narrow where it is
observable (memory, boundaries) and where it is measured (indices, division), and nowhere else.

### The examples under each

| | A | B | C | D |
|---|---|---|---|---|
| a Java file's bytes | `short[]`, two copies | `byte[]`, `& 0xFF` on read | `u8[]`, a type in the program | `byte[]` |
| `GetLastError` on LLVM | a rule in the printer | `(i32, σ_u)`, from the declaration | `u32` in the program | `(i32, σ_u)` |
| a loop counter on x64 | `i64` | `i64` | whatever the program declared | `i8` if it fits, plus extensions |
| n! in 128 bits on LLVM | limbs | `i128` | `u128` | `i128` |

---

## 7. What would be measured

Each before its `-accept`, against hand-written code:

1. **Java file bytes**: `ReadFile` + a byte histogram + `WriteFile` over 100 MB, `short[]` with copies
   against `byte[]` read `& 0xFF`. Expected: half the memory and no copy pass.
2. **Counters**: the histogram's `Array(0..2^32-1)` as `int[]` (σ_u) against `long[]` on the JVM,
   `[]uint32` against `[]int` on Go, `Uint32Array` against `Array` on V8.
3. **Registers on LLVM**: the sieve and the merge sort of backend-2026-10-26 with every proven-S₃₂ scalar
   in `i32` against `i64`, on x64 and i686.
4. **Above the word**: the u128 factorial and the Karatsuba programs with native `i128` against the limbs
   library, on LLVM.
5. **i686**: the Windows sieve compiled with the word S₆₄ against a 32-bit build, to show the cost of
   keeping one word.

---

## 8. Decisions for hamza

**W1. A representation is an abstraction function (H, d), and every integer representation is one.**
*Recommended: yes* (candidate B). One mechanism, with §2.3's theorem as its correctness argument, replacing
the six built separately. An ADR amending 0033, 0034, 0035 and 0079, fulfilling CLAUDE.md's waiting item.
Built in stages: new representations first (LLVM's widths, Java's σ_u `byte`, JavaScript's typed arrays),
then the existing mechanisms re-declared in the same form, each stage byte-identical except where a width
changes and is measured.

**W2. ρ chooses by place.**
*Recommended*:
- at a **host boundary or a record field**: the host's type, stated in the declaration, never chosen;
- in a **table**: the cheapest image holding the element set, by size, including a second reading of a
  host type where it is denser (Java's `byte` as U₈);
- for a **scalar**: the target's preferred register width for its fact (the word; the JVM's `int` for an
  index; V8's int32 where a bitwise operation needs it), narrower only where measured faster;
- **above the word**: W3.

**W3. Above the word, LLVM holds a finite set in native iN.**
*Recommended*: N the set's bit length rounded up to a multiple of 64, so the rung above is `i128`,
`i192`, …, and the limbs library is retired on LLVM targets once measured against it (§7 item 4). An
unbounded set stays refused on LLVM, which has no bignum, until a program asks for one.

**W4. The word is S₆₄ on every LLVM triple, i686 included.**
*Recommended*: the word is where arithmetic is exact, and i64 is exact on i686. The preferred register
width (W2) is 32 bits there, so values the analysis proves in S₃₂ are computed in `i32`. Pointer-sized
host types (`WPARAM`, `LPARAM`, handles) take the triple's width from the triple's target layer.

**W5. A table's element set is the declared one; inference waits for a program.**
*Recommended*: as now, a declared element set is a claim checked at every store and chooses the element
representation. Inferring an undeclared table's element set from its stores (a least fixed point) is built
when a program's table is wider than its stores and the difference is measured.

**W6. IR_P carries each value's representation.**
*Recommended*: N and the reading, so every printer reads widths from the IR's text alone (ADR 0038), and
`cmd/check` reprints them byte for byte.

**What follows, if taken**: one ADR (with ADR 0082 already decided), then one build, program first:
`hash/fnv` and `hash/crc32` over a file. They exercise W2 in a table (the file's bytes, CRC's table of
U₃₂) and at a boundary (`ReadFile`), ADR 0082's operations and fusion (FNV's `mod 2^64` and `mod 2^32`),
and Java's σ_u `byte`. They are measured at parity with Go's own on Go, Java and JavaScript. LLVM's widths
come with `ir/llvm`.

## 9. Not here

- **Pointers**: the next wall. A handle's width is the triple's, but what a pointer *is* is that step's.
- **Vectors**: a vector of i8 against i32 is a question of lanes, wall 4.
- **Floats' widths** (f32, f16): wall 5. Its algebra is the same shape: binary32 is a subset of binary64,
  held in 32 bits.
- **Bit fields** in records (`i3` fields of a C struct): LLVM can express them; no Win32 struct
  TinyRetroPad uses has one.
