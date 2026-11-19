# The book

Chapters, in order. Each is written for someone learning the language — human or model — and
leans on examples rather than explanation.

**Where the book stands, 2026-11-20.** The surface is J ([spec/j.md](../spec/j.md), ADR 0068). The
book introduces the language **one brick at a time**: a chapter uses only what the chapters before
it introduced, opens with the question the last one left, and shows a thing refused before it
shows it working (hamza, 2026-11-15: the order of Feynman's lectures, where nothing is used before
it is explained). Chapter 0 is the exception, a tour that may show everything.

**The book moves with the language** (hamza, 2026-11-20). A change to the language, a construct, a
word, a meaning, a refusal that a program can meet, updates the chapter that teaches it in the same
step: its examples written, run and checked, and its "what is not here" made true again. Where no
chapter teaches it yet, one is written or split, and the chapters renumbered. `cmd/check`'s book step
runs every chapter, so a chapter that no longer says what the compiler gives fails the check.

| | chapter | the one idea | state |
|---|---|---|---|
| 0 | [A tour](00-tour.md) | the map | J; 73 blocks checked 2026-11-20 |
| | **I. How a program is reduced** | | |
| 1 | [`fn`](01-fn.md) | β; reduction is compilation | J; 62 blocks checked 2026-11-20 |
| 2 | [Definitions](02-def.md) | δ; a name is an abbreviation | J; 71 blocks checked 2026-11-20 |
| 3 | [Modules](03-modules.md) | where a name lives; the target can supply it | J; 55 blocks checked 2026-11-20 |
| 4 | [Choice](04-choice.md) | `Bool`, `if`, guards; a known condition keeps one branch | J; 54 blocks checked 2026-11-20 |
| 5 | [Effects](05-effects.md) | what β may not drop, copy or move; a statement is a binding nobody reads | J; 40 blocks checked 2026-11-20 |
| | **II. What a program computes with** | | |
| 6 | [Integers](06-integers.md) | ℤ, a range is a set, the word, proven or refused, above the word | J; 87 blocks checked 2026-11-20 |
| 7 | [Bits](07-bits.md) | the bits are the integer's; wrapping is `mod 2ⁿ`; the same past the word | J; 66 blocks checked 2026-11-20 |
| 8 | [Numbers with a point](08-numbers.md) | exact decimals; `round` the one place precision is lost; `F64` | J; 73 blocks checked 2026-11-20 |
| 9 | [Text](09-text.md) | strings as sequences, `++`, the host's strings, bytes | J; 69 blocks checked 2026-11-20 |
| 10 | [Tuples and variants](10-data.md) | "and" and "or" of data; `case` with patterns | J; 46 blocks checked 2026-11-20 |
| 11 | [Records](11-records.md) | a record is a function on its labels; `with`; the zero | J; 47 blocks checked 2026-11-20 (new) |
| | **III. Repetition and storage** | | |
| 12 | [`loop`](12-loop.md) | the one iteration; termination | J; 59 blocks checked 2026-11-20 |
| 13 | [Tables](13-tables.md) | a table is a function on `0..len-1`; an index is proven; fusion | J; 65 blocks checked 2026-11-20 |
| 14 | [State](14-state.md) | `var`, buffers, maps, `inout`; why a table never changes under you | J; 69 blocks checked 2026-11-20 |
| 15 | [Failures](15-failures.md) | `Result`, `try`, `expect`, `relevant` | J; 44 blocks checked 2026-11-20 |
| | **IV. Claims and hosts** | | |
| 16 | [Claims](16-claims.md) | how a header's sets are proven: intervals, facts, obligations | J; 66 blocks checked 2026-11-20 |
| 17 | [Targets](17-targets.md) | host modules, writing a declaration, portability computed | J; 23 blocks checked 2026-11-20 |
| 18 | [Calling C](18-c.md) | a C function by its prototype; structs are records; a window procedure | J; 22 blocks checked 2026-11-20 (new) |
| 19 | [A whole program](19-whole.md) | one real program read line by line | J; 24 blocks checked 2026-11-20 |

Chapters 11 and 18 were added 2026-11-20, and the chapters after each renumbered (the old 11–16 are
12–17, the old 17 is 19); the findings below name chapters by their new numbers. Nothing
specified and not built gets a chapter (callbacks a host calls during a call, concurrency, `spec`
and `lemma`); each is named in a line where it would belong.

```bash
go run docs/book/check.go docs/book/04-choice.md      # from the repository's root; any written chapter
```

[check.go](check.go) pairs each fenced block with the directive on the line before it (an HTML
comment naming a file, a `gen`, an `oro`, a `build`, a `run` or a `portable`), runs it, and compares; a block
with no directive is counted and reported, not passed. It was trusted after planted faults failed:
seven in chapter 0, six in chapter 1, six in chapter 2, seven in chapter 3, seven in chapter 4, nine in chapter 5, ten in chapter 6, eleven in chapter 7, fifteen in chapter 8, fourteen in chapter 9, twelve in chapter 10, thirteen in chapter 12, sixteen in chapter 13, fifteen in chapter 14, thirteen in chapter 15, thirteen in chapter 16, ten in chapter 17 and ten in chapter 19, covering each kind of directive. `cmd/check`'s
book step runs it over every chapter.

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

Chapter 16, as it was written, uses `cmd/gen` and `cmd/build` rather than `oro`, because the checker lives at the
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

The tour (chapter 0, 2026-12-10) found seven, by writing each example the way a newcomer would
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
    compiles and answers correctly. **Fixed** 2026-11-16 ([bookfaults-2026-11-16](../../gauntlet/results/bookfaults-2026-11-16.md)): one constant, ⌈q⌉, for all four relations, so a branch's π is its guard's literal.
