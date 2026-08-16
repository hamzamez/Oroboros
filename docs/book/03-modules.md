# 3. Modules

Two chapters, two forms. [`fn`](01-fn.md) makes a function; [`def`](02-def.md) gives a term a name.
Everything so far has lived in one file, and every name has been a single word.

This chapter is about the third brick: **where a name lives**. It adds three forms — `module`,
`use`, `export` — and one character, `.`.

It also answers a question chapter 2 left open. §2.7 said a target primitive beats a definition and
called that the whole design. It never said *how* a library definition and a target primitive can
possibly be the same name. This chapter is that mechanism.

Examples run against real files in [docs/book/code/](code/):

```bash
go run ./cmd/oro -target=tutorial docs/book/code/box.oro
```

Two targets appear: `tutorial` from chapters 1 and 2, and `tutorial-native`, which is the same file
plus **one capability**. §3.10 is what that line does.

---

## 3.1 A name with a dot in it

Here is a library file, [code/geometry.oro](code/geometry.oro):

```lisp
(module geometry)

(export area perimeter scale)

(def area      (fn (w h) (* w h)))
(def perimeter (fn (w h) (* 2 (+ w h))))
(def scale     (fn (k w h) (area (* k w) (* k h))))

(def twice (fn (n) (* 2 n)))
```

And a program that uses it:

```lisp
(use geometry)
(geometry.area 3 4)
```

```lisp
⟶   (* 3 4)
```

Three things happened. `(use geometry)` found the file. `geometry.area` named a member of it. And
the definition **unfolded exactly as in chapter 2** — δ does not care that the name came from
another file.

```lisp
(use geometry)
(geometry.perimeter 3 4)
```

```lisp
⟶   (* 2 (+ 3 4))
```

Definitions inside a module see each other unqualified. `scale` calls `area` by its plain name, and
both unfold:

```lisp
(use geometry)
(geometry.scale 2 3 4)
```

```lisp
⟶   (* (* 2 3) (* 2 4))
```

## 3.2 `.` and `/` are different characters, and the difference is the whole grammar

> **`.` separates a module from a member. `/` is an ordinary letter.**

`shapes/circle` is **one name** — one module path with a slash in it, not two of anything. The
member comes after the dot:

```lisp
(use shapes/circle)
(circle.area 2)
```

```lisp
⟶   (* 3 (* 2 2))
```

A path maps to a file the obvious way: `shapes/circle` is found at `shapes/circle.oro` under a
search-path root. Nesting is directories; the language has none of its own.

This is why chapters 1 and 2 kept saying a binder must be a *simple* name. `(fn (a.b) …)` and
`(def a.b …)` are errors because `a.b` already means something — a member of module `a` — and a
function cannot bind into a module. `/` is fine in a binder, because it means nothing:

```lisp
(fn (a/b) a/b)      ;; legal: a/b is one ordinary name
(fn (a.b) a.b)      ;; error: a binder is a simple name
```

## 3.3 `use` — importing

```lisp
(use PATH)              ;; binds the LAST SEGMENT of PATH as the alias
(use PATH as ALIAS)     ;; binds ALIAS
```

`(use shapes/circle)` binds `circle`, not `shapes/circle`. The alias is a local nickname; the path
is the identity.

```lisp
(use geometry as g)
(g.area 3 4)
```

```lisp
⟶   (* 3 4)
```

**Imports stay qualified.** There is no `from geometry import area`, no wildcard, no way to make
`area` mean `geometry.area` without saying so at the use site. That is a deliberate refusal, and
its reason is the one in §3.9: on this system a name's meaning already depends on which target you
build for, and an unqualified import would add a second, invisible source of the same
dependence.

Forgetting the import is an error, and it names the repair:

```lisp
(geometry.area 3 4)
```

```
geometry is not imported: add (use geometry) or (use PATH as geometry)
```

Two aliases for the same module are fine, if a little pointless:

```lisp
(use geometry)
(use geometry as geo)
(+ (geometry.area 1 2) (geo.perimeter 1 2))
```

```lisp
⟶   (+ (* 1 2) (* 2 (+ 1 2)))
```

Two modules under **one** alias is not:

```lisp
(use geometry as g)
(use shapes/circle as g)
(g.area 3)
```

```
the program binds g to both geometry and shapes/circle — give one of them a
different (use … as …) alias
```

That error exists because `g.area` would otherwise mean whichever import the reader happened to
notice — and both modules really do have an `area`.

## 3.4 `module` — declaring

```lisp
(module PATH)
```

