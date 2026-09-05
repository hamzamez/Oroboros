# One name, several types

Research, 2026-09-04. No decision, no build.

On hamza's three questions: why hexadecimal in `\u{H…}`, whether storing a
length would settle the O(n) problem, and whether it is time to discuss
overloading.

They turn out to be connected. The third has an answer this repository already
uses and had not named, and the second changes what the third is being asked to
solve.

---

## 1. Hexadecimal, and it is not a base preference

`\u{1F642}` is hexadecimal because **a scalar value's canonical name is
hexadecimal**. Unicode writes U+1F642 in the standard, in every chart, in every
error message, and in every reference a programmer will have open. The escape's
job is to name a scalar; naming it the way the thing is named is not a
convention, it is the point.

Two supporting facts rather than tastes:

- **The set's structure is legible in hex and invisible in decimal.** The
  surrogate hole is D800–DFFF, the BMP ends at FFFF, the largest scalar is
  10FFFF. In decimal those are 55296–57343, 65535 and 1114111 — numbers that say
  nothing about the boundaries they mark.
- **A hex digit is four bits**, so UTF-16's split is "four digits" and UTF-8's
  boundaries fall on digit edges.

**And the objection is real**: this language has no hexadecimal anywhere else.
`(int 0 1114111)` is decimal, and `(pow 2 200)` exists precisely so that a large
bound need not be written in an unusual notation. So `\u{…}` introduces hex into
a language that otherwise has none.

The answer is that it is not an integer literal. It is a **delimited naming
context** — three characters of syntax whose whole content is a code point — so
it cannot be confused with a number anywhere else, and it costs one local
convention rather than a second numeric syntax. Writing `\u{128578}` for 🙂 would
be a number that appears in no Unicode document.

## 2. Storing the length: yes for one problem, and it is not the expensive one

The proposal is right about what it fixes and it fixes the cheaper half.

[string-operations.md §4](string-operations.md) measured two different O(n)s and
the document should have kept them further apart:

| | why it is O(n) | does a stored length fix it |
|---|---|---|
| `length s` | the scalar count must be computed by scanning | **yes** |
| `(s i)` | the *i*-th scalar's position depends on the widths of all before it | **no** |

The second is the expensive one — 262 ms on Go, 383 ms on V8, 97 ms on Java at
16,000 scalars, and quadratic — and it is quadratic because the encoding is
variable-width, not because anything is unknown. A length field cannot help,
because the question is not *how many* but *where*.

Three further costs, in order of weight:

**A stored length means our string is no longer the host's string.** A Go string
is a pointer and a byte length; a Java string is an object. To carry a scalar
count we would have to wrap one, and then every host API — which takes the host's
string — needs an unwrap at the boundary. That is a representation of our own,
which is the thing [strings.md §3](spec/strings.md) avoids on purpose: *"they are
borrowed, which is the whole Parasite position applied to a type."*

**The count is free exactly where it is already free.** A literal's count is
known at compile time; a concatenation's is the sum of its parts. What a field
would buy is the count of a string *received* from a host API — and that is the
one case where it cannot be maintained without computing it.

**And `alloc` already buys it, plus the expensive half.** `(alloc s)` costs one
native iteration (41, 74 and 60 µs at 16,000 scalars) and yields a table whose
`len` is O(1) *and* whose indexing is O(1). So a length field is **dominated**: it
buys one thing `alloc` also buys and does not buy the thing `alloc` is for. That
is the same shape `arrays-revisited.md` found for free mutation against
uniqueness types.

> **The honest residue**: `(loop ((i 0)) (>= i (length s)) …)` re-evaluates
> `length` per iteration, so an O(n) length inside a loop guard is O(n²) unless
> the compiler hoists it. That is a real trap and it is not specific to strings —
> it is the general question of hoisting a loop-invariant pure call, which this
> compiler does not do today and which `len` on an array does not need because it
> is O(1). Worth knowing before `length` ships.

## 3. Overloading: the mechanism already exists, and it is already used twice

The pressure is real and it comes from strings. If `string` gains operations,
three names collide with names the language already owns:

| | already means | would also mean |
|---|---|---|
| `len` | a table's domain bound | a string's scalar count |
| `alloc` | materialise a table | materialise a string into scalars |
| `=` | integer equality | scalar-sequence equality |

But **"same name" covers three different things, and only the last is
overloading.**

### (a) One operation, parametric in a type — already works

`len : (array V) → int` is polymorphic in `V` and always has been. Nothing
dispatches; the checker carries `V` and the emitter emits one thing. This is not
overloading and it needs no discussion.

### (b) One CONCEPT at several unrelated types — already used, never named