40. **A double compared with a rounded decimal crashes the compiler.**
    `round(F64, 1 ÷ 3) < round(D(18, 0..1), 1 ÷ 3)` on JavaScript: a nil dereference in
    `core.showAt`. numbers.md §6.2 says this comparison is refused. **Fixed** 2026-11-16 ([bookfaults-2026-11-16](../../gauntlet/results/bookfaults-2026-11-16.md)): refused for its reason, the message no longer showing an absent term.
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
  a constant carrying its own claim (`limit.oro`, `no-limit.oro`, `no-empty.oro.txt`);
- 15: chapter 3 §3.6 says a block's scope ends at its brace and shows the `use` a declaration
  after the blocks needs (`after-blocks.oro`, `no-after-blocks.oro`), in place of the defect's
  note;
- 19: chapter 4 §4.9 shows a one-armed `if` refused at an `Int` (`no-one-arm.oro`);
- 22: chapter 5 states the refusal in the present tense.

Doing that found three more, none fixed:

45. **A claimed constant is checked only where it is read.** With `half ∈ 0..cap = cap * 2`,
    a program that never reads `half` compiles; one that reads it is refused under the reader's
    name, *integers-near-half: the claim half ∈ 0..100 is false*. ADR 0072 decides a declared
    result at its definition; a constant's claim is the same kind of statement. **Fixed**
    2026-11-18 ([deadclaims-2026-11-18](../../gauntlet/results/deadclaims-2026-11-18.md)): a claimed
    constant is a definition of no parameters, decided at its definition, read or not, and refused
    under its own name, *half: the claim half ∈ 0..100 is false: its value is 200*.
46. **A claimed constant is not folded.** `half - 5 <= n` with `half ∈ 0..cap = cap / 2` emits
    `v11 := (50 - 5)` on Go, where the unclaimed constant gives `45`.
47. **A refusal names a definition in the core's spelling**: `export fn one_or_nothing` is
    refused as *gen: one-or-nothing: the unit () is (), but Int is required here*. The family
    of items 4, 7 and 21.

Chapter 9, Text (2026-11-16), found five, none fixed, each avoided in the chapter:

48. **windows does not check `string_of`'s precondition.** `export fn any(n ∈ Int) (String)
    { string_of(n) }` and `wide(n ∈ 0..70000)` are refused on Go and JavaScript (*string_of's
    parameter c, declared in 0..1114111 …*, *requires n <= 55295 || n >= 57344*) and compile on
    windows, to a call of `@oro.string_of` on any i64. A soundness hole: a surrogate or a
    negative number reaches the encoder. The chapter's two refusals are shown on Go. **Fixed**
    2026-11-16 ([strings-2026-11-16](../../gauntlet/results/strings-2026-11-16.md), ADR 0117):
    windows' `string_of` is now a definition stating the scalars' domain, so both are refused there.
49. **A disjunction does not prove the hole.** `if n < 0xD800 || n > 0xDFFF { string_of(n) }`
    is refused, *requires n <= 55295 || n >= 57344 … known: … assumed (if (< n 55296) true
    (> n 57343))*: the assumption is the obligation, held as a core term and not matched. Two
    `case` arms, one per side, prove it, and the chapter's `hole.oro` is written so.
50. **`s[0]` on a string bound in a block is refused in the lowering's words**: with
    `s = "abc"`, *lowering: an application of "abc"*. On a `String` parameter it is item 1's
    message.
51. **Messages.** `++` is named `concat` (*in argument 2 of concat*). `utf8_text` of bytes that
    are not valid says *requires utf8_valid(`array`(230, 151)), which is outside the decided
    fragment*, where the literal was decided and is false. `ascii(array { 72, 200 }, 0, 2)`:
    *a literal table's element 200 lies outside its declared element 0..127, which decides at a
    boundary (literal-elements.md)*. `'single'` in place of a string: *`single'` is not bound*.
    One failing `string_of` call reports *(2 obligations at calls fail …)* or 3.
52. **A join of two literals is not folded**: `"report" ++ ".txt"` reaches Go as
    `("report" + ".txt")`, and `string_of(99)` as `string(rune(99))`. Go folds the first itself;
    on windows each is a heap allocation at run time.

Teaching, not defects: nothing in the language takes a `String` apart (no length, index or
equality; strings.md §4, state.md), so chapter 9 says so plainly, and its only counting and
searching is `go/strings` in §9.12, marked Go-only by `portable`. §9.10 uses `array { … }`
three chapters early, as a way to write bytes down, and says so.

Chapter 10, Tuples and variants (2026-11-16), found one wrong answer and five smaller, none
fixed, each avoided in the chapter:

53. **Two payloads in scope are one name: a wrong answer.** With `set Maybe(T) = 'just(T) ∪
    'nothing` and `fn digit_of(c ∈ 0..127) { if c >= 48 && c <= 57 { 'just(c - 48) } else
    { 'nothing } }`,

        case digit_of(a) {
            'nothing -> -1
            'just(x) -> case digit_of(b) { 'nothing -> -1; 'just(y) -> 10 * x + y }
        }

    computes `10 * (b - 48) + (b - 48)`: for the characters `4` and `2` it prints 22 on Go,
    JavaScript and windows, where 42 is right. `oro -j` shows the residual:
    `#p = … b - 48; 10 * #p + #p`. Each constructor's payload is let-bound under the one
    spelling `#p`, and the inner binding captures the outer use; binding the two results to
    names first (`p = digit_of(a); q = digit_of(b)`) changes nothing. The same program with
    tuples for the results is right. It is CLAUDE.md's *a context keyed by name is captured by
    the name alone*, in sums.md §5's recount. **Fixed** 2026-11-16 ([capture-2026-11-16](../../gauntlet/results/capture-2026-11-16.md)):
    a `case`'s binders are fresh for its clauses (Barendregt's convention, ADR 0036), so 42; item
    76's `try` chain gives 22. Chapter 10 uses one payload at a time; chapter 15
    (`try` chains two results) cannot be written around it.
