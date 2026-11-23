# 2. Definitions

[Chapter 1](01-fn.md) was about `fn`, which makes a function and gives it no name. This chapter is
the second brick: **giving a term a name.** It is a smaller step than it sounds, and a stranger
one, because a name here is not what it is in most languages. It is not a variable, not a memory
location, and not a function the program calls. It is an abbreviation.

There is no keyword for it. A definition is written the way mathematics writes one, a name and
what it stands for:

```go
two = 2                      // a value
fn square(n) { n * n }       // a function; the same as  square = fn(n) { n * n }
```

Inside the compiler this form is called `def`, and the rule that uses one is called **δ**. Those
two words are all this chapter adds to chapter 1's β.

The examples are files in [code/def/](code/def/), run as chapter 1's were:

```bash
go run ./cmd/oro -j -target=tutorial docs/book/code/def/name.oro
```

`oro` reduces each `export` and prints what is left. The `tutorial` target supplies arithmetic,
the opaque functions `f`, `g` (one argument) and `h` (two), the opaque values `x()`, `y()`, `z()`,
and `shout`, which has an effect. A file named `no-…` is meant to be refused; one the reader
itself refuses ends in `.oro.txt`. `go run docs/book/check.go docs/book/02-def.md` re-runs every
block. Checked 2026-11-15.

---

## 2.1 Naming a term

<!-- file docs/book/code/def/name.oro -->
```go
fn square(n) { n * n }

cube = fn(n) { n * n * n }

two = 2

export fn run() { h(square(two), cube(two)) }
```

<!-- oro tutorial docs/book/code/def/name.oro -->
```go
run =
fn() { h(4, 8) }
```

Three definitions and one use of each. Using a name is the same as writing its term, so
`square(two)` is `fn(n) { n * n }(2)`, which β turns into `2 * 2`, which is `4`.

- `fn square(n) { … }` and `cube = fn(n) { … }` are one form in two spellings. The first is the
  usual one for a function, and it is the way mathematics writes an equation, *f(n) = n²*.
- A definition's term need not be a function. `two` is a number.
- **A definition is not a computation.** It contributes a name to the file. Nothing happens
  because it is there, and `oro` printed only `run`.

A file holds declarations and nothing else. An expression on its own at the top level has nowhere
to go:

<!-- file docs/book/code/def/no-bare.oro.txt -->
```go
h(1, 2)

export fn run() { 1 }
```

<!-- oro tutorial docs/book/code/def/no-bare.oro.txt -->
```
oro: docs/book/code/def/no-bare.oro.txt: line 1: "h" begins no declaration; a file holds fn, set, use, module and NAME = e (ADR 0064)
```

## 2.2 Order does not matter

<!-- file docs/book/code/def/order.oro -->
```go
export fn run() { outer(4) }

fn outer(n) { inner(n) }

fn inner(m) { h(m, 3) }
```

<!-- oro tutorial docs/book/code/def/order.oro -->
```go
run =
fn() { h(4, 3) }
```

A file's definitions are a **set**, not a sequence. Use a name before its definition, and put
definitions in whatever order reads best; there are no forward declarations and no prototypes.
This is the difference from a block (§1.12), whose bindings are applications, one inside the
other, and so are ordered.

## 2.3 A definition is unfolded, not stored

This is the one that surprises people, so here it is early.

<!-- file docs/book/code/def/unfold.oro -->
```go
one = 1

two = h(one, one)

four = h(two, two)

export fn run() { four }
```

<!-- oro tutorial docs/book/code/def/unfold.oro -->
```go
run =
fn() { h(h(1, 1), h(1, 1)) }
```

`h(1, 1)` is there **twice**. `two` is not a variable holding a value that was computed once; it
is a name for the term `h(one, one)`, and each use of the name is replaced by the term. That
replacement is δ:

```
name   ⟶   the term it is defined as
```

Compare a name bound in a block, which chapter 1 showed is an application and is shared:

<!-- file docs/book/code/def/share.oro -->
```go
costly = h(1, 2)

export fn at_the_top() { h(costly, costly) }

export fn in_a_block() { c = h(1, 2); h(c, c) }
```

<!-- oro tutorial docs/book/code/def/share.oro -->
```go
at_the_top =
fn() { h(h(1, 2), h(1, 2)) }
in_a_block =
fn() { c = h(1, 2); h(c, c) }
```

The two sources read alike and mean different programs. **A definition is copied; a binding is
shared.** Both are written `name = term`, and what tells them apart is where they stand: at a
file's top level, or inside braces.

> **Which do you want?** For a function, a definition: copying a function is how it disappears
> into its uses. For a constant, a definition: copying `64` costs nothing. For a piece of *work*
> you want done once, a binding in a block, at the place the work belongs.

The compiler does not let the difference become a wrong answer. If a definition's term has an
effect, copying it would repeat the effect, so it is refused:

<!-- file docs/book/code/def/no-effect.oro -->
```go
noisy = shout(1)

export fn run() { h(noisy, noisy) }
```

<!-- oro tutorial docs/book/code/def/no-effect.oro -->
```
oro: docs/book/code/def/no-effect.oro: the body of noisy is a computation, not a value, so unfolding it would repeat its effects
  Give it an empty parameter list — fn noisy() { … } — and apply it,
  or bind it in a block at the point of use.
```

The message names both repairs, and they are different programs:

<!-- file docs/book/code/def/effect.oro -->
```go
fn noisy() { shout(1) }

export fn run() { h(noisy(), noisy()) }

export fn once() { n = noisy(); h(n, n) }
```

<!-- oro tutorial docs/book/code/def/effect.oro -->
```go
run =
fn() { h(shout(1), shout(1)) }
once =
fn() { n = shout(1); h(n, n) }
```

A function may always be copied, because copying one runs nothing; the effect happens where it is
*applied*. `run` applies it twice and shouts twice, which is what it says. `once` applies it once
and uses the answer twice.

## 2.4 What can be a name

The rules are a parameter's (§1.4): a letter or `_`, then letters, digits and `_`, an optional
final `?`. Letters are Unicode's.

<!-- file docs/book/code/def/unicode.oro -->
```go
Δ = 3

half_Δ = Δ / 2

fn small?(n) { n < half_Δ }

export fn run() { small?(0) }
```

<!-- oro tutorial docs/book/code/def/unicode.oro -->
```go
run =
fn() { true }
```

(`3 / 2` is `1`: `/` is integer division.) What a definition's name may not be:

**A literal, a qualified name, or a name with a hyphen.** Each is refused by the reader in the same
words, since none of them begins a declaration: `a.b` is a member of *another* module, and
`half-x` is `half` minus `x`.

<!-- oro tutorial docs/book/code/def/no-literal.oro.txt -->
```
oro: docs/book/code/def/no-literal.oro.txt: line 1: "1" begins no declaration; a file holds fn, set, use, module and NAME = e (ADR 0064)
```

<!-- oro tutorial docs/book/code/def/no-qualified.oro.txt -->
```
oro: docs/book/code/def/no-qualified.oro.txt: line 1: "a" begins no declaration; a file holds fn, set, use, module and NAME = e (ADR 0064)
```

<!-- oro tutorial docs/book/code/def/no-hyphen.oro.txt -->
```
oro: docs/book/code/def/no-hyphen.oro.txt: line 1: "half" begins no declaration; a file holds fn, set, use, module and NAME = e (ADR 0064)
```

**A name already defined.**

<!-- file docs/book/code/def/no-twice.oro -->
```go
fn sq(n) { n * n }

fn sq(n) { n + n }

export fn run() { sq(1) }
```

<!-- oro tutorial docs/book/code/def/no-twice.oro -->
```
oro: docs/book/code/def/no-twice.oro: sq is defined twice
```

