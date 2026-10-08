# Out-parameters: what they are, and how the language should have them

2026-10-08. Research for gocoverage-2026-10-06's cause I and everything like it on the four primary
targets (ADR 0039). Measured in [outparams-2026-10-08](../gauntlet/results/outparams-2026-10-08.md).
Six decisions for hamza are collected in §9.

## 0. The answer, in brief

An out-parameter is a **result in disguise**. A host that writes its answer through a pointer
computes the same function as one that returns it:

```
f : X × Ref T → R        ≅        f' : X → T × R
```

So the language should give the program the result, and give the host its pointer, which the
compiler owns for the duration of one call. That much needs no language work when the declaration
fixes T, and the language already does it in two templates (§4.1). What needs work is the case
Go's standard library is full of, where **the caller chooses T**: `fmt.Sscan`, `json.Unmarshal`,
`binary.Read`, `sql.Rows.Scan`, `errors.As`. Such a function is not one function but a family indexed
by types, and the faithful form is a typed **out-cell** at the call:

```lisp
((fmt.Sscan "42 ana" (out int) (out go.bytestring)) (fn (age name n e) …))
```

which is Go's `var age int; var name string; n, err := fmt.Sscan("42 ana", &age, &name)`, under Go's
name (ADR 0046), for any number of cells, with the types the caller wrote. The declaration says how
the cells come back beside the host's own results, with a **type variable** the cell instantiates
(§5). Each target realizes a cell where its host has one, and a cell is the one thing every target
can make, so it passes the test for a language construct (§6).

---

## 1. What an out-parameter is

### 1.1 The isomorphism

