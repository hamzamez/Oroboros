# The book

Chapters, in order. Each is written for someone learning the language — human or model — and
leans on examples rather than explanation.

**Where the book stands, 2026-11-15.** The surface is J ([spec/j.md](../spec/j.md), ADR 0068). The
book introduces the language **one brick at a time**: a chapter uses only what the chapters before
it introduced, opens with the question the last one left, and shows a thing refused before it
shows it working (hamza, 2026-11-15: the order of Feynman's lectures, where nothing is used before
it is explained). Chapter 0 is the exception, a tour that may show everything.

| | chapter | the one idea | state |
|---|---|---|---|
| 0 | [A tour](00-tour.md) | the map | J; 58 blocks checked 2026-11-10 |
| | **I. How a program is reduced** | | |
| 1 | [`fn`](01-fn.md) | β; reduction is compilation | J; 62 blocks checked 2026-11-15 |
| 2 | [Definitions](02-def.md) | δ; a name is an abbreviation | J; 71 blocks checked 2026-11-15 |
| 3 | [Modules](03-modules.md) | where a name lives; the target can supply it | J; 55 blocks checked 2026-11-16 |
| 4 | [Choice](04-choice.md) | `Bool`, `if`, guards; a known condition keeps one branch | J; 54 blocks checked 2026-11-16 |
| 5 | [Effects](05-effects.md) | what β may not drop, copy or move; a statement is a binding nobody reads | J; 40 blocks checked 2026-11-15 |
| | **II. What a program computes with** | | |
| 6 | [Integers](06-integers.md) | ℤ, a range is a set, the word, proven or refused, above the word | J; 87 blocks checked 2026-11-16 |
| 7 | [Bits](07-bits.md) | the bits are the integer's; wrapping is `mod 2ⁿ` | J; 61 blocks checked 2026-11-16 |
| 8 | [Numbers with a point](08-numbers.md) | exact decimals; `round` the one place precision is lost; `F64` | J; 73 blocks checked 2026-11-16 |
| 9 | Text | strings as sequences, `++`, the host's strings, bytes | not written |
| 10 | Tuples and variants | "and" and "or" of data; `case` with patterns | not written |
| | **III. Repetition and storage** | | |
| 11 | `loop` | the one iteration; termination | not written |
| 12 | Tables | a table is a function on `0..len-1`; an index is proven; fusion | not written |
| 13 | State | `var`, buffers, maps; why a table never changes under you | not written |
| 14 | Failures | `Result`, `try`, `expect`, `relevant` | not written |
| | **IV. Claims and hosts** | | |
| 15 | [Claims](15-claims.md) | how a header's sets are proven: intervals, facts, obligations | **the notation J replaced**, and its examples did not run even in it; see its banner |
| 16 | Targets | host modules, writing a declaration, portability computed | not written |
| 17 | A whole program | one real program read line by line | not written |

Nothing specified and not built gets a chapter (records, callbacks, concurrency, `spec` and
`lemma`); each is named in a line where it would belong.

```bash
go run docs/book/check.go docs/book/04-choice.md      # from the repository's root; any written chapter
```

[check.go](check.go) pairs each fenced block with the directive on the line before it (an HTML
comment naming a file, a `gen`, an `oro`, a `build`, a `run` or a `portable`), runs it, and compares; a block
with no directive is counted and reported, not passed. It was trusted after planted faults failed:
seven in chapter 0, six in chapter 1, six in chapter 2, seven in chapter 3, seven in chapter 4, nine in chapter 5, ten in chapter 6, eleven in chapter 7 and fifteen in chapter 8, covering each kind of directive. `cmd/check`
does not run it.

**A reduction is printed in J**: `oro -j` writes each export's normal form in the surface's
notation (`core.ShowTerm`, [spec/j.md §6.1](../spec/j.md)), where `oro` alone writes the core term.
**A source the reader refuses is not `.oro`**: `core`'s `TestEveryCorpusFileReadsInJ` requires every
`.oro` file in the repository to read, so an example whose refusal is the reader's ends in
`.oro.txt`. A `no-….oro` reads, and is refused by a later stage.

**Chapters 1–5 teach reduction**, and their examples run against
[`targets/tutorial.oro`](../../targets/tutorial.oro), a target that exists to teach: it spells
arithmetic `+ - * /` so a chapter need not also be about module paths, and it declares `f`, `g`,
`h`, `x`, `y`, `z` as primitives so reduction halts exactly where a lesson wants it to.

