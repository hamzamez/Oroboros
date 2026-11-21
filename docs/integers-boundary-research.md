# Research: `Int` at an entry point, and one rendering of ℤ

2026-11-20. hamza, reading the book's §0.5: `square(n ∈ Int)` is refused while `cube(n ∈ Int)` is
accepted, so "which is it?"; printing through `big_str` "is deciding representation, it is no different
than go have + for int and addition for bigint … printing should be [no] different … it should start at
string"; and why `wide(n ∈ 0..2^63-1) (0..2^126)` compiles on Go and not on JavaScript, which has
`BigInt`. Then, on the derivation below: "1. yes 2. decimal, write the research note".

**Decided** (hamza, 2026-11-20):
1. **`Int` denotes ℤ at an entry point too**, its representation chosen from what is known there, the
   host's word spelled by its range.
2. **The rendering of ℤ is `decimal(n)`**, one function, its realization chosen by the representation.

This note derives both, measures what they change, and names what they decide beyond the two questions.

## 1. The two defects

The law already decided: **a set's name denotes one set** (ADR 0073), and **a value's representation is
ρ of its proven fact** (ADR 0079 item 2). Two rules break it.

**`Int` denotes two sets.** Inside a program `Int` is ℤ. At an `export` the book says "`Int` is the
host's own word" (§0.5, §6.8): −2⁶³..2⁶³−1 on Go, the JVM and windows, ±(2⁵³−1) on JavaScript. So

    export fn square(n ∈ Int) (Int) { n * n }

is refused for where it is, not for what it computes, and one export denotes a different set on each
target, so its portability is not well defined. The book's finding 94 is this gap from the other side:
all of ℤ has no spelling at an entry point (`n ∈ ..` is refused as "`Int`", and `Int` there is the
word), so a host can pass `Nat` or `..0` at any size, and not both halves in one parameter.

**Rendering is split by representation.** ADR 0095 item 3 decided "rendering is ℤ's at every sort,
chosen per representation", and `big_str` is that function, total on ℤ. But its name names a
representation, and beside it `io.print_int` is a host declaration over the word. So `cube` is accepted
in §0.5 only because `big_str`'s parameter is ℤ, and the book calls that "the consent"; the same `cube`
printed by `print_int` is refused. The consent is nothing but a consumer whose set is ℤ.

## 2. The law at an entry point

At an entry point nothing proves a parameter smaller than what the header states: its set, and the
constant bounds of its `where` (ADR 0075). That is the whole fact, so by ADR 0079 item 2 its
representation is ρ of it:

- **inside the word** (the set and the `where` bound it there): the word, as today;
- **otherwise**: the host's integer of any size, Go's `*big.Int`, JavaScript's `BigInt`, the JVM's
  `BigInteger`, windows' limbs (K(ℕ\*), ADR 0099).

A result's representation is ρ of the range the body proves, met with its claim (ADR 0072, 0087), as
for any value. So `square(n ∈ Int) (Int)` is `*big.Int → *big.Int` on Go and its counterpart on each
host, exact, accepted everywhere: **`Int` denotes ℤ at every place**, and the book's "at an export
`Int` is the host's word" goes. Finding 94 dissolves: all of ℤ at an entry point is `Int`.

**The host's word, at an interface that wants it, is its range**: `n ∈ -2^63..2^63-1` is an `int` on
Go, a `long` on the JVM, an `i64` on windows, and on JavaScript, whose word is ±(2⁵³−1), a `BigInt`,
because that is what the set needs there. No set of the language means "this target's word": such a
name would denote a different set on each target, which ADR 0073 refuses. A target may name its own
host type's set in its own module, `set int = -2^63..2^63-1` in `go`, a name that denotes one set.

**Precedent.** Dafny's `int` is ℤ, at a method's boundary as inside it, and is compiled to the target's
big integer (`BigInteger`, `BigInt`) unless a `newtype` with a range says less; Why3 extracts its
mathematical `int` the same way. Common Lisp and Python choose fixnum or bignum per value at run time;
here ρ chooses per value at compile time, from proofs.

