# Abstract Oroboros

Status: **decided; steps 1a, 1b and 1c built** ([abstract1-2026-10-11](../../gauntlet/results/abstract1-2026-10-11.md):
the computation's terms, `core/abs.go`, `core/mu.go`, `core/desugar.go`;
[abstract1b-2026-10-11](../../gauntlet/results/abstract1b-2026-10-11.md): program declarations, `core/decl.go`,
with [ADR 0051](../decisions/0051-a-definition-is-a-named-lambda.md)'s definition as a named λ;
[abstract1c-2026-10-11](../../gauntlet/results/abstract1c-2026-10-11.md): target files' declarations as
definitions with a host body, usage as modes, `emit/hostdef.go`).
Written 2026-10-10 on hamza's request ("specify. always mathematical
and algebraic"); [ADR 0050](../decisions/0050-the-abstract-syntax-fits-the-semantics.md) decides it
with [fit-research.md](../fit-research.md) §7 decisions 1–6, as recommended. The surface notation is
not decided (syntax-research.md), and §9.4's spellings are illustrations. The semantics is the language's as it stands,
ADR 0049's included. Only the presentation is new, except where §11 names a change.

The language is specified here as an algebra of trees, with no notation. A notation (§9) is a
printer and a parser of these trees, and each meaning (§5–§8) is a homomorphism out of them.

## 0. Summary

1. **The syntax is the initial algebra of a many-sorted binding signature Σ** (§1, §2): eight syntactic
   sorts, thirteen primitive constructors of expressions, six derived ones, each with its defining
   equation, and three kinds of binder.
2. **Well-formedness is eleven syntactic conditions** (§3), decidable in one pass. Several rules that
   today's compiler enforces by walkers are here the sort discipline itself, among them the tail rule
   (ADR 0015) and the claim-only forms.
3. **Four meanings, each defined by structural recursion, so each is the unique homomorphism with its
   clauses:**
   - erasure ε, which removes the logic (§5);
   - desugaring D, into today's core (§6);
   - obligations 𝒪, which hand the logic its sequents (§7);
   - usage 𝒰 and effects 𝓔, the modalities (§8).
4. **The meaning of a program is ⟦D(ε(P))⟧**, the atom's reduction of its erased desugaring. So the
   logic is inert by construction (Theorem E), and ADR 0009's staging law for claims is a definition,
   not a property to maintain.
5. **A notation is a rendering** (§9): a parser and a printer satisfying two round-trip laws, with
   comments attached to positions. Today's notation is not a rendering but maps into this algebra by μ
   (§10). The build check is that μ followed by D reproduces today's reader byte for byte.

## 1. Preliminaries

### 1.1 Binding signatures

A **many-sorted binding signature** (Fiore, Plotkin and Turi, "Abstract syntax and variable binding",
LICS 1999; Harper, *Practical Foundations for Programming Languages*, 2012, ch. 1) is a set of sorts S
and a set of operators, each with an arity

```
o : (s̄₁.s₁, …, s̄ₙ.sₙ) → s
```

meaning that o takes n arguments, the i-th of sort sᵢ with variables of sorts s̄ᵢ bound in it. Its
terms modulo α-equivalence, the **abstract binding trees**, form the initial algebra of the signature
(in nominal sets, Gabbay and Pitts, "A new approach to abstract syntax with variable binding", *FAC*
13, 2002). Three consequences are used throughout:
- **structural induction** is the proof principle;
- **structural recursion** defines functions: a set of clauses, one per operator, determines a
  unique homomorphism out of the initial algebra (Goguen, Thatcher, Wagner and Wright, *J. ACM* 24(1),
  1977). Every meaning in §5–§8 is one;
- **capture-avoiding substitution** is defined once, for all sorts.

`core.Term` already represents the atom this way: locally nameless, with `KBound` the bound variable
and names kept as hints (state.md §1).

**Sequences.** An operator taking a sequence (n binders, k arms) is a family oₙ indexed by length.
`ā` is a sequence a₁ … aₙ, and |ā| is its length.

### 1.2 Lexical carriers

| carrier | elements |
|---|---|
| 𝒩 | names, and qualified names `p.x` (ADR 0025: a module path is the host's path) |
| 𝒦 | literals: ℤ, F (IEEE doubles), Σ* (strings of scalar values), 𝔹 = {true, false} |
| 𝒯 | host templates: strings with holes `%s`, `%s...`, `%t1` (target-files.md §3) |

Names are atoms. **Their spelling belongs to a rendering** (§9.3): `best-name` and `best_name` are two
spellings of one name.

### 1.3 Owned names, and what is not syntax

**A construct is an operator of Σ exactly when one of the following holds:**
- it binds a variable;
- it controls evaluation (a choice, a loop, state);
- it carries a claim or a mode.

Everything else is a **name applied to arguments**: integer arithmetic and comparison, `=`, `tuple`,
a variant's constructors (generated definitions, sums.md §3), `array`, `table`, `len`, `alloc`, a
buffer's threaded store `store` (today's three-argument `set`), `map`, `insert`, `keys`, `concat`, `string-of`, `abandon`, `ignore`. On the logic's
side, the set operations are names too: `int`, `f64`, `bool`, `string`, ranges, ∪ ∩ ∖, the set former
`set(…)`, ×, `Array` (ADR 0059; `Seq` until then), `Map`, `option`, `subsets`. These are constants of a first-order signature Σ₀ that
the language owns and injects into every target (state.md §1), plus the names a program or a target
declares.

This is the atom's own principle, the normal form as a parameter, applied one level up: the abstract
syntax fixes binding, control and claims, and leaves operations to Σ₀. Types are static values
(ADR 0049 §5), so a set expression is an expression. Only the set former that binds, comprehension,
is an operator.

## 2. The signature Σ

### 2.1 Sorts

| sort | what it is |
|---|---|
| **Pat** | patterns: what an arm tests and what a binder destructures |
| **Bind** | binders, in three kinds (§2.2) |
| **Arg** | an argument of an application: an expression, or a spread or out-cell |
| **Expr** | the computation's terms, sets included (types are static values) |
| **Tail** | what may end a loop's clause: Expr ⊆ Tail, plus `again` and bindings around a tail |
| **Arm** | a guarded alternative: a pattern, a guard, and a body |
| **Formula** | the logic's sentences: Expr ⊆ Formula, plus quantifiers and claim-only forms |
| **Decl**, **Module** | definitions, variants, specifications, lemmas; and their grouping |

**Subsorts** (order-sorted algebra, Goguen and Meseguer 1992): Expr ⊆ Tail and Expr ⊆ Formula. The
two inclusions carry two of the language's laws:
- **ADR 0015's tail rule** is Tail ⊋ Expr: `again` is an operator of sort Tail and of no other sort,
  so an `again` under an `if` is not a tree;
- **ADR 0049's two languages** are Formula ⊋ Expr: the claim-only operators (∀, ∃, a specification
  function's call, equality off the integers) are of sort Formula, so the computation cannot contain
  them.

### 2.2 Binders and modes

Every binder is one of three kinds (fit-research §2, §3.3):

```
Bind ::= p = e            definitional:  p denotes e's value           (let; a loop's variables, ADR 0055)
       | x := e           state:         x starts at e, written by assign in its region   (a cell; a buffer, ADR 0052)
       | p ∈ S  [μ]        ranging:       p ranges over S             (a parameter, ∀, ∃, comprehension)
       | p        [μ]      ranging, no claim: p ∈ V with no obligation   (settypes §10: "no claim" ≠ V)
       | f ∈ λ(b̄; r; φ; ψ)  ranging over operators: f ∈ Π(P). R, a claim used as a set (ADR 0053)
```

