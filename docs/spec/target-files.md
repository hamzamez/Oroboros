# The target file format

Read off `emit/target.go` and the three backends, not from memory.

> **Status, 2026-08-15. Built.** The `structural` form exists, `prim` rejects a structural kind
> with a message naming the replacement, and the four target files no longer state a type for
> `let`, `if`, `fold-range` or `fold-range2`.
>
> Removing the false declaration found a real bug: `loop2`'s result type was being read from it.
> `typeOf` never handled `loop2` at all — it fell through to the declared `f64`, which was correct
> for centroid **by luck** and exactly as false as `fold-range`'s accumulator type. `GenCentroid`
> returned `/*unknown*/` the moment the lie was removed. A `loop2`'s result is its **finisher's**
> type, and now is.

> **Status, 2026-09-15. Respelled** as [theories.md §8](theories.md): a primitive is a `sig` whose
> host text sits in one `(host …)` clause, and every representation choice is a `repr`. The old
> words (`prim`, `int-repr`, `array-type`, `boxed`, `builtin-map`, …) are refused with the new
> spelling in the message, not kept as aliases. Every target and library in the repository was
> translated mechanically and loaded to the identical target before the old forms were deleted
> ([loader-2026-09-15](../../gauntlet/results/loader-2026-09-15.md)).

This is the file a **third party** writes to add a target — requirement 3 — and it is the format
the whole parasite thesis depends on strangers getting right. Until now every word in it was
described in a code comment or nowhere ([inventory.md §2](inventory.md)).

**It is data, not code.** Adding a host function is a line here and no Go. What a target author
*cannot* add is a new **structural kind**, because those bind variables and emit control flow, and
no template expresses that.

---

## 1. Grammar

```
file        ::= (target NAME decl…) decl…       ; forms inside the header or after it

decl        ::= (backend NAME)                   ; which code generator compiles this
              | (link "library"…)               ; always on the link line (§6a)
              | (build "…") | (artifact "…") | (data "…")
              | (type NAME (host "spelling"))
              | (type (array A) (host "…%s…"))   ; §2b
              | (type (map K V) (host "…%s…%s…"))
              | (repr (int LO HI) (host "spelling"))
              | (repr (ref T) (host "spelling")) ; the boxed spelling, theories.md §5.7
              | (repr big host) | (repr big limbs)
              | (repr map host) | (repr map library)
              | (repr shift N)
              | (repr narrow (host "template"))  ; how this host restricts a container
              | (fact NAME ((a (array A))) (<= (len a) N))
              | (implements T I…)
              | (module PATH sig…)               ; declares into a module namespace
              | sig
              | const
              | structural

sig         ::= (sig NAME (param…) result sclause… (host kind template hclause…))
const       ::= (const NAME INT (host "spelling" hclause…))   ; §3, "A constant"
param       ::= type | (NAME type)               ; `()` is arity zero
kind        ::= expr | stmt
template    ::= "…%s…"
sclause     ::= pure | index | (where φ) | (ensures φ)   ; a length is an ensures, §4 `length`
hclause     ::= (import "…") | (lib "…") | (checked NAME) | (jump "cc" ["compare"])

structural  ::= (structural NAME skind [pure])
skind       ::= let | cond | loop | loop2 | build
```

`NAME` is an identifier ([core-0 §1.1](core-0.md)). `PATH` is a module path — one identifier,
`/` being an ordinary identifier character ([modules.md §3](modules.md)).

**A `sig` makes two kinds of claim, and the `host` clause is where they part.** A sig clause is about
what the operation MEANS — purity, contracts, indexing, length. A host clause is text for, or a fact
about, ONE host. So a declaration with no `host` clause makes no claim about any host, and in a
target file that is an error: a target realizes declarations, and one with no realization belongs
in a module ([theories.md §5.2](theories.md)). A host word outside `(host …)`, or a sig word inside
it, is refused.

A name declared inside `(module PATH …)` is recorded **fully qualified** as `PATH.NAME`, because
resolution produces qualified names and R1 requires both to key one namespace.

**A file is the glue of its forms**, exactly as a directory is the glue of its files
([target-system.md §2](target-system.md)): `load(F₁ ++ F₂) = load(F₁) ⊔ load(F₂)`. So splitting a
file in two, merging two, or moving a form in or out of the header's parentheses never changes a
target. Two forms that disagree are refused in one file as they are across two.

Duplicate names are an error. An unknown kind is an error. A `sig` whose kind is `expr` or `stmt`
without a template is an error.

## 1b. `backend` — which code generator compiles this target

```lisp
(backend x86-64)      ; one of: go, js, java, x86-64
```

**Optional, and required in practice for any target that emits.** It is `B` of
target-system.md's `T = (B, Δ)`: `Δ` is data and anyone may write it, `B` is
compiler code and the set is **closed** — for the same reason §8 refuses new
structural kinds, since a backend binds variables and emits control flow and no
template can.

**Omitted, the target's own NAME is used when the name is a backend**, so
`(target go …)` need not also say `(backend go)`. When neither resolves, emitting
is an **error** rather than a default:

```
target "portable-js" does not say which backend compiles it, and its name is
not one, so it cannot emit code.
  Add (backend NAME) to the target file, where NAME is one of: go, js, java, x86-64
```

There is no default because a wrong one is a **silent miscompilation**, which is
what it was: `cmd/build` and `cmd/gen` switched on the `-target` *flag string* and
fell through to the Go backend for any name they did not recognise, so
`targets/portable-js.oro` emitted `package gauntlet` and a Go function
([backend-2026-09-06](../../gauntlet/results/backend-2026-09-06.md)).

**Declaring none is legitimate.** A target is a capability set first, and one that
only parameterises the *normal form* ([ADR 0002](../decisions/0002-capability-graph.md))
has nothing to emit with — `targets/blas.oro` declares `cblas_ddot(…)`, which is
C, and exists to show reduction stopping at a different point under `cmd/oro`.
Such a target works there and is refused for emission, which is the truth about
it.

`x86-64` rather than `windows`: the operating system and the instruction set are
different things, and target-system.md §5.2 wants them separable.

## 1c. `provides` — a target fragment in a LIBRARY's file

```lisp
; mylib/mylib-go.oro — beside the library, not inside targets/
(provides go std/words
  (sig split-words (string) (array string) pure (host expr "strings.Fields(%s)" (import "strings"))))
```

**It is exactly `(target T (module M decl…))`, written where the library lives.**
So it needs no new operation and no new precedence: a target is the glue of its
fragments (target-system.md §7.2), and this is one more place a fragment is
found. A library author never edits `targets/`; adding support for a host is one
file next to the library.

Picked up from the **module search path** (`-path`), and it forms the **lowest**
layer: a library's opinion about a host loses to that target's own files, which
are the authority on it.

What it buys is [modules.md §6](modules.md)'s conditional lowering, which was
specified in August and which nothing parsed: the same library is a host call on
a target that provides one and a portable definition — unfolded by δ — on a
target that does not.

> **Adding one is monotone on COVERAGE and not on EMISSION** (target-system.md
> T5′). A name in `P_T ∩ D` takes the native, so shipping a `provides` changes
> what an existing program compiles to. That is conditional lowering working as
> designed, and it is also how a library could silently change a program's
> answer — which is what the conformance obligation in §7 is for.

## 2. `type`

```lisp
(type f64 (host "float64"))
```

Maps **our** name for a type to **the target's** spelling. The language owns the name; the target
owns the spelling. `targets/js.oro` declares none at all, which is correct — JavaScript needs no
type layer, and that is [measured](../../gauntlet/results/js-2026-08-14.md) rather than assumed.

The spelling is emitted verbatim into function signatures and variable declarations. It is never
parsed, so it may be anything the host accepts — `map[string]int`, `HashMap<String,Integer>`,
`double*`.

**`any` is not a type.** `any` is
*the absence of a constraint*, used where the host itself is polymorphic; a target may give it a
spelling (`any` on Go, `Object` on Java) and the emitter uses that only when nothing else ever
constrains the name.

## 2a. `implements` — where a concrete type may stand

```lisp
(implements ptr-os-File io-Reader io-Writer io-Closer)
```

`T` is accepted wherever any of the listed types is declared. Several on one line
because the relation is a SET and a concrete type usually satisfies a family;
writing four lines would suggest four independent facts.

**AN INTERFACE IS AN EXISTENTIAL TYPE** — `∃X. X × Πᵢ(X → Tᵢ)`, a hidden
representation packed with the operations that consume it (Mitchell & Plotkin
1988). *Packing* one is manufacturing a closure and is refused
([callbacks.md](callbacks.md) tier 3). *Holding* one is nothing new: an opaque
host token with methods, which a Win32 `HANDLE` has always been. **Passing one is
neither** — `io.ReadAll(f)` does not ask us to build an `io.Reader`, it asks for
an `*os.File`, and the host inserts the coercion.

So this declares a fact the **type checker** needs and the **backend** does not:

```
    T ≤ I                    in the checker
    ⟦coerce⟧ = id            at emission — zero emitted characters
```

**The relation is a preorder**: reflexive by the checker's own equality,
transitive by a closure computed at load (so `(implements T ReadCloser)` and
`(implements ReadCloser Reader)` give `T ≤ Reader` without anyone spelling it),
and **antisymmetric** — a `*os.File` goes where an `io.Reader` is wanted and not
the reverse, because subsumption forgets every method but the interface's own and
forgetting has a direction.

**Declared rather than derived, and it is not the subtyping type-algebra.md
refuses.** Derived, `T ≤ I` iff `methods(I) ⊆ methods(T)` — Cardelli's record
subtyping — and a method signature may mention an interface, so the relation is
recursive and wants coinduction (Amadio & Cardelli 1993). Declared, it is a
relation on GROUND names decided by lookup. Pierce's undecidable F<: is about
BOUNDED QUANTIFICATION, and after staging nothing is quantified.