```bash
go run ./cmd/oro -target=tutorial FILE.oro
```

That a teaching target is *possible* is the thesis in miniature: the normal form is a parameter, so
choosing where reduction stops is choosing a target.

Chapter 0 uses the real targets, not the teaching one: its sources are in [code/tour/](code/tour/),
compiled with `cmd/gen`, built and run with `cmd/build`, and measured with `cmd/portable`. A file
named `no-…` is meant to be refused, and the chapter prints the refusal. The chapter it replaced was
about s-expressions, the notation before J, and is in git at `a50bd47`.

Chapter 15, as it was written, uses `cmd/gen` and `cmd/build` rather than `oro`, because the checker lives at the
emission boundary and the teaching target emits nothing. That is the lesson, not a workaround.

Chapter 3's library files are real and live in [code/](code/). Its examples run against two
targets — `tutorial` and `tutorial-native`, which is the same file plus one capability — because
the four cells cannot be shown with only one.

## The 2026-10-06 verification pass

A harness re-ran every paired source/output block in the book against the compiler. **Chapters 1–4
had drifted in 38 places**, and the drift had one dominant cause and three others:

| cause | what changed | where |
|---|---|---|
| **integer folding** | `(* 7 7)` now reduces to `49`. A fold is a promise that compile time and run time agree bit for bit ([ADR 0009](../decisions/0009-staging-preserves-results.md)), and the teaching targets declare a word, so they fold | 31 outputs across chapters 1–4 |
| **hygiene** | the residual's binders are named apart, so a shadowed `n` comes back `n1` and a second `_` comes back `_1` ([ADR 0036](../decisions/0036-a-residuals-binders-are-named-apart.md)) | chapter 1 §1.5, chapter 4 §4.9 |
| **booleans became literals** | `true` and `false` are term kinds, not names ([ADR 0017](../decisions/0017-booleans-are-in-the-language.md)), so chapter 2's Church-boolean section was **refused outright**, not merely different | chapter 2 §2.11 |
| **the portable layer was deleted** | `num/f64` and `num/int` are gone ([portable-2026-09-29](../../gauntlet/results/portable-2026-09-29.md)), so chapter 2's two-target showcase on `-target=go` and **all of chapter 5** stopped running | chapter 2 §2.7, chapter 5 |

What the repair taught, and it is the reusable part:

> **Folding hides exactly the lessons this book is about.** δ duplicating a definition, β sharing an
> argument, one source specialising to two residuals — each is a claim about the *shape* of what
> reduction leaves, and each is invisible when the shape is a number. The teaching targets supply
> `h`, `x`, `y` and `z` precisely so a lesson can show a shape, and the repaired chapters use them
> wherever the shape is the point. Chapter 2's `(def one 1) (def two (+ one one)) …` reduced to `4`
> and taught nothing; with `h` it reduces to `(h (h 1 1) (h 1 1))` and `two` is visibly there twice.

Five more findings, from running things rather than reading them:

1. **`targets/tutorial-native.oro` was mojibake.** Three comment lines held `â€¦`, `â€”` and `Î²` —
   UTF-8 bytes re-encoded as if Latin-1. Repaired. A comment carries no meaning
   ([ADR 0024](../decisions/0024-comments-are-erased.md)), which is exactly why nothing had noticed.
2. **Chapter 3's `noisy-lib.oro` did not exist.** The chapter showed a library file inline and the
   README claimed its files were real; it was the one that was not. Now
   [code/noisy-lib.oro](code/noisy-lib.oro) and [code/noisy-prog.oro](code/noisy-prog.oro).
3. **An impure primitive's application is folded anyway.** On
   [targets/tutorial-sloppy.oro](../../targets/tutorial-sloppy.oro), where `*` is declared impure,
   `(* 1 2)` reduces to `2` and `((fn (n) 7) (* 3 4))` to `(let 12 (fn (n) 7))` — the binding kept
   because the primitive is impure, the call itself evaluated away because the fold is a property of
   the language's integer operators and does not consult the purity bit. The `let` left behind binds
   a constant. Recorded in chapter 4 §4.10 as a question, not fixed: nothing in the corpus declares
   arithmetic impure, and which of the two decisions should give way is a design call. **Answered
   2026-11-15** ([ADR 0114](../decisions/0114-a-fold-is-an-equation-between-pure-terms.md)): a fold
   is an equation between pure terms, so `(* 1 2)` stays a call there, and the rewritten chapter
   (now 5, §5.13) shows it.
