# How a program reads: `def`, a shorthand for it, and constants

Research. **Decided and built the same day** — the recommendation in §7 was taken whole:
[defshorthand-2026-09-18](../gauntlet/results/defshorthand-2026-09-18.md). §8's discriminator went
into the reader with it. 2026-09-18, on hamza's three questions:

1. can a definition be written `(def main () …)` instead of `(def main (fn () …))`;
2. should one word `def` keep doing both jobs, as Scheme's `define` does, or should there be a second
   word — `defn`, `func`, `function`;
3. why is a constant written `(def cap-in (fn () 16777216))` rather than `(def cap-in 16777216)`?

Nothing is built. A prototype was written to price the change and then reverted; its patch is §6.4.
Every number here was measured today, with the compiler's own reader.

---

## 1. Question 3 first, because it is a fact rather than a preference

### 1.1 What a definition is

A definition is a **context extension**, not a term ([def.md §1](spec/def.md)):

```
Γ, f := t  ⊢  f ≡ t          (δ)
```

`f` is definitionally equal to a **term**, and the term's shape is not part of what `def` means. So
`(def cap-in 16777216)` is well-formed, and it works today:

```lisp
(def cap 16777216)
(def main (fn () (io.print-int cap)))      ; prints 16777216
```

**Nothing requires the `(fn () …)` wrapper around a literal.** In `wc.oro` and `freq.oro` it is habit.

### 1.2 Where the wrapper IS required, and the compiler says so

δ unfolds a definition at **every** use, so a body that *computes* would have its effects repeated —
[ADR 0010](decisions/0010-effects-as-structural-rules.md)'s value restriction. The compiler refuses it,
and prescribes the idiom in the message:

```
(def noisy (io.print-line "evaluated"))
→ the body of noisy is a computation, not a value, so unfolding it would repeat its effects
  Wrap it in (fn () …) and apply it, or bind it with let at the point of use.
```

The same refusal covers an **allocating** body, which is the case worth knowing:
`(def t (alloc (table 6 f)))` is refused for the same reason. Wrapping it does not make the allocation
shared — `(t)` allocates at each call — it makes the cost **visible at the call**, which is
construction.md's rule.

So the wrapper means *"this is a computation, and I am naming it rather than running it"*. That is
real, and it applies to none of the constants.

### 1.3 Measured: how the corpus writes definitions

`examples/`, `lib/`, the differential cases, the acceptance programs and `emit/`'s embedded libraries:
131 files, read with `core.Read`.

| | count |
|---|---:|
| definitions | **510** |
| whose body is a λ | **494** (96.9%) |
| whose body is a value | 16 — all `(array …)` JSON documents |
| nullary λ, `(fn () …)` | **57** |
| …of which the body is a bare literal, so the wrapper buys nothing | **19** |
| curried, `(fn (a) (fn (b) …))` | 14 |
| whose body is a `seq` | 17 |
| `sig`s with named parameters | 77 |
| …whose names disagree with the λ's | **4** (`dot`: the sig says `p q`, the λ says `a b`) |

**The answer to question 3:** those 19 are the `cap-in` shape, and every one of them can be a plain
value. Rewriting `wc.oro` that way — `(def cap-in 16777216)`, used as `cap-in` rather than `(cap-in)` —
emits **byte-identical Go** and prints the same answer. The remaining 38 nullary λs are `main` and
genuine computations, where the wrapper is load-bearing.

---

## 2. Question 1: should the shorthand exist?

### 2.1 What it is, algebraically

The shorthand is the **equation**. Mathematics writes `f(x) = e`, and λ-calculus reads that as
`f = λx.e`; the two are related by η. Every language in §3 that has the form treats it exactly so.
[def.md §4](spec/def.md) already recorded the position, in 2026-08:

> **Scheme's `(define (f x) body)` shorthand is sugar** for `(def f (fn (x) body))` and should be
> recognised as such if we adopt it — not as a second meaning for `def`.

