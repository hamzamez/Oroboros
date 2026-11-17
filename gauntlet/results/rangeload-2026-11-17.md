# φ's `!range` on a load: measured, not built, and the 7.9× is not the digit step

2026-11-17. Item 2 of [assessment-2026-11-17](../../docs/assessment-2026-11-17.md) §5. The item was
set on [d2](d2-2026-11-17.md) §4's reading: windows32's 7.9× on 20000! comes from the digit step's
64-bit arithmetic, and LLVM needs a digit's range to make it one `mull`. Both halves were measured,
and neither holds.

## 1. The algebra, and what LLVM reads of it

The set: a table's IR_P element type S is the hull of every value stored into it, each store proven
in S, and a table entering a function carries its declared element set. So every load yields a
value in S, which is exactly `!range`'s meaning; a value outside S would be poison, and none occurs.
The fact is sound. The question was whether LLVM uses it.

Probes, LLVM 23.1.4, `--target=i686-pc-windows-msvc -O2`:

| the step t = a·k + c | machine code |
|---|---|
| `load i64 … !range [0, 2³²)`, k a parameter with `range(i64 0, 2³²)` | both halves of a loaded, `mull` and two `imull`: **neither fact used** |
| inline `!range !{…}` on the load | parsed, not used |
| `load … !range !0`, and k = `and k, 2³² − 1` | one load, one `mull` |
| `load i32` of the low half, `zext`, and k masked | **one `mull` from memory and an `adc`**, Go's 386 step |

So the backend reads a value's high zeros from instructions (`zext`, `and`), not from a load's
metadata or a parameter's attribute. The fact in the form LLVM's i686 code generator uses is the
representation, U₃₂ read as `load i32` and widened (ADR 0083), which is D5's.

## 2. What it is worth

Hand-edited, the 20000! program's 73 element loads became `load i32` + `zext`, and `mul_small`'s
multiplier was masked. That is valid here, since every i64 table in that program holds digits or
headers below 2³², and the output was checked against Go's. Nine interleaved rounds, medians:

| | before | the representation form | Go's `math/big`, 386 |
|---|---|---|---|
| 20000!, computed and printed | 256.1 ms | 245.2 ms (**0.957**) | 27.6 ms |

**4%.** The product's inner loop is not where the time goes, and d2 §4 said it was. Split:

| | windows32 | Go 386 | x86_64 | Go amd64 |
|---|---|---|---|---|
| 20000! computed, its remainder mod 10⁹ + 7 printed | 135.5 ms | 16.4 ms | 31.0 ms | 10.2 ms |
| 20000! computed and printed | 215.9 ms | 28.1 ms | | |

The product alone is 8.3× Go's at 32 bits and 3.0× at 64. Making its digit step Go's buys 4%, so
the product's cost is outside the multiply. The candidate, measured nowhere yet: each `acc * i`
makes a new numeral. A region of 64 KiB or more is `VirtualAlloc`'s, with fresh pages faulted in,
and an 8-byte digit holding 32 bits doubles the bytes at 32 bits. That is a hypothesis, owed with
its measurement (page faults and allocator calls per step), not a finding.

## 3. What failed first

1. **d2 §4's conclusion.** I read the machine code of one loop, saw a 64×64 multiply, and named it
   the cause. The claim had already been corrected once, from the division; this is the second
   correction. The lesson is the one the assessment named: a cost is assigned by measuring it, not
   by reading the code that looks expensive.
2. **The hand experiment of d2 §4** did mark the product loop's load: the loop is `mul_small`
   inlined into `main`, whose load matches `%.eN`. I wrote there that it had missed it. It had not:
   LLVM ignored the metadata, which the probes show.

## Paid and bought

Nothing built: compile time, run time, code size, proofs, refusals and emitted text are unchanged.
The cost was the probes and two timed experiments, in the scratchpad.

## What was not built

- **φ's `!range` on a load**: sound, but unused where it was meant to help, and on x86_64 a digit is
  the unsigned word, which has no range to state.
- **D4's 2-by-1 `divl` for q̂**: d2 §4 bounds it at a few percent of printing. It waits for a
  measurement naming it.
- **The representation form** (`load i32`, `zext`): it is D5's (plan item 4), and worth 4% here.
- **The allocation hypothesis** (§2): owed, its measurement first.
