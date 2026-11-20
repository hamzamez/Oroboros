# Research: formers, a data structure's literal as its free algebra's term

2026-11-20. hamza, on the trailing block (scopeforms-research §3): "what you showed is similar to
what kotlin is doing, but I meant something more general, like creating a new datastructure like a
list, and make the syntax `list {a, b, ... }`, and more than that. the trailing block is just a
special case of what I want. can we research it? see where it leads? we can always start with the
trailing block."

**Short answer.** A literal `list { a, b, c }` is a word in the free monoid over expressions, and
a data structure is a monoid it maps into. By the free monoid's universal property there is
exactly one homomorphism from the word to the structure. That homomorphism is the literal's
meaning, so **a former is declared by its algebra** (its operations and its laws) and never by
a procedure on syntax. Four things follow.
1. **The laws decide which literals and comprehensions mean anything.** The Boom hierarchy (trees,
   lists, bags, sets: none, associativity and unit, then commutativity, then idempotence) is a
   chain of quotients. A fold from one level to another exists iff the target obeys the source's
   laws (§2).
2. **`array { e : k ∈ D }` is already a comprehension**, the language's one instance. The general
   one, `F { e : x ∈ s, φ }`, is the same homomorphism out of s's algebra (§2).
3. **A block of statements is a term of a larger signature** (sequence, choice, iteration, binding).
   A former that reads a block is an algebra for that signature: Swift's result builders, F#'s
   computation expressions, and in general a handler of algebraic effects. Our staging runs the
   handler at compile time, since the block is static. The trailing block is the case of one
   operation with one binder (§3, §4).
4. **It is the same lattice as callbacks.** A host's promise about its calls (none, each element
   once in no order, in order) is the Boom level of its traversal. The λ's effect must be an algebra
   of that level (§5), so the middle strength's word names a level of this hierarchy.

**My opinion** (§7): build it in four steps, the trailing block first, each a homomorphism with no
macro. The laws are claims the logic proves, or a host's documentation states, as `ordered` is.

## 1. What `list { a, b, c }` is

**The free monoid.** Let E be the expressions of some set V. E* is the set of finite words over E,
with concatenation and the empty word: the free monoid on E. Its universal property:

> for every monoid (M, ·, 1) and every map u : E → M there is exactly one monoid homomorphism
> ū : E* → M with ū(⟨x⟩) = u(x).

The literal `{ a, b, c }` is the word ⟨a⟩⟨b⟩⟨c⟩. A data structure with an empty value, a
combination and a singleton, (M, ·, 1, u), gives it the meaning ū(⟨a⟩⟨b⟩⟨c⟩) = u(a) · u(b) · u(c).
Nothing else could be its meaning that respects the monoid's laws. This is initial algebra
semantics (Goguen, Thatcher, Wagner and Wright 1977) applied to the literal's own little syntax,
and it is ADR 0050's principle again: the syntax is an initial algebra and every meaning a
homomorphism out of it.

**So a former is four things**, all of the language:
- a carrier M, a set the language can write (a record, a table, a map);
- an empty value 1 ∈ M;
- a combination · : M × M → M;
- a unit u : V → M, the one-element structure.

`list { a, b, c }` is then a term over these four, folded by staging where its elements are static.
No table needs to exist first.

**The Boom hierarchy** (Meertens 1986, after Hendrik Boom; Bird 1987; Bunkenburg 1994) is the chain
of free algebras on one binary operation, each a quotient of the one before by one more law:

| structure | laws of · | free algebra on V | its literal means |
|---|---|---|---|
| binary tree | none (· with a unit, 1) | trees with leaves in V | the shape as written |
| list | associative, 1 its unit | V*, words | the elements in order |
| bag | + commutative | multisets, ℕ^V | the elements and their counts |
| set | + idempotent | finite subsets | which elements occur |

The same words, quotiented further, mean less. `{1, 2, 1}` is a word of length 3, a bag with 1
counted twice, and the set {1, 2}.

