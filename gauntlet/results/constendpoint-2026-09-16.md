# A constant's name is a range endpoint

2026-09-16. `emit/constend.go`, `emit/target.go`, `core/read.go`,
`targets/go/unicode-utf8.oro`, `emit/declforms_test.go`.

Step 4 of the build order in [theories-b-or-c.md §9](../../docs/theories-b-or-c.md), whose test is
*"`(int 0 utf8.MaxRune)` accepted"*. [theories.md §8.3](../../docs/spec/theories.md) has said since it
was written that the endpoint is legal *"because the definiens is available when the endpoint is
evaluated"*; constdecl-2026-09-15 built `const` and recorded this half as not built.

## 1. What it is

A constant is the CONDITIONAL cell of theories.md §2 — a declaration with a definiens *and* a
realization:

```
MaxRune : int [1114111, 1114111]  =  1114111  ↦  "utf8.MaxRune"
```

An endpoint is a closed term of a tiny compile-time grammar — literals, unary `−`, `+ − *`, `pow`,
`±inf` — evaluated at arbitrary precision and never emitted (unbounded-rung.md §2.1). Extending that
grammar with a constant's NAME is **δ at the type level**: substitute the definiens and evaluate. It
terminates for the reason δ does at the term level — a constant's definiens is a literal, so one
substitution step reaches a closed expression — and it needs no new arithmetic, no new term kind and
no widening of `KInt`, because the same `endpoint` function still sees only literals.

So the property is a COMMUTING SQUARE, and that is the test:

> writing `(int 0 MaxRune)` and writing `(int 0 1114111)` give the same declaration.

`TestAConstantsNameIsARangeEndpoint` asserts the two `Prim`s are `DeepEqual`. Nothing downstream can
tell which was written, which is what makes this a spelling rather than a feature.

## 2. The singleton range IS the definiens

Nothing records that a declaration came from `const`, and nothing needs to. A **pure operation of no
arguments whose result is `(int v v)`** denotes v and can denote nothing else: the type already states
the value. So `constValue` reads the definiens off the ordinary `Prim` that `constSig` produced, and a
hand-written `(sig UTFMax () (int 4 4) pure (host expr "utf8.UTFMax"))` works identically — `const` is
the sugar that writes the value once instead of twice, exactly as constdecl-2026-09-15 said.

Four refusals follow from that one sentence, each with a witness: a name nothing declares, a name
declared with arguments, a name whose result range is not exact, and a name that is not pure. Each
names what was wrong.

## 3. Resolution is DEFERRED, because the environment is the whole target

A constant may be declared in another file, or in another layer. A file is a **fragment**, and
loader-2026-09-15's property is that a file is the glue of its forms:

> `load(F₁ ++ F₂) = load(F₁) ⊔ load(F₂)`

so resolving an endpoint against the file being read would make **splitting a file change a target**.
A declaration whose types name a constant is therefore carried through the glue UNPARSED and
elaborated once the target is whole, beside the other passes that need the finished signature
(companions, views). `TestAConstantEndpointResolvesAcrossFilesAndLayers` splits the constant and its
use across two files of one layer and then across two layers, and requires the same `Prim` all three
ways.

**A deferred declaration is glued and never overridden**, stated rather than discovered: it is
elaborated after the layers have been folded, so it no longer knows which layer it came from, and a
name it collides with is refused naming both. Writing the digits is the way out; nothing in the corpus
needs it.

## 4. The ambiguity, for the third time

`(int 0 255)` is a range and `(int int string)` is an ARGUMENT LIST of three types. Both are
applications headed by `int`, and the first implementation looked for the shape ANYWHERE in a
declaration — so it read every positionally-written windows declaration as a range over a constant
called `string`, and the tooling suite refused eleven acceptance programs at once.

That is typeargs-2026-09-15's finding in a third place: **the grammar is ambiguous without position**,
and the resolution is the same one — admit the shape only where it cannot mean the other thing. The
scan now reads kids 2 and 3 of a `sig`, the same two `primOf` reads, and distinguishes a named
parameter from a compound type by the same rule `primOf` uses (a head that is not a type former).
INSIDE a type there is no ambiguity, so the walk there is free to go everywhere — and it must, because
`(tuple A B)` arrives as the Church term `(fn (#k) (#k A B))`: a tuple type is the term a tuple value
is (data.md §3.4), and a walk that stopped at applications missed every `tuple` result.

`TestAnEndpointThatIsNotAConstantIsRefused` ends with the witness — `(sig f (int int string) int …)`
must load with three arguments — and with a control: `+inf` is an endpoint of the grammar, not a name
to look up.

## 5. What it is worth

`targets/go/unicode-utf8.oro` declared `MaxRune` and `UTFMax` and then wrote their values again as
digits in five declarations, with a comment beside them saying *"a scalar value or RuneError, so
0..MaxRune; its size is 0 only for empty input and at most UTFMax"* — the prose naming the constants
and the code repeating the numbers. A value written twice is two claims that can disagree, which is
`const`'s own reason one level up. Now:

```lisp
(sig RuneLen   ((r (int -2147483648 2147483647))) (int -1 UTFMax) pure …)
(sig DecodeRune ((p (array (int 0 255)))) (tuple (int 0 MaxRune) (int 0 UTFMax)) pure …)
```

The remaining literal ranges in that file are the ones that are NOT constants: `(int 0 255)` is a
byte, `(int -2147483648 2147483647)` is Go's `rune`, and `(int 0 9007199254740991)` is ADR 0012's
window. That is the honest boundary — a manifest type is step 4's other half and is not built.

## 6. Cost

- **Emission byte-identical**: `456 runs: 194 emitted, 262 refused; 2307 of 2413 integer operations
  bounded, 345 of 382 loops proven`, with no refusal text changed. It is a spelling, so nothing could
  move.
- **Code**: `emit/constend.go` is new at **174** lines, `emit/target.go` 2,398 → 2,410 and
  `core/read.go` 1,394 → 1,395 (one exported predicate, `IsTypeFormer`, which is the ambiguity rule
  the emitter now asks about rather than restating). **+193**.
- Every check step passes; differential and tooling green.

## 7. Not built

- **`const` in a program module**, so an endpoint naming one is a target declaration's alone. The
  mechanism does not care — `constValue` reads a `Prim` — but a program module has no `const` form yet.
- **A constant in a `repr` or a `fact`**. `(repr (int LO HI) …)` could take one by the same rule; no
  target file wants it.
- **Layer order on a deferred declaration**, §3.
