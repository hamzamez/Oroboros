# Six notations, side by side

Research, 2026-10-10, on hamza's request: *"remember we want our language to be easy for humans and
llms to read and reason about. I can speak for humans, I will leave the llm side to you. I noticed
that llms code well with s-expressions even when using an obscure language. the new research would
take what you did just now in notation-research.md and what it would look like if used a syntax like
F#, one of the cleanest I have ever used. and also what would it look like if we took it back to
s-expressions, maybe multiple s-expression syntaxes to compare from pure and simple, to one with
sprinkles like shen and clojure. lots of examples side by side so we can compare. and what about
untyped lambda, I know we kept staging, but did we loose that? answer in the research."*

It follows [notation-research.md](notation-research.md), whose semantics (the five unifications,
settypes-research's logic, ADR 0049) every notation here writes the same. Only the spelling differs.
Nothing is decided.

## 0. The answer, in brief

1. **The untyped λ is not lost, and ADR 0049 makes that structural** (§1). Since then, a term
   carries no type: types are sets *proved of* terms, which is Curry's extrinsic typing, not Church's
   intrinsic typing. The computation is the untyped λ-calculus, as the atom says. Measured on today's
   compiler:
   - **the static level keeps it whole, for every term with a normal form.** Church numerals, Church
     booleans, `compose` and `twice` reduce at compile time to `(fmt.Println (array 5 6 1 42 12))`;
   - **a fixed-point combinator diverges**: the reducer normalizes under binders, so Z unfolds
     forever. That is ADR 0014's "no recursion", kept by the static level too, and §1.3 says what
     relaxing it would cost.

   Every notation below writes λ as a first-class form, and none of them puts a type on a term.
2. **Six notations, the same eight programs in each** (§3): the mathematical one, F#'s with F\*'s
   refinements, four kinds of S-expressions (pure, today's, Clojure's, Shen's), and a measurement of
   each (§4).
3. **The LLM side** (§5), argued from how a model reads and writes code, and labelled where it is not
   measured:
   - **what transfers to an obscure language is its shape**, not its words;
   - **S-expressions have one shape**, the most-trained after C-like and Python-like ones, and no
     precedence. Their one real failure is balancing closing parentheses deep in a form, which a
     formatter and the reader catch mechanically;
   - **an ML shape (F#) transfers almost as well.** Its real hazard is a nested `match` swallowing
     the cases after it;
   - **TLA+'s notation transfers least**: it is rare in training data, and its Unicode, precedence and
     bullet alignment are each a new place to err;
   - **for reading claims, mathematical notation is the clearest**, for a model as for a person.
4. **A synthesis worth deciding on** (§6): one source, two renderings.
   - **Write** S-expressions, regular, mechanically checkable, comments as gaps;
   - **read** them rendered in mathematics: documents, error messages, and an editor's view.

   That is Lamport's own split, ASCII source and pretty-printed mathematics, and it serves both
   readers without asking either to write the other's notation. §7 proposes the experiment that would
   measure the LLM half instead of arguing it.

## 1. The untyped λ

### 1.1 What the language is

The atom (the-atom.md) is the λ-calculus with δ, whose normal form is a parameter: a target says
which names are primitive, and reduction runs until only primitives remain. The calculus has no types.
ADR 0049 put the types in a separate language, the logic, where a type is a set and "e ∈ S" is a claim
proven about e. That is the distinction between **Church-style** (intrinsic) typing, where a term is
typed by construction, `λx:A. e` (Church 1940), and **Curry-style** (extrinsic) typing, where terms are
untyped and types are assigned to them (Curry and Feys, *Combinatory Logic*, 1958). Reynolds names the
two and shows they meet ("The meaning of types: from intrinsic to extrinsic semantics", 2000). **Ours
is Curry's**: a term's meaning does not depend on any claim, and erasing every claim changes nothing
(settypes §1.3).

### 1.2 Measured: what the static level reduces

```lisp
(def two     (fn (s) (fn (z) (s (s z)))))
(def three   (fn (s) (fn (z) (s (s (s z))))))
(def plus    (fn (m) (fn (n) (fn (s) (fn (z) ((m s) ((n s) z)))))))
(def times   (fn (m) (fn (n) (fn (s) (m (n s))))))
(def to-int  (fn (c) ((c (fn (k) (+ k 1))) 0)))
(def church-true (fn (a) (fn (b) a)))
(def compose (fn (f g) (fn (x) (f (g x)))))
(def twice   (fn (f) (fn (x) (f (f x)))))
(def main () (fmt.Println (to-int ((plus two) three)) (to-int ((times two) three))
                          ((church-true 1) 2)
                          ((compose (fn (x) (* x 2)) (fn (x) (+ x 1))) 20)
                          ((twice (twice (fn (x) (+ x 3)))) 0)))
```

`go run ./cmd/oro -target=go` reduces `main` to `(fn () (go/fmt.Println (array 5 6 1 42 12)))`,
and the binary prints `5 6 1 42 12`. Church arithmetic, Church booleans, composition and a function
applied four times all disappear at compile time. The static level is the untyped λ-calculus, and
staging is its normalization.

### 1.3 The one boundary: a term without a normal form

```lisp
(def Z (fn (f) ((fn (x) (f (fn (v) ((x x) v)))) (fn (x) (f (fn (v) ((x x) v)))))))
(def fact (Z (fn (self) (fn (n) (if (= n 0) 1 (* n (self (- n 1))))))))
```

This does not reduce: *"reduction did not terminate within the step limit"*. The reducer computes the
**normal form**, reducing under binders, because a function body must be normalized for the residual
to be code. Z's body `(fn (v) ((x x) v))` then unfolds without end, before `fact` is applied to
anything. Under weak reduction (no reduction under λ) Z works, which is how Scheme runs it.

That is ADR 0014 seen from the static level. A program's static part must have a normal form, so
general recursion is out there too. It was kept out on purpose: a static level that may compute
without end is C++ templates' Turing-complete compile time, where a program's meaning can depend on
when the compiler gives up. Two ways exist to have more, if a program ever asks:
- **reduce weakly where a λ is not yet applied, and residualize what remains.** Z then terminates
  whenever its recursion is on static data. It is a change to the reducer's strategy (lazy
  specialization, Jones, Gomard and Sestoft, *Partial Evaluation*, 1993), and it needs a termination
  argument on static data, which size-change termination already provides for loops.
- **keep recursion out**, and use `loop` for the dynamic level and folds for the static one, as now.

### 1.4 What each notation must keep

Each notation below writes λ as a form of its own and puts no type on a term: claims are optional
sets on parameters, never part of the λ. In §3, example 8 is the untyped λ in each notation.

## 2. The six notations

| | notation | from |
|---|---|---|
| **M** | mathematical | notation-research: TLA+'s with PlusCal's state, Dijkstra's guards |
| **F** | F#, with F\*'s refinements | F#'s light syntax (offside rule, `let`, `match … with`, `fun`); F\*, which grew out of F#, for `x:int{x <> 0}` and `requires`/`ensures` |
| **P** | pure S-expressions | only parentheses and symbols; Scheme's `define`/`lambda`; Typed Racket's `(: f T)` and `(U …)` |
| **T** | today's Oroboros | the reader as it is, extended naturally for the set operations it lacks (marked †) |
| **C** | Clojure-flavored | `[…]` for binders, `{…}` for maps and clauses, `#{…}` for sets, `:keywords`, `loop`/`recur`, Plumatic Schema's `:-` claims |
| **S** | Shen-flavored | `define` with pattern rules `X -> e`, uppercase variables, `where` guards, `{A --> B}` signatures, sequent-rule `datatype`s |

Each example in §3 is the same program in the six, in that order.

## 3. Eight programs

### Example 1 — claims: a nonzero divisor, a predicate, set types

```tla
Number  == Int ∪ F64
Method  == {"GET", "POST", "PUT"}
NonZero == Int \ {0}
Even    == {n ∈ Int : n % 2 = 0}

div(a ∈ Int, b ∈ NonZero) ∈ Int == a ÷ b
space(b) == b = 32 ∨ b ∈ 9..13
```

```fsharp
type Number  = Int | F64
type Method  = "GET" | "POST" | "PUT"
type NonZero = n: Int { n <> 0 }
type Even    = n: Int { n % 2 = 0 }

let div (a: Int) (b: NonZero) : Int = a / b
let space b = b = 32 || b in 9..13
```

```scheme
(define-type Number  (union Int F64))
(define-type Method  (one-of "GET" "POST" "PUT"))
(define-type NonZero (minus Int (one-of 0)))
(define-type Even    (refine (n Int) (= (mod n 2) 0)))

(: div (-> Int NonZero Int))
(define (div a b) (quot a b))
(define (space b) (or (= b 32) (in b (range 9 13))))
```

```lisp
(def number   (or int f64))                          ; †
(def method   (or "GET" "POST" "PUT"))               ; †
(def nonzero  (and int (not 0)))                     ; †
(def even     (where (n int) (= (% n 2) 0)))         ; †

(sig div ((a int) (b nonzero)) int)
(def div (a b) (/ a b))
(def space? (b) (or (= b 32) (and (>= b 9) (<= b 13))))
```

```clojure
(def Number  [:or Int F64])
(def Method  #{"GET" "POST" "PUT"})
(def NonZero [:and Int [:not= 0]])
(def Even    [:fn {:binder n} Int (= (mod n 2) 0)])

(defn div :- Int [a :- Int, b :- NonZero] (quot a b))
(defn space? [b] (or (= b 32) (<= 9 b 13)))
```

```shen
(datatype nonzero
  if (not (= X 0))
  X : int;
  ____________
  X : nonzero;)

(datatype number
  X : int;   ________   X : number;
  X : float; ________   X : number;)

(define div
  {int --> nonzero --> int}
  A B -> (/ A B))

(define space?
  B -> (or (= B 32) (and (>= B 9) (<= B 13))))
```

### Example 2 — a loop: the sum of a table

```tla
total(xs ∈ Seq(0..1000)) ∈ Nat ==
  LOOP i := 0, s := 0 :
      i ≥ Len(xs) → s
   [] OTHER       → AGAIN(i + 1, s + xs[i])
```

```fsharp
let total (xs: Seq<0..1000>) : Nat =
    loop i = 0, s = 0 with
    | i >= len xs -> s
    | _           -> again (i + 1) (s + xs[i])
```

```scheme
(: total (-> (Seq (range 0 1000)) Nat))
(define (total xs)
  (loop ((i 0) (s 0))
    (cond ((>= i (len xs)) s)
          (else (again (+ i 1) (+ s (ref xs i)))))))
```

```lisp
(sig total ((xs (array (int 0 1000)))) (int 0 +inf))
(def total (xs)
  (loop ((i 0) (s 0))
    (>= i (len xs)) s
    else            (again (+ i 1) (+ s (xs i)))))
```

```clojure
(defn total :- Nat [xs :- (Seq (range 0 1000))]
  (loop [i 0, s 0]
    (if (>= i (count xs))
      s
      (recur (inc i) (+ s (xs i))))))
```

```shen
(define total
  {(seq (0..1000)) --> nat}
  Xs -> (loop [I 0 S 0]
           (>= I (length Xs)) -> S
           _                  -> (again (+ I 1) (+ S (nth I Xs)))))
```

### Example 3 — a variant and its eliminator

```tla
VARIANT Result(T, E) == ok(T) | err(E)
  SUCCESS ok
  RELEVANT

step(n ∈ Int) ∈ Result(Int, Int) == IF n ≥ 10 THEN err(n) ELSE ok(2 * n)

run(n) ==
  CASE step(n) OF
      ok(v)  → v
   [] err(e) → e + 1000
```

```fsharp
[<Success(Ok); Relevant>]
type Result<'T, 'E> =
    | Ok of 'T
    | Err of 'E

let step (n: Int) : Result<Int, Int> = if n >= 10 then Err n else Ok (2 * n)

let run n =
    match step n with
    | Ok v  -> v
    | Err e -> e + 1000
```

```scheme
(define-variant (Result T E) (ok T) (err E) (success ok) (relevant))

(: step (-> Int (Result Int Int)))
(define (step n) (if (>= n 10) (err n) (ok (* 2 n))))

(define (run n)
  (case (step n)
    ((ok v)  v)
    ((err e) (+ e 1000))))
```

```lisp
(variant (result T E) (ok T) (err E) (success ok) (relevant))

(sig step ((n int)) (result int int))
(def step (n) (if (>= n 10) (err n) (ok (* 2 n))))

(def run (n)
  (case (step n)
    (ok v)  v
    (err e) (+ e 1000)))
```

```clojure
(defvariant Result [T E] (ok T) (err E) {:success ok :relevant true})

(defn step :- (Result Int Int) [n :- Int] (if (>= n 10) (err n) (ok (* 2 n))))

(defn run [n]
  (match (step n)
    (ok v)  v
    (err e) (+ e 1000)))
```

```shen
(datatype result
  X : A;  ________________  [ok X]  : (result A E);
  X : E;  ________________  [err X] : (result A E);)

(define step
  {int --> (result int int)}
  N -> [err N] where (>= N 10)
  N -> [ok (* 2 N)])

(define run
  N -> (case (step N)
         [ok V]  -> V
         [err E] -> (+ E 1000)))
```

### Example 4 — a union of sorts, taken apart

```tla
show(v ∈ Int ∪ F64 ∪ String) ∈ String ==
  CASE v OF
      i ∈ Int    → render(i)
   [] f ∈ F64    → format(f)
   [] s ∈ String → s
```

```fsharp
let show (v: Int | F64 | String) : String =
    match v with
    | :? Int as i    -> render i
    | :? F64 as f    -> format f
    | :? String as s -> s
```

```scheme
(: show (-> (union Int F64 String) String))
(define (show v)
  (case-sort v
    ((Int i)    (render i))
    ((F64 f)    (format f))
    ((String s) s)))
```

```lisp
(sig show ((v (or int f64 string))) string)          ; †
(def show (v)
  (case v
    (int i)    (render i)                            ; †
    (f64 f)    (format f)
    (string s) s))
```

```clojure
(defn show :- String [v :- [:or Int F64 String]]
  (condp instance? v
    Int    (render v)
    F64    (format v)
    String v))
```

```shen
(define show
  {number-or-string --> string}
  I -> (render I) where (integer? I)
  F -> (format F) where (float? F)
  S -> S)
```

### Example 5 — local state, and Go storing through cells (the core of `ages`)

```tla
LOCAL n := 0, sum := 0, best_name := "", best_age := -1 IN
LOOP k := 0 :
    ¬Scanner.Scan(in) → report(in, n, sum, best_name, best_age)
 [] OTHER →
      LET (name, age, got, e) == fmt.Sscan(Scanner.Text(in), OUT String, OUT Int) IN
      (IF ok(e) ∧ age ∈ 0..150
       THEN (n := n + 1; sum := sum + age;
             IF age > best_age THEN (best_name := name; best_age := age)));
      AGAIN(k + 1)
```

```fsharp
let mutable n = 0
let mutable sum = 0
let mutable bestName = ""
let mutable bestAge = -1
loop k = 0 with
| _ when not (Scanner.Scan input) -> report input n sum bestName bestAge
| _ ->
    let name, age, got, e = fmt.Sscan (Scanner.Text input, out String, out Int)
    if ok e && age in 0..150 then
        n <- n + 1
        sum <- sum + age
        if age > bestAge then
            bestName <- name
            bestAge <- age
    again (k + 1)
```

```scheme
(local ((n 0) (sum 0) (best-name "") (best-age -1))
  (loop ((k 0))
    (cond ((not (Scanner.Scan in)) (report in n sum best-name best-age))
          (else
           (let-values (((name age got e)
                         (fmt.Sscan (Scanner.Text in) (out String) (out Int))))
             (when (and (ok? e) (in age (range 0 150)))
               (set! n (+ n 1))
               (set! sum (+ sum age))
               (when (> age best-age)
                 (set! best-name name)
                 (set! best-age age)))
             (again (+ k 1)))))))
```

```lisp
(local n 0  sum 0  best-name ""  best-age -1
  (loop ((k 0))
    (not (Scanner.Scan in)) (report in n sum best-name best-age)
    else
      (let (tuple name age got e) (fmt.Sscan (Scanner.Text in) (out go.bytestring) (out int))
        (seq (if (and (ok? e) (>= age 0) (<= age 150))
                 (seq (set n (+ n 1))
                      (set sum (+ sum age))
                      (if (> age best-age)
                          (seq (set best-name name) (set best-age age))
                          (tuple)))
                 (tuple))
             (again (+ k 1))))))
```

```clojure
(local [n 0, sum 0, best-name "", best-age -1]
  (loop [k 0]
    (if-not (Scanner/Scan in)
      (report in n sum best-name best-age)
      (let [[name age got e] (fmt/Sscan (Scanner/Text in) (out String) (out Int))]
        (when (and (ok? e) (<= 0 age 150))
          (set! n (inc n))
          (set! sum (+ sum age))
          (when (> age best-age)
            (set! best-name name)
            (set! best-age age)))
        (recur (inc k))))))
```

```shen
(local [N 0 Sum 0 BestName "" BestAge -1]
  (loop [K 0]
    (not (scanner.scan In)) -> (report In N Sum BestName BestAge)
    _ -> (let [Name Age Got E] (fmt.sscan (scanner.text In) (out string) (out int))
           (do (if (and (ok? E) (in-range? Age 0 150))
                   (do (set N (+ N 1))
                       (set Sum (+ Sum Age))
                       (if (> Age BestAge)
                           (do (set BestName Name) (set BestAge Age))
                           skip))
                   skip)
               (again (+ K 1))))))
```

### Example 6 — host declarations (a target file)

```tla
EncodeRune(p ∈ Buffer(Byte), r ∈ Int32) ∈ 1..4
  WHERE 4 ≤ Len(p)
  == HOST expr "utf8.EncodeRune(%s, rune(%s))" IMPORT "unicode/utf8"

Sscan(s ∈ go.bytestring, a ∈ Seq(Cell(go.Value))) ∈ Nat × Result(Unit, error)  VARIADIC
  == HOST expr "fmt.Sscan(%s, %s...)" IMPORT "fmt"
```

```fsharp
[<Host "utf8.EncodeRune(%s, rune(%s))"; Import "unicode/utf8">]
val EncodeRune : p: Buffer<Byte> -> r: Int32 -> 1..4  requires (4 <= len p)

[<Host "fmt.Sscan(%s, %s...)"; Import "fmt"; Variadic>]
val Sscan : s: go.bytestring -> a: Seq<Cell<go.Value>> -> Nat * Result<Unit, error>
```

```scheme
(: EncodeRune (-> (Buffer Byte) Int32 (range 1 4)))
(define-host (EncodeRune p r)
  (where (<= 4 (len p)))
  (host expr "utf8.EncodeRune(%s, rune(%s))" (import "unicode/utf8")))

(: Sscan (-> go.bytestring (Seq (Cell go.Value)) (Product Nat (Result Unit error))))
(define-host (Sscan s a) variadic (host expr "fmt.Sscan(%s, %s...)" (import "fmt")))
```

```lisp
(sig EncodeRune ((p (buffer (int 0 255))) (r int32)) (int 1 4)
     (where (<= 4 (len p)))
     (host expr "utf8.EncodeRune(%s, rune(%s))" (import "unicode/utf8")))

(sig Sscan ((str bytestring) (a (array (cell go.Value))))
     (tuple (int 0 9223372036854775807) (result.result (tuple) error)) variadic
     (host expr "fmt.Sscan(%s, %s...)" (import "fmt")))
```

```clojure
(defhost EncodeRune :- (range 1 4) [p :- (Buffer Byte), r :- Int32]
  {:requires (<= 4 (count p))
   :host     "utf8.EncodeRune(%s, rune(%s))"
   :import   "unicode/utf8"})

(defhost Sscan :- [Nat (Result Unit error)] [s :- go.bytestring, a :- (Seq (Cell go.Value))]
  {:host "fmt.Sscan(%s, %s...)" :import "fmt" :variadic true})
```

```shen
(declare-host encode-rune
  {(buffer byte) --> int32 --> (range 1 4)}
  where (<= 4 (length P))
  host expr "utf8.EncodeRune(%s, rune(%s))" import "unicode/utf8")

(declare-host sscan
  {go.bytestring --> (seq (cell go.value)) --> (nat * (result unit error))}
  variadic host expr "fmt.Sscan(%s, %s...)" import "fmt")
```

### Example 7 — the logic: a specification function, an invariant, a lemma

```tla
RECURSIVE sum(_, _, _)
sum(a, i, j) == IF i ≥ j THEN 0 ELSE a[i] + sum(a, i + 1, j)

total(a ∈ Seq(0..1000)) ∈ Nat
  ENSURE RESULT = sum(a, 0, Len(a))
  ==
  LOOP i := 0, s := 0  INVARIANT i ≤ Len(a) ∧ s = sum(a, 0, i) :
      i ≥ Len(a) → s
   [] OTHER      → AGAIN(i + 1, s + a[i])

LEMMA sum_snoc == ∀ a ∈ Seq(Int), i ∈ Nat :
  i < Len(a) ⇒ sum(a, 0, i + 1) = sum(a, 0, i) + a[i]
PROOF BY INDUCTION ON i
```

```fsharp
[<Spec>]
let rec sum (a: Seq<Int>) (i: Nat) (j: Nat) : Int =
    if i >= j then 0 else a[i] + sum a (i + 1) j

let total (a: Seq<0..1000>) : r: Nat { r = sum a 0 (len a) } =
    loop i = 0, s = 0
      invariant (i <= len a && s = sum a 0 i)
    with
    | i >= len a -> s
    | _          -> again (i + 1) (s + a[i])

let lemma sumSnoc (a: Seq<Int>) (i: Nat)
    : Lemma (requires i < len a) (ensures sum a 0 (i + 1) = sum a 0 i + a[i]) =
    by induction i
```

```scheme
(define-spec (sum a i j) (if (>= i j) 0 (+ (ref a i) (sum a (+ i 1) j))))

(: total (-> (Seq (range 0 1000)) Nat))
(define (total a)
  (ensure (= result (sum a 0 (len a))))
  (loop ((i 0) (s 0))
    (invariant (and (<= i (len a)) (= s (sum a 0 i))))
    (cond ((>= i (len a)) s)
          (else (again (+ i 1) (+ s (ref a i)))))))

(define-lemma sum-snoc
  (forall ((a (Seq Int)) (i Nat))
    (implies (< i (len a)) (= (sum a 0 (+ i 1)) (+ (sum a 0 i) (ref a i)))))
  (by (induction i)))
```

```lisp
(spec sum (a i j) (if (>= i j) 0 (+ (a i) (sum a (+ i 1) j))))      ; †

(sig total ((a (array (int 0 1000)))) (int 0 +inf)
     (ensures (= result (sum a 0 (len a)))))
(def total (a)
  (loop ((i 0) (s 0))
    (invariant (and (<= i (len a)) (= s (sum a 0 i))))              ; †
    (>= i (len a)) s
    else           (again (+ i 1) (+ s (a i)))))

(lemma sum-snoc                                                      ; †
  (forall ((a (array int)) (i nat))
    (=> (< i (len a)) (= (sum a 0 (+ i 1)) (+ (sum a 0 i) (a i)))))
  (by (induction i)))
```

```clojure
(defspec sum [a i j] (if (>= i j) 0 (+ (a i) (sum a (inc i) j))))

(defn total :- Nat [a :- (Seq (range 0 1000))]
  {:ensures (= % (sum a 0 (count a)))}
  (loop [i 0, s 0]
    {:invariant (and (<= i (count a)) (= s (sum a 0 i)))}
    (if (>= i (count a)) s (recur (inc i) (+ s (a i))))))

(deflemma sum-snoc [a :- (Seq Int), i :- Nat]
  {:requires (< i (count a))
   :ensures  (= (sum a 0 (inc i)) (+ (sum a 0 i) (a i)))
   :by       (induction i)})
```

```shen
(define sum
  {(seq int) --> nat --> nat --> int}
  A I J -> 0 where (>= I J)
  A I J -> (+ (nth I A) (sum A (+ I 1) J)))

(define total
  {(seq (0..1000)) --> nat}
  A -> (loop [I 0 S 0]
         invariant (and (<= I (length A)) (= S (sum A 0 I)))
         (>= I (length A)) -> S
         _                 -> (again (+ I 1) (+ S (nth I A)))))

(lemma sum-snoc
  (all A (all I (=> (< I (length A))
                    (= (sum A 0 (+ I 1)) (+ (sum A 0 I) (nth I A))))))
  by (induction I))
```

### Example 8 — the untyped λ at the static level, and a polymorphic declaration

```tla
two     == λ s : λ z : s(s(z))
plus    == λ m : λ n : λ s : λ z : m(s)(n(s)(z))
compose(f, g) == λ x : f(g(x))
twice(f) == λ x : f(f(x))

AsType(E ⊆ error)(err ∈ error) ∈ Option(E) == HOST expr "errors.AsType[%t1](%s)" IMPORT "errors"
… CASE errors.AsType(strconv.NumError)(e) OF some(ne) → … [] none → …
```

```fsharp
let two     = fun s -> fun z -> s (s z)
let plus    = fun m -> fun n -> fun s -> fun z -> m s (n s z)
let compose f g = fun x -> f (g x)
let twice f = fun x -> f (f x)

[<Host "errors.AsType[%t1](%s)"; Import "errors">]
val AsType<'E when 'E :> error> : err: error -> Option<'E>
… match errors.AsType<strconv.NumError> e with Some ne -> … | None -> …
```

```scheme
(define two     (lambda (s) (lambda (z) (s (s z)))))
(define plus    (lambda (m) (lambda (n) (lambda (s) (lambda (z) ((m s) ((n s) z)))))))
(define (compose f g) (lambda (x) (f (g x))))
(define (twice f) (lambda (x) (f (f x))))

(: AsType (All ((E (subset error))) (-> error (Option E))))
(define-host (AsType E) (err) (host expr "errors.AsType[%t1](%s)" (import "errors")))
… (case ((errors.AsType strconv.NumError) e) ((some ne) …) ((none) …))
```

```lisp
(def two     (fn (s) (fn (z) (s (s z)))))
(def plus    (fn (m) (fn (n) (fn (s) (fn (z) ((m s) ((n s) z)))))))
(def compose (f g) (fn (x) (f (g x))))
(def twice (f) (fn (x) (f (f x))))

(sig (AsType (E error)) ((err error)) (option E)                    ; † typevars-research
     (host expr "errors.AsType[%t1](%s)" (import "errors")))
… (case ((errors.AsType strconv.NumError) e) (some ne) … none …)
```

```clojure
(def two     (fn [s] (fn [z] (s (s z)))))
(def plus    (fn [m] (fn [n] (fn [s] (fn [z] ((m s) ((n s) z)))))))
(defn compose [f g] (fn [x] (f (g x))))
(defn twice [f] #(f (f %)))

(defhost AsType :- (Option E) [err :- error] {:params [E :< error] :host "errors.AsType[%t1](%s)" :import "errors"})
… (match ((errors/AsType strconv/NumError) e) (some ne) … none …)
```

```shen
(define two      S -> (/. Z (S (S Z))))
(define plus     M N -> (/. S (/. Z ((M S) ((N S) Z)))))
(define compose  F G -> (/. X (F (G X))))
(define twice    F -> (/. X (F (F X))))

(declare-host as-type
  {error --> (option E)} for-all E subset-of error
  host expr "errors.AsType[%t1](%s)" import "errors")
… (case ((errors.as-type strconv.num-error) E) [some Ne] -> … none -> …)
```

## 4. Measured: size and nesting

Counted by a script over §3's fenced blocks, eight per notation. Comments and the † marks are
dropped, whitespace is collapsed, and tokens are counted approximately: names, numbers, strings and
each punctuation mark. For the bracketed notations it also counts **depth**, the deepest nesting of
brackets of any kind, and the **closing run**, the longest unbroken run of closing brackets. Those two
are the measurable proxies for the balancing error §5.2 names.

| notation | blocks | lines | characters (whitespace collapsed) | tokens (approx.) | deepest nesting | longest closing run |
|---|---:|---:|---:|---:|---:|---:|
| M mathematical | 8 | 54 | 1882 | 731 | 3 | 3 |
| F F# | 8 | 62 | 2074 | 736 | 3 | 2 |
| P pure S-expr | 8 | 63 | 2427 | 944 | 9 | 7 |
| T today | 8 | 63 | 2233 | 920 | 9 | 7 |
| C Clojure | 8 | 56 | 2124 | 846 | 8 | 7 |
| S Shen | 8 | 75 | 2229 | 850 | 9 | 6 |

What it says:
- **M and F are about a fifth shorter in tokens** than the S-expressions (731 and 736 against
  920–944). Most of the difference is in claims and arithmetic, where infix needs no head and no
  closing bracket.
- **Clojure's sprinkles buy about 8%** over today's notation (846 against 920), mostly from `[…]`
  binders and `{…}` clauses replacing nested lists.
- **The S-expressions nest 8 or 9 deep, and close up to 7 brackets in a row.** The deepest are
  example 5 (local state with a host call and nested conditionals) and example 8 (Church numerals).
  M and F never exceed 3. That is the size of the risk §5.2 describes, and it is concentrated in a few
  forms: a formatter that breaks a long closing run across lines is the mitigation.
- **Shen is the longest in lines** (75): pattern rules and sequent `datatype`s spread over more
  lines than any other notation.

These sizes are of eight short programs. They describe the notations, and say nothing yet about
error rates, which only §7's experiment measures.

## 5. The LLM side

hamza speaks for people; this section is the model's, argued from how a model reads and writes
code. **It is reasoning, not measurement**, and §7 proposes the measurement.

### 5.1 What transfers to an obscure language

A model writing a language it has rarely seen leans on languages it has seen often, and what
transfers is **shape**, the grammar's skeleton, far more than vocabulary:
- **S-expressions have one shape**, `(head args…)`, and the Lisp family (Scheme, Racket, Common
  Lisp, Clojure, Emacs Lisp) is a large share of training data. Learning a new S-expression language
  is learning its heads: the shape is already known. This is hamza's observation, and it has a
  mechanism.
- **An ML shape** (F#, OCaml, Haskell, Elm, F\*, Scala's `match`) is also well represented, and F#'s
  is its cleanest member.
- **TLA+ is rare** in training data. Its notation is familiar as mathematics, from papers and LaTeX,
  but not as code a model writes. A model will mix its ASCII and Unicode spellings, misjudge `/\` and
  `\/` bullet alignment, and invent PlusCal where TLA+ is meant.
- **Shen is rarer still.** What a model knows of it is mostly its Lisp shape, which is why its flavor
  here is usable at all.

### 5.2 The failure modes, notation by notation

| notation | the errors a model makes most | caught by |
|---|---|---|
| S-expressions | **unbalanced closing parentheses** at the end of a deep form, `…))))))`; a clause in the wrong parenthesis level after an edit | the reader, at once, and a formatter that reindents to show the structure. The error is syntactic and local |
| F# | **a nested `match` swallowing the following cases** (the offside rule cannot tell where the inner one ends): F#'s own classic pitfall, for people as for models; indentation drifting in long edits | sometimes only by the type checker, sometimes not at all: an inner `match` taking an outer case can be well typed and wrong |
| mathematical | **precedence** (`a ∧ b ∨ c`), dangling `;` after `IF … THEN`, Unicode and ASCII mixed, a chained comparison misread | the parser, if the grammar refuses the ambiguous cases; otherwise a wrong program |
| Clojure | `[]` and `()` confused; `recur` outside a tail position | the reader; the tail rule |
| Shen | uppercase variables against lowercase symbols; rule order (`where` guards) | the reader, partly |

**The S-expression failure is the safest kind.** It is syntactic, it is caught by the reader before
anything is interpreted, and its fix is mechanical. F#'s and the mathematical notation's worst
failures can produce a program that parses, possibly type-checks, and means something else. For a
language whose emitted code must be trusted, an error that cannot become a wrong program is worth
more than an error that is rarer.

### 5.3 Reading and reasoning

- **Structure.** S-expressions put the abstract syntax tree on the page: scope, binding and what is
  applied to what are explicit, and reasoning about a transformation (inlining, a translation, a
  proof step) is reasoning about the visible tree. That is why this repository's own reducer, IR text
  and differential tooling are S-expressions, and why a model reasons about them well.
- **Formulas.** For claims (sets, quantifiers, arithmetic relations), mathematical notation is
  clearer to read for a model as for a person: `0 ≤ i < Len(a) ∧ s = sum(a, 0, i)` against
  `(and (<= 0 i) (< i (len a)) (= s (sum a 0 i)))`. It aligns with the derivations in the headers and
  the specifications, which are written in mathematics, so code and its argument can be compared
  line by line.
- **Edits.** A model edits by replacing text. In S-expressions a replacement must keep the
  parentheses balanced across its boundary, so a replacement whose span ends inside a closing run is
  the fragile case. A formatter that puts each clause on its own line, as today's corpus does,
  shortens the runs and makes spans clean. In F# a replacement must keep the indentation, which is
  usually easier, except at a nested `match`.

### 5.4 The model's ranking, for writing this language

1. **S-expressions with a few sprinkles (C, Clojure-like)**: one shape, `[…]` marking binders so a
   binding form is visible at a glance, `#{…}` writing a set as a set. The most reliable to write,
   and the errors are caught.