**Glue and override are the same operation here**, unlike every other field: a
relation is a set, so two layers both knowing that `*os.File` reads is not a
collision and there is no disagreement expressible.

**And the claim is checkable BY THE HOST**, which no `prim` template is:

```go
var _ io.Reader = *new(*os.File)
```

one line per edge, and `go build` decides. `gauntlet/stdlib/survey.go` generates
exactly that file and **emits only the edges the Go compiler accepted** — 182 of
1,651 candidates were false, because the api manifest lists the exported API and
an interface sealed by an unexported method looks satisfied by everything.

## 2b. `(type (array A) …)` and `(repr (int …) …)` — how the target stores a table

```lisp
(type (array A) (host "[]%s"))        ; Go;  "%s[]" on Java
(repr (int 0 255)    (host "byte"))   ; narrowest first
(repr (int -128 127) (host "int8"))
(repr (int 0 65535)  (host "uint16"))
```

`(type (array A) …)` realizes `lang`'s constructor, and resolves `(array V)` through **one** declaration instead of an entry per element type.
That enumeration is what [tables.md §10](tables.md) called the suffix explosion: Go had declared
seven `slice-*` types and the four targets together fifty-four names, because the type language had
no constructor.

`(repr (int LO HI) …)` is how a **range** picks a representation, which is
[ADR 0003](../decisions/0003-range-typed-integers.md)'s *"the compiler selects the representation
that fits"* moved out of Go and into the target file. The rule is four lines: **the narrowest
declared representation that CONTAINS the range wins**, searched in declaration order.

Three things a target author should know.

**Signedness is not a concept here and does not need to be.** A host that cannot hold `0..255` in
its byte — the JVM, whose `byte` is `-128..127` — simply does not declare that range for it, and
the range selects `short` instead. The declaration says what the host CAN hold and nothing else.

**Declaring none is a legitimate answer.** Then every integer is stored the one way the target
already does. That is correct for JavaScript, which has no integers, and it is a measured choice
rather than an omission: a plain packed `Array` is
[1.15× faster than a `Uint8Array`](../../gauntlet/results/jsontok-2026-08-26.md) on V8.

**The width is read off the declared range, not off the spelling.** A target that says it can hold
`-128..127` has said one byte, whatever it calls it — which is how `targets/windows/` gets byte
elements from `(repr (int 0 255) (host "db"))` without the emitter knowing what a `db` is.

A range never narrows a **local**: `(a i)` is an integer wherever it is used, and only a table's
element slot consults the width. See
[elemwidth-2026-08-27](../../gauntlet/results/elemwidth-2026-08-27.md).

## 2c. `max-len` — how many elements a table can have

```lisp
(fact max-len ((a (array A))) (<= (len a) 2147483647))   ; a Java array's length is an `int`
```

**It is a fact because it is one** — a proposition true of every array on this target, which the
refinement layer assumes ([theories.md §7](theories.md)). Facts are specified and not built, so this
is the one shape a target may state today, and any other fact is refused rather than read as it.

**Optional, and most targets should omit it.** A length is already bounded without any declaration:
`len` returns an `int`, ADR 0012 makes `int` exact within ±(2⁵³−1), so a table with more elements
has a length the language cannot count. See [tables.md §2.3.1](tables.md).

Declare it only where the host says something **tighter and specified**. Java does: `arraylength`
returns an `int` and `new T[n]` takes one, so 2³¹−1 is a fact of the platform rather than a guess
about memory. Go does not — a slice length is a 64-bit `int` and the real limit is the address
space, which is neither specified nor stable, so `targets/go/` declares nothing and keeps the
language's bound.

What a tighter bound buys is the **index type**: a counter bounded by a length fits the host's own
`int` only if the length does. That is the platform fact
[indextype-2026-08-25](../../gauntlet/results/indextype-2026-08-25.md) hardcoded in Go, moved to
where a target author can state it.

`N` beyond 2⁵³−1 is an **error**. A length the target cannot count exactly is not a length.

## 2d. `(repr big …)` — how the target stores a value above the portable window

```lisp
(repr big host)     ; the host's own arbitrary-precision integer
(repr big limbs)    ; a fixed number of base-2^24 limbs, in a `build`
```

**`(repr map library)` is the same kind of choice** — realize a language type by the language's own
library rather than the host's — and windows declares it because it ships no map
([theories.md §5.8](theories.md)). Like every declaration it composes by override, so a nearer layer's
`(repr map host)` wins.

**Optional.** The default is the only thing the target can do: `host` where it declares a bignum
(`big+` and the rest, see §3), `limbs` where it does not.

This is `(repr (int …))` one rung up, and it exists for the same reason. `(int 0 (pow 2 1300))` says the
value is a mathematical integer in that interval — a fact about the PROGRAM, true on every target.
Which storage it gets is a fact about the HOST, and putting the second in the program was measured
at **5.85x on Go, 74.9x on V8 and 2.82x on Java**
([bigrepr-2026-09-03](../../gauntlet/results/bigrepr-2026-09-03.md)).

