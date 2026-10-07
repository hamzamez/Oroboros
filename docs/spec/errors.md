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

**A call that succeeds with nothing is `(result (tuple) E)`**, 1 + E: `(tuple)` is the unit, the
terminal object (data.md §3.6), so one `result` serves every fallible call. Its payload occupies no
slot, so the representation is `(tag, E)`. hamza chose this over a second variant for 1 + E, because
the algebra has one type here, not two.

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

**Built for Go** (niche-2026-10-06, `emit/retract.go`). The construction is compositional. Write H(R)
for the host values behind a declared result R, and r for the term that rebuilds R from them:

```
H(T) = [T]                          H((tuple)) = []
H((option E)) = [E]                 r(e) = none if niche(e), else (some e)
H((V T E)) = H(T) ++ [E]            r(t̄, e) = s (r t̄) if niche(e), else c e
H((tuple R₁ … Rₙ)) = H(R₁) ++ … ++ H(Rₙ)
```

H is a functor on result shapes and r is natural in each factor, so one rule covers a sum, the unit with
an error, a partial product and their composites. What the build fixed in detail:
- the raw call is `#raw:MODULE:NAME` and a niche's test `#niche:TYPE`. A `#` keeps a program from
  writing them, and they contain no `.`, because core reads a dotted name as an import's alias;
- the module declaring a fallible call `(use result)`s, and the loader reads the variant from the
  library path, which a target loaded by directory takes from the `lib` beside its `targets`;
- a target library's `use` joins its module's imports, so core closes the imports again after gluing
  `D_T` in (core/reduce.go, `closeImports`);
- a view (`implements`) is a fact about the host's methods, so it finds a fallible method under its raw
  name;
- **a generated declaration stays the host's product.** Whether a call is a sum or a partial success is
  a claim about its documentation, which a generator cannot read (ADR 0023), so the survey keeps
  `(T, error)` and the hand file states the shape.

`gauntlet/stdlib/acceptance/errors-os.oro` runs all three shapes on Go, and what Go runs is
`f, err := os.Open(…); if err == nil { … }`.

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

**Classification is a surjection** h : E_host → K, the kinds plus the residue. That the same failure
lands in the same kind on every host is a claim, checked by a program (errors-research M3) as `os` is
checked by `roundtrip`.

**Built** (oskinds-2026-10-06). A portable call is (id + h) ∘ raw, where `id + h` is
`result.err-map h`. So `os` is a library module, `lib/os.oro`, that declares `os-error` and defines
each name once over `os/host`, each host's cell. **h is written once**, as a case analysis over three
predicates each cell declares (`os.IsNotExist`, Node's `code`, Java's exception classes): what
differs by host is the raw call and the predicates, and nothing else. The variant is
`(variant os-error not-found exists permission (other error))`, `error` being each target's own. On
Go, JavaScript and Java, `examples/io/roundtrip.oro` reads a missing path and prints `not-found`.

---

## 7. An error must be used: relevance

**A value of a `relevant` type must be used.** Bound and never read, or computed in statement position,
it is refused:

```lisp
(seq (os.WriteFile path data) …)            ; refused: the result is never looked at
(seq (ignore (os.WriteFile path data)) …)   ; accepted: the discard is written down
```

### 7.1 The rule: weakening, denied for one type