`len` is defined today on **both** `(array V)` and `(map K V)`. Those are
different host operations — a slice header's length and a hash table's count —
selected by the argument's type when the code is emitted. `maps.md` even records
the per-target cost of the second (*"JS's `len` is O(n) against O(1)
elsewhere"*), and `map-len` is a differential case.

So the language **already has one name meaning one concept at two unrelated type
constructors**, and it has had it since maps landed. Adding `string` is a third
instance of an existing mechanism, not a new mechanism.

**What makes it cheap is that the residual is monomorphic.**
`decidability-map.md` puts it plainly: reduction has already made the term
monomorphic, first-order and closed, so by the time anything must choose, the
argument's type is known exactly. There is no inference, no dictionary, no
runtime dispatch — the same static selection `int-repr` and `big-repr` already
perform for representations.

It is worth naming so it stops being invisible:

> **A CONCEPT NAME is a language name whose meaning is one operation, defined on
> a closed set of types, and resolved at emission.** `len` is the domain bound of
> anything with a known finite domain. `alloc` is materialisation. `=` is
> equality where it is decidable.

The set being **closed** is what keeps it from becoming ad-hoc: a target may not
add a type to it, exactly as a target may not declare `if`.

### (c) Genuine ad-hoc overloading — not needed, and not free

Two *unrelated* operations sharing a name, chosen by argument type: a
user-defined `area` on a circle and on a square. That needs resolution rules, and
in a language with subtyping or inference it needs a great deal more. Nothing in
this repository has asked for it, and §3(b) covers every case strings raise.

**Arity overloading** is worth separating out, because it has exactly one live
instance: `targets/go/fmt.oro` declares `Println`, `Println2`, `Println3` because
`tg.Prims` is keyed by name alone. That is a wart in a **target file**, which is
data, and it is a convenience question rather than a language one. Fixing it means
keying primitives by (name, arity) — small, contained, and it changes no program.

## 4. So the overloading question is the representation question

§3(b) makes `len`, `alloc` and `=` on strings free of new machinery — **if
`string` is its own type**.

But string-operations.md §3 observed that `Scalar*` *is* a length-indexed table,
which would make `string` literally `(array scalar)` and dissolve all three
collisions: they would not be a concept at two types, they would be one type.

§4 of that document is why it cannot be, in place: no host offers O(1) indexing
by scalar, so `(s i)` is O(n) and a loop over it is quadratic. And the type
cannot be spelled either — `Scalar` is `[0,D7FF] ∪ [E000,10FFFF]`, not an
interval, so `(array (int 0 1114111))` is a strict superset that admits values
with no UTF-8 at all.

> **The two questions are one.** If a string is a table, there is nothing to
> overload. If it is opaque, there are three names to share — and sharing them is
> a mechanism the language already has.
>
> Both roads are open and neither needs a new idea. What decides is a
> measurement nobody has taken.

## 5. The measurement that decides, and the program that takes it

**Does a real text program index a string, or only fold it?**

If it folds, `alloc` is rarely reached, the monoid alone is enough, and `string`
stays opaque with three concept names. If it indexes, the pressure to make it a
table (and to pay UTF-32) is real and should be measured before it is designed
around.

The first program available to ask is **decimal rendering of a bignum**:

- it is a **fold** — repeated divmod produces digit groups, and digits
  concatenate;
- it needs `concat` and `empty` and **nothing else** — no `len`, no `alloc`, no
  `=`, so it collides with no existing name and needs none of §3's machinery;
- windows needs it, and it is the **last capability gap** in the integer work:
  that host can compute an arbitrary-precision value and cannot print one;
- and the arithmetic half already exists — `div-small` and `rem-small` landed
  yesterday and are exactly repeated division by a power of ten.

## 6. Recommendation

**Build `concat` and `empty`, and write decimal rendering. Decide nothing else
about strings yet.**

That is the smallest step that produces a real program, closes the one capability
gap left on windows, and takes the measurement §5 needs — and it deliberately
avoids every question this document raises, because none of them has to be
answered to take it.

Then, in order, if the program asks for them:

1. **`length`, `=`** as concept names — §3(b)'s existing mechanism, one entry
   each, no new idea. Watch §2's loop-guard trap.
2. **`alloc` on a string** if and only if the program indexes.
3. **Keying primitives by (name, arity)**, which is a target-file tidy-up and
   changes no program.

**Not recommended**: ad-hoc overloading (§3c), a stored length (§2, dominated),
and making `string` an alias for `(array scalar)` (§4, quadratic and unspellable)
— each refused on a measured or structural ground rather than on taste.