4. **The closure refusal prints `#1.0`.** `gen: clo-mk: lowering: a λ in value position is a closure
   (callbacks.md): (fn (v) (* v #1.0))` — an internal bound-variable spelling in a user-facing
   diagnostic. Two of chapter 2's diagnostics had the same bug and were fixed when it was written;
   this one is in the IR's lowering and is still there.
5. **`core-0.md §2` says "six term kinds".** There are seven — `true`/`false` is the one it predates,
   and its grammar's `literal` production omits them. [state.md §1](../spec/state.md), which is read
   off the code, says seven. Not edited here, because core-0 is a spec with its own conventions.

The harness itself is three dozen lines and worth keeping: it pairs each ```` ```lisp ```` source
block with the following block that begins `⟶`, runs the source, and diffs. It would have caught
all 38 at any point in the last fortnight, which is the argument for running it when a chapter is
touched.

## For the writer

The specifications are in [docs/spec/](../spec/) and are the authority. A chapter must not
contradict one, and where a chapter simplifies it should say so. Writing chapter 1 found two real
bugs — a parameter list could repeat a name, and could bind a qualified one. Chapter 2 found four
more: `(def a.b …)` was accepted and unreachable, an `export` or a `sig` naming nothing was
silently dropped, and two diagnostics printed `#1.0` instead of the names the source used. That is
six bugs from two chapters. Reviewing the finished chapter then produced a seventh finding and an
ADR: recursion reduced but could not build, which is a promise the language does not keep, so it
is now rejected outright (ADR 0014).

Chapter 3 found four more: a library file's extra modules were visible only after something else
imported it — load-order-dependent meaning, which is the one thing a module system exists to
prevent — and three diagnostics named the wrong half of a qualified name. Chapter 4 found one
more, and a good one: the emitter recomputed a statement's value instead of binding it, so
`fmt.Println((strings.Fields(s)))` was followed by `return (strings.Fields(s))` — the exact
duplication the chapter is about, one layer below the chapter.

Chapter 5 found two. A `sig` was checked in both directions and then **thrown away**, so a program
whose claim had just been verified was refused for want of a type the claim stated. And an
assumption outside the decidable fragment was dropped rather than propagated, so the diagnostic
reported `known: nothing` about a program that plainly declared a `where`. Thirteen bugs from five
chapters.

The tour (chapter 0, 2026-11-10) found seven, by writing each example the way a newcomer would
first. None is fixed here; each is a defect or a question for its own step. **Planned 2026-11-15**
(README.md's "Current work"): the wrong answers, 1, 2, 10, 15, 17, 19, 20 and the 10-06 pass's 3,
were its item 3, and are **fixed 2026-11-15** ([bookfix-2026-11-15](../../gauntlet/results/bookfix-2026-11-15.md),
each with a planted fault); the surface's promises and the diagnostics, 3–7, 9, 11–14, 16, 21 and
the 10-06 pass's 4–5, were its item 4, and are **fixed 2026-11-15**
([surface-2026-11-15](../../gauntlet/results/surface-2026-11-15.md)), except 7's range payload, refused
now in J until a constructor has a contract. 8 and 18 were fixed with their chapters.

1. **`len(s)` of a `String` compiles, to three different answers**: Go's `len` (bytes), and `.length`
   on JavaScript and Java (UTF-16 units). [state.md §3](../spec/state.md) says string length is not in
   the language because the hosts disagree. `s[0]` is likewise taken as an indexing with an
   obligation, not refused as text.
2. **A dropped `relevant` value is accepted when a definition of the program made it**:
   `half(n); 0` and `r = half(n); 0` compile, with `half` returning a `result.Result`. A host call's
   result dropped the same way is refused (`examples/errors/discard.oro`).
3. **A big value passed to `io.print_int` is refused by the IR's verifier in its own words**:
   `IR_P does not verify: W5: %14 is big and flows into io.print-int's argument 1, which is int`.
4. **`x := e` on a name bound with `=` is refused as a store into a table**: *a store writes into 1,
   which is a frozen table*, where the fault is that `x` is not a `var`.
5. **`-n` on a variable is refused** (`` `-` takes 2 argument(s), given 1 ``), though j.md §3 lists
   prefix `-`; only a literal's sign reads.
6. **A table by a rule as an export's result is refused in the IR's words**, *a table rule that was
   never allocated*, with `#0.0` in the term it quotes; bound with `var` it compiles.
7. **Three diagnostics quote the notation J replaced or an internal name**: a variant's payload that
   is a range (`` `(ok int)`; got (circle (int 0 1000)) ``), the closure refusal's `#1.0` (finding 4
   of the list above, still there), and the word-overflow hint's `acc + t(i)` for `t[i]`.