So the question is not *what would it mean* — that is settled — but whether to adopt it, in which
spelling, and what it costs.

### 2.2 What makes sugar admissible here

This repository already carries `let`, `seq`, `and`/`or`/`not`/`cond`, `tuple`, `match` and `case` as
sugar, and refuses aliases such as `sum` for `variant`. The distinction that separates the two, stated
so it can be applied again:

- **Sugar** is a map from a surface form to a term, defined by an equation, which the reader applies
  once and which nothing downstream can observe. `(let e k) = (k e)`.
- **An alias** is a second name for the same declaration, with no expansion: two spellings the loader
  must both understand forever, which is what [data.md §10](spec/data.md) refuses.

A `def` shorthand is the first kind. Three conditions make it safe, and each is checkable:
1. **Unambiguous**: no existing well-formed program can be read as the new form.
2. **Erased in the reader**, so no analysis, no backend and no emitted file can tell.
3. **Invertible enough for diagnostics**: an error inside the body prints the desugared λ, exactly as
   it already does for `let` and `match`.

### 2.3 Measured: what it would buy

494 of 510 definitions would lose a `(fn …)` wrapper: about **2,500 characters** and, more to the
point, **one level of nesting each** in a language whose parentheses are already deep. `wc.oro`'s
`main` currently closes with nine consecutive `)`; the shorthand version closes with eight, and its
constant removes two more.

That is an ergonomic gain and nothing else. It buys no expressiveness: the language is exactly as
powerful either way.

---

## 3. Question 2: one word or two?

### 3.1 What the literature does, and **why**

| language | forms | why it splits, or does not |
|---|---|---|
| Scheme (R7RS), Racket | `define` for both, with `(define (f x) …)` | one namespace, one notion of binding |
| Clojure | `def`, and `defn` | `defn` is a **macro over `def`**; it exists to carry a docstring, metadata and several arities |
| Common Lisp | `defun`, `defvar`, `defconstant` | a **Lisp-2**: functions live in a different namespace, so the word picks the namespace |
| Standard ML | `val`, `fun` | `fun` **implies recursion**; `val` does not. The word carries a semantic difference |
| Haskell | equations, no keyword | definitions are equations; functions and values are not distinguished |
| Go | `func`, `var`, `const`, `type` | four **different link-time entities**; a `func` declaration is not a value binding |
| Rust | `fn`, `const`, `static` | items, not bindings; `const` is inlined, `static` has an address |
| Elixir | `def`, `defp` | the second is *private*, a visibility distinction, not a shape one |

**The rule the table shows: a second word is earned when the two forms MEAN different things** — a
namespace, recursion, a link-time entity, or visibility.

### 3.2 Which of those differences do we have?

None.
- **One namespace.** A name resolves to one definition ([modules.md](spec/modules.md)).
- **No recursion**, so no `fun`-vs-`val` distinction to carry
  ([ADR 0014](decisions/0014-recursion-is-not-in-the-language.md)).
- **No link-time entity**: every definition is inlined by δ, and what a backend emits is decided by
  the residual, not by the declaration.
- **No visibility on `def`**: `export` carries that, separately.
- **No metadata, no docstrings, and no arity overloading *today*** — the three things Clojure's `defn`
  exists for. The third is unsettled rather than refused, and §8 is what it would do to this.

And [theories.md §2](spec/theories.md) classifies a declaration by whether it has a **definiens** and a
**realization**, not by the shape of the definiens. A `defn` would be a fourth classification axis that
the theory does not have, and the language would have two words where it has one concept.

**So: keep `def`, and if the shorthand is adopted, adopt it as a form rather than as a word.** It then
adds **no word to [inventory.md](spec/inventory.md)**, which is a real consequence rather than a
slogan: the word set is pinned by a test, and this change leaves it at 136.

`func`/`function` are worse still: they name the *value's* shape, which is what `fn` already does
inside the term language.

