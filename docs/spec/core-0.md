# Core specification, draft 0

The atom, written down. See [the-atom.md](../the-atom.md) for why this is the atom rather than a
vocabulary.

Status: **draft, unimplemented.** Nothing here has been run. This exists so that the reducer can
be written against a specification rather than discovered by writing it.

---

## 0. What this is, and how it differs from λ-calculus

> **λ-calculus in which the normal form is a parameter.**

Three departures from λ-calculus, each one a place where λ-calculus would have cost an
allocation. They are not incidental; they are the project.

| λ-calculus | Here | Why |
|---|---|---|
| Recursion via the **Y** combinator | **δ — a separate global binding form** | Y allocates. `let` is encodable as `((fn (x) b) e)`; recursion is not encodable for free. Two binding forms where λ needs one. |
| Church numerals | **Literals are primitive terms** | Church numerals are λ-terms, so arithmetic would allocate. |
| Currying | **Multi-argument λ** | Curried application builds intermediate closures unless the compiler uncurries. Cheaper to never build them. |

So this is not λ-calculus, and it should not be described as such. It is λ-calculus plus what is
needed to avoid the heap.

---

## 1. Lexical

### 1.1 Encoding

Source is **UTF-8**. A file that is not well-formed UTF-8 is rejected — not repaired, not
replaced with U+FFFD.

Four restrictions, each closing a known failure:

| Rule | Closes |
|---|---|
| Source must be **NFC-normalized**; non-NFC is rejected | `é` as U+00E9 versus `e`+U+0301 are two byte strings that display identically. Without this, two distinct identifiers look the same. |
| **Bidirectional control characters are rejected** (U+202A–U+202E, U+2066–U+2069) | Trojan Source (CVE-2021-42574): code that displays differently than it parses. |
| Identifiers follow **UAX #31** (`XID_Start` followed by `XID_Continue`) | Uses the Unicode standard for identifiers rather than inventing one. |
| **Case is never semantically significant** | Shen makes capitals mean "variable". That is unimplementable in Arabic, Hebrew, Chinese, Japanese, Korean, Thai — scripts with no case at all. |

That last one is a real design constraint and worth stating as a rule rather than an accident:
**no syntactic distinction in this language may depend on letter case.**

Additionally permitted in identifiers beyond UAX #31: `- + * / < > = ! ? _` — so `dot-product`,
`<`, and `empty?` are single identifiers. The hyphen is preferred over the underscore in
canonical form.

### 1.2 Tokens

```
comment    ::= ";" any* newline
whitespace ::= space | tab | newline | return
delimiter  ::= "(" | ")" | whitespace | ";"

integer    ::= "-"? digit+
float      ::= "-"? digit+ "." digit+ ( ("e"|"E") "-"? digit+ )?
name       ::= idchar+                    ; UAX #31 XID plus - + * / < > = ! ? _
metavar    ::= "?" name                   ; pattern variable, compile-time only
```

Integers and floats are distinct token classes: `1` is an integer, `1.0` is a float. There is no
implicit conversion, because [ADR 0003](../decisions/0003-range-typed-integers.md) makes the
distinction semantic.

**Floats are read as IEEE-754 binary64, exactly**, by the shortest round-tripping rule. Compile
time and runtime must agree bit for bit
([ADR 0009](../decisions/0009-staging-preserves-results.md)).

### 1.3 Why `?x` for metavariables, and bare names for everything else

This language has two levels, so it needs to distinguish two kinds of variable:

- **Runtime variables** are bare names, bound by `fn`. `(fn (x) (add x 1))`.
- **Metavariables** are `?x`, and appear only in rule patterns. `(rule (dot ?a ?b) => ...)`.

No sigil for ordinary variables, following Lisp and ML: the binding form already says what is
bound, so a sigil is redundant. A sigil *is* justified for metavariables because they live at a
different level and a reader — human or model — genuinely needs to see which.

**Staging is deliberately not in the syntax.** MetaML marks stages with `<>` and `~`. Here,
[s2](../derivations/s2-multiplicity-inference.md) found that grade 0 is *observed on the residual*
rather than declared, so marking it in source would be redundant and would let source and reality
disagree.

---

## 2. Grammar

