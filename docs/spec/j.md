# J — the language's surface

**Status, 2026-10-25: built, the computation's forms; the logic's not.** J is the surface
([ADR 0068](../decisions/0068-j-is-the-surface.md)) and the only notation: J's reader (`core/jlex.go`,
`core/jparse.go`, `core/jreadtarget.go`) reads all 223 `.oro` files of the corpus, the tests' snippets and
the surveys' output (jswitch5b-2026-10-16). What is built and what is not, read off the reader by a probe
of each form through desugaring (2026-10-25):

| | built | not built |
|---|---|---|
| §1 lexical | NFC, bidi refused, gaps, every token class, §1.6's symbols with their ASCII, §1.7's line rule | **§1.4's reservation**: a word is a keyword only where the grammar takes it, so `spec`, `set`, `map`, `export` and others are accepted as names |
| §2.1–2.3 files | `use … as`, `module`, `provides`, companions, `private`, `export`, `axiom` | |
| §2.2 declarations | `fn`, `set` (variants with `success` and `relevant`), a value, a host constant | **`spec` and `lemma`** (ADR 0062) |
| §2.4 headers | binders with sets and modes, a telescope, the three result forms, `where`, `ensure`, `pure`, `index`, `variadic`; `x'` in a host result (ADR 0070) | **a ⊆-binder in a program** (built in a target file's host declaration, `errors.AsType(E ⊆ error)`); **the arrow** as a binder's set (ADR 0053); **`x'` in a program's `ensure`**, read and then refused as outside what the compiler decides |
| §2.6 blocks | `=`, `var … :=`, writes, concurrent (G5) and into a record (G6), a binder's claim, `expect`, a one-armed `if` | **`assert`**; **a local named `fn`** |
| §2.7 expressions | application, indexing, tags, tuples, `++`, `!=`, `out S` (G1), `xs...` (G2), `fn(a, b) { e }` | **a λ's header**: a claimed parameter or result on an anonymous `fn`; **membership `e ∈ S` as a test** (refused, and its message is garbled: "`` is not imported") |
| §2.8 choice and iteration | `case` with and without a scrutinee, a pattern a tag with one binder per payload, or `else`; `loop`, `again`; a case in a tail (ADR 0076); `try` without a handler | **a pattern that is a literal, `_` or `x ∈ S`**; **a case arm's guard `p if φ`**; **`invariant`**; **`try`'s handlers** (`else`, ADR 0057) |
| §2.9 sets | names, ranges, `Array`, `Map`, `∪`, `×`, `array { … }` both shapes, `map { k : v }`, `map(cap)` | **`set { x ∈ S : φ }` and `set { e : x ∈ S }`** (ADR 0061) |
| §2.10 formulas | the computation's boolean expressions in `where` and `ensure` | **`∀`, `∃`, `⇒`, `⇔`** |
| §6 printing | §6.1, show_J for messages | **print_J**, the lossless printer: it was the translator's and went with it |

Not built either: static argument inference (ADR 0066 item 2). Each waits for a program that needs it.
§7's translator is deleted (step 5b); its transitional spellings are still read and no corpus file uses
them (§7).

This document is its one grammar, consolidated from the decisions of surface-design.md's eight
installments, ADRs 0050–0067. It specifies **R_J**, a rendering of [abstract.md](abstract.md) in that
document's sense (§9): a language L_J ⊂ Σ*, and maps

```
parse_J : L_J ⇀ CST_J        print_J : CST_J → L_J        abs_J : CST_J → A
```

where a concrete tree is an abstract tree decorated with layout and gaps (comments), satisfying

```
(R1)  parse_J(print_J(c)) = c          print is lossless: layout, gaps and spelling survive
(R2)  abs_J(parse_J(s)) is defined and well formed (abstract.md §3) for every s ∈ L_J
```

The semantics is abstract.md's and is not restated: this document says how each operator of Σ is
written, and nothing about what it means. Where a form is new to the abstract syntax (a nested static
λ, a primed name, a named splice), the ADR that decided it is named.

---

## 0. Sources, conflicts and gaps

### 0.1 What each decision fixes

| ADR | fixes in J |
|---|---|
| [0050](../decisions/0050-the-abstract-syntax-fits-the-semantics.md) | modes on binders; one definition with its claims; sets formed explicitly (amended, §0.2 C1); pairs; one guarded choice; indexing marked |
| [0051](../decisions/0051-a-definition-is-a-named-lambda.md) | a definition is a named λ; the result is one binder, its pattern possibly a tuple |
| [0052](../decisions/0052-a-buffer-is-a-state-binder-holding-a-table.md), [0058](../decisions/0058-a-map-buffer-is-a-state-binder-holding-a-map.md) | a buffer and a map buffer are state binders, written in program order: `b[i] := v`, `m[k] := v`; R0–R4 |
| [0053](../decisions/0053-the-arrow-is-a-claim-on-a-host-parameter.md) | the arrow is a claim used as a set |
| [0054](../decisions/0054-let-and-local-are-one-scope.md), [0055](../decisions/0055-a-loop-variable-is-bound-by-application.md) | one scope, its binders definitional or state; J writes no head; a loop's variables are definitional |
| [0056](../decisions/0056-the-surface-is-d-or-j-and-its-choice-and-iteration.md) | one arm grammar; `again` positional; `var x := e`; a one-armed `if` |
| [0057](../decisions/0057-the-handled-bind-and-its-tail.md) | `try p = e [else arms], … { body } [else arms]`; `expect p = e else w` |
| [0059](../decisions/0059-a-tag-is-a-quoted-symbol-and-collections-have-one-former.md), [0061](../decisions/0061-separation-and-replacement-by-the-binders-place.md) | tags `'name`; one former shape `former { … }`; `Array(V)`, `Map(K, V)`; separation and replacement by the binder's place; a map's graph `map { k : v }` |
| [0060](../decisions/0060-j-set-operators-and-qualified-tags.md) | membership `∈`; an imported tag qualified `alias.'tag`; no `exposing` |
| [0062](../decisions/0062-a-header-is-a-membership-statement-and-the-logic-has-three-definitions.md) | the header as a membership statement; telescopes; one equality; `fn`, `spec`, `lemma`; `assert` and `expect`; `the` gone |
| [0063](../decisions/0063-a-type-is-a-static-value-and-instantiation-is-application.md), [0066](../decisions/0066-a-definition-over-sets-is-a-lambda-returning-a-lambda.md) | a definition over sets is a λ returning a λ, `E ⊆ S`; inference; the arrow's header; `x'`; named splices `${x}`; Code_T |
| [0064](../decisions/0064-a-file-is-a-theory-and-a-declaration-is-a-header-and-one-body.md) | files, modules, fragments; imports; `private` and `export`; one declaration form; `set`; variants as unions of tagged sets; `axiom` |
| [0065](../decisions/0065-j-writes-sets-in-tla-symbols.md), [0067](../decisions/0067-the-lexical-level.md) | TLA+'s set and logic symbols with TLA+'s ASCII; the lexical level; precedence |

### 0.2 Conflicts between the decisions, and their resolution

The installments were written over five days, each binding on the next, and consolidating them found six
places where a later decision changed an earlier one without saying so, and the translator a seventh. **The later decision governs**,
as ADR 0064's opening rule of the surface ("what comes after has to fit with what we pick now") makes
the earlier ones binding only until revised. **Confirmed by hamza, 2026-10-15** ("agree with all").

| | earlier | later | resolution in J |
|---|---|---|---|
| **C1** | ADR 0050 item 4: "a literal is never its own singleton by position" | ADR 0064 item 8: "in a set position a value denotes its singleton" (`'not_found ∪ 'exists`), decided with its derivation (the powerset monad's unit, coherent by naturality) | **ADR 0064**: in a set position (§2.9) a value denotes its singleton. The explicit former, `set { 'not_found }`, stays legal and denotes the same set. |
| **C2** | ADR 0054 item 3: "K and J write none" (no scope head); installment 2's J is "statements + braces", and `let … { }` was candidate L, not kept | ADR 0064 item 7: `let` "stays the head of a scope with a body (ADR 0054)"; ADR 0067 item 2 lists `let` as reserved | **ADR 0054**: J has no `let`. A scope is a block of statements (§2.6). 0064's sentence misread 0054; `let` leaves 0067's reserved list. |
| **C3** | ADR 0052 and 0047: a buffer from `table(n, 0)`, a copy `table(len(xs), xs)`; ADR 0056 item 8: `array(n, f)` | ADR 0059 item 3: one former shape, `array(n, f)` gone; ADR 0061 item 3: `array { e : k ∈ 0..n-1 }` | **ADR 0059/0061**: `var b := array { 0 : _ ∈ 0..n-1 }`; a copy is `array { xs[k] : k ∈ 0..len(xs)-1 }`. `table` is no word of J. |
| **C4** | ADR 0056 item 9: the set of tables is `Seq(S)` | ADR 0059 item 4: `Array(V)` | **ADR 0059**. |
| **C5** | ADR 0059 item 3 and installment 4: a map's pair `k ↦ v` | ADR 0061 items 4–5: `map { k : v }`, the colon a pair's separator in a map | **ADR 0061**. |
| **C6** | ADR 0060 item 2 (Python's set operators); ADR 0062 item 4 (`forall`, `exists`, `==>`, `<==>`) | ADR 0065, ADR 0067 item 7 | already superseded, each by name. |
| **C7** | ADR 0064 item 6: a host constant `MaxRune ∈ {1114111} = host …`, written with bare braces | ADR 0065 item 4: a set former always has its keyword, and braces with none are a block | **ADR 0065**: `MaxRune ∈ set { 1114111 } = host "utf8.MaxRune" …` (found by the translator, step 2d). |

### 0.3 Gaps: forms no decision fixed

Writing the grammar found three operators of Σ with no decided J spelling. This document proposed one
for each; **decided by hamza, 2026-10-15** ("agree with all"):

| | operator | where it occurs | proposed |
|---|---|---|---|
| **G1** | out(S), a fresh cell at an `out` parameter (ADR 0048 §6) | 37 `(out τ)` sites in the corpus | **decided:** `out Int` as an argument, the mode word before the set: `fmt.Sscan(s, out Int, out String)`. Ada writes `out` on the parameter and nothing at the call; the call needs the set, since the cell is made there. |
| **G2** | spread(e), a table passed as a variadic list (ADR 0046) | `(spread xs)` | **decided:** `xs...` as the last argument, Go's own spelling of exactly this call: `fmt.Println(xs...)`. `...` is one token, so `..` and `...` never meet (§1.6). |
| **G4** | a number unique to each use of a template, x86's labels, today's `%u` (30 sites) | `targets/windows/x64.oro` | **decided** (hamza, 2026-10-15): `${#}`, a splice no binder can be named, since `#` starts no identifier: `jno Lok${#}`. Found by the translator, step 2d. |
| **G5** | a buffer moved between places, today's `(again (mpass s a b w) a (* 2 w))`: two buffers swap each pass | 3 loops (merge-sort, freq, tally; gauntlet/surface/stores.go) | **decided** (hamza, 2026-10-15): the concurrent assignment `a, b := b, a`, Dijkstra's (1975), every right side in the state before. ADR 0052's R0 admits only a fresh table, so as written it refuses the three; a permutation of places keeps each table at one place, which is what R0 protects. Found building ADR 0052. |
| **G6** | a store into a record of a table of tuples, today's `(set (dt s) 1 v)` | 2 sites (freq, tally) | **decided** (hamza, 2026-10-15): `dt[s][1] := v`, the place extended to an index chain: the curried store is the point update at (s, 1) of the uncurried function [0,n) × Fin k → V. Found building ADR 0052. |
| **G7** | a growth call, `strconv.AppendInt(b, k, 10)`, written `b := f(b, …)` by ADR 0052's R0 | 7 growth calls, against 11 write-borrows | **decided** (hamza, 2026-10-15): one rule for every borrow: a call at a `linear` parameter rebinds the buffer, and its value is the rest of the declared result, so growth is the statement `strconv.AppendInt(b, k, 10)`, as `n = f.Read(buf)` is. Under ADR 0052's spelling, B ⊸ B would return the buffer and B ⊸ B ⊗ R would not, two readings of one shape. Found building ADR 0052. |
| **G3** | a lemma's proof steps beyond `assert` and `if` (ADR 0062 item 5) | installment 5's G4 only | **decided:** a lemma's body is a block whose statements are `assert` statements, `if`/`case` over formulas, and lemma uses `assert L(ā)`; nothing else is a proof step until a lemma needs one. |

---

## 1. Lexical structure

ADR 0067, unless named otherwise.

### 1.1 Characters

A source file is UTF-8 in Unicode Normalization Form C. **Bidirectional controls (U+202A–202E,
U+2066–2069) are refused anywhere**, comments and strings included: they make a text display otherwise
than it parses (Trojan Source, CVE-2021-42574). Line endings are LF.

### 1.2 Gaps

A gap is whitespace or a comment. **A comment is `//` to the end of the line**; there is no other kind.
A gap is never a token: the lexer erases it, and parse_J factors through Text/≡ where s ≡ s′ iff
tokens(s) = tokens(s′) (comments.md §1). The concrete tree keeps each gap on a position (abstract.md
§9.1), so print_J restores it and a translator transports it (§7).

### 1.3 Tokens

The lexer is maximal munch over these classes, in this order of precedence where two could start at
one character:

| class | spelling |
|---|---|
| **identifier** | UAX #31: a letter or `_`, then letters, digits, marks and `_`; optionally ending in one `?`. Case is significant for identity and carries no meaning (§1.5). |
| **quoted name** | `` `…` ``: any characters except a backquote, a newline and a bidirectional control, between backquotes. It names what an identifier cannot: a host operator, `` go.`>=` ``, a representation operation, `` `big+` ``. |
| **integer** | decimal digits. |
| **float** | digits `.` digits, optionally `e`/`E`, a sign and digits; or digits and an exponent: `0.5`, `1.5e-3`, `1e9`. A `.` not followed by a digit ends the number, so `0..n` is a range. |
| **string** | `"…"` with string-literals.md's six escapes, `\t \n \r \\ \"` and `\u{H…}`; an unknown escape is an error. A host template is a string (§2.11). |
| **tag** | `'` followed directly by an identifier: `'none`, `'other`. |
| **prime** | `'` directly after an identifier, with no space, and not followed by an identifier character: `x'`. |
| **symbol** | the operators and punctuation of §1.6. |

**The prime and the tag share `'`** and are told apart by one character before and one after: after an
identifier a `'` is a prime and may not be followed by an identifier character; anywhere else it begins
a tag and must be.

### 1.4 Words

**Reserved**, never an identifier: `fn spec lemma set array map var loop again case if else try expect
assert use module provides private export axiom true false`.

**Contextual**, keywords only in the clause positions the grammar gives them, identifiers everywhere
else: `where ensure invariant pure index variadic inout out linear as host expr stmt import lib checked
jump comma_ok success relevant` and the model's data words (`repr word niche narrow structural backend
link build artifact data include fact underlying`, §2.12).

**Read and refused**, each with a message naming what replaced it (inventory.md §4): `the`, the core's
ascription, refused for a binder's claim (ADR 0069); and the model's old words `max_len big_repr
int_repr`. And `_` is the wildcard (§1.5).

These three paragraphs are the reader's words: `TestJsKeywordsAreTheReadersKeywords`
(emit/inventory_test.go) reads every word a token is compared against in core/jparse.go and
core/jreadtarget.go, and requires the paragraphs to list that set, less the words not built (spec,
assert, invariant) and the two operands of repr the target loader reads (word, narrow). **Not built:
the reservation**: the reader takes a word as a keyword only where the grammar meets it (the status,
above).

**After `.`, any word is a member**, reserved ones included: `` java.List.set ``.

### 1.5 Spelling

A name of abstract.md's carrier 𝒩 is spelled by σ_J, which is injective per program (abstract.md §9.3):

- **`-` becomes `_`**, and a final `?` stays: `text-of` is `text_of`, `not-found?` is `not_found?`.
- **A name outside the identifier alphabet is quoted**: `` `+` ``, `` `big%-small` ``.
- **A name already holding `_` is quoted**, so that an unquoted `_` is always 𝒩's `-` and σ_J is
  invertible, not only injective: the reader's σ_J⁻¹ turns `_` back into `-` (step 3, jread-2026-10-15).
  No program name holds one; four host names do (`` `ERROR_FILE_NOT_FOUND` `` and its kin). The
  wildcard `_` is itself.
- **A program whose spelling collides** (`best-name` and `best_name`) is refused by the translator,
  naming the pair.

**Conventions**, which carry no meaning (the reader reads nothing from case): a set's name is
UpperCamelCase (`Int`, `Bool`, `F64`, `String`, `Array`, `Map`, `Option`, `result.Result`); values and
functions are snake_case; a host's names are spelled as the host spells them (`error`, `DirEntry`,
`ReadFile`).

### 1.6 Symbols

Each mathematical symbol has one ASCII spelling, TLA+'s; the file stores the symbol, the reader accepts
both, and print_J writes the symbol (ADR 0065).

| symbol | ASCII | | symbol | ASCII |
|---|---|---|---|---|
| `∈` `∉` | `\in` `\notin` | | `∀` `∃` | `\A` `\E` |
| `∪` `∩` `∖` | `\cup` `\cap` `\` | | `⇒` `⇔` | `=>` `<=>` |
| `⊆` `⊂` | `\subseteq` `\subset` | | `×` | `\X` |

The ASCII operators and punctuation:

```
+  -  *  /  %  ^  ++          arithmetic, concatenation, constant power
==  !=  <  <=  >  >=           comparison (no chains)
&&  ||  !                      the computation's connectives
=  :=  ->  :  ,  ;  .  ..  ... binding, writing, arms, formers, separators, member, range, spread
(  )  [  ]  {  }               application and grouping, indexing, blocks and formers
```

Maximal munch decides `...` before `..` before `.`, `:=` before `:`, `==` before `=`, `<=>` before
`<=`, `=>` before `=`, and `->` before `-`. A backslash begins only a TLA+ ASCII name or set difference
`\` followed by a space.

### 1.7 Lines

**A newline ends a statement, unless it cannot** (Scala 3's rule): not inside `( )` or `[ ]`; not after a
token that cannot end an expression (a binary operator, `,`, `=`, `:=`, `->`, `:`, `(`, `[`, `{`); not
before a token that cannot begin a statement (`where`, `ensure`, `invariant`, `else`, `)`, `]`, `}`, a
binary operator, `->`). `;` separates statements on one line. Braces do not suppress newlines: a block
holds statements.

---

## 2. Grammar

EBNF, with `X?` optional, `X*` and `X+` repetition, `X ("," X)*` written `X,+`. Terminals are quoted;
`NL` is a statement-ending newline or `;` (§1.7).

### 2.1 A file

```
file        ::= item*
item        ::= use | module | provides | declaration | axiom | model
use         ::= "use" path ("as" ident)?
path        ::= ident ("/" ident)*
module      ::= "module" path "{" item* "}"
provides    ::= "provides" ident path "{" item* "}"
```

- **A library or entry file is one theory, named by its path**: it has no header (ADR 0064 item 1). A
  `module` inside it is a child: a companion (§2.3) or an entry file's further module.
- **A target file is a set of fragments**: its outermost `module` names its module by absolute path, and
  the target is its directory; elsewhere a fragment is `provides TARGET PATH { … }` (ADR 0064 item 2).
- **`use p` binds p's last segment; `use p as a` binds a.** The tree always carries the alias; print_J
  omits one equal to the default (ADR 0064 item 3). In `use`, `/` separates path segments.

### 2.2 Declarations

```
declaration ::= visibility? ( fn_decl | spec_decl | lemma_decl | set_decl | value_decl )
visibility  ::= "private" | "export"
fn_decl     ::= "fn" name params result? clause* body
spec_decl   ::= "spec" name params result? clause* body
lemma_decl  ::= "lemma" name params clause* block
set_decl    ::= "set" name params? "=" expr marker*
value_decl  ::= binder "=" ( expr | host_body )
marker      ::= "success" tag | "relevant"
axiom       ::= "axiom" expr
name        ::= ident | quoted
```

- **`private` hides a name; public is the default**; a private set hides its tags (ADR 0064 item 4).
- **`export` marks an entry point**: in the entry file the build compiles it into a host function, and in
  a file reached by `use` it is ignored (ADR 0064 item 5).
- **`fn f(…) … { e }` is the tree of `f = fn(…) … { e }`**, def(f; λ); print_J writes the named form
  (ADR 0064 item 7).
- **`set S = e` is `S = e` with the claim S ∈ subsets(any)**, optional except on a host body, where it
  makes the body a carrier; `set F(T, …) = e` is a set-valued function, `fn F(T, …) (subsets(any)) { e }`
  (ADR 0064 item 7). A **variant** is a set definition whose right side is a union of tagged sets, its
  markers after it: `set Result(T, E) = 'ok(T) ∪ 'err(E) success 'ok relevant` (item 8).
- **A value**: `cap = 4096`, `C = 2^62 - 1`; a host constant's value is its claim,
  `MaxRune ∈ set { 1114111 } = host "utf8.MaxRune" import "unicode/utf8"` (item 6; §0.2, C7).
- **`axiom`** states an inclusion, `axiom NumError ⊆ error`, `axiom File ⊆ Reader ∩ Writer`; only in a
  target file or a fragment (ADR 0064 item 9).

### 2.3 Companions

A child `module` whose name is a set's in its parent is that set's companion (theories.md §3.3): its
definitions are the set's operations, its constructor among them as `new` (ADR 0063).
`self` is an ordinary parameter name, conventional for the value an operation acts on. `alias.T.m`
names member m of T's companion: `fs.DirEntry.Name(ent)`.

### 2.4 Headers and binders

```
params      ::= "(" binder,* ")"
result      ::= "(" ")" | "(" set_expr ")" | "(" binder,+ ")"
clause      ::= "where" formula | "ensure" formula | "pure" | "index" | "variadic"
binder      ::= pattern ( "∈" set_expr | "⊆" set_expr )? mode?
mode        ::= "linear" | "inout" | "out"
arrow       ::= "fn" params result? clause*
```

- **The header is the membership statement** f ∈ Π(x̄ ∈ {x̄ ∈ S̄ : φ}). {r ∈ U : ψ} (ADR 0062 item 1):
  binders with their sets, the result, `where φ`, `ensure ψ`, then markers.
- **The result**: `()` the unit; `(U)` a set alone; `(r ∈ U)` a named result, which ψ may use; a group
  `(q ∈ Int, m ∈ 0..b-1)` the tuple pattern tuple(q, m) ∈ Int × 0..b−1. In the result position a lone
  name is a set; a named result needs `∈`.
- **A binder list is a telescope**: a binder's set may name the binders before it,
  `fn get(a ∈ Array(Int), i ∈ 0..len(a)-1)` (ADR 0062 item 2). A relation between binders is a `where`.
- **`x ∈ S` ranges over values, `E ⊆ S` over sets** (E ∈ 𝒫(S)), and a binder with neither claims
  nothing (ADR 0062 item 8; ADR 0066 item 1). Go's `[E any]` is `E ⊆ any`.
- **A mode follows the set**: `c ∈ Array(Bool) linear`, `x ∈ Array(E) inout`. `linear` only on a
  parameter (ADR 0052).
- **The arrow is a λ's header without its body**, used as a binder's set:
  `cmp ∈ fn(a ∈ elems(x), b ∈ elems(x)) (Int) pure` (ADR 0066 item 4).

### 2.5 Bodies

```
body        ::= block | "{" host_body "}"
host_body   ::= "host" ( "expr" | "stmt" )? string host_clause*
host_clause ::= "import" string+ | "lib" string | "checked" ident | "jump" string string? | "comma_ok"
```

A host body is a definition's body, in a target file or a fragment; it may sit only under ⊆-λs (ADR
0066 item 10). Its template is §2.11's. An operation's body names its kind, `expr` or `stmt`; a set's
carrier and a constant's spelling have none: `set NumError = host "*strconv.NumError"`.

### 2.6 Blocks and statements

```
block       ::= "{" ( statement NL )* expr? "}"
statement   ::= binder "=" expr                       a definitional binder
              | "var" ident ":=" expr                 a state binder: a cell, a buffer or a map buffer
              | place ("," place)* ":=" expr ("," expr)*   a write; several, concurrent (G5)
              | "fn" name params result? clause* body a local named λ
              | "expect" pattern "=" expr "else" expr
              | "assert" formula
              | expr
place       ::= ident ( "[" expr "]" )*                a whole value, an element, a record's component (G6)
```

- **A block is scope(b̄; e)** (ADR 0054): its binders are definitional (`=`) or state (`var … :=`), in one
  sequence, and its value is its last expression, or `()` when it ends in a statement. **There is no
  `let`** (§0.2, C2).
- **A definitional binder may rebind a name**, a nested scope by the monoid action's law; a state
  binder's name is bound once in its region (ADR 0054 item 2).
- **`var b := e` makes a buffer when e is a fresh table, a map buffer when e is `map(cap)`, and a cell
  otherwise** (ADR 0052, 0058); R0–R4 hold of a buffer and a map buffer. `b[i] := v` and `m[k] := v` write
  an element, `dt[s][1] := v` a component of a table of tuples' record, `x := e` the whole value
  (spec/local.md §2.1).
- **`a, b := b, a` is a concurrent assignment**: every right side is evaluated in the state before,
  then every place written (Dijkstra 1975). It is how a buffer moves between places: the moves form a
  permutation, so no table has two places (R0).
- **A call borrows a buffer passed at a `linear` parameter** and the buffer holds the callee's last
  value of it after the call; the call's value is the rest of the result. `strconv.AppendInt(b, k, 10)`
  is a statement, and `n = f.Read(buf)` binds the count (G7).
- **A binder may carry a claim**: `b ∈ 0..255 = src[i]` (ADR 0062 item 7).
- **`expect p = e else w`** abandons with w when e does not match p (ADR 0057 item 4); **`assert φ`**
  proves φ from the path's facts and adds it, and is erased (ADR 0062 item 6).

### 2.7 Expressions

```
expr        ::= literal | name | qualified | tag_expr | prime
              | "(" ")" | "(" expr ")" | "(" expr "," expr,+ ")"
              | expr "(" arg,* ")"                     application
              | expr "[" expr "]"                      indexing
              | lambda | former | "map" "(" expr ")"
              | if_expr | case_expr | loop_expr | try_expr | block
              | prefix expr | expr infix expr
              | quantified                             (in a claim position only)
qualified   ::= ident ( "." member )+
member      ::= ident | quoted | reserved_word
tag_expr    ::= ( ident "." )? tag ( "(" expr,* ")" )?
prime       ::= ident "'"                              (in an `ensure` only)
arg         ::= expr | expr "..." | "out" set_expr     (G2, G1)
literal     ::= integer | float | string | "true" | "false"
lambda      ::= "fn" params result? clause* block
if_expr     ::= "if" expr block ( "else" ( block | if_expr ) )?
```

- **Application is `f(x)`, indexing `t[i]`**, two operators (ADR 0050 item 6); a static application is an
  application, `errors.AsType(strconv.NumError)(err)`, and an inferable static argument may be omitted
  (ADR 0066 item 2).
- **A tuple is `(a, b)`, the unit `()`**; `(e)` is grouping. String concatenation is `++`.
- **A tag is `'name`**, an imported one `alias.'name`, a module's own and the language's unqualified
  (ADR 0059, 0060): `result.'err(e)`, `'some(v)`, `'none`.
- **A one-armed `if c { a }` is if(c; a; ())**, a statement whose value is the unit (ADR 0056 item 6).
- **A λ** `fn(a, b) { e }` written at a call takes its claim from the parameter it is passed to
  (ADR 0066 item 5).

### 2.8 Choice, iteration and the handled bind

```
case_expr   ::= "case" expr? "{" ( arm NL )+ "}"
arm         ::= pattern ( "if" expr )? "->" expr       with a scrutinee
              | expr "->" expr                         without one: a guard
              | "else" "->" expr
loop_expr   ::= "loop" ( ident "=" expr ),+ ( "invariant" formula )? "{" ( loop_arm NL )+ "}"
loop_arm    ::= ( expr | "else" ) "->" tail
tail        ::= expr | "again" "(" expr,* ")"
              | "{" ( statement NL )* tail "}"
              | try_expr
try_expr    ::= "try" t_bind,+ block ( "else" handler )?
t_bind      ::= pattern "=" expr ( "else" handler )?
handler     ::= arm | "{" ( arm NL )+ "}"
pattern     ::= "_" | ident | literal | ( ident "." )? tag ( "(" pattern,* ")" )?
              | "(" pattern "," pattern,+ ")" | ident "∈" set_expr
```

- **One arm grammar** for `case` (ADR 0056 item 2): with a scrutinee an arm is a pattern with an optional
  guard; with none, `case { … }` is the guarded choice and an arm is a guard. `else` is the last arm.
- **A loop's variables are definitional**, bound at entry and at each `again` by application (ADR 0055):
  `loop i = 0, s = 0 { … }`. Its arms are guards over them; `again(…)` is positional and appears only in
  a tail (abstract.md W3). An invariant is optional, `invariant I`, absent meaning ⊤.
- **A tail** is an expression, an `again`, a block whose statements end in a tail, a `try` whose body
  is a tail, or a `case` on a scrutinee whose arms are tails (ADR 0076): the tail sort is closed under a
  sum's eliminator, each arm running once. A `case` with an arm that jumps stands only where a tail may.
  A `try`'s handler arms leave the loop with their value (ADR 0057 item 5; by ADR 0076 item 5 they are
  tails too, when handlers are built). `again` is never under an `if` (ADR 0015): an `if`'s condition
  belongs in the loop's guards, and only a pattern's binders need the case.
- **`try p₁ = e₁ [else h₁], … { body } [else h]`** is the handled bind, a group of n binders n nested
  binds; with no handler a failure passes through re-built (ADR 0057 items 1–2).
- **A pattern's name always binds and a tag never does** (ADR 0059 item 1); `x ∈ S` binds x at S's sort.

### 2.9 Sets

```
set_expr    ::= expr                                   an expression whose value is a set
former      ::= "array" "{" ( expr,* | expr ":" binder ) "}"
              | "set" "{" ( expr,* | binder ":" formula | expr ":" binder,+ ) "}"
              | "map" "{" ( expr ":" expr ),* "}"
range       ::= expr ".." expr | expr ".." | ".." expr          an end omitted is unbounded (ADR 0077)
```

- **A set is a static value** (ADR 0063), so a set expression is an expression: a name (`Int`,
  `Array(E)`, `fs.DirEntry`), a range `0..255`, `-2^63..2^63-1`, an unbounded one `0..` or `..0`, a former, the set operators `∪ ∩ ∖ ×`,
  a set-valued application (`result.Result(FileInfo, error)`, `elems(x)`), or an arrow (§2.4).
- **A former always has its keyword**, and braces with none are a block (ADR 0065 item 4):
  `array { 2, 3, 5 }` a table's graph; `array { e : k ∈ 0..n-1 }` a table by a rule, the ordered image;
  `set { x ∈ S : φ }` separation, `set { e : x ∈ S }` replacement, by the binder's place (ADR 0061);
  `map { 1 : 10, 2 : 20 }` a map's graph; `map(cap)` an empty map buffer's value.
- **A set position** is a binder's set, the right side of `∈`, `∉`, `⊆` and `⊂`, an operand of `∪ ∩ ∖ ×`,
  and a `set` definition's right side. **There a value denotes its singleton**, η(v) = {v}, and a tag
  applied to a set its image, 𝒫(c)(S) = {c(s) : s ∈ S} (ADR 0064 item 8; §0.2, C1):
  `'not_found ∪ 'other(error)`.

### 2.10 Formulas

```
formula     ::= expr                                   a boolean expression
              | quantified | formula "⇒" formula | formula "⇔" formula
quantified  ::= ( "∀" | "∃" ) binder,+ ":" formula
```

A formula's own forms (quantifiers, `⇒`, `⇔`, membership in a set the computation cannot decide, a
`spec`'s call, `==` at a sort the computation cannot compare, and a primed name) stand only in a claim
position: `where`, `ensure`, `invariant`, `assert`, a `spec`'s or `lemma`'s body, and a `set { … : φ }`
the program does not compute (abstract.md W6). `==` is the logic's ≐ in a claim and the computation's
equality elsewhere (ADR 0062 item 3). `&&`, `||` and `!` serve both. `a != b` is derived,
`!(a == b)`, and erased by the reader as `!` is (shape-2026-10-21: until then it read as an
application of a name `!=` that nothing binds).

### 2.11 Host templates

A template is a string whose holes are **named splices** (ADR 0066 items 9–10):

- `${x}` splices the binder x in scope: a value parameter by its argument's host code, a ⊆-binder by its
  host spelling in that position together with its imports, the result binder by its destination;
- `${#}` is a number unique to each use of the template, for a label (G4);
- a register view is a suffix, `${r:b}`, `${r:e}`;
- only `${` opens a splice; a lone `$` is text, and a literal `${` is `$${`.

The template denotes a concatenation in Code_T = Σ* × 𝒫(Requirement): text concatenated, requirements
united, rendered into the file once.

### 2.12 The model's data

In a target file or a fragment only, the model's declarations keep their words, with the notation's sets
(ADR 0064 item 10): `repr 0..2^8-1 host "byte"`, `repr -2^63..2^63-1 word`, `repr error niche host expr
"%s == nil"`, `backend go`, `link "kernel32"`, `structural NAME KIND`, `fact …`, `underlying T 0..255`,
`max_len N`, `big_repr host`, `int_repr …`. Their grammar is target-files.md §1's, each form's operands
written as J expressions; this document does not restate it. **A template in the model's data keeps
target-files.md's positional holes**, `%s`: the model's forms bind no names to splice, so §2.11's named
splices are a host declaration's. A name another module of the model declares is written with its path
quoted, σ_J's fallback for what is no identifier: `` axiom File ⊆ `go/io`.Reader ``; a name in the
fragment's own module is written bare.

---

## 3. Precedence

Highest first (ADR 0067 item 8):

| level | operators | associativity |
|---|---|---|
| 1 | application `f(…)`, indexing `t[i]`, prime `x'`, member `a.b` | left |
| 2 | `^` (static integers only) | right |
| 3 | prefix `-`, `!` | — |
| 4 | `*` `/` `%` | left |
| 5 | `+` `-` `++` | left |
| 6 | `..` | none |
| 7 | `×` | none |
| 8 | `∪` `∩` `∖`, one level, mixed only in parentheses | — |
| 9 | `==` `!=` `<` `<=` `>` `>=` `∈` `∉` `⊆` `⊂` | none: no chains |
| 10 | `&&` | left |
| 11 | `\|\|` | left |
| 12 | `⇒` | right |
| 13 | `⇔` | none |
| 14 | `∀ … :` and `∃ … :`, their body as far right as it can extend | — |

`=`, `:=`, `->`, `:` and `,` are not operators: they belong to statements, arms, formers and lists.

---

## 4. From J to the abstract syntax

abs_J, operator by operator (abstract.md §2):

| J | Σ |
|---|---|
| `x`, `42`, `"s"`, `true` | variable, literal |
| `fn(b̄) (r ∈ U) where φ ensure ψ { e }` | lam(b̄; r ∈ U; φ; ψ; e) |
| `fn f(…) … { e }`, `f = …` at a module's level | def(f; e) |
| `fn f(E ⊆ B) { fn(…) … { e } }` | def(f; lam(E ∈ subsets(B); lam(…; e))) |
| `f(a, b)`, `f(T)(x)` | app(f; a, b), app(app(f; T); x) |
| `t[i]` | index(t; i) |
| `{ x = e; var y := e′; b }` | scope(x = e, y := e′; b) (ADR 0054; today let and local) |
| `y := e`, `b[i] := v` | assign(y; e), assign(b[i]; v) |
| `case e { p if φ -> a; else -> c }` | case(e; p if φ → a, _ → c) |
| `case { φ -> a; else -> c }` | case(tuple(); _ if φ → a, _ → c), cond's equation |
| `if c { a } else { b }`, `if c { a }` | if(c; a; b), if(c; a; tuple()) |
| `loop i = 0 invariant I { φ -> t; else -> t′ }` | loop(i = 0; I; _ if φ → t, _ → t′) |
| `again(a, b)` | again(a, b) |
| `try p = e else h { b }` | bind(p; e; h; b) (ADR 0057) |
| `expect p = e else w` | expect(p; e; w; …) over the rest of the block |
| `set { x ∈ S : φ }`, `set { e : x ∈ S }` | compr(x ∈ S; φ), image(x ∈ S; e) |
| `e ∈ S` in a computation, in a claim | mem(e; S), the formula e ∈ S |
| `assert φ` | assert(φ; rest of the block) |
| `b ∈ S = e` | the binder b ∈ S over e, a claim: an obligation (a range marked as a parameter's range is, ADR 0028), proven or refused, then the(S; e) the IR reads; built (ADR 0069). `the` itself is not J |
| `(a, b)`, `()` | tuple(a, b), tuple() |
| `'c(e)`, `alias.'c(e)` | the tag c applied, resolved in the module that owns it |
| `array { 2, 3 }`, `array { e : k ∈ 0..n-1 }` | Σ₀'s `array(2, 3)`, `table(n; λk. e)` |
| `map { 1 : 10 }`, `map(cap)` | Σ₀'s `map` graph, an empty map of capacity cap |
| `∀ x ∈ S : φ`, `φ ⇒ ψ` | the formulas ∀ x ∈ S. φ, φ ⇒ ψ |
| `x'` in `ensure` | the inout binder x's value on exit (ADR 0066 item 7) |
| `set S = e` | def(S; e) with the claim S ∈ subsets(any) |
| `set S = 'a ∪ 'c(T) success 'a relevant` | variant(S; ; a(), c(T); success a; relevant) |
| `{ host expr "t" import "p" }` | host(expr; t; import p) |
| `use p as a`, `module p { … }`, `provides T p { … }` | use(p; a), module(p; …), provides(T; p; …) |
| `private`, `export` | the reduct's complement, an entry point (ADR 0064 items 4–5) |
| `axiom T ⊆ I` | the model's inclusion axiom (Σ_T) |
| `f(xs...)`, `f(out S)` | app(f; spread(xs)), app(f; out(S)) (G1, G2) |

The operators abstract.md derives (if, cond, seq, the connectives, try, expect, bind) are written as
their derived forms; the equation is applied by abs_J.

---

## 5. Well-formedness that belongs to J

abstract.md §3's conditions W1–W11 apply to abs_J's image. These are added, each checkable on the
concrete tree:

| | condition |
|---|---|
| **J1** | `again` appears only in a loop arm's tail (W3's sort, read off the grammar of §2.8). |
| **J2** | A primed name `x'` appears only in an `ensure`, and x is an `inout` or `linear` binder of that header; and, in a host declaration's result, as `p' ∈ S`, the storage a `linear` p hands back (ADR 0070). |
| **J3** | Both operands of `^` are static integers, the exponent non-negative; refused otherwise. |
| **J4** | A ⊆-binder of a computation's λ takes only a static set (Law S); a host body lies only under ⊆-λs. |
| **J5** | A host body, an `axiom` and the model's data appear only in a target file or a fragment. |
| **J6** | `private` and `export` appear only on a declaration at a module's level. |
| **J7** | A quantifier, `⇒`, `⇔`, a primed name and a `spec`'s call appear only in a claim position (W6). |
| **J8** | A name is spelled by σ_J injectively: a program in which two names spell alike is refused. |
| **J9** | A `var` binder's initializer satisfies R0 when the binder is written element-wise (ADR 0052). |

---

## 6. Printing

print_J is the inverse of parse_J on concrete trees (R1). Where L_J has two spellings of one tree,
**print_J writes one, the canonical spelling**:

| spellings of one tree | print_J writes |
|---|---|
| `use go/os as os`, `use go/os` | `use go/os` |
| `f = fn(x) { e }`, `fn f(x) { e }` | `fn f(x) { e }` |
| `\cup`, `∪` (every §1.6 pair) | the symbol |
| `slices.SortStableFunc(Int)(idx, c)` with Int inferable, and without it | without it |

A spelling that reads to a different tree is not a synonym and is printed as written: `set S = e` and
`S = e` (a claim, and none), `b ∈ S = e` and `b = e`.

**Layout**: four spaces per level; one statement per line; a block's closing brace on its own line
unless the block fits on one line; arms aligned at `->`. Layout is decoration (abstract.md §9.1), so a
file printed in another layout reads to the same abstract tree.

### 6.1 Messages

A diagnostic is a function of the program, so what it quotes is J (jmsg-2026-10-20, `core/jshow.go`):

- **a name** by σ_J (§1.5): `io.print_line`, ``go.`>=` ``, a module path quoted, `` `go/fmt`.Println ``;
- **a set** by J's formers: `0..9`, `-2^63..2^63-1`, `0..`, `Array(0..255)`, `Map(Int, F64)`,
  `Int × Bool`, `()`; an end too long to read keeps its size, `1.234e400 (401 digits)`;
- **a term** of the core by J's expressions, the inverse of D on D's image: a redex is a block
  `{ x = e; b }`, a Church tuple its pair, the connectives and comparisons infix (`=` as `==`), a
  right-nested chain of `if` a `case { φ -> a; … }`, a loop `loop x = z { … }` whose negated guards
  D normalized come back as `!c -> …`, a store `store(b, i, v)`, an integer in decimal (a power folds
  with its neighbours when read); an indexing a message knows is one is `t[i]`.

The law, tested on every corpus definition: **parse_J(show_J(t)) = t** for each core term t that D
makes and that holds no name the compiler made (`#…`) or ascription. A message's own words name J's
forms: `` `use go` ``, `fn f() { … }`, `` `axiom T ⊆ I` ``, `` `repr LO..HI word` ``, `` `x := v` ``.
Not rendered: the prover's fact log after `known:` (the linear fragment's notation, `-i + n + -1 <= 0`),
an interval `[lo, hi]`, the IR's own messages about its text, and a raw form a loader quotes after
`got`.

