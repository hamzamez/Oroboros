# 0043 — Relevance is checked where a binder meets its value, on every path

Date: 2026-10-07
Status: Accepted (hamza: "go ahead with step 6, always mathematical and algebraic"). Realizes
ADR 0040's `(relevant)` marker, and replaces errors.md §7's first plan, "checked on the residual by
occurrence counting at every binder of a host call", which step 4 made unworkable.

## Context

`(relevant)` on a variant denies weakening for its values: a value of `(result T E)` must be used
(errors.md §7). Through step 5 the marker was read and ignored, so `(seq (os.WriteFile p d) …)`
compiled and its error went unseen.

The plan was to count occurrences on the residual, at the binder of each host call whose declared
result is relevant. By step 6 no such binder exists there:
- a fallible host declaration is the raw call and a retraction (step 4), so the residual's host call
  returns the host's product, not the model;
- η for products takes every binding of a sum apart, read or not (gomigrate-2026-10-06), so the
  relevant value is never a binder in the residual;
- the laws of the conditional erase tests whose arms agree. `(seq r B)` and
  `(seq (case r (ok _) 0 (err _) 0) B)` are equal, and relevance must tell them apart: it is a
  property of the term written, not of its equivalence class.

The reducer is untyped, and the type checker sees only the residual.

## Decision

1. **The rule is weakening denied, with additive eliminators.** A relevant value bound to x must be
   used on every path of x's scope. A coproduct's elimination [f, g] takes f and g in one context (its
   universal property), so `(if c a b)` uses x when c does, or a and b both do. `(abandon w)`, of type
   0, uses every variable vacuously, as the crash already absorbs every strict context. `(again …)`
   does too, since every terminating execution leaves a loop by an exit.
2. **It is checked at β**, the one point where a binder and its value are both in hand. The value is
   normalised (impure, ADR 0010) or as written (pure). The body is the λ as written: a literal λ, or a
   definition's body before reduction.
3. **Relevance belongs to the value, through its constructors.** A relevant variant's constructor
   builds its tuple with the binder `#k!`, where other tuples have `#k`. A value is relevant when one
   of its tails is such a tuple, or a tuple containing one. α-equivalence ignores binder names, so the
   mark changes no equation.
4. **Handing a value on is a use exactly when the receiver uses it**: usage inference, a definition's
   parameter read off its body. It terminates because no definition reaches itself (ADR 0014). It is
   needed because reduction under a λ runs a callee's β before the λ meets its argument.
5. **Only the program's binders are checked**: names it wrote, and `seq`'s and a pattern's `_`. The
   compiler's own binders, a `case`'s slots, η's components and a conversion's continuation, bind
   values whose use the program already wrote.
6. **`ignore` is !_A : A → 1**, a language constant, since a relevant calculus cannot define it. Its
   reduction is the terminal object's universal property: on a tuple-valued term it is the eliminator
   with a constant body. **η for products extends to n = 0** (r = () for every r : 1), so
   `(seq (ignore e) B)` emits exactly what `(seq e B)` emitted before.

## Consequences

- No program in the corpus discarded a result. Step 5 had moved every one to `case`, and
  `examples/errors/discard.oro` pins the refusal on Go, JavaScript and Java. `ignore.oro` emits
  byte-for-byte what `discard.oro` emitted before this step.
- Nine planted faults, one per rule, each caught by `core/relevance_test.go`. Two mechanisms built on
  the way, scoped relevant names and η carrying the mark, were caught by nothing. Reduction under
  binders makes them dead, and they were removed.
- The rule names its limits (errors.md §7.4): a relevant value from a pure computation passed
  unnormalised; a relevant payload in a `case` arm; a loop variable; a hand-off to a function computed
  at run time.

## Why not

- **On the residual** (the first plan): the value is gone, η and the conditional's laws erase both
  uses and discards, and legality would depend on how much the optimiser simplifies.
- **A typed pass on the source**: the source is higher order, and the language types only the
  residual (types.md). The β check is the source judgement evaluated where staging makes the types
  concrete. Each binder meets its value in a monomorphic instance, which is what a source type system
  would need polymorphism over usage to express.
- **A mark wrapping the value**, `(#rel M)`, like ADR 0037's obligations: every rule and walker would
  have to commute with it (case-of-case, the n-ary let, η, absorb, the IR's lowering). The binder's
  name already says "this λ is a tuple" to every one of them. A mark on that name changes nothing
  they do.
- **Occurrence anywhere, not on every path** (Zig's and Rust's `must_use` practice): weaker than the
  logic. `(let w (write a) (try (ok u) (write b) (check w)))` loses w's error exactly when the second
  write fails, and only the additive rule refuses it.
- **A binder exemption for `ignore`**, `((fn (#!) (tuple)) e)` with `#!` unchecked: it encodes a
  constant as a λ the rule must special-case. !_A is a constant with its own reduction, as `let` is.
