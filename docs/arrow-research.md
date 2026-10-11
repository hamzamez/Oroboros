# The arrow: a function-space claim, not a set former

Research, 2026-10-11. surface-design.md §1.9 recorded hamza's decision *"function set formers: yes"*,
and the correction pass found that no ADR records it and that abstract.md lacks the arrow. hamza
asked: *"discuss 1. the arrow, function set formers. do we need or not. always mathematical and
algebraic."* Then: *"go ahead, I agree to the recommendations. document the reasoning behind it."*
**Decided** as [ADR 0053](decisions/0053-the-arrow-is-a-claim-on-a-host-parameter.md). Not built: it is
built with callbacks tier 1.

## 0. The answer, in brief

1. **The question is two questions.** Q1, semantic: does the logic ever need a claim f ∈ Π(x̄ ∈ P). R,
   "f is an operator that maps each x̄ in P into R(x̄)"? Q2, algebraic: if so, must Π be a **set
   former**, an operation of the type algebra, closed under ∪ ∩ ∖, nameable, usable wherever a set is?
   Or is a **heading** enough: a λ's claim (ADR 0051) without a body, used as a parameter's set?
2. **Q1: yes, at exactly one place.** Of four places a function-space claim could appear, three need
   none: a λ's claim and a definition's meaning already state it, and a definition's static parameter
   is discharged by inlining. The fourth needs it: **a host's callback parameter**, where the body is
   opaque, so nothing can be inlined and nothing derived. Measured: 150 of Go's 5,536 functions and
   methods take a function (2.7%).
3. **Q2: a heading, not a set former.** As a set former, Π would import arrow subtyping into the
   decidability map (semantic subtyping with arrows, Frisch, Castagna and Benzaken 2008) and a new
   universe into W9, for Boolean operations no host uses: no API takes a union of arrows, Go has no
   overloading (an intersection of arrows), and the complement of an arrow means nothing here. As a
   heading, the claim sort already exists, its only checks are inclusions between sets of values
   (P ⊆ P_f and R_f ⊆ R), and it can carry what a set cannot: how often the host calls the operator,
   whether it escapes, and that it captures no buffer.
4. **Functions a host hands back are not arrows.** 50 Go functions return one (`context.WithCancel`'s
   `CancelFunc`, the `iter.Seq` iterators, `sync.OnceFunc`). Such a value is held by the program, so it
   would be in V; as a member of Π it would reopen tier 3. It is a value of **a host type whose
   eliminator is application**, opaque like `go.Value`, and an iterator is consumed by a loop.
5. **So:** the arrow is a claim used as a set, ⟦claim⟧ = Π(P).R with its markers, written only on a
   host definition's parameter or as the value of a static definition that names it (Go's 18 named
   function types). No new operator, no `->` token, the domain a binder list (exact arity), and tier 3
   stays refused by construction.

## 1. The question, exactly

surface-design §1.8 derived that a callback declaration needs to say what shape of λ a parameter takes,
and §1.9 recorded *"function set formers: yes"*. §1.8 already restricted the arrow to "a binder's set,
and nowhere else", as a precaution. This document asks whether that restriction is a precaution or the
algebra, and whether the language needs an arrow at all.

## 2. The algebra

**Values and types.** V, the set of values, holds integers, floats, booleans, strings, tuples, tagged
values, tables and maps, and **no functions**: a λ is static and erased by staging, and a host
primitive is an operator that δ turns into a call (settypes §1.2). The types are subsets of V, and
they form a Boolean algebra (𝒫(V), ∪, ∩, ∖), ADR 0049.

**Operators.** Let Op be the operators a program can name: static λs, its own definitions, and host
functions. Op ∩ V = ∅. For P ⊆ Vⁿ and R a family of subsets of V indexed by P,

```
Π(x̄ ∈ P). R(x̄)  :=  { f ∈ Op | ∀x̄ ∈ P. f(x̄) ∈ R(x̄) }  ⊆  Op
```

the dependent function space (Martin-Löf 1972), with P → R its non-dependent case (Church 1940). It is
a subset of Op, not of V. So **adding Π adds a second universe beside 𝒫(V)**, not an element of it.

**Two questions follow.** Q1: is there a place where the logic must state f ∈ Π(P).R? Q2: if so, is
Π an operation of the type algebra, so that ∪, ∩ and ∖ apply to it, or a form usable only at that
place?

## 3. Measured

