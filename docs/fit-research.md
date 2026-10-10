# Which syntax fits the semantics

Research, 2026-10-10, on hamza's request: *"there is another way to look at it. which syntax best maps
to the semantics, we decided a lot in settypes-research.md, and I believe -tell me if I am wrong- that
some syntax will map better to the semantics of the language (both the type and the computation)
better than others. research and discuss and if necessary research."*

It follows [settypes-research.md](settypes-research.md) (decided, ADR 0049),
[notation-research.md](notation-research.md) and [syntax-research.md](syntax-research.md). Those two
asked which notation reads and writes best. This one asks which notation is the most faithful image of
the meaning. **Decisions 1–6 are decided** (hamza, 2026-10-10: "specify", then "decided. agreed to
recommendation"), by [ADR 0050](decisions/0050-the-abstract-syntax-fits-the-semantics.md), and specified in
[spec/abstract.md](spec/abstract.md). Decision 7, the surface, stays with syntax-research.

## 0. The answer, in brief

1. **hamza is right, and the claim can be made exact.** A syntax and a semantics are two algebras over
   one signature, and a meaning is a homomorphism between them (Montague; Goguen, Thatcher, Wagner and
   Wright). A syntax **fits** when its abstract syntax is the initial algebra of the semantic signature:
   one form per operation, one shape per kind of binder, every semantic distinction marked, and no
   distinction that means nothing. Deviations from that have names and can be counted (§1).
2. **But most of the fit is in the abstract syntax, not the concrete one.** Of the fourteen places
   where the semantics chooses (§3), ten are settled the same way in prefix, infix or ML notation. Only
   four discriminate the concrete notation, and they point in different directions:
   - **types are static values** (ADR 0049 §5), so types and terms need one grammar. That rules out
     F#'s separate type grammar (`<'E when 'E :> error>`) and Shen's `{A --> B}`;
   - **the logic's own forms** (chained comparisons, a binder with its set, set-builder notation)
     read as mathematics, and nothing else writes them as directly;
   - **β-substitution is textual in prefix notation and needs parentheses in infix notation.** I
     expected this to favor S-expressions. Measured on the corpus, it does not: 6 of 1,590 parameter
     occurrences at calls would need added parentheses, all of them a λ substituted at a head (§3.13);
   - **tables are values and λs are not** (V contains no functions). TLA+'s `t[i]` against `f(x)`
     marks exactly that line, and today's uniform `(t i)` hides it. Measured: the corpus never uses
     the uniformity (§3.8).
3. **Every notation written so far, today's included, misfits in the same three places**, and these
   are worth fixing whichever notation wins:
   - **usage is written as membership.** `(buffer V)`, `(cell T)`, `Buffer(Byte)` and `Seq(Cell(…))`
     are written as sets, but linearity and pass-by-value-result are modes of a binder, not sets of
     values (settypes §2.5). Quantitative type theory and Ada put them on the binder (§3.2);
   - **a value is punned with its singleton**, and a complement is taken relative to a sort that is not
     written down. settypes §7.2's `(def method (or "GET" "POST"))` and `(not 0)` mean different things
     by position (§3.5);
   - **a claim is split across forms**: `sig`, `def`, `where`, `ensures` and `requires`. One
     membership, f ∈ Π(x ∈ S). T(x), written as a Z schema (declarations, then a predicate), carries
     all of them (§3.4).
4. **Today's notation has eight binder shapes for three kinds of binder, and five forms that read
   structure from parity.** ADR 0024 had to refuse the datum comment `#;` because of the parity. That
   is the symptom of a misfit (§3.3, §3.6).
5. **So the semantics fixes an abstract syntax, "abstract Oroboros", almost completely, and leaves the
   concrete notation mostly to the readers.** That is Landin's separation of abstract ISWIM from
   physical ISWIM (1966). The recommendation is to write the abstract syntax as a spec first, fix the
   three misfits in it, and then let syntax-research's decision (one source, two renderings) choose the
   physical notation, a choice the semantics does not make for us (§6, §7).

## 1. What "fits" means

### 1.1 Compositionality

Frege's principle: the meaning of a compound is a function of the meanings of its parts. Montague made
it algebra ("Universal grammar", *Theoria* 36, 1970): the syntax is an algebra, the semantics is an
algebra of the same signature, and meaning is a homomorphism from the first to the second. Goguen,
Thatcher, Wagner and Wright made it computer science ("Initial algebra semantics and continuous
algebras", *J. ACM* 24(1), 1977): **the abstract syntax of a signature is its initial algebra, and a
compositional semantics is the unique homomorphism out of it.** Scott–Strachey denotational
semantics, one equation per syntactic form, is that homomorphism written out.

Two older distinctions complete the picture:
- **abstract against concrete syntax.** McCarthy introduced abstract syntax ("Towards a mathematical
  science of computation", IFIP 1962). Landin separated *abstract ISWIM*, what the semantics sees,
  from *physical ISWIM*, what people type, and called the second a separate question ("The next 700
  programming languages", *CACM* 9(3), 1966);
- **binders in abstract syntax.** A binder is part of the signature, not a convention. That is
  binding signatures (Fiore, Plotkin and Turi, "Abstract syntax and variable binding", LICS 1999) and
  abstract binding trees (Harper, *Practical Foundations for Programming Languages*, 2012).

### 1.2 Five ways to misfit

Take the semantic signature Σ (§2) as the standard. A syntax fits when its abstract syntax *is* Σ's
initial algebra. It can depart from that in five countable ways:

| | misfit | what it costs the semantics |
|---|---|---|
| **F1** | **synonym**: two forms, one operation | the semantic equations repeat, and the two can drift apart |
| **F2** | **homonym**: one form, two operations, told apart by position or context | an equation needs context, so meaning is no longer local |
| **F3** | **hidden binder**: a binder written by position (parity), or binder kinds merged or split arbitrarily | scope is not visible in the tree; tools that rewrite text can move a name across a scope |
| **F4** | **missing distinction**: a semantic difference with no syntactic mark; or **F4′**, a syntactic difference with no semantic one | a reader cannot see the difference, or looks for one that is not there |
| **F5** | **checked, not written**: an invariant the grammar could state, enforced by a later check | an ill-formed program is writable and refused later, by a walker that must not miss a case |

F1–F5 are properties of the **abstract** syntax. The concrete choices (prefix or infix, parentheses or
indentation, words or symbols) affect fit in one place only: whether the semantics' own operations,
above all substitution, are **textual** operations on the notation (§3.13, §3.14). The rest of what
separates S-expressions from mathematics is legibility, syntax-research's subject, not fit.

## 2. The semantic signature

From settypes-research §1–2, ADR 0049 and the specs. This is what a syntax must be an image of.

**The computation** (it reduces and prints):

| operation | meaning |
|---|---|
| λ, application | the untyped λ-calculus (Curry-style, syntax-research §1); every λ is static and is erased by staging |
| a host call | δ: a primitive survives as a call |
| `let` | a binding, the residual's `(let e (fn (x) b))` |
| tuple, projection | the product, with η (ADR 0031) |
| constructor, `case` | the tagged sum and its eliminator (ADR 0042) |
| `if` | 𝔹's eliminator, 𝔹 = 1 + 1 |
| `loop`, `again` | Dijkstra's guarded commands with an exit value (ADR 0015), a least fixed point |
| `local`, `set` | state threaded by the state-passing translation (ADR 0047, PlusCal's) |
| table by graph, by rule; indexing | a finite function on [0, n) and its application |
| map read, insert, keys | a finite partial function |
| `try`, `expect`, `ignore` | the exception monad's bind, give-up, and !_A : A → 1 (ADR 0040, 0043) |

**The logic** (it never prints):

| operation | meaning |
|---|---|
| sorts, singletons, ∪ ∩ ∖, comprehension, ×, tagged +, Seq, maps | the Boolean algebra of sets with constructors (settypes §2.1) |
| membership, =, <, ∧ ∨ ¬ ⇒, ∀ ∃ | first-order formulas over V |
| a parameter's set, a result's set, `where`, `ensures` | membership claims; dependence is Π(x ∈ S). T(x) |
| `assert`, `invariant`, `lemma` | claims at a point, at a loop head, and as a proven sentence (settypes §6.3) |
| a recursive specification function | defined in the logic only (settypes §6.3) |
| a static λ over sets | polymorphism, instantiated by β (ADR 0049 §5) |

**The modalities** (neither computation nor sets, settypes §2.5):

| modality | meaning |
|---|---|
| linear | a buffer is used exactly once along each path (ADR 0018, 0020) |
| relevant | a value is used at least once on every path (ADR 0043) |
| pure | an effect bit (ADR 0010) |
| value-result | a host writes a cell through a reference to a fresh copy (ADR 0048): copy in, copy out |

**The binders.** Every binder in the semantics is one of three kinds:

| kind | means | where |
|---|---|---|
| **definitional**, x = e | x denotes e's value | `let`, `def`, a constant, a residual let |
| **state**, x := e | x is a state variable, starting at e and changing along a path | a loop variable, a cell, a buffer; the translation makes cells loop variables, so these are one kind |
| **ranging**, x ∈ S | x ranges over S | a parameter, ∀ and ∃, comprehension, a `case` arm (x ranges over a piece of the partition), a static parameter over sets (T ∈ 𝒫(S)) |

A pattern (a tuple, a constructor) generalises the name in any of the three.

## 3. Fourteen places where the semantics chooses

Each item gives the semantics, the form that fits it, which notations have that form, and, where it
can be measured, the measurement.

### 3.1 Types are static values: one grammar for types and terms

**Semantics.** ADR 0049 §5: `or`, `and`, `tuple`, `array` are static functions on sets, a named type is
a `def`, and polymorphism is a static λ instantiated by β.

**Fit.** A type expression must be a term: the same application, the same names, the same definitions.
A separate type grammar is an F4′ (it distinguishes what the semantics identifies) and makes
instantiation a different operation from application.

**Notations.** The mathematical one (M) fits: TLA+'s sets are ordinary expressions (`Seq(S)`,
`S \cup T`). The S-expression flavors (P, T, C) fit by construction. **F# does not**: `'T list`,
`Result<'T, 'E>` and `<'E when 'E :> error>` are a second grammar, which is System F's stratification
(Church-style), not ours. F\* fits better, since its types are terms. **Shen's flavor does not**:
`{A --> B}` is a separate type language. Zig, whose types are compile-time values, is the nearest
working language to fit here.

### 3.2 Usage is not membership: modes on binders

**Semantics.** settypes §2.5: linearity and relevance are substructural judgements (Girard 1987), and
purity is an effect bit, none of them a set. ADR 0048's cell is **call by value-result**: the host
receives a reference to a fresh copy, and the copy's final value is read back. That is Algol W's
value-result (Wirth and Hoare, "A contribution to the development of ALGOL", *CACM* 9(6), 1966) and
Ada's `in out` mode (ANSI/MIL-STD-1815A, 1983). An `(out τ)` is Ada's `out`: no value in, a value out,
and outparams-research's "an out-parameter is a result".

**The misfit, in every notation so far.** Each one writes usage as a set constructor:

| notation | as written |
|---|---|
| today's (`targets/go/encoding/hex.oro`, `fmt.oro`) | `(dst (buffer (int 0 255)))`, `(a (array (cell go.Value)))` |
| M (notation-research §4.8) | `p ∈ Buffer(Byte)`, `a ∈ Seq(Cell(go.Value))` |
| F, P, C, S (syntax-research example 6) | `Buffer<Byte>`, `(Buffer Byte)`, `(Seq (Cell go.Value))` |

`(buffer V)` and `(array V)` have the same values. A frozen buffer *is* a table, and what differs is
how the binder may be used. Writing it as a set says that some values are buffers, which is false, and
it suggests that `(buffer V) ∪ (array V)` or `(array (buffer V))` mean something. The second is
refused today by a special rule ("a buffer may not be an element type", ADR 0020), which is an F5
symptom: the form permits what the semantics does not have.

**Fit.** The mode goes on the binder, where usage happens:
- **quantitative type theory** puts a usage quantity on each binder, `(1 x : A) → B` (McBride, "I got
  plenty o' nuttin'", 2016; Atkey, "Syntax and semantics of quantitative type theory", LICS 2018;
  Idris 2, Brady 2021);
- **Linear Haskell** puts it on the arrow, `a %1 -> b` (Bernardy et al., POPL 2018);
- **Ada** puts the passing mode on the parameter, `X : in out T`.

ADR 0020 already says "uniqueness on parameters", so the binder is the place the semantics names:

```
today          (sig Encode ((dst (buffer (int 0 255))) (src (array (int 0 255)))) …)
fitted, prefix (sig Encode ((dst (array byte) linear) (src (array byte))) …)
fitted, M      Encode(dst ∈ Seq(Byte) LINEAR, src ∈ Seq(Byte)) ∈ Nat

today          (sig Sscan ((str bytestring) (a (array (cell go.Value)))) … variadic …)
fitted, prefix (sig Sscan ((str bytestring) (a go.Value inout variadic)) …)
fitted, M      Sscan(s ∈ go.bytestring, a ∈ go.Value INOUT VARIADIC) ∈ Nat × Result(Unit, error)
```

In the variadic case, the mode applies to each trailing argument: ADR 0046 makes the trailing
arguments the list, so `inout` on the list parameter is `inout` on each element. `relevant` stays on
the variant's declaration, where it means "every binder ranging over this set carries the relevant
mode": a default mode from a set, which is how ADR 0043 already works. `pure` stays a marker on a
declaration. **Nothing changes in the semantics.** The checker reads the same judgement from a
different place.

### 3.3 Three binder kinds, three shapes

**Semantics.** §2: definitional, state, ranging.

**Today's notation has eight shapes:**

| shape | forms |
|---|---|
| flat name–value alternation | `let`, `local`, `build-map` |
| a list of names | `fn`, `def` |
| a list of (name value) pairs | `loop` |
| a list of (name set) pairs | `sig`, `where` |
| a constructor pattern `(ctor x…)` | `case`, `try`, `expect` |
| a tuple pattern | `let`'s left side |
| match patterns with `_` and `when` | `match` |
| a head binding | `variant`, `type` |

That is an F3 in both directions. `local` and `let` share a shape but bind different kinds (state and
definitional). `local` and `loop` bind the same kind (the translation makes one into the other) but
have different shapes.

**Fit.** One shape per kind, with patterns in place of names:
- M: `x == e`, `x := e`, `x ∈ S`. These are three spellings for three kinds, and TLA+ and PlusCal
  already use exactly these three.
- In prefix notation: `(x e)` in a definitional block, `(x := e)` or a `state` block, `(x S)` or
  `(x ∈ S)` in a ranging position.
- F#: `let x = e`, `let mutable x = e`, `(x : S)`. F# also fits here.

### 3.4 A claim is a membership: one definition form

**Semantics.** settypes §1.3 and §2.1: a parameter's type, a `where`, an `ensures` and a `sig`'s result
are all claims, and dependence is Π. A definition with its claims states one membership:

```
f ∈ Π((x, y) ∈ {(x, y) ∈ S × T : φ(x, y)}). {r ∈ U : ψ(x, y, r)}
```

- the parameters' sets are the declarations;
- `where`/`requires` is the comprehension over the parameter tuple;
- `ensures` is the comprehension in the result.

**The misfit.** Today, five forms carry it: `sig`, `def`, `where`, `ensures` and `requires`. `sig` and
`def` name the same object twice (F1), so the checker checks the claim "in two directions"
(types.md). syntax-research's P writes Typed Racket's `(: f T)` beside `define`, F writes F\*'s `val`
beside `let`, and S writes Shen's `{A --> B}` beside `define`. All are the same split.

**Fit.** One form, declarations then a predicate: a **Z schema** (Spivey, *The Z Notation*, 1992),
`[x : S; y : T | φ]`, the oldest precedent for exactly this shape. M's header
`f(x ∈ S, y ∈ T) ∈ U WHERE φ ENSURE ψ == e` and Plumatic Schema's `(defn f :- U [x :- S] …)` (C) are
close. A host declaration is the same form whose body is in the host.

```
today          (sig div ((a int) (b (int 1 +inf))) int)  (def div (a b) (/ a b))   ; ℤ ∖ {0} is not expressible today
fitted, prefix (def div ((a int) (b (minus int #{0}))) int  (/ a b))
fitted, M      div(a ∈ Int, b ∈ Int \ {0}) ∈ Int == a ÷ b
```

def.md §4's rule that "a parameter list is a list of NAMES" would give way to "a parameter list is a
list of ranging binders, a name with an optional set". This is the change ADR 0049 already implies.

### 3.5 A value is not its singleton, and a complement is relative

**Semantics.** 3 and {3} are different objects: 3 ∈ ℤ, and {3} ⊆ ℤ. Since types are static values
(§3.1), both exist at the static level, so a notation must tell them apart.

**The misfit, in settypes §7.2** (decided in principle, spelled provisionally):
- *"a literal in a type position is its singleton"*: `(def method (or "GET" "POST"))`. Read as a term,
  this is `or` applied to two strings. Read as a type, it is a union of singletons. Which one is meant
  depends on where `method` is later used: F2;
- *"`(not 0)`, ℤ ∖ {0}, within int where it is used"*: the complement is relative to a sort the form
  does not name, and V ∖ {0} contains every string. Also F2.

The pun can be made consistent: identify a set with its characteristic predicate, so `or` on
predicates is pointwise, and coerce a literal to its singleton by position. Typed Racket and
TypeScript do this. But the coercion *is* the context-dependence F2 names, and the two readings
diverge exactly where a static value is both a term and a set.

**Fit.** An explicit set former and an explicit difference:
- M: `{"GET", "POST"}`, `Int \ {0}`;
- C: `#{"GET" "POST"}`, the one Clojure sprinkle that the semantics justifies;
- P/T: `(set "GET" "POST")` and `(minus int (set 0))`.

`or`, `and` and `not` remain the connectives on formulas, and ∪ ∩ ∖ are the operations on sets. The
characteristic-function isomorphism 𝒫(V) ≅ 2^V is a theorem the checker uses, not a pun the reader has
to resolve. This amends settypes §7.2's spelling and leaves its decision as it is.

### 3.6 A pair is a pair: no structure by parity

**Semantics.** A `let` is a sequence of (binder, value) pairs and a body. A `loop` is a set of
guarded commands, each one (guard, body). A `case` is a set of (piece, arm) pairs.

**The misfit.** Today, five forms read their structure from the parity of a flat list:
- `let` and `local`: `x e y f body`;
- `loop`'s clauses: `c e c e else e`;
- `case`'s arms;
- `cond`.

This is an F3, and the project has paid for it. ADR 0024 refused the datum comment `#;` because
*"our forms are discriminated by arity and parity"*. `#;` would let a comment change which construct a
form is, *"Scheme can afford it because nothing in Scheme is parity-discriminated in that way."* A
rule about comments had to protect a misfit of binders.

**Fit.** Pairs written as pairs:
- M: `x == e`, and `[] g → e`;
- F#: `let`, and `| p -> e`;
- prefix: `((x e) (y f))` as in Scheme, or `(g → e)` clauses.

Clojure's vectors `[x e y f]` and its flat `cond` keep the parity, so C misfits here. Once the
structure is explicit, `#;` is as safe as it is in Scheme, and ADR 0024's principle (a comment never
carries meaning) no longer needs the refusal to protect it.

### 3.7 One eliminator, and derived forms that name a law

**Semantics.** Elimination is a membership test on a partition (settypes §2.2). A constructor is a
piece of a sum, a sort is a piece of V, and a guard is a piece of 𝔹. `try` is bind (a theorem of the
exception monad), `expect` is bind with an abandon arm, and `ignore` is !_A.

**Today.** Six forms eliminate: `if`, `cond`, `case`, `match` used as a choice, `try` and `expect`.

**Fit.** Not all six are F1. Felleisen's criterion ("On the expressive power of programming
languages", 1991) separates a derived form that is a mere macro from one that names an operation:
- **`try`, `expect` and `ignore` each name an algebraic operation with a law**, so they fit;
- **`cond` and `match`-as-choice are spellings of guarded choice**, so they are F1. notation-research's
  single `CASE` (on constructors, sorts or guards) fits;
- **`if` is `CASE` on {true, false}**: a synonym, kept in almost every language because the two-way
  case is by far the most common. Fit says one; legibility says keep it.

### 3.8 Tables are values, operators are not: `t[i]` against `f(x)`

**Semantics.** settypes §1.2: V contains tables and maps (finite functions as data) and no functions.
A λ is static and erased, and a host primitive is an operator that δ turns into a call. data.md's
*"indexing is application"* is true of the meaning (a table is a function on [0, n)), and the
untyped computation applies both alike.

**The two readings.** TLA+ distinguishes functions, which are values and are applied as `f[x]`, from
operators, which are not values and are applied as `F(x)`. That distinction is **exactly** ours:
what is in V, and what is not. Today's uniform `(t i)` fits data.md's reading and hides the V line.

**Measured** (a script over the 85 `.oro` files in `examples/` and `lib/`, the 334 definitions with a
parameter list, and their calls in the same file):
- **119 parameters are applied** in their definition's body and have at least one call;
- **3 receive a λ literal**: `get` in the three copies of `examples/native/generic-*.oro`'s
  `reduce-over`. **None receives both a λ literal and a table literal.** Arguments that are names were
  not resolved;
- **the one program that could pass a table as a function writes the λ instead**:
  `(reduce-over (len a) (fn (i) (a i)) …)`, η-expanding `a` where `a` alone would have reduced the
  same way.

So the uniformity is not used, and the corpus already writes the distinction. Marking data
application (`t[i]`, or `(at t i)` in prefix) costs nothing measured, and it shows the reader the line
between what lives at run time as data and what is reduced away or called. That is a F4 fix in today's
notation, and M has it already.

### 3.9 Staging is normalization: no level marks

**Semantics.** The static level is everything the reducer normalizes away, and the residual is what
remains (the atom). Every λ is static, because closures do not survive (closures-direction.md). The
binding time is not something a programmer decides. It is a result.

**Fit.** No binding-time annotations: no MetaML brackets (Taha and Sheard, 1997), no Template Haskell
splices, no Zig `comptime`. Each would claim the programmer decides what the reducer decides, an F4′.
The λ itself is already the static level's only mark, and every notation has one (`fn`, `λ`, `fun`).
**All candidates fit.**

### 3.10 The tail rule is grammar

**Semantics.** `again` is a jump: a clause's body, or under a binding whose body is a tail (ADR 0015,
0027, 0041), never under an `if`.

**Today.** This is checked by walkers, and *"every walker of a clause chain walks four forms, and one
that … misses a form is unsound"* (CLAUDE.md). That is F5.

**Fit.** The rule is a regular tree grammar (Comon et al., *Tree Automata Techniques and
Applications*, 2007):

```
clause ::= guard → tail
tail   ::= value | again(e…) | let p = e in tail | try p = e in tail | expect p = e in tail | host-call(…, λ p. tail)
```

Under it, an `again` under an `if` cannot be parsed. A context-free grammar (M, F) and a form checker
over S-expressions (a tree grammar) both express it, so **this is notation-neutral**. The point is to
make it the grammar in whichever notation is chosen.

### 3.11 Two languages: claims are erasable, formulas contain expressions

**Semantics.** Erasing every claim leaves a program with the same meaning (settypes §1.3), and the
logic may write what cannot run: ∀ over an infinite set, a recursive specification function, a lemma.

**Fit.**
- **Erasure is subtree deletion.** Claims sit in claim positions (a binder's set, the result's set,
  `where`, `assert`, `invariant`), and removing them never restructures the program. Every candidate
  can do this, and §3.4's single form makes it uniform.
- **Two nonterminals, Formula ⊇ Expr.** The non-computable forms parse only as Formula, and a claim
  position takes a Formula. Membership `x ∈ S` is in both: as a claim it is asserted, and as a test it
  is computed when S is decidable at run time (notation-research §4.4). One symbol fits, because the
  meaning is one (the characteristic function), and only the role differs.

**Notation-neutral.**

### 3.12 ρ: no representation names in the source

**Semantics.** A representation is chosen from a proven set (ADR 0049 §1, ADR 0033). A programmer
writes sets.

**Fit.** `0..255` and `(int 0 255)` are sets, and the IR chooses a byte. Host sorts (`go.int64`,
`go.bytestring`) are legitimate at a host's boundary, because they are the host's sets. F#'s `byte`,
`int32` and `uint64` as everyday types are an F4′ in this language: they name a ρ the compiler
should choose. M, P, T, C, S and F\* fit.

### 3.13 Substitution: the argument for prefix, measured

**Semantics.** β is substitution, and staging is β and δ run to normal form, so reasoning about what
a program reduces to is substitution done in one's head.

**The theorem.** In fully parenthesized prefix notation, substitution is textual (up to α): replace
the name with the argument's text. In infix notation with precedence, it is not. `x * 2` with
x := `a + b` must be written `(a + b) * 2`. And keyword forms that extend as far right as they can (M's
`IF`, `LET`, `λ`) must be parenthesized as operands and at a head. I expected this to be the
computation side's argument for S-expressions.

**Measured** (the same script as §3.8). For every call of a definition and every occurrence of a
parameter in its body, it asks whether the argument's infix rendering, put in place of the occurrence,
needs parentheses the text does not have. It uses notation-research §4.9's precedence; keyword forms
always need them as operands. Chained comparisons count as needing them, and so does the right operand
of `-`, `/` and `%`.

| | count |
|---|---:|
| parameter occurrences at calls | 1,590 |
| with a compound argument | 300 |
| with an argument headed by an operator or keyword form | 79 |
| of those, placed under a tighter operator | **0** |
| a λ substituted at the head of an application, `(λ i : a[i])(k)` | **6** |

The 79 operator arguments all land in looser contexts: `+` under `<` (10), `+` under `>=` (6), `+`
under `if` (10), and an argument of a call or an index, which delimits itself. **So 6 of 1,590
substitutions (0.4%) are not textual in an infix rendering, all of them a λ at a head.** The theorem
holds, and on this corpus it does not matter. This argument does not discriminate, and I withdraw it.

### 3.14 The logic's laws: mathematics

**Semantics.** The logic is a Boolean algebra with constructors and first-order formulas, and its
literature (Presburger, BAPA, the array property fragment, semantic subtyping) is written in
mathematical notation.

**Fit.** Three of its forms have a conventional notation that carries a law:
- **a chain**, `0 ≤ i < n`, is a conjunction sharing a term;
- **a binder with its set**, `∀ x ∈ S : P`, `{x ∈ S : P}`, is bounded quantification and comprehension;
- **set-builder notation**, for the same reason.

Associativity and commutativity are shown equally well by n-ary prefix: `(and a b c)` is closer to
the AC normal form a decision procedure computes, a multiset of operands, than `a ∧ b ∧ c` is. The
procedures themselves work on normal forms and do not care about notation. **So the logic leans to
mathematics only where these three forms appear: in claims.** That is syntax-research §5.3's finding,
now with its reason.

## 4. The fit table

✓ fits, ✗ misfits, – neutral (any notation can, the choice is inside it). The notations are as written
in syntax-research §3; F is F# with F\*'s refinements.

| § | criterion | M | F | P | T | C | S |
|---|---|:-:|:-:|:-:|:-:|:-:|:-:|
| 3.1 | one grammar for types and terms | ✓ | ✗ | ✓ | ✓ | ✓ | ✗ |
| 3.2 | usage as a mode on the binder | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| 3.3 | three binder kinds, three shapes | ✓ | ✓ | – | ✗ | – | ✗ |
| 3.4 | one definition carrying its claims | ✓ | ✗ | ✗ | ✗ | ✓ | ✗ |
| 3.5 | singleton and difference explicit | ✓ | ✓ | ✗ | ✗ | ✓ | – |
| 3.6 | pairs as pairs, no parity | ✓ | ✓ | ✓ | ✗ | ✗ | ✓ |
| 3.7 | one eliminator, derived forms with laws | ✓ | ✓ | – | ✗ | – | ✓ |
| 3.8 | data application marked | ✓ | ✓ | ✗ | ✗ | ✗ | – |
| 3.9 | no binding-time marks | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| 3.10 | the tail rule as grammar | – | – | – | ✗ | – | – |
| 3.11 | claims erasable, Formula ⊇ Expr | – | – | – | – | – | – |
| 3.12 | no representation names | ✓ | ✗ | ✓ | ✓ | ✓ | ✓ |
| 3.13 | substitution textual | 0.4% not | 0.4% not | ✓ | ✓ | ✓ | ✓ |
| 3.14 | claims read as mathematics | ✓ | – | ✗ | ✗ | ✗ | ✗ |

Reading it:
- **M fits best as written**: one misfit (§3.2), shared with every other notation. Most of its ✓ are
  abstract-syntax choices notation-research made, not consequences of infix.
- **Today's T fits worst**, and every one of its ✗ except §3.14 is an abstract-syntax choice that an
  S-expression notation could make the other way. **A fitted S-expression notation**, T′ below, would
  score ✓ everywhere but §3.14, and ✓ on §3.13 by the theorem.
- **F loses on §3.1 and §3.12**, both from F#'s Church-style heritage (a type grammar and machine
  types). F\*'s own syntax would recover §3.1.
- So the semantics separates **fitted from unfitted**, much more than it separates **prefix from
  infix**. Between M and T′ it leaves one row, §3.14, and that row is about claims.

## 5. What the fit changes, side by side

The core of `ages` (syntax-research example 5), today, fitted in prefix (T′), and fitted in M (M′).
T′ uses `(x e)` pairs for definitional binders, a `state` block whose binders use `:=`, `(x S)` for
ranging binders, `(g → e)` clauses, and modes on binders. Every change from T is one of §3's, and
`report`'s and `run`'s headers in T′ and M′ also carry claims (parameter and result sets) that today's
program does not state, to show ranging binders in use.

```lisp
; today
(def report (in n sum best-name best-age skipped)
  (case (Scanner.Err in)
    (result.err e) (seq (fmt.Println "ages: cannot read standard input") 1)
    (result.ok u)  …))

(def run ()
  (let in (bufio.NewScanner (gos.Stdin))
    (local n 0  sum 0  best-name ""  best-age -1  skipped 0
      (loop ((k 0))
        (not (Scanner.Scan in)) (report in n sum best-name best-age skipped)
        (>= k cap-lines)        (seq (fmt.Println "ages: more than 16777216 lines") 1)
        else
          (let (tuple name age got e) (fmt.Sscan (Scanner.Text in) (out go.bytestring) (out int))
            (seq (if (and (ok? e) (>= age 0) (<= age cap-age) (< n cap-lines)) … (tuple))
                 (again (+ k 1))))))))
```

```lisp
; T′: fitted prefix
(def report ((in Scanner) (n nat) (sum nat) (best-name string) (best-age (int -1 150)) (skipped nat)) (int 0 1)
  (case (Scanner.Err in)
    ((result.err e) → (seq (fmt.Println "ages: cannot read standard input") 1))
    ((result.ok u)  → …)))

(def run () (int 0 1)
  (let ((in (bufio.NewScanner (gos.Stdin))))
    (state ((n := 0) (sum := 0) (best-name := "") (best-age := -1) (skipped := 0))
      (loop ((k := 0))
        ((not (Scanner.Scan in)) → (report in n sum best-name best-age skipped))
        ((>= k cap-lines)        → (seq (fmt.Println "ages: more than 16777216 lines") 1))
        (else → (let (((tuple name age got e) (fmt.Sscan (Scanner.Text in) (out go.bytestring) (out int))))
                  (seq (if (and (ok? e) (>= age 0) (<= age cap-age) (< n cap-lines)) … (tuple))
                       (again (+ k 1)))))))))
```

```
\* M′: fitted mathematics
report(in ∈ Scanner, n ∈ Nat, sum ∈ Nat, best_name ∈ String, best_age ∈ -1..150, skipped ∈ Nat) ∈ 0..1 ==
  CASE Scanner.Err(in) OF
      err(e) → fmt.Println("ages: cannot read standard input"); 1
   [] ok(u)  → …

run ∈ 0..1 ==
  LET in == bufio.NewScanner(os.Stdin) IN
  LOCAL n := 0, sum := 0, best_name := "", best_age := -1, skipped := 0 IN
  LOOP k := 0 :
      ¬Scanner.Scan(in) → report(in, n, sum, best_name, best_age, skipped)
   [] k ≥ cap_lines     → fmt.Println("ages: more than 16777216 lines"); 1
   [] OTHER → LET (name, age, got, e) == fmt.Sscan(Scanner.Text(in), OUT go.bytestring, OUT Int) IN
              IF ok(e) ∧ 0 ≤ age ≤ cap_age ∧ n < cap_lines THEN … ELSE ⟨⟩;
              AGAIN(k + 1)
```

The declaration it calls, in the three:

```
today  (sig Sscan ((str bytestring) (a (array (cell go.Value))))
         (tuple (int 0 9223372036854775807) (result.result (tuple) error)) variadic (host …))
T′     (def Sscan ((str bytestring) (a go.Value inout variadic))
         (tuple nat (result.result (tuple) error)) (host …))
M′     Sscan(s ∈ go.bytestring, a ∈ go.Value INOUT VARIADIC) ∈ Nat × Result(Unit, error) == HOST …
```

Counted as syntax-research §4 counts, over the three `ages` blocks above:

| | tokens (approx.) | deepest nesting | longest closing run |
|---|---:|---:|---:|
| today | 173 | 9 | 8 |
| T′ | 253 | 10 | 9 |
| M′ | about 200 | 2 | 1 |

T′ carries claims that today's block does not, so its token count is not a like-for-like comparison
with T. M′ carries the same claims in about 200 tokens, an approximate count because the counter
strips text after `;`, and M′ uses `;` for sequencing. Explicit pairs cost T′ one more level of
nesting. That is the price of §3.6 in prefix notation, and it is syntax-research's legibility
question, not this one's.

## 6. Where hamza is right, and where the derivation corrects it

**Right:** some syntax maps better to the semantics than other syntax, and the difference is not
taste. It is the distance between a notation's abstract syntax and the initial algebra of the
semantic signature, and it can be counted (§1.2, §4). On that count, today's notation is the furthest
from the semantics of any notation compared. The reason is not that it is made of S-expressions. Its
forms grew one at a time, before ADR 0049 said what the semantics is.

**Corrected in three ways:**
1. **The fit is decided mostly in the abstract syntax.** Prefix against infix, the question
   syntax-research put side by side, is decided by the semantics in only one row (§3.14, claims as
   mathematics). β's textual substitution, which I expected to favor prefix, measured 0.4% (§3.13).
2. **No notation written so far fits the modalities** (§3.2). Each copied the C++/Rust habit of usage
   as a type constructor (`&mut T`, `Buffer<T>`), which the semantics refuses.
3. **settypes §7.2's own spelling misfits in two places** (§3.5): the singleton pun and the relative
   complement. The decision stands, and its spelling should change.

**What the semantics does not decide**, and so stays with the readers: words or symbols, capitals,
parentheses or indentation, and whether there is one notation or two. syntax-research's synthesis (one
source, two renderings) is compatible with every row of §4. A rendering is a homomorphism of the same
abstract syntax into another concrete one, so it fits exactly as well as the source does. That is the
strongest reason to fix the abstract syntax first: then the physical notations, one or two, are
renderings of one fitted algebra.

