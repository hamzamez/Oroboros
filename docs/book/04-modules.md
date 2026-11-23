# 4. Modules

Two chapters, two bricks. [`fn`](02-fn.md) makes a function, and a [definition](03-def.md) gives a
term a name. Everything so far has lived in one file, and every name has been one word.

This chapter is the third brick: **where a name lives.** It adds three words, `use`, `pub` and
`module`, and one character, `.`.

It also answers a question chapter 3 left open. §3.7 said the target's own name beats a
definition, and called that the design. It did not say how a library's definition and a target's
operation can be *the same name* in the first place. This chapter is that mechanism.

The examples are files in [code/](code/), run as before:

```bash
go run ./cmd/oro -j -target=tutorial docs/book/code/area.oro
```

Two targets appear: `tutorial`, from chapters 2 and 3, and `tutorial-native`, which is the same
target with one more declaration; §4.10 is what that declaration does. A file named `no-…` is
meant to be refused. `go run docs/book/check.go docs/book/04-modules.md` re-runs every block.
Checked 2026-11-15.

---

## 4.1 A file is a module

Here is a file, [code/geometry.oro](code/geometry.oro):

<!-- file docs/book/code/geometry.oro -->
```go
pub fn area(w, h) { w * h }

pub fn perimeter(w, h) { 2 * (w + h) }

pub fn scale(k, w, h) { area(k * w, k * h) }

fn twice(n) { 2 * n }
```

And another, beside it, that uses it:

<!-- file docs/book/code/area.oro -->
```go
use geometry

export fn area(u, v) { geometry.area(u, v) }

export fn around(u, v) { geometry.perimeter(u, v) }

export fn scaled(u, v) { geometry.scale(2, u, v) }
```

<!-- oro tutorial docs/book/code/area.oro -->
```go
area =
fn(u, v) { u * v }
around =
fn(u, v) { 2 * (u + v) }
scaled =
fn(u, v) { 2 * u * (2 * v) }
```

Four things happened.

- **`geometry.oro` is the module `geometry` because that is its name.** It has no header, no
  `package` line and no `module` declaration. A file is a module, named by its path.
- **`use geometry` found the file**, beside the one that asked for it.
- **`geometry.area` names a member of it.** The dot separates a module from one of its
  definitions.
- **The definition unfolded exactly as in chapter 3.** δ does not care which file a name came
  from. `scale` calls `area` by its plain name, since inside a module its definitions see each
  other unqualified, and both unfolded.

`area.oro` defines its own `area`, and that is no conflict with `geometry.area`: they are
different names. A module is a namespace.

## 4.2 Paths

A module in a directory is named by its path, with `/`:

<!-- file docs/book/code/shapes/circle.oro -->
```go
use math/trig

pub fn area(r) { 3 * (r * r) }

pub fn point_at(r, ang) { r * trig.cos(ang) }
```

<!-- file docs/book/code/round.oro -->
```go
use shapes/circle

export fn run(r) { circle.area(r) }

export fn point(r, a) { circle.point_at(r, a) }
```

<!-- oro tutorial docs/book/code/round.oro -->
```go
run =
fn(r) { 3 * (r * r) }
point =
fn(r, a) { r * `math/trig`.cos(a) }
```

- **`shapes/circle` is the file `shapes/circle.oro`.** Nesting is directories; the language has
  none of its own.
- **`use shapes/circle` binds the last segment**, `circle`. The path is the module's identity;
  the last segment is the local name it is used by.
- **`/` separates a path's segments, and `.` separates a module from a member.** They are
  different characters doing different jobs, which is why chapters 2 and 3 refused `a.b` as a
  parameter and as a definition's name: it already means *member b of module a*.

One spelling rule, because you will meet it. A file's name may hold a hyphen; a name in a program
may not, since `-` there is subtraction. So a `-` in a path is written `_` wherever the program
uses the name:

<!-- file docs/book/code/noisy-prog.oro -->
```go
use noisy-lib

export fn main() { noisy_lib.k(x()) }
```

