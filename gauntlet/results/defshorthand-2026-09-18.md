# The def shorthand, and a constant is a value

2026-09-18. `core/read.go`, `core/reduce.go`, `core/defshorthand_test.go`, and 128 `.oro` files.
Decided by hamza on [program-surface.md](../../docs/program-surface.md), which measured the question
first and recommended this spelling.

## 1. What is built

```lisp
(def square (n) (* n n))          ≡  (def square (fn (n) (* n n)))
(def main () (io.print-int 1))    ≡  (def main (fn () (io.print-int 1)))
```

**Reader sugar with a unique expansion**, erased before anything else looks: it is mathematics'
`f(n) = n²`, and [def.md §4](../../docs/spec/def.md) had already recorded in August that Scheme's
`(define (f x) …)` *is* sugar and should be adopted as such if it were adopted at all.

**One word.** A second word — `defn`, `func` — is earned by a semantic difference: a namespace
(Common Lisp), recursion (ML's `fun`), a link-time entity (Go), visibility (Elixir's `defp`). This
language has none of them, and [theories.md §2](../../docs/spec/theories.md) classifies a declaration
by whether it has a definiens, not by the definiens' shape. So the **word set does not move: 136**,
and [inventory.md](../../docs/spec/inventory.md)'s test says so.

**`(def f (fn …))` remains exactly as legal**, and is still the way to write a curried definition.

### Three rules, each refused by name

| rule | why | refusal |
|---|---|---|
| a parameter list is a list of **names only** | it keeps the slot open for several arities under one name (program-surface.md §8.3) | *"several arities under one name are not built … a parameter list is a list of names"* |
| **one body form** | an implicit `seq` would delete a **pure** leading expression in silence, because β may drop one | *"def f takes ONE body; `(seq a b)` is how two expressions are sequenced"* |
| the same parameter rules as `fn` | one implementation, because two would be two rules that can disagree | `paramList` is shared; `fn`'s messages are unchanged |

## 2. A constant is a value

`(def cap-in (fn () 16777216))` was habit, and the corpus had 19 of them. A definition binds a
**term**, so a constant is written `(def cap-in 16777216)` and used as `cap-in`.

The `(fn () …)` wrapper means something else, and it is worth knowing what: δ unfolds a definition at
**every** use, so a body that *computes* would repeat its effects — and an **allocating** body is
refused for the same reason, `(def t (alloc …))` included. The value restriction's message now offers
the new spelling:

```
the body of noisy is a computation, not a value, so unfolding it would repeat its effects
  Give it an empty parameter list — (def noisy () …) — and apply it,
  or bind it with let at the point of use.
```

**The convention, stated where the corpus can follow it: a constant is a value; a computation gets an
empty parameter list.**

## 3. The corpus moved, and nothing it compiles to did

A text rewriter (temporary, deleted) took the whole corpus: **128 files, 495 definitions shortened,
18 constants made values**, with their uses `(cap-in)` → `cap-in`. 845 lines changed, and four
comments that quoted the old spelling were fixed by hand.

| check | result |
|---|---|
| `go run ./cmd/check` — emission | **456 runs, byte-identical to the baseline** |
| proof counts | **identical**: 2,361 of 2,413 operations, 345 of 382 loops |
| differential — 30 programs built and run on four targets | pass |
| tooling — the surveys, 13 acceptance programs, every hand declaration against its host | pass |
| `go test ./core/ ./emit/` | pass |

That emission is byte-identical across 194 emitted files is the whole argument that this is sugar: no
analysis, no backend and no artifact can tell which spelling was written.

`wc.oro` is the shape of the change:

```diff
-(def cap-in (fn () 16777216))
-(def main (fn ()
+(def cap-in 16777216)
+(def main ()
-              (if (>= (len src) (cap-in))
+              (if (>= (len src) cap-in)
```

## 4. Cost

- **`core/read.go`: +80 lines**, of which the shorthand itself is about 20 — one clause in the
  reader's empty-parameter-list case (`()` is not a term, so `(def main () …)` cannot be parsed
  without the reader knowing it is reading a `def`, exactly as for `fn` and `sig`), one rewrite in
  `toForm`, and `allNames`, the discriminator.
- **`core/reduce.go`: 1 line**, the message.
- **`core/defshorthand_test.go`: 141 lines.** The main property is a **commuting square**: the two
  spellings must read to the `DeepEqual` term, not merely behave alike. The rest is one test per
  refusal, each asserting the text it must name, plus a control that the three-element form is
  untouched for values, tables, applications and curried λs.
- **`at(line)`**: a message with no line no longer says *"line 0"*.

## 5. What is not built

- **Several arities under one name.** Refused by name, and §8.3's discriminator is what keeps the
  grammar slot free for it.
- **Currying sugar** — 14 definitions in the corpus are curried λs and stay written out.
- **Variadic parameters.** They belong to `fn`'s parameter list, and the shorthand will inherit
  whatever is decided there. One spelling is already gone: `&` is a declared primitive on Go, Java and
  JavaScript, so Clojure's marker would parse today as an ordinary parameter; `...` is free.
- **A `sig`/`def` parameter-name agreement check.** Four definitions in the corpus disagree with their
  signature's names (`dot` says `p q` in the sig and `a b` in the λ). The shorthand makes that
  visible; whether to check it is a separate question.
