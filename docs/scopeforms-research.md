# Research: a scope form, `keyword options { block }`, and what a macro would be

2026-11-19. hamza, on P6's epoch (GlobalLock … GlobalUnlock, views-finish-research §2): "This
sounds like calling those functions at the beginning then at the end, if we add a feature to the
language to extend the syntax similar to how lisp uses macros, and we extend the form `keyword
options { block }` research derive do the math and give me your opinion."

**Short answer.** There are two questions, and only one of them is syntax.
1. **The meaning**: a view that the host lends at one call and a later, named call ends. It is
   borrow-research's epoch, Theorem L, with its ending event named on the declarations. It decides
   soundness whatever the syntax.
2. **The syntax**: `lock(h) { p -> … }`, a user-defined scope form.
   - A Lisp macro, a procedure from syntax to syntax, is not what the algebra admits (§2).
   - What it admits is a **derived operator**: a definition whose last parameter is a λ, inlined by
     staging, which the language already has. Add one piece of surface, the trailing block,
     `f(a…) { x̄ -> b }` read as `f(a…, fn(x̄) { b })`.
   - The same construct is callbacks tier 1 (`sort.Slice(xs) { i, j -> … }`), so one decision
     serves both.

**My opinion** (§5): build the meaning on the declarations, and the trailing block as the
user-defined form. No macros.

## 1. What the construct is: a bracket over a view

GlobalLock(h) gives a pointer to the global object's bytes, valid until GlobalUnlock(h). In views
(ADR 0084), with three impure events in program order (ADR 0010):

    {emp}        p = GlobalLock(h)      {p ↦ v}            the host lends a view of the bytes
    {p ↦ v}      body(p)                {p ↦ v ∗ R}        the program reads it; R is the result
    {p ↦ v}      GlobalUnlock(h)        {emp}              the host takes the view back

This is the **bracket**:
- bracket(acq, rel, k) = let p = acq in let r = k(p) in (rel(p); r);
- Haskell's `bracket`, and C++'s RAII scope;
- in linear logic, acq : 1 ⊸ P, rel : P ⊸ 1, k : P ⊸ P ⊗ R, composed.

**The law that makes it sound** is borrow-research's Theorem L with a named end. The epoch of p is
the interval (e_acq, e_end), where e_end is the first event that ends the view. If every read of
p's alias closure A(p) lies inside the epoch, the program behaves as if it read a copy taken at
e_acq. Theorem L took e_end as the next impure event, for Go's `ReadSlice`, whose buffer the next
call may reuse. Here **the host names the end**: GlobalUnlock(h), or GlobalFree(h) or
GlobalReAlloc(h), on the same h. Other calls inside the epoch leave the view alone, unless they are
one of those on h.

So the rule is a property of the **declarations**, not of any syntax:
- GlobalLock's result is lent until an ending call on its argument;
- GlobalUnlock, GlobalFree and GlobalReAlloc are its ending calls;
- a read of A(p) after an ending call on h is refused (use after the epoch, CWE-416);
- a path that never reaches one is refused (a leak, since a view is linear: pointers-research §15);
- a second ending call is refused (a double unlock).

Each is decided on the program's order of impure events, as R0–R4 are on a scope.

## 2. What a macro is, in the algebra

ADR 0050: the language's abstract syntax is the initial algebra of a binding signature Σ, and its
meanings are homomorphisms out of it (erasure ε, desugaring D, obligations 𝒪, usage 𝒰).
theories.md §7: *extend the rules, never the fragments*. A user syntax extension adds an operator:
Σ′ = Σ + {lock}. For every meaning to stay a homomorphism, lock's meaning must be given
**compositionally**, as a term over Σ in its arguments. With binders, that is a second-order term
with metavariables:

    lock(h; p.B)  :=  { p = GlobalLock(h); r = B; ignore(GlobalUnlock(h)); r }

That is a **derived operator** of a second-order algebraic theory (Aczel 1978's binding
signatures; Fiore and Hur 2010, second-order equational logic). Then:
- **Conservative**: every Σ′-term has exactly one Σ-term, by the unique homomorphism that sends
  lock to its definition. So no program gains a meaning it did not have. Felleisen (1991, "On the
  expressive power of programming languages") calls such a construct *macro-expressible*: it adds
  no expressive power.
- **Hygienic**: the metavariable B is substituted capture-avoidingly, which the reducer's
  named-apart binders already guarantee (ADR 0036).
- **Typed after expansion**: the checker and the decision run on the residual, as for every
  definition.

**A Lisp macro is more than that.** It is a procedure from syntax trees to syntax trees, run
before the meaning. It may inspect its argument's shape, count its nodes, or splice in syntax its
caller never wrote. None of these is a homomorphism: a macro that branches on whether its argument
is a literal gives two meanings to two terms the algebra calls equal. So:
- ADR 0050's fit fails (fit-research: misfits are countable, and a procedural macro is an
  uncountable source of them);
