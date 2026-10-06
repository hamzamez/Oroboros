# The book

Chapters, in order. Each is written for someone learning the language — human or model — and
leans on examples rather than explanation.

**Every reduction shown in a chapter is real output**, and that is checkable rather than asserted:
chapters 0–4 were re-run against the compiler on **2026-10-06** and every reduction in them is the
byte-for-byte output of running the example. Chapter 5 is the exception and says so at its head.

**Every reduction shown in a chapter is real output.** The examples run against
[`targets/tutorial.oro`](../../targets/tutorial.oro), a target that exists to teach: it spells
arithmetic `+ - * /` so a chapter need not also be about module paths, and it declares `f`, `g`,
`h`, `x`, `y`, `z` as primitives so reduction halts exactly where a lesson wants it to.

```bash
go run ./cmd/oro -target=tutorial FILE.oro
```

That a teaching target is *possible* is the thesis in miniature: the normal form is a parameter, so
choosing where reduction stops is choosing a target.

| | |
|---|---|
| [0. syntax](00-syntax.md) | s-expressions: one character moves, application as the only compound form, the parenthesis count, why not postfix, no precedence, the whole grammar, names, comments, the history, the pros and cons, and what the notation buys *this* language |
| [1. `fn`](01-fn.md) | functions, parameters, binding, shadowing, capture, what survives |
| [2. `def`](02-def.md) | naming a term, δ, why a definition duplicates, why a primitive wins, recursion, and what λ-calculus alone can do |
| [3. modules](03-modules.md) | `module`, `use`, `export`, the four cells, one namespace for targets and libraries, and why a functor is just a function |
| [4. effects](04-effects.md) | one declared bit, the β side condition, `seq`, and the substructural logic it turns out to be |
| [5. types](05-types.md) | the checker that is not in the language, `sig` checked in two directions, refinements, and why our proofs do not transfer — **its examples do not run; see the banner at its head** |

Planned: targets.

Chapter 0 needs no target: it is about the reader, so its examples are reductions on `tutorial` and
a handful of read errors. It is addressed to someone arriving from Python or JavaScript, and it is
deliberately the one chapter that teaches a notation rather than a language.

Chapter 5 uses `cmd/gen` and `cmd/build` rather than `oro`, because the checker lives at the
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
   arithmetic impure, and which of the two decisions should give way is a design call.
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

Chapter 0 found none in the compiler, which is itself a result worth recording: it is the one chapter
about the **reader**, and the reader is the most-exercised code in the project — every target file,
every `provides` fragment and every program goes through it. Writing it did turn up one corrupted
file and one missing one (both in the list above), and it found that the single most interesting
thing about the notation is not a syntax argument at all: **`+` can be a `sig` in a data file only
because application is the only compound form**, so the parenthesised prefix notation is what makes
ADR 0002's parameter expressible. That is §0.17, and it is the chapter's reason for existing rather
than being an appendix.

A chapter is also allowed to teach something that is not about this language. Chapter 2's last
section is Church encodings, because the compiler's own vector type turns out to be one; chapter
3's is ML functors, because it turns out we already have them and did not notice; chapter 4's is
substructural logic, because the effect rule derived from three concrete hazards turns out to be
Lambek's ordered fragment, named in 1958; chapter 5's is Presburger arithmetic and Liquid Types,
almost all of which we decline, and the reason why.

Chapter 4 also uses [targets/tutorial-sloppy.oro](../../targets/tutorial-sloppy.oro) —
`targets/tutorial.oro` with the word `pure` deleted from one primitive — so the cost of a
forgotten purity marker can be shown rather than asserted.