<!-- oro tutorial docs/book/code/noisy-prog.oro -->
```go
main =
fn() { x() * 3 }
```

The `use` spells the path as the file system does, `noisy-lib`; the program calls it `noisy_lib`.

## 4.3 `use`

```go
use PATH            // binds PATH's last segment
use PATH as NAME    // binds NAME
```

<!-- file docs/book/code/alias.oro -->
```go
use geometry as g
use geometry

export fn short(u, v) { g.area(u, v) }

export fn either(u, v) { h(geometry.area(u, v), g.perimeter(u, v)) }
```

<!-- oro tutorial docs/book/code/alias.oro -->
```go
short =
fn(u, v) { u * v }
either =
fn(u, v) { h(u * v, 2 * (u + v)) }
```

Two names for one module are allowed, if a little pointless. Like definitions, the `use` lines are
part of a set: one may come after the code that needs it
([code/late.oro](code/late.oro) puts it last, and reduces the same).

**Imports stay qualified.** There is no `from geometry import area`, no `import *`, and no way to
make a bare `area` mean `geometry.area`:

<!-- file docs/book/code/no-unqualified.oro -->
```go
use geometry

export fn run(u, v) { area(u, v) }
```

<!-- oro tutorial docs/book/code/no-unqualified.oro -->
```
oro: docs/book/code/no-unqualified.oro: in run: area is not bound — it is not a parameter, not a definition, and not a primitive on this target
```

That is a refusal on purpose. In this language what a name means can already depend on the target
you build for (§4.10). An unqualified import would add a second, invisible source of the same
dependence: a bare `area` whose meaning changes when a library you use adds a definition.
Qualified, every name says where it came from, at the place it is used.

Forgetting the `use` is an error that names the repair:

<!-- file docs/book/code/no-import.oro -->
```go
export fn run(u, v) { geometry.area(u, v) }
```

<!-- oro tutorial docs/book/code/no-import.oro -->
```
oro: docs/book/code/no-import.oro: run: geometry is not imported: add `use geometry` or `use PATH as geometry`
```

And one local name for two modules is the one naming conflict the system can produce:

<!-- file docs/book/code/no-clash.oro -->
```go
use geometry as g
use shapes/circle as g

export fn run(u) { g.area(u) }
```

<!-- oro tutorial docs/book/code/no-clash.oro -->
```
oro: docs/book/code/no-clash.oro: the program binds g to both geometry and shapes/circle — give one of them a different alias, `use … as …`
```

Both modules really do have an `area`, so `g.area` would have meant whichever import was noticed
first.

## 4.4 `pub`: what another module may name

`geometry` defines four things and marks three `pub`. The fourth is private:

<!-- file docs/book/code/no-private.oro -->
```go
use geometry

export fn run(u) { geometry.twice(u) }
```

<!-- oro tutorial docs/book/code/no-private.oro -->
```
oro: docs/book/code/no-private.oro: run: module geometry defines twice and does not make it `pub`: a declaration is private to its module unless marked `pub` (ADR 0078)
```

A name that was never there gets a different message, because it is a different mistake:

<!-- file docs/book/code/no-member.oro -->
```go
use geometry

export fn run(u) { geometry.volume(u, u) }
```

<!-- oro tutorial docs/book/code/no-member.oro -->
```
oro: docs/book/code/no-member.oro: run: module geometry has no member volume
```

- **Private is the default.** A module's interface is exactly what it marks
  ([ADR 0078](../decisions/0078-a-modules-interface-is-what-it-marks-pub.md)). `pub` goes on a
  function, a value or a `set`.
- **`pub` controls only the outside view.** Inside the module every definition is in scope: a
  `pub` function may call a private one, and the private one is unfolded into the caller's code
  like any other.
- **Case means nothing.** There is no capital-letter rule as in Go, and no leading underscore as
  in Python.

**`pub` and `export` answer different questions**, and neither implies the other. `pub`: *may
another module name this?* `export`: *does the host get a function for this?* (§4.5). `box.oro`
exports `box` and does not mark it `pub`:

<!-- file docs/book/code/no-exported.oro -->
```go
use box

export fn run(u) { box.box(u, u) }
```

<!-- oro tutorial docs/book/code/no-exported.oro -->
```
oro: docs/book/code/no-exported.oro: run: module box defines box and does not make it `pub`: a declaration is private to its module unless marked `pub` (ADR 0078)
```

## 4.5 The file you run, and the files it uses

Any `.oro` file can be either of two things, and which one is decided by how it was reached:

> The **entry file** is the one named on the command line. A **library file** is one reached from
> it by `use`.

They are roles, not kinds. `geometry.oro` is a library when `area.oro` uses it, and an entry file
when you run it directly; then it has no `export`, and nothing to print.

| | entry file | library file |
|---|---|---|
| found by | being named | its path: beside the entry file, then on the search path (`lib/`) |
| its `export`s | the program's entry points | not the program's |
| `export fn main()` | where a built program starts | ignored |

A library with entry points of its own:

<!-- file docs/book/code/lib-main.oro -->
```go
pub fn k(n) { n }

export fn main() { 1 }

export fn other() { 2 }
```

<!-- file docs/book/code/uses-lib.oro -->
```go
use lib-main

export fn run() { lib_main.k(5) }
```

<!-- gen go docs/book/code/uses-lib.oro -->
```go
func CodeRun() int {
    return 5
}
```

One Go function, the entry file's. `lib-main`'s `main` and `other` are not there. **`use` is a
dependency, not an inclusion**: C's `#include` pastes a file in and everything in it becomes
yours; `use` says *this program needs that module*, and nothing crosses but the definitions you
name, each unfolded where you named it.

The same rule decides where a program starts. A library's `main` does not give you one:

<!-- file docs/book/code/no-entry.oro -->
```go
use lib-main

fn unused() { lib_main.k(1) }
```

<!-- build go docs/book/code/no-entry.oro -->
```
build: docs/book/code/no-entry.oro has no entry point: a program needs `export fn main() { … }`
```

Otherwise adding a dependency could change where your program begins.

## 4.6 Several modules in one file

A file reached by `use` is one module. The entry file may hold several, each in a `module` block:

<!-- file docs/book/code/diamond.oro -->
```go
module left {
    use geometry

    pub fn twice_area(w, h) { 2 * geometry.area(w, h) }
}

module right {
    use geometry

    pub fn half_area(w, h) { geometry.area(w, h) / 2 }
}

module top {
    use left
    use right

    export fn both(w, h) { left.twice_area(w, h) + right.half_area(w, h) }
}
```

<!-- oro tutorial docs/book/code/diamond.oro -->
```go
top.both =
fn(w, h) { 2 * (w * h) + w * h / 2 }
```

- **Each block is a module like any other**: its own `use` lines, its own `pub` marks, its own
  namespace. `top` must `use left` to name it, although they share a file.
- **An export in a module carries the module's name**, `top.both`.
- **This is a diamond**: `top` reaches `geometry` by two routes. Nothing about that is special.
  There is one `geometry.area`, and two paths to it agree because there is only one thing there.

**A block's scope ends at its brace.** What is written outside every block belongs to the file's
own module, and that module is like any other: it sees a block only by `use`.

<!-- file docs/book/code/after-blocks.oro -->
```go
module inner {
    pub fn k(u) { h(u, 1) }
}

use inner

export fn run(u) { inner.k(u) }
```

<!-- oro tutorial docs/book/code/after-blocks.oro -->
```go
run =
fn(u) { h(u, 1) }
```

Without the `use` line, `run` cannot name `inner`, although it is a few lines up in the same file:

<!-- oro tutorial docs/book/code/no-after-blocks.oro -->
```
oro: docs/book/code/no-after-blocks.oro: run: inner is not imported: add `use inner` or `use PATH as inner`
```

Two modules may give their members the same name, and nothing needs renaming:

<!-- file docs/book/code/same-name.oro -->
```go
module alpha {
    pub fn area(n) { n * n }
}

module beta {
    pub fn area(n) { n + n }
}

module top {
    use alpha
    use beta

    export fn both(n) { h(alpha.area(n), beta.area(n)) }
}
```

<!-- oro tutorial docs/book/code/same-name.oro -->
```go
top.both =
fn(n) { h(n * n, n + n) }
```

That is the whole benefit of qualified names, and it is why §4.3's alias clash is the only naming
conflict there is.

## 4.7 Modules may use each other

<!-- file docs/book/code/cyc/a.oro -->
```go
use cyc/b
pub fn f(n) { b.g(n) }
```

<!-- file docs/book/code/cyc/b.oro -->
```go
use cyc/a
pub fn g(n) { n * 2 }
```

<!-- file docs/book/code/cycle.oro -->
```go
use cyc/a

export fn run(u) { a.f(u) }
```

<!-- oro tutorial docs/book/code/cycle.oro -->
```go
run =
fn(u) { u * 2 }
```

`cyc/a` uses `cyc/b`, which uses `cyc/a`. Imports are followed until nothing new is found, and a
module already loaded is not read again, so the cycle ends by itself. There are no header guards,
no forward declarations, and no order to get right.

Two modules that *use* each other are fine. Two definitions that *mention* each other are still
recursion, and still refused (§3.8). They are different questions: one is about where names are
found, the other about whether unfolding ends.

## 4.8 What reduction sees: nothing

> **Modules are resolution, not reduction**
> ([ADR 0011](../decisions/0011-modules-add-nothing-to-the-reducer.md)). Before reduction starts,
> every name has been replaced by its full, qualified one, and there are no modules left. β and δ
> cannot tell that they ever existed.

You have already seen the evidence. `circle.point_at` calls `trig.cos`, and §4.2's output says:

<!-- oro tutorial docs/book/code/round.oro -->
```go
point =
fn(r, a) { r * `math/trig`.cos(a) }
```

`` `math/trig`.cos ``: the **full path**, not the `trig` the source wrote. (It is in backquotes
because a path with a `/` is not an identifier.) An alias is a convenience of one file, and
resolution erases it. What reaches reduction is one flat set of fully named definitions, as if
you had written every name out by hand.

This is why chapters 2 and 3 needed nothing added. δ unfolds `geometry.area` for the reason it
unfolds `square`: it is a name with a definition.

## 4.9 A module the target provides

<!-- file docs/book/code/trig.oro -->
```go
use math/trig

export fn run(u) { trig.sin(u) }
```

<!-- oro tutorial docs/book/code/trig.oro -->
```go
run =
fn(u) { `math/trig`.sin(u) }
```

There is no `math/trig.oro` anywhere. The `use` found no file, **and that is not an error**:
`math/trig` is a module the *target* supplies. In [targets/tutorial.oro](../../targets/tutorial.oro):

<!-- part targets/tutorial.oro -->
```go
module math/trig {
    fn sin(a ∈ num) (num) pure { host expr "sin(${a})" }
    fn cos(a ∈ num) (num) pure { host expr "cos(${a})" }
}
```

Each of those is a declaration with a host's code for a body, which is what a target is made of.
Reduction stops at `math/trig.sin` because it is the target's own, chapter 2's rule unchanged.
Chapter 1's `use go/strings` was this: Go's `strings` package, as a module the `go` target
provides.

But *no file* is also what a typo looks like:

<!-- file docs/book/code/no-typo.oro -->
```go
use geometrie

export fn run(u) { geometrie.area(u, u) }
```

<!-- oro tutorial docs/book/code/no-typo.oro -->
```
oro: docs/book/code/no-typo.oro: in run: geometrie.area is not bound — it is not a parameter, not a definition, and not a primitive on this target
  `use geometrie` matched no file on the search path and this target provides no module geometrie either, so every name from it is unbound. Check the path.
```

The first line blames `geometrie.area`; the second says the trouble is the half before the dot. A
module that does exist and lacks the member gets no such hint, because there the first line is
right:

<!-- file docs/book/code/no-tan.oro -->
```go
use math/trig

export fn run(u) { trig.tan(u) }
```

<!-- oro tutorial docs/book/code/no-tan.oro -->
```
oro: docs/book/code/no-tan.oro: in run: `math/trig`.tan is not bound — it is not a parameter, not a definition, and not a primitive on this target
```

## 4.10 The four cells

Everything in this chapter has been building one table. A qualified name can be **defined by a
library**, **provided by a target**, both, or neither:

| | the target provides it | the target does not |
|---|---|---|
| **a library defines it** | the target's is used; the definition is the fallback | the definition unfolds |
| **nothing defines it** | it is the target's; reduction stops | **refused**: not bound |

Three of the cells you have seen: `geometry.area` on `tutorial` (top right), `math/trig.sin`
(bottom left), `geometrie.area` (bottom right). The fourth is the one the language is for.
[code/box.oro](code/box.oro) names no target:

<!-- file docs/book/code/box.oro -->
```go
use geometry

export fn box(w, h) { geometry.area(w, h) }
```

<!-- oro tutorial docs/book/code/box.oro -->
```go
box =
fn(w, h) { w * h }
```

<!-- oro tutorial-native docs/book/code/box.oro -->
```
note: geometry.area is defined here and provided natively by target "tutorial-native"; the target's is used
box =
fn(w, h) { geometry.area(w, h) }
```

One source, two targets, two results: the library's arithmetic on one, a call to the target's own
implementation on the other. The whole difference between the two target files is one
declaration:

<!-- part targets/tutorial-native.oro -->
```go
module geometry {
    fn area(a ∈ num, b ∈ num) (num) pure { host expr "area(${a}, ${b})" }
}
```

That is what §3.7 promised and could not show, and it works because of a rule this chapter has
assumed throughout:

> **One namespace.** A target and a library name into the *same* space of qualified names. The
> target declares `geometry.area`; the library defines `geometry.area`; they are one name.

Were they separate, no library definition could ever coincide with a target's operation, every
program would quietly get the portable fallback, and no host's own implementation would be
reachable. Everything would still compile and run, and the language would never do the one thing
it exists to do.

The portable libraries are built this way. `io`, from chapter 1, is a module with no definitions
at all, only what each target provides for it, written beside the library and not in the target:

<!-- part lib/io/go.oro -->
```go
provides go io {
    use io
    fn print_line(s ∈ String) (String) { host stmt "fmt.Println(${s})" import "fmt" }
```

`provides go io { … }` says: *on the target `go`, the module `io` has these.* There is one such
file for each target `io` supports, and a program that says `io.print_line` gets Go's
`fmt.Println` on one and `console.log` on another, having named neither. A name provided this
way may also be a definition over the others, which is what `use io` inside the block is for:
`io.print_int` is defined there as `io.print_line` of the integer's text.

---

## 4.11 What a module is not

Modules here are much less than the word usually promises, and the gaps hold weight.

### Not a unit of compilation

In Modula-2, Ada and most languages since, a module is something compiled *separately*: compile it
once, ship the result, and a user needs only its interface. That is what modules were invented
for, and this language does not do it.

It cannot. Reduction needs the *body* of `geometry.area` to unfold it, and unfolding is the whole
compilation strategy; a helper costs nothing because it disappears into its caller. An interface
without a body would preserve exactly the call the language exists to remove.

So compilation is **whole-program**, and that is a real trade: no compiled library to hand
someone, and build time that grows with everything you use. What it buys is chapter 2's free
abstraction. It is right *here*, because parity with hand-written code is the requirement.

### Not a value

You cannot pass a module to a function, store one or return one. `use` takes a path written out,
and nothing at reduction time can refer to a module, which is §4.8 said as a limitation.

### No functors, and none needed

ML's parameterised module is the **functor**: a function from modules to modules, with its own
typing and its own syntax, and a reputation as the hardest part of the language. We cannot have
one, since a module is not a value. But look at what a functor *does*: it takes the operations you
are parameterised over and gives back what you built from them. That is a function returning a
function, and chapter 2 had those.