`go run gauntlet/stdlib/callbacks.go`, over Go's API manifest (`$GOROOT/api/go1*.txt`), one line per
declaration, `internal/` and `vendor/` excluded:

| | count |
|---|---:|
| functions and methods | 5,536 |
| **take a function** (a callback the host calls) | **150** (2.7%) |
| … through a literal `func(…)` type | 120 |
| … only through a named function type | 30 |
| … of order 2: the callback itself takes a function | 9 |
| **return a function** (a function the host hands back) | **50** |
| both | 8 |
| named function types | 18 |
| methods declared on a named function type | 3 |

The packages with the most callbacks: `slices` 17, `testing` 13, `bytes` 9, `strings` 9, `flag` 8,
`go/ast` 7, `sync` 6. In the ten packages targets/go has a file for, coverage counts 21 names under
cause A (callbacks) and 6 under cause B (implementing a host interface).

The order-2 callbacks are all iterator consumers: `iter.Pull`, `iter.Pull2`, `slices.Collect`,
`slices.AppendSeq`, `slices.Sorted`, `slices.SortedFunc`, `slices.SortedStableFunc`, `maps.Collect`,
`maps.Insert`. Each takes an `iter.Seq`, a function of `yield`. The named function types are
`archive/zip`'s `Compressor` and `Decompressor`, `bufio.SplitFunc`, `context.CancelFunc` and
`CancelCauseFunc`, `expvar.Func`, `go/ast`'s `Filter`, `FieldFilter` and `Importer`, `go/doc.Filter`,
`go/importer.Lookup`, `go/scanner.ErrorHandler`, `go/types.Qualifier`, `io/fs.WalkDirFunc`,
`iter.Seq` and `Seq2`, `net/http.HandlerFunc` and `path/filepath.WalkFunc`.

## 4. Q1: where a function-space claim could appear

### 4.1 A λ's own claim: no

Since ADR 0051 a λ carries its claim, lam(x̄ ∈ P; r ∈ R; φ; ψ; e), and 𝒪 (abstract.md §7) generates
its obligations: the body under x̄ ∈ P ∧ φ establishes r ∈ R ∧ ψ.

**Lemma 1.** lam(c; e) ∈ ⟦c⟧ exactly when 𝒪(lam(c; e)) holds, where ⟦c⟧ = Π(x̄ ∈ P_φ). R_ψ.

So `the(Π(P).R, λx̄. e)` and lam(x̄ ∈ P; r ∈ R; e) state the same thing, and having both is a synonym,
fit-research's F1. That is why dropping candidate E (surface-design §1.5) cost nothing.

### 4.2 A definition's meaning: no

abstract.md §2.6 states a definition's meaning as f ∈ Π(x̄ ∈ {x̄ ∈ S̄ : φ}). {r ∈ U : ψ}. The
definition's claim *is* that statement, so it needs no second spelling (Lemma 1, with a name).

### 4.3 A definition's parameter that receives a λ: not for soundness

The corpus has 119 parameters that are applied in their definition's body (fit-research §3.13), and
none carries a claim today.

**Lemma 2.** Let g be a definition whose parameter f is applied in its body. Every obligation of g's
body at a call g(λ) is an obligation of the residual after β.

*Sketch.* λs are static (closures-direction.md): β substitutes the λ for f, so every application
f(ā) becomes λ's body at ā. ADR 0037 puts every obligation in the residual and decides it there. ∎

So no claim on f is needed for soundness. A claim f ∈ Π(P).R would buy **modular** checking: g's body
checked once, under the hypothesis, and each argument obliged at the call. That is a higher-order
contract (Findler and Felleisen, "Contracts for higher-order functions", ICFP 2002), and CLAUDE.md
lists the higher-order contract gap (ADR 0028) as deliberately not next. If a program ever asks, the
claim is a heading on the parameter, the same form as §4.4.

### 4.4 A host's callback parameter: yes