---

## 4. Which spelling

Three candidates, all of which desugar to the same term.

| | spelling | nullary case |
|---|---|---|
| **A** (hamza's, Clojure/Elixir-shaped) | `(def sq (n) (* n n))` | `(def main () …)` |
| **B** (Scheme's) | `(def (sq n) (* n n))` | `(def (main) …)` |
| **C** (a second word) | `(defn sq (n) …)` | `(defn main () …)` |

**A and B are both unambiguous**, and for the same reason: `(def NAME TERM)` has exactly three
elements, so a four-element `def` is free (today it is refused with *"def takes a name and one term"*),
and in B the second element is a list rather than a name. Measured: both spellings are refused by
today's compiler, so neither can silently change an existing program.

**A costs one clause in the parser, and B costs none.** `()` is **not a term**
([core-0.md](spec/core-0.md)), so `(def main () …)` cannot be parsed without the reader knowing it is
reading a `def` — the reader already has exactly this special case for `fn` and for `sig`, and A adds
`def` to it (one line). B's `(main)` is an ordinary application and parses already.

**Three arguments for A**, which is the one I would take:
- It is **the same shape as `sig`**: `(sig sq ((n int)) int)` and `(def sq (n) …)` put the name in the
  same place, and a reader compares them line by line.
- The defined name stays **greppable and sortable** — `(def sq` finds it; in B it is `(def (sq`.
- The nullary case, which is 57 definitions here, reads better: `(def main () …)` against
  `(def (main) …)`.

**One argument for B**: it mirrors the call site — you write down what `(sq n)` *is* — which is the
pedagogical reason Scheme chose it, and it extends to curried definitions `((f a) b)`, which we do not
want anyway (§5).

## 5. What should NOT come with it

- **No implicit `seq` over several body forms.** Scheme's `define` has an implicit `begin`, and here it
  would be a hazard rather than a convenience: `seq` deletes a **pure** leading term (β is allowed to
  drop it), so `(def f () a b)` with a pure `a` would silently discard `a`. Written out, `(seq a b)`
  is the programmer asking for the ordering. One body form.
- **No curried shorthand** (`(def add (a) (b) …)`). 14 definitions are curried; they stay explicit λs,
  and nothing is lost.
- **No shorthand for `sig`.** Its parameter list carries types and is a different object.
- **No change to `fn`.** The anonymous λ is unaffected, and a shorthand def is exactly a λ bound to a
  name — a value that can be passed, exactly as today.

## 6. What it would cost the compiler

### 6.1 Measured with a prototype

The prototype was written, run, and reverted. It is **17 lines**: one clause in the reader's list
parser (so `()` is a parameter list after `def`, as it already is after `fn` and `sig`), and one
rewrite in `toForm` that builds the λ.

| check | result |
|---|---|
| `go test ./core/ ./emit/` | pass |
| `go run ./cmd/check` (vet, compiler, emission) | pass, **byte-identical to the baseline** |
| `(def sq (n) (* n n))`, `(def cap () 65536)`, `(def main () …)` | compile and run; emits `func WSq(n int) int` |
| `wc.oro` rewritten with the shorthand and a value constant | **emitted Go byte-identical**, prints the same 455 |
| the word set | unchanged: 136 words, so `inventory.md` does not move |

### 6.2 What else would have to change if it is adopted

- **The value-restriction message** should offer the new spelling: *"write `(def noisy () …)` and apply
  it"*.
- **`ToForm` is shared with target files**, so a `provides` block's `def`s get the shorthand for free.
  That is consistent, and it should be stated rather than discovered.
- **Documents**: [def.md §4](spec/def.md) (turn its conditional sentence into the rule),
  [state.md](spec/state.md)'s sugar table, the book's chapter 2, and the README's examples.
- **The corpus is not obliged to move.** Sugar is opt-in; a rewrite of 494 definitions is a separate,
  purely cosmetic change, and it cannot alter emission — which is checkable, and is how it should be
  checked if it happens.

### 6.3 What it would cost a reader of the language

One more thing to know, and a genuine ambiguity of *style*: after this, a constant can be written
`(def cap 65536)` or `(def cap () 65536)`, and the two differ at the use site (`cap` against `(cap)`).
That is not new — both are legal today — but the shorthand makes the second cheaper to write, so the
convention should be stated where the corpus can follow it: **a constant is a value; a computation
gets an empty parameter list.**

### 6.4 The prototype patch, for reproduction

```diff
@@ core/read.go, in (*reader).list — the empty parameter list
-			kids[0].Name == "sig") && r.peek() == '(' {
+			(kids[0].Name == "sig" || kids[0].Name == "def")) && r.peek() == '(' {
@@ core/read.go, in toForm, case "def"
+		if len(t.Kids) == 4 && t.Kids[1].Kind == KName && t.Kids[2].Kind == KApp {
+			ps := make([]string, 0, len(t.Kids[2].Kids))
+			ok := true
+			for _, p := range t.Kids[2].Kids {
+				if p.Kind != KName { ok = false; break }
+				ps = append(ps, p.Name)
+			}
+			if ok {
+				t = &Term{Kind: KApp, Kids: []*Term{t.Kids[0], t.Kids[1], Fn(ps, t.Kids[3])}}
+			}
+		}
```

## 7. Recommendation

1. **Answer to question 3, now, with no decision needed:** the wrapper around a literal is habit. Write
   `(def cap-in 16777216)`. Keep `(fn () …)` for a computation, which is what the value restriction is
   about, and say so in the examples that currently mislead.
2. **Question 2: keep one word.** A second word is earned by a semantic difference — namespace,
   recursion, link-time entity, visibility — and this language has none of them. `def` is the
   definiens of theories.md's single declaration form.
3. **Question 1: adopt spelling A**, `(def NAME (params…) body)`, as reader sugar: one body form, no
   currying, no new word, and the message about computations updated to match.

**What would refute 3:** a program in which the shorthand makes the value restriction harder to see —
someone writing `(def f () (io.print-line …))` and expecting the print to happen once, at load. The
wrapper's visibility is the one thing the shorthand spends, and a real program is what would show it.

**What is deliberately not proposed:** multi-body definitions, curried definitions, a `sig`/`def`
parameter-name agreement check (4 corpus mismatches make it tempting, and it is a separate question),
and rewriting the corpus.

---

## 8. Variadics and overloading, and what they would do to the shorthand

hamza, on §3.2: *"we have yet to settle variadic functions, and type and arity overloading, and we
might need it."* Correct, and §3.2's "refused" was too strong. Three features get lumped together and
they land in three different places, only one of which is `def`'s surface.

| | what it is | where it lives |
|---|---|---|
| **V** | calling a **host** variadic function — `fmt.Println(a, b, c)` | the **declaration**: `sig`, and how `tg.Prims` is keyed |
| **D** | a **program** definition taking a rest parameter | **`fn`'s parameter list** |
| **O** | **overloading**: one name, several definitions, chosen by arity or by type | the **name table** |

### 8.1 Where the pressure actually is, measured

- **V dominates, and it is a host fact.** Go: **117 variadic symbols, 2.5%**, and it is why `fmt` is
  **1 of 23** usable ([gostdlib](../gauntlet/results/gostdlib-2026-09-06.md)). JavaScript's natives
  under-report it (`Math.max.length` is 2, and it is variadic). Win32 has the `printf` family.
- **O is mostly the JVM, and also a host fact:** **4,948 overloads, 16.0%** of the emitted surface,
  because `nextInt()` and `nextInt(int)` are one entry in a table keyed by name
  ([surveys](../gauntlet/results/surveys-2026-09-10.md)). Our own target files carry the same wart by
  hand: `Println`, `Println2`, `Println3`.
- **D and program-level O have no demand at all.** No program in the corpus has asked for either, and
  a second definition of one name is refused today: *"f is defined twice"* (`core/reduce.go`).

So the live pressure is on **declarations**, which the shorthand does not touch.

### 8.2 What each would change

**V — nothing in the term language.** A call site has a fixed arity after reduction, so what is
missing is a declaration that can say *"any number of these"*: either `tg.Prims` keyed by
`(name, arity)` — the small version, which deletes `Println2`/`Println3` — or a template with a
repeating hole. No `def`, no `fn`, no reader change.

**D — one marker in `fn`'s parameter list, which the shorthand inherits verbatim**, because the
shorthand's parameter list *is* `fn`'s. Whatever `(fn (a … r) …)` comes to mean, `(def f (a … r) …)`
means the same by construction; there is no second decision to make.

> **Derivation — a rest parameter is static, or it is a table.** At the static level an application's
> argument count is known, so β can bind the rest to a `tuple` and nothing survives; a tuple is a
> function on `Fin n` and costs nothing ([data.md](spec/data.md)). If the rest must survive to run
> time, its length is decided at run time, and a dynamic index forces homogeneity
> ([tables.md](spec/tables.md)) — so it is an `(array V)`, and a heterogeneous variadic cannot
> survive staging. That is the same shape as the rule for closures, and it is what makes `...any`
> a host-boundary question rather than a language one.

**O — the name table, not the surface.** Today Γ maps a name to one definition. Overloading makes it a
map to a set, resolved by arity (decidable at the call site) or by type at emission —
[overloading.md](overloading.md)'s **concept name**, of which `len` on tables and maps is already one.
That is a change in `Load` and in resolution; `(def f …)`'s spelling is not involved either way.

### 8.3 The one real interaction: the grammar slot

If arity overloading ever reaches program definitions, the natural spelling is the one Clojure and
Erlang use — several clauses under one name:

```lisp
(def f ((a) (* a 2))
       ((a b) (+ a b)))
```

That wants the **same slot** as the shorthand: a `def` with four or more elements. The two stay
distinguishable, and the rule should be fixed now rather than discovered later:

> **A parameter list is a list of NAMES ONLY; a clause is a list whose first element is itself a
> list.** `(a b)` is a parameter list; `((a b) body)` is a clause.

That is the rule already used twice, for type arguments and for constants as range endpoints: *admit a
shape only where it cannot mean the other thing*. The prototype in §6.4 already tests exactly this —
it falls back to today's meaning unless every element is a name — so the insurance costs nothing; it
needs writing into the spec so the next person does not spend it.

### 8.4 One spelling is already taken, measured

Clojure's rest marker is `&`, and **`&` is not free here**: it is a declared primitive on Go, Java and
JavaScript (bitwise and, `targets/go/builtin.oro`), and `symbolChars` makes it an ordinary name. So
`(fn (a & r) …)` parses **today** as three parameters, silently.

`...` is free: the reader refuses it as a name, naming its own rule — *"`...` has an empty segment; `.`
separates qualifiers and cannot begin, end, or double"*. A rest marker would therefore be `...` at the
cost of one reader clause, or a sublist such as `(rest r)`. Either way the decision belongs to `fn`.

### 8.5 The answer

**Adopting the shorthand costs nothing in any of the three directions, and helps in one.**
- V and O live in declarations and in the name table; the shorthand is invisible to both.
- D lives in `fn`'s parameter list, and the shorthand inherits whatever is decided there.
- If multi-arity definitions ever arrive, the shorthand is the form they extend — Clojure and Erlang
  both write them exactly that way — and **without** the shorthand there would be no natural place to
  put them at all, since `fn` is one λ and `(def f (fn …))` twice is refused.

The only thing to bank today is §8.3's discriminator. If the answer to the shorthand is yes, that
sentence goes into the spec with it.
