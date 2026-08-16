# Core specification, draft 0

The atom, written down. See [the-atom.md](../the-atom.md) for why this is the atom rather than a
vocabulary.

> **Status, 2026-08-15: implemented.** This said *"draft, unimplemented — nothing here has been
> run"* for two months after it stopped being true. Everything below is running, and the parts that
> are **not** are now marked as such rather than left to look settled.
>
> Rewritten rather than appended to, per CLAUDE.md: *a stale design document is worse than no design
> document.*

---

## 0. What this is

> **PCF, reduced to normal form at compile time, with the constant set as a per-target
> parameter.**

An earlier draft of this section claimed three departures from λ-calculus — literals instead of
Church numerals, multi-argument λ instead of currying, and δ instead of **Y**. **All three were
wrong**, and the correction is in [pcf.md](pcf.md):

- λ-calculus **with constants** is standard, and reducing `(add 2 3)` is what Barendregt already
  calls δ-reduction.
- Currying is an **isomorphism**; the uncurried presentation has no semantic content.
- `(def f t)` with `t` mentioning `f` is a recursive equation whose meaning is the **least fixed
  point** — that is `fix`, and **λ + constants + fix is PCF** (Plotkin, 1977).

The stated reason for the third was wrong too. "Y allocates" is a performance claim; the real
obstacle is that **`Y f` has no normal form**, so a calculus whose semantics is *reduce to normal
form* must take `fix` as primitive and must not unfold it. That is the **unfolding strategy**
problem in partial evaluation, and "do not unfold recursive calls" is its standard answer.

> **And the third correction has itself been superseded**, 2026-08-16 by
> [ADR 0014](../decisions/0014-recursion-is-not-in-the-language.md). Declining to unfold `fix` is
> the right *reduction* answer and it is not a *compilation* answer: no backend emits a recursive
> definition, and emitting one would ship a construct whose stack depth differs by orders of
> magnitude across the three targets. A recursive definition is now **rejected** before reduction,
> so `fix` is not in the calculus at all — see [pcf.md §9](pcf.md), which is where that leaves the
> PCF identification.

**Nothing in the mathematics is new.** What is new is architectural: `P` is a *per-target*
parameter, and the resulting normal form is the compilation output. A name that is a constant on
one target and a defined function on another is not a different calculus — it is the same calculus
instantiated twice.

The benefit of saying it this way is not modesty. It means **PCF's results apply directly** rather
than having to be redone.

---

## 1. Lexical

### 1.1 Encoding

Source is **UTF-8**. A file that is not well-formed UTF-8 is rejected — not repaired, not replaced
with U+FFFD.

| Rule | Closes | Status |
|---|---|---|
| Source must be **NFC-normalised** | `é` as U+00E9 versus `e`+U+0301 display identically | ✅ rejected, not repaired — as with invalid UTF-8 |
| **Bidirectional controls rejected** (U+202A–U+202E, U+2066–U+2069) | Trojan Source, CVE-2021-42574 | ✅ |
| Identifiers follow **UAX #31** | uses the standard rather than inventing one | ✅ approximated with stdlib categories |
| **Case never assigns meaning** | Shen makes capitals mean "variable"; Go makes them mean "exported". Both are unimplementable in Arabic, Hebrew, Chinese, Japanese, Korean, Thai — scripts with no case at all | ✅ |

That last one needs stating precisely, because the short version is ambiguous:

> **No syntactic category or visibility rule may be determined by letter case.**
> Identifiers are still **case-sensitive for identity**: `Xr`, `xr` and `XR` are three different
> names.

The distinction matters. Case-*insensitive* identifiers would be a different and worse property:
case folding is **locale-dependent** — in Turkish, `I` lowercases to `ı` and not to `i` — so
case-insensitive matching cannot be specified portably at all. It would fail the three-question
test on the same grounds as `length` ([strings.md §2](strings.md)).

Permitted in identifiers beyond UAX #31: `- + * / < > = ! ? _`. So `dot-product`, `<`, `empty?`
and the module path `go/strings` are each a single identifier.

**`.` is not among them.** It is the qualifier separator ([modules.md §3](modules.md)):
`words.split-words` is a name whose segments are separated by `.`, and a name may not begin, end,
or double it. `/` stays an ordinary identifier character, which is what makes a module path one
token.

### 1.2 Tokens