Everything after it belongs to that module, until the next `(module …)`. A file with no `(module …)`
is one anonymous scope, which is what chapters 1 and 2 were writing all along.

**A library file declares exactly one module, and it is the one its path names.** Three ways to get
that wrong, three messages:

```lisp
;; nodecl.oro — no header at all
(def area (fn (w h) (* w h)))
```

```
nodecl: a library file must declare (module nodecl) before its definitions
```

```lisp
;; mixup.oro — declares a different module than its path
(module geo)
(export a)
(def a 1)
```

```
mixup declares (module geo); a library file's module must be the path that imports it
```

```lisp
;; sub/one.oro — declares its own module and a second one
(module sub/one) (export k) (def k 1)
(module sub/two) (export j) (def j 2)
```

```
sub/one also declares (module sub/two); a library file declares one module, and it is
the one its path names — put sub/two in its own file, or its members are reachable only
after something else has imported this one
```

That last message says why the rule exists, and it is worth reading twice. Before the rule,
`(use sub/two)` **failed on its own and succeeded if `(use sub/one)` came first** — the same
program, two meanings, decided by load order. A module system whose visibility depends on what
else you imported is not doing the job a module system exists to do.

### The entry file is different

The file you actually run is not found by path, so it may declare as many modules as it likes.
This is one file:

```lisp
(module left)
(use geometry)
(export twice-area)
(def twice-area (fn (w h) (* 2 (geometry.area w h))))

(module right)
(use geometry)
(export half-area)
(def half-area (fn (w h) (/ (geometry.area w h) 2)))

(module top)
(use left)
(use right)
(export both)
(def both (fn (w h) (+ (left.twice-area w h) (right.half-area w h))))
```

```lisp
left.twice-area =
(fn (w h) (* 2 (* w h)))
right.half-area =
(fn (w h) (/ (* w h) 2))
top.both =
(fn (w h) (+ (* 2 (* w h)) (/ (* w h) 2)))
```

That is a **diamond** — `top` reaches `geometry` by two routes — and nothing about it is special.
`geometry` is loaded once, `geometry.area` is one name, and the two paths to it agree because there
is only one thing there to disagree about.

`(module …)` in an entry file also names its exports:

```lisp
(module app)
(use geometry)
(export main-area)
(def main-area (fn () (geometry.area 3 4)))
```

```lisp
app.main-area =
(fn () (* 3 4))
```

## 3.5 `export` — what is visible

`geometry` exports three names and defines four. The fourth is private:

```lisp
(use geometry)
(geometry.twice 5)
```

```
module geometry defines twice but does not export it; add it to that module's (export …)
```

A name that was never there gets a different message, because it is a different mistake:

```lisp
(use geometry)
(geometry.volume 3 4)
```

```
module geometry has no member volume
```

`twice` is not useless — `scale` could call it, because inside the module every definition is in
scope. Export controls the *outside* view only.

> **A module with no `(export …)` at all exports everything.** That is a transitional convenience
> and not a design: it means "no signature has been written yet". Write the list.

Exporting a name you did not define is an error, from [chapter 2 §2.9](02-def.md):

```
the program exports are, which it does not define
```

## 3.6 Two modules may use the same member name

```lisp
(module alpha)
(export area)
(def area (fn (n) (* n n)))

(module beta)
(export area)
(def area (fn (n) (+ n n)))

(module main-mod)
(use alpha)
(use beta)
(export both)
(def both (fn (n) (+ (alpha.area n) (beta.area n))))
```

```lisp
alpha.area =
(fn (n) (* n n))
beta.area =
(fn (n) (+ n n))
main-mod.both =
(fn (n) (+ (* n n) (+ n n)))
```

No clash, no renaming, no import order to reason about. This is the entire benefit of qualified
imports, and it is why the alias-collision error in §3.3 is the *only* naming conflict the system
can produce.

## 3.7 Cycles are fine

```lisp
;; cyc/a.oro
(module cyc/a)
(use cyc/b)
(export f)
(def f (fn (n) (b.g n)))
```

```lisp
;; cyc/b.oro
(module cyc/b)
(use cyc/a)
(export g)
(def g (fn (n) (* n 2)))
```

```lisp
(use cyc/a)
(a.f 5)
```

```lisp
⟶   (* 5 2)
```

Imports are followed to a fixpoint and a module already in scope is never re-read, so a cycle
terminates by itself. There is no header-guard, no forward declaration, no ordering constraint.

Two mutually recursive **modules** are fine. Two mutually recursive **definitions** are still an
error ([chapter 2 §2.8](02-def.md)) — different question, different answer.