2. **Pure or today's S-expressions (P, T)**: equally reliable; slightly harder to scan, because every
   bracket is the same.
3. **F# (F)**: the most pleasant to read, close behind in writing, with one structural hazard that can
   hide (nested `match`).
4. **Shen (S)**: Lisp-shaped and so writable, but uppercase variables and rule-based definitions are
   two more conventions to keep.
5. **Mathematical (M)**: the best to *read* claims in, and the riskiest to *write*.

## 6. A synthesis: one source, two renderings

TLA+ is written in ASCII and read pretty-printed. The source has `\A x \in S : P`, and the printed
page has ∀x ∈ S : P. Lamport separated what a person types from what a person reads. The same
separation answers this research's tension:
- **the source is S-expressions**: regular, one shape, the LLM-reliable kind, with comments as gaps
  (ADR 0024 unchanged) and a reader already built;
- **the rendering is mathematics**: documents, error messages, and an editor's hover or side view
  print claims and code in the notation of §3's first column, from the same forms;
- **the five unifications and settypes' logic are notation-free**, so they are built in the
  S-expression source either way.

A rendering is a function of the source, so it cannot disagree with it, and the source never depends
on the rendering. That is the property that made the IR's printer trustworthy (ADR 0038: the text is
read, not written by others). The cost is a printer from forms to mathematics, which the
notation-research translator would have needed anyway, and no conversion of the corpus beyond what
the S-expression flavor changes.