---

## 7. From today's notation

**Historical**: the translator ran once, moving the corpus to J (jswitch-2026-10-16), and was deleted
with today's reader in step 5b (jswitch5b-2026-10-16). This section records what it did. The
transitional spellings of the last paragraph are still read; no corpus file uses them, and three tests do.

The translator was tr_{T→J} = print_J ∘ (id × transport) ∘ μ (abstract.md §9.2, §10). It carries every
gap to its abstract position, `;` becoming `//`, and a comment that quotes code stays as written. It
rewrites, mechanically:

| today | J |
|---|---|
| a name with `-` | σ_J: `_` |
| `(module X)` in a library file | nothing |
| `(target T (module P …))` | `module P { … }` |
| `(export f …)` in a library | public by default; `private` on each definition not listed |
| `(export main)` in an entry file | `export fn main()` |
| `(sig f …)` beside `(def f …)` | one `fn f(…) … { … }` |
| `(sig f … (host k "t" …))` | `fn f(…) … { host k "t′" … }`, t′ with `%s`, `%n`, `%r` and `%tᵢ` named, `%u` as `${#}`; a parameter given only its set named `a`, `b`, … |
| `(implements P.T I…)` beside `(module P …)` | `axiom T ⊆ I ∩ …` inside `module P { … }` |
| `(type N (host "s"))`, `(type N S)` | `set N = host "s"`, `set N = S` |
| `(const N v (host "s" …))` | `N ∈ set { v } = host "s" …` |
| `(variant (n T…) (c S…)… (success c) (relevant))` | `set N(T…) = 'c(S…) ∪ … success 'c relevant` |
| `(implements T I…)` | `axiom T ⊆ I ∩ …` |
| `(sig (f (E (subsets B))) …)`, `(def f (E) (fn …))` | `fn f(E ⊆ B) { fn(…) … }` |
| `(int lo hi)`, `(pow 2 k)` | `lo..hi`, `2^k`, a 2ᵏ-form literal as a power |
| `(local b (table n f) …)`, threaded `(set b i v)` | `var b := array { … }`, `b[i] := v` (ADR 0052, when built) |
| `((f a) (fn (q r) e))` | `(q, r) = f(a)` |
| a set's name | UpperCamelCase |

