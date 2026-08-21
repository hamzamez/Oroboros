# Closures: what putting them back in would mean

hamza's question: what if we allow full higher-order programming? What happens to the language, the
type system, the data structure, the compiler? What does the literature say? And — inventing anew —
what would the data structure and memory model be?

**The first finding is that the question is mis-stated, and the mis-statement is ours.**

---

## 1. We already have closures. They work. Here is one.

```lisp
(def nil    (fn (c n) n))
(def cons   (fn (x xs) (fn (c n) (c x (xs c n)))))
(def foldr  (fn (l f z) (l f z)))

(def total (fn (k)
  (foldr (cons 1 (cons 2 (cons 3 (cons k nil)))) go.+ 0)))
```

`cons` returns a lambda that **captures `x` and `xs`**. That is a closure by any definition. It
compiles, today, unmodified:

```go
func CTotal(k int) int {
	return (1 + (2 + (3 + (k + 0))))
}
```

That is a **Church-encoded list with `foldr`** — the free monoid, its universal property, the thing
[data-structures.md §1.2](data-structures.md) said we could not have because
[ADR 0014](decisions/0014-recursion-is-not-in-the-language.md) removed recursion. It needs no
recursion: `cons` nests closures at *construction*, and `foldr` is application.

**So §1.2 is wrong as stated.** Inductive data does not need recursion; it needs closures, which we
have. What it needs recursion for is a *dynamic* length:

```lisp
(loop ((l nil) (i 0))
  (>= i k)  l
  else      (again (cons i l) (+ i 1)))
```
```
gen: application of a non-name: ((loop (fn (l i) …) (fn (c n) n) 0) go.+ 0)
```

**That is the boundary, exactly.** A list whose length is known at compile time reduces to nothing.
A list whose length is a runtime value leaves a closure in the residual and is refused.

---

## 2. The reframing: this is a two-level language and does not say so

The language has two levels and has never named them:

| | | closures | recursion | data structures |
|---|---|---|---|---|
| **static** — the reducer | β, δ, whole-program | **yes, unrestricted** | via fuel | lists, trees, whatever |
| **dynamic** — the residual | what a backend emits | **no** | no | tables |

"Closures are refused" is the wrong sentence. The true one is **"a closure may not survive
staging."** Everything the language actually forbids follows from that, and so does everything it
permits — including the Church list above.

This is a known and well-populated design point:

- **Nielson & Nielson**, *Two-Level Functional Languages* (1992) — the formal treatment.
- **Davies & Pfenning**, *A modal analysis of staged computation* (JACM 2001) — `□A` as "code",
  which is the type of a dynamic-level value.
- **MetaML** (Taha & Sheard 1997) and **MetaOCaml** — brackets and escapes making the level
  explicit.
- **Lightweight Modular Staging** (Rompf & Odersky 2010) — `Rep[T]` is the dynamic level.
- **Terra** (DeVito et al., PLDI 2013) — Lua at compile time, a C-like language at runtime. Very
  close to our shape.
- **Zig's `comptime`** — the mainstream one, and the closest living relative: a full language at
  compile time, a restricted one at runtime, with a branch quota where we have fuel.
- **Jones, Gomard & Sestoft**, *Partial Evaluation and Automatic Program Generation* (1993) —
  **binding-time analysis** is precisely "which level is this expression at?"

We arrived here from cost measurements and never wrote down that it is where we are.

---

## 3. So the real question

Not *"should we have closures"* — we do. It is:

> **Should a closure be allowed to survive staging into the residual?**

Everything below answers that.

---

## 4. What it would cost

### 4.1 Termination dies, and this one is close to fatal

**Closures plus mutable storage give you recursion, whatever the term-level check says.** It is
Landin's knot, and with [ADR 0018](decisions/0018-immutable-values-linear-buffers.md)'s buffer it
is writable:

```lisp
(build 1 (fn (b)
  (set b 0 (fn (x) (if (== x 0) 1 (* x ((b 0) (- x 1))))))))
```

The closure captures `b` and calls `(b 0)`, which is itself. That is factorial, by backpatching, and
no check on `def` sees it.