**Unlike `int-repr`, this cannot be derived**, and that is why it is a declaration rather than a
rule. Widths nest, so *narrowest containing* is complete. Above the word there is no such order:
ours wins where the operation is LINEAR and the host's wins where it is QUADRATIC
([bigarith-2026-08-28](../../gauntlet/results/bigarith-2026-08-28.md)). So a target declares what
somebody **measured**, with the measurement in the file beside it.

An UNBOUNDED range — `(int 0 +inf)` — is not affected: ℤ has no fixed size, so it always takes the
host's bignum, and a target with none refuses the program by name.

**A target that declares `host` must also declare `big-fit`** (§3), because the declared bound has
to be enforced under BOTH representations. A fixed width traps; a host bignum is exact whatever the
declaration says, so without an explicit check the two would disagree about which programs are
legal — and selecting a representation would change an answer, which is
[ADR 0009](../decisions/0009-staging-preserves-results.md) at the representation boundary. The
compiler refuses rather than dropping the bound.

`cmd/build` and `cmd/gen` take `-big-repr=limbs|host` to override this. It is for **measuring the
alternative** before changing the declaration — not a knob a program should depend on, since the
bound, and therefore the answer, is the same either way.

## 2e. `(repr shift N)` — how wide a shift is exact here

```lisp
(repr shift 63)    ; Go, the JVM, x86 — a 64-bit shift
(repr shift 31)    ; JavaScript — V8 coerces `>>` and `&` to int32
```

**Optional.** A target that declares nothing gets no rewrite, which is the safe
default for a third-party target.

`x / 2^k` on a SIGNED value is not a shift: truncation toward zero needs a
rounding correction, and all three of Go, the JVM and x86 emit one even for a
constant power-of-two divisor. Where the compiler can PROVE the dividend
non-negative, it rewrites the division into a shift and the remainder into a
mask, using this target's own spelling of each — found by spelling the way `=`
and `+` are. Measured at **2.39x** on our fixed-limb factorial, which is the
dominant cost in that program
([shiftdiv-2026-09-03](../../gauntlet/results/shiftdiv-2026-09-03.md)).

**This does not promote bitwise operators to the language.** integers.md §0a
keeps them out because V8 coerces both operands of `& | ^ << >>` to int32, an
observable disagreement inside the portable window, and that reason stands
untouched. What this declaration adds is a fact about the HOST that a proof can
be checked against, so the compiler may use the host's own shift for an
operation a program may not write.

`N` is the largest bit-width for which `>>` and `&` are exact on values in
`[0, 2^N)`, and the ceiling is 63 — a host claiming to shift values it cannot
represent exactly would be claiming something ADR 0012 already denies.

## 3. `sig` with a `host` clause — expression and statement primitives

These are **pure data**: an arity, types, a template, and attributes.

### `expr`

```lisp
(sig add (f64 f64) f64 pure (host expr "%s + %s"))
```

The template is filled with the emitted arguments and **wrapped in parentheses by the emitter**, so
a template never needs to parenthesise itself. Arity is checked: the number of arguments applied
must equal the number of declared argument types.

### Several results

```lisp
(sig ReadFile ((path string)) ((array (int 0 255)) error) (host expr "os.ReadFile(%s)"))
(sig Open     ((path string)) (ptr error)                 (host expr "os.Open(%s)"))
```

A result may be a **list** of two or more types. The language has had several
results since [values.md](values.md) — `(values a b)` is the negative product,
reader sugar for `(fn (#k) (#k a b))`, measured at 0.99x on Go with zero
allocations — and this is the ability for a target to say that a **host** call
has that shape.

**A result LIST is not a compound TYPE.** `(int int)` is two results;
`(array int)`, `(int 0 255)` and `(map int int)` are one. Both are an application
of names, so the rule is: anything the type language already spells is one
result, and everything else is a list.

**The elimination form is application to a continuation**, which is how a product
is consumed already:

```lisp
((os.ReadFile "go.mod") (fn (src err) …))
```

and the backend emits the host's own form — `src, err := os.ReadFile(…)` on Go,
where a result the body never reads becomes `_`. A continuation of the wrong
arity is an error naming both counts.

Worth **19.8% of Go's callable standard library** on its own, and more than that
indirectly: `(T, error)` is Go's constructor idiom, so it also unlocks the types
those calls return and the methods on them
([multiresult-2026-09-06](../../gauntlet/results/multiresult-2026-09-06.md)).

### A result RANGE

```lisp
(sig ones ((x (int 0 4294967295))) (int 0 64) pure (host expr "bits.OnesCount64(uint64(%s))"))
```

A range in the result position says what the host call gives back, and the
interval analysis reads it. **A primitive has no body, so a declaration is the
only source of that fact there can be** — the same reason `ensures` belongs here
and is redundant on an internal definition.

Without it every host call is ⊤, so
[ADR 0019](../decisions/0019-precision-by-declaration.md)'s bounded-by-default
refuses *any arithmetic on any result from any ecosystem*, which was measured on
Go and on Win32 independently. It is a range rather than an `ensures` because a
range in a result position **is** an `ensures` (scalarrange-2026-08-31) and
because `ensures` feeds the refinement layer while being in-window is decided by
the interval layer.

