# The language's notation, from first principles

Research, 2026-10-10, on hamza's request: *"I want to entertain the idea of redoing it from scratch,
including the choices of keywords, and the rest of the language not just types. I like choices
lamport did with tla+ for the same reasons he did, it is math, simple, and well founded, that does
not mean I want it exactly. but if we did it from first principles, and from the algebra, and knowing
everything we know right now, what would it look like? what would we keep, change, get rid of? even
if we have to rewrite all the examples and target files … show it to me in a research document. then
we decide how to proceed."*

It builds on [settypes-research.md](settypes-research.md) and
[ADR 0049](decisions/0049-types-are-sets-and-the-logic-never-prints.md). Nothing here is built, and
nothing is decided.

## 0. The answer, in brief

1. **The language is the executable subset of a mathematical notation.** settypes-research made the
   logic a set theory and the computation a calculus whose values are functions on finite domains.
   Both are mathematics already. Today's S-expressions spell the mathematics in a notation that is
   not mathematical. The header comments of `count.oro` and `ages.oro` write their algorithm as
   mathematics, and then the code writes it again differently. The proposal is one notation for
   both, as Lamport made TLA+ and PlusCal and Abrial made B and its executable subset B0, in which
   the code is the mathematics.
2. **What changes is the notation and five unifications the algebra offers. The semantics does not
   change.**
   - the reducer, the IR, the analyses, the target model, the error model, local state and every ADR's
     decision stay as they are, and only their spelling changes;
   - the IR's text stays S-expressions: it is an internal format (ADR 0038), not a surface.