There is no redefinition and no "last one wins". A name means one thing in a file, and there is
no overloading either: `sq` cannot be one function on integers and another on strings.

**A name the language owns.** `len`, `if` and the operators are among the language's
own vocabulary, the same on every target, and a program may not redefine them:

<!-- file docs/book/code/def/no-language.oro -->
```go
len = 3

export fn run() { len }
```

<!-- oro tutorial docs/book/code/def/no-language.oro -->
```
oro: docs/book/code/def/no-language.oro: the program defines len, which is the language's own: J's reader writes its constructs with these names (`if c { a } else { b }` is `if`, `a + b` is `+`), so a definition would capture them in its module or be ignored beside the target's. A definition introduces a fresh name; choose another
```

That includes the operators. §1.6 refused a *parameter* named `` `+` ``, which would have changed
what `a + b` means inside one function. A *definition* of `+` would change it in every line of the
file, and is refused for the same reason: a definition introduces a fresh name, as a binder does.

<!-- file docs/book/code/def/no-operator.oro -->
```go
fn `+`(p, q) { p - q }

export fn run() { 10 + 1 }
```

<!-- oro tutorial docs/book/code/def/no-operator.oro -->
```
oro: docs/book/code/def/no-operator.oro: the program defines `+`, which is the language's own: J's reader writes its constructs with these names (`if c { a } else { b }` is `if`, `a + b` is `+`), so a definition would capture them in its module or be ignored beside the target's. A definition introduces a fresh name; choose another
```

**There is no operator overloading**, and this is where it is refused.

## 2.5 Every definition is checked

A definition's body follows §1.5's rule: each free name must be a parameter, a definition, or the
target's. And it is checked whether or not anything uses it.

<!-- file docs/book/code/def/no-helper.oro -->
```go
fn helper(n) { n + qq }

export fn run() { 1 * 2 }
```

<!-- oro tutorial docs/book/code/def/no-helper.oro -->
```
oro: docs/book/code/def/no-helper.oro: in helper: qq is not bound — it is not a parameter, not a definition, and not a primitive on this target
```

`run` never mentions `helper`. A typo in a function you have not called yet is still a typo.

## 2.6 Which meaning a name has

Three things can give a name its meaning. When more than one does, the nearest wins.

<!-- file docs/book/code/def/shadow.oro -->
```go
fn double(n) { h(n, 2) }

export fn plain() { double(5) }

export fn a_parameter() { fn(double) { double(5) } }

export fn a_binding() { double = f; double(5) }
```

<!-- oro tutorial docs/book/code/def/shadow.oro -->
```go
plain =
fn() { h(5, 2) }
a_parameter =
fn() { fn(double) { double(5) } }
a_binding =
fn() { f(5) }
```

- In `plain`, `double` is the definition, and it unfolded.
- In `a_parameter`, `double` is the parameter. Nothing unfolds: the definition is not what is
  meant there.
- In `a_binding`, `double` is the block's binding, which is a parameter too (§1.12).

> **A parameter or a binding beats a definition.**

Nothing warns you when this happens, and that is a decision ([def.md §11](../spec/def.md)): the
meaning is the same on every target, and a program that shadows by mistake usually fails to type
check a line later. Prefer distinct names.

The third thing that can supply a name is the target, and there the order runs the other way.

## 2.7 The target's own beats a definition, and that is the design

`f` is one of the `tutorial` target's own names. Define it anyway:

<!-- file docs/book/code/def/override.oro -->
```go
fn f(n) { n * 2 }

export fn run() { f(5) }
```

<!-- oro tutorial docs/book/code/def/override.oro -->
```
note: f is defined here and provided natively by target "tutorial"; the target's is used
run =
fn() { f(5) }
```

The definition was not used. δ does not unfold a name the target provides, and the note is the
compiler saying so. That looks like a trap. It is the central mechanism of the language, and here
it is doing its work, on two targets that differ by one declaration
([targets/tutorial.oro](../../targets/tutorial.oro) and
[targets/tutorial-native.oro](../../targets/tutorial-native.oro)).