**A range result is received into the language's integer.** A range says what a
value IS; its width belongs to storage (elemwidth-2026-08-27). So a backend whose
host gives back a narrower type converts on receipt — Go emits `int(utf8.RuneLen(…))`,
and for several results `r, n := utf8.DecodeRune(…)` becomes a temporary and
`r := int(tmp)`, since a multiple assignment cannot convert in place. That
direction needs no knowledge of the host's type, because `int(x)` compiles from
every Go integer type. **The argument direction does**, and a hand-written
`(int 0 64)` is a range rather than a host type, so an argument conversion is the
TEMPLATE's: `bits.OnesCount64(uint64(%s))`. A template that omits it compiles for a
literal — an untyped constant in Go — and is refused for a variable.

### A constant

```lisp
(const MaxRune 1114111 (host "utf8.MaxRune" (import "unicode/utf8")))
```

is exactly `(sig MaxRune () (int 1114111 1114111) pure (host expr "utf8.MaxRune" (import "unicode/utf8")))`,
the declaration a range result already makes for a call of no arguments, with its value written once
instead of twice ([theories.md §8.3](theories.md)). **The value must be an integer**, because a
constant is a declaration whose value the compiler reads, and only an integer's is read. A float,
string or bool host constant is a zero-argument `sig` with its type as the result. The `host` clause
takes no kind: a constant is a value.

### `stmt`

```lisp
(sig dict-inc (dict string) dict (host stmt "%s[%s]++"))
```

The filled template is emitted as **its own line**, and

> **the value of the term is argument 0.**

That contract has been written in every target file since word count and **no backend implemented
it** until `print-line` forced the issue — `dict-inc` concealed it by declaring `dict` for both its
argument and its result, so the wrong answer and the right answer coincided
([effects-2026-08-14 §5](../../gauntlet/results/effects-2026-08-14.md)). It is now honoured: a
`stmt`'s type is argument 0's type, and its declared result type is a fallback.

Arity is **not** checked for `stmt`.

### Templates and `%s`

The template is filled by cycling the operands to cover however many `%s` holes it has.

| template | operands | result |
|---|---|---|
| `%s[%s]++` | `m`, `k` | `m[k]++` |
| `%s[%s] = (%s[%s] ?? 0) + 1` | `m`, `k` | `m[k] = (m[k] ?? 0) + 1` |
| `fmt.Println(%s)` | `x` | `fmt.Println(x)` |

Cycling rather than a fixed repeat, because *"repeat the operands twice"* was a fact about
`dict-inc` promoted to a rule about a kind, and it produced `console.log(label)%!(EXTRA
string=label)` the first time a one-operand statement existed.

### Templates on a host with no expressions

Everything above assumes the host has **nested expressions**: the hole is filled with the
operand's emitted expression and the host's parser rebuilds the tree. `targets/windows/` emits
x86-64 assembly, which has no tree, and it needed three more holes and one more declaration — and
nothing removed ([ADR 0016](../decisions/0016-targets-need-not-have-expressions.md),
[windows-target.md](windows-target.md)).

| hole | means |
|---|---|
| `%r` | the destination the emitter allocated. There is no expression to *be* the result. |
| `%u` | a unique number, so a template may carry its own labels and its own control flow. |
| `%1`…`%9` | operands by position. An instruction sequence rarely uses them in order. |
| `%b1`, `%br` | that operand's register at 8 bits. `%e1`, `%er` at 32. x86 gives one register three names. |
| `%%` | a literal `%`. |

A template may span lines. `%s` still takes the next operand in sequence, so a single-instruction
template is written exactly as before.

```lisp
(sig add (int int) int pure (host expr "mov %r, %1\nadd %r, %2"))
```

### `jump` — a predicate in branch position

```lisp
(sig setl (int int) bool pure (host expr "mov %r, %1\ncmp %r, %2\nsetl %br\nmovzx %er, %br" (jump "l")))
(sig test-byte ((p ptr) (i int)) bool pure (host expr "…" (jump "ne" "cmp byte ptr [%1+%2], 0")))
```

The `expr` form is what the predicate costs **as a value**; `(jump …)` is what it costs **as a
guard** — the backend emits the comparison and the negated conditional jump and materialises
nothing. The optional second string is the flag-setting instruction when the host's default is not
it.

There were two pseudo-codes here, `"and"` and `"or"`, and
[ADR 0017](../decisions/0017-booleans-are-in-the-language.md) removed them. They made
short-circuiting **a claim a target author makes**, unverifiable by the format and observable only
through a trapping operand — and on the windows target they made one name mean the strict
instruction in value position and the branch in a guard. Short-circuiting is the language's `if`.

Go, JavaScript and Java fold a comparison into a branch inside their own compilers, so all three
declare no `jump` at all. **A host that does not is the reason this exists**, and without it every
loop guard is two comparisons.

