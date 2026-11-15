# The book

Chapters, in order. Each is written for someone learning the language — human or model — and
leans on examples rather than explanation.

**Where the book stands, 2026-11-10.** The surface is J ([spec/j.md](../spec/j.md), ADR 0068), and
the chapters are being rewritten for it in order:

| chapter | written in | checked |
|---|---|---|
| 0 | J, the language as decided and built | 2026-11-10, by `check.go` (below): 58 blocks, every source, emitted function, refusal and output |
| 1 | J | 2026-11-10, by `check.go`: 76 blocks, every source, reduction, emitted function and refusal |
| 2 | J | 2026-11-15, by `check.go`: 71 blocks |
| 3 | J | 2026-11-15, by `check.go`: 52 blocks |
| 4 | the notation J replaced (git `1e2bdcd`) | 2026-10-06, in that notation; its examples no longer read |
| 5 | the notation J replaced | its examples did not run even then; see its banner |

```bash
go run docs/book/check.go docs/book/00-tour.md      # from the repository's root
go run docs/book/check.go docs/book/01-fn.md
go run docs/book/check.go docs/book/02-def.md
go run docs/book/check.go docs/book/03-modules.md
```

[check.go](check.go) pairs each fenced block with the directive on the line before it (an HTML
comment naming a file, a `gen`, an `oro`, a `build`, a `run` or a `portable`), runs it, and compares; a block
with no directive is counted and reported, not passed. It was trusted after planted faults failed:
seven in chapter 0, six in chapter 1, six in chapter 2 and seven in chapter 3, covering each kind of directive. `cmd/check`
does not run it.

**A reduction is printed in J**: `oro -j` writes each export's normal form in the surface's
notation (`core.ShowTerm`, [spec/j.md §6.1](../spec/j.md)), where `oro` alone writes the core term.
**A source the reader refuses is not `.oro`**: `core`'s `TestEveryCorpusFileReadsInJ` requires every
`.oro` file in the repository to read, so an example whose refusal is the reader's ends in
`.oro.txt`. A `no-….oro` reads, and is refused by a later stage.

**Chapters 1–4 teach reduction**, and their examples run against
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
| [0. a tour](00-tour.md) | the whole language for someone arriving from Go, Python, JavaScript, Java, C or Rust: what is like those and what is not. Hello on two hosts, one function on four, blocks, types as sets, integers that are integers, functions that vanish, `loop` and `again`, state and tables, variants, failures as values, tuples, bits, modules and the host, what is absent, and a table from familiar spellings to this one |
| [1. `fn`](01-fn.md) | the first brick: a function is a value, β, what folds and what waits, what a parameter may be, bound and free names, shadowing and α, capture, exact arity, reduction under a binder, functions as arguments and results and the closure that is refused, how often an argument is evaluated with and without an effect, a block as an applied function, a named function's header and its claims, and Ω |
| [2. definitions](02-def.md) | the second brick: naming a term, with no keyword; δ; a definition copied where a binding is shared; an effect refused; what a name may be, and the names the language owns; every definition checked; parameter, then target, then definition, and why the target winning is the design; no recursion, by a cycle of any length; what reaches the host and what copying costs; then Church's booleans, pairs, numerals and lists, SKK, Kleene's predecessor, Y, and the Church pair in the library |
| [3. modules](03-modules.md) | the third brick: a file is a module named by its path; `.` and `/`; `use` and why imports stay qualified; `pub` against `export`; the entry file and the files it uses; several modules in one file, a diamond, one member name in two modules; modules in a cycle; resolution, not reduction; a module the target provides; the four cells and the one namespace; and what a module is not, with the functor that is a function |
| [4. effects](04-effects.md) | one declared bit, the β side condition, `seq`, and the substructural logic it turns out to be |
| [5. types](05-types.md) | the checker that is not in the language, `sig` checked in two directions, refinements, and why our proofs do not transfer — **its examples do not run; see the banner at its head** |

Planned: targets.

Chapter 0 uses the real targets, not the teaching one: its sources are in [code/tour/](code/tour/),
compiled with `cmd/gen`, built and run with `cmd/build`, and measured with `cmd/portable`. A file
named `no-…` is meant to be refused, and the chapter prints the refusal. The chapter it replaced was
about s-expressions, the notation before J, and is in git at `a50bd47`.

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

The tour (chapter 0, 2026-11-10) found seven, by writing each example the way a newcomer would
first. None is fixed here; each is a defect or a question for its own step:

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

A chapter is also allowed to teach something that is not about this language. Chapter 2's last
section is Church encodings, because the compiler's own vector type turns out to be one; chapter
3's is ML functors, because it turns out we already have them and did not notice; chapter 4's is
substructural logic, because the effect rule derived from three concrete hazards turns out to be
Lambek's ordered fragment, named in 1958; chapter 5's is Presburger arithmetic and Liquid Types,
almost all of which we decline, and the reason why.

Chapter 4 also uses [targets/tutorial-sloppy.oro](../../targets/tutorial-sloppy.oro) —
`targets/tutorial.oro` with the word `pure` deleted from one primitive — so the cost of a
forgotten purity marker can be shown rather than asserted.
