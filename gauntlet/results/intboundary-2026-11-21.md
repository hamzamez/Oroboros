# intboundary — `Int` is ℤ at an entry point too, and `decimal` renders ℤ (ADR 0147), 2026-11-21

hamza, 2026-11-20: "1. yes 2. decimal, write the research note", then "go ahead build it now. and
rewrite chapter 0 from scratch when you finish". The note is
[integers-boundary-research.md](../../docs/integers-boundary-research.md); the decision ADR 0147.

## The law

`Int` denotes one set, ℤ (ADR 0073), at every place. A representation is ρ of what is proven
(ADR 0079): an entry point's parameter is ρ of its header's set and `where`, the word where they
bound it inside the word, otherwise the host's integer of any size. A result declared `Int` is ℤ,
so each yield whose own fact leaves S is demanded exactly, as `decimal`'s argument is. Printing is
write ∘ decimal, decimal : ℤ → Σ\* realized per representation: on S the host's own formatting
(`strconv.Itoa`, `String`, `Long.toString`), above it the big integer's, on windows the limbs'.
ℤ = S ⊔ (ℤ ∖ S) is windows' `print_int`, the field on S and `decimal` beyond.

Three facts the build derived, each a theorem of the storage:

- **a length bounds**: len(t) ≤ L, the target's largest table, so `i < len(t)` gives i ≤ L − 1;
  and **difference bounds close**: x ≤ y + c with y ≤ h gives x ≤ h + c, the two-variable fragment
  of Fourier–Motzkin, settled in n rounds (Bellman–Ford) for n parameters;
- **a table's element and a map's value are words**: every store owes its value in the word
  (ADR 0119), so an `Array(Int)`'s element lies in S whatever its set;
- **a `where`'s conjuncts are independent assumptions**: dropping the ones over a parameter moved
  above the word is sound and keeps the rest.

## The programs

The book: chapter 0 rewritten from scratch (108 blocks, every function shown with its use, output
and emitted code), and chapters 3, 5–8, 10, 12–14, 16, 17, 19 brought to ADR 0147, every chapter
checked. `square(n ∈ Int) (Int)` prints 10²⁰ on Go, JavaScript and windows; 3 squared seven times
prints 3¹²⁸'s 62 digits on all four; `wide(n ∈ 0..2^63-1)` is portable to JavaScript, a `BigInt`;
25! prints whole on both windows triples.

## What was built, and the law each rests on

| | |
|---|---|
| `core.Sig.Entry`, `emit.EntrySig` | an export's `Int` parameter takes its `where`'s bounds (constants, `lenBound`, the difference closure), the word inside S, else `int lo hi` with an open end; windows refuses an unbounded one, its callers C's |
| `ir/big.go` | a JavaScript pair parameter at an entry a `BigInt`; a result declared `Int` demands each yield whose own fact leaves S (`resultIsZ`); `decimal` of a word value is `decimal-word`; a residue's spine hi·10¹⁵ + lo is a literal, not an exact value |
| `emit.PlanBig` | a results-only plan, `ResultsOnly`, which re-decides nothing when nothing is held exactly; `CheckClaim` skips it, ℤ being a claim nothing fails |
| `ir/interval.go` | an `Int` element and map value in S by storage |
| `ir/lower.go` | `assumeWhere` drops only the conjuncts over a parameter above the word |
| `ir/keep.go` | a definition whose untyped parameter the body uses as an integer is inlined (P4), not kept as `any` |
| `ir/golang` | `math/big` imported for a big integer in a function's type; a host result spelled as the word, `Word`, received as Go's `int` without a conversion |
| `core/jshow.go` | the big type shown as `Int` in a message |
| targets, lib | `decimal` (renamed from `big_str`, 55 files), `decimal_word` on Go, the JVM and JavaScript; each host module's `set Word`, 159 host declarations writing their word as `Word`; `io.print_int` a definition over ℤ on Go, the JVM and JavaScript; windows' `print_int` split at the word |
| examples | the gauntlet's harnesses `run(k ∈ 0..2^31-1)`, divmod's divisor, `sieve_count`, `power`'s base, `parse.step` given the ranges they meant |