54. **A variant's name is no set at a parameter.** `fn warm(c ∈ Colour)` is refused, exported or
    not: *its header's parameter c names colour, which is no set: not one of the language's and
    not declared by target go (ADR 0073)*. `(Colour)` as a result's set compiles. A tuple at a
    parameter, `p ∈ Int × Int`, is refused in the notation J replaced: *sig take: prod(int, int)
    is a product, and a product is an ELEMENT type — a TABLE of them is `(array (tuple int
    int))`*. The chapter's functions over variants take a parameter with no set.
55. **A `case` with a scrutinee needs `=`, which the tutorial target lacks.** `oro -j
    -target=tutorial` on `case s { 'circle(r) -> … }` with `s` unknown: *in area: `=` is not
    bound*. So the chapter shows a variant's reduction on Go's emitted code, not as a residual.
56. **A constructor's payload takes a set's name only**, by design (item 7), but a second
    payload is a reader's error with no reason: `'rect(Int, Int)` gives *line 1: "," where ")"
    was expected*.
57. **Unchecked shapes.** A pattern may omit a payload's binder, `'circle -> 1` for
    `'circle(Int)`, and compiles. `s = 'circle; 0` (a constructor never applied) compiles. A
    non-variant in an arm of a function whose result is declared a variant, `n == 1 -> 5` under
    `(Colour)`, is *lowering: an application of 5*; `case n { 'red -> … }` on an integer is the
    closure refusal with `fn(#t, #p)` in it.