What dies with it: **size-change termination** ([sct-2026-08-19](../gauntlet/results/sct-2026-08-19.md),
96% of loops proven), the interval analysis's fixpoint, and ADR 0015's claim that *termination is a
computed program property*. Recoverable only by forbidding a closure to capture a buffer — which is
a substructural condition on capture, i.e. Rust's `Fn`/`FnMut`/`FnOnce` distinction, which exists
in Rust for exactly this reason.

### 4.2 `(a i)` stops being unambiguous

[tables.md §3.2](spec/tables.md) rests on a checked invariant: **in a residual, an application whose
operator is a variable can only be a table**, because a surviving lambda is refused. Let closures
survive and the operator of an application may be a function, so indexing and calling become
indistinguishable without types — and `targets/js/` declares none.

Cost: `(at a i)` comes back, or the emitter needs a type at every application.

### 4.3 Every analysis needs a call graph it no longer has

The refinement checker, the interval analysis and the size-change checker all assume they can see
every call. A closure stored in a table and applied later is an **indirect call**: the callee is
unknown, so the analysis must either give up or run **k-CFA** (Shivers 1991), which is expensive and
much weaker.

The concrete numbers at risk: *100% of integer operations provably in the portable window* and *96%
of loops proven terminating*, both from
[sct-2026-08-19](../gauntlet/results/sct-2026-08-19.md).

### 4.4 x86 needs a runtime

Go, V8 and the JVM have closures and would emit theirs. **`targets/windows/` has none.** A closure
there is a heap-allocated environment record and an indirect call through a code pointer — code we
write and ship. That is a runtime, against requirement 6 (small binaries) and against the property
that the windows target reaches parity with hand-written assembly with no support code at all.

### 4.5 Hidden allocation, which is what killed the predecessor

An environment record is a heap allocation the source does not mention.
[CLAUDE.md](../CLAUDE.md) names this as the failure mode of the predecessor project. Escape analysis
recovers *some* of it — measured working on Go and Java
([product-2026-08-19](../gauntlet/results/product-2026-08-19.md)) — but "some, silently" is the
SISAL cliff again.

### 4.6 ADR 0018 needs rank-2 types

Its escape argument is that closures are refused. Restore them and `build` needs Haskell's
`runST :: (forall s. ST s a) -> a` — a phantom scope parameter and rank-2 quantification, which is
the one thing that type exists for.

---

## 5. What it would buy

**Inductive data at runtime.** §1's Church list with a *dynamic* length. Combined with
whole-program **defunctionalization** (Reynolds 1972), a Church-encoded list becomes a tagged sum —
which is a real list. **MLton does exactly this**: monomorphise and defunctionalize the whole
program. It is available to us specifically because we already reduce whole-program.

**Tier 3 callbacks.** [callbacks.md](spec/callbacks.md) shows Tiers 1 and 2 are reachable without
closures; the gap is a *manufactured* callback handed to a host API, and dispatch tables. Closures
close it.

**Laziness, and therefore Okasaki.** A thunk is a closure. Okasaki's amortised persistent
structures depend on lazy evaluation, so with closures the whole persistent-data-structure design
space opens — finger trees, RRB vectors, persistent maps.

**Objects and modules.** A closure over state is an object; the Scheme tradition has said so since
`lambda: the ultimate` (Steele, 1978).

**Familiarity.** Every host language in our target set has closures, and so does every language an
LLM has read a lot of.

---

## 6. The literature on doing it well

**Closure conversion**: Appel, *Compiling with Continuations* (1992) — flat versus linked closures,
and the space-safety problem Shao & Appel later fixed.

**Typed closure conversion**: Minamide, Morrisett & Harper (POPL 1996) — a closure is an
**existential type**, `∃env. ((env × A) → B) × env`. That is the type-theoretic answer and it is
clean: the environment's type is hidden, which is exactly what makes two closures of the same
arrow type interchangeable.

**Defunctionalization**: Reynolds (1972); Danvy & Nielsen's modern treatment. Needs the whole
program, which we have.

**Control-flow analysis**: Shivers' k-CFA (1991) for the cases defunctionalization cannot close.

**Escape analysis**: Blanchet (1999), Choi et al. (1999) — the reason closures are affordable in
Java and Go today.