The structural rules of a context are weakening (a variable may go unused), contraction (used twice)
and exchange. A type without weakening is **relevant**: used at least once, not affine (Anderson and
Belnap's relevance logic; Walker, *Substructural Type Systems*, ATTAPL ch. 1, 2005). For x : A with A
relevant,

```
Γ, x : A ⊢ M : B      only if  x is used in M on every path
```

**On every path**, because the eliminators are additive. A coproduct's elimination [f, g] : A + B → C
takes f and g in ONE context Γ (that is its universal property), so a variable of Γ is used by
`(if c a b)` when c uses it, or when a and b both do. `bool = 1 + 1`, so `if` is that rule, and `case`,
`try`, `and` and `cond` are built from it. Two forms use every variable vacuously:

- **`(abandon w)`**, whose type is 0, the initial object. A path that reaches it never returns, and
  the reducer already says so: a crash absorbs every strict context (§8), the commuting conversion at
  arity 0. In linear logic, 0 on the left proves any sequent, whatever its context.
- **`(again a…)`**, a back edge. A path around a loop continues into the next iteration, and every
  terminating execution leaves by an exit, so a loop uses x when each of its exits does (or each
  iteration before its `again`). A divergent path uses nothing, vacuously, as a crash does.

So `(if verbose (print-err r) 0)` is refused, because on the quiet path the error goes unread: the
program writes `(if verbose (print-err r) (ignore r))`. And `(let w (os.WriteFile a d) (try (ok u)
(os.WriteFile b d) (check w)))` is refused, because when the second write fails, `try` returns its
error and the first one's is lost.

### 7.2 Where it is checked: where a binder meets its value

The reducer is untyped, and the type checker sees only the residual, where a relevant value no longer
exists: η for products has taken it apart, and the laws of the conditional (state.md) have erased tests
whose arms agree. Relevance is not invariant under those equations, `(seq r B)` and
`(seq (case r (ok _) 0 (err _) 0) B)` being equal and only one of them written by someone who looked.
So it is checked on the term as written, at the one moment the binder and its value are both in hand:
**β**, when `((fn (x…) M) a…)` is reduced and each aᵢ is known.

- **What is relevant is a value, by its constructors.** A constructor of a `(relevant)` variant builds
  its tuple with the binder `#k!` where every other tuple has `#k` (ADR 0041's encoding, the same term
  otherwise). A term is relevant when one of its tails is such a tuple; in a typed program every tail
  has the one type, so one suffices. The tails are walked through the forms η for products walks:
  `if`, a residual `let`, a host call's continuation, a scope, a loop's exits. A fallible host
  declaration's retraction (§4.2) builds its result with the model's constructors, so a host call is
  relevant exactly when its declared result is a relevant model, and so is every definition over one,
  such as the portable `os`.
- **An impure argument** is normalised before it is bound (ADR 0010: let-bound, never substituted), so
  its value's tails are known. **A pure one** is substituted unnormalised, and is relevant only when it
  is a constructor's tuple, or tails into one, as written: `(seq (result.ok 1) B)` is refused, and a
  pure computation whose relevance shows only after reduction is not checked (§7.4).
- **The binder is the program's**: a name it wrote, or `seq`'s and a pattern's `_` (`#_`, `#_N`,
  binding.md). The binders the compiler makes, a `case`'s slots, η's components, a conversion's
  continuation, bind values whose use the program already wrote.
- **Its body is the λ as written**: the literal λ of a `let` or `seq`, or a definition's body before
  reduction. The body after reduction is not the program's: the conditional's laws erase tests.
- **Handing it on is a use exactly when the receiver uses it.** Passed bare to a λ or a definition,
  `(drop r)`, the variable is used when that parameter is used on every path of the receiver's body as
  written. A definition's parameter is read off its body once, and the inference terminates because no
  definition reaches itself (ADR 0014). It is needed because reduction is normal order under binders:
  a λ's body is reduced before the λ meets its argument, so `drop`'s β has already turned `(drop r)`
  into `0` while `r` was still abstract. Passed to a host call, or to a function computed at run time,
  the value escapes, and that counts as a use.

Relevance therefore flows through definitions with no annotation. A definition's parameter is checked
at each call that hands it a relevant value, where the type is known: the residual is monomorphic, and
this is its pre-image. This is usage inference, the analysis Linear Haskell's multiplicities make
explicit (Bernardy et al., POPL 2018).

### 7.3 `ignore` is the map to the terminal object

`(ignore e)` evaluates e and has the unit as its value: it is !_A : A → 1, the unique arrow into the
terminal object (data.md §3.6). The language must have it as a constant, because a relevant calculus
cannot define it: `(fn (x) (tuple))` is exactly the λ the rule refuses.