Which S-expression flavor would be decided on its own merits. C's brackets (`[…]` for binders,
`#{…}` for sets) add two token kinds and remove a class of misreading. P and T add none.

## 7. The experiment that would measure the LLM half

The ranking in §5.4 is an argument. The measurement would be:
- **write**: from a natural-language specification of each of 20 small programs (the gauntlet's,
  `count`, `ages`, a variant, a union, a lemma), have a model write the program in each candidate
  notation, given only that notation's grammar summary and three examples. Count the programs that
  parse, that pass the checker, and that compute the right answer;
- **repair**: plant one fault in each program (an off-by-one, a missing case, a wrong claim) and count
  the faults a model finds and fixes;
- **edit**: ask for one change (add a clause, a parameter, a claim) and count the edits that leave the
  program well formed.

Each run uses a fresh context, several models, and three seeds. The notation with the most right
answers, not the fewest syntax errors, wins the LLM half, because a syntax error is caught and a wrong
answer is not. It is a few hours of model time, and it needs each candidate's reader, or at least a
parser, which the S-expression flavors almost have already.

## 8. Decisions for hamza

1. **Whether the source and the reading notation are one or two** (§6). If two: S-expressions to
   write, mathematics to read.
2. **Which source notation**: M, F, or one of the S-expression flavors P, T, C, S.
3. **Whether to run §7's experiment** before deciding 2.
4. **Whether to relax ADR 0014 at the static level** (§1.3), letting a fixed-point combinator reduce
   when its recursion is on static data. *Not recommended* until a program asks: the static level's
   normal form is what makes compile time terminate.

What the research recommends from the LLM side: **two notations (§6), C or T as the source, and §7's
experiment to confirm before the conversion.** The human half is hamza's.