## 3. One rendering: `decimal`

Let **r : ℤ → Σ\*** be the decimal numeral: r(n) = "−" ++ r(−n) for n < 0, the digits of n in base 10
otherwise. It is one total function, and **printing is write ∘ r**. So the primitive is the map into
`String`, and printing composes with it:

- **`decimal(n)`**, a word of Σ_L over ℤ, replaces `big_str` (renamed; same meaning, ADR 0095 item 3).
  Its realization is chosen by ρ of its argument, as `<` at F64 is `fcmp`: in the word `strconv.Itoa`,
  `String(n)`, `Long.toString`, and on windows `print_int`'s digit loop over `sdiv` and `srem`; on a
  host big integer its own `String()`; on limbs the limb library's `render` (emit/bignum.oro).
- **`io.print_int(n)` is a definition over ℤ**, `print_line(decimal(n))`, no longer a host declaration
  over the word. Its 499 uses keep their spelling. On windows it becomes total: today it stops past 20
  digits (print-2026-11-19), and that owed item closes.
- `string_of(n)` is not the name: `string_of(65)` is already "A" (a scalar's text, strings.md).
- **`num.decimal(x, s)`** renders an element of ℤ[1/10] at scale s (ADR 0086). Its restriction to ℤ at
  scale 0 is `decimal`: num.decimal(n, 0) = decimal(n) for n ∈ ℤ, a law the build checks on every
  target.

## 4. JavaScript's `wide`

    export fn wide(n ∈ 0..2^63-1) (0..2^126) { n * n }

0..2⁶³−1 is above JavaScript's word, so by §2 the parameter is a `BigInt`, the product exact, the
result a `BigInt`: it compiles. Today it is refused ("only a value in the signed word widens into
arbitrary precision"), and measured:

| export | Go | JVM | JavaScript |
|---|---|---|---|
| `below(n ∈ ..0) (..0)` | `*big.Int` | `BigInteger` | a `BigInt` parameter, accepted |
| `wide(n ∈ 0..2^63-1) (0..2^126)` | `int` → `*big.Int` | `long` → `BigInteger` | refused |
| `sq(n ∈ Int) (Int)` | refused | refused | refused |

The boundary takes a parameter in the word or a set unbounded on a side as the host's big integer, and
has no case for **a bounded range above the word**, which is exactly `wide`'s on JavaScript. The book's
§0.5 rationalizes the gap ("a JavaScript number above 2⁵³ has already lost its low digits"); with a
`BigInt` parameter a JavaScript caller passes `2n ** 62n` exactly. So yes: the program can be written
for JavaScript, and its refusal is a defect of the boundary rule.

## 5. Host declarations

A host declaration is its host's prototype (ADR 0022, 0094), and its sets are claims about the host. A
host's `Int` would claim ℤ for a type that is the word: **134 of the 826 host declarations write `Int`**
(Go 35, the JVM 54, JavaScript 22, LLVM 21, blas 2). Each states its host type's set instead, by a name
its target defines once (`go.int`, `java.long`, `js.int`: JavaScript's safe integers, ±(2⁵³−1)), as
Win32's already state C's (ADR 0126). The survey's generator writes them so.

## 6. What changes, measured

Of 699 exports, **159 write a bare `Int`**. A result changes nothing: an export accepted today has its
result proven inside the word, and ρ of that hull is the word. A parameter changes where its `where`
does not bound it inside the word: **27 exports** by the headers' text (a regex over the sources;
the build counts them exactly):

- **17 are the book's examples**, most of them refusals shown on purpose (`no-square`, `no-add`,
  `no-chain`, `no-midpoint`, `no-bound`, …): each refusal that was the boundary's becomes accepted,
  and its section is rewritten;
- **3 are the gauntlet's harness entries**, `run(k ∈ Int)` in `json/tokenize`, `json/tree` and
  `kara/core`, which hand-written benchmark code calls with an `int`: each states k's range, keeping
  the word and the benchmark's interface unchanged;
- **7 are examples whose parameter is a host word in fact**, `divmod`'s divisor (four, on Go and
  the JVM), `sieve_count`, `power`'s base, `parse.step`: each states its range, or becomes exact where that is what it means.

