# 0. Syntax

This chapter exists because of one line of punctuation.

```lisp
(square 7)
```

If you are coming from Python, JavaScript, Java, C, Go, Rust or almost anything else, you have
written that expression ten thousand times as `square(7)`, and the version above looks like a
different kind of language. It is not. **One character moved.** The opening parenthesis went from
after the function's name to before it, and everything else in this chapter follows from that
single move.

Nothing here is unique to Oroboros. The notation is sixty-six years old, it is in daily use well
outside the Lisp family, and the arguments for and against it are a literature. This chapter is
that notation: where it came from, what it costs, what it buys, what we do with it, and —
specifically — the one thing it buys *this* language that no other notation would.

Every error message and every reduction below was produced by running the example:

```bash
go run ./cmd/oro -target=tutorial FILE.oro
```

`tutorial` is a target that exists to teach ([targets/tutorial.oro](../../targets/tutorial.oro)).
It spells arithmetic `+ - * /`, and declares `f`, `g`, `h` as opaque functions and `x`, `y`, `z` as
opaque values, so an example can show a *shape* without showing an implementation.

---

## 0.1 One character moves

Here is the whole translation, in both directions:

| you have written | we write |
|---|---|
| `square(7)` | `(square 7)` |
| `max(a, b)` | `(max a b)` |
| `print("hi")` | `(print "hi")` |
| `f(g(x))` | `(f (g x))` |
| `a + b` | `(+ a b)` |
| `a + b * c` | `(+ a (* b c))` |
| `x < y` | `(< x y)` |
| `trig.sin(x)` | `(trig.sin x)` |
| `len(xs)` | `(len xs)` |

Read `(f a b)` out loud as **"f of a and b"**. That is all it ever means. If you can read
`f(a, b)`, you can read this; the comma is gone and the `f` is inside.

The rule in one sentence:

> **Move the function's name inside the parenthesis, and delete the commas.**

Try it on something with real nesting. A conditional expression, in a language you know:

```python
def f(a):
    y = h(g(a), 1)
    return y + 1
```

```lisp
(fn (a) (let y (h (g a) 1) (+ y 1)))    ⟶   (fn (a) (+ (h (g a) 1) 1))
```

Same tree, same shape, same order. The parentheses even sit in the same *places* — count them. (The
reduction on the right is what the compiler prints: `let` is an application, so `y` was substituted.
[Chapter 1](01-fn.md) is about why.)

## 0.2 Application is the only compound form

In Python, `f(x)` is a call, `a + b` is a binary operation, `a[i]` is a subscript, `a.b` is an
attribute access, `if/else` is a statement, `[1,2,3]` is a display, `lambda` is an expression, and
`for` is something else again. Eight kinds of syntax, each with its own grammar rule.

Here there is **one** compound form. A parenthesised list is a function applied to arguments, and
that is the only compound thing a program can contain:

```lisp
(square 7)              ; a call
(+ a b)                 ; also a call
(a i)                   ; indexing a table — also a call
(if c then else)        ; also a call
(fn (n) (* n n))        ; also a call (of the form `fn`)
(loop ((i 0)) … )       ; also a call
```

This is not a trick of presentation. It is what the compiler sees. Reduce the `loop` from §0.11 and
watch the sugar come off:

```lisp
(def sum-to (n)
  (loop ((acc 0) (i 0))
    (< i n)  (again (+ acc i) (+ i 1))
    else     acc))
```

```lisp
sum-to =
(fn (n) (loop (fn (acc i) (if (< i n) (again (+ acc i) (+ i 1)) acc)) 0 0))
```

The variable list `((acc 0) (i 0))` became **a function of `acc` and `i`, applied to `0` and `0`**.
The clause chain became a nested `if`. There was never a `loop` statement; there was a name applied
to three arguments, one of which happened to be a function.

That uniformity is the point of the notation, and §0.17 is where it pays.

## 0.3 Count the parentheses, exactly

The complaint about this notation is always the parentheses, so let us count them. Take the example
that makes the point best — a nested conditional:

```lisp
(if cond1 (if cond2 A B) C)
```

In a language where `if` is an expression you call, the same tree is:

```
if(cond1, if(cond2, A, B), C)
```

| notation | written | `(` `)` | separators |
|---|---|---|---|
| name outside the parenthesis | `if(cond1, if(cond2, A, B), C)` | **4** | 4 commas |
| name inside the parenthesis | `(if cond1 (if cond2 A B) C)` | **4** | 0 |

**The parenthesis count is identical, and the familiar notation needs four more characters of
punctuation to get there.** This is not a coincidence about this example; it is arithmetic. For an
expression tree with *n* internal nodes of arities *a₁…aₙ*:

```
name inside:    2n  parentheses,  0 separators                 =  2n         delimiters
name outside:   2n  parentheses,  Σ(aᵢ − 1) commas             =  2n + Σ(aᵢ−1) delimiters
```

Both pay `2n` for the nesting, because *nesting* is what parentheses are for. The second pays again,
once per argument after the first, because having moved the head outside it now has to separate the
arguments from each other.

> **Parentheses are the price of nesting, not the price of Lisp.** Every notation that lets an
> argument be an arbitrary expression pays `2n`. You have been paying it all along; you were just
> paying it with the head on the other side and with commas on top.

The honest version of the complaint is not about the count. It is about **runs** — `))))` at the end
of a deeply nested form — and about the fact that in `(f x y)` nothing marks `f` as the head, where
in `f(x, y)` the parenthesis does. Both are real, both are about *reading*, and §0.15 treats them as
real.