> **Amended by [ADR 0052](../decisions/0052-a-buffer-is-a-state-binder-holding-a-table.md), not built.**
> A buffer is a **state** binder whose value is a table, `b := table(n; f)`, written in program order:
> `assign(b[i]; e)` writes an element, `assign(b; e)` the whole value. A definitional binder holding a
> table is an immutable table. `linear` is written only on a ranging binder, a parameter, where it is
> a borrow passed by reference. Linearity through a body follows from R0–R4: a buffer's value is fresh,
> it is named only by its binder, a call borrows it and is never handed it twice, no closure over it
> survives its scope, and the scope's value freezes it. Σ₀'s `store` leaves the source signature and
> stays in Λ⁺ as D's image. This section, §2.4 and §9.4's `local` and store rows are rewritten when the
> surface is built.

**Modes** μ (fit-research §3.2, decision 2, ADR 0050):

| mode | kind | meaning | where |
|---|---|---|---|
| `linear` | usage | used exactly once on every path (ADR 0018, 0020) | a parameter; implied on a buffer binder (ADR 0052: a parameter only) |
| `inout` | passing | call by value-result: a reference to a fresh copy in, the copy's value out (ADR 0048; Algol W, Ada's `in out`) | a host declaration's parameter |
| `out` | passing | value-result with no value in: a fresh zero in, a value out (ADR 0048 §6) | a host declaration's parameter |

**Relevance is a property of a set's values**, declared on a variant (`relevant`). A binder ranging
over a relevant variant carries the relevant usage by default (ADR 0043). **Purity is a property of a
declaration** (§2.6). Usage modes are judgements (§8) and are erased. Passing modes change a call's
translation (§6) and are kept.

### 2.3 Patterns

```
Pat ::= _                 wildcard
      | x                 a name
      | k                 a literal, k ∈ ℤ ∪ 𝔹 (match.md §2: no float or string patterns)
      | c(p̄)              a constructor of a declared variant
      | tuple(p̄)          a product
      | x ∈ S             a set test that binds x at S's sort: occurrence typing (settypes §2.2)
```

bv(p) is the set of names a pattern binds. A pattern is **irrefutable** if it is `_`, a name, or a
tuple of irrefutable patterns. A constructor pattern's sub-patterns are names or `_` today (sums.md),
and §12 names nesting as an extension.

### 2.4 Expressions

The thirteen primitive operators, with arities written as in §1.1, and a name for each:

| operator | arity | meaning |
|---|---|---|
| `x`, `k` | variable, literal | |
| **lam**(b̄; r; φ; ψ; e) | (Bind*. (Bind. (Formula, Formula, Expr))) → Expr | the λ; its binders are ranging, with an optional set and mode, and it carries what it claims: the result binder r with its set, a precondition φ and a postcondition ψ (ADR 0051). Static: erased by staging (closures-direction.md) |
| **app**(e; ā) | (Expr, Arg*) → Expr | application, of a definition, a host name, a λ, or an owned name |
| **index**(e; e′) | (Expr, Expr) → Expr | a table's or map's application to a point (fit-research §3.8, decision 6, ADR 0050) |
| **let**(b̄; e) | (Bind*. Expr) → Expr | definitional binders, sequential: bᵢ's names are bound in bᵢ₊₁ … and in e. ADR 0054 (not built): `let` and `local` are one scope, scope(b̄; e), whose binders are definitional or state, each by its operator; a scope is the free monoid of binder words acting on expressions |
| **case**(e; ā) | (Expr, Arm*) → Expr | choice: the first arm whose pattern matches e's value and whose guard holds |
| **loop**(s̄; I; ā) | (Bind*. (Formula, Arm*)) → Expr | iteration: definitional binders s̄, bound at entry by their initializers and at each again by its arguments, by application (ADR 0055); invariant I; arms with Tail bodies (ADR 0015) |
| **local**(b̄; e) | (Bind*. Expr) → Expr | local state: buffer binders (definitional, linear) and cell binders (state); ADR 0047. ADR 0052 (not built): both are state binders, a buffer the one whose value is a fresh table. ADR 0054 (not built): merged into `let`'s scope |
| **assign**(x; e) | (𝒩, Expr) → Expr | a cell's write in program order; value the unit (local.md §3) |
| **compr**(p ∈ S; φ) | (Expr, Pat. Formula) → Expr | the set {p ∈ S : φ}, a static value: separation |
| **image**(p̄ ∈ S̄; e) | (Expr*, Pat*. Expr) → Expr | the set {e : p̄ ∈ S̄}, a static value: replacement (ADR 0061) |
| **mem**(e; S) | (Expr, Expr) → Expr | membership as a test, where S is decidable at run time (W6) |
| **the**(S; e) | (Expr, Expr) → Expr | a claim: e's value lies in S; the value is e's |
| **assert**(φ; e) | (Formula, Expr) → Expr | a claim: φ holds here, then e (settypes §6.3) |

```
Arg ::= e | spread(e) | out(S)          spread: a table as a variadic list (ADR 0046);
                                         out: a fresh cell at an out parameter (ADR 0048 §6)
Arm ::= p ⟨if φ⟩ → body                  body : Expr in case, Tail in loop; bv(p) bound in φ and body
```

In a loop, an arm's pattern is matched against the tuple of the state variables. A pattern of `_`s
is today's guarded clause, and any other pattern is `match` (match.md: a match is a loop whose
scrutinees are its variables). So **`match` is not an operator. It is a loop whose arms have
patterns.**

### 2.5 Tails and formulas

```
Tail    ::= e
          | again(ā)                                   the back edge; |ā| = |s̄| of the nearest loop
          | let(b̄; t)                                   a binding around a tail; a host call's continuation
                                                       is a let with a tuple pattern (ADR 0027)
          | try(p; e; t)  |  expect(p; e; w; t)          a success arm is a tail (ADR 0041)
          | bind(p; e; ā; t)                              its handler's arms leave the loop (ADR 0057)
          | assert(φ; t)

Formula ::= e                                          a pure boolean expression
          | e ∈ S                                       membership, any S (a claim, not a test)
          | e ≐ e′                                      the logic's equality, at any sort
          | ¬φ  |  φ ∧ ψ  |  φ ∨ ψ  |  φ ⇒ ψ
          | ∀ b̄. φ  |  ∃ b̄. φ                          b̄ ranging binders
          | app(f; ā)                                   with f a specification function (§2.6)
```

`≐` is the logic's identity of values. The computation's `=` is integer equality (match.md §6), a
name of Σ₀, and the two agree on ℤ.

### 2.6 Declarations and modules

```
Decl ::= def(f; e)                           a name bound to a value; a function's is a named λ (ADR 0051)
       | def(f; λ(b̄; r; φ; ψ); host(kind; template; clauses))   a host's: the λ's claims, a host body
            kind ∈ {expr, stmt};  markers ⊆ {pure, index, variadic}
       | def(n; λ(b̄; r; φ; ψ))                 a named claim, static: a host's named function type (ADR 0053)
       | variant(n; b̄; c₁(S̄₁) … cₖ(S̄ₖ); success c?; relevant?)
       | spec(f; b̄; r ∈ U; e)                 a specification function: logic only, may recur
       | lemma(n; φ; steps)                   a proven sentence; steps a sequence of formulas

Module ::= module(path; decl*) | use(path; alias?) | export(f̄) | provides(target; path; decl*)
```

