# 2. `def`

[Chapter 1](01-fn.md) was about `fn`, which makes a function but does not name one. This chapter
is about the form that gives a term a name — and about what a name *is* here, which is not what it
is in most languages.

Every reduction below was produced by running the example:

```bash
go run ./cmd/oro -target=tutorial FILE.oro
```

The `tutorial` target is the one from chapter 1 ([targets/tutorial.oro](../../targets/tutorial.oro)):
`+ - * /` and `<` are arithmetic, `f`, `g`, `h` are opaque functions, `x`, `y`, `z` are opaque
values, `shout` is the one impure primitive, and `if`, `let` and `fold-range` are structural.

---

## 2.1 Naming a term

```lisp
(def square (fn (n) (* n n)))
(square 7)
```

```lisp
⟶   (* 7 7)
```

`(def NAME TERM)` associates a name with a term. Using the name is the same as writing the term.

The term does not have to be a function:

```lisp
(def two 2)
(* two two)
```

```lisp
⟶   (* 2 2)
```

A `def` on its own prints nothing, because a `def` **is not a computation**. It contributes a name
to a scope. Only terms get reduced.

## 2.2 Order does not matter

```lisp
(square 7)
(def square (fn (n) (* n n)))
```

```lisp
⟶   (* 7 7)
```

Definitions are a set, not a sequence. Use a name before you define it, define things in whatever
order reads best. This is different from `let`, which is an application and therefore ordered.

Definitions may refer to each other in any direction:

```lisp
(def outer (fn (n) (inner n)))
(def inner (fn (m) (* m 3)))
(outer 4)
```

```lisp
⟶   (* 4 3)
```

## 2.3 A `def` is unfolded, not stored

This is the one that surprises people, so here it is early.

```lisp
(def one 1)
(def two  (+ one one))
(def four (+ two two))
four
```

```lisp
⟶   (+ (+ 1 1) (+ 1 1))
```

`two` appeared **twice** in the output. A definition is not a variable holding a value; it is a
name for a term, and every use is replaced by that term. That step is called **δ** — the second of
the language's two reduction rules, the first being β from chapter 1.

Compare with a `let`, which is an application, and which chapter 1 showed is shared:

```lisp
(def costly (h 1 2))
(+ costly costly)
```

```lisp
⟶   (+ (h 1 2) (h 1 2))
```

```lisp
((fn (costly) (+ costly costly)) (h 1 2))
```

```lisp
⟶   (let (h 1 2) (fn (costly) (+ costly costly)))
```

Same-looking source, different output. **`def` duplicates; β shares.**

> **Which do you want?** Usually `def`, because most definitions are functions, and duplicating a
> function is how the function disappears. When a definition names *work* rather than a function —
> a specific computation you want performed once — bind it with `fn` at the point of use.

The compiler does not let this become a correctness problem. If the body has an **effect**,
duplicating it would duplicate the effect, so it is refused outright:

```lisp
(def noisy (shout 1))
(+ noisy noisy)
```

```
the body of noisy is a computation, not a value, so unfolding it would repeat its effects
  Wrap it in (fn () …) and apply it, or bind it with let at the point of use.
```

The error names both repairs. `(fn () …)` makes it a function, and a function may be duplicated
freely because duplicating it does not run it.

## 2.4 What can be a name

The same rules as a `fn` parameter, for the same reason: a name is a name.

**A literal cannot be one.**

```lisp
(def 1 (fn (n) n))
;; def takes a name and one term: (def 1 (fn (n) n))
```

**A qualified name cannot be one.**

```lisp
(def a.b (fn (n) n))
;; def a.b: a definition names a member of this module,
;;          and `.` qualifies a member of an imported one
```

`.` always means "member of an imported module". A definition names a member of *this* one, so the
two cannot both be true. Before this rule existed, `(def a.b …)` was accepted and then unreachable:
you could define it, and no term could ever mention it.

**The form takes exactly two things.**

```lisp
(def)                                  ;; def takes a name and one term: (def)
(def square)                           ;; def takes a name and one term: (def square)
(def square (fn (n) n) (fn (m) m))
;; def takes a name and one term: (def square (fn (n) n) (fn (m) m))
```

There is no multi-value `def` and no implicit body sequence. One name, one term.

**A name may be defined only once.**

```lisp
(def sq (fn (n) (* n n)))
(def sq (fn (n) (+ n n)))
(sq 1)
```