58. **Messages.** A variant's name is printed in lower case (*case on light*, *variant list*).
    `set A = 'x ∪ 'y` and `set B = 'x ∪ 'z` in one module: *x is defined twice — sum b declares a
    variant of that name*. A pattern with two binders: *a clause pattern is a constructor, a
    constructor and one binding — `(ok v)` — or `else`; got (circle a b)*. `'circle(n, n)` and
    `(x, y, z) = (a, 1)`: *arity: expects 1 argument(s), given 2, in: fn(n) { fn(#p) { (0, #p)
    } }*. `p[0]` on a tuple: *lowering: an application of 0*. `(a, b) == (b, a)` and `'red ==
    'red`: *(#1.0, #1.1) is 2 values where one is wanted*. `'circle("big")` under a declared
    `(Shape)`: *p_f: its header declares result Int, and the body yields String*.

Also: `export fn unit(a ∈ 0..9) () { () }` emits a Go function returning `int`, `return 0`.

Chapter 12, `loop` (2026-11-16), found five, none fixed, each avoided in the chapter:

59. **On windows a `main` whose value is a loop's `()` does not lower.** `export fn main() {
    loop i = 1 { i <= 3 -> { io.print_int(i); again(i + 1) }; else -> () } }` builds on Go and
    JavaScript and on windows is refused: *lowering: an application whose operator is an
    application, and not an eliminator: loop row = 1 { … }(fn() { `windows/kernel32`.
    ExitProcess(0) })*. The run's continuation, exit(0) ∘ main, is applied to the loop. With a
    statement after the loop (`count.oro` ends in a `print_line` arm) it builds.
60. **`invariant` is not read.** j.md §2.8's grammar has `loop … invariant I { … }` and its
    status lists the loop as built; `loop i = 1, p = 1 invariant p <= N {` is *"invariant"
    where "{" was expected*. So a bound the interval analysis cannot find is stated as a guard
    that runs (§12.7's `p <= 4 * 10^17`), which is CLAUDE.md's "a clamp in a program is a
    missing proof", taught as the tool there is. **Fixed** 2026-11-16
    ([invariant-2026-11-16](../../gauntlet/results/invariant-2026-11-16.md)): `invariant I` is read
    and decided by Hoare's rule. §12.7's bound stays a guard, because p · i ≤ N does not follow
    from p ≤ N: the invariant that carries it is p = (i − 1)!, outside the linear fragment.
61. **A `case` on a loop's variant result must have arms of one type**, where the same `case`
    on a function's variant result need not: `'some(r) -> io.print_int(r); 'none ->
    io.print_line("no")` is accepted after `digit_of` (chapter 10's `maybe.oro`, where
    case-of-case removes it) and refused after a loop, *the branches of a conditional are Int
    and String*. Both are statements whose value nobody reads.
62. **`fib` and `fact` to the word's edge are refused on JavaScript for the wrong reason**:
    `b <= 4 * 10^18` there is *a comparison of a value held in the unsigned word as a pair is
    not built*, and an exported `fact` *a result is held in the unsigned word … declared Int*
    (item 42's family). The chapter runs them on Go and names no other target.
63. **The compiler's method for a product is the product of the bounds**, 20²⁰ for `fact(20)`,
    so `p * i` over `n ∈ 0..20` is refused though 20! fits. Not a defect (the result is a claim
    the analysis cannot make), and the chapter says so; recorded because the natural first
    loop after a sum is a product, and the reader meets the refusal on page two.

Teaching: termination is reported and not required (*n of m loop(s) proven terminating*), and
the chapter shows a stuck loop compiling with *0 of 1* beside the halting problem. A loop of
static trip count is not unrolled (`three()` emits a loop), which the chapter uses to say what
the dynamic level is.

Chapter 13, Tables (2026-11-16), found six, none fixed, each avoided in the chapter:

64. **A tuple pattern on a table's element emits code that is not Go.** `export fn width(p ∈
    Array(0..100 × 0..100)) … { (x, y) = p[i]; again(i + 1, s + x * y) }` emits
    `v15, ok14 := v0[v7]` followed by `v14 := 1; if ok14 { v14 = 0 }`: a map's comma-ok read of
    a slice, and `x` taken as its success flag. With `Array(Int × Int)` it is refused, *cannot
    determine a type for the value of (read …), of type any*. `p[i][0]` and `p[i][1]` are
    right, and the chapter reads pairs that way. **Fixed** 2026-11-16 ([bookfaults-2026-11-16](../../gauntlet/results/bookfaults-2026-11-16.md)): the product's eliminator is its projections, k·i + j.
65. **Two `each` composed are refused.** With `fn each(f, t) { array { f(t[i]) : i ∈
    0..len(t)-1 } }`, `total(each(f, each(g, t)))` is *an index into a table given by a rule
    requires #req("an index into a table given by a rule", "index", "int 0
    9223372036854775807", i) < len(t) at a call, which does not follow*, with `i < len(t)`
    among the facts it lists. One `each` fuses, and the same two rules bound to names in one
    body fuse (`chain.oro`). So the chapter's library section composes nothing.
66. **A table of tuples cannot be written, and a table of tables not at all.** `array { (1, 2),
    (3, 4) }` is *lowering: (1, 2) is 2 values where one is wanted*; `Array(Array(Int))` and
    `array { array { 1, 2 }, … }[i][j]` are *lowering: an application whose operator is an
    application, and not an eliminator*. So chapter 10's promised rectangle and list are two
    parallel tables, which the chapter presents as the struct-of-arrays layout it is.
67. **Division has no sign fact, so binary search cannot be written as it is taught.** With
    `0 <= lo`, `lo < hi` and `hi <= len(t)` all among the known facts, `t[lo + (hi - lo) / 2]`
    is refused, *0 <= lo + (hi - lo) / 2 does not follow*: x ≥ 0 ⇒ x / k ≥ 0 is not derived
    (products.md §6 added k·(x/k) ≤ x and no more). **The midpoint is proven** 2026-11-16 (store-2026-11-16): div_floor's guard
    holds only under the loop's guards, and lang's facts are now instantiated where an obligation
    is, so both `lo + (hi - lo) / 2` bounds follow; chapter 13's `no-search.oro` is refused now for
    its arithmetic in the word, and its passage says so. `(lo + hi) / 2`, `>> 1` and a length-based
    form fail alike. The chapter carries the midpoint as a third loop variable under a guard;
    its termination is then *0 of 1 proven* (a measure of two variables), which the chapter
    argues by hand. The same gap refuses `grid[r * w + c]` at `0 <= r * w + c`.
68. **Segments and `alloc` at a boundary.** `t[lo:hi]` as an export's result with `where lo <=
    hi && hi <= len(t)`: *1 table allocation(s) cannot be proven to have a size in [0, …]:
    tabulate size [-1000000, 1000000]: below 0*, the `where` unused. With `lo ∈ Int`: *`a
    segment b[lo:hi]`'s parameter lo, declared in 0..2^63-1, requires lo <= 9223372036854775807*.
    And `sq = alloc(array { i * i : i ∈ 0..99 }); sq[n]` with `n ∈ 0..99`: *n < len(sq) does not
    follow*. The chapter uses a segment inside a body and does not teach `alloc`.
69. **Messages.** An out-of-range literal index prints the core's table, *`array`(2, 3, 5, 7,
    11)[5] is an indexing*; a rule's fact prints as *i < (len t)*. `map` and `table` are names
    of Σ_L, so the two natural names for this chapter's helpers are refused (*the program
    defines `map`, which is the language's own*); the chapter's is `each`. A guard `steps >=
    len(next)` is not taken as a trip bound where `steps >= 1000` is.

Teaching: chapter 9's and chapter 10's debts are paid here (bytes as a table; a variant
carrying a position), and chapter 12's guard-as-stated-bound returns twice (`len(t) <= 10^6`
in a header, binary search's second arm).

Chapter 14, State (2026-11-16), found a soundness hole and five smaller, none fixed:

70. **A store's position is not checked.** Each of these compiles, on Go, JavaScript and
    windows:

        var b := array { 0 : _ ∈ 0..n-1 };  b[n] := 1                       // one past the end
        var b := array { 0 : _ ∈ 0..n-1 };  b[i] := 1                       // i ∈ Int
        loop i = 0 { i <= n -> { b[i] := i; again(i + 1) }; else -> b }      // the off-by-one

    Go emits `v10[v0] = 1` and panics at run time (*index out of range [5] with length 5*);
    JavaScript's array grows; windows writes past the block. The same index in a *read* is
    refused (chapter 13). tables.md §6 says the obligation was needed "in three places", `build`
    among them, and maps.md §6 cites `(set stk sp …)` carrying `sp < cap`; nothing enforces it
    now. Chapter 14 §14.3 tells the reader of the gap in so many words, and every store in its
    examples is under a guard that would prove it. **Fixed** 2026-11-16 ([store-2026-11-16](../../gauntlet/results/store-2026-11-16.md),
    ADR 0119): a store owes 0 <= i < len as a read does, so all three are refused, on every target;
    §14.3 now shows the refusal.
71. **A map's declared capacity is not enforced, and `len` of a map is wrong on windows.**
    `var m := map(2)` with three stores gives `len(m)` 3 on Go and JavaScript (maps.md §6
    chose a declared capacity so that all targets agree) and **12** on windows. **`len` fixed**
    2026-11-16 ([bookfaults-2026-11-16](../../gauntlet/results/bookfaults-2026-11-16.md)): 3 on
    windows, a map handed on through a tuple now tracked; **the capacity fixed** 2026-11-16
    ([capacity-2026-11-16](../../gauntlet/results/capacity-2026-11-16.md), ADR 0121): a map of
    capacity c is in {m : len(m) <= c}, so the three stores into `map(2)` are refused on every target.
72. **A map read does not build on windows.** A program that reads a map built in a block:
    *(read …) is not printed on LLVM*; one that reads it in a loop: *1 of 97 integer
    operation(s) cannot be proven … llvm.sdiv [0, +inf] in llvm.sdiv(a, b)*, an operation of
    the map library's and not the program's. The chapter's map blocks run on Go and JavaScript. **Fixed** 2026-11-16 ([maprealize-2026-11-16](../../gauntlet/results/maprealize-2026-11-16.md),
    ADR 0120): the map is realized after the decision, so a read in a block or a function builds on
    windows; chapter 14's `tally` runs there now.
73. **A loop that answers `()` freezes the buffer it wrote.** `var b := …; loop i = 0 { i < 5 ->
    { b[i] := 5 - i; again(i + 1) }; else -> () }; b[0] := 100` is *a store writes into #nl2,
    which is a frozen table*. The same loop inside a function that takes `b` `linear`, called
    from that place, is accepted, and so are straight-line stores. So two passes over one
    buffer need two borrowed functions (the chapter's `sort` and `sieve` are written so).
74. **A map's value has no set, and a read of a live map is fragile.** `m[k] := look(m, k) + 1`
    is refused as unbounded, and a guard written as its own arm (`look(m, k) >= 1000 ->`) does
    not help, since two reads of one key are two terms; binding the read first and storing
    `if 0 <= c && c < N { c + 1 } else { c }` works (`tally.oro`). `case m[42] { 'some(v) ->
    io.print_int(v); … }` as a statement is *lowering: … is 2 values where one is wanted*, and
    with `seen = case m[t[i]] { … }` inside the building loop, *a tuple pattern of 2 names
    takes apart a `build` or a `loop`*.
75. **Messages.** The alias refusal prints the core's term, *R1: the buffer b is bound to
    another name …: { d = b; b := store(b, 0, 7); d(0) }*. A `fn` inside a block that writes
    the block's cell: *`:=` writes a `var`, and the place written here, 0, is none*, which is
    the refusal for a name bound with `=`. `var b := t`, with `t` a parameter, compiles to a
    copy, where ADR 0052's R0 says "the copy is explicit"; the chapter writes the rule.

A flaky test, seen twice (chapters 8 and 14): `go test ./core/` reports FAIL once and then
passes three times running, with no change between. Identified in chapter 15: item 78.

Teaching: cells are taught as a notation for new names (static single assignment said as the
meaning, not as an optimization), and the chapter's question, "why a value you hold never
changes", is answered by R1 before `linear` is introduced.

Chapter 15, Failures (2026-11-16), confirmed item 53 where it matters most and found three
more, none fixed:

76. **Item 53 is a wrong answer in the plainest `try` chain.** With `half` returning
    `result.'ok(n / 2)` or `result.'err(n)`,

        try result.'ok(x) = half(a) { try result.'ok(y) = half(b) { result.'ok(x + y) } }

    gives 4 for a = 40, b = 4 (Go: `v32 := (v1 >> 1); v33 := (v32 + v32)`), where 22 is right.
    A chain where each step feeds the next (`try 'ok(a) = half(n) { try 'ok(b) = half(a)
    { half(b) } }`) is right, and so is the two-answer form when the first answer is passed to
    a function as a parameter (`fn add_half(x, b) { try 'ok(y) = half(b) { 'ok(x + y) } }`):
    22. The chapter's §15.4 is written that way and carries a boxed warning to the reader.
77. **`try` with several binds is not read.** j.md §2.8: `try_expr ::= "try" t_bind,+ block`.
    `try result.'ok(x) = half(a), result.'ok(y) = half(b) { … }` is *"," where "{" was
    expected*. The handler, `else`, is listed as not built. **Fixed** 2026-11-16
    ([invariant-2026-11-16](../../gauntlet/results/invariant-2026-11-16.md)): the binds nest,
    `try b₁, …, bₙ { e }` = `try b₁ { try b₂, …, bₙ { e } }`.
78. **The flaky test is a race between two packages.** `go test ./core/ ./cmd/gen/` fails about
    one run in five with *TestEveryCorpusFileReadsInJ: open ..\cmd\gen\booktest3866318782\
    p.oro: The system cannot find the path specified*: `core`'s corpus walk lists a temporary
    directory that `cmd/gen`'s tests make under their own package and remove while the walk is
    reading. Either test alone passes every time. (This is the failure noted under chapters 8
    and 14.) **Fixed** 2026-11-16 ([bookfaults-2026-11-16](../../gauntlet/results/bookfaults-2026-11-16.md)): the corpus is read from git's index.
79. **Messages.** *try: variant light declares no (success …) constructor, so it is not a model
    of the exception monad* is in the notation J replaced and names the variant in lower case.
    The relevance refusals name no function they occur in (*gen: the result of half is a value
    of a relevant type …*, with no `f:`).

Teaching: `expect`'s crash is shown on windows, where the output is the reason alone; Go's and
JavaScript's carry a stack trace with temporary paths, so they are shown as emitted code.
`examples/errors/` already held `discard`, `ignore` and `expect`; the chapter's are its own.

Chapter 16, Claims (2026-11-16), rewritten in J from nothing (the old chapter, in the notation
J replaced and with no running example, is in git at `98ff798`). It found five, none fixed:

80. **A binder's claim with a variable end is not checked.** `k ∈ 0..n = i` with `i ∈ Int`
    compiles and returns `i`. `where lo <= hi { m ∈ lo..hi-1 = lo + (hi - lo) / 2; m }` compiles,
    and the claim is false at `lo == hi`. With constant ends the same construct is checked and
    refused properly (*the claim s ∈ 0..150 is not proven: n + m <= 150 does not follow*). The
    dropped claim is not used as a fact either (`t[k]` after `k ∈ 0..len(t)-1 = i` is still
    refused), so no access is wrongly proven; what is lost is that a false statement in the
    source is accepted without a word. In a function with no header sets the same claim is
    *build: not in normal form for target "go": int*. **Fixed** 2026-11-16
    ([claims-2026-11-16](../../gauntlet/results/claims-2026-11-16.md)): x ∈ lo..hi with a term
    at an end is the formula lo <= x && x <= hi, decided as any claim.
81. **A binder's claim on a name nothing reads is not checked**: `k ∈ 0..5 = n; 0` and
    `s ∈ String = n; 0` compile, with `n ∈ 0..100`. The family of item 45. **Half fixed** 2026-11-16
    ([claims-2026-11-16](../../gauntlet/results/claims-2026-11-16.md)): an unread binding's range
    claims stay, moved onto the scope's body, so `k ∈ 0..5 = n; 0` is refused; `s ∈ String = n; 0`
    still compiles, a typing claim the checker decides on a residual that no longer holds it. Item
    45, a top-level constant nothing reads, fixed 2026-11-18 (deadclaims-2026-11-18).
82. **A result's claim and a binder's claim are decided by different procedures.**
    `(0..100) where a <= b { b - a }` is refused, *its result is [-100, 100]* (intervals alone;
    `(Nat)` likewise), and `d ∈ 0..100 = b - a; d` under the same header is proven by the
    refinement layer and then satisfies the same result claim. The chapter teaches the binder
    as the way to carry a relational fact to a result, which is a workaround presented as a
    technique.
83. **Three facts the layer does not have**, each shown refused in §16.8: `n mod 2 >= 0`
    (`mod` is defined as never negative, integers.md; `n % 2` on a non-negative `n` is
    proven); `a - b != 0` from `where a != b`, with *assumed ne(a,b)* printed among the known
    facts (refinements.md §3c covers a disequality beside an order); `n * n >= 0`.
84. **Messages.** A union of ranges at a parameter, `n ∈ 0..9 ∪ 20..29`: *sets takes a ∪(int 0
    9, int 20 29). A variant type crosses a boundary as a RESULT today; as a parameter it is
    not built*. An index through a claimed binder prints the core's `the`: *t[the("int 0
    999999", r * w)] is an indexing*. The `known:` line prints `+ +1` and `+ -100`; the chapter
    teaches the reader to read it as it is.

Teaching: `cmd/intervals` is named in one sentence and not shown, since `check.go` has no
directive for it and it prints only what it could not bound. The chapter has no run block:
every example is a `gen`, because its subject is what the compiler decides, and a proven
claim leaves nothing to run.

Chapter 17, Targets (2026-11-16), found three, none fixed:

85. **A host declaration's result set is a fact for the intervals and not for the refinement
    layer.** With `fn small() (0..9) pure { host expr "1000" }`, `small() * 10^18` is *1 of 1
    integer operations bounded*, and `array { 0, …, 9 }[small()]` is refused, *0 <= liar.small()
    does not follow; known: nothing*. The same split as item 82, at the trusted base: one
    procedure believes the header and the other has not been told.
86. **Any directory named `targets` is read as target files by the corpus test.** The chapter's
    examples were first in `docs/book/code/targets/`, and `TestEveryCorpusFileReadsInJ` failed
    on each program there, *line 3: "export" begins no form of a target file*. They are in
    `code/hosts/`. The family of item 30 (a file named after a target).
87. **Java's emitted class is named after the output file**: `gen … java g.out` writes `public
    final class g.out`, which is not Java. `-name` prefixes the functions and not the class.

Teaching: §17.5 lies to the compiler on purpose, with a `pure` on a print and a result claimed
`0..9` that is 1000, and shows the dropped line and the wrapped product (3875820019684212736)
as the one way a silent wrong answer enters a program whose proofs are all valid. Five blocks
are `part`s of the repository's own target and library files (`lib/io/go.oro`, `lib/io/js.oro`,
`lib/io/windows.oro`, `targets/go/strings.oro`, `targets/go/go.oro`, `targets/js/js.oro`), so
a change to those lines fails this chapter's check, which is intended: the chapter quotes them.

Chapter 19, A whole program (2026-11-16), reads a 74-line ledger (adapted from
`examples/num/ledger.oro`) line by line, runs it on three targets, and removes five of its
guards to show each refusal. It found four, none fixed:

88. **Three calls of one 40-line function exceed the reducer's step limit.** A `main` that
    called the ledger on three files was *reduction did not terminate within the step limit*,
    on every target (windows, which keeps definitions as functions, included). One call
    builds. The chapter's three cases are three entry programs over one module.
89. **74 lines of source emit 1,553 lines of Go** (595 of JavaScript): the chain of arms is
    copied where a cell's write is specialized. The chapter says so in §19.8.
90. **In a whole program a refusal names the compiler's binder and lists every fact.**
    `src[i]` unguarded is *#nl15[i] is an indexing, and i < len(#nl15) does not follow*, and
    its `known:` line runs to some forty facts, most of them `num.decimal`'s. A bound removed
    from the parser (`digits < 10^6`, `qty < 100`) is refused inside `num.digits`, *requires a
    / 100 <= 9223372036854775807*, the first place the unbounded value fails to fit. The
    chapter shows the first line of two such messages and says what the names are.
91. **Without the file-size test the program is refused on JavaScript for the pair again**:
    *a comparison of a value held in the unsigned word as a pair is not built* (item 42), where
    the cause is that the subtotal's numerator may pass 2⁵³. `portable` gives the right
    verdict, and the chapter shows that line.

Teaching: the chapter introduces nothing. Its sections follow the file, each `part` block a
run of the program's own lines, each paragraph naming the chapter that taught the construct.

**The chapters after the second round of fixes (2026-11-16).** With items 39, 40, 48, 53/76, 60,
64, 67, 70, 71, 72, 77, 78, 80 and half of 81 fixed, every chapter was re-run against the
compiler at `f0b5a62` (all eighteen pass) and the passages that worked around a
fixed item were rewritten, sixteen planted faults caught in the new blocks:
- 53/76, 77: chapter 15 §15.4 is one `try` inside another, then `try a, b, c { … }` with a later
  step reading an earlier name (`both.oro` rewritten, `several.oro`); the boxed warning and its
  line in "What to remember" are gone;
- 70: chapter 14 §14.3 shows a store one past the end refused (`no-past.oro`) in place of the
  warning about the gap;
- 71, 72: chapter 14 §14.9 has a section on the capacity as a claim (`no-full.oro`,
  `distinct.oro` proven from `len(t) <= 16`, `no-distinct.oro` at 17, `distinct-guard.oro` under
  `len(m) < 16`), `tally` runs on windows, and the two lines of "what maps do not do" are gone;
- 60: chapter 16 §16.5 has "Stating an invariant yourself" (`no-walk.oro`, `walk.oro`,
  `no-kept.oro`, `no-entry.oro`): s <= 2 * i, a coefficient no template guesses; chapter 12 §12.7
  points at it and says why the factorial's bound stays a guard; chapter 19's "not built" list
  no longer names it;
- 80, 81: chapter 16 §16.6 shows a claim with terms at its ends and a claim on a name nothing
  reads, each refused (`no-between.oro`, `no-idle.oro`), in place of "two limits today";
- 64: chapter 13 §13.11 takes a pair apart by pattern (`pairs-named.oro`), the same two reads;
- 67: chapter 16 §16.8's table loses the quotient's row (chapter 13 was updated with the fix).

Chapters 8 (39, 40) and 12 (67) had been updated with their fixes. Chapter 9 shows 48's two
refusals on Go and needed nothing; chapter 10 never worked around 53 in its text. Probed and
still open, so their passages stay: 37, 49, 54, 66, 73.

Doing that found one more, not fixed:

92. **A map's capacity refusal names the compiler's version of the map and lists every insert.**
    Three stores into `map(2)`: *a map's capacity requires len(#vm1) < 2 at a call, which does
    not follow*, where the source's name is `m`, and "at a call" where the source has a store;
    its `known:` line has fourteen facts over `#vm`, `#vm1`, `#vm2` and `(insert m 1 10)` in the
    core's notation, then ADR 0028's advice about a definition's parameter range. The chapter
    shows the first line and says what `#vm1` is. The family of item 90.