**Closures against linearity**: Rust's three closure traits (`Fn`, `FnMut`, `FnOnce`) exist
*because* capture mode interacts with ownership, and Linear Haskell needs multiplicity-polymorphic
arrows for the same reason. **There is no cheap version of this interaction**, and it is §4.1's cost
stated as type theory.

---

## 7. With closures in, what is the best data structure?

Honestly: **algebraic data types plus arrays** — which is to say, ML's.

The reasoning is short. Closures restore inductive data (§1); defunctionalization turns the Church
encoding back into a tagged sum, which is a real algebraic data type; laziness restores the
persistent structures. At that point the best design is the one forty years of ML and Haskell
converged on: **sums of products, with arrays for the flat dense case**, and a garbage collector
underneath.

And that is the problem, not a recommendation:

> With closures surviving to runtime, the design converges on **an ML with four backends**. The
> things that make this project distinctive — parity with hand-written code, no runtime, portability
> *computed* rather than promised — all get harder at once, and the competition becomes MLton and
> OCaml, which are very good.

The current design is distinctive precisely because it refuses the thing that would make it
ordinary.

---

## 8. Inventing it anew

If I started over with the same requirements — four targets including bare metal, parity with
hand-written code, a small language, portability computed — I would build **the same thing, and I
would say the two-level structure out loud from the beginning.**

```
STATIC LEVEL   full higher-order. closures, Church-encoded data, recursion,
               compile-time lists and maps. Bounded by fuel; divergence is a
               COMPILE error, which is honest.

DYNAMIC LEVEL  first-order. tables, loops, scalars. No closures, no recursion,
               no hidden allocation. This is what a backend emits.

THE BOUNDARY   binding-time analysis. Everything static must vanish.
```

Data structure: **tables at the dynamic level** — a function with a known finite domain, indexed by
application ([tables.md](spec/tables.md)). Anything you like at the static level, because it costs
nothing.

Memory: **immutable values plus one scoped linear buffer**
([ADR 0018](decisions/0018-immutable-values-linear-buffers.md)) — and the reason it works is that
the dynamic level is first-order, so a buffer cannot be captured.

Three things I would do differently, and all three are cheap:

**Say the levels in the diagnostics.** Not *"a bare abstraction reached the emitter: this is an
escaping closure"* but:

```
in total: this function would have to exist at run time.
  The runtime level is first-order — closures, and data built from them, must be
  gone by the end of reduction. This one survives because `k` is a runtime value,
  so the list's length is not known until the program runs.
```

That is a better message and it is also *true*, which the current one is not (§1 of
[callbacks.md](spec/callbacks.md) — a closed lambda is not a closure).

**Allow recursion at the static level.** ADR 0014 refuses a recursive definition *before* reduction,
because `oro` accepted programs `build` refused. Under the two-level view the right rule is: a
recursive definition that **fully reduces away** is a compile-time loop and is fine; one that
survives is refused. That is strictly more permissive, equally safe, and it makes compile-time list
and tree processing writable. It is exactly Zig's `comptime` with its branch quota, and our fuel is
the quota.

**Give the static level a real library.** Lists, maps, trees, sorting — all of it free, all of it
gone by emission. The language already supports it (§1) and nothing says so.

---

## 9. Verdict

**Do not let closures survive staging. Do adopt the two-level framing.**

The costs in §4 are not evenly matched. §4.4 (a runtime on bare metal) and §4.5 (hidden allocation)
attack the project's central claim. §4.1 (termination) destroys a result that took three rounds to
get. §4.3 weakens every analysis at once. Against that, §5's real gain is a *manufactured* callback
and dispatch tables — which [callbacks.md](spec/callbacks.md) shows is the small end of the API
question, because almost every host API is called from the place that knows what to do.

**What would change this:** a program that must manufacture a callback and hand it to a host API —
a GUI window procedure that closes over application state, a dispatch table keyed at runtime, a
comparator that captures. If one appears and cannot be written with the `(function, context)`
convention that C, Win32 and POSIX all use, then Tier 3 is a real requirement and this decision
should be re-argued with that program as the evidence.

**What to do now, none of which is a language change:** rename the levels in the documentation and
the diagnostics; relax ADR 0014 to refuse recursion that *survives* rather than recursion that is
*written*; and correct
[data-structures.md §1.2](data-structures.md), which says inductive data is impossible when what is
actually impossible is inductive data of dynamic length.