Chapter 1 in J (2026-11-10) found one in the reducer, fixed with the chapter, and three more left
as they are:

8. **β dropped a binding that was read, and left its variable unbound** (fixed). In
   `fn(n) { fn(m) { fn(n) { h(m, n) } }(n) }(x())`, the chapter's capture example, the residual was
   `fn(n1) { h(n, n1) }` with no binding of `n`. `occurrences` opened each λ by its hints and read a
   parameter spelled like the name as shadowing it, so β's recount counted no use and dropped the
   binding as pure and unread. It now reads the closed body, as `subst` does
   (`TestABindingIsKeptUnderABinderOfItsSpelling`); emission is byte-identical. The lesson is
   CLAUDE.md's own, *a context keyed by name is captured by the name alone*.
9. **A telescope in a program's header is refused**, `fn at(a ∈ Array(Int), i ∈ 0..len(a)-1)`:
   *sig at: (int 0 (- (len a) 1)) is not a type*, in the notation J replaced. j.md's status lists a
   telescope as built.
10. **A result declared `()` accepts a body that yields a value**: `export fn nothing(n ∈ 0..9) () { n }`
    compiles and returns `n`.
11. **The step limit's message quotes a core term**: *last term head: (fn () (fn (k) k))*.

Chapter 2 in J (2026-11-15) found three, none fixed:

12. **A pure binding nothing reads is left in the residual.** With `fn pair(a, b) { fn(sel) { sel(a, b) } }`
    and `fn snd(p) { p(fn(a, b) { b }) }`, `snd(pair(x(), y()))` reduces to `{ a = x(); y() }`: the
    argument was bound because its use stood under a λ, and the binding is kept after the use
    reduced away. The same before and after item 8's fix.
13. **A constant in a header's range is refused**, `export fn small(n ∈ 0..1000) (0..cap)` with
    `cap = 100`: *sig small: (int 0 cap) is not a type*. The same refusal as item 9's telescope: a
    range's end in a program's header takes literals only.
14. **A top-level value with a claim is refused by the reader**, `cap ∈ 0..100 = 64`: *"cap" begins
    no declaration*. j.md §2.2 gives `value_decl ::= binder "=" …`; only a target file's host
    constant reads so.

Chapter 3 in J (2026-11-15) found three, none fixed:

15. **A declaration outside the `module` blocks of a file that has them is read into the last
    block.** `module inner { … }` followed by `export fn run(u) { inner.k(u) }` names the export
    `inner.run` and refuses it, *inner is not imported*; written before the blocks, it is refused as
    *module inner exports run, which it does not define*. A block's `{ }` closes its scope in J's
    grammar (j.md §2.1), and the core's `(module X)`, which D makes of it, runs to the next one.
16. **A module file with `_` in its name cannot be used.** `use zz_b7` looks for `zz-b7` (σ_J⁻¹), so
    `zz_b7.oro` is unreachable, and the refusal suggests the `use` that fails.
17. **`use Geometry` finds `geometry.oro` on Windows**, whose file system ignores case; the same
    program is refused where it does not.

Chapter 4, Choice (2026-11-15), found one in the reader, fixed with the chapter, and three left:

18. **A guarded choice with no `else` read its last guard as one, and an arm after `else` crashed
    the reader** (fixed). `case { c -> 1; d -> 2 }` answered 2 where neither guard held, and
    `case { else -> 2; c -> 1 }` was a nil dereference in D's chain. The reader checked W4 for a
    loop's arms and not a `case { … }`'s; it now does (`TestAGuardedChoicesLastArmIsElse`);
    emission is byte-identical, so no corpus program relied on it.
19. **A one-armed `if`'s unit passes as an integer.** `export fn f(c ∈ Bool) (Int) { if c { 1 } }`
    compiles, and returns 0 where `c` is false. The same family as item 10.
20. **`true = 1` at a file's top level is accepted and ignored.** The literal stays `true`.
21. **The equality refusal names a deleted host operation**: *name it: go.`==`, js.`===`,
    java.`==`, x64.sete*, and `x64` went with MASM.