- every meaning (ε, D, 𝒪, 𝒰) would have to run the user's procedure first;
- an error is reported in expanded code the program never wrote.

binding-surface.md §6 decided it once already: "no user macros: the reader is a closed macro
expander."

**And the derived operator is already in the language.** A definition with a λ parameter, inlined
by β at staging, is exactly lock's definition. The static level is unrestricted and higher-order,
and erased by staging (closures-direction.md). The reducer is the expander, and it is hygienic.

    fn locked(h ∈ winbase.HGLOBAL, k ∈ fn(p ∈ ptr) (R)) (R) {
        p = winbase.GlobalLock(h)
        r = k(p)
        ignore(winbase.GlobalUnlock(h))
        r
    }

    s = locked(h, fn(p) { winbase.text_of_c(p) })

What is missing is only the notation for the λ, the block after the call.

## 3. The surface: a trailing block

**The form**: `f(a₁, …, aₙ) { x̄ -> b }` is `f(a₁, …, aₙ, fn(x̄) { b })`, and `f(a…) { b }` is
`f(a…, fn() { b })`. It is D's, a reader rule like `xs...` (G2), and it adds no core term.

hamza's `keyword options { block }` is this form: the keyword is a definition's name, the options
its arguments, the block its last argument, a λ binding x̄.

    s = locked(h) { p -> winbase.text_of_c(p) }
    sort.Slice(xs) { i, j -> xs[i] < xs[j] }
    filepath.WalkDir(root) { path, d, err -> … }

Existing forms read the same way: `round(S) { … }` (numbers.md §3) is a scope with a head and
options, and `var` is a scope whose region is the block.

**What makes it a scope and not just a closure** is that the λ runs **once, now, and never
escapes**. That is the premise of the fourth tail form (a host call's continuation, ADR 0027), of
ADR 0139's join point, and of callbacks tier 1 (callbacks-tier1-research: "a callback applied only
during the call … a region of the call"; Theorem D). Its marker is a usage in 𝕌 and an order: the
λ's usage is 1 (or 0..ω where the host loops, for sort's comparator), and its parameter's view is
lent for that application. So the trailing block is **tier 1's surface**. The construct is one
whether the definition is the program's (`locked`) or the host's (`sort.Slice`).

## 4. The options

| | soundness of the epoch | syntax | cost |
|---|---|---|---|
| **A. The ending call on the declarations, with no new form** | §1's rule | `p = GlobalLock(h); …; GlobalUnlock(h)` as C writes it | the lent-until mode on GlobalLock's result, its ending calls marked on their declarations, and §1's four refusals |
| **B. A, and the trailing block** | A's | `locked(h) { p -> … }` by a definition, or by the program's own | A, a reader rule, and tier 1's usage marker on a λ parameter |
| **C. A built-in `lock` keyword** | the keyword's own rule | `lock h { p -> … }` | a keyword per protocol, against theories.md §7 |
| **D. Lisp macros** | none: a macro is syntax | any | ADR 0050's fit, every meaning running user code, errors in expanded code |

A is needed under every option, since a program may call GlobalLock directly, as C does. B adds the
notation without adding a meaning. C and D add meanings where the algebra says to add rules.

## 5. My opinion

**B: the meaning on the declarations, and the trailing block as the user-defined scope form.**
1. **The soundness is not in the syntax.** A block form that calls GlobalUnlock at its end does
   not stop a program calling GlobalLock itself, or keeping p past the block. Only the
   declarations' protocol does that, so A comes first in any case.
2. **The user-defined form is a definition**, a derived operator, and staging is its hygienic
   expander. Felleisen's criterion says it adds no power, and ADR 0050 says that is exactly what a
   notation may add.
3. **One construct, three uses**:
   - P6's epoch;
   - callbacks tier 1, which hamza left to discuss (sort.Slice, filepath.WalkDir, strings.IndexFunc);
   - every bracket a program writes: a file opened and closed, a dialog shown and ended, a DC got
     and released.
4. **Not Lisp macros**: a macro is a procedure on syntax, which no meaning can be a homomorphism
   over, and the language already decided against them (binding-surface §6).

**Decisions for hamza:**
1. A: a view lent until a named call, on GlobalLock's result, with its ending calls marked.
2. B's surface: `f(a…) { x̄ -> b }`. This also decides tier 1's spelling at a call.
3. The program: the clipboard reader made up for it, then TinyRetroPad's own brackets
   (BeginPaint … EndPaint, GetDC … ReleaseDC).