## 0.4 What about postfix? (The question that settles it)

A fair objection: if the goal is to avoid punctuation, drop the parentheses altogether and write
postfix, as Forth, PostScript and RPN calculators do.

```
cond1 cond2 A B if C if
```

That is the same tree with **zero** delimiters. Seven tokens, no parentheses, no commas. It really is
shorter, and for a moment it looks like it wins.

It wins on one condition: **every operator's arity must be fixed and known from its name alone.**
That is how the stack machine knows that `if` consumes exactly three things. Take that away and the
notation collapses, because nothing tells you where an argument list ends.

Our arities are not fixed:

```lisp
(fn (a b) …)                      ; `fn` takes a parameter list of any length
(let a 1 b 2 c 3 body)            ; `let` binds as many names as you like
(array 104 105 33)                ; a table literal, any length
(cond p1 e1 p2 e2 else e)         ; any number of clauses
(loop ((acc 0) (i 0)) c1 e1 … )   ; any number of variables and clauses
```

Write `(array 1 2 3)` in postfix and you must say *how many*:

```
1 2 3 array-3          ; the arity has moved into the name
1 2 3 ( array )        ; or a delimiter has come back
```

And that first repair is not hypothetical — it is in this repository. Go's `fmt.Println` is variadic,
the declaration format has no variadic, so [targets/go/fmt.oro](../../targets/go/fmt.oro) declares a
**family**, one member per arity:

```lisp
(sig Println  ((a any)) any                 (host stmt "fmt.Println(%s)" …))
(sig Println2 ((a any) (b any)) any         (host stmt "fmt.Println(%s, %s)" …))
(sig Println3 ((a any) (b any) (c any)) any (host stmt "fmt.Println(%s, %s, %s)" …))
```

The arity moved into the name. That is the price of giving up a delimiter *at a declaration*, paid
deliberately and in one place; paying it at every call site is what postfix would ask for.

So the objection is exactly right, and it is worth stating as the rule it is:

> **Postfix is parenthesis-free only for fixed arities. The moment application is variadic, you need
> a delimiter to say where the arguments stop — and a pair of delimiters is a parenthesis, whatever
> you spell it.**

There is a second cost, which matters more for a language meant to be read. In `cond1 cond2 A B if C
if` the tree is not visible; you recover it by simulating a stack in your head. In
`(if cond1 (if cond2 A B) C)` the tree *is* the indentation. We are a compiler whose whole thesis is
that reduction is visible — a notation that hides the tree is the wrong notation for us.

## 0.5 So why is the head inside?

§0.3's table put the commas in because that is how the familiar notation writes them. Take them out
and `if(cond1 if(cond2 A B) C)` — head outside, no commas — costs exactly what
`(if cond1 (if cond2 A B) C)` costs. Both are 4 parentheses and 0 separators. So why move the head
in?

Because of what happens when the head is **not a name**.

```lisp
((fn (n) (* n n)) 7)        ⟶   49
```

With the head inside, that is just a list whose first element happens to be another list. The reader
needs no new rule: it reads a list, and reads its elements, one of which is a list.

With the head outside, the same term is `(fn (n) (* n n))(7)` — an expression followed by a
parenthesis — which is a *juxtaposition* rule, a second way for two things to combine. Then
`f(x)(y)` needs its own case, and so does `(a ? b : c)(x)`, and so does every other expression you
might apply. You have added a grammar rule to save nothing.

Put the head inside and these are all one rule:

```lisp
(square 7)                      ; head is a name
((fn (n) (* n n)) 7)            ; head is a function
(trig.sin x)                    ; head is a qualified name
((if (< 1 2) f g) 7)            ; head is a conditional choosing a function — reduces to (f 7)
```

The head sits in **the same position as any argument**, so it can be any term an argument can be.
That is what "application is the only compound form" means operationally, and it is the property
§0.17 cashes in.

## 0.6 No operators, therefore no precedence

Because `+` is the head of an application like any other name, there are **no operators** in this
language. There is no precedence table, no associativity, no fixity, and nothing to look up.

What you lose is familiarity: `(+ a (* b c))` is four characters longer than `a + b * c` and reads
worse the first few hundred times. What you gain is an entire class of bug and an entire piece of
machinery.

**The bug.** In C, `&` binds *looser* than `==`, so this famous line:

```c
if (flags & MASK == 0)        /* parses as:  flags & (MASK == 0) */
```

does not test what it appears to test. C has fifteen precedence levels; nobody has them memorised,
which is why compilers now warn about this specific pair. The confusion cannot be written in this
language, because there is nothing for a precedence rule to get wrong — the two readings are two
different terms and they *look* different:

```lisp
(= (x64.and flags mask) 0)      ; (flags & mask) == 0
(x64.and flags (= mask 0))      ; flags & (mask == 0)   — what C actually parses
```