A library defines `area` out of arithmetic every target has ([code/geometry.oro](code/geometry.oro)):

<!-- part docs/book/code/geometry.oro -->
```go
pub fn area(w, h) { w * h }
```

A program uses the library, and names no target ([code/box.oro](code/box.oro)):

<!-- part docs/book/code/box.oro -->
```go
use geometry

export fn box(w, h) { geometry.area(w, h) }
```

On a target with no `geometry.area` of its own, the definition unfolds:

<!-- oro tutorial docs/book/code/box.oro -->
```go
box =
fn(w, h) { w * h }
```

On a target that provides `geometry.area`, the same two files stop one step earlier:

<!-- oro tutorial-native docs/book/code/box.oro -->
```
note: geometry.area is defined here and provided natively by target "tutorial-native"; the target's is used
box =
fn(w, h) { geometry.area(w, h) }
```

Chapter 1 said reduction runs until only the target's own operations remain. This is the other
half of that sentence: **which names are the target's own is the target's choice**, so the same
program reduces to different things on different targets, each as far as that target needs.

> **A definition is the fallback for a target that lacks the name.** Where a host already has the
> operation, tuned and native, the program reaches it; where it does not, the definition is the
> implementation.

That is how one source file becomes idiomatic code on four hosts without a line of it mentioning
a host. It is reported as a note, not an error, because being overridden is the intended outcome;
and it is reported at all because the file that decides it is one your program never names.

So the full order is:

> **parameter or binding**, then **the target**, then **definition**.

## 2.8 A definition may not mention itself

<!-- file docs/book/code/def/no-countdown.oro -->
```go
fn countdown(n) { if n < 1 { 0 } else { countdown(n - 1) } }

export fn run() { countdown(3) }
```

<!-- oro tutorial docs/book/code/def/no-countdown.oro -->
```
oro: docs/book/code/def/no-countdown.oro: countdown is recursive: it is defined in terms of itself, and recursion is not in the language, since δ unfolds a definition and a cycle has no normal form (ADR 0014). Iteration is `loop` with `again` (ADR 0015).
  If the self-reference was not deliberate, this definition is shadowing the countdown you meant.
```

**Recursion is not in the language**, and §2.3 is why. A definition is an abbreviation, and δ
replaces it by what it abbreviates. An abbreviation that contains itself never finishes being
written out, and since reduction is compilation, a term that never finishes reducing is a program
that never finishes compiling. It is refused at the definition, before anything reduces.

A cycle of any length is found the same way, through functions or values:

<!-- file docs/book/code/def/no-mutual.oro -->
```go
fn even?(n) { odd?(n) }

fn odd?(n) { even?(n) }

export fn run() { even?(3) }
```

<!-- oro tutorial docs/book/code/def/no-mutual.oro -->
```
oro: docs/book/code/def/no-mutual.oro: even?, odd? are mutually recursive, and recursion is not in the language, since δ unfolds a definition and a cycle has no normal form (ADR 0014). Iteration is `loop` with `again` (ADR 0015)
```

<!-- file docs/book/code/def/no-cycle.oro -->
```go
fn a() { b }

b = a

export fn run() { 1 }
```

<!-- oro tutorial docs/book/code/def/no-cycle.oro -->
```
oro: docs/book/code/def/no-cycle.oro: a, b are mutually recursive, and recursion is not in the language, since δ unfolds a definition and a cycle has no normal form (ADR 0014). Iteration is `loop` with `again` (ADR 0015)
```

There is a second reason, about the hosts. A recursive function emitted as a host function that
calls itself can go only as deep as that host's stack, and that depth differs by orders of
magnitude between Go, the JVM and a browser, none of which promises tail calls. One program and
one input would finish on one target and crash on another, which is the thing this language
exists to prevent ([ADR 0014](../decisions/0014-recursion-is-not-in-the-language.md)).