**Chapter 0 after hamza's reading (2026-11-17).** Four questions on §0.5 and §0.6, each answered
in the chapter, ten planted faults caught in the new blocks:
- all of ℤ: a section showing `Int` inside a program computed exactly (`any-size.oro`, run on Go
  and windows), a half-open set at an export as the host's bignum, and `Int` at an export as the
  word;
- the trap: the `-checked` command lines for `gen` and `build`, and a program that overflows
  when it runs (`no-grow.oro`), refused without the flag and stopped with it. The checker's `run`
  takes `-checked`, and a `run` block may elide lines, for a trap's trace;
- `wide` on JavaScript: the refusal itself, why it is the parameter and not the product, that the
  file's other export is not emitted either, and the narrowed file portable to four
  (`square53.oro`);
- "Functions vanish": ADR 0113 said. `kept.oro` on Go is two copies of a loop; on windows the
  library's `print_int` is one function called three times. The four rules of what is kept.

Doing that found two, not fixed:

93. **`-checked` does not trap on JavaScript.** `no-grow.oro`, 3 squared seven times: `build
    -checked -target=js` says *1 of 2 integer operation(s) are traps, not proofs*, emits
    `v4 = (v4 * v4)` with no test, and the program prints `1.1790184577738583e+61`. Go panics
    and windows stops on an illegal instruction. A wrong answer under the flag that promises a
    stop. The chapter says not to rely on `-checked` for that target. **Fixed** 2026-11-18
    ([checked-2026-11-18](../../gauntlet/results/checked-2026-11-18.md)): wider than JavaScript,
    since an unproven `<<` was unchecked on Go and the JVM too (3 << 69 printed 0 on Go). Under
    `-checked` every operation is now proven, trapped or closed on the word, or refused; JavaScript
    traps `+ − ×`, `<<` and `~`, and Go and the JVM `<<`.