Chapter 5, Effects (2026-11-15), found none in the compiler and leaves one question, **decided and
built 2026-11-15** ([ADR 0116](../decisions/0116-a-statement-whose-value-is-a-function-is-refused.md),
[chapters56-2026-11-15](../../gauntlet/results/chapters56-2026-11-15.md)): the statement is refused,
`ignore(f)` its discard:

22. **A function named with no parentheses, on a line of its own, vanishes without a word.**
    `io.print_line` between two prints compiles and prints two lines; `shout; 7` reduces to `7`.
    It follows from the rules (a function is a value, a pure statement is dropped), and it is the
    likeliest mistake a newcomer from C makes. A question, not a defect: should a statement whose
    value is a function be refused, as Go refuses *f evaluated but not used*? Chapter 5 §5.11
    teaches it as a trap.

Chapter 6, Integers (2026-11-15), found seven, none fixed; the chapter avoids each. **Planned
2026-11-15** with chapter 5's (README.md's "Current work", item 5), and **fixed 2026-11-15**
([chapters56-2026-11-15](../../gauntlet/results/chapters56-2026-11-15.md), each with a planted
fault): 23–28 build or are refused for their true reason, and 29 is deferred until a program needs
it. Read against the decisions, 27
and 28 are defects, not questions: ADR 0033 refuses Go's version of 27, and ADR 0079 item 4 decides
28 (a value above the word reaches the word where its fact is inside it):

23. **A product of naturals is not proven a natural.** With `fn square(n ∈ Nat) (Nat) { n * n }`,
    `square(square(10^20))` is refused: *square requires 0 <= { n1 = 100000000000000000000;
    the("int 0 +inf", n1 * n1) } at a call, which does not follow*. The obligation is nonlinear,
    the fact (a square is not negative) is not derived, and the message prints a core term.
24. **An export with no declared result may not yield a tuple of computed values.**
    `export fn ops(a ∈ 0..100, b ∈ 1..100) { (a + b, a - b) }` is refused, *declares 1 result(s)
    and does not produce them: the body yields 2*, while a tuple of literals with no declared
    result is accepted and returns several values.
25. **An empty range's refusal is in the notation J replaced**: `n ∈ 9..0` gives *sig emp: (int 9
    0) is not a type*. The same family as items 4, 7, 11 and 21.