## 7. Decisions

Decided 2026-10-10, as recommended: 1–6 by [ADR 0050](decisions/0050-the-abstract-syntax-fits-the-semantics.md).
7 is open with syntax-research.


1. **Specify "abstract Oroboros" before choosing a surface**: the signature of §2 as an abstract
   grammar, its three binder kinds, Formula ⊇ Expr, and the tail rule as grammar (§3.10). It is
   spec-first, it is shared by every candidate surface, and the translator notation-research §8
   proposes would be a homomorphism out of it. **Decided**: [spec/abstract.md](spec/abstract.md).
2. **Usage as modes on binders** (§3.2): `linear` and `inout`/`out` on a binder, in place of the type
   formers `(buffer V)` and `(cell T)`; `relevant` as a variant's default mode; `pure` on a
   declaration. The semantics is unchanged, and it would need an ADR for the surface of ADR 0020 and
   ADR 0048. **Decided.**
3. **One definition form carrying its claims** (§3.4): declarations then a predicate, the Z schema
   shape, merging `sig`, `def`, `where`, `ensures` and `requires`; a host declaration is the same form
   with a host body. **Decided.**
4. **An explicit set former and difference** (§3.5), amending settypes §7.2's spelling: `{…}` or
   `(set …)`, and `S ∖ T` or `(minus S T)`. A literal is never its own singleton, and `or`/`and`/`not`
   stay connectives. **Decided.**
5. **Pairs as pairs** (§3.6) and **one guarded choice** (§3.7) in whichever notation is chosen. The
   datum comment then becomes safe, which a successor to ADR 0024 could allow while keeping its
   principle. **Decided**; admitting `#;` is not, and would be a successor to ADR 0024.
6. **Mark data application** (§3.8): `t[i]` (or `(at t i)`) for a table or map, `f(x)` for an operator.
   The measurement shows the uniformity unused. **Decided.** It is the one change that departs from
   data.md's "indexing is application" as a reading of the source, though not as semantics.
7. **The surface stays syntax-research's decision**, with two constraints from here: no separate type
   grammar (§3.1), and no binding-time marks (§3.9). On the remaining question, prefix or mathematics,
   the semantics leans to mathematics for claims only (§3.14), which is what the two-renderings
   synthesis already gives. **Open.**

Decisions 2–6 hold in any notation, which is why they could be taken before the surface is chosen.
