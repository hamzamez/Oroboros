# Errors

Status: **specified 2026-10-06** ([ADR 0040](../decisions/0040-a-failure-is-a-value-of-a-marked-sum.md)),
not built. The research is [errors-research.md](../errors-research.md), and this is its candidate
**E4**, with hamza's decisions:
- the encoding of failure is a property of a host's error type, a niche;
- `try` and relevance are roles a variant declares, so the compiler depends on no library;
- `expect` is the one way to give up;
- one error type per `try` chain, with `err-map`;
- the portable `os` has three kinds.

The measurements are [errors-2026-10-04](../../gauntlet/results/errors-2026-10-04.md) (the product leaks;
a propagated sum costs nothing) and [sumofsums-2026-10-06](../../gauntlet/results/sumofsums-2026-10-06.md)
(a sum of sums reduces). Section numbers are stable.

---

## 0. What changes

| | today | this specification |
|---|---|---|
| a fallible host call | returns a product `(tuple T error)`, with an unwritten rule: when the error is not nil, the value means nothing | returns a **sum** `(result T E)`, built at the boundary from the host's encoding; or, where the host succeeds in part, a product carrying an **option** (§4.3) |
| reading the value after a failure | compiles; Go gives 0, JavaScript and Java crash (errors-2026-10-04 §2) | impossible: the value is bound only in the success arm |
| which failure | nothing portable | a **kind**, matched by `case` (§6) |
| a dropped error | silent | **refused**, unless discarded in so many words (§7) |
| passing failure up | an `if` at every level | **`try`**, the exception monad's bind (§5) |
| giving up on purpose | nothing visible | **`expect`**, a permitted crash with a reason (§8) |
| what is emitted | Go's own `if err != nil` | the same: a sum eliminated where it is built leaves no tag (§11) |

---

## 1. Three kinds of error, three algebras

1. **A domain condition**: an operation applied outside its domain, such as an index out of range, a
   zero divisor, or a `where` false at a call. It is a **bug**, and not a run-time error in this
   language: an obligation, proven or refused (refinements.md §3a, ADR 0028).
2. **Abandonment**: a state the program has no way on from. It is the exception effect with no handler
   in the process, crash : E → 0. It happens only where the source says `expect` (§8) or the build says
   `-checked`. It ends the process: today the program, under concurrency one supervised process
   (concurrency-research.md §2.4).
3. **The environment's failure**: the world did not cooperate. It is **not a bug**, and no proof can
   exclude it. It is a **value**, a summand of a coproduct, and most of this document is about it.

---

## 2. The theory the compiler knows: a sum with a success summand

The compiler knows no error type. It knows a **theory**: a variant with one constructor marked
`success`, A + R, where A is the success constructor's payload and R the sum of the others. Any variant
that declares the role is a **model** of it (ADR 0021: a declaration is a theory, a module a model).

Two markers join the variant declaration (sums.md §2):

```lisp
(variant (result T E) (ok T) (err E)
  (success ok)        ; the FIRST constructor; try continues on it
  (relevant))         ; a value of this type must be used (§7)
```

- **`(success c)`** names one of the declaration's constructors, the value of the monad. A variant has
  at most one `success`.
- **`(relevant)`** says the structural rule of weakening is denied for this type's values (§7).

`success` and `relevant` are therefore **reserved as constructor names** in a variant declaration, so
that `(success ok)` cannot be read as a constructor `success` carrying an `ok`.

A + R is the **exception monad** with E = R: return is the success constructor, and bind is `try`. The
monad laws (§5) depend on nothing but this shape, so they hold for every model.

The models today:

| model | declared in | success | relevant |
|---|---|---|---|
| `(result T E)`: `ok`, `err` | `lib/result` | `ok` | yes |
| `(option T)`: `some`, `none` | `lang` | `some` | no: a map read may be ignored |
| any program's own variant, such as `(read T)`: `got`, `not-found`, `denied`, `failed` | the program | as declared | as declared |

So `try` works on a map read, `(try (some v) (m k) …)`, and on a flat sum per operation. A sum of sums and a flat
sum are the same set, A + (K₁ + K₂) ≅ A + K₁ + K₂, and both are models.

