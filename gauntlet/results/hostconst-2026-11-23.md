# A constant of ℤ is the host's own

Date: 2026-11-23. hamza, 2026-11-22: "yes, go with a for windows and the hosts. always mathematical
and algebraic" (ADR 0157). He chose it after seeing a constant rebuilt in a loop measured on windows
and on Go.

## The law

A constant c ∈ ℤ is a nullary operation of the initial ring. Its realization on a target is a closed
term over the host's signature that denotes c (ADR 0089). Of those terms, the founding rule takes the
one at the highest layer the host provides:

- **Go:** a `*big.Int` made once from its decimal;
- **the JVM:** a static `BigInteger`;
- **JavaScript:** a BigInt literal;
- **a target with no bignum:** K(ℕ\*)'s numeral itself (ADR 0099), laid out in read-only data.

The spine, Horner over 10¹⁵ computed where c is read, also denotes c. But it lowers further than the
host requires: a loop rebuilt the constant on every iteration.

Two laws bound the rule:

- **The frame rule (ADR 0101).** A program function's result is a birth its caller owns and releases.
  - A static numeral is therefore only where every use borrows it.
  - Go's `*big.Int` is mutable, so a constant that a terminator moves is a copy.
- **A closed term is a constant only where a constant has a form of its own.** That is the host's
  bignum and K(ℕ\*). On fixed limbs the layout is the instance's width, so a closed term keeps its
  structure, which shares its operands.

## Measured first

On HEAD (228a3c26), `acc + i · 2¹⁰⁰`:

| target | constant rebuilt (HEAD) | constant made once |
|---|---|---|
| Go, 100,000 iterations | 7.8–9.0 ms | 3.3–4.6 ms (2.4×) |
| windows, 2,000,000 iterations | 111–115 ms | 32–33 ms (3.5×) |

- On windows the numeral was rebuilt on every iteration, and the function was 1,701 lines of LLVM IR.
- The Go figure comes from a hand benchmark written to mirror the emitted code.

## What was built

1. **The IR's constant held exactly** (`ir/bigconst.go`, `ir/big.go`).
   - It is an `OConst` of the rung's type, `big`, whose literal is the integer (a numeral past `int64`).
   - The rung's widening makes one (`bigConst`) where it made a spine.
   - Its fact is its point: `ConstValue` and `pointOf` read it.
   - The IR's text writes it as its decimal, and the reader reads it back as the integer.
   - `PlantConstantSpined` puts the spine back.