Each row that changes semantics, not spelling, waits for its ADR to be built: until then the translator
prints today's construct in J's spelling (a threaded store as the call `store(b, i, v)`), and the ADR's
step rewrites the J corpus behind the byte-identical gate.

**The reader reads those transitional spellings back** (jread-2026-10-15): `b linear = array { … }` is
today's buffer binder, `store(b, i, v)` today's three-argument `set`, `var x := e` and `x := e` a
cell and its write, `m linear = map(cap)` a map buffer. Each is a call or binder J's grammar already
has, so no production exists only for the transition, and each goes when its ADR's step rewrites the
corpus. A program's definition in a target fragment (`provides … { fn f(x) { … } }`) is read by the
program reader, a host declaration by the target reader: the body's first token decides, `host` or a
λ returning one for the second.

---

## 8. Example

`lib/result`, complete, and a function over it:

```go
// result: a computation that may fail, as a value (spec/errors.md §3, ADR 0040).

set Result(T, E) = 'ok(T) ∪ 'err(E) success 'ok relevant

fn err_map(f, r) {
    case r {
        'ok(v)  -> 'ok(v)
        'err(e) -> 'err(f(e))
    }
}
```
```go
use go/os
use go/fmt
use result

fn size_or_zero(path ∈ bytestring) (n ∈ 0..2^63-1) {
    try result.'ok(data) = os.ReadFile(path) else result.'err(e) -> { ignore(e); 0 } {
        len(data)
    }
}

export fn main() {
    av = os.Args()
    loop i = 1 {
        i >= len(av) -> ()
        else         -> { fmt.Println(av[i], size_or_zero(av[i])); again(i + 1) }
    }
}
```

---

## 9. Not here

- **The semantics of every form**: abstract.md, the ADRs, and the specs they name.
- **The target files' model data in full**: target-files.md §1, operands written as J expressions.
- **An editor input method** for the symbols: tooling, with the reader.
- **A pipe** (`x |> f`), a receiver form, raw strings, digit separators, block comments: each refused or
  deferred by its ADR until a program asks.