- **def.** A definition binds a name to a value, the binder kind `let` has, so a module is a group of
  definitional binders. **A function's definition is a named λ, and the λ carries its claims**
  (ADR 0051, D1): its binders' sets, the result binder r with its set U, φ and ψ. A constant is a name
  bound to any other value (def.md §4). The binders' names and r are bound in φ, ψ and body. φ is the
  precondition, a comprehension over the parameter tuple: today's `where` and `requires`. U and ψ are
  the result's set and the postcondition, a comprehension over the result: today's result type and
  `ensures`. **The tree keeps a set per binder; the meaning is two sets** (D2), P = {x̄ ∈ S̄ : φ} and
  R = {r ∈ U : ψ}, since ρ chooses each value's representation from its own set and ADR 0028 checks a
  parameter's set on its argument. **The result binder's pattern may be a tuple** (D3),
  `tuple(q, r) ∈ int × int`, which names several results. So a λ with its claims states one
  membership,
  ```
  f ∈ Π(x̄ ∈ {x̄ ∈ S̄ : φ}). {r ∈ U : ψ}
  ```
  which is a Z schema (Spivey 1992) in its declarations and predicate. **A host declaration is the same
  operator with a host body**, an axiom checked against the host (ADR 0022).
- **A function-space claim is a claim used as a set** (ADR 0053, not built). A host's callback
  parameter ranges over operators: `less ∈ λ(i ∈ int, j ∈ int; r ∈ bool)`, meaning
  less ∈ Π(P). R, P the parameters' comprehension and R the result's, intersected with what the
  claim's markers require (call count, escape, no captured buffer, decided with callbacks tier 1).
  An argument with claim (P_f, R_f) fits iff P ⊆ P_f and ∀x̄ ∈ P. R_f(x̄) ⊆ R(x̄): inclusions
  between sets of values. Π is not an operation of the type algebra: no ∪, ∩ or ∖ applies to a
  claim, and no claim is a value, so tier 3 stays refused (arrow-research.md).
- **Polymorphism needs no operator.** A static parameter is a ranging binder over a set of sets,
  `E ∈ subsets(error)`, and instantiation is app (ADR 0049 §5).
- **variant.** Its binders are its type parameters, ranging over sets of values. Its constructors are
  generated definitions (sums.md §3). `success` and `relevant` are the role markers (errors.md §2).
- **spec.** The logic may define by recursion over finite data what the computation may not
  (settypes §6.3). A spec's name may be called only in a Formula (W6).
- **lemma.** Its proof is a sequence of formulas, each decided from the ones before (settypes §6.3:
  Coq's power by structure, not by proof terms).

**Target declarations** (`type`, `repr`, `implements`, `fact`, `underlying`, `const`, `structural`,
`backend`, `link`, `max-len`) are a second signature Σ_T, the model's data (ADR 0021,
target-files.md §1). In the algebra:
- `implements T I` is the inclusion axiom T ⊆ I;
- `fact` is a universally quantified axiom;
- `repr` is a clause of ρ.

Σ_T is specified by target-files.md and is unchanged here.

### 2.7 Derived operators

Each derived operator is a **definitional extension**: an equation in Σ's primitive terms, so it adds
no expressive power in Felleisen's sense ("On the expressive power of programming languages", 1991).
Each is kept because it names an algebraic operation with a law (fit-research §3.7).

| derived | definition | the law it names |
|---|---|---|
| if(c; a; b) | case(c; true → a, false → b) | 𝔹 = 1 + 1 and its eliminator (ADR 0017) |
| cond(φ₁ → e₁, …, else → eₖ) | case(tuple(); _ if φ₁ → e₁, …, _ → eₖ) | guarded choice (Dijkstra 1975) |
| seq(e; e′) | let(_ = e; e′) | sequencing as a discarded binding (ADR 0010) |
| a ∧ b, a ∨ b, ¬a | if(a; b; false), if(a; true; b), if(a; false; true) | the connectives as 𝔹's eliminator (booleans.md) |
| try(s(x̄); e; b) | case(e; s(x̄) → b, c₁(ȳ₁) → c₁(ȳ₁), …, cₙ(ȳₙ) → cₙ(ȳₙ)) | the exception monad's bind (errors.md §5) |
| expect(s(x̄); e; w; b) | case(e; s(x̄) → b, cᵢ(ȳᵢ) → app(abandon; w) …) | bind, with crash : R → 0 (errors.md §8) |
| bind(p; e; ā; b) | case(e; p → b, ā) | the handled bind: the success arm b continues, the arms ā handle the rest; try and expect are its instances (ADR 0057) |

In try and expect, the cᵢ are the other constructors of s's variant. So these two are defined
**relative to a declaration environment**, which is why today they expand in `Load` and not in the
reader (sums.md §3).

## 3. Well-formedness

Eleven conditions on a tree, each syntactic and decidable in one pass, given the declaration
environment of resolution (ADR 0011).