Repetition is `loop`, which the language owns and every target turns into its own loop:

<!-- file docs/book/code/def/loop.oro -->
```go
fn total(n) {
    loop i = 0, acc = 0 {
        i < n -> again(i + 1, h(acc, i))
        else  -> acc
    }
}

export fn run(n) { total(n) }
```

<!-- oro tutorial docs/book/code/def/loop.oro -->
```go
run =
fn(n) { loop i = 0, acc = 0 { i < n -> again(i + 1, h(acc, i)); else -> acc } }
```

`total` unfolded and the `loop` stayed: it is what runs.

The refusal's second sentence is for a case that has nothing to do with recursion:

<!-- file docs/book/code/def/no-size.oro -->
```go
fn size(v) { size(v) }

export fn run() { 3 }
```

That is rarely a recursive `size`. It is a `size` meant to call some *other* `size`, one you
forgot to `use` or spelled differently, and the definition being written took its name. The
message says so, and, as in §2.5, it says so though nothing calls `size`.

## 2.9 What reaches the host

`export` marks the definitions a host can call. `oro` prints them; a real target emits one host
function for each.

<!-- file docs/book/code/def/exports.oro -->
```go
export limit = 64

export fn double(n) { n * 2 }

fn used(n) { h(n, limit) }

fn never_used(n) { g(n) }

export fn run() { used(1) }
```

<!-- oro tutorial docs/book/code/def/exports.oro -->
```go
limit =
64
double =
fn(n) { n * 2 }
run =
fn() { h(1, 64) }
```

`used` is inside `run`, unfolded. `never_used` is nowhere. **A definition that is not exported
exists only in the places it was used.** Compiled for Go:

<!-- file docs/book/code/def/emitted.oro -->
```go
cap = 100

fn clamp(n) { if n > cap { cap } else { n } }

fn nobody_calls_this(n) { n + 1 }

export fn small(n ∈ 0..1000) (0..100) { clamp(n) }

export fn halved(n ∈ 0..1000) (0..100) { clamp(n / 2) }
```

<!-- gen go docs/book/code/def/emitted.oro -->
```go
func DefHalved(v0 int) int {
    v10 := (v0 >> 1)
    v12 := (v10 > 100)
    if v12 {
        return 100
    }
    return v10
}
…
func DefSmall(v0 int) int {
    v10 := (v0 > 100)
    if v10 {
        return 100
    }
    return v0
}
```

Two exports, two Go functions. There is no `clamp`, no `cap` and no `nobody_calls_this`: the
constant is the literal `100` in each place, and `clamp`'s body is in both. Each copy was then
compiled for where it stands, which is why one of them begins with a shift.

**Copying is real**, and you should know its price. A definition with a loop in it, used twice:

<!-- file docs/book/code/def/copies.oro -->
```go
fn sum_below(n ∈ 0..65536) (Int) {
    loop i = 0, acc = 0 {
        i < n -> again(i + 1, acc + i)
        else  -> acc
    }
}

export fn both(a ∈ 0..65536, b ∈ 0..65536) (Int) { sum_below(a) - sum_below(b) }
```

<!-- gen go docs/book/code/def/copies.oro -->
```go
func DefBoth(v0 int, v1 int) int {
    var v26 int = 0
    var v27 int = 0
    for ; ; v26 = (v26 + 1) {
        v28 := (v26 < v0)
        if v28 {
            v33 := (v27 + v26)
            v27 = v33
            continue
        }
        break
    }
    var v39 int = 0
    var v40 int = 0
    for ; ; v39 = (v39 + 1) {
        v41 := (v39 < v1)
        if v41 {
            v46 := (v40 + v39)
            v40 = v46
            continue
        }
        break
    }
    v49 := (v27 - v40)
    return v49
}
```