2. **A closed term is its constant** (`ir/big.go`): an operation whose fact is one point and which is
   held in ℤ becomes that constant. This applies only under `closed`, which means the host's bignum
   or K(ℕ\*) (`plan.Var`). It never applies to:
   - a point of U on a target with an unsigned word or a pair (that word's constant);
   - a point past the decision's 192-bit ends (ADR 0105), since the facts state no such point.

   `PlantClosedTermsOnLimbs` turns the rule on for fixed limbs.
3. **The printers.**
   - **Go:** a package-level `var oroBigN, _ = new(big.Int).SetString("…", 10)`. A yield, a
     `continue` or a `break` moves it as `new(big.Int).Set(oroBigN)`. `PlantSharedConstantMoved`
     returns the shared constant instead.
   - **The JVM:** `private static final java.math.BigInteger oroBigN`.
   - **JavaScript:** `cn`.
   - The constants are declared from one registry per printer (`emit.GoConstDecls`,
     `JavaConstDecls`), written by `emit.File`, `JavaFile` and `WriteProgram`.
4. **windows** (`ir/limbs.go`, `ir/llvm`).
   - A constant every use of which borrows it (`onlyBorrowed`: a call of the rung's operations, or
     `decimal`) is its numeral in read-only data: `@.num.N = private unnamed_addr constant { i64, [L x i64] }`,
     the digits in base 2^dbits followed by the header 2m + s.
   - Every other constant is its spine (`spineConsts`).
   - No birth makes a static numeral, so nothing releases it (ir/release.go owns births only).
   - IR_P's element classes read the numeral's digits (`ir/final.go`).
   - `PlantStaticNumeralEscapes` keeps a numeral static even when it leaves its function, and its
     caller then releases read-only data.
5. **The analyses.**
   - The interval analysis reads a numeral constant's length and elements, after the `KInt` fast path.
   - The IR's interpreter reads a numeral constant.
6. **JavaScript's constant supply, refined** (`ir/exact.go`, `constUsesOutsideThePair`): a constant
   of U is held by the pair where only the pair's operations, a comparison or a join read it. A
   table's storage is a sink, so a literal table past the word is refused (see below).
7. **Tests.**
   - `TestAConstantIsTheHostsOwn` (cmd/gen): checks the code's shape on four targets, the spine plant,
     and Go's copy with its plant.
   - `TestAClosedTermOnLimbsIsItsTerm` (cmd/gen).
   - `TestAStaticNumeralIsNeverReleased` (cmd/build): on windows, a kept function returns constants
     and is called in a loop; with the plant, the run fails.
   - The table refusal joins `TestAConstantOutsideTheWordIsHeldExactly`.
   - Three tests changed:
     - `TestTheOneSetIsCheckedWhereItClaims`: x is a loop's now; read from a one-element table it was
       a point, and x·3 a constant with no operation to fit;
     - `TestAQuotientByANumeralIsAlgorithmDs`: its operands are opaque;
     - the logic midpoint's expected BigInt literal.
8. **The book.**
   - Chapter 0's Go code shows `oroBig0` and the copy.
   - Chapter 12's `ones.oro` note counts 60 operations.

## What failed first

- **Every compile was 13× slower.** freq on Go took 286 s against 22 s, and the compile-time gate
  paired 139 suspects. The cause was the interval analysis's `OConst` case: it called `ConstValue` and
  the numeral's element reading for every literal. Putting the `KInt` fast path first made the compile
  equal to HEAD's.
- **The IR's text reader refused a numeral constant.** It now reads the decimal.
- **windows' `print_int` of a literal in U was refused** (an unbounded `sdiv`): the closed-term and
  supply rules had made a point of U exact. Excluded on Unsigned and Pair targets.
- **windows32: "table takes declared element … [−∞, +∞]".** IR_P had no source for a numeral
  constant's elements. `final.go` now reads them.
- **"type big has no LLVM type".** `spineConsts` did not walk a branch's arms, and `LowerLimbs`
  returned early from a function with constants and no plan.
- **A static numeral returned to its caller** would be released by the caller. That is the frame
  rule; the witness is `PlantStaticNumeralEscapes`.
- **Go's shared `*big.Int` was returned itself**, so a caller writing into its result would change the
  constant. Terminators now copy it.
- **Fixed limbs, one constant per closed term.**
  - It meant one spine per term and per chunk of its numeral.
  - big-signed-ops compiled 3.2× slower on windows, and the JVM's limbs passed their 64 KB method
    ("code too large").
  - The rule is now off there, and its plant shows the code grows.
- **The first JavaScript supply rule was too broad**: it refused `pairs.oro` and was narrowed to what
  the pair holds.

**A wrong answer, older than this step:** on JavaScript, `array { 9223372036854775807,
9223372036854775806 }` was emitted as Numbers, and both elements read 2⁶³. A table's element is a word
by storage, so a constant past JavaScript's word in a table is now refused, as a computed one is.

## Paid and bought

| | |
|---|---|
| compile time | 1.00× the baseline's median after the fix; none confirmed slower |
| run time | a loop over 2¹⁰⁰: windows 111 → 32 ms (3.5×); Go 2.4× by the hand form; on the JVM a static field and on JavaScript a literal, not timed; no benchmark program changed |
| code size | 19 files, 94,653 lines inserted and 116,329 deleted; big-bits on windows 115,708 → 94,175 lines changed, its function in the measured loop 1,701 → 665 lines; JavaScript's big-signed-ops 28 lines longer (each arm its BigInt literal, the fit at the claim gone, the constants proven inside it) |
| proofs | 12,595 of 12,649 operations → 12,504 of 12,558; 1,588 of 1,889 loops → 1,575 of 1,876. The 91 operations and 13 loops are all big-bits on windows (213 → 122 operations, 25 → 12 loops): the spines' limb operations, now a numeral; the 54 and 301 left are unchanged |
| refusals changed | none in the corpus; a JavaScript table literal past the word refused (it answered wrong) |
| emitted text changed | 19 files, 38 accepted changes: big-bits on four targets; big-negmid, big-sign, big-signed-ops and big-subdiv on Go, the JVM and JavaScript; JavaScript's logic midpoint, double-word and quotient-corner. Each reads a constant where it built one, and every differential case agrees |
| compiler lines | `ir/bigconst.go` new (215 lines); 261 inserted and 14 deleted in 14 files of `ir/` and `emit/` |
| language lines | none |
| a rule made less general | the closed-term rule stops at fixed limbs, at U on an unsigned or pair word, and past the decision's 192-bit ends (big-sign on Go folds only partly) |

## Not built

- A static numeral that leaves its function. It would need a result that the caller does not own,
  that is, a release test on read-only data or a borrowed result: a change to the frame rule.
- The decision's ends past 192 bits, which would let a closed term past 2¹⁹¹ be a point.
- Timing the JVM and JavaScript forms.