```
comment    ::= ";" any* newline
whitespace ::= space | tab | newline | return
delimiter  ::= "(" | ")" | whitespace | ";"

integer    ::= "-"? digit+
float      ::= "-"? digit+ "." digit+ ( ("e"|"E") "-"? digit+ )?
string     ::= '"' ... '"'
name       ::= segment ( "." segment )*
segment    ::= idchar+
```

Integers and floats are distinct token classes: `1` is an integer, `1.0` is a float. There is no
implicit conversion — [arithmetic.md](arithmetic.md) makes the distinction semantic, and the one
conversion that exists, `num/f64.of-int`, is a primitive you call.

**Floats are read as IEEE-754 binary64, exactly**, by the shortest round-tripping rule. Compile
time and runtime must agree bit for bit
([ADR 0009](../decisions/0009-staging-preserves-results.md)).

**Strings** were added for target templates and specified afterwards, which was the wrong order —
[strings.md](strings.md) is the correction.

### 1.3 ~~Why `?x` for metavariables~~ — reserved, never implemented, never needed

This section specified `?x` as a distinct token class for pattern variables in rewrite rules, on
the reasoning that a two-level language needs to distinguish two kinds of variable.

**There are no metavariables.** `?` is an ordinary identifier character, so `?y` reads as a plain
name today and always did. §7.5 had already narrowed their purpose to "residual-to-residual
transformation only", and no such transformation was ever written: δ+β turned out to cover layer
lowering *and* fusion, which is the finding [q5](q5-do-we-need-rules.md) records.

Kept as a section rather than deleted, because a syntax reserved for a mechanism that turned out
unnecessary is exactly the kind of thing this project should notice about itself.

**Staging is still deliberately not in the syntax.** MetaML marks stages with `<>` and `~`; here
[s2](../derivations/s2-multiplicity-inference.md) found that grade 0 is *observed on the residual*
rather than declared, so marking it in source would let source and reality disagree.

---

## 2. Grammar

```
program ::= form*

form    ::= "(" "def"    name term ")"           ; a global definition        (δ-rule)
          | "(" "module" path ")"                ; open a module scope
          | "(" "use"    path ( "as" name )? ")" ; import
          | "(" "export" name+ ")"
          | "(" "sig"    name "(" param* ")" name ( "(" "where" term ")" )? ")"
          | term

param   ::= name | "(" name name ")"             ; a type, or a named type

term    ::= name                                 ; variable or global reference
          | literal
          | "(" "fn" "(" name* ")" term ")"      ; `λ` is accepted and normalises to `fn`
          | "(" "let" term term ")"              ; sugar:  (let e k) ⟶ (k e)
          | "(" "seq" term term+ ")"             ; sugar:  (seq a b) ⟶ ((fn (_) b) a)
          | "(" term term* ")"                   ; application

literal ::= integer | float | string
```

**Six term kinds** — name, integer, float, string, abstraction, application. (The *representation*
has a seventh, `KBound`, which no program can write; see [state.md §1](state.md).) `let` and `seq` are
erased by the reader and never reach the reducer.

`()` is **not a term**, and **is** a legal parameter list: `(fn () b)` is the nullary abstraction a
program's entry point has to be ([build.md §2](build.md)).

### Scope

**Name resolution is a pass, and it is not the covering check.** The two look at the same thing —
free names — and answer different questions:

| | question | kind of answer |
|---|---|---|
| **scope** | is this name bound *anywhere*? | a **program** error |
| **covering** | can *this target* provide it? | ADR 0001's portability property |

Conflating them left three holes, all of which existed until 2026-08-15: `oro` printed a warning
and exited 0, `gen` never checked at all, and a name appearing only in a definition the program
never reaches was **never looked at**, so a typo in unused code was invisible. The last is the
classic reason name resolution walks *everything* rather than only what reduction happens to visit.

**A program may not declare primitives.** `(prim …)` and `(target …)` in a program are errors;
which names are primitive comes from a target file ([target-files.md](target-files.md)), which is
ADR 0002's parameter and is now literally a separate file.

---

## 3. Reduction

Two rules.

### β — application of an abstraction

```
((fn (x₁ … xₙ) b) a₁ … aₙ)  ⟶  b[x₁ := a₁, … , xₙ := aₙ]
```

Arity must match exactly; a mismatch is an error, not a partial application.