```
program ::= form*

form    ::= "(" "def"  name term ")"      ; a global definition       (δ-rule)
          | "(" "prim" name+ ")"          ; declare names primitive
          | term

term    ::= name                          ; variable or global reference
          | literal                       ; integer or float
          | "(" "fn" "(" name* ")" term ")"
          | "(" term term* ")"            ; application

literal ::= integer | float
```

Four term constructors. `fn` is the canonical spelling of abstraction; the reader also accepts
`λ` (the UTF-8 decision makes it free), and the canonical formatter rewrites it to `fn` so that
there is one printed form.

---

## 3. Reduction

Two rules.

### β — application of an abstraction

```
((fn (x₁ … xₙ) b) a₁ … aₙ)  ⟶  b[x₁ := a₁, … , xₙ := aₙ]
```

Arity must match exactly; a mismatch is an error, not a partial application. Substitution is
capture-avoiding — guaranteed by representation rather than by a check
([s1](../derivations/s1-substructural.md): locally nameless).

**Substitution is by let-binding, not by copying**, when a variable occurs more than once in `b`
and the argument is not a literal or a variable. This is [g4](../derivations/g4-word-count.md)'s
Defect 1: naive substitution duplicates work, and with effects it duplicates the effect.

### δ — unfolding a global definition

```
f  ⟶  t        where (def f t) is in scope
                and f ∉ P
```

where **P is the target's primitive set**.

### Normal form

> A term is in normal form when it contains **no β-redex** and **no name outside P**.

That is the whole parasite model. `P` is the parameter.

---

## 4. Targets

A target is a set of names.

```
(target go   (prim add mul lt index len loop var set break dict-empty dict-update))
(target c    (prim add mul lt index len loop var set break))
(target blas (prim add mul lt index len loop var set break dict-empty dict-update dot))
```

Nothing else distinguishes them at this level. Both directions of
[ADR 0002](../decisions/0002-capability-graph.md) are **which side of `P` a name falls on**:

- `dot ∈ P` on `blas` — reduction halts immediately, emitting a BLAS call. This is "compiling up."
- `dict-empty ∉ P` on `c` — reduction continues into a hash table. This is "lowering."

---

## 5. Worked examples

### 5.1 β, plainly

```lisp
((fn (x) (add x 1)) 4)
```

```
⟶β   (add 4 1)                              ; normal, if add ∈ P
```

### 5.2 δ, and where it stops

```lisp
(def double (fn (x) (mul x 2)))
(def quad   (fn (x) (double (double x))))

(quad 3)
```

```
⟶δ   ((fn (x) (double (double x))) 3)
⟶β   (double (double 3))
⟶δ   ((fn (x) (mul x 2)) (double 3))
⟶β   (mul (double 3) 2)
⟶δ   (mul ((fn (x) (mul x 2)) 3) 2)
⟶β   (mul (mul 3 2) 2)                      ; normal: mul ∈ P, no redex
```

Six steps, and the result contains only primitives. `double` and `quad` have no runtime
existence — they are grade 0, *observed* by their absence from the normal form.

### 5.3 The same term, two normal forms

This is the whole thesis in one example.

```lisp
(def dot (fn (a b) (sum (zip mul a b))))
```

**On `blas`, where `dot ∈ P`:**

```
(dot p q)
⟶       (dot p q)                            ; already normal — dot is primitive
```

Zero steps. Emits a BLAS call.

**On `go`, where `dot ∉ P` but `fold`, `mul`, `add` are:**

```
(dot p q)
⟶δ    ((fn (a b) (sum (zip mul a b))) p q)
⟶β    (sum (zip mul p q))
⟶δβ   (fold add 0.0 (zip mul p q))
⟶      … fusion rules …
⟶      (fold-range 0.0 (len p) (fn (acc i) (add acc (mul (index p i) (index q i)))))
```

Normal, because `fold-range`, `len`, `index`, `add`, `mul` are all in `go`'s `P`.

**Same source. Same rules. Different `P`. Different normal form.** One word of target
declaration separates a BLAS call from a loop.

### 5.4 A residual λ — the escaping closure

```lisp
(def make-scaler (fn (f) (fn (v) (mul v f))))

(make-scaler 3)
```

```
⟶δ   ((fn (f) (fn (v) (mul v f))) 3)
⟶β   (fn (v) (mul v 3))                     ; normal — a λ survives
```