`sort.Slice(x any, less func(i, j int) bool)`. Go's body is opaque, so Lemma 2 does not apply: there is
nothing to inline. The declaration must state:
- the arity;
- P, from which ρ chooses what the host passes (`int`, not `int64`, in Go's func type);
- R, from which ρ chooses what the callback returns.

The obligations run both ways, which is what makes a function space contravariant:
- **the callback's body may assume x̄ ∈ P.** That is the host's promise, assumed as an export's
  caller's uniqueness is (ADR 0020), and checked against the host's manifest by the generator
  (ADR 0022);
- **the callback's body must establish r ∈ R.** That is our obligation.

**Rule (fitting an argument to a parameter).** Let the parameter claim c = (P, R) and the argument
carry c_f = (P_f, R_f). Then

```
f fits c   ⇔   P ⊆ P_f   ∧   ∀x̄ ∈ P. R_f(x̄) ⊆ R(x̄)
```

the domain contravariant, the result covariant (Cardelli, "A semantics of multiple inheritance",
1984), and the higher-order contract's blame split (Findler and Felleisen 2002): a failed domain
inclusion is the host's fault, a failed result inclusion ours. A call is reasoned about from the
operator's specification alone, as Hoare's rule of adaptation reasons about a procedure call
("Procedures and parameters: an axiomatic approach", 1971). **Both inclusions are between sets of values**, in the algebra settypes §5 already decides. For a
tier-1 λ written at the call site without a claim of its own, the check is simpler still: the λ is
checked *under* c, its body assuming P and establishing R, which is Lemma 1 with c as its claim.

**So Q1's answer is yes, and only here**: where an operator crosses into a body the compiler cannot
see. 150 Go functions, 2.7% of the API.

### 4.5 A function the host hands back: not an arrow

50 Go functions return a function. By kind:
- **iterators** (`iter.Seq`, `iter.Seq2`): `strings.Lines`, `strings.SplitSeq`, `bytes.FieldsSeq`,
  `maps.Keys`, `reflect.Value.Seq`, and the `go/types` accessors;
- **cancellation and once**: `context.WithCancel`, `WithDeadline`, `WithTimeout`, `AfterFunc`,
  `signal.NotifyContext`, `sync.OnceFunc`, `OnceValue`, `OnceValues`;
- **others**: `reflect.Swapper`, `net/http.ProxyURL`, `go/types.RelativeTo`, `iter.Pull`.

Such a function is a value the program **holds**: bound, passed on, called later. Giving it a set
Π(P).R would put a function in V, which is tier 3, refused (callbacks.md §2, ADR 0018). It needs no
arrow. It is a value of **a host type whose one eliminator is application**, opaque as `go.Value` is
(ADR 0045), declared in the target file with its call as an operation:

```
CancelFunc : a host type;   call : CancelFunc → 1
```

An iterator is consumed by a loop, `for x := range seq`, which is tier 1 turned around: the loop's
body becomes the host's `yield`. The order-2 callbacks of §3 all take an iterator, and the argument is
almost always such a host value (`slices.Collect(maps.Keys(m))`), not a λ the program writes. **So a
heading inside a heading is not needed now.** It would be, the day a program writes its own push
iterator as a λ, and the form allows it (§6).

## 5. Q2: a set former, or a heading

### 5.1 What a set former costs

If Π is an operation of the type algebra, the algebra must decide inclusion between any expressions
that contain it.
- **Alone, cheap.** Π(P).R ⊆ Π(P′).R′ ⇔ P′ ⊆ P ∧ R ⊆ R′, for P′ ≠ ∅.
- **Under ∪, ∩ and ∖, expensive.** That is semantic subtyping with arrows (Frisch, Castagna and
  Benzaken, "Semantic subtyping", *JACM* 55(4), 2008; CDuce). Interpreting an arrow as a set without
  circularity needs their construction, arrows read as sets of finite relations, and deciding
  ⋀ᵢ(Pᵢ → Rᵢ) ⊆ P → R needs their decomposition over subsets of the intersection's arrows, which is
  exponential. It would be a new region of settypes §5's decidability map.
- **And W9 would gain a universe.** W9 admits a binder's set at two levels, sets of values and sets of
  sets of values. A set of operators is neither, so it would be a third kind beside them, closed under
  the Boolean operations.

### 5.2 What it would buy: nothing a host uses

| operation on arrows | meaning | where a host has it |
|---|---|---|
| (P → R) ∪ (P′ → R′) | f maps P to R, or P′ to R′, and the caller does not know which | no API takes one |
| (P → R) ∩ (P′ → R′) | an overloaded function | Go has no overloading; a Java lambda targets one functional interface; JavaScript callbacks are untyped |
| Op ∖ (P → R) | an operator outside a function space | meaningless as a parameter's set |

The Boolean operations would be decided and never used.

### 5.3 What a heading costs, and what it carries

A heading is ADR 0051's Claim, a λ's binders with their sets, its result binder, φ and ψ, with no
body. The sort exists (`core/abs.go`, `Claim`). Its meaning is

```
⟦c⟧ = Π(x̄ ∈ {x̄ ∈ S̄ : φ}). {r ∈ U : ψ}   ∩   M(markers)
```

where M is the set of operators satisfying the markers. Its only checks are §4.4's two inclusions, in
the algebra of sets of values. Nothing is closed under anything new.

**And a callback's contract is more than a function space.** What the host does with the operator
matters as much as its type:
- **how often it is called**: `sort.Slice`'s `less` many times, `sync.Once.Do`'s once, `strings.Map`'s
  once per rune. That is 𝒰's [0, ∞] scaling (abstract.md §8.1), and it decides whether the callback
  may consume a relevant value;
- **whether it escapes**: `time.AfterFunc` and `go` run it later or elsewhere, `sort.Slice` only
  during the call;
- **that it captures no buffer** (callbacks.md §4), ADR 0052's R3.

A set in a Boolean algebra has no place for these. A claim already has markers (`pure`, modes), so the
heading's markers are where they go.

**The precedents.** Algol 60 specified only that a parameter is a procedure, not its parameters, a
known gap. ISO Pascal closed it with a heading: a procedural argument's parameter list must be
*congruent* with the parameter's (ISO 7185 §6.6.3.6). Algol 68 closed it with a mode,
`proc (int, int) bool`, a type former. Go's function types are headings written as types,
`func(i, j int) bool`, and Go has no operation on them but naming. And naming one is Z's schema naming
(Spivey 1992): a schema is named, then used as a declaration.

### 5.4 The two are one meaning

The heading denotes exactly Π(P).R intersected with its markers' conditions, so the meaning hamza
decided is kept whole. What is withheld is only **closure**: ⟦·⟧ maps claims into subsets of Op, and
its image is used as a binder's set, never as an operand of ∪, ∩ or ∖, of `the`, or of a static
parameter's range. That is the restriction §1.8 imposed as a precaution, and §5.2 shows it is the
algebra: the closure has no use.

## 6. What it fixes

1. **The arrow is a claim used as a set.** A ranging binder's set may be a claim, meaning Π(P).R and
   its markers. No new operator in Σ.
2. **Where.** On a host definition's parameter, and as the value of a static definition that names it,
   for the host's named function types (`SplitFunc == (data ∈ Seq(Byte), atEOF ∈ Bool) → …`). Not on a
   definition's parameter until a program asks for modular checking (§4.3).
3. **W9 gains a clause**, not a level: a binder's set may be a claim whose binders range over sets of
   values. A claim inside a claim's binder is the order-2 case (§4.5), admitted by the same clause
   when a program needs it.
4. **The domain is a binder list**, so arity is exact. surface-design §1.9's open question, tuple or
   curried, is settled: the tuple form, by construction.
5. **The surface needs no `->` token.** An arrow is a λ's header without a body:
   `less(i ∈ Int, j ∈ Int) ∈ Bool` in D, `less ∈ fn(i ∈ Int, j ∈ Int) (r ∈ Bool)` in J. One shape for
   a definition's header and a callback's set.
6. **Tier 3 stays refused, by construction**: a claim is not a value, so no table holds one.
7. **A function a host returns is a host type with application**, declared in the target file, and an
   iterator is consumed by a loop. Neither is an arrow.

**Not decided here**, and decided when callbacks tier 1 is built: the markers' vocabulary (call
count, escape), and how a target file writes a callback hole (callbacks.md §2).

## 7. The literature

| | answers |
|---|---|
| Church 1940, "A formulation of the simple theory of types" | the arrow as a type former |
| Martin-Löf 1972; 1984 | Π, the dependent function space |
| Algol 60 report (Naur et al. 1963); Algol 68 (van Wijngaarden et al. 1976) | a formal procedure specified only as `procedure`, the gap; then a mode `proc (int) bool` |
| Hoare 1971, "Procedures and parameters: an axiomatic approach" | a call reasoned about from the procedure's specification: the rule of adaptation |
| ISO 7185 (Pascal), §6.6.3.6 | a procedural argument's parameter list congruent with the parameter's: a heading |
| Cardelli 1984, "A semantics of multiple inheritance" | the function space contravariant in its domain |
| Spivey 1992, *The Z Notation* | a schema named and used as a declaration |
| Findler and Felleisen 2002, "Contracts for higher-order functions" | the domain contravariant, blame split between caller and callee |
| Frisch, Castagna and Benzaken 2008, "Semantic subtyping" | the cost of arrows in a Boolean algebra of types |
| Reynolds 1978, "Syntactic control of interference" | what a procedure argument may share; the no-buffer marker |