## 3.8 What the reducer sees: nothing

Here is the claim that makes all of the above cheap.

> **Modules are resolution, not reduction.** By the time reduction starts, every name is fully
> qualified and there are no modules left. The reducer cannot tell they ever existed.

You can see it in the output. `circle.point-at` calls `trig.cos`, and the residual says:

```lisp
(use shapes/circle)
(circle.point-at 2 1)
```

```lisp
⟶   (* 2 (math/trig.cos 1))
```

`math/trig.cos` — the **full path**, not the alias `trig` that the source wrote. Aliases are a
source-level convenience that resolution erases. What survives is one flat namespace of qualified
names, exactly as if you had written them by hand.

This is why chapters 1 and 2 needed no revision to accommodate modules. `core/reduce.go` gained no
rule, no term kind, and no parameter. δ unfolds `geometry.area` for the same reason it unfolds
`square`: it is a name with a definition.

## 3.9 A module the *target* provides

```lisp
(use math/trig)
(trig.sin 1)
```

```lisp
⟶   (math/trig.sin 1)
```

There is no `math/trig.oro` anywhere. The import found no file — **and that is not an error**. It
means `math/trig` is a module the *target* implements natively, and `targets/tutorial.oro` says so:

```lisp
(module math/trig
  (prim sin (num) num expr "sin(%s)" pure)
  (prim cos (num) num expr "cos(%s)" pure))
```

Reduction stops on `math/trig.sin` because it is a primitive — chapter 1's rule, unchanged.

**But "no file" is also what a typo looks like**, and for a long time the two were
indistinguishable until much later:

```lisp
(use geometrie)
(geometrie.area 3 4)
```

```
at the top level: geometrie.area is not bound — it is not a parameter, not a definition,
and not a primitive on this target
  (use geometrie) matched no file on the search path and this target provides no module
  geometrie either, so every name from it is unbound. Check the path.
```

The first line blames `geometrie.area`; the second says the problem is the half before the dot. A
module that *does* exist but lacks the member gets no hint, because there the first line is right:

```lisp
(use math/trig)
(trig.tan 1)
```

```
at the top level: math/trig.tan is not bound — it is not a parameter, not a definition,
and not a primitive on this target
```

## 3.10 The four cells

Everything in this chapter has been building one table. A qualified name can be **defined by a
library**, **provided by a target**, both, or neither:

| | target provides it | target does not |
|---|---|---|
| **a library defines it** | the target's wins; the definition is a fallback | the definition unfolds |
| **no definition** | it is a primitive; reduction stops | **error** — unbound |

Row 1 is the interesting one, and here it is running. [code/box.oro](code/box.oro) mentions no
target at all:

```lisp
(use geometry)
(export box)
(def box (fn (w h) (geometry.area w h)))
```

```bash
go run ./cmd/oro -target=tutorial docs/book/code/box.oro
```

```lisp
box =
(fn (w h) (* w h))
```

```bash
go run ./cmd/oro -target=tutorial-native docs/book/code/box.oro
```

```
note: geometry.area is defined here and provided natively by target "tutorial-native";
the target's is used
```
```lisp
box =
(fn (w h) (geometry.area w h))
```

One source. Two targets. Two normal forms — the library's arithmetic on one, a call into the
target's own implementation on the other. And the difference between the two target files is
**one declaration**:

```lisp
(module geometry
  (prim area (num num) num expr "area(%s, %s)" pure))
```

That is the mechanism chapter 2 §2.7 promised and could not show, and it only works because of a
rule this chapter has been assuming all along:

> **One namespace.** A target and a library name into the *same* qualified namespace. The target
> declares `geometry.area`; the library defines `geometry.area`; they are the same name.

If targets and libraries had separate namespaces the intersection would be permanently empty, every
program would silently get the portable fallback, and no target's native implementation would ever
be reachable. The whole system would still typecheck and still run — and would quietly never do the
one thing it exists to do.

Scaling that up is what the language is for. Write `num/vec.dot` once; on a plain target it fuses
into a loop, on a target with BLAS it becomes `cblas_ddot`. [examples/modules.oro](../../examples/modules.oro)
is that program, and `-target=go` versus `-target=blas` is that pair.

---

## 3.11 Everything a module is not

This section is the honest part. Modules here are much less than the word usually implies, and the
gaps are load-bearing rather than accidental.

### Not a unit of compilation

In Modula-2, Ada, and every language that followed them, a module is a **separately compilable
unit**: compile it once, ship the object file, and a consumer needs only its interface. That is the
reason modules were invented, and this language does not do it.