Two loops, because the name was used twice. Nothing ran twice that the source did not ask to run
twice; what doubled is the *code*. For most definitions that is the right trade: no call, and
each copy proven and optimized for its own arguments. Where it is not, the compiler may keep a
large definition as one host function, on a target and for a definition where it can prove that
changes no answer and no verdict
([ADR 0113](../decisions/0113-a-programs-definitions-are-kept-as-functions-where-the-target-prints-them.md));
the source says nothing either way, and means the same.

A definition can also be an alias, which costs nothing and leaves no trace:

<!-- file docs/book/code/def/alias.oro -->
```go
fn square(n) { h(n, n) }

area = square

export fn run() { area(3) }
```

<!-- oro tutorial docs/book/code/def/alias.oro -->
```go
run =
fn() { h(3, 3) }
```

---

## 2.10 Everything from nothing

Now the part that is not about this language.

The λ-calculus has no numbers, no booleans, no pairs and no lists. It has functions. In the 1930s
Church showed that this is no limitation: each of those can be *encoded* as a function. We have
`fn` and definitions, which is enough to try it, and there is a reason to try it here and not on
paper, which the end of this section is about.

The textbook names are mostly taken: `true` and `false` are literals here, and §2.4 forbids
redefining what the language owns. So the encodings below are spelled `yes` and `no`, `inv`,
`conj`, `disj`, `excl`. The shapes are Church's.

### Booleans

A boolean is a choice between two things. So *be* the choice:

<!-- file docs/book/code/def/bool.oro -->
```go
fn yes(t, e) { t }

fn no(t, e) { e }

fn inv(b) { b(no, yes) }

fn conj(p, q) { p(q, no) }

fn disj(p, q) { p(yes, q) }

fn excl(p, q) { p(q(no, yes), q) }

export fn yes_xor_yes(a, b) { excl(yes, yes)(a, b) }

export fn yes_xor_no(a, b) { excl(yes, no)(a, b) }

export fn no_or_no(a, b) { disj(no, no)(a, b) }

export fn yes_and_yes(a, b) { conj(yes, yes)(a, b) }

export fn not_yes() { inv(yes) }
```

<!-- oro tutorial docs/book/code/def/bool.oro -->
```go
yes_xor_yes =
fn(a, b) { b }
yes_xor_no =
fn(a, b) { a }
no_or_no =
fn(a, b) { b }
yes_and_yes =
fn(a, b) { a }
not_yes =
fn() { fn(t, e) { e } }
```

`yes` takes two things and gives the first; `no` gives the second. A conditional is then just
application, and each connective is one line. Read the answers by which argument came back: *yes
xor yes* chose the second, so it is `no`; *yes xor no* is `yes`; *not yes* reduced to `no` itself,
printed as the function it is. No boolean type was involved, and the language's own `true`,
`false` and `if` were not used.

### Pairs

A pair is something that hands both halves to whoever asks:

<!-- file docs/book/code/def/pair.oro -->
```go
fn pair(a, b) { fn(sel) { sel(a, b) } }

fn fst(p) { p(fn(a, b) { a }) }

fn snd(p) { p(fn(a, b) { b }) }

fn swap(p) { pair(snd(p), fst(p)) }

export fn first(u, v) { fst(pair(u, v)) }

export fn second(u, v) { snd(pair(u, v)) }

export fn swapped(u, v) { fst(swap(pair(u, v))) }
```

<!-- oro tutorial docs/book/code/def/pair.oro -->
```go
first =
fn(u, v) { u }
second =
fn(u, v) { v }
swapped =
fn(u, v) { v }
```

Build a pair, swap it, take the first: `v`. No pair was ever built.

### Numbers

A Church numeral is *apply this function n times*:

<!-- file docs/book/code/def/numerals.oro -->
```go
fn zero(s, z) { z }

fn two(s, z) { s(s(z)) }

fn three(s, z) { s(s(s(z))) }

fn plus(m, n) { fn(s, z) { m(s, n(s, z)) } }

fn mult(m, n) { fn(s, z) { m(fn(a) { n(s, a) }, z) } }

export fn show_three() { three(g, 0) }

export fn show_zero() { zero(g, 0) }

export fn two_plus_three() { plus(two, three)(g, 0) }

export fn two_times_three() { mult(two, three)(g, 0) }

export fn really_six() { mult(two, three)(fn(n) { n + 1 }, 0) }
```

