# Callbacks — what refusing closures actually costs

hamza's question: the Windows API, Go and JavaScript all use higher-order functions. Does refusing
closures cost us access to them?

**Short answer: much less than it looks, and the current refusal is broader than the design needs.**
Three tiers, and only the third is a genuine closure.

---

## 1. The refusal is broader than "no closures"

A **closure** is a function value carrying an environment — free variables from an enclosing scope,
outliving it. A **function pointer** is a function value with *no* free variables. Every one of our
four hosts can express a function pointer; three can express a closure.

Today we refuse both, with one message:

```lisp
(def twice  (fn (x) (go.* x 2)))
(def get-fn (fn (a) twice))
```
```
gen: a bare abstraction reached the emitter: (fn (x) (go.* x 2))
  This is an escaping closure.
```

**`twice` is not a closure.** It has no free variables, and it is exactly `func twice(x int) int`
in Go, `twice` in JavaScript, a static method on Java and a label on x86. The diagnostic is wrong
and the refusal costs more than [ADR 0018](../decisions/0018-immutable-values-linear-buffers.md)
needs, which is only that a *buffer* cannot be captured.

---

## 2. Three tiers

### Tier 1 — the lambda is written at the call site, and the HOST closes over it

This is most callback APIs, and it needs **no function values in our language at all**.

```lisp
(go.go     (fn () body))                    →  go func() { body }()
(go.defer  (fn () body))                    →  defer func() { body }()
(sort.Slice a (fn (i j) less))              →  sort.Slice(a, func(i, j int) bool { return less })
(js.addEventListener el "click" (fn (e) b)) →  el.addEventListener("click", (e) => { b })
(js.setTimeout (fn () b) 100)               →  setTimeout(() => { b }, 100)
```

The lambda never becomes a value in *our* residual. It sits in a **structural position** — the same
kind of position a `loop`'s body or a `let`'s continuation sits in, which the backends already read
positionally and never treat as a value (§14.1 of [tables.md](tables.md)). The backend emits the
body inside the **host's own** lambda syntax, and **the host does the capture**.

That last point is the one that matters: Go's closure, V8's closure and the JVM's lambda are all
free to us and are exactly what hand-written code would use, so the result is at parity by
construction rather than by analysis.

**What it needs:** a *callback hole* in the template language — a `%s` that receives a lambda's
parameter names and its emitted body rather than an expression. That is a mechanism the target file
format does not have and is not a language change.

**On x86 there are no lambdas**, so a callback hole must lower differently: emit a separate
procedure and pass its address, with captures made explicit (§2.2). That is a backend difference,
not a capability gap.

### Tier 2 — a function pointer, no capture

```lisp
(win.EnumWindows my-callback context)
(js.addEventListener el "click" on-click)
(qsort a n size compare)
```

A top-level definition with no free variables, referenced by name. All four hosts have this and
none of them needs a heap environment.

**And the Win32 API is designed for exactly this.** C has no closures, so every Windows callback
takes a bare function pointer plus an explicit context word:

```c
BOOL EnumWindows(WNDENUMPROC lpEnumFunc, LPARAM lParam);
HANDLE CreateThread(…, LPTHREAD_START_ROUTINE lpStartAddress, LPVOID lpParameter, …);
void qsort_s(…, int (*compare)(void*, const void*, const void*), void *context);
```

The `LPARAM` / `lpParameter` / `context` **is the environment, passed explicitly**. So the OS API
that looks most hostile to a closure-free language is the one best suited to it — the C convention
*is* closure conversion done by hand, and it is what we would have had to invent.

The same convention appears wherever C does: POSIX `pthread_create`, `bsearch`, signal handlers,
`SetConsoleCtrlHandler`, window procedures.

**What it needs:** the ability to reference a top-level function by name in an argument position —
that the emitter emits `twice` as a real function and passes `twice` rather than inlining it. This
is *not* a closure and should not be refused. It is the cheapest thing in this document to fix and
the one with the clearest payoff.

### Tier 3 — a genuine closure that escapes

```lisp
(def make-adder (fn (n) (fn (x) (+ x n))))      ; n is captured and outlives make-adder
```

**Refused, and it stays refused.** This is what needs a heap environment, and it is what
[g6](../derivations/g6-escaping-closures.md) priced and
[ADR 0018](../decisions/0018-immutable-values-linear-buffers.md) depends on not existing.

The mitigation, where the host would have used one, is the Tier 2 convention: pass a function and a
context explicitly. That is not a workaround — it is what C, Win32 and every OS API already do.