The residual **contains an abstraction**. That is an escaping closure, and
[g6](../derivations/g6-escaping-closures.md) measured its cost: 16 bytes and one indirect call on
Go, identical to hand-written.

So `fn` in the *residual* is a capability, not a core primitive — consistent with CLAUDE.md's
"closures are not a core primitive," because the constraint is about what survives, not about
what the calculus contains.

### 5.5 Where it must not reduce

```lisp
(def twice (fn (f x) (f (f x))))

(twice read-line 0)
```

If `read-line` is effectful, substituting it twice duplicates the effect
([g5](../derivations/g5-bindings.md)). β must let-bind:

```
⟶β   (let ((t read-line)) (t (t 0)))        ; NOT (read-line (read-line 0))
```

`let` is sugar for `((fn (t) …) read-line)`, so this is still β — it is β applied in the
non-duplicating order.

---

## 6. What must be proved

| Property | Statement | Status |
|---|---|---|
| **Confluence** | reduction order does not change the normal form | to prove; layer stratification is the argument |
| **Termination** | reduction reaches a normal form for any well-formed program and any `P` | to prove; the layer DAG is the argument |
| **Stage soundness** | the normal form computes what the unreduced term computes | to prove; this is [ADR 0009](../decisions/0009-staging-preserves-results.md) |
| **Parity** | the emitted normal form matches hand-written target code | **not provable** — measured, per [ADR 0008](../decisions/0008-measurement-over-principle.md) |

Termination needs a condition, and it is not free: **δ on a recursive definition does not
terminate.** `(def loop-forever (fn () (loop-forever)))` unfolds indefinitely. So:

> A recursive definition is **never** δ-reduced. It stays in the residual as a target function.

Which is [g3](../derivations/g3-generics.md)'s finding — recursive functions cannot be rules —
arriving here as the termination side-condition rather than as an observation.

---

## 7. Open questions in this draft

1. **What is the minimum `P`?** Every target must provide *something*. `add`, `mul`, `lt`,
   `index`, `len`, and structured control flow are assumed throughout the derivations and have
   never been written down as a required floor.
2. **Are `let` and `loop` sugar or primitive?** `let` is derivable from `fn`. `loop` is not
   derivable without recursion, and recursion is residual — so `loop` is probably primitive.
3. **Do `def` and `prim` scope?** Draft 0 assumes one flat global namespace. Modules are
   [open decision 5](../design-direction.md).
4. **Integer literal typing.** `1` has no range until it is used. Range inference from context is
   [ADR 0003](../decisions/0003-range-typed-integers.md)'s problem and is not specified here.
5. ~~**Rule syntax.**~~ **Settled on paper** — [q5-do-we-need-rules.md](q5-do-we-need-rules.md).
   δ over `def` covers **all layer lowering and, unexpectedly, fusion** — the latter if vectors
   are represented as a length paired with an index function, which makes foldr/build fusion fall
   out of β alone. It is *stronger* here than in Haskell, where the same technique depends on the
   inliner firing; here reduction to normal form is the definition of compilation.

   **One counterexample survives:** SROA on a loop-carried accumulator. `fold-range` is primitive,
   so `acc` is a bound variable of a surviving abstraction rather than an application of `struct`,
   and no β-redex exists. That transformation acts on the *residual* and dispatches on a type, not
   a name.

   So §1.3's `?x` stays, but **only for residual-to-residual transformation**. The capability
   graph — the project's central mechanism — needs no pattern matching at all.

   Side effect: [g1 §5](../derivations/g1-dot-product.md)'s deforestation rules do not exist if
   fusion is δ+β, so the measure check leaves the machinery list and termination reduces to "δ
   over a DAG, β without self-application."

   **`filter` checked** — [q5b-filter.md](q5b-filter.md). The concern was correct: pull arrays
   genuinely cannot express it. It is handled by a second, dual representation — a collection as
   its own fold — which is still pure δ+β, and hand-reduces to exactly the hand-written filtered
   loop. Pull does `zip`, push does `filter`; pull→push is free and push→pull materialises, at
   the same point hand-written code materialises. **Two representations are more library, not
   more core, so the conclusion holds.**

   Notable: stream fusion would unify the two and would need case-of-case — a shape-directed rule
   load-bearing for the whole collection library. **The elegant unification costs strictly more
   machinery than keeping two representations.**