**The language's own formers fit this table.**
- `array { 2, 3 }` is a list realized as a table (ADR 0059's one former);
- `record { l : e }` is a product, not a free algebra: its labels are a fixed signature, which is
  why `with` and not `·` updates one;
- a map literal is a monoid of finite partial functions under right-biased union, which is
  associative and not commutative: `{k: 1} · {k: 2} = {k: 2}`. Under disjoint union it is commutative
  but partial, so each literal owes the obligation that its keys are distinct.

The other familiar structures fit the same way:
- a priority queue is a bag (its order is the elements', not the literal's);
- a string builder is the free monoid realized by bytes;
- a matrix literal `{ {1, 2}, {3, 4} }` is a list of lists with the obligation that every row has
  one length.

## 2. Comprehensions are the same homomorphism

`array { f(k) : k ∈ 0..n-1 }` is in the language today (tables.md). Read through §1, it is ū for
the list 0..n-1 and u = ⟨f(·)⟩: a homomorphism from the source's free monoid into the target's.
The general comprehension is

    G { e(x) : x ∈ s, φ(x) }   =   ⨀ over x in s with φ(x) of  u_G(e(x))

and its literature is old:
- Wadler (1990, "Comprehending monads"): a comprehension in any monad;
- Buneman, Naqvi, Tannen and Wong (1995, "Principles of programming with complex objects and
  collection types"): structural recursion on collections;
- Fegaras and Maier (2000, "Optimizing object queries using an effective calculus"): the monoid
  comprehension calculus.

**The well-formedness condition is the theorem that matters** (Fegaras and Maier's, and Buneman et
al.'s): the homomorphism from the source's free algebra exists **iff the target obeys every law of
the source's level**.
- Folding a set into a list is ill-defined: the list would need an order the set does not have.
  `list { x : x ∈ {1, 2} }` has no meaning, and a compiler must refuse it, naming commutativity.
- Folding a bag into a sum is fine, since + is commutative and associative; into a max, fine; into
  a concatenation of strings, refused.
- Folding a set into a count is refused: a count is not idempotent, so `{1, 1}` and `{1}` would
  count differently.

So a former's laws are not documentation. They are what the compiler checks before it accepts a
comprehension, and each refusal names the law that is missing.

## 3. A block is a term too

A block `{ s₁; s₂; if c { s₃ } else { s₄ }; loop … }` is a term of the signature
Σ_block = { sequence, let, choice, iteration, an operation call }. A former that gives a block a
meaning is an algebra for Σ_block, and the meaning is again the unique homomorphism. The surface
languages that do this name the algebra's operations:

| | sequence | choice | optional | iteration | element |
|---|---|---|---|---|---|
| Swift result builders (SE-0289, 2021) | `buildBlock(c₁…cₙ)` | `buildEither(first:)`, `(second:)` | `buildOptional` | `buildArray` | `buildExpression` |
| F# computation expressions (Petricek and Syme 2014) | `Combine`, `Delay` | (by `Combine` of arms) | `Zero` | `For`, `While` | `Yield`, `Bind`/`Return` |
| Haskell `do` | `>>=` | (in the monad) | | `mapM_` | `return` |
| Kotlin builders, `buildList { add(1) }` | the receiver's methods in order | | | | `add` |

**Algebraic effects say what all of these are** (Plotkin and Power 2003, "Algebraic operations and
generic effects"; Plotkin and Pretnar 2009, "Handlers of algebraic effects"). A block's operation
calls are a term of the free model of an effect's signature, and a handler is a model. Handling is
the unique homomorphism from the free model into it. Swift's builder is a handler whose
operations are fixed by the compiler (block, either, array). Kotlin's `buildList` is a handler of
one operation, `add`.

**Our two levels make the handler static.** Koka, Eff and OCaml 5 need continuations at run time,
because their computation's tree is built dynamically. Here a block written at a former is known at
compile time, and the static level is unrestricted and higher-order (closures-direction.md). So
the handler runs during staging, and its result is first-order: tables and loops.
- **Choice** is a conditional over the two arms' images.
- **Sequence** is the fold of ·.
- **Iteration is the one constraint.** A dynamic loop's body runs a run-time number of times, so a
  former can interpret a loop only by a fold whose carrier is first-order (a table, a record of
  scalars), carried as the loop's variable. That is Swift's `buildArray`, and it is our ADR 0047 in
  another hat: a cell is a loop variable.

A former whose carrier is a function, a static-only value, can read a block without loops and is
refused at one, naming the loop.

## 4. The trailing block is one operation with one binder

scopeforms-research §3's `f(a…) { x̄ -> b }` is `f(a…, fn(x̄) { b })`: an operation of the signature
whose last argument binds x̄ (a binding signature, Fiore, Plotkin and Turi 1999; ADR 0050's own
framework). In the picture above it is the case of a former with one operation that takes one body:
- `locked(h) { p -> … }`, the bracket, is a handler of no operations, a body run between two calls;
- `sort.Slice(xs) { i, j -> … }` is a host's callback written as a block;
- `each(a) { x -> … }` is a definition of the program's (ADR 0144) written as a block.

So the trailing block is the smallest piece, and it is meaningful alone. What it adds to §1–§3 is
only that a block can be the last argument. What §1–§3 add to it is a block whose statements are
the former's own operations, `list { a; if c { b }; c }`, and a literal with no binder at all,
`list { a, b, c }`.

## 5. The callbacks lattice is the Boom hierarchy

Callbacks-sketches Part 3 and ADR 0143 classify a host's promise about how it applies f to a
collection's elements, and what the λ's writes must then be. Laid beside §1's table:

| the host promises | its traversal is the free … | the λ's writes must be … |
|---|---|---|
| nothing (any count, any order) | (no algebra: not even every element) | none: pure (Theorem D) |
| every element, possibly again, no order | set | a semilattice: max, min, ∨, ∪ |
| **each element exactly once, no order** | **bag** | **a commutative monoid: +, ·, xor, and the above** |
| each once, in order (`ordered`) | list | any monoid: a print, Horner's step |

This is §2's well-formedness theorem again. The host's traversal is a fold out of its level's free
algebra, and the λ's effect is the target algebra, so the effect must obey the source's laws.
**So the middle strength's word names the bag level of this hierarchy.** If the language has
formers with declared laws, it is the same notion as the claim that a former is commutative. The
word hamza set aside (callbacks: "Decide later") is then the bag level's name, whatever it is.

## 6. Where the derivation leads, step by step

1. **The trailing block** (scopeforms §3): a reader rule, D's, no core term. It serves the bracket,
   a host's callbacks and a program's own higher-order definitions. Buildable now.
2. **A former declared by its algebra**:

       former Bag(V) = cmonoid { empty : bag_empty, combine : bag_union, unit : bag_single }

   - the carrier a set the language writes;
   - the level one of the hierarchy's names, its laws the claims the declaration makes;
   - `Bag { a, b, c }` meaning `bag_single(a) · bag_single(b) · bag_single(c)`, folded by staging
     where its elements are static.

   A host's collection (Go's map as a set of keys, the JVM's `HashSet`) gets its level from its
   documentation, kept as evidence like `ordered`.
3. **Comprehensions between formers**, `G { e : x ∈ s, φ }`: generalizing `array { e : k ∈ D }`,
   admitted iff G's level is at most s's, refused naming the missing law.
4. **Block formers**: an algebra for Σ_block's sequence, choice and iteration, a loop interpreted
   by a fold over a first-order carrier. Run as a handler during staging.

**The laws are claims, and that is the one new piece of logic.** A former whose operations are the
language's (`+` on ℤ, `max`, `∪` on a table's set) has its laws by theorem. A former whose
operations a program writes needs them proven:
- associativity of a record operation over the integers is a Presburger formula when the operation
  is linear, decided by the Omega test (`presburger/`);
- in general it is a lemma, ADR 0049's second language, not built.

It is the same piece of logic a comparator's law needs (a strict weak order for `sort`). So
building one serves the other.

## 7. My opinion

**Start with the trailing block, then formers by their algebra, then comprehensions, then block
formers. No macros at any step.**
1. Each step is a homomorphism out of an initial algebra, so ADR 0050's fit holds. A macro, a
   procedure on syntax, gives none of these guarantees (scopeforms §2).
2. The laws are where the meaning is: they decide which literals and folds exist, they choose a
   representation (a bag's order is the realization's to choose), and they unify formers with
   callbacks.
3. Staging is the handler: a former costs nothing at run time where its block is static, and a
   loop in a block is a fold, as a cell is a loop variable.

**Decisions for hamza:**
- **F1. A former is declared by its algebra** (carrier, empty, combine, unit), not by a conversion
  from a table (Haskell's `IsList.fromListN`, Swift's `ExpressibleByArrayLiteral`). The derivation:
  the algebra. A conversion builds the table first, an allocation the universal property avoids.
- **F2. The laws from the hierarchy's fixed levels** (none, monoid, commutative monoid,
  semilattice), not arbitrary user equations. The derivation: the levels. Well-formedness is then a
  comparison in a chain, decidable. Arbitrary equations make the word problem undecidable in
  general.
- **F3. The surface `F { a, b, c }`**, a name then braces, as `array { … }` and `record { … }`
  already are, and `F { e : x ∈ s }` for a comprehension: the reader's existing shapes.
- **F4. The trailing block now** (scopeforms §3), with the program it was made for: the clipboard
  reader, `locked(h) { p -> … }`, and a callback written as a block.
- **F5. The middle strength's word named by its level** (the bag's), decided with F2, since §5 makes
  them one notion.

**Deliberately not here:**
- mixfix and arbitrary notation (Agda's operators, Lean 4's `syntax` and `macro_rules`, Ullrich and
  de Moura 2020): a grammar a program extends is a reader the program runs;
- handlers with resumable continuations at run time (Koka, OCaml 5): a function value at the
  dynamic level, tier 3;
- formers over infinite structures (streams, coinduction).

## References

- Goguen, Thatcher, Wagner, Wright, "Initial algebra semantics and continuous algebras", JACM 1977.
- Meertens, "Algorithmics: towards programming as a mathematical activity", CWI 1986.
- Bird, "An introduction to the theory of lists", 1987.
- Bunkenburg, "The Boom hierarchy", Glasgow FP workshop 1993 (1994).
- Wadler, "Comprehending monads", LFP 1990.
- Buneman, Naqvi, Tannen, Wong, "Principles of programming with complex objects and collection
  types", TCS 1995.
- Fegaras, Maier, "Optimizing object queries using an effective calculus", TODS 2000.
- Fiore, Plotkin, Turi, "Abstract syntax and variable binding", LICS 1999.
- Plotkin, Power, "Algebraic operations and generic effects", Applied Categorical Structures 2003.
- Plotkin, Pretnar, "Handlers of algebraic effects", ESOP 2009.
- Petricek, Syme, "The F# computation expression zoo", PADL 2014.
- Swift Evolution SE-0289, "Result builders", 2021.
- Ullrich, de Moura, "Beyond notations: hygienic macro expansion for theorem proving languages",
  IJCAR 2020.