3. **The proposed notation**, TLA+'s with PlusCal's state:

   ```
   div(a ∈ Int, b ∈ Int \ {0}) ∈ Int == a ÷ b

   total(xs ∈ Seq(0..1000)) ∈ Nat ==
     LOOP i := 0, s := 0 :
         i ≥ Len(xs) → s
      [] OTHER       → AGAIN(i + 1, s + xs[i])
   ```

   - `==` defines, and `∈` gives a claim's set;
   - `0..1000` is a range;
   - `Seq(S)` is a table of S, and `xs[i]` applies it to an index;
   - the clauses are Dijkstra's guarded commands, and `AGAIN` is the jump back.

   Reserved words are capitals, as in TLA+, so they can never collide with a name. Every symbol has
   an ASCII spelling (`\in`, `|->`, `/\`), as TLA+'s does.
4. **The five unifications**:
   - **one choice**: `CASE` takes the place of `cond`, `case`, and `match` used as a choice;
   - **one local state**: buffer elements are written in program order, `b[i] := v`, as cells are, so
     a host's write-borrow becomes a write;
   - **one finite function**: tables and maps are functions on finite domains, made by
     `[x ∈ D ↦ e]`, applied by `f[x]`, measured by `DOMAIN`, updated by `EXCEPT`, and ρ chooses slice
     or hash map from the domain's proven shape;
   - **one definition**: a signature's claims sit in the definition's header, so `sig` goes away,
     and a host declaration is a definition whose body is in the host;
   - **membership is a test**: `x ∈ S` is computable wherever S's membership is decidable at run
     time, and `CASE v OF i ∈ Int → …` takes a union apart.
5. **What goes, beyond those**: S-expressions as the surface, hyphens in names (`-` becomes
   subtraction), `seq` (becomes `;`), `the` (becomes a claim), the `(int LO HI)` spelling (becomes
   `LO..HI`), and the legacy kinds `loop2` and `fold-range`.
6. **The cost.** About 200 `.oro` files (12,200 lines), 76 Go test files with embedded source, and
   628 code blocks in 91 documents. That is feasible only with a mechanical translator: today's
   reader parses the old notation, and a printer writes the new one, carrying every comment. Writing
   it is most of the work, and it is the first step if hamza chooses the notation.

## 1. What a notation must serve

### 1.1 The two languages

From settypes-research §1:
- **The computation** is the λ-calculus with δ, the table rule, `if`, and `loop`/`again` iteration.
  Its values are integers, floats, booleans, strings, host values, tuples, tagged values, and finite
  functions (tables and maps).
- **The logic** is first-order set theory over those values. A type is a set, and a claim is a
  formula.

Every construct of the computation denotes something in the logic, so one notation can write both.
The computation is the subset whose terms the compiler can run: everything except the
non-computable (`CHOOSE`, a quantifier over an infinite set, a set used as a value at run time).
That is Abrial's B0, the implementable subset of B (*The B-Book*, 1996), and PlusCal's relation to
TLA+ (Lamport, "The PlusCal algorithm language", ICTAC 2009).

### 1.2 The criteria

hamza named three, Lamport's own. To them I add four that this project has learned.

| criterion | means |
|---|---|
| **mathematics** | a program reads as the mathematics its derivation states: sets, functions, formulas in their usual notation |
| **simple** | few constructs and few words, one spelling per idea |
| **well founded** | every construct denotes something in set theory (settypes §1.2), and every word is a construct |
| **uniform** | one shape for every binder (a name and its set), for every definition, for every multi-way choice |
| **unambiguous** | a small precedence table, and parentheses wherever two families of operators meet |
| **comments are gaps** | ADR 0024: erased by the lexer, never meaning, preserved by every tool that rewrites source |
| **translatable** | the whole corpus can be converted mechanically, comments included |

### 1.3 Why Lamport chose mathematics

Lamport's argument (*Specifying Systems*, 2002, and its introduction to TLA+) is that ordinary
mathematics is the most expressive and best understood language people have for precise
statements. It is centuries older than any programming language, its meaning is fixed, and engineers
already read it. TLA+ is that mathematics with an ASCII spelling and a pretty-printer. Lamport kept
programming constructs out of it, and then wrote PlusCal for the people who wanted to write
algorithms, translated into TLA+ so they could be checked.

Three of his choices transfer whole:
- **definitions with `==`**, distinct from equality `=`, a formula;
- **every binder carries its set**: `\A x \in S : P`, `{x \in S : P}`, `[x \in S |-> e]`;
- **reserved words in capitals**: `IF`, `THEN`, `CASE`, `OTHER`, `LET`, `IN`, `EXCEPT`, `DOMAIN`.

Others transfer with changes. TLA+ has no types, so its sets are only formulas; ours also choose
representations (settypes §1.4). TLA+ describes behaviours and does not compute; ours computes, so we
add PlusCal's assignment and sequencing, and iteration.

### 1.4 The other precedents

- **Dijkstra's guarded commands** ("Guarded commands, nondeterminacy and formal derivation of
  programs", CACM 1975): `if g₁ → s₁ [] g₂ → s₂ fi` and `do … od`. TLA+'s `CASE … [] …` comes from
  them, and so, unknowingly, did ADR 0015: `loop` with guarded clauses is Dijkstra's `do`, with an
  explicit exit value.
- **Z** (Spivey, *The Z Notation*, 1992) and **B** (Abrial 1996): set-theoretic specification, with
  B refined down to B0 and generated as C or Ada. This is the closest precedent for compiling
  mathematics to code.
- **ISWIM** (Landin, "The next 700 programming languages", CACM 1966): `let` and `where` as
  mathematical notation for programs.
- **Fortress** (Steele et al., 2008): mathematical notation for a general-purpose language, rendered
  in Unicode. It is evidence that a precedence table for mathematics is buildable, and that the
  rendering needs a formatter.

## 2. Two notations, weighed

**A: keep S-expressions and rename.** `(def total (xs) (loop ((i 0) (s 0)) …))` becomes better
words in the same shape. It needs the least migration and the smallest parser.

**B: the mathematical notation.** TLA+'s, with PlusCal's state and Dijkstra's guards.

| criterion | A: S-expressions | B: mathematics |
|---|---|---|
| reads as the mathematics | no: every formula is prefix, `(and (<= 0 i) (< i n))` | yes: `0 ≤ i < n` |
| claims (sets, quantifiers) | verbose: `(forall (x S) (in (f x) T))` | native: `∀ x ∈ S : f(x) ∈ T` |
| uniformity | partly illusory: `def`, `let`, `loop` and `match` each have their own grammar inside the parentheses, discriminated by arity and parity (ADR 0024's motive) | one expression grammar, with keyword forms |
| precedence | none needed | a small table (§4.9), parentheses where families meet, and a formatter |
| comments | gaps, and no block comments (ADR 0024) | gaps, block comments allowed: no grammar is chosen by arity any more |
| input | ASCII | Unicode, with an ASCII spelling for every symbol |
| target files | prefix declarations | headers that read like the host's documentation: `EncodeRune(p ∈ Buffer(Byte), r ∈ Int32) ∈ 1..4` |
| migration | small | total, by a translator |
| error messages | quote S-expressions | quote the mathematics the programmer wrote |

**Recommendation: B.** It is the only one of the two that meets the first criterion, and since
ADR 0049 the logic, which is formulas, is a primary half of the language. A's uniformity argument is
weaker than it looks: the forms inside the parentheses already have separate grammars. The cost is
the migration, and §9 makes it mechanical.

## 3. The algebra each construct comes from

The notation follows the algebra, section by section of settypes-research §2:

| in the algebra | construct | notation |
|---|---|---|
| a definition (a conservative extension) | definition | `name == e`, `f(x, y) == e` |
| a set | type | `Int`, `0..255`, `{"GET", "POST"}`, `S ∪ T`, `S ∩ T`, `S \ T`, `{x ∈ S : P}` |
| a product | tuple | `(a, b)`; the set `S × T` |
| a tagged sum | variant | `ok(v)`; the set `ok(T) \| err(E)` |
| a function on [0, n) | table | `⟨a, b, c⟩` by its graph, `[i ∈ 0..n-1 ↦ e]` by its rule; the set `Seq(S)` |
| a finite partial function | map | `[k ∈ K ↦ e]`; the set `[K → S]` |
| a function on field names | record (not built) | `[name ↦ "ana", age ↦ 42]`; the set `[name : String, age : Int]` |
| application | call, index | `f(x)` for a definition or a host function, `t[i]` for data |
| a bool's eliminator | conditional | `IF c THEN a ELSE b` |
| a sum's eliminator, guards | choice | `CASE … OF …` |
| a least fixed point by iteration | loop | `LOOP x := z : clauses`, `AGAIN(…)` |
| state threaded by translation | local state | `LOCAL x := e IN …`, `x := e`, `t[i] := e`, `;` |
| the exception monad's bind | errors | `TRY`, `EXPECT`, `IGNORE` |
| membership as a formula | claim | `x ∈ S` |
| membership as a test | test | `x ∈ S` in a computation, where S is decidable at run time |
| a formula at a point | claim | `ASSERT P`, `INVARIANT P`, `ENSURE P` |
| a proven sentence | lemma | `LEMMA name == P`, `PROOF BY …` |
| quantification over sets | polymorphism | `f(T ⊆ S)(x ∈ T) …`, a static parameter list first |

## 4. The language

### 4.1 Modules and definitions

```
MODULE ages
USE go, go/os AS os, go/bufio AS bufio, go/fmt AS fmt, result
EXPORT main

cap_lines == 16777216
ok(r) == CASE r OF result.ok(u) → TRUE [] result.err(e) → FALSE
```

- **`==` defines and `=` is equality**, a formula. A definition is the only way a name enters, and it
  is conservative (settypes §4.2). A definition ends where the next `name ==` begins, as in TLA+.
- **`LET x == e IN body`** binds in sequence, as today's `let` does.
- **`λ x : e`** (ASCII `LAMBDA x : e`) is the static level's function, today's `fn`. A residual still
  never holds one (closures-direction.md).
- **Module paths stay the host's** (ADR 0025), and `USE PATH AS ALIAS` stays.
- **A static parameter list comes first, in its own parentheses**: `AsType(E ⊆ error)(err ∈ error)`.
  The static level is applied at compile time, as settypes §6.2 makes polymorphism β.

### 4.2 Sets

```
Int  F64  Bool  String              the sorts, defined in the prelude, not reserved words
0..255   -1..+∞   Nat == 0..+∞      ranges: semilinear sets; today's (int 0 255)
{"GET", "POST"}  {0}               enumerations; a literal is its singleton
Int ∪ F64 ∪ String                  union
Even ∩ 0..100                       intersection
Int \ {0}                           difference
{n ∈ Int : n % 2 = 0}               comprehension: a binder, its set, a formula
Int × String                        product
Seq(Byte)                           tables of a set: ⋃ₙ [0..n-1 → Byte]
[0..9 → Bool]                       functions on a finite domain
Option(T)  Result(T, E)             variants, defined in the prelude and lib/result
Buffer(Byte)  Cell(Int)             usage modalities, not sets (settypes §2.5)
```

The sorts are **definitions in a prelude** (`Int`, `F64`, `Bool`, `String`, `Nat`, `Unit == {()}`),
not reserved words, because types are static values (ADR 0049). TLA+'s standard modules define `Nat`
and `Int` the same way.

**A variant is a set of tagged values**, and its declaration introduces its constructors:

```
VARIANT Result(T, E) == ok(T) | err(E)
  SUCCESS ok
  RELEVANT
```

`ok(T) | err(E)` is {ok(t) : t ∈ T} ∪ {err(e) : e ∈ E}. `|` writes a tagged sum and introduces
names, while `∪` combines sets that exist already. That difference is why `VARIANT` is a keyword:
no definition introduces names silently.

### 4.3 Data: tuples, tables, maps, records

```
(a, b)                              a tuple: a product, eliminated by projection or a pattern
()                                  the unit
⟨104, 105, 33⟩                      a table by its graph (ASCII << … >>)
[i ∈ 0..n-1 ↦ i * i]                a table by its rule (ASCII [i \in 0..n-1 |-> i * i])
[k ∈ keys ↦ 0]                      a map: a function on a finite set of integers
t[i]                                application to data: indexing is application (tables.md §2)
Len(t)  DOMAIN m                    a table's length; any finite function's domain, in order
[t EXCEPT ![i] = v]                 a function like t but at i: a value, not a store
s ∘ u                               concatenation of strings and of tables (ASCII \o)
```

**Tuples and tables stay distinct, by polarity** (types-direction §6.3). A tuple is a product,
eliminated by projection or by a pattern, and it need never exist. A table is data, indexed by a
computed integer, and it must exist. Mathematics writes the ordered pair `(a, b)` and the sequence
`⟨a, b, c⟩` differently for the same reason.

**Tables and maps become one kind of value**: a function on a finite domain. A table is one whose
domain is 0..n−1 and a map is one whose domain is any finite set of integers. They share one
construction, `[x ∈ D ↦ e]`, one application, `f[x]`, one domain, `DOMAIN f`, and one update,
`EXCEPT`. ρ chooses the carrier from the domain's proven shape: a slice when the domain is proven
contiguous from 0, a hash map otherwise. This is data.md's thesis, "every data form is a function
whose domain differs", made into the notation. A map's declared capacity (maps.md) becomes a claim
on `|DOMAIN m|`.

### 4.4 Control

```
IF c THEN a ELSE b

CASE r OF                           a sum's eliminator
    ok(v)  → v
 [] err(e) → e + 1000

CASE                                guards: Dijkstra's if … fi, today's cond
    n < 0  → -1
 [] n = 0  → 0
 [] OTHER  → 1

CASE v OF                           sorts: a union's eliminator, narrowing each arm
    i ∈ Int    → render(i)
 [] f ∈ F64    → format(f)
 [] s ∈ String → s
```

`CASE` is the one multi-way choice: on constructors, on guards, or on sorts. It takes the place of
`case`, `cond`, `try`'s and `expect`'s implicit arms, and `match` when that is used as a choice.

```
LOOP i := 0, s := 0 :
    i ≥ Len(xs) → s
 [] OTHER       → AGAIN(i + 1, s + xs[i])
```

`LOOP` is Dijkstra's `do` with an exit value. Its body is a guarded chain, each clause either a value
(an exit) or ending in `AGAIN` (a back edge). ADR 0015's rule stays: `AGAIN` closes a clause, or sits
under a binding, never inside an expression. `match`'s other use, a state machine where `again`
means "match again", is a `LOOP` whose clauses are its patterns, so it needs no word of its own.

**Membership is also a test.** In a computation, `x ∈ S` is computable when S's membership is
decidable at run time: a range, an enumeration, a sort, a finite union of these, or a comprehension
whose formula is itself computable. `IF age ∈ 0..150 THEN …` is a comparison. `CASE v OF i ∈ Int →`
reads the tag ADR 0042 already stores. A set whose membership is not computable is refused in a
test, and is still a claim in the logic.

### 4.5 Local state

```
LOCAL lines := 0, words := 0, buf := [i ∈ 0..n-1 ↦ 0] IN
  …
  lines := lines + 1;
  buf[i] := b;
  …
```

PlusCal's state, in the language's own form:
- `LOCAL x := e IN body` opens a scope;
- `x := e` writes a cell, and `t[i] := e` writes a table's element;
- `;` sequences two statements;
- a scope's value is its body's.

The meaning is unchanged: the state-passing translation (local.md §3), which is also exactly how
PlusCal is translated into TLA+, its variables becoming state. The initializer still decides what a
binder is (ADR 0047): a function on a finite domain makes a buffer, anything else a cell.

**One change of meaning, and it is a simplification: buffers are written in program order too.**
Today a buffer is threaded by hand, `(again (set s j true) …)`. That form exists because the
refinement layer's facts induct on a buffer's store chain. The translation can produce that chain
itself, as it produces cells' loop variables, so the programmer writes `s[j] := TRUE` and the analyses
still see the chain. Two consequences follow:
- **a host's write-borrow becomes a write.** `hex.Encode(buf, src)` writes `buf` and returns its
  count. The template no longer returns the buffer it borrowed: today's
  `func(dst, src []byte) ([]byte, int) { return dst, hex.Encode(dst, src) }(%s, %s)` becomes
  `hex.Encode(%s, %s)`. That is ADR 0048's stage B, for buffers;
- **linearity becomes structural.** A buffer is reached only through its scope's name, so no alias
  can be written.

### 4.6 Errors

```
TRY ok(f) == os.Open(path) IN …           the bind: any other constructor is the value
EXPECT ok(n) == strconv.Atoi(s) ELSE "not a number" IN …   give up with a reason
IGNORE fmt.Fprintln(os.Stderr, msg)       the discard, !_A : A → 1
```

The meaning is ADR 0040's, 0041's and 0043's. Only the spelling is new.

### 4.7 Claims: the logic, written where it applies

```
div(a ∈ Int, b ∈ Int \ {0}) ∈ Int == a ÷ b                    parameter and result sets

encode_len(n ∈ Nat) ∈ Nat
  ENSURE RESULT = 2 * n                                        a postcondition; RESULT names the value
  == 2 * n

copy(dst ∈ Buffer(Byte), src ∈ Seq(Byte)) ∈ Nat
  WHERE Len(src) ≤ Len(dst)                                    a relation among parameters
  == …

ASSERT k < Len(a)                                              a formula at a point, then a fact

LOOP i := 0, s := 0  INVARIANT s = sum(a, 0, i) : …           proven at entry, kept by each back edge

RECURSIVE sum(_, _, _)                                         a specification function: never runs
sum(a, i, j) == IF i ≥ j THEN 0 ELSE a[i] + sum(a, i + 1, j)

LEMMA sum_snoc == ∀ a ∈ Seq(Int), i ∈ Nat :
  i < Len(a) ⇒ sum(a, 0, i + 1) = sum(a, 0, i) + a[i]
PROOF BY INDUCTION ON i
```

Today's `sig` and `def` become one definition carrying its claims. The claims are sets in the header,
`WHERE` for relations among parameters, and `ENSURE` for the result, named `RESULT`. The name
`result` belongs to `lib/result`, so the result's name is a reserved word that cannot clash. TLA+
also marks a recursive definition `RECURSIVE`. Here that marks a definition the logic may use and
the computation may not (ADR 0014, settypes §6.3).

### 4.8 Host declarations and targets

A host declaration is a definition whose body is in the host. It is an axiom, checked against the
host by the tooling (ADR 0022):

```
MODULE go/unicode/utf8

MaxRune == HOST "utf8.MaxRune" IMPORT "unicode/utf8"
ValidRune(r ∈ Int32) ∈ Bool  PURE  == HOST expr "utf8.ValidRune(rune(%s))" IMPORT "unicode/utf8"
EncodeRune(p ∈ Buffer(Byte), r ∈ Int32) ∈ 1..4
  WHERE 4 ≤ Len(p)
  == HOST expr "utf8.EncodeRune(%s, rune(%s))" IMPORT "unicode/utf8"

MODULE go/fmt
Println(a ∈ Seq(go.Value)) ∈ Unit  VARIADIC  == HOST stmt "fmt.Println(%s...)" IMPORT "fmt"
Sscan(s ∈ go.bytestring, a ∈ Seq(Cell(go.Value))) ∈ Nat × Result(Unit, error)  VARIADIC
  == HOST expr "fmt.Sscan(%s, %s...)" IMPORT "fmt"
```

A target's own forms take the same shape:

```
TYPE error == HOST "error"
TYPE Seq(A) == HOST "[]%s"                      today's (type (array A) (host "[]%s"))
REPR -9223372036854775808..9223372036854775807 AS WORD
go/os.File ⊆ go/io.Writer                       today's implements: an inclusion, an axiom
FACT ∀ t ∈ Seq(Any) : Len(t) ≤ 9223372036854775807
```

`implements` is literally an inclusion between sets, so it is written as one.

### 4.9 Lexical, and precedence

- **Identifiers** are letters, digits and `_`. `-` is subtraction, so `best-name` becomes
  `best_name`. Host names keep the host's spelling (`fmt.Sscan`, `os.ReadFile`), as ADR 0025 has them.
- **Reserved words are capitals**: `MODULE USE AS EXPORT VARIANT SUCCESS RELEVANT LET IN IF THEN ELSE
  CASE OF OTHER LOOP AGAIN LOCAL TRY EXPECT IGNORE OUT LAMBDA WHERE ENSURE RESULT ASSERT INVARIANT
  RECURSIVE LEMMA THEOREM PROOF BY INDUCTION ON HOST IMPORT PURE VARIADIC TYPE REPR FACT DOMAIN
  EXCEPT SUBSET TRUE FALSE`. That is 48, against today's 136 words, a count which includes target
  kinds and the bignum names a target declares. Capitals are TLA+'s choice for its reason: a reserved
  word cannot collide with a program's name or a host's (Go's exported names are mixed case).
- **Every symbol has an ASCII spelling**, TLA+'s where TLA+ has one:

  | symbol | ASCII | | symbol | ASCII |
  |---|---|---|---|---|
  | `∈` `∉` | `\in` `\notin` | | `∪` `∩` `\` | `\cup` `\cap` `\` |
  | `⊆` | `\subseteq` | | `×` | `\X` |
  | `∀` `∃` | `\A` `\E` | | `↦` `→` | `\|->` `->` |
  | `∧` `∨` `¬` `⇒` | `/\` `\/` `~` `=>` | | `⟨` `⟩` | `<<` `>>` |
  | `≤` `≥` `≠` | `<=` `>=` `/=` | | `÷` `∘` | `\div` `\o` |
  | `λ` | `LAMBDA` | | `[]` | `[]` |

  A formatter prints one canonical spelling (§10, decision 3).
- **Comments are gaps** (ADR 0024): `\*` to the end of the line, and `(* … *)`. ADR 0024 refused
  block comments because a comment could change which grammar an arity-discriminated form takes.
  An expression grammar has no such forms, so block comments become safe. That would need a
  successor to ADR 0024, which keeps its principle: a comment never carries meaning.
- **Precedence**, loosest first:
  1. `;`
  2. `:=`
  3. `⇒`
  4. `∨`
  5. `∧`
  6. `¬`
  7. `=` `≠` `<` `≤` `>` `≥` `∈` `∉` `⊆`, which chain as mathematics does (`0 ≤ i < n` is
     `0 ≤ i ∧ i < n`)
  8. `∪` `\`
  9. `∩`
  10. `..`
  11. `+` `-` `∘`
  12. `*` `÷` `%`
  13. unary `-`
  14. `×`
  15. application `f(x)`, indexing `t[i]`, qualification `a.b`

  The keyword forms (`IF`, `CASE`, `LET`, `LOOP`, `LOCAL`, `λ`, `∀`) extend as far right as they can,
  as TLA+'s do, and parentheses close them. The formatter parenthesizes wherever `∧` meets `∨` or
  `×` meets `∪`, so a reader never needs the table for those.

## 5. Two programs, before and after

### 5.1 `count`, today (`examples/io/count.oro`, its code)

```lisp
(def space? (b) (or (= b 32) (and (>= b 9) (<= b 13))))

(def main ()
  (let av (os.Args)
    (if (< (len av) 2)
        (seq (io.print-line "usage: count FILE") 0)
        (case (os.ReadFile (av 1))
          (result.err k) (seq (io.print-line "count: cannot read that file") 0)
          (result.ok src)
          (if (>= (len src) cap-in)
              (seq (io.print-line "count: file is larger than this tool accepts") 0)
              (local lines   0
                     words   0
                     in-word false
                (loop ((i 0))
                  (>= i (len src))
                    (seq (io.print-int lines) (io.print-int words) (io.print-int (len src)))
                  else
                    (let b (src i)
                      (seq (if (and (= b 10) (< lines cap-in)) (set lines (+ lines 1)) (tuple))
                           (cond
                             (space? b)       (set in-word false)
                             in-word          (tuple)
                             (< words cap-in) (seq (set in-word true) (set words (+ words 1)))
                             else             (tuple))
                           (again (+ i 1)))))))))))
```

### 5.2 `count`, proposed

```
space(b) == b = 32 ∨ b ∈ 9..13

main ==
  LET av == os.Args() IN
  IF Len(av) < 2 THEN (io.print_line("usage: count FILE"); 0)
  ELSE CASE os.ReadFile(av[1]) OF
      result.err(k) → io.print_line("count: cannot read that file"); 0
   [] result.ok(src) →
      IF Len(src) ≥ cap_in THEN (io.print_line("count: file is larger than this tool accepts"); 0)
      ELSE
        LOCAL lines := 0, words := 0, in_word := FALSE IN
        LOOP i := 0 :
            i ≥ Len(src) → io.print_int(lines); io.print_int(words); io.print_int(Len(src))
         [] OTHER →
              LET b == src[i] IN
              (IF b = 10 ∧ lines < cap_in THEN lines := lines + 1);
              (CASE space(b)       → in_word := FALSE
                 [] in_word        → ()
                 [] words < cap_in → in_word := TRUE; words := words + 1
                 [] OTHER          → ());
              AGAIN(i + 1)
```

`IF` without `ELSE` is a statement, whose omitted branch is `()`. That is the one statement form
`IF` gains.

### 5.3 `ages`, proposed (today's in `examples/lines/ages.oro`)

```
run ==
  LET in == bufio.NewScanner(os.Stdin()) IN
  LOCAL n := 0, sum := 0, best_name := "", best_age := -1, skipped := 0 IN
  LOOP k := 0 :
      ¬Scanner.Scan(in) → report(in, n, sum, best_name, best_age, skipped)
   [] k ≥ cap_lines     → fmt.Println("ages: more than 16777216 lines"); 1
   [] OTHER →
        LET (name, age, got, e) == fmt.Sscan(Scanner.Text(in), OUT String, OUT Int) IN
        (IF ok(e) ∧ age ∈ 0..cap_age ∧ n < cap_lines
         THEN (n := n + 1; sum := sum + age;
               IF age > best_age THEN (best_name := name; best_age := age))
         ELSE IF skipped < cap_lines THEN skipped := skipped + 1);
        AGAIN(k + 1)
```

### 5.4 Types as sets, and the climb

```
Number  == Int ∪ F64
Method  == {"GET", "POST", "PUT"}
NonZero == Int \ {0}

show(v ∈ Int ∪ F64 ∪ String) ∈ String ==
  CASE v OF i ∈ Int → render(i) [] f ∈ F64 → format(f) [] s ∈ String → s

total(a ∈ Seq(0..1000)) ∈ Nat
  ENSURE RESULT = sum(a, 0, Len(a))
  ==
  LOOP i := 0, s := 0  INVARIANT i ≤ Len(a) ∧ s = sum(a, 0, i) :
      i ≥ Len(a) → s
   [] OTHER      → AGAIN(i + 1, s + a[i])
```

`total`'s claim needs `sum_snoc` (§4.7) for the invariant's preservation. Each obligation (entry,
preservation, exit) is one decided query once the lemma is in hand.

## 6. What we keep, change, and remove

### 6.1 Kept: the semantics, all of it

- the atom, staging, and β/δ with the effect discipline;
- every data form, including its representation (ADR 0042) and its linearity (ADR 0018);
- `loop`/`again`, local state, the error model, relevance;
- modules as host paths (ADR 0025), targets as theories and models (ADR 0021);
- host declarations by hand (ADR 0022), the IR and its text (ADR 0032, ADR 0038), every analysis.

Every ADR's decision stands. Only the surface sections of some change spelling: ADR 0015 (loop),
0017 (booleans), 0024 (comments, needing a successor), 0031 and 0047 (scopes), 0040 to 0046 (errors,
variadics).

### 6.2 Changed: the spelling, word by word

| today | proposed |
|---|---|
| `(def f (x) e)`, `(sig f ((x T)) R)` | `f(x ∈ T) ∈ R == e` |
| `fn`, `λ` | `λ x : e`, `LAMBDA` |
| `let` | `LET … IN` |
| `seq` | `;` |
| `if` | `IF … THEN … ELSE` |
| `and`, `or`, `not` | `∧`, `∨`, `¬` |
| `case`, `cond`, `match` as a choice | `CASE` |
| `loop`, `again`, `else` in a loop | `LOOP … :`, `AGAIN(…)`, `OTHER` |
| `local` | `LOCAL … IN` |
| `(set c v)`, `(set b i v)` | `c := v`, `b[i] := v` |
| `tuple`, `(tuple)` | `(a, b)`, `()` |
| `(array a b c)` | `⟨a, b, c⟩` |
| `(table n f)` | `[i ∈ 0..n-1 ↦ f(i)]` |
| `(a i)`, `len` | `a[i]`, `Len(a)` |
| `(map …)`, `build-map`, `insert`, `keys` | `[k ∈ K ↦ e]`, `f[k] := v` in a scope, `DOMAIN` |
| `concat`, `string-of` | `∘`, `render` |
| `(int LO HI)`, `+inf`, `pow` | `LO..HI`, `+∞`, `^` |
| `int`, `f64`, `bool`, `string` | `Int`, `F64`, `Bool`, `String`, prelude definitions |
| `(array T)`, `(buffer T)`, `(cell T)` | `Seq(T)`, `Buffer(T)`, `Cell(T)` |
| `variant`, `success`, `relevant` | `VARIANT … == c₁(A) \| c₂(B)`, `SUCCESS`, `RELEVANT` |
| `option`, `some`, `none` | `Option(T)`, `some`, `none` |
| `where`, `ensures`, `result` | `WHERE`, `ENSURE`, `RESULT` |
| `try`, `expect`, `ignore` | `TRY`, `EXPECT`, `IGNORE` |
| `out`, `spread` | `OUT`, `xs...` (the host's spelling of a spread) |
| `module`, `use`, `as`, `export` | `MODULE`, `USE`, `AS`, `EXPORT` |
| `true`, `false` | `TRUE`, `FALSE` |
| `=`, `<=`, `>=` | `=`, `≤`, `≥` |
| `/`, `%` | `÷`, `%` |
| `;` comments | `\*` and `(* … *)` |
| target `(sig … (host KIND "…"))`, `pure`, `variadic`, `import` | `… == HOST KIND "…" IMPORT "…"`, `PURE`, `VARIADIC` |
| target `type`, `repr`, `fact`, `implements` | `TYPE`, `REPR … AS`, `FACT`, `S ⊆ T` |

### 6.3 Removed

- **S-expressions as the surface.** The IR's text keeps them.
- **`seq`**, which becomes `;`.
- **`cond` and `match` as words**, which become `CASE` and `LOOP`.
- **`the`**, the ascription, which becomes a claim, `ASSERT e ∈ S`, or a definition's result set.
- **`sig` as a separate form**, whose claims move into the definition.
- **Hyphens in names**, and the `?` and `!` suffixes, which become `_`.
- **The threaded buffer idiom**, `(again (set b i v) …)`, which becomes a write.
- **The buffer-returning host templates**, which become plain calls (§4.5).
- **`loop2`, `fold-range`**, the retired portable layer's kinds, and **`alloc`** where `[i ∈ D ↦ e]`
  covers it.

### 6.4 Added, by the algebra

- the set operators `∪ ∩ \ ⊆ ×` and comprehension;
- enumerations, and literal singletons;
- `∀`, `∃`;
- membership as a test, and `CASE` on sorts;
- `ASSERT`, `INVARIANT`, `RECURSIVE`, `LEMMA`, `THEOREM`, `PROOF BY`;
- `DOMAIN` and `EXCEPT`;
- the record notation, reserved until a program needs records.

## 7. What a notation change does not change

- **The analyses and the IR** see nothing new. The reader produces the same core terms, so every
  emitted file is byte-identical through the change. That is the acceptance test.
- **The target model**: a target file is still a theory, and only its spelling changes.
- **The book, the specifications and the results**: their code blocks are translated by the same
  tool, and the documents' text is reread where it quotes the old words.

## 8. The translator, and the order

**The translator is the reader and a printer.** Today's reader already parses every `.oro` file into
forms, and the gaps (comments) are tracked by the lexer. A printer from core forms to the new notation,
carrying each gap to its place, converts a file mechanically. That is the rule CLAUDE.md states for
every rewriter: "preserves every gap or refuses the edit". Its acceptance test is the gate:
- every converted program, read by the new reader, gives the same core terms;
- so `cmd/check` stays byte-identical.

The order:
1. **The new reader**, beside the old, chosen by file. Its grammar is §4, and its output the same
   core terms.
2. **The printer**: old forms to new text, comments carried. It is checked by a round trip on every
   file: parse old, print new, parse new, and the core terms are equal.
3. **The conversion**:
   - the corpus, the targets, the libraries, the tests' embedded source and the documents' code
     blocks;
   - the inventory test, rewritten for the new words;
   - the old reader deleted.
4. **The two unifications that change meaning**, each with its own result document and ADR: buffers
   written in program order, and tables and maps as one finite function.
5. **settypes-research's §8**, built in the new notation.

**The cost.** The reader is 2,819 lines today, and a precedence parser with the keyword forms is of
that order. The printer is smaller. The conversion is mechanical but large: 200 files, 76 test files,
628 code blocks. The earlier this is done, the less there is to convert. A notation change after
settypes is built would convert its constructs twice.

## 9. Risks

- **Precedence bugs.** A misparsed formula is a wrong program. The mitigations are a small table, the
  formatter's parentheses where families meet, and the round trip.
- **Unicode input.** Every symbol has an ASCII spelling, and the formatter chooses the canonical one.
- **Bikeshedding.** A notation can absorb unlimited attention. This research fixes the principles
  (§1.2) so that a word's choice follows from them rather than from taste.
- **The balance.** This is mostly not compiler growth, since the reader is replaced rather than
  added, but it is a large piece of work before the next program. It is cheaper now than later.

## 10. Decisions for hamza

1. **The mathematical notation (B)** rather than renamed S-expressions (A). *Recommended.*
2. **Reserved words in capitals**, as TLA+ has them. *Recommended*: they cannot collide with a
   program's names or a host's.
3. **Unicode and ASCII both accepted**, with a formatter that prints one canonical form: Unicode in
   documents, and a choice for source files. *Recommended*: Unicode canonical, since it reads as the
   mathematics, with ASCII always accepted.
4. **The five unifications** of §0.4:
   - one `CASE`;
   - buffers written in program order, and host write-borrows as writes;
   - tables and maps as one finite function;
   - claims in the definition's header, with `sig` removed;
   - membership as a test.

   *Recommended*, the second and third each with its own ADR and result document when built.
5. **The order of §8**: translator first, then conversion, then settypes in the new notation.
   *Recommended.*
6. **Names in snake_case**, since `-` becomes subtraction. *Recommended.*