(Bitwise operations are deliberately not in the language; `x64.and` is a name the `windows` target
provides, which is §0.17's subject.)

**The machinery.** Infix parsing is its own subject: Pratt's *Top down operator precedence*
(POPL 1973) exists because the problem is not trivial. Go's specification needs five precedence
levels for binary operators. Haskell lets a *library* declare fixity with `infixl`/`infixr` at levels
0–9, and Prolog lets a program change the operator table at run time with `op/3` — which means that
in both, **you cannot parse an expression without knowing what was imported.** Our reader is
context-free, and a comment in `core/read.go` says so where it matters.

And it is not only infix. Three more hazards, each famous, none of which exists here:

| hazard | where | why it cannot happen here |
|---|---|---|
| dangling `else` | ALGOL 60 onward — `if a then if b then X else Y` | `(if a (if b X Y) Z)` has the arms in fixed positions |
| automatic semicolon insertion | JavaScript — `return` ⏎ `x` returns `undefined` | there are no statements and no inferred terminators |
| the "most vexing parse" | C++ (Sutter, *Exceptional C++*, 2000) | a declaration is not a form; `sig` is |

> **An ambiguity you cannot write down is not an ambiguity you have to specify.** Every one of the
> rows above is a paragraph in somebody's standard.

## 0.7 The whole grammar

Here it is, whole — the lexical rules from [spec/core-0.md §1](../spec/core-0.md) and the term shapes
that survive the reader's sugar:

```
comment    ::= ";" (any but newline)* (newline | end-of-input)
whitespace ::= space | tab | newline | return
gap        ::= (whitespace | comment)*
delimiter  ::= "(" | ")" | whitespace | ";"

integer    ::= "-"? digit+
float      ::= "-"? digit+ "." digit+ ( ("e"|"E") "-"? digit+ )?
string     ::= '"' ... '"'
name       ::= segment ( "." segment )*
segment    ::= idchar+

term       ::= name | integer | float | string | "true" | "false"
             | "(" "fn" "(" name* ")" term ")"
             | "(" term term* ")"                     ; application
```

**Four token classes** — integer, float, string, name — plus the two parentheses, plus a comment
that is not a token at all. **Seven term kinds** a program can write, of which exactly **two are
compound**. That is the syntax of the language.

Everything else you will write is one of two things.

**A top-level form**, which is not a term and is not reduced — it contributes to a scope:

```
form ::= "(" "def"    name term ")"                    ; a definition          (δ)
       | "(" "sig"    name "(" param* ")" name … ")"   ; a type claim, and a precondition
       | "(" "module" path ")"                          ; open a module scope
       | "(" "use"    path ( "as" name )? ")"           ; import
       | "(" "export" name+ ")"                         ; what leaves the program
       | term
```

**Or sugar**, which the reader rewrites into the two term shapes above before anything else looks at
it: `let`, `seq`, `cond`, `and`, `or`, `not`, `loop`, `again`, `match`, `build`, `build-map`,
`tuple`, `variant`, `case`. §0.2 showed `loop` coming apart into a `fn` and an `if`; every other
rewrite is written down in [spec/](../spec/).

`true` and `false` are literals, not names and not sugar — `bool` is data and `if` is its eliminator
([ADR 0017](../decisions/0017-booleans-are-in-the-language.md)):

```lisp
(if true (x) (y))      ⟶   (x)
```

Compare: Python 3's reference grammar runs to over a hundred rules and needs an indentation
tokeniser that emits INDENT and DEDENT tokens, a `tokenize` module to get it right, and a
specification for what a tab is worth.

Two consequences of the grammar above that surprise people:

**`()` is not a term.**

```lisp
()
;; line 1: empty list is not a term
```

But it *is* a legal parameter list — `(fn () b)` is the nullary function a program's entry point has
to be.

**A number can be the head of an application.** The grammar permits it, so the reader does:

```lisp
(1 2 3)         ⟶   (1 2 3)
```

Reduction has nothing to say about it; it is the **type checker**, at the emission boundary, that
rejects applying a number ([chapter 5](05-types.md)). The reader is deliberately not where meaning
is decided.

## 0.8 Names are generous, and `.` is not a name character

A name is one or more `idchar`s. Beyond the letters and digits of
[UAX #31](https://www.unicode.org/reports/tr31/), these are name characters:

```
- + * / < > = ! ? _ % & | ^ ~
```

So each of these is **a single name**, one token, nothing special about it:

```lisp
+            <            space?           big-of!
dot-product  sub-total    go/strings       count-primes
```

Those are not invented examples: `space?`, `big-of!`, `dot-product` and `count-primes` are names in
`examples/` and `lib/` today.

Three things follow, and they are why the grammar can be as small as §0.7.

**There is no operator class.** `+` is a name that a target file supplies. `go/strings` is a name —
the `/` is an ordinary name character, which is what makes a module path a single token
([ADR 0025](../decisions/0025-a-module-path-is-the-hosts.md)).

**Hyphens are letters.** `sub-total` is one name, not `sub` minus `total`, because `-` is an idchar
and the subtraction would have been written `(- sub total)`. This is the single most common
first-day surprise, and it is free: with no infix operators there is no ambiguity to resolve.

**Two characters are reserved**, and each has its own diagnostic rather than the generic one, which
is how you can tell a reservation from an oversight:

```lisp
(fn (x') x')
;; line 1: "x'" contains '\'', which is not an identifier character

(fn (a:b) a:b)
;; line 1: "a:b" contains ':', which is not an identifier character
```

`'` is held for symbols and `:` is held for whatever a later design wants
([spec/data.md §2.2](../spec/data.md)); §0.14 is what `'` is being held *for*. Keeping `'` out also
leaves the door open to mathematical primes — `x'` — which a reader would otherwise have to
disambiguate from a symbol.

**`.` is the one punctuation character inside a name**, and it is a *separator*, not an idchar:

```lisp
(a . b)
;; line 1: "." has an empty segment; `.` separates qualifiers and cannot begin, end, or double
```

`words.split-words` is a name of two segments. `.` qualifies a module member
([spec/modules.md §3](../spec/modules.md)), which is why a `fn` cannot bind one:

```lisp
(fn (a.b) (+ a.b 1))
;; line 1: a.b cannot be a parameter; a binder is a simple name,
;;         and `.` qualifies a module member
```

### Case, and three things about the bytes

**Case never assigns meaning.** `Xr`, `xr` and `XR` are three different names — identifiers are
case-*sensitive* — but no syntactic category and no visibility rule may be decided by case. Shen
makes capitals mean "variable"; Go makes them mean "exported". Both rules are unimplementable in
Arabic, Hebrew, Chinese, Japanese, Korean and Thai, which have no case at all
([core-0.md §1.1](../spec/core-0.md)).

Three checks run on the **bytes**, before anything is a token:

```lisp
(def café 1)    ; é as U+00E9           ⟶   fine
(def café 1)    ; é as e + U+0301       ⟶   source is not NFC-normalised, at or before byte 11;
                ;                           two identifiers can look identical and not be equal.
                ;                           Save the file as NFC.
```

```lisp
(def a 1) ; ‮mpharmless
;; byte 12: bidirectional control U+202E is not permitted
;;          (source must display as it parses)
```

Invalid UTF-8 is rejected too — not repaired, not replaced with U+FFFD. The second check is
[Trojan Source](https://trojansource.codes/) (CVE-2021-42574), and note **where** it fired: inside a
comment. §0.9 is about a comment being invisible to the compiler; these two checks are the exception,
and they are the exception for a reason — they are checks on the *text*, and their whole job is to
guarantee that the text you see is the text that parses.

## 0.9 Comments

`;` to the end of the line, or to the end of the file:

```lisp
; a line of commentary
(+ 1 2)   ; or trailing, after a form
```

That is the entire comment syntax. There is no `/* … */`, no `#`, no `//`, no `"""docstring"""`, and
no doc comment.

**A comment is not a token.** It belongs to the *gap* between tokens, together with whitespace, and
the reader erases it. So a comment may appear anywhere a space may — which is anywhere at all:

```lisp
(+ 1 ; inside a form
   2)                  ⟶   3
```

```lisp
(def one ( ; inside an empty parameter list
         ) 1)
(one)                  ⟶   1
```

```lisp
; a file of nothing but comments
; and nothing else
                       ⟶   no forms, and that is not an error
```

```lisp
(+ 1 2) ; at the end of the file, with no trailing newline
                       ⟶   3
```

And `;` is a **delimiter**, so it ends a name. There is no escape and no way to put a `;` in a name:

```lisp
foo;bar
;; at the top level: foo is not bound — it is not a parameter,
;;                   not a definition, and not a primitive on this target
```

Read that error again: it names `foo`, not `foo;bar`. The `;bar` was a comment.

Inside a string literal, `;` is an ordinary character — a string is scanned by its own rule, which
the gap never enters:

```lisp
"a;b"                  ⟶   "a;b"        ; three characters
```

### What a comment means

**Nothing.** It has no position, no attachment and no representation. Reading factors:

```
parse : Text ⟶ Form*        =        Text ⟶ Token* ⟶ Form*
```

and the first arrow erases the gap. So `parse` is **not injective**: the same program with and
without its comments reads to an *identical* term, and no later pass — reduction, the checker, a
backend — can recover a comment or be affected by one. That is what makes a comment free, and it is
a decision, not an accident
([ADR 0024](../decisions/0024-comments-are-erased.md), [spec/comments.md](../spec/comments.md)).

Two things follow that are worth your attention.

**Documentation is a term, not a comment.** There are no doc comments and no pragmas. A comment
attached to a *term* has no image under reduction anyway — δ copies a definition into every use, and
β deletes the branch it does not take, so there is no one place for the comment to land. If
documentation is ever to be read by a program it will arrive as a **form** the reader keeps, beside
`sig`, in a language where [declarations are theories](../spec/theories.md).

**The compiler cannot lose your comment, so a tool can.** Since nothing downstream will ever notice
a missing comment, the only thing that can drop one is a program that rewrites source — and one did,
twice, on the day the flat `let` landed. The rule:

> **A source rewriter preserves every gap, or refuses the edit and names the site.**

The stake, measured today across `examples/` and `lib/`: **2,158 comment-only lines out of 4,168**,
and 217 of `freq.oro`'s 425. Slightly more than half of this project's source in the language says
*why*, and the compiler is defined not to read it.

## 0.10 No block comment, and no datum comment

Neither is built. The first is merely unrequested; the second is **refused**, and the reason is
specific to this language.

**`#| … |#`** would need a nesting rule and a second lexer mode, and nothing has asked for it. You
comment a region by prefixing its lines, which every editor does.

```lisp
(+ 1 #| a block |# 2)
;; line 1: "#|" is not a valid identifier or number
```

**`#;datum`** — Scheme's expression comment, which discards *the next form* — cannot be added here
without making the lexer semantic. Our forms are discriminated by **arity and parity**:

- `def`'s shorthand is chosen by whether the second element is a *list of names*
  ([program-surface.md §8.3](../program-surface.md));
- `let`'s grammar is chosen by whether the element count after the head is *odd or even*
  ([spec/binding.md §2](../spec/binding.md));
- `cond`, `loop` and `match` require their clauses to come *in pairs*.

So consider what `#;` would do:

```lisp
(let x 1 y 2 body)          ; binds x and y
(let x 1 #;y #;2 body)      ; would bind only x
```

The second is not "the same program with a note on it". It is **a different program, chosen by a
comment.** Likewise `(def f #;(a) (+ a 1))` would turn a one-parameter function into a constant with
a free variable in it. A comment that can change which grammar a form takes is not a comment; it is
syntax with a misleading name.

Scheme can afford `#;` because none of its forms is parity-discriminated this way. We cannot, and
noticing that is the sort of thing this project would rather write down than discover later:

```lisp
#;1
;; line 1: "#" is not a valid identifier or number
```

`#` is not an identifier start, so both spellings are read errors today. (The reader uses `#k` for
the binder a tuple pattern desugars to — which is exactly why no source term can contain one.)

## 0.11 How to read it, and how to write it

The parentheses are for the machine. **The indentation is for you**, and in this notation
indentation and structure cannot disagree, because the structure is the nesting.

Three rules cover the house style, which you can check against any file in `examples/`:

**One. Two spaces, and the body under the head.**

```lisp
(def cross (c i n)
  (loop ((c c) (j (go.* i i)))
    (go.< j n)  (again (set c j true) (go.+ j i))
    else        c))
```

**Two. Closing parentheses never get their own line.** They stack at the end — `c))` above — and you
never count them, because the indentation already told you where you are. This is the convention
every Lisp settled on and every Lisp editor implements; if you are counting parentheses, your editor
is not configured.

**Three. Clauses line up in columns.** A guarded chain is a table, so write it as one. Above, the
guards (`(go.< j n)`, `else`) are in one column and the results in another, and the shape of the
`loop` is legible from two feet away.

**To read an unfamiliar form, read the first element and stop.** It tells you what the rest is:

```lisp
(sig count-primes ((n int)) int (where (and (<= 0 n) (< n 1048576))))
 ↑
 a signature: name, parameters, result, and a precondition
```

You do not need to know the grammar of `sig` to know you are looking at one, because the head is
always the first thing on the line.

**A complete small program**, to see it all together:

```lisp
(export sum-to)
(sig sum-to ((n (int 0 65536))) int)
(def sum-to (n)
  (loop ((acc 0) (i 0))
    (< i n)  (again (+ acc i) (+ i 1))
    else     acc))
```

In Python that is:

```python
def sum_to(n):
    acc = 0
    for i in range(n):
        acc += i
    return acc
```

Six lines against five, and two of the six — the `export` and the `sig` — have no Python equivalent
at all; the function itself is four lines against five. The one real difference is that the Oroboros
version **states the range of `n`**, and that is what lets `cmd/gen` report, on the Go target:

```
note: demo-sum-to: 2 of 2 integer operations bounded; 1 of 1 loop(s) proven terminating
```

Two additions proven to stay inside the target's word, and one loop proven to finish. That is what
the extra line bought, and it is the subject of [chapter 5](05-types.md).

## 0.12 Where this notation came from

The lineage is short and worth knowing, because the interesting part is that **nobody chose it.**

**Church, 1932–1936.** The λ-notation is prefix and parenthesised: `λx.M`, and application written by
juxtaposition and grouped with parentheses. The calculus this language is built on was written in
roughly this shape before there were computers to run it.

**McCarthy, 1960.** *Recursive Functions of Symbolic Expressions and Their Computation by Machine,
Part I* (CACM 3(4)) introduces **two** notations:

- **S-expressions** — "symbolic expressions" — parenthesised lists, intended as the *data* the
  language manipulates;
- **M-expressions** — "meta-expressions" — the notation programs were to be *written* in:
  `car[x]`, `cond[[p₁; e₁]; [p₂; e₂]]`. Brackets, semicolons, head outside.

The plan was that you would write M-expressions, and they would be translated into S-expressions for
the machine. **The plan was never finished.** `eval` was specified over S-expressions, which meant
that a program written directly as an S-expression could be run immediately — and programmers,
finding that convenient, simply stopped waiting for the M-expression translator. McCarthy records
this in *History of Lisp* (HOPL, 1978): the S-expression form was supposed to be temporary.

So the most-criticised syntax in computing is **a data format that got used as a programming
notation because the intended notation was late**, and then turned out to be good enough that sixty
years of attempts to replace it have not.

**Then, the attempts.** They are the honest evidence, and they cut both ways:

| attempt | what it did | what happened |
|---|---|---|
| Scheme (1975), Common Lisp (1984) | kept it | both still in use |
| **Dylan** (1992) | started as s-expressions, **replaced them with an infix syntax in 1995** | the infix version is the one that shipped |
| Clojure (2007) | kept parentheses, added `[]`, `{}`, `#{}` as distinct reader forms | the most-adopted new Lisp; the extra brackets are *visual landmarks*, and that is the point |
| SRFI 49 (2003), SRFI 110 (2013) | indentation-sensitive readers for Scheme ("I-expressions", "sweet-expressions") | specified, implemented, not adopted |

Dylan is the one to sit with. A team that knew Lisp extremely well, building a Lisp, looked at the
notation and swapped it out. The parenthesis complaint is not only made by people who have not tried.

## 0.13 Where else this notation is used

It is easy to think of s-expressions as "the Lisp syntax". They are better described as **the
default notation for anything tree-shaped that a program has to both read and write**, and most of
the places you will meet them are not Lisps:

| | |
|---|---|
| **WebAssembly text format** (`.wat`) | the official text form of Wasm is folded s-expressions |
| **SMT-LIB 2** | the command language of every SMT solver — Z3, CVC5, Yices |
| **ACL2** | a theorem prover whose language is Common Lisp |
| **KiCad** | schematic and PCB files |
| **Sexplib** (Jane Street, OCaml) | the house serialisation format for a large production OCaml codebase |
| **Canonical S-expressions** (Rivest, 1997) | the SPKI/SDSI certificate format, with a *canonical* byte encoding for signing |
| **DSSSL** (ISO/IEC 10179:1996) | the SGML stylesheet language, Scheme-based — XSLT's ancestor |
| **SXML** (Kiselyov) | XML as s-expressions, so XML transformation is ordinary list processing |
| **GNU Guix** | Scheme s-expressions as a package and whole-operating-system configuration language |
| **Emacs Lisp** | the configuration language of the editor many of these were written in |

Two of those rows are directly relevant to us. **SMT-LIB** is what a prover's input looks like, and
this is a compiler with a prover in it — our obligations and an SMT query want the same notation.
And **Wasm** is a 2017 design, by a committee that could have picked anything, choosing parenthesised
prefix for a format that tools emit and humans occasionally read.

The property that makes it the default for this job is that **the reader and the writer are the same
twenty lines of code**. There is no asymmetry between parsing and printing, which is why every one of
those formats is round-trippable without a library. Our own `cmd/check` leans on exactly this: it
reprints every emitted program from the IR's text and compares byte for byte
([ADR 0038](../decisions/0038-the-irs-text-is-read-not-written.md)).

## 0.14 An s-expression language that is not a Lisp

If you know Lisp, the list of what we *do not* have matters more than the list of what we do, because
the absences are deliberate:

| Lisp has | we do not | why |
|---|---|---|
| `'x`, `` `x ``, `,x` — quote and quasiquote over any datum | `'x` is **specified**, restricted to one identifier, and not built | the rest of this section |
| macros, `defmacro` | none | the rest of this section |
| reader macros (`#(`, `#'`, `#\`) | none; the reader is fixed and context-free | a program may not change how a program is read |
| dotted pairs, improper lists, `cons` cells | a form is a flat list; there is no pair | nothing in the calculus is built from pairs |
| `[]`, `{}`, `#{}` (Clojure) | parentheses only | one delimiter pair, so §0.7's grammar stays three lines |
| symbols as run-time values | a symbol exists only at compile time, by design | [spec/data.md §2.4](../spec/data.md) |
| `nil` as false, `()` as a value | `()` is not a term; `bool` is data with `if` as its eliminator | [ADR 0017](../decisions/0017-booleans-are-in-the-language.md) |

### `'` is taken, and what it is taken for

"Quote" is four different things here, and the table's row is only about the fourth. Worth
separating, because three of them exist:

| | |
|---|---|
| **quotation marks** | `"a;b"` is a string literal, one of §0.7's four token classes, scanned by its own rule that the gap never enters (§0.9). We have these |
| **`Quote` the host function** | `Quote`, `QuoteToASCII`, `QuoteToGraphic`, `QuoteRune` and its two variants, six `AppendQuote…` forms, `Unquote`, `QuotedPrefix`, `UnquoteChar` — all declared on the Go target ([targets/go/strconv.oro](../../targets/go/strconv.oro)). `(sc.Quote s)` emits `strconv.Quote(v0)`. An ordinary function on bytes |
| **a quoted identifier, `'x`** | **Specified** ([spec/data.md §2](../spec/data.md)) as a *symbol*, and not built |
| **quote over any datum** | `'(1 2)`, `` `(a ,b) `` — what macros need. Specified as an **error**, deliberately |

The third is the interesting one, and it is narrower than Lisp's on purpose:

```
symbol ::= ' identifier
```

A **symbol** is a name used as data rather than as a reference: `'x` means *the name x*, where `x`
means *the value bound to x*. That is the use–mention distinction, and it is what a record label is —
`(p 'x)` asks for the component named `x`, not for one whose index is the value of a variable `x`.

Four things about it are worth having, and each is a decision rather than a convenience:

**It reads as sugar, so the term language does not grow.** `'x` and `(quote x)` read to the same
thing: an application of an injected name `quote` to **the string literal of the spelling**. So the
spelling is never resolved as a variable — it is a string inside the reader's output, not a name —
and §0.7's seven term kinds stay seven. `quote` would join `if`, `let` and `loop` as a name the
compiler injects and a target may not declare (§0.17's rule, running the other way).

**It is not a string value**, even though its spelling is "identifier characters, no whitespace". A
string is an element of Σ\* that exists at run time; **a symbol exists only at compile time**, may
appear only as a record label, and anywhere else in the residual is refused with the same diagnostic
shape as a closure that survives — callbacks.md's *free at the static level, refused at the dynamic
one*, applied to the smallest possible value. A later design may give a surviving symbol a
representation; none is given, because no program has needed one.

**`'` and not `:`, and the reason is about readers rather than parsers.** Neither is an identifier
character, so they cost the same. But `:x` is a *keyword* in Clojure, Elixir and Ruby, where it is a
run-time value — interned, comparable, storable — and a label here never exists at run time, so that
reading would be the wrong one. `'x` is `(quote x)` in Lisp and Scheme, which is the use–mention
reading we want, in the syntax family we already belong to. `:` stays free, and ascription is already
`(the T e)`.

**Quoting a general datum is refused, and that is the line.** `'` is to be followed directly by an
identifier, and the spec makes `'(…)`, `'3` and `'"s"` errors that say so. The reason is the thesis:

> **Quoting a general datum would be staging, and staging in this language is reduction, not
> quotation** ([the-atom.md](../the-atom.md)).

So the thing we decline is precisely the thing a macro system is built on, and we decline it for a
stated reason rather than for want of a character.

> **Neither of the last two senses is implemented, and the reservation is.** `'` is kept out of the
> identifier charset with its own diagnostic (§0.8), but nothing reads a symbol yet and `quote` is
> not injected:
>
> ```lisp
> 'x          ;; line 1: "'x" is not a valid identifier or number
> (quote x)   ;; at the top level: quote is not bound — it is not a parameter,
>             ;;                   not a definition, and not a primitive on this target
> ```
>
> Today those four refusals are the lexer's generic one rather than the specific messages §2.2
> describes, which is what "not built" looks like from outside. Symbols sit in
> [CLAUDE.md](../../CLAUDE.md)'s *"Not built: records, symbols, `with`"* — specified because records
> need labels, unbuilt because records are. Read data.md §2 for what `'x` will mean, not for what it
> does.

### Homoiconicity, which we decline

The usual argument for s-expressions is *homoiconicity* — the word is generally traced to Kay's 1969
thesis, describing TRAC — meaning that code and data share a representation, so a program can
construct a program. That is what quote-over-any-datum and macros are for, and it is the
single most-cited advantage of the notation.

**We do not claim it, and we do not use it.** The one quotation we specify reaches exactly one
identifier and dies at staging; nothing can hold a program; there are no macros, so nothing extends
the syntax. We use s-expressions for the grammar alone.

The reason we can get away with that is a finding, not a preference
([q5](../spec/q5-do-we-need-rules.md)). The question was whether this language needs a `rule`
construct — pattern-matching rewrite rules, with metavariables, which the early drafts reserved the
`?x` sigil for. The answer:

> Every rewrite rule whose left-hand side is **a name applied to arguments** is just a `def`.

And since application is the only compound form (§0.2), that is *every* lowering in the project's six
derivations — the whole capability graph, in both directions, with no pattern matching anywhere. δ
over `def` turned out to cover layer lowering *and* fusion, which was not obvious. So the macro
system was never built, and the `?x` token class was retired without ever being implemented.

> **A macro system is how you get a rewrite rule in a language where `(f x)` is only one of eight
> kinds of syntax.** Where it is the only kind, a definition already is one.

## 0.15 The pros and the cons, honestly

Both columns are real. Taking the cons seriously first, because this chapter is addressed to people
for whom they are the whole of their first impression.

### Against

**It is unfamiliar, and unfamiliarity is a real cost.** Not a small one, and not one that argument
dissolves. The backronym — *Lots of Irritating Superfluous Parentheses* — has stuck for fifty years
because it describes something people genuinely feel.

**The head is not visually marked.** This is the substantial technical complaint. In `f(x, y)` the
parenthesis distinguishes the function from its arguments; in `(f x y)` nothing does, and `f`, `x`
and `y` are three tokens in a row. The notation therefore leans on **indentation** to carry what
other languages carry in punctuation — which is why Lisp has strong indentation conventions and why
every Lisp editor auto-indents. Clojure's `[]` and `{}` are a direct response to this, and they are a
genuine improvement we decline for the sake of §0.7's four-line grammar.

**Closing-parenthesis runs.** `)))` is harder to check by eye than `}` on its own line, and the only
real answer is "your editor matches them for you" — which is true, and is also an admission that the
notation needs tooling that `if/else` does not.

**Infix arithmetic is worse.** `(+ (* a b) (* c d))` against `a*b + c*d`. For code that is mostly
arithmetic, the familiar notation is genuinely more readable, and the project has no measurement
claiming otherwise.

**People who knew have walked away from it.** §0.12's Dylan row, and the two indentation-sensitive
SRFIs, are evidence that should not be explained away.

### For

**The term grammar is three lines, and the lexical grammar is nine (§0.7).** Not a simplification —
the whole of it. The downstream effects
are large and specific: no precedence table, no fixity, no ambiguity to resolve, context-free
reading, and a reader small enough that a *target file*, a `provides` fragment and a program are read
by literally the same code path. That last one is not a convenience; it is why a target is data
rather than a case in a Go file.

**Whole classes of bug do not exist** (§0.6): the `&`/`==` precedence trap, dangling `else`, the most
vexing parse, automatic semicolon insertion. Each is a paragraph in somebody's standard.

**The parenthesis count is not actually higher** (§0.3). It is `2n` either way, and the familiar
notation adds commas on top.

**Structure and indentation cannot disagree.** In Python they are the same thing *by fiat*, enforced
by a tokeniser with INDENT and DEDENT tokens and a specification for the width of a tab. Here they
agree because the parentheses are the structure and the indentation is derived from it — a
mis-indented file is wrong-looking but never wrong-meaning, and an auto-indenter is a total function.

**Structural editing is cheap.** `paredit` has existed since the 1990s and moves, wraps, splices and
transposes *subtrees* rather than characters. The same facility for a C-family language needs a full
parser and a decade of tree-sitter.

**Reading and writing are symmetric.** §0.13's last paragraph: this is why `cmd/check` can reprint
every program from its IR text and compare byte for byte.

### And the thing neither column gets to claim

The question *"is the familiar syntax easier?"* has been measured, and the answer is not the one
either side expects. Stefik and Siebert, *An Empirical Investigation into Programming Language
Syntax* (ACM TOCE 13(4), 2013), had novices rate token choices across several languages and included
a control: **Randomo**, a language whose keywords and operators were chosen partly at random. The
C-family syntaxes tested — C, C++, Java, Perl — rated **no more intuitive to novices than Randomo**.
Python, Ruby and Quorum rated significantly higher.

Be careful with what that does and does not say. It does **not** say s-expressions are good; they
were not in the study. What it says is narrower and more useful:

> **"Everyone knows C-style syntax, therefore C-style syntax is natural" confuses familiarity with
> intuitiveness.** The study separates them, and finds that for the C family the second does not
> follow from the first.

Which leaves the honest position for this project, and it is the position
[CLAUDE.md](../../CLAUDE.md) demands:

> We have **no measurement of our own** on whether this notation is easier or harder to read than an
> infix one, for anybody. The arguments in §0.15 are arguments, and *every design claim in this
> project that was not measured has been wrong about half the time.* What is measured is §0.3's
> delimiter count, §0.7's grammar size, and the bug classes in §0.6 — and none of those is a claim
> about readability.

## 0.16 The parentheses do not survive

One last thing, and it is the thing that most often goes unsaid. The notation is for the *reader*.
None of it reaches the program you ship.

```lisp
(export square)
(sig square ((n (int 0 1000))) int)
(def square (n) (* n n))
```

One source file, four targets, real output:

```go
func SqSquare(v0 int) int {       // Go
	v9 := (v0 * v0)
	return v9
}
```

```javascript
export function sqSquare(v0) {    // JavaScript
	return (v0 * v0);
}
```

```java
public static long sqSquare(int v0) {    // Java
	final int v9 = (v0 * v0);
	return v9;
}
```

```asm
sq_square proc                    ; Windows, MASM
        mov rbx, rcx
        imul rbx, rbx
        mov rax, rbx
        ...
```

`(* n n)` became infix `*` three times and `imul` once. The parentheses were a notation for a tree;
the tree was compiled; what is left is each host's own idiom
([ADR 0001](../decisions/0001-parasite-model.md)).

## 0.17 What this notation buys *this* language

Everything above is true of any s-expression language. This section is the part that is only true of
this one, and it is the reason the question "should we use a friendlier syntax?" has a real answer
rather than a stylistic one.

The thesis of the project is that **the normal form is a parameter**
([the-atom.md](../the-atom.md)): a target partitions names into primitive and defined, reduction runs
until only primitives remain, and which names are primitive is a *data file*
([ADR 0002](../decisions/0002-capability-graph.md)). Compare the arithmetic in two target files:

```lisp
(sig + (num num) num pure (host expr "%s + %s"))     ; targets/tutorial.oro
```

```lisp
(sig +  ((a int) (b int)) int pure
        (host expr "%s + %s" (checked add-exact)))   ; targets/go/builtin.oro, inside module `go`
```

Addition is a `sig` in a file. Its name, its arity, its purity, its types, its host template and
whether it has a checked variant are all **data**, and a different target may supply a different set
or none at all. `math/trig.sin` is a module the `tutorial` target provides natively, with no file on
the search path, and that is not an error.

**None of that is possible if `+` is syntax.** An infix `+` with a precedence lives in the grammar.
A grammar is not a parameter — it is compiled into the reader, it cannot be supplied per target, and
a target cannot decline it. The moment you add an operator table you have asserted that addition is
part of the language rather than part of a target, and ADR 0002 is no longer implementable.

So the chain is short and it is load-bearing:

```
application is the only compound form              (§0.2)
    ⟹  the head sits in an argument position        (§0.5)
    ⟹  every primitive is just a name in a file     (§0.17)
    ⟹  the set of primitives is a per-target parameter
    ⟹  the normal form is a parameter               ← the thesis
```

And the same property is why there are no macros (§0.14): where `(f x)` is the only compound shape,
a rewrite rule and a `def` are the same object, so δ does the work a macro system would.

> The syntax is not a style decision that happens to resemble Lisp. It is the smallest notation in
> which this language's central claim can be *stated*.

---

## What to remember

- **`f(a, b)` is `(f a b)`.** The head moved inside the parenthesis and the commas went away. Read
  it "f of a and b".
- **Application is the only compound form.** A call, an operator, an index, an `if` and a `loop` are
  all one list.
- **The parenthesis count is the same** as in the notation you know — `2n` for both — and the
  familiar one adds a comma per extra argument. Parentheses are the price of nesting.
- **Postfix escapes them only for fixed arities.** Variadic application needs a delimiter, and a
  delimiter pair is a parenthesis however it is spelled.
- **There are no operators**, so no precedence, no fixity, no dangling `else`, and no ambiguity to
  resolve. The reader is context-free.
- **The term grammar is three lines, the lexical grammar nine.** Everything else — `let`, `cond`,
  `loop`, `match` — is sugar the reader removes.
- **Names are generous**: `-`, `+`, `?`, `!`, `/` are ordinary letters, so `dot-product` and
  `go/strings` are single names. `.` is the one separator, and it qualifies a module member.
- **A comment is `;` to end of line, it is gap rather than token, and it means nothing.** `parse` is
  not injective, which is what makes it free — and which is why a tool that rewrites source must
  preserve every gap or refuse the edit.
- **No `#;`**, because our forms are discriminated by arity and parity, and a comment that changes
  which grammar a form takes is not a comment.
- **We use s-expressions for the grammar, not for homoiconicity.** `'` is reserved and specified for
  **symbols** — `'x`, one identifier, a compile-time record label, not built — and quote over a
  general datum is refused, because that would be staging and staging here is reduction. No macros
  either: δ over `def` already is a rewrite rule.
- **The real cost is that the head is not visually marked.** Indentation carries it. Configure your
  editor and stop counting parentheses.
- **The notation does not survive compilation.** `(* n n)` ships as `v0 * v0` and as `imul`.

Next chapter: [`fn`](01-fn.md), and the one of these seven term kinds that does the work.