```
sq is defined twice
```

No redefinition, no "last one wins". Two definitions of a name is a merge conflict, not a feature.

**Anything else that is a legal name is a legal definition name.** Hyphens, `?`, `!`, and
non-ASCII are ordinary letters, exactly as in chapter 1:

```lisp
(def Δ 3)
(def half-Δ (/ Δ 2))
half-Δ
```

```lisp
⟶   (/ 3 2)
```

## 2.5 Free names inside a definition

A definition's body follows chapter 1's rule: every free name must be a parameter, another
definition, or a primitive on the target. What is new is **when** it is checked.

```lisp
(def helper (fn (n) (+ n qq)))
(* 1 2)
```

```
in helper: qq is not bound — it is not a parameter, not a definition,
           and not a primitive on this target
```

Note what was reduced here: `(* 1 2)`. `helper` is never used. It is checked anyway.

```lisp
(def unused (fn (n) (nope n)))
(+ 1 2)
```

```
in unused: nope is not bound — it is not a parameter, not a definition,
           and not a primitive on this target
```

**Every definition is checked, not only the ones reduction reaches.** A typo in a function you
have not called yet is still a typo. This was not always true, and a misspelled name in dead code
was invisible until something called it.

## 2.6 Names shadow in a fixed order

Three things can supply a name. When more than one does, the order is always the same:

> **parameter** beats **definition** beats **primitive**.

A `fn` parameter wins over a definition:

```lisp
(def double (fn (n) (* n 2)))
(fn (double) (double 5))
```

```lisp
⟶   (fn (double) (double 5))
```

`double` inside is the parameter, so nothing unfolds. Legal, and almost never what you meant —
chapter 1's advice about shadowing applies here too.

The third rule is the interesting one, and it runs the other way.

## 2.7 A target primitive beats a definition — and that is the whole design

`f` is a primitive on the `tutorial` target. Define it anyway:

```lisp
(def f (fn (n) (* n 2)))
(f 5)
```

```lisp
⟶   (f 5)
```

The definition was **ignored**. δ does not unfold a name the target declares primitive.

That looks like a trap and is in fact the central mechanism of the language. Here it is doing real
work. [examples/modules.oro](../../examples/modules.oro) defines a dot product out of small pieces,
none of which the target knows about:

```lisp
(def zip (fn (g a b) (vec (vlen a) (fn (i) (g (vindex a i) (vindex b i))))))
(def sum (fn (v)   (fold-range 0.0 (vlen v) (fn (acc i) (f64.add acc (vindex v i))))))
(def dot (fn (a b) (sum (zip f64.mul (of-array a) (of-array b)))))
```

On a target that has no `dot`, the definitions unfold and fuse into a loop:

```bash
go run ./cmd/oro -target=go examples/modules.oro
```

```lisp
(fn (p q) (fold-range 0.0 (alen p) (fn (acc i)
  (num/f64.add acc (num/f64.mul (aindex p i) (aindex q i))))))
```

On a target that declares `dot` primitive — a machine with BLAS — the *same source file* stops
one step earlier:

```bash
go run ./cmd/oro -target=blas examples/modules.oro
```

```lisp
(fn (p q) (num/vec.dot p q))
```

One line of difference between the two target files decides between a hand-rolled loop and a call
into a tuned library. **A definition is a fallback lowering for targets that lack the name**, and
"primitive beats definition" is how the compiler reaches for the target's own implementation when
there is one.

So it is not a shadowing hazard, it is the point. It becomes a hazard only when the collision is
accidental — which is a reason to read the target file for names you are about to define.

## 2.8 A definition that mentions itself

```lisp
(def countdown (fn (n) (if (< n 1) 0 (countdown (- n 1)))))
(countdown 3)
```

```lisp
⟶   (countdown 3)
```

Reduction **stops**. It does not unfold `countdown` into itself forever, and it does not compute
the answer either. This is correct and deliberate: a recursive definition has no normal form in
general, and a calculus whose whole job is "reduce to normal form" must decline to unfold one.
Mutual recursion is detected the same way:

```lisp
(def even? (fn (n) (odd? n)))
(def odd?  (fn (n) (even? n)))
(even? 3)
```

```lisp
⟶   (even? 3)
```

**But recursion cannot be compiled.** Reduction is only the first half; the residual then goes to a
backend, and no backend emits a recursive definition:

```
gen: gen-countdown mentions the recursive definition(s) countdown, and no backend
     emits recursion yet — iteration is fold-range (docs/spec/def.md §9)
```

So a recursive definition is legal, reduces correctly, and will not build. The reason no program
in this repository is recursive is that iteration here is a primitive:

```lisp
(def total  (fn (n)   (fold-range 0 n (fn (acc i) (+ acc i)))))
(def scaled (fn (n k) (fold-range 0 n (fn (acc i) (+ acc (* k i))))))
(scaled 10 3)
```

```lisp
⟶   (fold-range 0 10 (fn (acc i) (+ acc (* 3 i))))
```

`fold-range` is what the target turns into a `for`. See [def.md §9](../spec/def.md) for the state
of recursion and [§10](../spec/def.md) for why tail-call optimisation is not promised.

There is a second reason to care. A self-reference you did not intend also just stops:

```lisp
(def size (fn (v) (size v)))
(size 3)
```

```lisp
⟶   (size 3)
```

That is a `size` meant to call some *other* `size`, and nothing complains until you try to build.

## 2.9 `export` and `sig`

A definition is private until exported. `export` picks the entry points:

```lisp
(def area (fn (w hh) (* w hh)))
(export area)
```

```lisp
area =
(fn (w hh) (* w hh))
```

Exported names are what a backend emits — one function per export, named after it. A definition
that nothing exports and nothing uses simply never appears.

Exporting something you did not define is an error:

```lisp
(def area (fn (w hh) (* w hh)))
(export are)
```

```
the program exports are, which it does not define
```

That message used to not exist: the typo was silently dropped, and the program was built with no
entry points at all.

`sig` attaches a type to a definition:

```lisp
(sig area ((w num) (hh num)) num)
(def area (fn (w hh) (* w hh)))
(export area)
```

Parameters are **named**, not just typed, because a refinement — the `(where …)` clause — has to
say *which* parameter it is about. A `sig` for a name you did not define is an error too:

```lisp
(def area (fn (w hh) (* w hh)))
(sig aera ((w num) (hh num)) num)
(export area)
```

```
the program declares (sig aera …) but does not define aera
```

Signatures are chapter 5's subject; what matters here is that `sig` and `export` both attach to a
`def` and neither can float free.

## 2.10 Legal, and still a mistake

**Defining a name the language uses in a form.** `def`, `fn` and `use` are special only as the
*first* element of a form. Everywhere else they are ordinary names:

```lisp
(def def 1)
(+ def def)
```

```lisp
⟶   (+ 1 1)
```

```lisp
(def fn 1)
(+ fn fn)
```

```lisp
⟶   (+ 1 1)
```

Both reduce. Neither should be written.

**Defining an operator.**

```lisp
(def + (fn (p q) (- p q)))
(+ 10 1)
```

```lisp
⟶   (+ 10 1)
```

`+` is a name, so it can be defined — and on this target it is also a primitive, so §2.7 applies
and the definition is dead. On a target without `+` it would be live, and addition would mean
subtraction.

**Aliasing.** A definition's body can be a bare name:

```lisp
(def square (fn (n) (* n n)))
(def area square)
(area 3)
```

```lisp
⟶   (* 3 3)
```

Free, since δ unfolds both. Also invisible in the output, so a reader of the generated code has no
way to know `area` existed.

**Naming a definition after a `fn` parameter you will also use.** §2.6 says the parameter wins.
Nothing warns you.

---

## 2.11 Everything from nothing

Now the part that is not about this language.

λ-calculus has no numbers, no booleans, no pairs, no lists, and no data structures of any kind. It
has functions. In the 1930s Church showed that this is not a limitation: every one of those things
can be *encoded* as a function. Nearly a century of people have been finding out how far that goes.

We have `fn` and `def`. That is enough to try it. And there is a reason to try it here rather than
on paper, which the last example in this section is about.

### Booleans

A boolean is a choice between two things. So *be* the choice:

```lisp
(def true  (fn (t e) t))
(def false (fn (t e) e))
```

`true` takes two arguments and returns the first. `false` returns the second. Now conditionals are
just application, and the logical operators are three-line definitions:

```lisp
(def not (fn (b)   (b false true)))
(def and (fn (p q) (p q false)))
(def or  (fn (p q) (p true q)))
(def xor (fn (p q) (p (q false true) q)))

((xor true true)  (x) (y))
((xor true false) (x) (y))
((or  false false) (x) (y))
```

```lisp
⟶   (y)
⟶   (x)
⟶   (y)
```

Those are correct answers to `true xor true = false`, `true xor false = true`,
`false or false = false` — computed with no boolean type anywhere, by a language whose target
declares `bool` and never used it.

### Pairs

A pair is something that hands both halves to whoever asks:

```lisp
(def pair (fn (a b) (fn (sel) (sel a b))))
(def fst  (fn (p) (p (fn (a b) a))))
(def snd  (fn (p) (p (fn (a b) b))))

(fst (pair (x) (y)))
(snd (pair (x) (y)))
```

```lisp
⟶   (x)
⟶   (y)
```

And they compose:

```lisp
(def swap (fn (p) (pair (p (fn (a b) b)) (p (fn (a b) a)))))
(fst (swap (pair (x) (y))))
```

```lisp
⟶   (y)
```

### Numbers

A Church numeral is "apply this function *n* times":

```lisp
(def zero  (fn (s z) z))
(def three (fn (s z) (s (s (s z)))))
```

To see what one *is*, apply it to a real successor and a real zero:

```lisp
(def inc (fn (n) (+ n 1)))
(three inc 0)
(zero  inc 0)
```

```lisp
⟶   (+ (+ (+ 0 1) 1) 1)
⟶   0
```

Arithmetic on them is arithmetic on function composition:

```lisp
(def plus (fn (m n) (fn (s z) (m s (n s z)))))
(def mult (fn (m n) (fn (s z) (m (fn (a) (n s a)) z))))

((plus two three) inc 0)
((mult two three) inc 0)
```

```lisp
⟶   (+ (+ (+ (+ (+ 0 1) 1) 1) 1) 1)
⟶   (+ (+ (+ (+ (+ (+ 0 1) 1) 1) 1) 1) 1)
```

Five `+`s and six `+`s. `2 + 3 = 5` and `2 × 3 = 6`, done by a compiler that has not been told what
addition is.

### Lists

The list encoding is "be your own `fold`" — a list is the function that folds itself. (This is
Böhm–Berarducci encoding; the numerals above are its special case for a type with two
constructors.)

```lisp
(def nil  (fn (c n) n))
(def cons (fn (hd tl) (fn (c n) (c hd (tl c n)))))
(def sum  (fn (l) (l (fn (a b) (+ a b)) 0)))

(sum (cons 1 (cons 2 (cons 3 nil))))
```

```lisp
⟶   (+ 1 (+ 2 (+ 3 0)))
```

`map` is a wrapper that changes what the fold sees:

```lisp
(def map (fn (k l) (fn (c n) (l (fn (a b) (c (k a) b)) n))))
(sum (map g (cons (x) (cons (y) nil))))
```

```lisp
⟶   (+ (g (x)) (+ (g (y)) 0))
```

**Look at what is not there.** No list. No cons cells. No intermediate list between `map` and
`sum`. In a language with runtime lists this program allocates a list, walks it, allocates a
*second* list, and walks that. Here the two traversals fused into one expression and the data
structure never existed. Compiler people call this deforestation and write papers about doing it;
here it fell out of β and δ, because reduction happens at compile time and there is no runtime for
the list to exist in.

### Combinators

The three that generate everything (Schönfinkel, 1924; Curry):

```lisp
(def I (fn (a) a))
(def K (fn (a) (fn (b) a)))
(def S (fn (a) (fn (b) (fn (c) ((a c) (b c))))))
```

`S K K` is the classic identity of combinatory logic:

```lisp
((S K) K)
(((S K) K) (x))
```

```lisp
⟶   (fn (c) c)
⟶   (x)
```

The first line is worth pausing on. `((S K) K)` reduced to `(fn (c) c)` — the compiler *proved*
`SKK = I`, with no arguments in sight, because reduction goes under binders. That is chapter 1
§1.8 doing algebra.

### Where it stops

Two limits, and both are honest.

**Exact arity.** Textbook λ-calculus is curried: every function takes one argument. Ours does not
(chapter 1 §1.7), and the classic encodings assume currying. So the two-parameter numerals above
break the moment something applies one partially — `exp = λm.λn. n m` relies on it:

```lisp
(def exp (fn (m n) (n m)))
((exp two two) (fn (n) (+ n 1)) 0)
```

```
arity: expects 2 argument(s), given 1, in: (fn (s z) (s (s z)))
```

Same for `S K K` written with a three-parameter `S`:

```lisp
(def S (fn (a b c) (a c (b c))))
(S K K)
```

```
arity: expects 3 argument(s), given 2, in: (fn (a b c) (a c (b c)))
```

Nothing is lost — write the currying yourself, as the `S` and `K` above do, and everything works.
Kleene's predecessor, famously the hard one, works too:

```lisp
(def three (fn (s) (fn (z) (s (s (s z))))))
(def pred (fn (n) (fn (s) (fn (z)
  (((n (fn (gg) (fn (hh) (hh (gg s))))) (fn (u) z)) (fn (u) u))))))
(((pred three) inc) 0)
```

```lisp
⟶   (+ (+ 0 1) 1)
```

Two. Currying is a representation choice, not a semantic one — but it is a choice you have to
write down here.

**Terms with no normal form.** The smallest divergent term is Ω, the self-application of
self-application:

```lisp
(def omega (fn (a) (a a)))
(omega omega)
```

```
reduction did not terminate within the step limit; last term head: (fn (a) a)
```

And the famous one, the **Y combinator** — the fixed-point operator that gives untyped λ-calculus
its recursion:

```lisp
(def Y (fn (ff) ((fn (a) (ff (a a))) (fn (a) (ff (a a))))))
(Y f)
```

```
reduction did not terminate within the step limit; last term head: (fn (ff) (fn (a) (ff (a a))))
```

This is not a bug and not a missing feature — it is the reason §2.8 exists. `Y f` has **no normal
form**. Reduction here *is* compilation, so a term with no normal form is a program that will not
finish compiling. That is why recursion is a named definition the reducer declines to unfold rather
than a combinator you can write, and why iteration is `fold-range`. The step limit is the guardrail
in front of the same fact.

### The encoding that is actually in the compiler

Here is why this section is not a curiosity. Look again at the pair from earlier and then at
[lib/num/vec.oro](../../lib/num/vec.oro), which is real, shipped library code:

```lisp
(def vec    (fn (n f) (fn (sel) (sel n f))))
(def vlen   (fn (v)   (v (fn (n f) n))))
(def vindex (fn (v i) ((v (fn (n f) f)) i)))
```

**That is a Church pair.** `vec` holds a length and an index function, and the language's entire
vector type is those three lines and nothing else. Give it a use:

```lisp
(def scale (fn (v k) (vec (vlen v) (fn (i) (* k (vindex v i))))))

(vindex (scale (vec 10 (fn (i) (h i i))) 3) 4)
(vlen   (scale (vec 10 (fn (i) (h i i))) 3))
```

```lisp
⟶   (* 3 (h 4 4))
⟶   10
```

Reading element 4 of a scaled 10-element vector compiles to `(* 3 (h 4 4))`. The vector was never
built, the scaling never ran over anything, and asking for the length is a compile-time constant.

An encoding costs nothing **when the thing that takes it apart is known at compile time** — and
here it always is, because reduction runs until only primitives remain. Church's encodings are
usually taught as a proof of expressiveness and dismissed as impossibly slow. In a language that
reduces at compile time they are not slow; they are *free*, and they are how the standard library
is written.

The catch, and it is a real one: a vector must eventually become memory. That is what
`materialize` is for, and what it costs is measured in
[docs/spec/construction.md](../spec/construction.md). Free until it has to be real.

---

## What to remember

- **`def` names a term, and every use is replaced by that term** — δ. It duplicates; β shares.
- **Definitions are a set.** Order does not matter, mutual reference is fine.
- **A definition with an effect in its body is refused**, because unfolding it would repeat it.
- **Every definition is checked**, including ones nothing uses.
- **Parameter beats definition beats primitive** — and the last of those is the whole parasite
  model: a definition is the fallback for targets that lack the name.
- **Recursion reduces and does not compile.** Iteration is `fold-range`.
- **`export` chooses entry points; `sig` types them.** Neither can name something undefined.
- **`fn` and `def` alone are Turing-complete**, and the encodings that prove it cost nothing here,
  because the eliminator is always known at compile time. The library uses this on purpose.

Next chapter: `use`, `module`, and how a name gets a qualified spelling.
