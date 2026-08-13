# Exploration: does liveness-based reuse survive function boundaries?

Exploration only. No commitments, no ADR.

[g7](g7-aliasing.md) priced a uniqueness false negative at 40×–1,540×, unbounded in structure
size, and named the remaining gap: **everything derived so far is intraprocedural.** A dict
threaded through several calls, or stored in a struct, is where Perceus-style reuse gets hard —
and it is the one place where getting it wrong is unbounded rather than a constant factor.

**Result: the boundary problem is far smaller than it looks, because most boundaries do not
survive rewriting. What remains is solved by putting multiplicity in the signature. And the
runtime fallback is nearly free — but only if you still do the static analysis, which is the
negative result worth having.**

---

## 1. Most boundaries do not exist

[g3](g3-generics.md) established that a **non-recursive function definition is a rewrite rule**.
So it is inlined, and a dict threaded through non-recursive calls ends up in one function body,
where liveness is intraprocedural again and [s2](s2-multiplicity-inference.md) already applies.

The boundaries that actually survive rewriting:

| Survives | Why |
|---|---|
| **Recursive functions** | Cannot be rules — the right-hand side contains its own head ([g3 §9](g3-generics.md)) |
| **Outlined functions** | If we outline to save binary size, we create a boundary deliberately |
| **Externs** | Host code; no body to analyze |
| **Indirect calls** | Escaping closures ([g6](g6-escaping-closures.md)) — callee not statically known |

Four cases, not "every call." That is the first and largest reduction in the problem.

## 2. The multiplicity grade *is* the ownership annotation

For the boundaries that remain, the question is: may the callee reuse the structure it was
given? That is exactly what Rust encodes as *move versus borrow*, and it is what Perceus
computes as *owned versus borrowed*.

We already have the vocabulary. A parameter at **grade 1** is used once, which for a heap
structure means the callee is its last user — so the callee may reuse. **Grade ω** means shared,
so it must not.

> **A parameter's multiplicity in the signature is its ownership declaration.**

The analysis stays modular, which matters for
[ADR 0006](../decisions/0006-ir-file-format.md)'s separate compilation:

- The **callee's body** determines the grade it *needs*.
- The **caller** determines the grade it can *provide*.
- If a callee wants grade 1 and the caller holds grade ω, **the caller copies once at the call
  site** — not per iteration inside.

No whole-program analysis. Checked at each call, exactly like an ordinary type.

Worked against the case that motivated this:

```lisp
(fn build ((n nat) (d (dict string i32))) -> (dict string i32)
  (if (= n 0) d
      (build (- n 1) (dict-insert d (key n) n))))
```

`build` uses `d` once, so its parameter is grade 1. A caller passing a fresh `(dict-empty)` has
grade 1 available and reuse happens all the way down. A caller that also uses the dict later
holds grade ω, so it copies once at the call and reuse happens inside. Both correct, and the
expensive case costs one copy rather than *n*.

**It also settles an interaction left open by [g3](g3-generics.md).** Outlining a function to
save binary size creates a boundary — but with grades in the signature the boundary is not
conservative, so outlining does not break reuse. The specialization-versus-size tension is not
aggravated by uniqueness.

## 3. Externs need a third column

An extern has no body, so whether it *retains* its argument cannot be inferred — the same
argument [s2 §5](s2-multiplicity-inference.md) made for purity.

The good news is that the existing mechanism absorbs it: **passing a value to a retaining extern
makes it live from that point on**, so ordinary liveness then forbids in-place mutation
afterwards. Nothing new is needed beyond the declaration itself.

Binding files therefore carry three columns — name/type mapping, purity, retention — and
retention defaults to *retains*, which is the safe direction. As with purity, a wrong
declaration is an unsoundness sourced from data that no check on our side catches.

## 4. What the fallback costs

Where the static analysis cannot decide, the alternative is to decide at runtime with a
reference count — what Koka and Lean do. Measured across a real (`//go:noinline`) boundary:

| Strategy | ns/op | vs static | allocs |
|---|---|---|---|
| Static — grade crosses in the signature | 8.51 | — | 0 |
| RC check, non-atomic | 8.81 | **+3.5%** | 0 |
| RC check, atomic load | 8.66 | **+1.8%** | 0 |
| Copy (8 entries) | 343 | 40× | 5 |
| Copy (~500 entries) | 11,960 | **1,400×** | 5 |

Threaded through three boundaries:

| | ns/op | vs static |
|---|---|---|
| Static | 29.6 | — |
| RC with **borrowed** parameters | 30.6 | **+3.4%** |
| RC with **naive retain/release** | 411 | **13.9×** |
| RC with naive atomic retain/release | 431 | 14.6× |

**The reference-count check is nearly free — 2–4%.** That was a surprise; an atomic *load* on
x86 is an ordinary `mov`, so even the concurrency-safe version costs almost nothing. (An atomic
*increment* would not be, which is exactly what the next row shows.)

## 5. The negative result

The naive rows are the important ones, and they came out of a benchmark bug that turned out to
be the finding.

Retaining before each call makes the count 2 for the duration, so the uniqueness check inside
**always fails and copies** — 13.9×, with allocations on every call. That is not an artifact of
the measurement; it is the classic naive-RC failure mode, and avoiding it is precisely what
Perceus contributes over ordinary reference counting.

> **Reference counting is not an alternative to the static analysis. It is a fallback that only
> works if you have already done the analysis** — because deciding which parameters are borrowed
> is itself the static question. Skip it and RC costs 14×; do it and RC costs 3%.

So the design cannot choose "just use RC and avoid the type-system work." Those are not two
options. The realistic choice is: static grades everywhere they decide, RC where they do not,
and borrowing inferred in both cases.

## 6. One more thing the hosts do not do

Go, JS, and Java are all garbage collected and none of them ever reuses a structure for a
functional update — a functional dict update in Go means allocating a new map, and nobody writes
that. **Hand-written Go mutates.**

So on this program the parity standard is the in-place version, and there is no credit for being
functional. We must reuse to *match* hand-written code, and lose 40×–1,400× if we do not. This
is one of the few places where the language's semantics and the host's idiom disagree, and the
host's idiom is the bar.

## 7. Findings

1. **Most boundaries do not survive rewriting.** Non-recursive functions are rules, so they
   inline. Only recursive, outlined, extern, and indirect calls remain.
2. **Parameter multiplicity in the signature is the ownership annotation** — grade 1 is owned,
   grade ω is shared. Modular, no whole-program analysis, compatible with separate compilation.
3. **The caller copies, once, at the call site** when it cannot supply grade 1 — not per
   iteration inside the callee.
4. **Outlining does not break reuse**, so g3's size-versus-specialization tension is not made
   worse by uniqueness.
5. **Externs need a retention declaration** — a third binding-file column, defaulting to
   *retains*. Existing liveness absorbs it with no new mechanism.
6. **An RC check costs 2–4%**, atomic or not, and is an acceptable fallback.
7. **Naive RC costs 14×** and destroys reuse entirely. **RC does not remove the need for the
   static analysis** — it depends on it.
8. **The hosts never reuse**, so hand-written Go mutates and that is the bar. There is no credit
   for functional semantics here.

## 8. Verdict

The highest risk in the design comes down substantially. The boundary problem shrinks to four
cases; grades in signatures handle them modularly; and where they cannot decide, the runtime
fallback is 3% rather than 1,400%.

What it costs: **parameter multiplicity becomes part of a function's signature**, inferred from
the body. That means changing a function body can change its signature and break callers — the
same API-stability problem Rust has at inference boundaries, and a real cost that should be
recorded rather than waved at. Making it *visible* in the cost report
([g6 §9](g6-escaping-closures.md)) is the mitigation, and it is the same report that already has
to carry `could not prove unique — copying, O(n)`.

**Still untested:** a structure stored *inside* another structure. Everything here threads a
value through calls; reuse for a dict held in a struct field is reachability rather than
liveness, and that is genuinely harder. It is also the case Perceus handles with RC rather than
statically, which — given finding 6 — may simply be the right answer.
