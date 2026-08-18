# JavaScript's top level

An experiment. **The measurement is
[gauntlet/results/js-toplevel-2026-08-18.md](../../gauntlet/results/js-toplevel-2026-08-18.md)** —
read that first. Companion to [../go-toplevel/](../go-toplevel/), and the point of running both is
that the walls are *different*.

## The question

Everything Node and the browser both provide with **no import and no require**. Written as
deliberately non-portable programs, per
[ADR 0001](../../docs/decisions/0001-parasite-model.md).

Eight modules, declared in `targets/js.oro`, mirroring how JavaScript organises itself rather than
how Go does:

```lisp
(use js/global as g)    ; parseInt, Number, String, NaN, true/false
(use js/math as m)      ; Math.*
(use js/int as ji)      ; integer / and %, which are not portable
(use js/array as a)     ; Array methods and constructors
(use js/string as s)    ; String methods
(use js/object as o)    ; a null-prototype object as a dictionary
(use js/json as json)   ; JSON.parse / stringify
(use js/console as c)   ; console.log / error
```

## Programs

| | what it exercises |
|---|---|
| [wordstats.oro](wordstats.oro) | strings, a dictionary, JSON, a max-scan fold |
| [sieve.oro](sieve.oro) | the Go experiment's sieve, ported, to test whether the loop penalty is host-specific |

`*.txt` files are benchmark harnesses, kept as text so they are not modules.

```bash
go run ./cmd/gen experiments/js-toplevel/wordstats.oro js /tmp/ws.mjs
cd /tmp && node -e "import('./ws.mjs').then(m => m.genMain())"
```

## What worked, and it is more than Go

**Method syntax was free.** A template's first hole is the receiver, so `"%s.map(%s)"` needed no new
mechanism. Go's top level is predeclared functions; JS's is methods and namespace statics; the same
`(prim …)` grammar expresses both.

**No types at all.** Go needed `slice-int`, `slice-bool` and a `make` per element type, because the
type language has no constructors. JS needed none — `targets/js.oro` still declares zero `(type …)`
forms. The wall that dominated the Go experiment is simply absent here.

**Conditionals are idiomatic.** `if` is an expression in our core; Go has no conditional expression,
so every branch inside a loop became `var t2 T; if c { … } else { … }`. JavaScript's `?:` maps
one-to-one, and the emitted code reads like hand-written JS.

**`Math.random` is impure and the target says so** — an `expr` that is not `pure`, which is exactly
the case that kills the tempting "expr means pure" heuristic
([effects.md §3](../../docs/spec/effects.md)).

## The walls

### 1. The loop's iteration space — **445×**

The same wall as Go, replicated: no start, no step, no early exit. The sieve degrades from
O(n log log n) to O(n²) here too. Go measured 1117×; JS measures 445×, the difference being that
idiomatic JS is slower to begin with.

Two hosts is the point. **This is the language, not a host.**

### 2. The higher-order API is unreachable — and that is fine

`map`, `filter`, `reduce`, `some`, `findIndex`, `sort`, `forEach` all take a function, and a bare
abstraction is refused as an escaping closure. So the idiomatic half of JavaScript's array API
cannot be called at all.

**Measured, it is worth 3.6× to 133× *against* us to call it.** Fusing into one loop beats
JavaScript's own idioms, worst on typed arrays. The one thing we cannot reach is the one thing not
worth reaching — which is a better answer than "we will get to it", and it is a measurement rather
than a rationalisation.

The honest caveat: this measures *array* callbacks. `Promise.then`, `addEventListener`,
`setTimeout` and every event-driven API also take functions, and for those the argument above says
nothing at all. A browser program is mostly callbacks. That is a real limit and it is untested.

### 3. No multi-value return, again

`js/object.get` returns `(d[k] ?? 0)` — Go's zero-value behaviour, chosen because there is no way to
return "found" alongside the value. Identical to the Go experiment's `v, ok := m[k]` wall, from a
different direction. Third independent demand for a product type.

### 4. Not attempted

`Promise` and `async`/`await`, classes, `Symbol`, `Proxy`/`Reflect`, generators and iterators,
`RegExp` beyond one fixed pattern, `Date`, destructuring, spread, template literals, getters, the
DOM. Several are refused by design; the rest need their own experiment, and the callback-heavy ones
would need the closure question answered first.

## What it returned

Go's experiment found two emitter bugs. This one found **a bug in the target file's own
documentation**, which is a smaller thing but a nicer one: `targets/js.oro` said its argument types
"are never consulted", and the type checker consults them on every target. The first draft of
`wordstats.oro` started a longest-word fold at `0` where it needed `""`, and **the checker caught it
on the host with no type layer** — which is the case
[types.md §1](../../docs/spec/types.md) was written for.