A call `f(x, &c)` that writes c and returns r is, observably, a function of x giving (c', r), provided
nothing else can see c during the call. That proviso is the whole theory:

- **Copy-out** (Ada's `out` mode, ARM §6.4.1): the parameter's final value is copied to the actual on
  return. It is a result, and the language may pass any location for it.
- **Value-result** (Hoare 1971; Algol W): `in out` is copy-in and copy-out, an argument and a result.
- **By reference** (C, Go): the callee writes the caller's memory directly. It equals copy-out exactly
  when the location is **not aliased** during the call: no other name for it is read or written while
  the callee runs (the classical condition under which call-by-reference and call-by-value-result
  agree; Hoare 1971, Reynolds 1978 on interference).

The language has no aliasing (ADR 0018: values are immutable, and the one mutable thing, a buffer, is
linear). If **the compiler** allocates the cell, uses it for one call, and reads it once afterwards,
nobody else has a name for it. Then by-reference is copy-out, and the isomorphism holds by
construction. A cell is a linear buffer of length one, written by the host, frozen at the call's
return.

### 1.2 Three things a pointer parameter can be

Measured on Go (§3), only the first is an out-parameter:

| kind | algebra | example | language |
|---|---|---|---|
| **out** | a result, T | `fmt.Sscan(s, &x)`, Win32 `GetConsoleMode(h, &mode)` | §5 |
| **in-out** | an argument and a result, T → T | `flag`-less in Go; Win32 `_Inout_` lengths (`lpcchSize`) | §5.6 |
| **retained** | a shared cell with identity, read or written later or concurrently | `flag.IntVar(&n, …)` (written at `Parse`), `atomic.AddInt64(&n, 1)` | not this; §5.7 |

A retained pointer is not a result: the callee keeps the address, so the cell is aliased by
definition, and the isomorphism fails. That is mutable shared state, which concurrency-research.md
owns.

### 1.3 Buffers are already out-parameters

host-buffers.md's **write-borrow**, `(buffer T)` filled by the host and handed back, is the out-
parameter of a *table*: Go's `io.Reader.Read(p)`, Java's `InputStream.read(byte[])`, the browser's
`crypto.getRandomValues(a)`, Win32's `_Out_writes_(n)`. A cell is the same thing at length one with
no initial contents: an out-cell of T ≅ a write-borrow of a one-element buffer of T, read once. So
this research adds no new kind of effect, only a new place one appears.

---

## 2. Who fixes the type

Everything else in this document follows from one distinction.

**The declaration fixes T.** Win32's `GetConsoleMode(HANDLE, LPDWORD)` always writes a `DWORD`. The
declaration can return it, and the call passes nothing for it:

```
GetConsoleMode : HANDLE → BOOL × DWORD
```

**The caller fixes T.** `fmt.Sscan(str, a ...any)` writes whatever its arguments point to: it looks at
each pointer's dynamic type (`*int`, `*string`, `*float64`, `*[]byte`, a `Scanner`) and parses
accordingly. Its type is a **family** indexed by a list of types:

```
Sscan : Π (τ₁ … τₖ : Type). B* → τ₁ × … × τₖ × ℕ × (1 + E)
```

This is a dependent product over a type list, a function whose result type depends on a *type*
argument. In System F it is type application, `Sscan [int, string] s`; with a list of any length it is
a **variadic type parameter**: C++'s parameter packs (2011), TypeScript's variadic tuple types (4.0,
2020), Swift's parameter packs (5.9, 2023, `each T`). Go itself has no such type: it gets the family by
reflection at run time, which is why the parameter is `any`.

**Our language already has the machinery for half of this.** A variant takes type arguments
(`(result T E)`), instantiated at each use, and the residual is monomorphic: staging erases every type
abstraction (closures-direction.md, the two-level language). A type-indexed host declaration is the
same thing at a boundary: polymorphic in the declaration, instantiated by the call, monomorphic after
staging. What is missing is a way for the *call* to name the instance. In Go the pointers' declared
types name it. Here an out-cell names it: `(out int)` is the type argument, written where Go writes
`&x`.

---

## 3. What is measured

From [outparams-2026-10-08](../gauntlet/results/outparams-2026-10-08.md):

**Go.** Of 11,928 exported functions and methods in Go's API manifest, **49** take an explicit pointer
to a value, and none is an out-parameter: `sync/atomic` (35, shared cells) and `flag` (14, retained
until `Parse`). Go's out-parameters are typed `any` and documented as pointers. Read off the doc
comments of every such function, about **25 across 8 packages**, and they are central:

| package | caller-typed out-parameters |
|---|---|
| `fmt` | `Scan`, `Scanf`, `Scanln`, `Sscan`, `Sscanf`, `Sscanln`, `Fscan`, `Fscanf`, `Fscanln` (cause I) |
| `encoding/json` | `Unmarshal`, `Decoder.Decode` |
| `encoding/xml` | `Unmarshal`, `Decoder.Decode`, `Decoder.DecodeElement` |
| `encoding/gob` | `Decoder.Decode` |
| `encoding/asn1` | `Unmarshal` |
| `encoding/binary` | `Read`, `Decode` |
| `database/sql` | `Row.Scan`, `Rows.Scan` |
| `errors` | `As` |

`errors.As(err, &target)` is the error model's own type test (§8.2). And Go moves each out-cell to the
heap (`-gcflags=-m`: *moved to heap: a*), because its address is boxed into `any`; hand-written Go pays
the same allocation, so the realization in §6.1 is at parity by construction.

**Win32.** Of 11,575 flat entry points in the SDK's headers, **3,461 (29.9%) have at least one SAL
out-cell** (`_Out_`, `_Out_opt_`, `_Outptr_…`): 2,907 with cells only, 554 with cells and out-
buffers. A further 977 have out-buffers only. There are 4,598 out-cell parameters, of which **at least
2,133 point to one machine word** by the SDK's naming (`LPDWORD`, `PHANDLE`, `PULONG`…; the name list
misses `UINT32*`, `int*`, `bool*`, `PULONGLONG`), and many more point to a pointer
(`void**`, `PSID*`, COM interfaces: an object the host allocates and hands back, one word too). The rest
point to structs (`GUID`, `RECT`, `FILETIME`, `POINT`), which wait for records. **Every Win32 out-cell's
type is fixed by its declaration.** Win32 has no caller-typed out-parameter.

**Android and the JVM.** Java has no pointers. An out-parameter is a caller-allocated array or object
the method fills, and Android names them so: `View.getLocationOnScreen(int[] outLocation)`,
`View.getHitRect(Rect outRect)`, `Location.distanceBetween(…, float[] results)`,
`Paint.getTextBounds(String, int, int, Rect bounds)`. An array is a write-borrow (§1.3, built). A filled
object is a host object whose state a call changes, a method with an effect on a handle (built, as any
impure method). Nothing on the JVM needs an out-cell of a value type.

**The browser.** JavaScript has no pointers either, and Web APIs return their results: `TextEncoder.
encodeInto(s, dest)` returns `{read, written}`, a product, and fills `dest`, a write-borrow.
`crypto.getRandomValues(a)` fills and returns `a`. Nothing in the Web platform needs an out-cell.

So: caller-typed cells are Go's (reflection is how Go spells a type-indexed family), fixed-type cells
are Win32's (thousands), and the JVM and the browser have only the buffer form, already built.

---

## 4. What the language has today

### 4.1 Fixed-type cells, by template

A template that **owns the pointer** turns a fixed-type out-parameter into a result, and the language
never sees a pointer:

```lisp
; strconv.UnquoteChar's template already does it (a tuple payload, step 5b)
; Win32's WriteFile does it with a global data cell, __written:
(sig WriteFile ((h HANDLE) (buf (array (int 0 255))) (n DWORD)) BOOL
     (host asm "mov rcx, %1\nmov rdx, %2\nmov r8, %3\nlea r9, __written\n…call WriteFile"))
```

Two problems with doing all of Win32 this way: every declaration writes the cell's plumbing by hand,
in each backend's language; and `__written` is a **global**, so it is not reentrant and cannot be
returned (the count is discarded). The general form should be one declaration form and one
realization per backend.

### 4.2 Caller-typed cells: nothing

`fmt.Scanln` was declared once with `any` operands and removed: it compiled and could store nothing.
gocoverage-2026-10-06 proposed one template per type (`SscanInt`), which works and is not Go's name, the
mistake ADR 0046 closed.

---

## 5. The design

### 5.1 A declaration's parameter may be an out-cell

```lisp
(sig GetConsoleMode ((h HANDLE) (mode (out DWORD))) BOOL
     (host expr "GetConsoleMode(%s, %s)"))
```

`(out T)` in a **parameter** list says: the host writes a T through this argument. The template's hole
for it receives the cell's address (Go `&v`, x86 a stack slot's address, §6). The call passes nothing
for it, and **the call's results are the declared result followed by the cells, in order**:

```lisp
((win.GetConsoleMode h) (fn (ok mode) …))      ; BOOL × DWORD
```

That is §1.1's isomorphism stated in the declaration, and it replaces §4.1's hand plumbing. (Results
then cells, or cells then results, is §9's decision 4. Results first keeps the host's own result where
a program finds it today, and the cells are what the declaration added.)

### 5.2 A type variable makes it caller-typed

```lisp
(sig As ((err error) (target (out T))) (option T)
     (host expr "%s" …))
```

A declaration may name a **type variable** in an out-cell. The variable is instantiated at each call by
the call's own `(out τ)` argument, which is the only argument a type variable can take:

```lisp
(case (errors.As e (out go/io/fs.PathError))
  (some pe) (fmt.Println "path error on" (PathError.Path pe))
  none      (fmt.Println "something else"))
```

Instantiation is unification on the declaration's type, as a variant's `(result T E)` is instantiated
by its constructors (sums.md), and it happens before reduction, where the call's head and its `(out τ)`
are both in hand (the loader, as for variadic calls, ADR 0046). After it, the declaration is the
monomorphic instance, `As_PathError : error → option PathError`, and nothing downstream learns that the
declaration was polymorphic.

### 5.3 The cells combine with the host's results by the declaration's shape

The declared result says what the cells *mean* beside the host's own results, and the retraction
(errors.md §4.2, H and r) already reads that compositionally:

| declaration | result | because Go documents |
|---|---|---|
| `errors.As` | `(option T)`: `some` the cell when As returned true, `none` otherwise | "As finds the first error … that matches target, and if one is found, sets target to that error value and returns true" |
| `json.Unmarshal(data, (out T))` | a partial success, the cell and the relevant 1 + E | on a syntax error nothing is stored; on a type error it "skips that field and completes the unmarshaling as best it can". Which shape is the declaration's claim, read off these sentences (ADR 0022) |
| `fmt.Sscan(s, (out τ)…)` | the cells, `n`, and the relevant 1 + E: a partial success | "returns the number of items successfully scanned"; a cell past n keeps its zero |
| `binary.Read(r, order, (out T))` | `(result.result T error)` | `ErrUnexpectedEOF` after some bytes, and the value is then partial, so it means nothing beside the error |

So the cell is not always simply returned: an `option` and a sum are the honest types where Go's
documentation says the cell means nothing in one case, and a partial success where it means something
even with an error. This is ADR 0044's rule, read off the docs, now
with the cell as one of H's components: H(out T) = [T], and r builds the declared shape from the host's
values and the cells'.

### 5.4 Variadic cells

`Sscan`'s cells are its variadic list (ADR 0046), so its type variable is a **pack**:

```lisp
(sig Sscan ((str bytestring) (cells (out Ts …)))
     (tuple Ts … (int 0 9223372036854775807) (result.result (tuple) error))
     variadic (host expr "fmt.Sscan(%s, %s...)" (import "fmt")))

((fmt.Sscan "42 ana" (out int) (out go.bytestring)) (fn (age name n e) …))
```

`Ts …` is instantiated by the call's cells, here `int go.bytestring`, and the result is
`(tuple int go.bytestring n err)`. This is the variadic-generics construct of §2 (Swift's `each T`), and
it is the one genuinely new thing in the type language. It is needed for the Scan family and
`sql.Rows.Scan` alone, and it is small because the pack is used once, in the result, never iterated
over.

### 5.5 What `(out τ)` is, in the term language

`(out τ)` is the first **type written in argument position**. Types appear today in `sig`, in `the`, in
`variant` and in patterns' declarations. It is static, a type argument, and it is erased with every
other type: after the loader instantiates the declaration, the call has no argument for the cell at
all. It has no value: `(out τ)` anywhere but a call's out-cell position is refused, as `spread` is
outside a variadic list. τ is any type a cell can hold on the target, so one host value (types.md §3.3):
a struct type waits for records.

### 5.6 In-out

Win32's `_Inout_` lengths (`lpcchSize`: the buffer's size in, the needed size out) are an argument and a
result. `(inout T)` would take the value at the call and return the final one: `(f x (inout n))`. Not
needed by Go (its explicit pointer parameters are atomics and flags, §3) and by Win32 mostly beside a
buffer, so recorded and left for the first Win32 program that meets it.

### 5.7 What is not an out-cell

`flag.IntVar(&n, "n", 0, "…")` keeps the address and writes it at `flag.Parse`; `atomic.AddInt64(&n, 1)`
shares a cell between goroutines. Both make the cell aliased, so neither is a result, and `(out T)` is
refused for them by declaration: a declaration that marks a parameter `out` claims the host does not
keep it (ADR 0022's rule: the declaration states what the host's documentation says). Go's
`flag.Int(…)` returns a `*int` the package owns, a host handle, read after `Parse`: declarable today as
a handle with a read. `sync/atomic` belongs to concurrency-research.md.

### 5.8 What it rests on

- **Soundness of copy-out = by-reference** (§1.1): the compiler allocates the cell, hands its address to
  one call, and reads it once, so it is unaliased. ADR 0018 already guarantees a program cannot name it.
- **Relevance**: a partial success's error is relevant (ADR 0044), so `(fmt.Sscan …)`'s error is used
  or ignored.
