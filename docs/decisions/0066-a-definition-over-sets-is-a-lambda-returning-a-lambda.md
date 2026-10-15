# 0066 — A definition over sets is a λ returning a λ; the arrow is a λ's header; a host template is quoted code with named splices

Date: 2026-10-15
Status: Accepted (hamza, 2026-10-15, on surface-design.md §7.7–7.10: "I agree with all 11"). Amends ADR
0063's spelling (its semantics unchanged) and abstract.md §2.6 (a host body may sit under static λs).
Not built: the surface is built when it is chosen.

## Context

surface-design installment 7 designed static parameters, the arrow and application, tested on `top`
(the N largest files under a directory, ordered by `slices.SortStableFunc` with a comparator by a key),
written by hand in D and J because callbacks tier 1 is not built. `gauntlet/surface/apply.go` counted the
corpus: 75 λs are continuations, a synonym of the tuple binding; 41 are passed to the program's own
higher-order definitions; none to a host. Go's API (`generics.go`, `callbacks.go`): 88 generic functions
and methods, 150 taking a callback, and 50 of its 53 approximation bounds `~[]E` or `~map[K]V`.
`templates.go` counted the 913 host templates: five hole syntaxes, all positional, 387 templates filling
two or more holes by order.

## Decision

1. **A definition over sets is a λ that returns a λ, written nested**: `fn AsType(E ⊆ error) { fn(err ∈
   error) (option(E)) { … } }`. The outer λ binds sets and is applied at compile time; the inner is the
   function that runs. No parameter groups. **The binder's relation marks the level**: `x ∈ S` ranges
   over values, `E ⊆ S` over sets (E ∈ 𝒫(S), ADR 0063's `(E (subsets S))`); Go's `[E any]` is `E ⊆ any`.
   Instantiation is β on the outer λ (ADR 0063, unchanged).
2. **A static argument is inferred** when a λ whose binders are all ⊆-binders is applied to arguments
   that are not sets and its body is a λ literal whose parameter sets mention them: the sets are found by
   first-order matching against the arguments' sorts (`slices.SortStableFunc(idx, cmp)`, E = Int).
   Otherwise, and where a binder occurs only in the result, the call writes the set. What is inferred is
   a sort, never a set known only at run time. The written form reads to the same tree.
3. **Go's `~[]E` and `~map[K]V` bounds collapse** into `Array(E)` and `Map(K, V)`: the language has one
   table set per element set, so the bound's parameter is determined, a definitional extension.
4. **The arrow is a λ's header without its body**: J `cmp ∈ fn(a ∈ S, b ∈ S) (Int)`, D
   `cmp ∈ λ(a ∈ S, b ∈ S) (Int)`. Its domain may name an earlier parameter, the telescope of ADR 0062:
   `fn(a ∈ elems(x), b ∈ elems(x)) (Int)`. Markers follow it: `pure`; tier 1 (called during the call,
   not retained) is the default; a captured buffer is refused (callbacks.md §4). A host's law on its
   callback is a `where`, higher-order in the declaration and first-order at each call after β.
5. **A λ at the call site takes its claim from the parameter** (checking mode), and closes only over
   frozen values.
6. **No receiver form**: `.` stays qualification; `de.Name(ent)`, the companion reached through its type.
   A pipe waits for a program.
7. **An inout binder's value on exit is primed in `ensure`**, `x'`, TLA+'s action notation.
8. **One binding form for a call's results**; the continuation λ `((f a) (fn (q r) …))` goes.
9. **A host template is quoted host text whose holes are named splices, `${x}`**: a value parameter by
   its argument's code, a ⊆-binder by its host spelling, the result binder by its destination; a lone
   `$` is text, and a literal `${` is `$${`. Replaces `%s`, `%1…%n`, `%r`, `%tᵢ` and `%%`; x86's register
   views are suffixes, `${r:b}`. The host body stays one line, `host KIND "template" CLAUSES`.
   **`comma_ok`** stays a host clause: Go's pair (v, ok) read as option(E) by the retraction
   r(v, true) = 'some(v), r(v, false) = 'none.
10. **A host body is a static value of the target's code monoid** Code_T = Σ* × 𝒫(Requirement), text
    concatenated and requirements (imports, link libraries, helpers emitted once) united, rendered into
    the file once. **A template literal denotes a concatenation in Code_T**; `${x}` injects a binder,
    a ⊆-binder as its spelling in that position together with its imports. A host body may sit only under
    ⊆-λs, so it closes over no run-time value.
11. **An explicit `++` on code values is legitimate at the static level and not admitted until a
    declaration needs it**: every one of the 913 templates is one literal.

## Why not

- **Parameter groups, `AsType(E ⊆ error)(err ∈ error)`** (installment 7's first Finding 1). The same tree
  in the equational style, but a second header shape; the nested form adds none, shows the staging
  boundary as a block boundary, and is how today's notation already writes a program's static λ.
- **A static argument as a set known at run time** (0..n−1 for idx's elements). A static argument is
  static (Law S); the run-time fact flows through the callback's dependent domain instead.
- **The arrow as a set former of the type algebra.** ADR 0053's reasons stand: no ∪ ∩ ∖ on claims.
- **A receiver form `ent.Name()`.** It resolves `Name` by ent's set, known only after checking, while
  resolution comes first (ADR 0011); and `.` would mean qualification or a method by the sort of its
  left side.
- **Dafny's `old(x)`.** It changes what an unmarked name means between the parameters and the `ensure`;
  the prime marks only the new value, and D reads TLA+'s actions so.
- **`++` with a set spliced as a string** (hamza's first template). A set's spelling depends on its
  position in host code (Java `long` and `Long`, Go `*T` and `T{…}`, JavaScript none) and brings its
  imports, which a string cannot carry; text concatenation is sound only for atoms, which the IR
  guarantees and a template author should not have to.
- **A block for the host text** (hamza's second). It puts host text in a block of the language's code,
  and turns 913 one-line declarations into nested blocks.
- **A bare brace as the antiquote.** 42 templates contain `{`; none contains `${`.

## Consequences

- Today's `(sig (AsType (E (subsets error))) …)` and `(def is-a (E) (fn (e) …))` translate to the nested
  form; `%s` and its kin translate to the parameters' names, mechanically, emission unchanged.
- Callbacks tier 1, when built, has its declaration form: the arrow with a dependent domain and markers.
- The printers' process-wide import set (`emit.Imports`) is the second component of Code_T written as an
  effect; returning it with the text is named as an implementation step, not built.