## What failed first

- `wide` on JavaScript: the parameter typed in the unsigned word's pair, no host type. The pair at
  an entry is a `BigInt` now.
- `square` still refused after `EntrySig`: the decision runs before the rung, so the widened header
  must reach `ir.Entry` itself.
- On windows `io.print_int` as `print_line(decimal(n))` inlined `llvm/lang`'s `render`, a
  realization, at every call: kept.oro went from one function and three calls to three copies. It
  stays `print.print_int` on windows, the kept function, with the arm above the word inside it; a
  split `print_word` was dropped by P4 at its unsigned-word call, and a constant 2⁶⁴ − 1 hit
  U₆₄ ↪ ℤ (not built).
- Chapter 6 found `export fn any(n ∈ Int) (Int) { n }` emitted `*big.Int` without `import
  "math/big"` (finding 109, fixed).
- Chapter 7's 64-bit FNV-1a, printed, was refused at the target's `+`: Fuse's residue spine taken
  as an exact constant. The same program with `big_str` fails at HEAD; `print_int` over ℤ made the
  rung run where it had not.
- A result's demand keyed by its root took binary search's loop variable, whose widened join leaves
  S, with its π-narrowed yield: a yield its own fact keeps in S is not demanded.
- `search(t, x ∈ Int) where len(t) <= 10^9`: the whole `where` dropped for `x`, a big integer now.
- Chapter 14's sort on windows: once an element is a word, `sink(b, start)` is kept, and `start`,
  used only as a loop's initializer, lowered as `any`, which LLVM held as a pointer. The first
  rule (any untyped parameter) inlined `print_str(s)` too, a string, rightly a pointer; narrowed to
  a parameter used as an integer.
- The first full check found four more. `fnv64` on both windows triples: an unsigned hash
  printed reached `decimal`'s arm above the words, whose disjunctive guard narrows nothing, so it
  was demanded exactly (U₆₄ ↪ ℤ, not built). The arm is now two guards with signed-word constants
  (n/2 > 2⁶³ − 1 for n ≥ 2⁶⁴), `decimal`'s demand reads its argument's own fact, on a target
  with no bignum a value in U is no demand, and a closed constant in U joined with a residue mod
  2⁶⁴ is that residue's literal (windows32's offset basis). Taking every constant in U for the
  word's literal broke −2⁶³ · 3⁴⁰ (big-nat-ops: refused, and trapped in the harness). `du` on Go:
  exempting a `Word` result from Go's conversion dropped `int64`'s, the same set; reverted, a
  `Word` result converted like any range (a no-op `int(…)`). gauntlet/stdlib's strconv program
  defined `fn decimal()`, overridden by the target's new name (ADR 0002's ▷, with its note), so
  its section printed nothing: renamed `atoi_itoa`.
  `js-shape`: a host result typed as the big integer taken for a word.
- The accepted baseline was checked outcome by outcome against HEAD's, and one program had gone
  from emitted to refused unnoticed among the 154 changes: `sieve-js` on JavaScript, which counts
  with the host's own `js.+` on `Any`. The result's demand made its accumulator, a join of host
  results, exact and then refused the host's operator. A join is exact by demand only where some
  input can be, the least fixed point over joins (`TestAJoinOfHostResultsIsNotExact`). Other
  tests that wrote `(Int)` or a print meaning the word now state it.
- A heredoc ate a backslash five times; scripts went to files.

## Paid and bought