94. **All of ℤ has no spelling at an entry point.** `n ∈ ..` is refused: *`..` with neither end
    is ℤ, which is `Int`*; and `Int` at an export is the host's word (§6.8), its arithmetic
    refused. `Nat` and `..0` are each taken as a bignum, so a host can pass either half and not
    both. A result has the same gap: `fn diff(a ∈ Nat, b ∈ Nat) (Int) { a - b }` is refused as
    wider than the word. Inside a program `Int` is ℤ and is computed exactly where a claim asks.
    Item 27's question from the other side: what is `Int` at an entry point?

Writing chapter 11 (Records) and bringing chapters 7, 12, 13 and 14 up to date (2026-11-20) found
seven more. 95–99 are fixed with the chapter
([recordclaim_test](../../cmd/build/recordclaim_test.go), each with its planted fault); 100 and
101 are not:

95. **A claim on a record hid it, and did not reach its parts.** `p ∈ Point = record { x : 2, y :
    3 }` then `p.x` was refused, *the projection x reads a term whose record labels are not
    known*: the claim's ascription stood between the record and the projection. And
    `p ∈ Pen = record { ink : 300, … }` with `ink ∈ 0..255` compiled and printed 300, the part's
    set never decided. A claim on a record is now a claim on each part (e ∈ Π S_l iff e.l ∈ S_l),
    refused naming the part.