---

## 3. What is actually lost

Going through the APIs that matter:

| | reachable? | how |
|---|---|---|
| **Windows** callbacks — `EnumWindows`, `CreateThread`, `SetConsoleCtrlHandler`, `WndProc` | **yes** | Tier 2; the API already passes context explicitly |
| **Go** `go func(){}`, `defer func(){}` | **yes** | Tier 1 — these are *syntax*, not values |
| **Go** `sort.Slice`, `http.HandleFunc`, `sync.Once.Do`, `time.AfterFunc` | **yes** | Tier 1 |
| **JS** `addEventListener`, `setTimeout`, `fs.readFile`, `.then` | **yes** | Tier 1 |
| **Java** `Runnable`, `Comparator`, `CompletableFuture` | **yes** | Tier 1 |
| **JS** array methods — `map`, `filter`, `reduce`, `sort` | Tier 1, and **we do not want them** | measured **3.6×–133× slower than a loop** (js-toplevel-2026-08-18) |
| **Go** `strings.FieldsFunc`, `TrimFunc`, `IndexFunc` | Tier 1 | |
| storing a handler **in a table** for dispatch | **no** | needs Tier 3 |
| a **partially applied** function handed to the host | **no** | needs Tier 3 |
| currying, function factories, combinators as values | **no** | needs Tier 3 |

**The pattern: a callback written at the call site is reachable; a callback *manufactured* and
handed over is not.** Almost every host API is the first kind, because almost every host API is
called from the place that knows what to do.

And one of the "losses" is a win already measured: the JS array methods are declared in
`targets/js/methods.oro` and recorded as uncallable, and
[js-toplevel-2026-08-18](../../gauntlet/results/js-toplevel-2026-08-18.md) puts them at **3.6× to
133× slower than a loop**. The one thing the language cannot reach on that host is the one thing
not worth reaching.

---

## 4. Two hazards to write down before any of this is built

**A callback body must not capture a buffer.** If a lambda body is emitted inside a *host* closure,
the host closure captures our locals — and if one is a `(buffer V)`, two goroutines could hold it
and ADR 0018's linearity is gone:

```lisp
(go.go (fn () (set b 0 1.0)))          ; MUST BE REFUSED
```

The check is the occurrence machinery ADR 0018 already needs: **the free variables of a callback
body may not include a buffer.** Tables are fine — they are immutable and freely shareable, which
is precisely the property that makes concurrency safe here.

**A host closure allocates, and that is correct.** Emitting `go func(){…}()` lets Go build an
environment record. That is hidden allocation, which this project warns about — but it is *the
host's own idiom*, it is what hand-written code does at that call site, and it is therefore at
parity by construction. The rule against hidden allocation is about the **core**; this is the
parasite model using the host's machinery where the host has it.

---

## 5. What this means for concurrency

[ADR 0018](../decisions/0018-immutable-values-linear-buffers.md) was written partly so that
concurrency would be safe when it arrives: tables are immutable and shareable, buffers are linear
and never shared. **Every concurrency entry point on every target is Tier 1 or Tier 2** —
`go func(){}`, `CreateThread(fn, ctx)`, `new Thread(runnable)`, `new Worker(url)` — so the closure
refusal does not block it.

Which is the answer to the question in one line: **the refusal costs us function values, not host
APIs**, and the two places it genuinely bites — dispatch tables and manufactured callbacks — are
places this language has other reasons not to go.

---

## 6. Recommendation

**Fix the diagnostic and Tier 2; specify Tier 1 when a program needs it; keep Tier 3 refused.**

1. **Stop calling a closed lambda a closure.** `emit/*.go` should distinguish a lambda with free
   variables from one without, and say which:
   ```
   in get-fn: `twice` is returned as a value.
     A function with no captured variables can be passed to a target that asks for one,
     but it cannot be returned from an exported function — the caller is host code.
   ```
2. **Allow a top-level function to be referenced by name** where a target primitive declares a
   function parameter. Emit it as a real function and pass its name. No closure, no environment,
   four targets, and it is what every C-style API wants.
3. **Add a callback hole to the target file format** when a program needs Tier 1 — a `%s` that
   takes a lambda's parameters and body. Target-native, capability-graph territory, no language
   change. With the buffer-capture check of §4.
4. **Tier 3 stays refused**, and if it is ever revisited, ADR 0018 must be revisited with it.

None of this is a language change. All of it is emitter and target-file work, which is the shape
of change this project handles well.