| | condition | today |
|---|---|---|
| **W1** | **scope**: every free name of a declaration is bound in it, declared in its module, imported, owned (Σ₀) or a target's | resolution |
| **W2** | **linearity of patterns**: the names of one pattern are distinct, and so are the names of one loop's state. In a scope, by kind (ADR 0054): a definitional binder may rebind a name, a nested scope by the action's law, so it shadows (today's `let`: `(let x 1 x (+ x n) x)` ⟶ `(+ 1 n)`); a state binder's name is bound once in its region (today's `local` refuses a repeat) | match.md §2, local.md |
| **W3** | **the tail rule**: `again` occurs only as a Tail of the nearest enclosing loop's arm, with \|ā\| = \|s̄\|, and no λ lies between them | iteration.md; four walkers |
| **W4** | **a loop is total**: its last arm's pattern is irrefutable and its guard is absent (`else`, iteration.md §2) | the reader |
| **W5** | **a cell is written in its scope**: assign(x; e) has x a state binder of an enclosing local (ADR 0054: of an enclosing scope); a λ between them is an elimination's, a let's or an arm's, or is marked for the reducer's decision on a normal form (local.md §3, `#elim`) | the cell translation |
| **W6** | **the logic stays in the logic**: a Formula-only operator (∀, ∃, ≐ off ℤ, a spec's call) occurs only in a claim position: a binder's set, φ, U, ψ, the, assert, an invariant, a lemma; mem(e; S) and compr in a computation need S's membership decidable at run time | new |
| **W7** | **modes where they mean something**: `inout` and `out` only on a host def's parameters; out(S) and a cell only as arguments at such a parameter; `linear` only on a ranging binder of a def or λ; a claim as a binder's set only on a host def's parameter, or as a named claim's value | local.md §5–6, ADR 0020, ADR 0053 |
| **W8** | **no recursion**: the dependency graph of defs is acyclic (ADR 0014); a spec may refer to itself and to other specs, and owes a well-foundedness obligation (§7) | the front end |
| **W9** | **stratification**: a binder's set is a set of values or a set of sets of values, never higher; no set contains a set of its own level. Beside them, not above: a claim whose binders range over sets of values, meaning a set of operators (ADR 0053) | new (ADR 0049 §5: no Type : Type) |
| **W10** | **variants**: constructors distinct; `success` names one of them; `success` and `relevant` are not constructor names | sums.md §2 |
| **W11** | **owned names are not declared** | state.md §2 |

W3 and the first half of W6 are not checks at all. They are the sorts of §2.5, so a parser that
builds trees of these sorts cannot produce a violation. **Today they are enforced by walkers**, and
"one that misses a form is unsound, not imprecise" (CLAUDE.md). This is the main structural gain of
the abstract syntax (fit-research §3.10).

**Built (step 1a): the walkers are deleted.** In a reader that builds trees bottom-up, as today's
reads, an `again` is built before the form around it is known. So a constructor that needs an Expr
where it receives a Tail *demotes* it to the free name `again` applied, and marks the tree stray; a
loop refuses a stray clause body. Within a loop the sort decides, and outside every loop a demoted
`again` is the unbound name it is.

**A λ applied by hand is no binding** (ADR 0051, D5, refused since step 1b): `((fn (x) (again x)) e)`
puts a λ between a loop and its jump. Today's walker accepted it, because it could not tell it from the
redex a `let` reads as; in the tree `let` is its own operator, and `let` is how a jump is bound.

## 4. Fragments

| fragment | operators |
|---|---|
| **A** | all of Σ |
| **A_C**, the computation | Σ without the claim operators: no the, assert, invariants, φ, ψ, U, binder sets on λ and def, spec, lemma; usage modes absent; passing modes kept |
| **A₀**, the core | A_C without the derived operators of §2.7 |

A_C ⊆ A and A₀ ⊆ A_C are subalgebras. Passing modes and mem tests are in A_C because they compute.

## 5. Erasure

ε : A → A_C is the homomorphism with these clauses, and the identity on every other operator:

```
ε(lam(b̄; r ∈ U; φ; ψ; e))         = lam(names(b̄); ε(e))      a λ's claims go; passing modes kept
ε(loop(s̄; I; ā))                  = loop(s̄; ⊤; ε(ā))
ε(the(S; e))                      = ε(e)
ε(assert(φ; e))                   = ε(e)           and ε(assert(φ; t)) = ε(t)
ε(spec …) = ε(lemma …)             = nothing        (removed from the module)
```

names(b̄) keeps each binder's pattern and passing mode, and drops its set and usage mode. Arm
patterns `x ∈ S` are tests, not claims, and stay.

**Proposition E1.** ε is a retraction of A onto A_C: ε ∘ ι = id on A_C, where ι : A_C → A is the
inclusion, and hence ε ∘ ε = ε. *Proof.* Each clause maps a claim operator to its computational
argument, and every other operator to itself. On a tree of A_C no claim operator occurs. ∎

**Algebraically, ε is the unique homomorphism into A_C made a Σ-algebra by interpreting each claim
operator as a projection**: the(S; −) and assert(φ; −) as the identity on their last argument, a
binder's set as forgetting it. That is the precise sense in which the logic never prints.

### 5.1 The meaning of a program

```
⟦P⟧  :=  ⟦D(ε(P))⟧_atom
```

where ⟦·⟧_atom is the atom's reduction to a target's normal form (the-atom.md, state.md §1), D is §6,
and the residual is printed by the IR's printers.

**Theorem E (claims are inert).** For programs P and P′ with ε(P) = ε(P′), ⟦P⟧ = ⟦P′⟧. *Proof.*
Immediate from the definition. ∎

This is ADR 0009 ("staging must not change an answer") and ADR 0049 §1 ("the logic never prints")
together, holding by construction. What the logic *may* affect is **legality**: a program whose
obligations (§7) are not all discharged is refused. And through ρ it affects **representation**,
which by ADR 0049 §1 changes no answer.

## 6. Desugaring

D : A → Λ⁺ maps into **today's core**: the atom (seven term kinds, state.md §1) extended with the
forms `Load` still expands. These are `case`, `try` and `expect` (sums.md §3, errors.md §5, §8); the
cell form `#cell` and the state-passing translation S⟦·⟧ (local.md §3); `build` and `build-map`
(tables.md §2.4, maps.md); and the variadic, `spread` and `out` rewrites (variadic.md, local.md §6).
Choosing Λ⁺ and not the bare atom makes D the reader's output, so every later stage stays as it is.

**D carries claims; ε is applied after the logic has read them** (corrected 2026-10-11, abstract1). A
claim operator maps to today's mark: `the(S; e)` to `(the S e)`, which the checker reads
(`emit.KeepSortAscriptions`) and emission erases, and a definition's claims to its `sig` (step 1b).
This is ADR 0037's pattern, obligations in the residual. Theorem E is unchanged: every mark is erased
before a file is printed, so the printed program is D(ε(P))'s.

D is the homomorphism with these clauses (⌜S⌝ is S in today's type spelling, §10):

```
D(x) = x        D(k) = k
D(lam(x̄; e))                 = (fn (x̄) D(e))
D(app(e; ā))                 = (D(e) D(a₁) … D(aₙ))
D(spread(e)) = (spread D(e))      D(out(S)) = (out ⌜S⌝)
D(index(e; i))               = (D(e) D(i))                    indexing is application (tables.md §3)

D(let(x = e, b̄; e′))         = ((fn (x) D(let(b̄; e′))) D(e))
D(let(tuple(ȳ) = e, b̄; e′))  = (D(e) (fn (ȳ) D(let(b̄; e′))))   marked a tuple binding (iteration.md §2)
D(let(; e))                  = D(e)

D(case(e; c₁(x̄₁) → e₁, …))   = (case D(e) (c₁ x̄₁) D(e₁) …)    constructor arms, no guards: Load's case
D(case(e; ā))                = let s = D(e) in T(s; ā)        otherwise, a decision tree:
    T(s; p if φ → b, ā′)     = (if ⌈p⌉ₛ ∧ φ then b else T(s; ā′))   first match
    T(s; )                   = unreachable by exhaustiveness (§7)
    ⌈_⌉ₛ = ⌈x⌉ₛ = true,  ⌈k⌉ₛ = (= s k),  ⌈true⌉ₛ = s,  ⌈false⌉ₛ = (not s),
    ⌈x ∈ S⌉ₛ = the tag test of S's sort (settypes §6.4; not built)

D(loop(x̄ = z̄; _; ā))         = (loop (fn (x̄) C(ā)) z̄)       ā with only _ patterns
    C(_ if φ → t, ā′)        = (if φ D(t) C(ā′))       C(_ → t) = D(t)
    D(again(ā))              = (again D(a₁) … D(aₙ))
D(loop(x̄ = z̄; _; ā))         = match.md's expansion            arms with patterns on the state

D(local(b =₁ table(n; f), b̄; e))  = (build D(n) (fn (b) fill(b, f) D(local(b̄; e))))   local.md §1
D(local(x := e₀, b̄; e))      = (#cell D(e₀) (fn (x) D(local(b̄; e))))   then S⟦·⟧ in Load
D(assign(x; e))              = (set x D(e))
D(mem(e; S))                 = the decision of S's membership on D(e): a range a comparison, a sort a tag test

D(try(s(x̄); e; b))           = (try (s x̄) D(e) D(b))
D(expect(s(x̄); e; w; b))     = (expect (s x̄) D(e) D(w) D(b))
D(derived o)                 = D(its definition, §2.7), except if, which is (if D(c) D(a) D(b))
```

Host definitions go to today's target-file form, with modes read back into today's type formers:

```
D(def(f; b̄; …; host(kind; τ; cl)))  = (sig f (D(b₁) …) ⌜U⌝ markers (host kind τ cl…))
D(x ∈ S inout) = (x (cell ⌜S⌝))      D(x ∈ Array(S) linear) = (x (buffer ⌜S⌝))      D(x ∈ S) = (x ⌜S⌝)
```

### 6.1 Properties

**Proposition D1 (D is compositional).** D is defined by structural recursion. So D(e[a/x]) =
D(e)[D(a)/x] for every operator but the four whose clause reads a declaration (case, try, expect
and loop with patterns), where it holds once the declaration is fixed. *Proof.* Structural induction;
each clause is a context applied to the images of its arguments. ∎

**Proposition D2 (the derived operators are conservative).** For each derived operator o with
definition δₒ, D(o(…)) and D(δₒ(…)) reduce to α-equal atom terms. *Proof.* For if, by the
conditional's laws (booleans.md: `(if (not c) a b)` is built as `(if c b a)`, and
`(if true a b) → a`). For try and expect, by errors.md §5 and §8's expansions, which are Load's. For
the connectives, by booleans.md §4.2. ∎

**Theorem D3 (conservativity over today's language), the build check.** For every program t of the
corpus, D(ε(μ(t))) ≡_α read_T(t), where μ is §10's map from today's notation and read_T is today's
reader. **It is not proven here. It is the test of the first build step** (§13): implement μ and D,
and require `cmd/check` byte-identical on every emitted file, refusal and proof count.

## 7. Obligations

𝒪 maps a tree to a finite set of **sequents** Γ ⊢ φ, Γ a context of formulas. Each sequent is decided
by the procedures of settypes §5 or the program is refused (ADR 0049 §2). 𝒪 is a verification-condition
generator in the tradition of Floyd ("Assigning meanings to programs", 1967), Hoare (1969) and
Dijkstra's weakest preconditions (1975), presented as one clause per operator. Γ grows along a path
with the facts each binder and test establishes. 𝓕(e) denotes the strongest set the decided fragment
states for e's value: today's interval and refinement facts, and Presburger once ADR 0049's checker
is built.

```
𝒪(lam(x̄ ∈ S̄; r ∈ U; φ; ψ; e))
    = 𝒪_{Γ₀}(e)  ∪  { Γ₀ ⊢ e ∈ U ∧ ψ[e/r] }            Γ₀ = x̄ ∈ S̄, φ
      (a host def's λ: none; its claims are axioms, checked against the host, ADR 0022)

𝒪_Γ(app(f; ā)),  f a def whose value is that λ (ADR 0051)
    = ⋃ᵢ 𝒪_Γ(aᵢ)  ∪  { Γ ⊢ ā ∈ S̄ ∧ φ[ā/x̄] }            the call's obligation (ADR 0028)
      and the call's value gains r ∈ U ∧ ψ[ā/x̄]          the exact swap (postconditions.md)

𝒪_Γ(index(t; i))      = 𝒪_Γ(t) ∪ 𝒪_Γ(i) ∪ { Γ ⊢ 0 ≤ i < len(t) }     a map: none (a read is an option)
𝒪_Γ(app(table; n, f)) = … ∪ { Γ ⊢ 0 ≤ n ≤ max-len }                    irstep4f: a size is an obligation

𝒪_Γ(let(x = e, b̄; e′)) = 𝒪_Γ(e) ∪ 𝒪_{Γ, x ∈ 𝓕(e), x ≐ e if e is pure}(let(b̄; e′))

𝒪_Γ(case(e; p₁ if φ₁ → e₁, …, pₖ if φₖ → eₖ))
    = 𝒪_Γ(e) ∪ ⋃ⱼ 𝒪_{Γ, ¬m₁, …, ¬mⱼ₋₁, mⱼ}(eⱼ)  ∪  { Γ ⊢ m₁ ∨ … ∨ mₖ }
      mⱼ = "e matches pⱼ" ∧ φⱼ, binding bv(pⱼ);  for pⱼ = x ∈ σ:  mⱼ = e ∈ σ, binding x ≐ e,
      so x ∈ 𝓕(e) ∩ σ in the arm   (Theorem P)
      exhaustiveness is decided by closedness for constructors, by Theorem P for sorts

𝒪_Γ(loop(x̄ = z̄; I; ā))
    = { Γ ⊢ I[z̄/x̄] }                                                    entry
    ∪ ⋃ⱼ, each again(ā′) in armⱼ's tail:  { Γ, I, ¬m₁…¬mⱼ₋₁, mⱼ, path ⊢ I[ā′/x̄] }   preserved
    ∪ the exits' obligations under Γ, I, their tests                     and I holds at each exit
      I absent: the invariant the analysis infers (Houdini, array-facts.md); I ≡ ⊤ when it infers none
      termination: size-change termination on the IR, reported (ir/sct.go); not an obligation today

𝒪_Γ(the(S; e))        = 𝒪_Γ(e) ∪ { Γ ⊢ e ∈ S }
𝒪_Γ(assert(φ; e))     = { Γ ⊢ φ } ∪ 𝒪_{Γ, φ}(e)
𝒪(spec(f; …; e))      = { the recursion is well founded }                size-change on its calls
𝒪(lemma(n; φ; ψ₁ … ψₘ)) = { ψ₁ … ψᵢ₋₁ ⊢ ψᵢ }ᵢ ∪ { ψ₁ … ψₘ ⊢ φ }; then φ is a hypothesis of every later Γ
```

On every other operator, 𝒪 is the union over the arguments, with Γ extended by each binder's 𝓕.

**What is not in 𝒪.** Whether an integer operation fits its target's word is ρ's question, not the
logic's: ADR 0026 and 0033 decide it on the IR, per target. A representation is not a claim.

**Theorem O (soundness of 𝒪), the target the checker must meet.** If every sequent of 𝒪(P) is valid
in the standard model of V, then:
- every application in the reduction of D(ε(P)) is within its domain: an index in its table, a
  precondition at its call;
- every def's value, at arguments in its parameters' sets satisfying φ, lies in U and satisfies ψ.

*Proof sketch.* Induction on the reduction of D(ε(P)), with Γ's facts as the invariant of the paths
reduction follows: the soundness of Hoare logic (Cook 1978), with the loop rule's invariant, and
Theorem P for the arms. A full proof would be part of ADR 0049's checker, built against the reducer's
four rules (state.md §1). It is stated here so the checker has a statement to be tested against: every
planted fault must break one of its two conclusions.

## 8. The modalities

### 8.1 Usage

The usage of a name x in a tree is **the set of counts of x's occurrences over the tree's paths**,
abstracted by its convex hull: an exact interval abstraction in the sense of Cousot and Cousot (1977).
The domain is

```
𝕌 = {∅} ∪ { [m, n] : m ∈ ℕ, n ∈ ℕ ∪ {∞}, m ≤ n }
[a, b] + [c, d] = [a + c, b + d]              sequence: uses add          ∅ + u = u + ∅ = ∅
[a, b] · [c, d] = [ac, bd]                     hand-off: a receiver's usage scales an argument's
u ⊔ v           = the hull of u ∪ v            branches: arms join          ∅ ⊔ u = u
```

(𝕌, +, ·) is a semiring, as quantitative type theory's usages are (McBride 2016; Atkey 2018), and ⊔
is its join for branches. ∅ is the path that never reaches the scope's end. It absorbs +, since
nothing after it runs, and it is the unit of ⊔, since it adds no path. That is 0, the initial object,
in the usage algebra: abandon's type, which errors.md §7.1 says "uses every variable vacuously".

```
𝒰ₓ(x) = [1,1]      𝒰ₓ(y) = 𝒰ₓ(k) = [0,0]
𝒰ₓ(app(f; ā))        = Σᵢ ( 𝒰_{pᵢ}(body_f)  if aᵢ is x itself, handed on bare     f a def or a λ in place:
                            𝒰ₓ(aᵢ)         otherwise: an argument is evaluated )  usage inference (errors.md §7.2)
                     = 𝒰ₓ(f) + Σᵢ 𝒰ₓ(aᵢ)    a host name, or computed: passing it escapes, a use
𝒰ₓ(let(y = e; b))    = 𝒰_y(b) if e is x itself, else 𝒰ₓ(e) + 𝒰ₓ(b)    a bare x handed to y; to `_`, dropped
𝒰ₓ(case(e; ā))       = 𝒰ₓ(e) + ⊔ⱼ (𝒰ₓ(φⱼ) + 𝒰ₓ(bodyⱼ))   additive eliminators; a pattern binding x shadows it
𝒰ₓ(try(p; e; b))     = 𝒰ₓ(e) + (𝒰ₓ(b) ⊔ [0,0])          the failure arm returns the failure, using nothing
𝒰ₓ(expect(p; e; w; b)) = 𝒰ₓ(e) + (𝒰ₓ(b) ⊔ ∅)            the failure arm crashes
𝒰ₓ(app(abandon; w))  = 𝒰ₓ(w) + ∅
𝒰ₓ(again(ā))         = Σᵢ 𝒰ₓ(aᵢ)       if x is bound inside the loop's iteration
                     = ∅                otherwise: the path continues into the next iteration
𝒰ₓ(loop(…))          = 𝒰ₓ(z̄) + the clauses, each guard before its body, the bodies joined
𝒰ₓ(lam(ȳ; e))        = 𝒰ₓ(e)            a use inside a λ counts, applied or not (errors.md §7.4)
```

**Corrected and built in step 2** ([abstract2-2026-10-11](../../gauntlet/results/abstract2-2026-10-11.md),
`core/usage.go`). Three clauses of the first draft were wrong, and the build found each one:
- **a hand-off scales only a bare variable.** The draft multiplied every argument's usage by the
  receiver's. But a compound argument is evaluated, so `(drop (check r))` uses r whatever drop does;
  only x itself, handed on, is used as its parameter is. That is what the reducer computes;
- **a λ's body counts once, applied or not.** The draft scaled it by [0,∞], which would refuse every
  use inside a λ passed to a host. Today's check counts it, and errors.md §7.4 names the gap it leaves
  ("a λ never applied: a use inside it counts"). The tree states the rule as built, and the gap stays
  named;
- **`try`'s and `expect`'s failure arms** were not written. They are now.

**The back-edge clause found a soundness hole in the reducer's check.** `Env.usedOnEveryPath` made
every `again` vacuous, so a relevant value bound inside an iteration and dropped before the jump,
`(loop … else (seq (write i) (again …)))`, was accepted, where `(seq (write n) 1)` is refused. It is
fixed (`again` is vacuous only for a variable bound outside the loop the jump goes round), and one test
had been relying on the hole without knowing it.

**Agreement.** On every definition of the 96 corpus programs that load on Go, 630 parameters, the
tree's usage and the reducer's usage inference give the same answer (140 of them are not used on every
path), and nine planted faults each break agreement or a unit test (TestRelevantUsageAgreesOnTheCorpus,
TestTreeUsageOnTheShapesRelevanceTurnsOn).

**The requirements:**

| mode | requirement on x's scope |
|---|---|
| relevant (by its set) | 𝒰ₓ = ∅ or min 𝒰ₓ ≥ 1: used on every path that ends normally |
| `linear` | **an ordering, not a count** (below) |
| none | anything |

**`linear` is not "exactly once"** (corrected 2026-10-11). The draft required 𝒰ₓ ∈ {∅, [1,1]}, but
ADR 0018's buffer is used by an ordering: reads, `(b i)` and `(len b)`, are free while the buffer is
live; a move, a store into it or handing it on bare, happens at most once on a path; and nothing reads
it after the move, except `len`, which cannot observe a store. Per path that is the regular language
(r | ℓ)* (m ℓ*)? over reads r, lengths ℓ and a move m, and an unused buffer is allowed. It is Wadler's `let!`, read-
only access before the linear use ("Linear types can change the world", 1990), and Clean's observers;
an interval of counts cannot express it, since it forgets order. So usage is two judgements, a count
semiring for relevance and an ordering automaton for buffers, which is what `CheckLinear` already is.

**A tree's linear check reaches only buffers bound by the tree**, a `local` buffer (68 scopes in the
corpus) and a `linear` parameter (2 program signatures): `CheckLinear` is seeded by type, and a loop
variable or a host call's result is a buffer only by its type. Its tests mostly build residual terms
of exactly those shapes. So the tree version is not built: it would have almost nothing to be held
against. It is built when the surface reports linearity at source positions.

### 8.2 Effects

**𝓔 is the core's effect judgement pulled back along D: 𝓔(t) = impure(D(t))**, `Env.pureTerm`
(corrected 2026-10-11). The draft listed clauses one per operator, but the effect bit depends on the
target's purity bits and the definitions, which live in the core where β reads them (ADR 0010).
Re-deriving on trees every shape D builds — a `loop`'s name, a match guard's `=`, a filled table's
`set` — would be a second rule that can disagree with the first, and this project has paid for that
before. One rule, composed with D, is a homomorphism too. Its content is ADR 0010's: an impure term is
let-bound, never substituted, a λ argument's body counts as run, and a λ value is not a call.

## 9. Renderings

### 9.1 Definition

A **rendering** R of A is a set L_R of strings and three maps,

```
parse_R : L_R ⇀ CST_R        print_R : CST_R → L_R        abs_R : CST_R → A
```

where a concrete tree c ∈ CST_R is an abstract tree decorated with layout and **gaps** (comments):
c = (a, g) with g : Pos(a) → Gap*, Pos(a) the positions of a's subtrees. The maps satisfy two laws:

```
(R1)  parse_R(print_R(c)) = c                       print is lossless: layout, gaps and spelling survive
(R2)  abs_R(parse_R(s)) is defined and well formed (§3) for every s ∈ L_R
```

and abs_R(a, g) = a forgets the decoration. **A gap never changes the abstract tree**: abs_R ignores g
by definition. That is ADR 0024's principle ("a comment never carries meaning") as a property of
abs_R, so it needs no grammar rule to protect it. Today it is protected by refusing `#;`, because
today's forms are told apart by parity (fit-research §3.6).

### 9.2 Translation between renderings

For renderings R and R′, the translation is

```
tr_{R→R′} = print_R′ ∘ (id × transport) ∘ parse_R
```

where transport moves each gap to the same abstract position. The positions are the abstract tree's,
so both renderings have them. A gap that R attached between two tokens that R′ does not have
(between a keyword and its argument, say) moves to the nearest enclosing position. **A translator
never loses a comment**, because a gap is data on a position of a tree both sides share. That is the
property CLAUDE.md asks of "a tool that rewrites source", and the reason two rewriters lost comments
before is that they rewrote text, not decorated trees.

### 9.3 Spelling

Each rendering has an injective spelling σ_R : 𝒩 → Identifiers_R per program. For example, M′ maps
`-` to `_`, because `-` is subtraction there. A program whose spelling collides in R
(`best-name` and `best_name` both present) is not printable in R, and the translator refuses it,
naming the pair.

### 9.4 Two renderings

T′ (fitted prefix) and M′ (fitted mathematics), from fit-research §5. Each row is one operator.
print_R is the second or third column, and parse_R its inverse.

| operator | T′ | M′ |
|---|---|---|
| lam(x ∈ S, y; e) | `(fn ((x S) y) e)` | `λ x ∈ S, y : e` |
| app(f; a, b) | `(f a b)` | `f(a, b)` |
| index(t; i) | `(at t i)` | `t[i]` |
| let(x = e, tuple(a, b) = e′; e″) | `(let ((x e) ((tuple a b) e′)) e″)` | `LET x == e, ⟨a, b⟩ == e′ IN e″` |
| case(e; c(x) → a, _ → b) | `(case e ((c x) → a) (_ → b))` | `CASE e OF c(x) → a [] OTHER → b` |
| case arm with guard | `((c x) if φ → a)` | `c(x) ∧ φ → a` |
| loop(i = 0, s = 0; I; _ if φ → t, _ → t′) | `(loop ((i 0) (s 0)) (invariant I) (φ → t) (else → t′))` | `LOOP i == 0, s == 0 INVARIANT I : φ → t [] OTHER → t′` |
| again(a, b) | `(again a b)` | `AGAIN(a, b)` |
| local(b =₁ table(n; f), x := 0; e) | `(local ((b (table n f)) (x := 0)) e)` | `LOCAL b == [i ∈ 0..n-1 ↦ f(i)], x := 0 IN e` |
| assign(x; e) | `(set! x e)` | `x := e` |
| compr(x ∈ S; φ) | `(where (x S) φ)` | `{x ∈ S : φ}` |
| mem(e; S) | `(in e S)` | `e ∈ S` |
| the(S; e) | `(the S e)` | `(e ∈ S)` as an annotation, `e ∷ S` |
| assert(φ; e) | `(assert φ e)` | `ASSERT φ; e` |
| def(f; x ∈ S; φ; r ∈ U; ψ; e) | `(def f ((x S)) U (requires φ) (ensures ψ) e)` | `f(x ∈ S) ∈ U WHERE φ ENSURE ψ == e` |
| mode inout | `(a go.Value inout)` | `a ∈ go.Value INOUT` |
| out(S) | `(out S)` | `OUT S` |
| ∀ x ∈ S. φ | `(forall ((x S)) φ)` | `∀ x ∈ S : φ` |
| set(1, 2) | `(set 1 2)` | `{1, 2}` |

The two columns are two printings of one tree. In T′, assign is `set!` and the buffer's store is
`store`, because `set` is the set former: one spelling per operator, Scheme's convention for a write.
The precondition is `requires`, because `where` is the comprehension. The spellings are illustrative.
Choosing them is syntax-research's decision, and this spec does not depend on it.

## 10. Today's notation, and the map μ

**Today's notation T is not a rendering of A.** Its forms are told apart by arity and parity (ADR
0024), and two of them are homonyms resolved by position: `set` with two arguments writes a cell and
with three stores into a buffer, and a literal in `(table n v)` is a constant function. But T maps
into A by a function μ : T → A, total on the corpus, which the first build step implements:

| today's form | μ | |
|---|---|---|
| `(def f (x…) e)`, `(def c e)` | def(f; λ(x̄). μe), def(c; μe) | ADR 0051 |
| `(sig f ((x S)…) U (where φ) (ensures ψ))` beside its def | the claim of the def's λ: x̄ ∈ S̄, result ∈ U, φ, ψ; kept apart when the name has no λ definition of its arity, or two claims or two definitions | decision 3, ADR 0051 |
| `(sig f … (host k τ …))` in a target | def(f; …; host(k; τ; …)) | |
| `(variant (n T…) (c S…)… (success c) (relevant))` | variant(n; T̄; c(S̄)…; c; relevant) | |
| `(fn (x…) e)`, `(λ (x…) e)` | lam(x̄; μe) | |
| `(f a…)` | app(μf; μā) | whether f is a table is a fact about its value, known after reduction, so μ cannot produce index from this notation; index is built by a rendering that marks data application (corrected 2026-10-11) |
| `(let x e … b)`, `(let (tuple a b) e … b)` | let(x = μe, tuple(a, b) = …; μb) | |
| `(seq a b)` | seq(μa; μb) | derived |
| `(if c a b)`, `(and …)`, `(or …)`, `(not a)`, `(cond …)` | if, ∧, ∨, ¬, cond | derived |
| `(case e (c x…) b …)` | case(μe; c(x̄) → μb, …) | |
| `(try (s x) e b)`, `(expect (s x) e w b)` | try, expect | derived |
| `(match (e…) p… (when c) b … else b)` | loop(x̄ = ē; ⊤; tuple(p̄) if c → b, …, _ → b) | match.md §4 |
| `(loop ((x z)…) c e … else e)`, `(again a…)` | loop(x̄ = z̄; ⊤; _ if c → e, …, _ → e), again(ā) | |
| `(local b (table n f) … x e … body)` | local(b =₁ table(n; f), …, x := e, …; μbody) | ADR 0047's rule: by the initializer's form |
| `(set c v)`, c a cell | assign(c; μv) | the homonym split |
| `(set b i v)`, b a buffer | app(store; b, i, v) | Σ₀'s threaded store |
| `(build n (fn (b) …))`, `(build-map m cap …)` | local(b =₁ table(n; 0); …), local(m =₁ map(cap); …) | core forms, kept in Λ⁺ |
| `(the S e)` | the(⌜S⌝⁻¹; μe) | |
| `(out τ)`, `(spread xs)` | out(S), spread(μxs) | |
| `(int LO HI)`, `(array V)`, `(map K V)`, `(tuple A…)` | the names of Σ₀: the range LO..HI, Array(V), Map(K, V), A × … | |
| `(buffer V)` at a parameter | x ∈ Array(V) linear | decision 2 |
| `(cell T)` at a parameter, `(array (cell T))` variadic | x ∈ T inout, each trailing argument | decision 2 |
| `pure`, `variadic`, `index` | markers of the def | |
| `(module …)`, `(use p as a)`, `(export …)`, `(provides …)` | module, use, export, provides | |

Theorem D3 is the statement that μ followed by ε and D is the identity on meaning: D(ε(μ(t))) ≡_α
read_T(t).

## 11. Laws

Each law below is an equation between trees of A that holds **in the meaning**: both sides have the
same ⟦·⟧. Each follows from D's clauses and a reduction rule of the atom, or is named as tested.

| law | equation | from |
|---|---|---|
| β for the coproduct | case(c(v̄); …, c(x̄) → e, …) = e[v̄/x̄] | sums.md §4, the commuting conversion |
| first match | case(e; p → a, p → b, ā) = case(e; p → a, ā) | T's clauses |
| exhaustive last arm | case(e; ā, p → b) = case(e; ā, _ → b) when the arms ā, p cover e's set | 𝒪's exhaustiveness |
| if's laws | if(true; a; b) = a, if(¬c; a; b) = if(c; b; a) | booleans.md |
| the monad laws | try(s(x); s(v); b) = b[v/x]; try(s(x); e; s(x)) = e; associativity | errors.md §5 |
| loop unfolding | loop(x̄ = z̄; I; ā) = let(x̄ = z̄; ā[again(w̄) ↦ loop(x̄ = w̄; I; ā)]): one iteration is a let, so a loop's binders are let's (ADR 0055) | iteration.md §2's semantics: a least fixed point |
| a cell is a loop variable | local(x := z; loop(ȳ = w̄; ā[assign x])) = loop(ȳ, x = w̄, z; ā′) projected: state is implemented by parameters | local.md §4, `TestACellReducesToTheLoopVariableItMeans` |
| erasure | ε(the(S; e)) = ε(e), ε(assert(φ; e)) = ε(e) | §5 |
| η for products | let(tuple(x̄) = e; b) = b[πᵢ e / xᵢ] when e's tails are tuples | state.md §1 |
| ignore | app(ignore; e) = the unit, e's effects kept | errors.md §7.3 |
| let | let(x = e; x) = e; let(x = e; b) = b[e/x] for e pure and duplicable | the reducer's β (ADR 0010) |

These are the refactorings a program, a rendering's formatter or a translator may apply without
changing the meaning. A tool that applies any other rewrite has to prove it.

## 12. The worked example

The sum of a table, with its claims, from fit-research §5:

```
def( total;
     xs ∈ Array(0..1000);
     ⊤;
     r ∈ Nat;
     ⊤;
     loop( i = 0, s = 0;
           s ≐ sum(xs, 0, i);                                   I, with sum a spec
           _ if i ≥ len(xs) → s,
           _               → again(i + 1, s + index(xs; i)) ) )

spec( sum; a ∈ Array(Int), i ∈ Nat, j ∈ Nat; r ∈ Int;
      if(i ≥ j; 0; index(a; i) + sum(a, i + 1, j)) )
```

**Well formed**:
- the again is a Tail of its loop's last arm, with 2 arguments for 2 state variables (W3);
- `sum` occurs only in the invariant, a claim position (W6);
- the last arm is `_` with no guard (W4).

**Erased**: ε gives def(total; xs; ⊤; r; ⊤; loop(i = 0, s = 0; ⊤; …)), and the spec is removed.

**Desugared** (§6):

```lisp
(def total (fn (xs) (loop (fn (i s) (if (>= i (len xs)) s (again (+ i 1) (+ s (xs i))))) 0 0)))
```

That is what today's reader makes of `(def total (xs) (loop ((i 0) (s 0)) (>= i (len xs)) s else
(again (+ i 1) (+ s (xs i)))))`, as Theorem D3 requires.

**Obligations** (§7), Γ₀ = xs ∈ Array(0..1000):

| sequent | decided by |
|---|---|
| Γ₀ ⊢ 0 ≐ sum(xs, 0, 0) | the spec's definition, one unfolding |
| Γ₀, I, ¬(i ≥ len xs) ⊢ 0 ≤ i < len(xs) | Presburger |
| Γ₀, I, ¬(i ≥ len xs) ⊢ s + xs[i] ≐ sum(xs, 0, i + 1) | **not decided**: induction over sum. Climbed by the lemma sum(a, 0, i + 1) ≐ sum(a, 0, i) + a[i], whose own proof unfolds the spec (settypes §6.3) |
| Γ₀, I, i ≥ len xs ⊢ s ∈ Nat | needs s ≥ 0, which I does not state: the analysis infers it as an invariant (s + xs[i] ≥ 0 since xs[i] ∈ 0..1000, the array property fragment), or the programmer strengthens I to s ≐ sum(xs, 0, i) ∧ s ≥ 0 |

**Usage**: no binder has a usage mode, so 𝒰 asks nothing. **Effects**: pure.

**Rendered**, T′ and M′ are fit-research §5's two printings. Today's T is μ's preimage.

## 13. What is new, and the order to build it

**New in meaning: nothing**, by Theorem E and Theorem D3.

**New in structure:**
- the tail rule and the logic's boundary become sorts (§3);
- a claim becomes one operator with an erasure;
- modes are on binders;
- a comment is a decoration on a position (§9);
- the homonym `set` is split into two operators (§10).

**New constructs, not built:**
- index as its own operator (ADR 0050, decision 6);
- arm guards on case (today only `match` has `when`);
- set patterns `x ∈ S` and mem tests (settypes §6.4, after Theorem P is built);
- spec, lemma, assert and invariant (settypes §8 step 4);
- comprehension (settypes §8 step 3).

**The order, each step program-first and emission byte-identical:**
1. **A as a Go type, μ from today's reader, and D.** The reader builds trees of A, and D produces
   what it produces today. `cmd/check` must report no difference anywhere (Theorem D3), and each
   W-condition gets a planted fault that the parser or the check refuses. The walkers that enforce W3
   are then redundant, and they are deleted only after a planted fault shows that the sort catches what
   they caught.
   - **1a, built** ([abstract1-2026-10-11](../../gauntlet/results/abstract1-2026-10-11.md)): the
     computation's terms. Every `.oro` file and 1,032 test snippets read identically, messages
     included; emission byte-identical; the W3 walkers deleted, nine planted faults caught.
   - **1b, built** ([abstract1b-2026-10-11](../../gauntlet/results/abstract1b-2026-10-11.md)):
     program declarations as trees, a `sig` the claim of its definition's λ (ADR 0051), the
     hand-written redex refused. The corpus and every test snippet read identically; seven planted
     faults caught.
   - **1c, built** ([abstract1c-2026-10-11](../../gauntlet/results/abstract1c-2026-10-11.md)): every
     target declaration (876) passes through its tree, a definition with a host body whose claim
     carries modes (`linear`, `inout`) in place of `(buffer V)` and `(cell T)`; every primitive
     identical; emission byte-identical; eight planted faults caught.
   - **2, built** ([abstract2-2026-10-11](../../gauntlet/results/abstract2-2026-10-11.md)): relevant
     usage on trees, agreeing with the reducer on 630 corpus parameters; it found and fixed a soundness
     hole in the relevance check at `again`; 𝓔 is the core's judgement pulled back along D; `linear`
     corrected to an ordering, its tree version deferred (§8.1).
   - **3a, built** ([presburger-2026-10-11](../../gauntlet/results/presburger-2026-10-11.md)): the Omega
     test, a complete procedure over ℤ, measured beside today's entailment on the corpus. It changes no
     outcome, so ε and 𝒪 on trees wait for claims that carry sets: **next**, types as static values
     and the Boolean algebra of sorts, demanded by `errors.AsType` (settypes §8, reordered).
   - **Deferred to the step that consumes them**: `assign`, which needs the scope above it and has no
     consumer before step 2; a `case` pattern resolved into Pat, which needs Load's declaration
     environment and belongs with the step that moves Load onto trees.
2. **𝒰 and 𝓔 on A**, checked against CheckLinear, relevance and pureTerm on the corpus. A
   disagreement is a bug in one of them, and errors.md §7.4's named gaps become decidable.
3. **ε and 𝒪 on A**, as ADR 0049's checker is built (settypes §8): obligations generated from trees
   with claims, decided by the Presburger core.
4. **A rendering**, once syntax-research decides the notation: print_R and parse_R, laws R1 and R2
   checked by printing and reparsing the corpus, and the translator, which carries every comment.

The alternatives to decisions 2–6 (usage as set formers, `sig` beside `def`, literal singletons by
position, structure by parity, uniform application) and why each was rejected are in ADR 0050's
"Why not".

## 14. Not here

- **Records**, `with` and views (data.md, not built): one more product with field names, a Σ₀ name and
  a pattern.
- **Nested patterns**: compiled to decision trees by the standard algorithms (Wadler 1987; Maranget,
  "Compiling pattern matching to good decision trees", ML Workshop 2008). They need no new operator,
  only a recursive Pat.
- **Buffers in program order**: decided by
  [ADR 0052](../decisions/0052-a-buffer-is-a-state-binder-holding-a-table.md), and built with the
  surface (§2.2's note).
- **Callbacks, tiers 1 and 2** (callbacks.md): a λ argument to a host name, which W5 and 𝒰's
  [0,∞] scaling already describe.
- **Temporal formulas for concurrency** (settypes §10): a further sort of Formula.
- **Σ_T, the target signature**: unchanged, specified by target-files.md.