<!-- file docs/book/code/functor.oro -->
```go
fn accumulate(combine, unit) {
    fn(a, b, c) { combine(combine(combine(unit, a), b), c) }
}

fn add(p, q) { p + q }

fn mul(p, q) { p * q }

export fn sum(u, v, w) { accumulate(add, 0)(u, v, w) }

export fn product(u, v, w) { accumulate(mul, 1)(u, v, w) }

export fn opaque(u, v, w) { accumulate(h, 0)(u, v, w) }
```

<!-- oro tutorial docs/book/code/functor.oro -->
```go
sum =
fn(u, v, w) { 0 + u + v + w }
product =
fn(u, v, w) { 1 * u * v * w }
opaque =
fn(u, v, w) { h(h(h(0, u), v), w) }
```

One definition, instantiated at addition, at multiplication, and at an operation the compiler
knows nothing about. In algebra's words, `accumulate` is written over any monoid, and each line
hands it one. **Nothing survives the application**: no dictionary, no table of functions, no
indirection, and no mention of `accumulate`.

A structure with several members works the same way, because §3.10's Church pair holds as many
things as you like:

<!-- file docs/book/code/vec2.oro -->
```go
fn add(p, q) { p + q }

fn mul(p, q) { p * q }

fn make_vec2(plus, times) {
    fn(a, b) { fn(sel) { sel(a, b, plus, times) } }
}

fn dot2(v, w) {
    v(fn(a, b, plus, times) {
        w(fn(c, d, _, _) { plus(times(a, c), times(b, d)) })
    })
}

export fn ordinary(p, q, r) { dot2(make_vec2(add, mul)(p, q), make_vec2(add, mul)(r, 4)) }

export fn strange(p, q, r) { dot2(make_vec2(h, mul)(p, q), make_vec2(h, mul)(r, 4)) }
```

<!-- oro tutorial docs/book/code/vec2.oro -->
```go
ordinary =
fn(p, q, r) { p * r + q * 4 }
strange =
fn(p, q, r) { h(p * r, q * 4) }
```

`make_vec2` takes an addition and a multiplication and gives back a maker of vectors that carry
both; `dot2` is a client of that structure. What is left is a dot product with the arithmetic put
in, and the structure and the parameterisation are gone.

**So what costs ML a second language costs nothing here, because it was already there.** What
modules give that functions cannot is the thing functors were never about: a namespace a target
can also write into. That is §4.10, and it is the reason modules exist.

### Not `import *`, not `#include`, not a package manager

- No unqualified import, by refusal (§4.3).
- `use` is a dependency, not a textual inclusion (§4.5).
- A module is found on a search path, the entry file's own directory and then `lib/` (or what
  `-path` names), and that is all. Versions, fetching and publishing do not exist and are not
  designed.

---

## What to remember

- **A file is a module, named by its path.** It has no header.
- **`.` separates a module from a member; `/` separates a path's segments.** A `-` in a file's
  name is `_` in the program.
- **`use PATH` binds the last segment; `use PATH as N` binds `N`.** Imports stay qualified, always.
- **`pub` is the interface, and private is the default.** `export` is a different question: what
  the host gets.
- **The entry file is the one you name; a library is one a `use` reached.** Only the entry file's
  exports are the program's, and only its `main` starts it.
- **The entry file may hold several modules**, each in a `module { … }` block.
- **Modules may use each other in a cycle.** Definitions still may not.
- **Modules are resolution, not reduction.** They are gone before β and δ start, and what is left
  shows full paths.
- **A `use` that finds no file names a module the target provides**, and a misspelled path is
  reported as that.
- **One namespace for targets and libraries.** That is what lets a library's definition be the
  fallback for a target's own operation.
- **A functor is a function.** Modules stayed small because `fn` had already done the hard part.

Next: [choice](05-choice.md), the first piece of data: a value that is one of two, and what a
program does with it.
