# 0053 — The arrow is a claim on a host parameter, not a set former

Date: 2026-10-11
Status: Accepted (hamza, 2026-10-11: "go ahead, I agree to the recommendations. document the
reasoning behind it"), on [arrow-research.md](../arrow-research.md). Records and narrows
surface-design.md §1.9's decision 1 (*"function set formers: yes"*). Amends
[spec/abstract.md](../spec/abstract.md) §2.2, §2.6, W7 and W9, under
[ADR 0050](0050-the-abstract-syntax-fits-the-semantics.md) and
[ADR 0051](0051-a-definition-is-a-named-lambda.md). **Not built**: it is built with callbacks tier 1
([callbacks.md](../spec/callbacks.md)).

## Context

A host callback parameter, `sort.Slice(x any, less func(i, j int) bool)`, must say what shape of
operator it takes. surface-design §1.8 derived that this is the arrow's one job, and hamza decided
*"function set formers: yes"*. No ADR recorded it, and abstract.md has no arrow.

V holds no functions (settypes §1.2), so a function space Π(x̄ ∈ P). R = {f ∈ Op | ∀x̄ ∈ P.
f(x̄) ∈ R(x̄)} is a set of operators, outside the Boolean algebra 𝒫(V) of types. Measured on Go's API
manifest (`go run gauntlet/stdlib/callbacks.go`): 150 of 5,536 functions and methods take a function
(2.7%), 50 return one, 9 take a callback that itself takes one (all iterator consumers), and Go has 18
named function types.

## Decision

1. **A function-space claim is needed at exactly one place: a host's callback parameter.** The host's
   body is opaque, so nothing can be inlined or derived. A λ's claim and a definition's meaning
   already state their membership (Lemma 1), and a definition's static parameter is discharged by
   inlining (Lemma 2).
2. **The arrow is a claim used as a set, not a set former.** A ranging binder's set may be a claim, a
   λ's binders with their sets, its result binder, φ and ψ, without a body (ADR 0051's sort). Its
   meaning is Π(x̄ ∈ {x̄ ∈ S̄ : φ}). {r ∈ U : ψ}, intersected with what its markers require.
3. **Where:** on a host definition's parameter, and as the value of a static definition that names
   it, for the host's named function types. Not on a definition's own parameters until a program asks
   for modular checking.
4. **Fitting an argument** f with claim (P_f, R_f) to a parameter with claim (P, R) is
   P ⊆ P_f ∧ ∀x̄ ∈ P. R_f(x̄) ⊆ R(x̄): the domain contravariant, the result covariant, both
   inclusions between sets of values. A tier-1 λ written at the call site without a claim is checked
   under the parameter's claim.
5. **A function a host returns is not an arrow.** It is a value of a host type whose eliminator is
   application, opaque as `go.Value` (ADR 0045), and an iterator is consumed by a loop. Tier 3 stays
   refused.

## Why not

- **No arrow.** 150 Go functions would stay undeclarable, and a callback's shape, which ρ needs for
  the host's func type, would have no statement.
- **Π as a set former in the type algebra**, closed under ∪, ∩ and ∖. Deciding inclusion between
  Boolean combinations of arrows is semantic subtyping with arrows (Frisch, Castagna and Benzaken,
  *JACM* 2008), exponential in the intersections, and a new universe in W9. It buys operations no host
  uses: no API takes a union of arrows, Go has no overloading (an intersection of arrows), and the
  complement of an arrow means nothing as a parameter's set.
- **An arrow on a λ or in `the`** (candidate E). A λ carries its claim since ADR 0051, so a second
  spelling is a synonym (fit-research's F1).
- **Claims on a definition's higher-order parameters now.** Inlining discharges every obligation on
  the residual (ADR 0037). Modular checking is the higher-order contract gap, deliberately not next.
- **A returned host function as a member of Π.** It would be a function held as a value, in V: tier 3.
- **A curried domain.** A claim's binder list has exact arity, as Σ's λ does (surface-design §1.5,
  candidate G).

## Consequences

- abstract.md: a ranging binder's set may be a claim (§2.2); a definition may bind a name to a claim
  (§2.6); W7 places a claim-as-set only on a host definition's parameter or a named claim; W9 gains a
  clause, a claim whose binders range over sets of values, beside the two levels and not above them.
- The surface needs no `->` token: an arrow is a λ's header without a body, one shape for a
  definition's header and a callback's set. surface-design §1.9's question of a tuple or a curried
  domain is settled: the tuple.
- The heading carries what a set cannot: how often the host calls the operator, whether it escapes,
  and that it captures no buffer (ADR 0052's R3). The markers' vocabulary is decided with callbacks
  tier 1.
- Dependent results are sayable (R may mention x̄), and stay on the deliberately-not-next list.
- An order-2 callback, a claim inside a claim's binder, is admitted by the same W9 clause when a
  program writes its own push iterator as a λ. None does now: the 9 order-2 Go functions take an
  iterator, which a program holds as a host value.