### `checked` — the representation a declared range selects

```lisp
(sig + (int int) int pure (host expr "%s + %s" (checked add-exact)))
(sig add-exact (int int) int pure (host expr "Math.addExact(%s, %s)"))
```

An integer operation whose result the compiler proves stays inside the portable window keeps the
host's own operator. One it cannot prove is rewritten to the `checked` primitive
([selection-2026-08-19](../../gauntlet/results/selection-2026-08-19.md)). The name is resolved in
the same module, so it is qualified like any other.

**A target may declare none**, and three of the four do something different: the JVM has an
intrinsic, x86 has a flag and one instruction, Go has neither and uses a func literal called
immediately, and JavaScript declares nothing at all — so a program needing exact arithmetic is
simply not portable there, and covering says so.

### `where` and `ensures` — what a call requires and guarantees

```lisp
(sig /  ((a int) (b int)) int pure (where (!= b 0)) (host expr "%s / %s"))
(sig size ((v any)) int pure (ensures (<= 0 result)) (host expr "size(%s)"))
```

`where` is a **precondition**, discharged at every call site
([refinements.md](refinements.md)). `ensures` is a **postcondition** over the parameter names and
`result`, and it is **assumed** at every call site whose precondition was discharged
([postconditions.md](postconditions.md)).

A primitive is the one place a postcondition cannot be derived, because it has no body — which is
why it belongs here and is redundant on an internal definition, where reduction inlines the call and
the analysis sees the body with the caller's own values.

Two things a target author should know, and both are soundness rather than style.

**A guarantee needs its requirement.** A contract is `P ⟹ Q`, so `ensures` is licensed only where
`where` was *proven* — not merely not refused. An obligation outside the decidable fragment is
reported as *propagated, not proven*, and that does not license the guarantee.

**It attaches to the name the result takes.** Two occurrences of an impure call denote different
values, so the anchor is the binder — which
[ADR 0010](../decisions/0010-effects-as-structural-rules.md) guarantees exists, since an impure
argument is never substituted. For a **pure** primitive there is usually no binder, so `ensures`
carries as an opaque atom and discharges only by syntactic match; that limit is stated in
[postconditions.md §5](postconditions.md).

### `data` — storage the target owns

```lisp
(data "__written qword 0")
```

Win32's `WriteFile` takes a pointer to a cell it writes the byte count into. The language has no
pointers to locals, no addresses and no multiple returns, so there is **nowhere to put one** — and
the target declares it and hides it inside the template. Emitted verbatim into the artifact, and
only when the label appears in the code, exactly as an import is.

Every host before this one could allocate from inside an expression, so no target had ever needed
to declare storage.

### What a target may NOT declare

`if`, `and`, `or`, `not` and `cond` belong to the language
([ADR 0017](../decisions/0017-booleans-are-in-the-language.md)), and declaring one is an error.
`if` is **injected into every target** as a structural `cond`, so the backends are unchanged; it is
no longer a structural *declaration*. `true` and `false` are literals and not names at all.

A module may still declare `and` — that is `logic.and`, a qualified name like any other.

## 4. `structural` — and what a target may NOT declare

> **Changed 2026-08-20.** `if`, `let` and `loop` are the **language's**, injected into every
> target, and **declaring one is an error**. A target's structural set is now normally **empty** —
> the four native targets declare none.
>
> `if` was already like this ([ADR 0017](../decisions/0017-booleans-are-in-the-language.md)). `let`
> and `loop` were not: eleven target files declared them identically, while `core/read.go` already
> desugars `let`, `seq` and `loop` into applications of those precise names — so a target spelling
> either differently broke every program, and the declaration could only ever be written one way.
> It was 22 lines a third-party author could forget, and forgetting one made an
> [ADR 0015](../decisions/0015-loop-and-again.md) language construct silently unavailable.
>
> The rule it generalises: **a construct promoted to the language works on every target and the
> compiler finds the implementation.** A target neither declines one nor declares one. The
> capability graph is for *target-native* names, where "this target cannot do it" is a true and
> useful answer a program can be told.
>
> What is still declarable here is the **retired portable layer** — `fold-range`, `fold-range2`,
> `make-vec` — which is library, not language, and is kept only because the gauntlet's portable
> targets still use it.

## 4. `structural` — the four the backend implements

```lisp
(structural fold-range loop pure)
```

**A structural primitive declares no types.** It cannot: `fold-range` is
`A × int × ((A, int) → A) → A`, which needs type variables and function types — a whole type
language in a target file, for four primitives that a target author may not add anyway. Writing
`(f64 int any) f64` was a **false statement in all four target files** since the loop existed, and
word count has passed a *dictionary* as the accumulator the entire time
([inventory.md §1.1](inventory.md)).

Their types live in the backend beside their emission, which is where their behaviour already
lives. The declaration carries the name, the kind, and purity — the three things the *reducer*
needs.

The contracts below are what the emitters guarantee. They are the same on all three backends
except where noted.

### `loop` — `(fold-range init count step)`