<!-- oro tutorial docs/book/code/def/numerals.oro -->
```go
show_three =
fn() { g(g(g(0))) }
show_zero =
fn() { 0 }
two_plus_three =
fn() { g(g(g(g(g(0))))) }
two_times_three =
fn() { g(g(g(g(g(g(0)))))) }
really_six =
fn() { 6 }
```

To see what a numeral is, hand it a successor and a zero. With the opaque `g` you can count: five
`g`s for 2 + 3, six for 2 × 3. With a real successor, `n + 1`, the six applications fold, and the
answer is the integer `6`: arithmetic done by a compiler that was never told what addition of
these numerals is.

### Lists

A list is the function that folds itself (the Böhm–Berarducci encoding, 1985; the numerals are its
special case):

<!-- file docs/book/code/def/list.oro -->
```go
fn nil(c, n) { n }

fn cons(hd, tl) { fn(c, n) { c(hd, tl(c, n)) } }

fn total(l) { l(fn(a, b) { h(a, b) }, 0) }

fn mapl(k, l) { fn(c, n) { l(fn(a, b) { c(k(a), b) }, n) } }

export fn summed() { total(cons(1, cons(2, cons(3, nil)))) }

export fn mapped(u, v) { total(mapl(g, cons(u, cons(v, nil)))) }
```

<!-- oro tutorial docs/book/code/def/list.oro -->
```go
summed =
fn() { h(1, h(2, h(3, 0))) }
mapped =
fn(u, v) { h(g(u), h(g(v), 0)) }
```

**Look at what is not there.** No list, no cells, and no second list between the `mapl` and the
`total`. In a language with lists at run time, `mapped` allocates a list, walks it to build
another, and walks that. Here the two traversals became one expression and the structure never
existed. Compiler writers call that *deforestation* (Wadler 1990) and write passes to do it; here
it is β and δ, because the list was only ever a way of writing the program.

### Combinators

Three functions generate everything (Schönfinkel 1924; Curry):

<!-- file docs/book/code/def/ski.oro -->
```go
fn I(a) { a }

fn K(a) { fn(b) { a } }

fn S(a) { fn(b) { fn(c) { a(c)(b(c)) } } }

export fn skk() { S(K)(K) }

export fn skk_of(v) { S(K)(K)(v) }
```

<!-- oro tutorial docs/book/code/def/ski.oro -->
```go
skk =
fn() { fn(c) { c } }
skk_of =
fn(v) { v }
```

`S(K)(K)` reduced to `fn(c) { c }`, the identity. The compiler proved *SKK = I* with no argument
in sight, because reduction goes inside functions (§1.9).

### Where it stops

Two limits, and both are honest.

**Arity is exact** (§1.8). The textbook calculus is curried, every function taking one argument,
and some classic encodings lean on it. Exponentiation, *exp m n = n m*, applies a two-parameter
numeral to one argument:

<!-- file docs/book/code/def/no-exp.oro -->
```go
fn two(s, z) { s(s(z)) }

fn exp(m, n) { n(m) }

export fn run() { exp(two, two)(g, 0) }
```

<!-- oro tutorial docs/book/code/def/no-exp.oro -->
```
oro: arity: expects 2 argument(s), given 1, in: fn() { fn(s, z) { s(s(z)) } }
```

Nothing is lost: write the currying yourself, as `S` and `K` do above, and everything works.
Kleene's predecessor, famously the hard one, with curried numerals:

<!-- file docs/book/code/def/pred.oro -->
```go
fn three(s) { fn(z) { s(s(s(z))) } }

fn pred(n) {
    fn(s) {
        fn(z) { n(fn(p) { fn(q) { q(p(s)) } })(fn(u) { z })(fn(u) { u }) }
    }
}

export fn run() { pred(three)(g)(0) }
```