**The representation is locally nameless**, as [s1](../derivations/s1-substructural.md) specified:
a free variable is a name, a bound variable is an index. β replaces indices with terms, so a
substituted term **cannot** be captured — there is no check, because the collision cannot be
written down. `Params` survives as a naming hint so emitted code keeps saying `acc` and `i`; a hint
cannot change meaning.

Reduction never *opens* an abstraction. Opening and re-closing with a colliding hint would
re-introduce exactly the hazard, and not needing names is what makes never opening possible.

β carries **two side conditions**, and both were found by measurement rather than derived:

**Call-by-need.** An argument used more than once is normalised and let-bound unless it is
*duplicable* — a literal, a name, or **an abstraction**. The λ case is load-bearing: a duplicated λ
must be substituted or fusion dies, because the two copies reduce to different small things and
that is the entire mechanism ([callbyneed](../../gauntlet/results/callbyneed-2026-08-14.md)).

**Effects.** An **impure** argument is *never* substituted — it is let-bound at the application
site, in argument order, whatever its occurrence count. The three clauses deny contraction,
weakening and exchange, which is [ADR 0010](../decisions/0010-effects-as-structural-rules.md):
purity is the licence to use the structural rules.

### δ — unfolding a global definition

```
f  ⟶  t        where (def f t) is in scope, f ∉ P, and f is not recursive
```

where **P is the target's primitive set**.

δ needs no side condition of its own, given one restriction: **a definition's body must be a
value.** Unfolding copies the body to every occurrence, which is contraction; a λ is a value
whatever its body does, so this rejects only a name bound to a *computation*.

### Normal form

> A term is in normal form when it contains **no β-redex** and **no name outside P**.

That is the whole parasite model. `P` is the parameter.

---

## 4. Targets

A target is a set of names, and the file that declares them is specified in
[target-files.md](target-files.md). Both directions of
[ADR 0002](../decisions/0002-capability-graph.md) are **which side of `P` a name falls on**:

- `num/vec.dot ∈ P` on `blas` — reduction halts immediately, emitting a BLAS call. *Compiling up.*
- `dict-empty ∉ P` on a target without one — reduction continues into a hash table. *Lowering.*

A name that is in **both** `P` and the definitions is [modules.md §6](modules.md)'s conditional:
δ is inhibited and the native wins. That is the whole of "different implementations per target",
and it needs no syntax.

---

## 5. Worked examples

Each is a test in `core/reduce_test.go`.

### 5.1 β, plainly

```lisp
((fn (x) (add x 1)) 4)      ⟶β   (add 4 1)
```

### 5.2 δ, and where it stops

```lisp
(def double (fn (x) (mul x 2)))
(def quad   (fn (x) (double (double x))))
(quad 3)                    ⟶*   (mul (mul 3 2) 2)
```

Six steps, and the result contains only primitives. `double` and `quad` have no runtime existence
— they are grade 0, *observed* by their absence from the normal form.

### 5.3 The same term, two normal forms

The whole thesis in one example, and now spread across two **files**: the library is
`lib/num/vec.oro`, the program is `examples/dot.oro`.

```
blas:  (fn (p q) (num/vec.dot p q))
go:    (fn (p q) (fold-range 0.0 (alen p)
                   (fn (acc i) (num/f64.add acc (num/f64.mul (aindex p i) (aindex q i))))))
```

**Same source. Same rules. Different `P`. Different normal form.**

### 5.4 A residual λ — the escaping closure

```lisp
(def make-scaler (fn (f) (fn (v) (mul v f))))
(make-scaler 3)             ⟶*   (fn (v) (mul v 3))
```

The residual **contains an abstraction**. That is an escaping closure;
[g6](../derivations/g6-escaping-closures.md) measured its cost at 16 bytes and one indirect call on
Go. **All three backends refuse to emit one**, so `fn` in a residual is a capability that is
declared but not implemented — consistent with "closures are not a core primitive", because the
constraint is about what survives rather than about what the calculus contains.

### 5.5 Where it must not reduce

```lisp
(def twice (fn (f x) (f (f x))))
(twice read-line 0)
```

If `read-line` is effectful, substituting it duplicates the effect. β let-binds instead — and note
this is stronger than the original draft said: the binding happens **whatever the occurrence
count**, including zero, because dropping an effect is as wrong as duplicating one. That is what
makes `seq` work at all ([effects.md §5](effects.md)).