- **Staging**: type variables are instantiated before reduction and erased after, as variants' are
  (ADR 0009 is untouched: no value is computed at compile time).

---

## 6. On each target

A cell is the one storage every target can make: a local variable whose address the call takes. So
`(out T)` passes the test for a language construct (CLAUDE.md, "Adding to the language"): it means the
same on every target, and every target realizes it.

### 6.1 Go

```lisp
((fmt.Sscan line (out int) (out go.bytestring)) (fn (age name n e) …))
```

```go
var v1 int
var v2 string
v3, v4 := fmt.Sscan(v0, &v1, &v2)
```

This is Go's own code. Escape analysis moves `v1` and `v2` to the heap, exactly as for hand-written
Go, so it is at parity by construction.

### 6.2 Windows (x86-64, MASM)

A cell is a stack slot in the caller's frame, and its address is `lea`'d into the argument register:

```lisp
((win.GetConsoleMode h) (fn (ok mode) …))
```

```asm
    ; illustrative: the slot's offset is the frame's
    lea rdx, [rsp+40]        ; the cell, a slot in this frame
    mov rcx, rbx             ; h
    call GetConsoleMode
    mov r12d, dword ptr [rsp+40]   ; the cell, read once
```

This replaces `__written`'s global cell with a reentrant one, and lets `WriteFile` return its count.
The x86 backend already colours values into registers and slots (spec/ir.md §9.6). A cell is a slot
whose address escapes into one call and is read once after it. 3,461 Win32 functions have such
cells, so this is where the construct pays most.

### 6.3 Android (the JVM)

Java has no pointers, and Android's out-parameters are arrays and objects, which the language already
has (write-borrows and handles). Where a Java API wants a holder for a single value, a cell is a
one-element array the template allocates:

```java
int[] c = new int[1];
f(x, c);
int v = c[0];
```

HotSpot and ART's escape analysis may scalar-replace it. No JDK or Android API measured here needs
this, so the realization is specified and not built until one does.

### 6.4 The browser

JavaScript has no pointers, and Web APIs return their results as values or objects, so a JavaScript
declaration never declares a cell. A JavaScript library that takes a holder object (`{value}`) would be
realized as `const c = {}; f(x, c); c.value`. Specified, not needed.

### 6.5 Portability

A program that calls `fmt.Sscan` is Go's. `(out T)` itself is portable: the same declaration form on
every target, each realizing the cell its own way. That is the capability graph's division (ADR 0002):
the construct is the language's, and the functions that use it are the host's.

---

## 7. Scanf and its format

`fmt.Sscanf(str, format, cells…)` must agree with its format's verbs: `%d` writes an integer, `%s` a
string, `%v` anything. Go checks this at run time, and `go vet`'s printf analysis checks it statically
when the format is a literal. With a literal format the compiler could check `(out τ)` against each verb
as a refinement on the literal. It is the same evaluation route a host precondition now has (core.
Evaluate, variadic-2026-10-08), extended to strings. Without it a mismatch is Go's run-time error,
reported through `err`. Recommended as a second step (§9, decision 6).

---

## 8. Examples

### 8.1 Reading typed input: the Scan family

```lisp
; Read "name age" pairs from standard input until EOF, and sum the ages.
(def sum-ages (in)
  (loop ((total 0))
    ((fmt.Fscan in (out go.bytestring) (out (int 0 200))) (fn (name age n e)
      (case e
        (result.ok u)  (again (+ total age))
        (result.err x) total)))))
```

```go
for {
    var name string
    var age int
    n, err := fmt.Fscan(in, &name, &age)
    if err == nil { total = total + age; continue }
    return total
}
```