| | |
|---|---|
| compile time | 1.09× the baseline (`cmd/check`'s gate, 106 compiles over 300 ms), none confirmed slower paired against HEAD's binary; the second accept 1.00×. Paired serially and warm against HEAD's `gen`: `examples/logic/windowtext.oro` windows 1.35× (+130 ms, under the gate's 250 ms), `examples/json/tokenize.oro` java 1.02×. The rung now runs on every program that prints (`decimal`), and is decided again after it |
| run time | printing an integer on Go is `strconv.Itoa` then `fmt.Println(s)` where it was `fmt.Println(n)`: into `io.Discard` 52 ns against 36 (1.4×), into a file 2.05 µs against 1.93 µs a line (median of 5, under the 15% floor: the write's system call dominates). No gauntlet program prints in its timed loop; their emitted code changed by numbering only (below). A function taking `x ∈ Int` now compares a big integer where it compared a word (chapter 13's `position`, `search`), a cost its header chose |
| code size | windows: an export's ranged parameter lowers its `where` as an assumption, a dead test LLVM removes (`tokenize_run`, `bits_step`); the kept `print_int` carries its dead arm above the word in its text, the binary unchanged (kept.exe 4,096 bytes before and after) |
| emitted text changed | 154 changes accepted, then 4 more (`ACCEPTED.md`), by target in the first: Go 28, JavaScript 21, the JVM 22, windows 89, emitted text or compiler output. Go, JavaScript, the JVM: every program printing an integer (`strconv.Itoa`, `String(n)`, `Long.toString`), the JSON harnesses renumbered (their `run(k ∈ 0..2^31-1)`, the JVM's parameter an `int`), no-op `int(…)` on a `Word` result (`wordcount`, `lines`), `idot`'s big integers gone. Windows: `print_int`'s two arms above the words in every printing program, its kept function 1,119 → 3,021 lines in `examples/logic/maximum` (the numerals' render in a dead arm, which LLVM removes: kept.exe 4,096 bytes before and after), and a `where`'s conjuncts assumed beside a parameter above the word (dead tests). The gauntlet's changed harnesses timed against HEAD's: windows g6-alloc, g6-into, g7-tok 1.00× over three alternating rounds |
| proofs | 12,725 of 12,779 operations and 1,603 of 1,906 loops, from 12,697 of 12,753 and 1,501 of 1,806: the corpus's 26 new operations and 100 new loops all proven, windows' kept `print_int` among them; two unproven operations fewer (the logic midpoint on JavaScript, refused now at its literal 2⁶² before counting) and two loops more proven (`sieve-js`'s); no program changes between emitted and refused against HEAD's outcomes |
| refusals changed | accepted now: an export's `Int` arithmetic, a result `Int` above the word (finding 27, JavaScript's `medium`), `diff(a ∈ Nat, b ∈ Nat) (Int)` (finding 94), `print_int` of any integer, chapter 7's hash64 printed; refused now: windows' unbounded entry `Int`; a map read with a key of any size (finding 111, `lookup(k ∈ Int)`, accepted before as a word); messages: `big` shown as `Int`, `io.print_int`'s refusals named inside it (finding 110) |
| compiler lines | +522 −34 in 14 files outside tests (`ir/big.go`, `emit/wherebounds.go`, `ir/keep.go`, `ir/lower.go`, `ir/interval.go`, `emit/bigrep.go`, `ir/golang`, `ir/pipeline.go`, `core/`); tests +88 −57 and two new files (`intboundary_test` with eight planted faults, `keptany_test`) |
| language lines | 222 + / 166 − in targets and lib; the book's chapter 0 rewritten (24 sources, 13 new) and 9 new or changed sources in chapters 6, 12, 13, 14, 16 |
| documents | the research note; ADR 0147; the book's chapters 0, 3, 5–8, 10, 12–14, 16, 17, 19 and findings 27, 94 answered and 108–111; integers.md §1a, ir.md's `where` row, inventory.md (`decimal`); CLAUDE.md, README; this |
| rule made less general | none: a `where` assumes more, a kept definition inlines in one more case (an untyped integer parameter), as P4 does |
| not built | an intermediate whose every consumer is bounded (the word midpoint's `lo + hi`, finding 108), a question for hamza; a binder's `sum ∈ Int` as a demand; a map read with a key of any size answering `'none` (finding 111); a definition's claim checked at its call (finding 110); U₆₄ ↪ ℤ (a result `Int` from an unsigned parameter, still refused on Go and the JVM); `-checked` on windows trapping a printed value the other targets compute (finding 112); a literal past JavaScript's word as a `BigInt` (the logic midpoint's 2⁶², still refused there, its message now the literal's); a big comparison against a word hoisted out of a loop |