---

## 6. What must be proved

| Property | Statement | Status |
|---|---|---|
| **Confluence** | reduction order does not change the normal form | **unproved**, and untested — normal order is deterministic here, so the implementation cannot observe non-confluence |
| **Termination** | reduction reaches a normal form for any well-formed program and any `P` | **unproved**; guarded by the recursion side condition plus a fuel limit, which is a mechanism rather than a proof |
| **Stage soundness** | the normal form computes what the unreduced term computes | **unproved**; still untestable, because no primitive is ever evaluated |
| **Type preservation** | a well-typed source term reduces to a well-typed residual | **unproved**, and now *statable*: the checker exists ([types.md](types.md)) and this is the multi-stage theorem [types-direction §3.6](../types-direction.md) names |
| **Parity** | the emitted normal form matches hand-written target code | **not provable** — measured, per [ADR 0008](../decisions/0008-measurement-over-principle.md). **One program currently fails**: [ADR 0013](../decisions/0013-accept-the-allocation-price.md) |

Termination needs a condition and it is not free: **δ on a recursive definition does not
terminate.** So:

> A recursive definition is **never** δ-reduced. ~~It stays in the residual as a target function.~~

The first sentence stands and is what δ does. The second was written as a prediction, never run,
and is **false**: no backend emits a recursive definition, so nothing downstream could consume the
survivor this sentence promised. `Residual` was built around it and therefore reported success on
a term the emitter could not compile.

[ADR 0014](../decisions/0014-recursion-is-not-in-the-language.md) settles it — a recursive
definition is rejected before reduction — so the guard is no longer what makes termination
plausible. What remains is self-application: `Ω` and `Y` still have no normal form and are still
caught only by fuel. That is the honest state of this row, and
[pcf.md §9](pcf.md) names what would close it.

The first sentence is [g3](../derivations/g3-generics.md)'s "recursive functions cannot be rules",
arriving as the termination side condition rather than as an observation.

---

## 7. Open questions in this draft

1. ~~**What is the minimum `P`?**~~ **Answered by enumeration rather than by derivation** —
   [primitives.md](primitives.md) classifies every primitive, and [target-files.md](target-files.md)
   says a target may provide any subset. There is no required floor: what a target must provide is
   whatever the programs built for it reach, computed by the residual check.
2. ~~**Are `let` and `loop` sugar or primitive?**~~ **Both, and the split is the answer.** `let` is
   sugar in *source* and a structural primitive in the *residual*, which is what lets the compiler
   re-introduce sharing without the programmer's `let` killing fusion. `loop` is structural, and
   [arithmetic.md §2](arithmetic.md) explains why the structural set is what it is.
3. ~~**Do `def` and `prim` scope?**~~ **Settled** — [modules.md](modules.md),
   [ADR 0011](../decisions/0011-modules-add-nothing-to-the-reducer.md). Resolution runs before
   reduction and the reducer never learns modules existed.
4. **Integer literal typing.** Partly settled: [ADR 0012](../decisions/0012-portable-integer-range.md)
   fixes the portable range at ±(2⁵³−1). **Still open**: nothing checks it, and doing so needs
   range *inference* over every integer expression rather than the declared obligations
   [refinements.md](refinements.md) discharges today. It is the second of the two holes shaped like
   a refinement, and the one still open.
5. ~~**Rule syntax.**~~ **Settled, and then the machinery was never needed** —
   [q5](q5-do-we-need-rules.md). δ over `def` covers all layer lowering *and* fusion, the latter
   because a vector is a length paired with an index function, which makes foldr/build fusion fall
   out of β alone. Stronger here than in Haskell, where the same technique depends on the inliner
   firing; here reduction to normal form **is** compilation.

   `filter` was the surviving concern and it was real: pull arrays cannot express it. It is handled
   by a second, dual representation — a collection as its own fold — still pure δ+β. **Two
   representations are more library, not more core.** Stream fusion would unify them and would need
   case-of-case, a shape-directed rule load-bearing for the whole collection library: *the elegant
   unification costs strictly more machinery than keeping two representations.*

   The SROA counterexample stands and is unaddressed: `fold-range` is primitive, so a loop-carried
   accumulator is a bound variable of a surviving abstraction and no β-redex exists. That
   transformation acts on the residual and dispatches on a **type**, which is now a thing this
   project has.