Its reduction is the universal property. Any two arrows into 1 are equal, so on a product or a sum,
whose value is a tuple, ! is its eliminator with a constant body:

```
(ignore e)  ⟶  (e (fn (x̄) (tuple)))      e's tails are n-tuples, relevant or not
(ignore e)  ⟶  (tuple)                   e pure, of any other type
(ignore e)  ⟶  (let e (fn (_) (tuple)))  e impure, of any other type: its effects stay (ADR 0010)
```

On a host call's result the eliminator then reduces as a `case` does: case-of-case pushes it into the
niche test's arms, and the test, whose arms now agree, goes by idempotence. Nothing is emitted for it.
The unit is then bound by `seq`, and **η for the terminal object** takes that binding apart into no
components: r = () for every r : 1, the n = 0 case of η for products. So
`(seq (ignore (f.Close)) B)` emits what `(seq (f.Close) B)` emitted before this step. `ignore` never
reaches a target: it is the language's, like `let`, and a target may not declare it.

### 7.4 Not checked, named

- **A relevant value produced by a pure computation** and passed unnormalised: `(seq (pick c) B)` with
  `pick` returning `(ok 1)` or `(err 2)`. β substitutes a pure argument without reducing it, and
  reducing a dropped one only to find its type would compile terms the program never runs. Every value
  that matters comes from a host call, which is impure, so is normalised, so is checked.
- **A relevant payload** left unread in a `case` arm, a result of results. The pattern's binder becomes
  a slot, which the compiler made.
- **A hand-off to a function computed at run time**, or to a function parameter: the receiver's body is
  not known where the value is bound, so the value is taken to escape. A closure may not survive
  staging, so such a receiver is rare in a residual, and the case is named rather than solved.
- **A loop variable** holding a relevant value: `loop` binds its variables itself, not by β.
- **A λ never applied**: a use inside it counts. A closure may not survive staging, so a λ that is
  never applied is a pure value, which β drops.

**Where the host's algebra makes an error redundant, the declaration says so.** A `bufio.Writer`'s
errors are sticky: the first is kept, and `Flush` returns it. The errors form a monoid in which the
first absorbs the rest, so a write's error is subsumed by `Flush`'s. Such a write is declared **not**
fallible, and `Flush` fallible, which moves the obligation to where the host reports it.

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
| relevance: a relevant binder is used on every path (§7.1) | checked at β, on the term as written | no silent discard of a host's error |
| `ignore` is !_A, the unique arrow into 1 | its reduction is the universal property | a discard costs nothing, and is written down |
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
4. **The niche** in target files and the loader's δ (§4.2); `(fails …)` for Win32 (§4.4). **Built on Go**
   (niche-2026-10-06), with `os.Open`, `File.Read` and `File.Close` declared in the model. JavaScript's
   and Java's niches come with the portable `os` in step 5, where a program calls a declaration that
   reads them; `(fails …)` with the first Win32 declaration that needs it.
5. **The migration**. **Built** (oskinds-2026-10-06, gomigrate-2026-10-06): the portable `os` with
   kinds on three hosts, the JavaScript and Java niches, the six corpus programs that called it (of
   which `wc`, `jsonfmt` and `freq` emit byte-identical code), and every fallible declaration in
   `targets/go` with its shape. As planned:
   - the 69 fallible Go declarations, each with its shape (sum, or partial product), read from Go's
     documentation (gocoverage-2026-10-06);
   - the portable `os` and `io` with kinds and classifications;
   - the corpus's 16 fallible call sites (errors-2026-10-04 §1).

   Where a program tested only `err-nil`, the emitted code should not change, and the emission gate
   checks it.
6. **Relevance** (§7) and `ignore`, with a planted fault. **Built** (relevance-2026-10-07): the
   check at β, additive over the eliminators, with usage inference through definitions; `ignore` as
   !_A; η for the terminal object. Nine planted faults, one per rule, each caught.
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