26. **Two of JavaScript's refusals say the wrong thing.** `export fn third(n ∈ 0..2^60) (Int)
    { n / 3 }`: *an operand of div, [0, 1152921504606846976], may be negative*, of an interval
    that starts at 0. And `(lo + hi) / 2` over `0..2^63-1`: *the result of `>>` lies in [0,
    2147483647]*, an interval the program does not have. Both refusals may be right; their
    reasons are not the ones printed.
27. **On JavaScript a declared `Int` result can be a `BigInt`.** `export fn medium(n ∈ 0..2^31)
    (Int) { n * n }` is accepted there, computed as a pair of 32-bit digits and returned as a
    `BigInt`; `(n ∈ 0..2^53-1) (Int) { n + 1 }` likewise. On Go the analogous program, a result in
    the unsigned word declared `Int`, is refused (*held in the UNSIGNED word, not an `Int`*). A
    question: which of the two is the rule at an entry point?
28. **A value above the word cannot come back into it.** `square(n) / 2^40` with `n ∈ 0..2^40`
    and a result declared `0..2^40` is refused as *WIDER than the word*, though its value fits.
    The message spells `big-str` (it is `big_str` in J) and cites `docs/unbounded-rung.md`. A
    question for the plan: a narrowing a claim could consent to, as a claim consents to the
    widening.
29. **No digit separator, and no binary literal**: `1_000_000` and `0b101` are refused by the
    reader as two tokens. A question, not a defect; j.md does not promise either.

Chapter 7 (2026-11-16) found seven, none fixed, each avoided in the chapter:

30. **A file named after a target, beside the source, is read as that target.** With a `js.oro`
    in the program's directory, every build of a neighbour for `js` fails: *js.oro: line 1:
    "export" begins no form of a target file*. The program's directory is searched for the target
    before `targets/`. The chapter's file is `narrow.oro` for that reason.
31. **JavaScript's refusal of a bit operation on an unbounded operand gives a reason that is
    false.** `export fn low(n ∈ Int) (0..255) { n & 255 }`: *the result of `&` lies in [0, 255],
    and target js computes `&` only where the result is proven in one of -2147483648..2147483647,
    0..4294967295*. [0, 255] is inside both. With `n ∈ -2^63..2^63-1` the message is the true one,
    *an operand of and is held in arbitrary precision*. The family of item 26.
32. **An unsigned 64-bit value at a JavaScript export is refused for a `>>` the program does not
    have.** `export fn mix(a ∈ 0..2^64-1, b ∈ 0..2^64-1) (0..2^64-1) { a ⊕ b }`: *the result of
    `>>` lies in [0, 4294967295], and target js computes `>>` only where …*. The refusal may be
    right (JavaScript has no such number to pass); the reason is the pair expansion's, not the
    program's. Inside a program the same value works, and chapter 7's `hash64.oro` runs there.
33. **A value in the unsigned word cannot be printed whole.** `io.print_int(h)` with
    `h ∈ 0..2^64-1` prints on windows, and on Go is refused by the verifier in its own words,
    *IR_P does not verify: W5: %13 is u64 and flows into io.print-int's argument 1, which is int*
    (item 3 fixed this for a big value, not for U). `big_str(h)` is refused on all three as
    *WIDER than the word*. The chapter prints `h >> 32` and `h & 0xFFFFFFFF`.
34. **`mod` of a value in the unsigned word is refused where `%` compiles**, on Go:
    `(a * b) mod 7` with `a, b ∈ 0..2^32-1` is *held in the UNSIGNED word, not an `Int`*, and
    `(a * b) % 7` is accepted. And `(a * b / 3) mod 2^64` over `0..2^64-1` is refused with the
    advice *reduce it with `mod 2^64`*, which the program does.
35. **A shift count declared `Nat` is refused**, *an operand of shr is held in arbitrary
    precision*, though integers.md §0b gives `>>`'s count no upper bound; `k ∈ 0..1000` compiles.
    A question: a count past the word is decided by saturation, so the count's representation
    need not matter.
36. **Two small ones.** There is no way to print a `Bool` (`io` has no `print_bool`; the chapter
    defines `bit`). And `export fn f(k ∈ 0..64) (0..2^64) { 1 << k }` is refused with `big-of` in
    the message, a core name (it is the family of item 28's `big-str`).

Chapter 8 (2026-11-16) found eight, none fixed, each avoided in the chapter:

37. **`÷` on exact numbers does not compile for Go or windows.** `round(Int, c ÷ 3, 'down)` with
    `c ∈ 0..10^9`, and `round(D(2, 0..100), 100 ÷ 3)`, are refused by the printers: *the target
    has no spelling for neg* (Go), *for (neg exact)* (windows). JavaScript and the JVM compile
    them. So the chapter's two `÷` programs on decimals, `split.oro` and `quotient.oro`, run on
    JavaScript only, and the chapter says so. `÷` under a rounding onto `F64` is unaffected.
38. **JavaScript prints `--7` for a rounded quotient of a negative literal.**
    `round(Int, -7 ÷ 2, 'zero)`, or the same through a function called with `0 - 7`, emits
    `v = (--7);`, which is not JavaScript; `build` catches it only because it runs `node --check`.
    The chapter divides positive literals.
39. **Three comparisons of one folded double do not verify.** `x = round(F64, 0.1)` followed by
    `x > 0.1`, `x < 0.1` and `x == 0.1` in one `main` is refused with *IR_P does not verify: W6: π
    … which its arm's guard %6 = false does not imply (Theorem E)*. Any one of the three alone
    compiles and answers correctly.
40. **A double compared with a rounded decimal crashes the compiler.**
    `round(F64, 1 ÷ 3) < round(D(18, 0..1), 1 ÷ 3)` on JavaScript: a nil dereference in
    `core.showAt`. numbers.md §6.2 says this comparison is refused.
41. **`round` onto `Nat` or `0..`.** `round(Int, 5.5)` and `round(0..10, 5.5)` are integers.
    `io.print_int(round(Nat, 5.5))` is refused with *a rounding is N, but Int is required here*,
    and `x = round(0.., 5.5)` with *round onto Nat: a rounding's set is a grid restricted to an
    interval, D(k, lo..hi) or a range*, though `Nat` is a range.
42. **`num.decimal` on JavaScript, and a numerator past 2⁵³.** `num.decimal(x, 18)` is refused
    there: *mod of a value held in the unsigned word as a pair, by 10, not a power of two: not
    built*; small `k` works, so `sum.oro` and `ring.oro` run on Go and windows. And a decimal
    whose numerator passes 2⁵³ (`n * 0.01 + 0.005 < 3`, `n ∈ 0..10^15`) is refused on JavaScript
    as *a comparison of a value held in the unsigned word as a pair is not built*, not as an
    operation outside the word, so `portable` reports a wall where the reader expects a width.
43. **A double cannot be printed.** `num.decimal(round(F64, 0.1), 3)` is refused with
    `num.digits`'s internal obligations (*requires 0 <= a / 1000 …*), and no library renders a
    double (the gauntlet's wall G5). The chapter compares doubles with exact numerals instead.
44. **Messages.** `3.5 / 2` and `2.0 << 1` say *a float literal is F64, but Int is required
    here*: a literal is an exact number (numbers.md §1), and the same refusal on `7 * 0.5` says
    N. The doubles' refusals name parameters `#0.0` and `#0.1` (item 7's closure names). A bare
    `1 ÷ 3` passed to `num.decimal` is refused with `num.digits`'s obligation instead of the `÷`
    rule. `0.5 + 0.5` at `io.print_int` on windows is refused inside the printer's own loop: *in
    again's argument 1: in a store's value: … m is N*. And `round(F64, 1e400)` prints all 401
    digits of the literal.

One thing compiles that the specification says is not built: `export fn f(p ∈ D(2, 0..1000))`
takes `p` as its numerator, an `int`, on Go (numbers.md §9: "D at an entry point's boundary"
is not in step 1). The chapter's exports take integers and doubles only.

**The chapters after the fixes (2026-11-16).** With items 1–28 fixed, every chapter was re-run
(all nine pass) and the passages that worked around a fixed item were rewritten, seven planted
faults caught in the new blocks:
- 5: the tour's `abs` and chapter 6's `magnitude` write `-n`, and the two notes saying a name
  cannot be negated are gone;
- 13, 14, 25: chapter 6 §6.3 shows the empty range's refusal, a constant as a range's end, and
  a constant carrying its own claim (`limit.oro`, `no-limit.oro`, `no-empty.oro`);
- 15: chapter 3 §3.6 says a block's scope ends at its brace and shows the `use` a declaration
  after the blocks needs (`after-blocks.oro`, `no-after-blocks.oro`), in place of the defect's
  note;
- 19: chapter 4 §4.9 shows a one-armed `if` refused at an `Int` (`no-one-arm.oro`);
- 22: chapter 5 states the refusal in the present tense.

Doing that found three more, none fixed:

45. **A claimed constant is checked only where it is read.** With `half ∈ 0..cap = cap * 2`,
    a program that never reads `half` compiles; one that reads it is refused under the reader's
    name, *integers-near-half: the claim half ∈ 0..100 is false*. ADR 0072 decides a declared
    result at its definition; a constant's claim is the same kind of statement.
46. **A claimed constant is not folded.** `half - 5 <= n` with `half ∈ 0..cap = cap / 2` emits
    `v11 := (50 - 5)` on Go, where the unclaimed constant gives `45`.
47. **A refusal names a definition in the core's spelling**: `export fn one_or_nothing` is
    refused as *gen: one-or-nothing: the unit () is (), but Int is required here*. The family
    of items 4, 7 and 21.

A chapter is also allowed to teach something that is not about this language. Chapter 2's last
section is Church encodings, because the compiler's own vector type turns out to be one; chapter
3's is ML functors, because it turns out we already have them and did not notice; chapter 4's is
Boole's algebra and partial evaluation; chapter 5's is referential transparency, thunks, and
substructural logic, because the effect rule derived from three concrete hazards turns out to be
Lambek's ordered fragment, named in 1958; chapter 7's is positional notation, the adder's identity
and Gauss's congruences, because "the machine may wrap early" is the statement that a remainder
is a ring homomorphism; chapter 8's is which fractions terminate in a base, fixed point against
floating point, the bias of rounding a half upward, and what IEEE 754's arithmetic is once it is
written out: a rounding after every operation.

Chapter 5 also uses [targets/tutorial-sloppy.oro](../../targets/tutorial-sloppy.oro) —
`targets/tutorial.oro` with the word `pure` deleted from one primitive — so the cost of a
forgotten purity marker can be shown rather than asserted.