`step` must be `(fn (acc i) …)`. Emits:

```go
acc := init
n   := count          // evaluated ONCE, before the loop
for i := 0; i < n; i++ {
    acc = <body>
}
```

- `i` has type `int`; `acc` has the type of `init`.
- **The count is evaluated once, before the loop.** This is a guarantee, not an accident, and it
  is why the emitted stencil beats idiomatic hand-written Go: Go does not hoist `len(a)-2` out of
  a loop condition, and our residual cannot express the un-hoisted form
  ([arithmetic.md](arithmetic.md) status).
- If the body's value **is** the accumulator, no assignment is emitted — a `stmt` primitive has
  already updated it in place. That is what makes `acc[k]++` legal as a fold step.

### `loop2` — `(fold-range2 x0 y0 count stepX stepY finish)`

Both steps must be `(fn (ax ay i) …)`; `finish` must be `(fn (ax ay) …)`. The two accumulators are
updated **simultaneously**, which is [g2 §6](../derivations/g2-structs.md)'s parallel-assignment
discipline: Go uses tuple assignment, JS and Java need temporaries because destructuring
allocates. The second step's parameters are renamed to the first's before emission.

Both accumulators are `f64`. That restriction is real and undocumented until now.

### `cond` — `(if c then else)`

Go has no conditional expression, so it always introduces a temporary. JS and Java use a ternary
**when neither branch emits a statement**, and fall back to an if-statement otherwise. So the ANF
that [g3 §6](../derivations/g3-generics.md) and [g5 §4](../derivations/g5-bindings.md) both derived
as necessary is **required by exactly one target of three** — a target property, not a language
one.

Branches are emitted in place and are never hoisted, which is what preserves the guarding of an
effect ([effects.md §4](effects.md)).

### `let` — `(let value (fn (x) body))`

The continuation must be a one-parameter abstraction. A `let` reaches the emitter only in a
residual the *reducer* produced, since a source-level `let` is erased by the reader
([def.md](def.md)).

**If the binder is used zero times it is a sequencing point, not a binding**: the value is emitted
for its effect and no variable is declared. That is what makes `seq` work, and Go would reject the
alternative as an unused variable ([effects.md §5](effects.md)).

### `index` and `narrow`

`index` on a two-argument primitive says **argument 0 is a container indexed by argument 1**.
`(repr narrow (host "…"))` on the target says how that host restricts a container to a known length.

Together they let the backend emit `q = q[:n]` before a loop, which hands the host's own
bounds-check elimination a proof it will accept — worth 1.96× on compute-bound loops and nothing on
memory-bound ones ([bce-2026-08-15](../../gauntlet/results/bce-2026-08-15.md)). A target that
declares no `narrow` gets no transformation, which is correct for JavaScript and Java.

### `length` — how long the result is

```lisp
(sig make-bool ((n int)) slice-bool (ensures (= (len result) n)) (host expr "make([]bool, %s)"))
(sig set-bool ((c slice-bool) (i int) (x bool)) slice-bool
     (ensures (= (len result) (len c))) (host stmt "%s[%s] = %s"))
```

**A result's length is a postcondition**, and there are two shapes the compiler reads
(`refine.go`, `lengthContract`):

| postcondition | reading | example |
|---|---|---|
| `(= (len result) n)` | a **count**: the length is argument `n`'s value | `make([]bool, n)` has length n |
| `(= (len result) (len c))` | a **pass-through**: the result is as long as argument `c` | `c[i] = true` returns something as long as `c` |

It was two positional attributes, `(length N)` and `(length-of N)`, beside the clause that already
says what a call guarantees. Those spellings are now refused, naming the postcondition (theories.md
§8.4). The shape is read off the equation, never off an argument's type, because `targets/js/` types
every argument `any`.

It is what lets the compiler prove an index is in range for an array the program built itself. The
sieve's `(let (go.make-bool n) (fn (c) … (go.at-bool c i)))` has no other route: without the
declaration `len(c)` is an opaque variable unrelated to `n`, and the bounds goal `i < len(c)` has
nothing to resolve against. Lengths propagate through lets, loops and conditionals, and a threaded
array's length is established as a **loop invariant** — taken from the initial value and verified
against every back edge.

**It is a claim about the host call, and only the target author can make it.** Nothing about the
string `"make-bool"` says the result is `n` long; the compiler does not guess and a primitive that
does not declare it simply proves nothing.

**Do not declare it on a map.** `set-map` and `set-bool` are the same three characters of Go —
`%s[%s] = %s` — and opposite facts: an array store leaves the length alone, a map insert can add a
key. This is the same shape as `pure`: the safe direction is to omit it, and omitting it costs a
proof rather than a correct program.

## 5. `pure`

```lisp
(sig add (f64 f64) f64 pure (host expr "%s + %s"))
```

Licenses the reducer to copy, drop and reorder an application of this primitive
([effects.md §3](effects.md)).

**It defaults off.** A target author who forgets `pure` gets a slower program; under the opposite
default they would get a silent miscompilation. **The default must be the one whose failure mode is
slow, not wrong.**