A declared range on a cell, `(out (int 0 200))`, is a claim about what the host writes. Go's `Fscan`
does not promise it, so it is an obligation the program would discharge with a test after the call,
not a type the host guarantees (the same rule as a declared result's range, refinements.md §6b).

### 8.2 The error model's type test: `errors.As`

```lisp
(case (os.Open path)
  (result.ok f) …
  (result.err e)
    (case (errors.As e (out go/io/fs.PathError))
      (some pe) (fmt.Println "cannot open" (PathError.Path pe) ":" (PathError.Err pe))
      none      (fmt.Println e)))
```

(`io/fs` has no file yet, cause K; `PathError`'s fields are read by declarations such as
`(sig Path ((self PathError)) bytestring (host expr "%s.Path"))`, which need no records.)

`errors.As` is a downcast into a sum: As : error × (type τ) → τ + 1. That is ADR 0040's model
exactly, a failure as a value of a marked sum, here produced by a type test. The portable `os`'s
classification (`not-found | exists | permission | other`) uses Go's `os.IsNotExist`; `errors.As` is
what a program uses for anything finer.

### 8.3 A fixed-format decode: `binary.Read`

```lisp
((binary.Read r (binary.LittleEndian) (out (int 0 4294967295))) …)   ; a uint32
```

`binary.Read` decodes a fixed-size value, so a cell of a fixed-size integer, or a table of them, works
today. A struct waits for records, which is where most of `encoding/binary`, `json`, `xml`, `gob` and
`asn1` use is.

### 8.4 Win32: two handles out of one call

```lisp
(sig CreatePipe ((rd (out HANDLE)) (wr (out HANDLE)) (sa LPSECURITY_ATTRIBUTES) (size DWORD)) BOOL
     (host asm "…"))

((win.CreatePipe (x64.null) 0) (fn (ok rd wr) …))     ; sa is _In_opt_, passed as NULL
```

Two fixed-type cells, two stack slots, three results. No type variable: Win32's types are the
declaration's.

### 8.5 What stays outside

- `json.Unmarshal(data, &v)` into a struct: records (D in gocoverage).
- Win32's `GetSystemTime(LPSYSTEMTIME)`: records.
- `flag.IntVar`, `atomic.*`: retained cells, not out-parameters (§5.7).

---

## 9. Decisions for hamza

1. **Out-cells in declarations, `(p (out T))`**: the host writes a T through the parameter, the call
   passes nothing, and the call's results include the cell. Replaces the hand plumbing of §4.1 on Go and
   Windows. *Recommended.*
2. **Type variables in host declarations, instantiated by `(out τ)` at the call**: the caller-typed
   family (§2, §5.2), monomorphic after the loader. The first type written in argument position.
   *Recommended*: it is the only form that gives `fmt.Sscan`, `errors.As`, `json.Unmarshal` and
   `binary.Read` under Go's names.
3. **A type pack, `Ts …`, for variadic cells** (§5.4): Swift's parameter packs, used only in the
   result. Needed by the Scan family and `sql.Rows.Scan`. *Recommended with 2.*
4. **The order of a call's results**: the declared result, then the cells (recommended: the host's own
   result stays where programs find it), or the cells first, as Go's argument order puts them.
5. **Retained pointers are not out-cells** (§5.7): `flag.*Var` and `sync/atomic` stay out of this
   design. `flag.Int`'s returned handle is declarable today, and atomics go to concurrency research.
   *Recommended.*
6. **Scanf's format checked against its cells when it is a literal** (§7): a second step, after 1–3.

**Cost**, if 1–3 are taken: a spec (`docs/spec/outcells.md`), the loader's instantiation (beside the
variadic rewrite), the checker's and the IR's result arity taken from the instance rather than the
declaration, the retraction reading a cell as one of H's components, and a cell realization in the Go
and x86 printers (Java and JavaScript specified, built when a program needs one). Roughly G and H
together. It unlocks: the Scan family (cause I, 9 names), `errors.As`, `binary.Read`/`Decode` on
integers and tables, `json`/`xml`/`gob`/`asn1` decoding into the types the language has, `sql`'s
`Scan` once `database/sql` has a file, and the thousands of fixed-type Win32 cells, `WriteFile`'s count
among them.
