# Tables, the read side — 2026-08-25

`(array e…)`, `(table n f)`, `(len t)` and **indexing by application**, built on the reducer and on
all four backends. [ADR 0018](../../docs/decisions/0018-immutable-values-linear-buffers.md)'s write
side — `build` and `set` — is not in this.

The pass condition was set before building: **portable `dot` at the recorded 485 ns with the
hand-rolled vector library deleted.** It passed, and by the stronger test.

---

## 1. The result

| | ns/op |
|---|---|
| `NativeDot` — six definitions, hand-rolled vector library | 448.1 – 451.7 |
| **`TableDot` — the language's table** | **448.6 – 451.5** |

**1.00×**, five runs each, interleaved. And the instruction sequences are **identical** — 20
instructions, differing only in the source filename and in jump addresses, which differ because
the two functions sit at different addresses:

```asm
MOVSD_XMM 0(AX)(CX*8), X1
MULSD 0(DI)(CX*8), X1
ADDSD X1, X0
INCQ CX
CMPQ BX, CX
JG ADDR
```

### What was deleted

Four of six definitions. `vec`, `vlen`, `vindex` and `of-array` existed only because a table was
not in the language:

```lisp
(def vec      (fn (n f) (fn (sel) (sel n f))))     ; gone — (table n f) IS this
(def vlen     (fn (v)   (v (fn (n f) n))))         ; gone — (len t)
(def vindex   (fn (v i) ((v (fn (n f) f)) i)))     ; gone — (t i)
(def of-array (fn (a)   (vec (go.len a) …)))       ; gone — a parameter IS a table
```

And **54 target declarations** become one per target. Go declared 19 `at-*`/`make-*`/`set-*`/`len`
primitives plus seven `slice-*` types; JavaScript 9, Java 13, windows 13. `(array-type "[]%s")` on
Go and `("%s[]")` on Java is the whole replacement, because the suffix explosion was the type
language having no constructor.

---

## 2. The hole this build found, which is the important part

**Indexing-as-application silently deleted the bounds obligation.**

```lisp
(sig f ((a (array f64)) (i int)) f64)
(def f (fn (a i) (a i)))        ; i is unconstrained — ACCEPTED
```

while the form it replaced was correctly refused:

```
go.at-float64 requires -i <= 0, which does not follow
```

The obligation had lived in the *primitive's* `(where (and (<= 0 i) (< i (len v))))`. Making
indexing application deleted the primitive, and the obligation went with it — a refactor that looks
clean, passes every existing test, and removes a safety property.

It is generated from the **form** now, so a target author cannot forget it and it applies on all
four targets at once. [tables.md §6](../../docs/spec/tables.md) already said the right thing and it
reads differently after this: **bounds are the domain.** `0 <= i < len(a)` is not a check bolted
onto an operation, it is the condition for the application to be *defined*.

```
(a i) is an indexing, and (<= 0 i) does not follow
  known: nothing
  A table is a function with a finite domain, so 0 <= i < len is the
  condition for the application to be DEFINED — not a check bolted on
```

**And the same deletion hit bounds-check elimination.** `narrowTargets` looked for
`(at-float64 a i)`; with indexing as application it stopped firing and `b = b[:n1]` vanished from
the output — worth **1.96× on compute-bound loops** ([bce-2026-08-15](bce-2026-08-15.md)). Both
were invisible in the timings, because `dot` still ran at 448 ns either way. They were found by
reading the emitted code and by writing a deliberately unsafe program.

---

## 3. Fusion needed one thing argument had not predicted

`(table n f)` had to be made **duplicable**, or the pipeline did not fuse:

```lisp
(let (table (len a) (fn (i) (go.f* (a i) (b i)))) (fn (v) (loop …)))
```

`sum` mentions its argument twice — as `(len v)` and as `(v i)` — so β let-bound it and the
intermediate survived.

What looks like duplication is the step that **erases** the intermediate. Substituting puts
`(len (table n f))` where `(len v)` was, which folds to `n`, and `((table n f) i)` where `(v i)`
was, which is `(f i)`. The table is gone on both sides and `n` still appears once.

The condition is **purity**, not duplicability: the first version demanded the parts be duplicable
and `(len a)` is an application, so it refused exactly the case it existed for.

A **graph** is deliberately *not* duplicable. A rule is a length and a function; a graph is data,
and copying it copies the elements — the code growth
[staticdata-2026-08-20](staticdata-2026-08-20.md) measured as a pure loss on Java and JavaScript.
The cost is a missed fold: behind a binder, β-tab cannot see the array, so `(v 0)` survives even
with both table and index literal. That is the constant folding
[tables.md §4.3](../../docs/spec/tables.md) deliberately defers.

---

## 4. Per target

| | indexing | `len` | `(array V)` type |
|---|---|---|---|
| **Go** | `a[i]` | `len(a)` | `[]float64` |
| **JavaScript** | `a[i]` | `a.length` | untyped |
| **Java** | `a[(int) i]` | `a.length` | `double[]` |
| **windows** | `[rbx+rcx*8]` | **refused, deliberately** | — |

**Java's `(int)` cast is not optional** and the backend now carries it: our `int` maps to `long`,
a Java array index must be an `int`, and without the cast `javac` refuses the file with *"possible
lossy conversion"*. The declaration `at-double` used to carry it as `%s[(int) %s]`; moving indexing
into the backend moves the host detail with it, which is one fewer thing a target author has to
know. Verified by running `javac`.

**windows is the honest gap.** Indexing works — a table is an address and `(a i)` is a scaled load.
`len` does not, because x86 has only an address and *whether a table here is a fat pointer, a length
at offset 0, or a separate argument is a representation decision*. It belongs with `(alloc …)`,
which is what allocates. Deciding it here would settle it by implementation accident, which is the
failure [assessment-2026-08-20 §2](../../docs/assessment-2026-08-20.md) recorded. The refusal says
exactly that.

---

## 5. Method

**Zero of 188 residuals changed** — every example on all four targets, before and after. The
construct is purely additive, like case-of-case and the loop-drop before it.

Also checked, because timings hid both of the §2 findings: the emitted Go was read line by line
against the version it replaces, the emitted Java was compiled with `javac`, and a deliberately
unsafe program was written to confirm the refusal fires.

The 1.00× is inside the ~15% noise floor and the identical instruction sequence is what the claim
actually rests on.