`big_str` is used 37 times and becomes `decimal`; `num.decimal` 62 times, unchanged.

## 7. What this decides beyond the two questions

**A printed value is computed exactly.** `print` demands ℤ, and by ADR 0034 a demand above the word
is what selects the rung above, so a value whose only consumer is a print is exact at any size. Chapter
0's `no-grow`, 3 squared seven times and printed, is refused today; it then prints 3¹²⁸'s 62 digits.
**"Bounded by default"** (ADR 0019 item 2, ADR 0079 item 3) becomes precise: an operation computes the
integer it means, and **a refusal happens only where a value must enter a bounded set it is not proven
in** — a table's index, a host's word parameter (WriteFile's DWORD), a declared range. That is ADR
0026's sentence, "an `Int` is an integer", with no exception at the boundary or at the printer. The
cost of a value above the word is not hidden: the compiler reports each operation computed there
("N operation(s) above the word"), as it reports the unsigned word's today, and `-checked` keeps its
meaning where a bounded set is demanded.

**Findings that close**: 94 (all of ℤ at an entry point), 27 (which rule at an entry point: §2), and
windows' 20-digit stop (print-2026-11-19). Finding 28, a value above the word coming back into it, is
not touched.

**Built** (2026-11-21, ADR 0147, [intboundary](../gauntlet/results/intboundary-2026-11-21.md)) for an
entry point's parameters and results, for printing and for a host's word. **Not built** for an
intermediate whose every consumer is bounded: the word midpoint's `lo + hi` is still refused, and
taking §7's sentence whole would make an accumulator whose fact is unbounded only because a table's
length is a big integer silently (the book's finding 108), a question for hamza.

## 8. Not decided here, owed to the build

- the cost of `io.print_int` as `print_line(decimal(n))` against today's direct call (Go's
  `fmt.Println(n)`): measured on the gauntlet, which prints its results;
- whether an export's parameter above the word should also accept the host word at the call (a Go
  caller passing `int` to a `*big.Int` parameter writes `big.NewInt(n)`): the interface is the set's,
  and an adapter is the host's code, not ours;
- the order in the plan (refusals of correct programs, so eligible to go ahead of the struct forms).

## 9. The build, each step its witness

1. **The boundary** (§2, §4): an entry point's parameter ρ of its set and `where`, a bounded range
   above the word the host's big integer; `square` and `wide` on every target. The fault: a bounded
   range above the word taken as the word, which must refuse JavaScript's `wide` again.
2. **Host declarations** (§5): each target's word set named once and written in its 134 declarations;
   the survey's generator and checker read it.
3. **`decimal`** (§3): Σ_L's word, `big_str` renamed in the 37 uses; `io.print_int` a definition over ℤ;
   the law num.decimal(n, 0) = decimal(n) on every target. The fault: a realization chosen by the
   static type and not by ρ, which must print a word value through the big path, or refuse.
4. **The corpus** (§6): the 27 exports' ranges stated where they mean the word, the gauntlet's
   interfaces unchanged, its timings re-measured.
5. **The book**: §0.5 and §6.8 rewritten (no exception at the boundary, no "consent"), `no-grow` as
   the exact program it becomes and `-checked` shown where a bounded set is demanded; findings 27 and
   94 closed; inventory.md's `decimal`.
6. An ADR for the decision, reversing ADR 0079 item 3's "an entry point's parameter … supplies" as the
   host's word, amending ADR 0095 item 3's name, and its "Why not": a target-dependent `Word` set; `Int`
   kept as the word at a boundary with `..` for ℤ (two spellings of one set, with two meanings);
   `string_of` for the rendering.