96. **A missing label was refused in the core's notation**: `p ∈ Point = record { x : 2 }` said
    *the projection x reads … the("record{x: int; y: int}", #rec("x", 2))*. Now X2, naming `y`,
    and an extra label X3, and a record set is printed as J writes it.
97. **A record or a tuple as a variant's payload was refused as "a range"**: `'rect(Rect)` said
    *gives a range, which a payload does not take*. A payload of several parts is not built (a
    variant is its tag and one slot per payload set), and the refusal now says so.
98. **`==` on two records was refused in the lowering's words**, *(1, 2) is 2 values where one is
    wanted*. Now: a tuple or a record is several values, and `==` is integer equality only.
99. **A set that mentions itself was accepted unused** and refused at its first use as
    *`n#next` names Node, which is no set*. `set Node = record { value ∈ Int, next ∈ Node }` is
    now refused where it is defined, as a variant containing itself is.
100. **Reading a map while it is still being filled is refused in internal words**: `var m :=
     map(4); m[9] := 50; case m[9] { … }` says *{ (#t, #p) = #0.0(9); … } is 2 values where one
     is wanted*. Narrowed 2026-11-20: a read in an argument works, `io.print_int(case m[9] { … })`
     printing 50; what fails is a `case` on the read in the scope's tail whose arms print (refused
     as a closure), and a read followed by a store (the map's capacity no longer follows). Both
     are in the plan (CLAUDE.md, README's "Current work"), ahead of the decided list.
101. **A table of tables, or a table of tuples written as a list, read at an index not known when
     compiling**, is refused in the lowering's words, the first quoting `#0.0`:
     `array { array { 1, 2 }, array { 3, 4 } }[k][0]` and `array { (3, 4), (10, 2) }[k]`.
     Finding 66's limitation, still there: read at a constant index the table folds away, which
     is how it hides. In the plan, beside finding 100.
102. **A function bound to a name wrote a cell and the write was lost** (found 2026-11-20 probing
     tier 1's callbacks, not writing a chapter; fixed). `var n := 0; add = fn(x) { n := n + x };
     add(1); add(2)` printed n as 0, and `get = fn() { n }` read the cell's first value. A λ
     passed as an argument was refused, and one bound by a name was not checked. Now both are
     refused, and §14.2 shows the refusal and the working form
     ([callbacks-derivation-2026-11-20](../../gauntlet/results/callbacks-derivation-2026-11-20.md)).

A chapter is also allowed to teach something that is not about this language. Chapter 2's last
section is Church encodings, because the compiler's own vector type turns out to be one; chapter
3's is ML functors, because it turns out we already have them and did not notice; chapter 4's is
Boole's algebra and partial evaluation; chapter 5's is referential transparency, thunks, and
substructural logic, because the effect rule derived from three concrete hazards turns out to be
Lambek's ordered fragment, named in 1958; chapter 7's is positional notation, the adder's identity
and Gauss's congruences, because "the machine may wrap early" is the statement that a remainder
is a ring homomorphism; chapter 8's is which fractions terminate in a base, fixed point against
floating point, the bias of rounding a half upward, and what IEEE 754's arithmetic is once it is
written out: a rounding after every operation; chapter 9's is ASCII's layout, Unicode's scalar
values and the surrogates' hole, UTF-8 and UTF-16, the free monoid, and a homomorphism as the
shape of a property that is proven piece by piece (`nulfree`, beside chapter 7's remainder);
chapter 10's is state machines, Hoare's null, case-of-case as one law shared with a tuple's
pattern, and counting values, which is why the two are called products and sums; chapter 12's is
Euclid's algorithm with his termination proof, Steele's tail call, a measure as the one argument
that a loop ends, the halting problem as the reason termination is counted and not required, and
Collatz as the loop nobody can prove; chapter 13's is zero-based and half-open ranges, Heartbleed
as an unchecked index, fusion as β on a table given by a rule, binary search with its invariant,
and positions in place of pointers (struct of arrays, a list by `next`, a grid by `r·w + c`);
chapter 14's is `=` against `:=`, static single assignment, aliasing as the bug the one-name rule
removes, gather against scatter, insertion sort, and Eratosthenes' sieve; chapter 15's is the
three older answers to failure (a sentinel, a flag, an exception) and Ariane 501, the exception
monad said as "the first stop wins", relevance as chapter 5's no-dropping rule on a value, and
Erlang's let-it-crash as the case for `expect`; chapter 16's is testing, typing and proving as
three kinds of knowledge, Fourier–Motzkin elimination by adding inequalities, invariants, and
sound against complete with Presburger, Matiyasevich and Rice as the reason for the wall;
chapter 17's is the foreign function interface, the trusted base (with TypeScript's declaration
files as the same idea), and one virtual machine against being native to each host.

Chapter 5 also uses [targets/tutorial-sloppy.oro](../../targets/tutorial-sloppy.oro) —
`targets/tutorial.oro` with the word `pure` deleted from one primitive — so the cost of a
forgotten purity marker can be shown rather than asserted.