It cannot. Reduction needs the *body* of `geometry.area` to unfold it, and unfolding is the whole
compilation strategy — the reason a helper function costs nothing is that it disappears. An
interface without a body would preserve exactly the call this language exists to remove.

So compilation here is **whole-program**, and that is a real trade with a real cost: you cannot
distribute a compiled library, and build time grows with everything you depend on. What you get for
it is chapter 2 §1.8 — abstraction that vanishes. It is not obvious that the trade is right in
general; it is right *here*, because parity with hand-written code is the requirement and a
preserved call is a lost one.

### Not first-class

You cannot pass a module to a function, store one, or return one. `(use …)` takes a literal path,
and a module is not a value. Modules exist at resolution time and nothing at reduction time can
refer to one — that is §3.8 restated as a limitation.

### No functors — and none needed

ML's answer to parameterised modules is the **functor**: a function from structures to structures,
with its own type system, its own application syntax, and a well-known reputation for being the
hardest part of the language.

We cannot have one, since modules are not values. But look at what a functor *does* — take the
operations you are parameterised over, return the operations you built from them — and then look at
chapter 2 again. That is a function returning a record. And a record is a function
([chapter 2 §2.11](02-def.md)).

```lisp
(def accumulate (fn (combine unit)
  (fn (a b c) (combine (combine (combine unit a) b) c))))

((accumulate + 0) 1 2 3)
((accumulate * 1) 1 2 3)
((accumulate h 0) 1 2 3)
```

```lisp
⟶   (+ (+ (+ 0 1) 2) 3)
⟶   (* (* (* 1 1) 2) 3)
⟶   (h (h (h 0 1) 2) 3)
```

One definition, instantiated at addition, at multiplication, and at an operation the compiler knows
nothing about. That is a functor applied three times, and **nothing survives the application** — no
dictionary, no vtable, no indirection, not even a mention of `accumulate`.

A structure with several members works the same way, because a Church record has as many fields as
you like:

```lisp
(def make-vec2 (fn (add mul)
  (fn (x yy) (fn (sel) (sel x yy add mul)))))

(def dot2 (fn (v w)
  (v (fn (a b add mul) (w (fn (c d _ __) (add (mul a c) (mul b d))))))))

(dot2 ((make-vec2 + *) 1 2) ((make-vec2 + *) 3 4))
(dot2 ((make-vec2 h *) 1 2) ((make-vec2 h *) 3 4))
```

```lisp
⟶   (+ (* 1 3) (* 2 4))
⟶   (h (* 1 3) (* 2 4))
```

`make-vec2` is a functor: it takes an addition and a multiplication and returns a vector structure
carrying both. `dot2` is a client of that structure. The result is a dot product with the arithmetic
substituted in — and the structure, the closure, and the parameterisation are all gone.

**So the feature that costs ML a second language costs us nothing, because we already had it.** A
functor is a function, applied at compile time, over a record that is also a function. `fn` and
`def` were enough; modules never needed to grow to reach it.

What modules give that `fn` and `def` cannot is the thing functors were never about: **a namespace
that a target can also write into**. That is §3.10, and it is the only reason this form exists.

### Not `import *`, not `#include`, not a package manager

- No unqualified import, by refusal (§3.3).
- `(use …)` is a **dependency**, not a textual inclusion. A library's own bare terms and exports are
  not the program's; only the entry file contributes entry points.
- Where files are found is a search path (`-path`, default `lib`, entry file's own directory first)
  and nothing more. Versioning, fetching and publishing do not exist and are not designed.

---

## What to remember

- **`.` separates module from member; `/` is an ordinary letter.** `shapes/circle` is one name.
- **`(use PATH)` binds the last segment**; `(use PATH as A)` binds `A`. Imports stay qualified.
- **`(module PATH)` opens a scope.** A library file declares exactly one, and it is the one its path
  names. The entry file may declare many.
- **`(export …)` is the outside view.** Inside a module every definition is in scope. No list at all
  means everything is public, which means the list has not been written yet.
- **Cycles and diamonds are fine.** Mutually recursive modules yes; mutually recursive definitions
  no.
- **Modules are resolution, not reduction.** They are gone before β and δ start; the residual shows
  full paths, never aliases.
- **An import that finds no file is a module the target provides** — and the error for a misspelled
  path says so, because those two look identical from the outside.
- **One namespace for targets and libraries.** That is what lets a definition be a fallback for the
  target's own implementation, and it is the point of the whole form.
- **A functor is just a function.** Modules stayed small because `fn` had already done the hard
  part.

Next chapter: effects — the one thing β is not allowed to move.