<!-- oro tutorial docs/book/code/def/pred.oro -->
```go
run =
fn() { g(g(0)) }
```

Two.

**A term with no normal form does not compile.** The fixed-point combinator Y is how the untyped
calculus gets recursion without naming anything:

<!-- file docs/book/code/def/no-y.oro -->
```go
fn Y(k) { fn(a) { k(a(a)) }(fn(a) { k(a(a)) }) }

export fn run() { Y(f) }
```

<!-- oro tutorial docs/book/code/def/no-y.oro -->
```
oro: reduction did not terminate within the step limit; last term head: fn() { fn(k) { fn(a) { a } } }
```

No definition there mentions itself, so §2.8's check has nothing to refuse; but `Y(f)` unfolds to
`f(f(f(…)))` without end, and the step limit is the guard in front of that. It is the same fact as
§2.8, met from the other side: recursion is not a thing you can smuggle in, because what is missing
is not a keyword but a normal form.

### The encoding that is in the library

This section is not a curiosity. Look at the pair again, and then at three lines of
[lib/num/vec.oro](../../lib/num/vec.oro), which is library code:

<!-- part lib/num/vec.oro -->
```go
pub fn vec(n, f) { fn(sel) { sel(n, f) } }
pub fn vlen(v) { v(fn(n, f) { n }) }
pub fn vindex(v, i) { v(fn(n, f) { f })(i) }
```

**That is a Church pair**: a length and a function from index to element. Give it a use:

<!-- file docs/book/code/def/vec.oro -->
```go
fn vec(n, f) { fn(sel) { sel(n, f) } }

fn vlen(v) { v(fn(n, f) { n }) }

fn vindex(v, i) { v(fn(n, f) { f })(i) }

fn scale(v, k) { vec(vlen(v), fn(i) { k * vindex(v, i) }) }

export fn element() { vindex(scale(vec(10, fn(i) { h(i, i) }), 3), 4) }

export fn length() { vlen(scale(vec(10, fn(i) { h(i, i) }), 3)) }
```

<!-- oro tutorial docs/book/code/def/vec.oro -->
```go
element =
fn() { 3 * h(4, 4) }
length =
fn() { 10 }
```

Element 4 of a ten-element vector scaled by 3 compiles to `3 * h(4, 4)`. The vector was never
built, the scaling never ran over anything, and the length is a constant.

Church's encodings are usually taught as a proof that functions are enough, and then dismissed as
hopelessly slow. That is true where they run. **An encoding costs nothing when the thing that takes
it apart is known at compile time**, and here it always is, since reduction continues until only
the target's operations are left. The language's own tuples, variants and tables by a rule
(chapter 0) are this idea, given syntax.

---

## What to remember

- **A definition names a term**: `name = term`, or `fn name(…) { … }` for a function. There is no
  keyword, and it is not a computation.
- **δ: a name is replaced by its term, at every use.** A definition is copied; a binding in a
  block is shared.
- **A definition with an effect in it is refused**, since copying it would repeat the effect. Make
  it a function, or bind it where it is used.
- **A file's definitions are a set.** Order does not matter; a name is defined once; there is no
  overloading, of names or of operators.
- **Every definition is checked**, used or not.
- **A parameter or a binding beats the target's own name, which beats a definition.** The last is
  the design: a definition is the fallback for a target that lacks the name.
- **No definition may mention itself**, directly or round a cycle. An abbreviation that contains
  itself has no end; repetition is `loop`.
- **Only `export`s reach the host.** Every other definition exists only where it was used, and a
  definition used twice is code written twice.
- **Functions and definitions alone can encode booleans, pairs, numbers and lists**, and here the
  encodings cost nothing, because whatever takes one apart is known at compile time.

Next: [modules](03-modules.md), how a name in another file is reached.