---

## 3. The result type, in a library

```lisp
; lib/result.oro
(module result)
(export result ok err err-map)
(variant (result T E) (ok T) (err E) (success ok) (relevant))

; result is a bifunctor in E: err-map f = id + f
(def err-map (f r) (case r (ok v) (ok v) (err e) (err (f e))))
```

`(result T E)` is the coproduct T + E. **It is not in `lang`**: hamza chose a library, and the compiler
needs none, because everything it does reads the markers (§2), not the name.

**`err-map` is functoriality**: err-map id = id, and err-map (g ∘ f) = err-map g ∘ err-map f. It moves a
result between error types, where one module's errors meet another's (§5).

**Why a sum and not the product.** As sets, {(t, e) ∈ T_⊥ × E′ | e = nil ⇔ t ≠ ⊥} ≅ T + E. The product
restricted by its rule is the coproduct encoded, and an encoding admits the states its rule excludes
(errors-2026-10-04 §2).

---

## 4. A fallible host call

### 4.1 The encoding is a property of the host's error type: a niche

A host does not give the coproduct. **Go's `error` is 1 + E**: `nil` is the 1, absence, encoded in the
type's own value space. That is a **niche**, the representation sums.md §9 names as still open. The
target states it once, for the type:

```lisp
(repr error (niche (host expr "%s == nil")))      ; Go
(repr error (niche (host expr "%s === null")))    ; JavaScript (the try/catch templates give null)
(repr error (niche (host expr "%s == null")))     ; Java
```

So Go's `(T, error)` is T × (1 + E). A declaration whose result is a model A + R with exactly one
non-success constructor, whose payload is E, **means the host's product composed with the retraction**

  r : T × (1 + E) → T + E,   r(t, none) = success t,   r(t, some e) = the other constructor e.

Declarations need nothing else:

```lisp
(use result)
(sig ReadFile ((name bytestring)) (result (array (int 0 255)) error)
     (host expr "os.ReadFile(%s)" (import "os")))
```

### 4.2 What the loader makes of it: δ, nothing new

The target loader turns such a declaration into two things:
- the **raw host call**, under a name a program cannot write (`ReadFile#raw`), declaring the host's
  product `(tuple T error)` as today;
- a **definition** under the host's own name that applies r, in the sum's Church encoding:

  ```lisp
  (def ReadFile (name)
    ((ReadFile#raw name) (fn (t e)
      (if (niche-test e) (fn (#k) (#k s# t)) (fn (#k) (#k c# e))))))
  ```

  where s# and c# are the tags of the success constructor and of the one other constructor, which the
  loader reads off the declared variant.

A definition in a target is `D_T` (target-system.md §6.2), unfolded by δ like any other. So building
the sum needs no new pass, no new term kind and no backend change. Where the sum is eliminated in the
same program, reduction removes it and the host's own test is what remains (§11).

### 4.3 A partial success is a product with an option

Some host calls succeed in part. `io.Reader.Read` returns n bytes **and** an error, and Go's
documentation says to process the n bytes first. Its honest type is ℕ × (1 + E), and the niche gives it
directly:

```lisp
(sig Read ((r Reader) (p (buffer (int 0 255)))) (tuple (int 0 9223372036854775807) (option error))
     (host expr "%s.Read(%s)"))
```

An `(option E)` component of a host result is read through the niche: `none` when it is absent,
`(some e)` otherwise. The declaration states which shape the host has, from the host's documentation:
`ReadFile` is a sum, `Read` a partial product, and `ParseInt` a sum of products (`ErrRange` comes with
the saturated value). That is ADR 0022's rule for every claim a declaration makes.

### 4.4 A sentinel that varies by function

Win32 signals failure by a sentinel result that differs per function (0, `NULL`, `INVALID_HANDLE_VALUE`,
−1), with the error read from `GetLastError`. A type-wide niche cannot say that, so the declaration says
it:

```lisp
(sig WriteFile ((h handle) (p (array (int 0 255)))) (result (int 0 4294967295) win32-error)
     (host expr "…")
     (fails (sentinel 0) (error GetLastError)))
```

`(fails …)` is the per-declaration form of a niche, and only a host without a type-wide niche needs it.

### 4.5 Declared truthfully

A call that can fail **must** be declared fallible. A host call that throws while declared total, or
`pure` and partial (Java's `parseLong`, errors-2026-10-04 §1), breaks this rule. The audit that finds
them comes with the browser and Android `os` (ADR 0039, §12).

---

## 5. Passing failure on: `try`

```
(try (s x) e body)  ⟶  (case e  (s x) body  (c₁ y₁) (c₁ y₁)  …  (cₙ yₙ) (cₙ yₙ))
```

`try` is the **bind** of the exception monad. Its pattern names the success constructor s, which is
how the language types a sum: by the constructors a program writes, as `case` does (sums.md). So the
variant is resolved where the program is loaded, s is checked to be its marked `success` (a `try` on an
unmarked variant, or naming another constructor, is refused), and the `try` expands to an ordinary,
exhaustive `case`: on s it runs the body with the payload, and every other constructor cᵢ is rebuilt
unchanged. Typing: e : A + R and body : B + R with x : A, give B + R.

**The monad laws are the refactorings** a program relies on:

```
(try (s x) (s v) body)         =  body[v/x]                                  left identity
(try (s x) e (s x))            =  e                                          right identity
(try (s y) (try (s x) e M) N)  =  (try (s x) e (try (s y) M N))   x ∉ fv(N)    associativity
```

Inlining a helper, splitting one, and reassociating a chain of fallible steps change nothing.

**It costs nothing inside a program.** Every call is inlined, and case-of-case fuses each `try` with the
constructor it meets, folding the test on the known tag. Measured over three levels: one branch per
error path, no tag (errors-2026-10-04 §3).

**One error type per chain.** Every `try` in a chain shares R. Moving between error types is
`err-map` (§3). The union of error sets, ordered by inclusion with join as union (Zig's inferred error
sets), needs structural sums, and the language's are nominal and closed. Most chains stay inside one
module's errors (`os` returns `os-error`), so `err-map` appears only where modules meet. If programs
fill with it, the union is the change.

---

## 6. Kinds: what an error carries

An error type is chosen per module:
- **opaque**: the host's own error, printable and comparable only by the host's functions;
- **kinds**: a closed variant of the failures a program can act on, with the host's error kept as the
  residue.

**The portable `os`** (`lib/os`) returns kinds:

```lisp
(variant os-error not-found exists permission (other host-error))
```

They are the three portable sentinels Go's own `io/fs` declares, and each maps onto the other hosts:

| kind | Go | JavaScript (Node) | Java |
|---|---|---|---|
| `not-found` | `fs.ErrNotExist` | `ENOENT` | `NoSuchFileException` |
| `exists` | `fs.ErrExist` | `EEXIST` | `FileAlreadyExistsException` |
| `permission` | `fs.ErrPermission` | `EACCES`, `EPERM` | `AccessDeniedException` |
| `other e` | everything else, with the host's error | | |

Left out: **`is-directory`**, because Go has no portable sentinel for it and Java often reports a bare
`IOException`; **`fs.ErrInvalid` and `ErrClosed`**, because they are bugs, which the language handles as
obligations and linearity.

**Classification is a surjection** h : E_host → K, the kinds plus the residue, written once per host
where the host's errors enter `os`. That the same failure lands in the same kind on every host is a
claim, checked by a program (errors-research M3) as `os` is checked by `roundtrip`.

---

## 7. An error must be used: relevance

**A value of a `relevant` type must be used.** Bound and never read, or computed in statement position,
it is refused:

```lisp
(seq (os.write-file path data) …)            ; refused: the result is never looked at
(seq (ignore (os.write-file path data)) …)   ; accepted: the discard is written down
```

This is the structural rule of weakening, denied for one declared type: relevant, used at least once,
not affine. ADR 0010 already denies weakening for impure terms, which are let-bound and never dropped,
so a fallible host call's result is a binder in the residual, and **its binder must occur**. It is
checked on the residual by occurrence counting, as a buffer's linearity is (ADR 0018), at every binder
of a host call whose declared result is a relevant model. `ignore` is a language name for the discard:
its argument's effects run, and nothing reads its value.

**Where the host's algebra makes an error redundant, the declaration says so.** A `bufio.Writer`'s
errors are sticky: the first is kept, and `Flush` returns it. The errors form a monoid in which the
first absorbs the rest, so a write's error is subsumed by `Flush`'s. Such a write is declared **not**
fallible, and `Flush` fallible, which moves the obligation to where the host reports it.

**Not checked, named:** a relevant value made by a **pure** definition and then dropped. β drops an
unused pure argument before the checker runs, and the reducer does not see types. The fallible values
that matter come from host calls, which are impure and so always bound.

---

## 8. Giving up on purpose: `expect`

```
(expect (s x) e why body)  ⟶  (case e  (s x) body  (c₁ y₁) (abandon why)  …  (cₙ yₙ) (abandon why))
```

`expect` is total: on the success summand it binds the payload and runs the body, and on any other it
is crash : R → 0, ending the process with the reason. Nothing is proven, and the source says so.
Erlang's `{ok, Bin} = file:read_file(F)` and Rust's `.expect("…")` are this. Its pattern names the
success constructor, checked as `try`'s is.

**`abandon`** is the crash. It is a language name every target realizes (§9), of type string → 0, and
impure, so it is never moved or dropped. Its value is in 0, the initial object, and three rules follow
from the one arrow 0 → Y that exists for every Y (built in expect-2026-10-06):

- **it absorbs its context.** k ∘ abandon = abandon for every continuation k, since any two arrows out
  of 0 are equal. It is the commuting conversion at arity 0: 0 is the empty coproduct, its eliminator is
  the empty copairing [], and K ∘ [g₁, …, gₙ] = [K ∘ g₁, …, K ∘ gₙ] at n = 0 is K ∘ [] = []. So an
  application with a crash in a strict position, an argument that is not a λ or an `if`'s condition,
  reduces to the crash, keeping the effects evaluated before it in order (ADR 0010):

  ```
  (f a₁ … aₖ₋₁ (abandon w) …)  ⟶  (let aᵢ (fn (_) … (abandon w)))   for each impure aᵢ, i < k
  ```

  `(let (abandon w) k)` and `(seq (abandon w) k)` are instances. A crash in an `if`'s branch or under a
  λ is not evaluated there and stays;
- **it types as anything.** The checker gives it no type of its own, so it agrees with every demand, and
  a conditional with a crashing branch has the other branch's type;
- **it ends its region.** The IR's terminator `unreachable` (spec/ir.md §1.3) follows the call, and the
  verifier's W3 refuses one whose region does not end in it. Such a region yields nothing, so a function
  whose every path crashes takes its declared results: 0 → R exists.

**Inside a loop, a success arm may hold `again`** ([ADR 0041](../decisions/0041-a-success-arm-is-a-tail.md)).
It binds the payload and runs once, now, so it is a binding's tail, as a host call's continuation is
(ADR 0027). The other arms leave the loop: `try`'s with the failure as the loop's value, `expect`'s into 0.
So the clause list is still every back edge. A loop yields the sum `try` leaves as two results, because a
constructor is the tuple (tag, payload) (sums.md §1):

```lisp
(loop ((i 0) (acc 0))
  (>= i n)  (result.ok acc)
  else      (try (result.ok h) (half (* 2 i))
              (again (+ i 1) (+ acc h))))
```

**So abandonment is never implicit.** It happens at an `expect`, at an operation `-checked` turned into a
trap, or at a host failure declared abandoning. Each is visible in the source or the build line.

A proof-based projection, an `ok!` discharged as an obligation, is **not** in the language: a host call
is never evaluated at compile time (ADR 0009), so its tag is known only after a `case` or a guard, where
the `case` already reduces it away. It would be a construct the compiler nearly always refuses.

---

## 9. Abandonment, on each target

| target | `abandon why` |
|---|---|
| Go | `panic(why)` |
| JavaScript | `throw new Error(why)` |
| Java | `throw new IllegalStateException(why)` |
| Windows | `why`'s bytes and a newline to standard error, `WriteFile(GetStdHandle(-12), …)`, then `ExitProcess(1)` |

Every target writes the reason. Windows writes its bytes as they are, which a console shows in its code
page (printing-research §1's open question, as for every string there). Its template is the one
assembly template that calls several functions, `(import "GetStdHandle" "WriteFile" "ExitProcess")`
(target-files.md), and it may use any register, because nothing after it reads one.

The differential suite checks the property that matters: every target prints the same answers and
then crashes at the same point with the same reason. A case names it, `; abandons: REASON`, and a run
that stops with it on standard error answers `abandon` (gauntlet/differential/cases/expect.oro).

The status and the message's spelling differ by host, and the difference is observable, so a crash's
text carries no portability claim: it is Tier 2, as an arithmetic trap's already is.

---

## 10. Laws, and what they buy

| law | where it holds | what it buys |
|---|---|---|
| β for the coproduct: a `case` on a constructor is its arm | always | a sum built and eliminated in one program leaves nothing (§11) |
| the monad laws for `try`, in every model of §2 | always | refactoring fallible code is safe |
| associativity, A + (K₁ + K₂) ≅ A + K₁ + K₂ | always | a result of kinds costs what a flat sum costs |
| functoriality of `err-map` | always | moving between error types composes |
| relevance: a relevant binder occurs | checked | no silent discard of a host's error |
| classification h : E_host → K is total | per host, checked by a program | the portable `os`'s kinds mean one thing |

---

## 11. What is emitted

Nothing new. A fallible host call unfolds by δ to its raw call and r (§4.2); `case`, `try` and `expect`
expand to applications of the value; β and case-of-case reduce the constructors away; and what remains
is the host's own test:

```go
v12, v13 := os.ReadFile(v11)
if v13 == nil { … } else if os.IsNotExist(v13) { … } else if os.IsPermission(v13) { … } else { … }
```

A model's value crossing an export's boundary is a tag and a payload, as every sum is (sums.md §6).

---

## 12. Building it, in order

1. **The markers** `success` and `relevant` in variant declarations, with their checks; `option` marks
   `some`.
2. **`lib/result`**: the variant and `err-map`.
3. **`try`, `expect`, `abandon`**: the expansions into `case`, and `abandon` on the four targets, with
   its three rules (§8). **Built** (sumofsums-2026-10-06, expect-2026-10-06). `ignore` moves to step 6,
   where relevance gives it something to do.
4. **The niche** in target files and the loader's δ (§4.2); `(fails …)` for Win32 (§4.4).
5. **The migration**:
   - the 69 fallible Go declarations, each with its shape (sum, or partial product), read from Go's
     documentation (gocoverage-2026-10-06);
   - the portable `os` and `io` with kinds and classifications;
   - the corpus's 16 fallible call sites (errors-2026-10-04 §1).

   Where a program tested only `err-nil`, the emitted code should not change, and the emission gate
   checks it.
6. **Relevance** (§7) and `ignore`, with a planted fault.
7. **Later, with the browser and Android `os`** (ADR 0039): the audit of JavaScript and Java declarations
   that throw while declared total or `pure`.

Each step keeps the emission gate, and each new rule has a planted fault that fails.

---

## 13. Not here

- **Inferred error sets** (Zig): the union of a chain's error sets. `err-map` until programs need more.
- **Resource management** (`defer`, `errdefer`): it needs handles the language does not have. This
  specification keeps error paths explicit, which checking it will need.
- **Asynchronous failure**: a goroutine's panic, a promise's rejection. Concurrency's, as messages to a
  supervisor.
- **Restarts** (Common Lisp): a static function parameter already expresses the caller's policy
  (errors-research §2.6).
- **Validation**, collecting every error rather than the first: an applicative over a semigroup R, a
  library.
- **Relevance for pure values** (§7).