## 6. `import`

```lisp
(sig sqrt (f64) f64 pure (host expr "math.Sqrt(%s)" (import "math")))
```

An opaque string handed to the backend's import mechanism, collected across every primitive the
emitted file actually uses. Go and Java emit it; JavaScript ignores it.

## 6a. `lib` and `link` — what the linker must be handed

```lisp
(link "kernel32" "msvcrt" "ucrt" "vcruntime" "legacy_stdio_definitions")
(sig IsCharAlphaA (int) int
     (host expr "mov rcx, %1\ncall IsCharAlphaA\nmovsxd %r, eax" (import "IsCharAlphaA") (lib "user32")))
```

On a host whose toolchain resolves an import at LINK time rather than at compile time, naming the
symbol is half of what a call needs. `(import "X")` makes the emitted unit refer to `X`; something
must also hand the linker a library that defines it. **That is a fact about the host, so it is
declared**, on the same terms as the import.

- **`(lib "NAME")`** on a `prim` names the import library that resolves its `import`, without an
  extension. It is collected **exactly as the import is**: only from primitives the emitted program
  uses, so declaring ten thousand costs a program nothing.
- **`(link "NAME"…)`** at target level names libraries every program is linked against, whatever
  it calls — the ones the backend's own code needs (x86-64 injects `ExitProcess`) and the runtime
  the target's hand-written declarations assume.

The link line is **`link` in declared order, then each used `lib` not already present, in sorted
order**. Both halves are functions of the input, so the build script is too. A `lib` repeated or
already in `link` is not an error: a library is a set member and naming it twice says nothing new.
Across layers `link` is appended, for the same reason `implements` is — a set cannot collide.

**Only the x86-64 backend reads either field.** Go, Java and JavaScript resolve their imports with
the host's own compiler or loader, so there is no line to build. A `lib` on such a target is
accepted and ignored, the way JavaScript ignores `import`.

**What `lib` claims, and why a declaration may carry none.** Naming a library chooses a DLL: an
import library is an archive of short import objects, each binding one symbol to one DLL, and the
loader goes to that DLL at run time. So `lib` claims more than *this links* — it claims **this is
the DLL the name means**. Usually every library that lists a name binds it to the same DLL
(`onecore.lib` binds `MulDiv` to `kernel32.dll` exactly as `kernel32.lib` does), and then any of
them is right. Sometimes they disagree: `AbortPrinter` is bound to `winspool.drv` by one library and
to `spoolss.dll` by another, which is the print client and the spooler's own side, and both link. A
generator that cannot tell which is meant **does not write a `lib`**, and the call is refused at
link time rather than bound to the wrong code
([structval-2026-09-12](../../gauntlet/results/structval-2026-09-12.md) §6,
[linkline-2026-09-13](../../gauntlet/results/linkline-2026-09-13.md)).

## 7. What a target author is promising

A declaration is believed. Nothing here is checked, so each line is an obligation:

1. **The template is valid host syntax** in the position its kind implies.
2. **The `%s` count matches**, or divides, the operand count (§3).
3. **`pure` is true** — the call has no observable effect and no fresh identity. Getting this
   wrong is a miscompilation, not a slowdown.
4. **A `stmt`'s value really is argument 0.**
5. **The declared types are the host's actual types**, since they drive the emitted signatures.
6. **The import is what the template needs**, and on a link-time host **the `lib` defines it** —
   and defines the one the name means, not merely one that links.
7. **If the name belongs to a module carrying a signature, the implementation conforms to it.**
   This is the only obligation with a mechanism behind it — a conformance suite
   ([modules.md §8](modules.md)) — and it exists because covering proves a name is *provided* and
   can never prove it is *right*. `split-words` satisfies every check above and gives different
   answers on Go and JS.

## 8. What is deliberately not in the format

- **Register allocation, or anything else that binds a value to a machine location.** The windows
  target allocates registers in `emit/asm.go` and a target file never names one that holds a
  value; it may only clobber a declared volatile set. This is the same line as the one below.
- **New structural kinds.** They bind variables and emit control flow; adding one is a compiler
  change. The set is closed for a reason: [arithmetic.md §2](arithmetic.md) shows the four are
  exactly the eliminators whose scrutinee is dynamic.
- **Overloading.** Names are unique keys, so `print-line` takes `any` rather than having ten
  signatures. This is the first thing machine-generated target files will break on, and it is a
  *type* problem ([types-sketch §5](../types-sketch.md)), not a format one.
- **Type constructors.** `vec-f64` is an opaque name, not `(vec f64)`. Deferred until a program
  needs `(vec int)`.
- **Cost or priority annotations.** Which of two natives is better is a *measurement*
  ([ADR 0008](../decisions/0008-measurement-over-principle.md)), and belongs in
  `gauntlet/results/`, not in a declaration.
- **Conditional declarations.** A target either provides a name or does not; the conditional lives
  in `P_T ∩ D` ([modules.md §6](modules.md)) and needs no syntax.
