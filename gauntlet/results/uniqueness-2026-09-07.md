# A buffer is a nameable type, and the boundary cost is gone

2026-09-07. [ADR 0020](../../docs/decisions/0020-uniqueness-on-parameters.md)
built, on the research in [uniqueness.md](../../docs/uniqueness.md).
[examples/kara/workspace.oro](../../examples/kara/workspace.oro),
`emit/linearity.go`, `core/read.go`.

ADR 0018 predicted exactly one residual gap — *"a portable program still
allocates where a caller-supplied buffer would not, at an EXPORTED boundary"* —
and [arrays-revisited](../../docs/arrays-revisited.md) fired its trigger 2 on a
measurement: Karatsuba's workspace, **1.07x–1.66x, 10 allocations and 504 KB per
multiply**. That was measured on *hand-written Go*, because the Oroboros side
could not be written at all.

> **`(sig f ((w (buffer int))) …)` parses, checks and emits.** The same body,
> exported twice, differing only in where the workspace comes from:
> **18,509 ns and ZERO allocations against 84,167 ns and 512 KB — 4.5x.**
>
> **51 lines of non-comment code across three files.** No new term kind, no
> attribute, no coercion, no borrow machinery — and no backend learned that
> buffers exist: the parameter emits as `[]int`, `long[]`, a plain JS array and
> a register.
>
> **On windows it is not a speed question at all.** That allocator is one
> `VirtualAlloc` per `build` and never freed, so the allocating form leaks
> 512 KB per call. `gen_mac_fresh` contains one `VirtualAlloc`; `gen_mac_into`
> contains none.

---

## 1. The measurement

`mac` multiplies two byte arrays elementwise into a workspace of 65,536 `int`.
`mac-into` takes the workspace as a `(buffer int)` parameter; `mac-fresh` is
**the same shared body** with the workspace `build`-ed inside. Both are exported,
so reduction cannot remove either boundary — which is the whole point, since
inside a program reuse was already free and `examples/kara/core.oro` already
demonstrated it.

Go 1.x, `-benchtime=2000x -count=7`, medians:

| | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `GenMacInto` — ADR 0020 | **18,509** | **0** | **0** |
| `GenMacFresh` — build inside | 84,167 | 524,288 | 1 |

**4.5x** (an earlier pass gave 18,835 / 79,768, 4.2x; the noise floor here is
~15% and the effect is far outside it). `TestWorkspaceFormsAgree` checks the two
forms return the same 65,536 products before either is timed.

### 1.1 Why this is bigger than 1.66x, said before it is asked

**The work is proportional to the workspace, deliberately.** arrays-revisited
found the cost *"growing with the size of the workspace relative to the work"*,
so this kernel is one pass over the buffer — the ratio at which the allocation
is comparable to the arithmetic. Karatsuba does O(n^1.58) work over a linear
arena, which is why it sits at 1.07x–1.66x.

**This is the same effect measured where it is largest, not a larger effect.**
Quoting 4.5x as *the* cost of the boundary would be the mistake this repository
has made and caught four times: a number without its condition. What is
unconditional is the allocation column — **0 against 1**, and 0 against 512 KB.

## 2. What was built

**The whole surface is one type name.** `(buffer V)` joins `(array V)` and
`(map K V)` in `TypeName`, canonicalised to `"buffer V"` the same way.

That it is this small is not luck. **ADR 0018 had already made uniqueness a
distinction between two type CONSTRUCTORS rather than an attribute on every
type** — `(array V)` shared and immutable, a buffer linear and scoped — so
there is no attribute lattice, no attribute variables and no inferred coercion,
which is most of Clean's implementation and all of its reputation for unreadable
signatures. The one coercion Clean infers, we already write, as the freeze.

**And the linearity half is `CheckLinear` with a different SEED**, which is
exactly what uniqueness.md predicted:

```go
if sig != nil && t.Kind == core.KFn {
    body, raw, _ := openFresh(t, map[string]bool{}, asmIdent)
    for i, name := range raw {
        if core.IsBuffer(sig.Params[i].Type) {
            c.walk(body, name, &linState{declared: true})
        }
    }
}
```

The SAME walk `build` already used, started from the signature instead of from
`build`'s binder. Nothing about the rule changed — only where the buffer came
from — and `linState` carries one bit so the diagnostic can name ADR 0020's
parameter rather than ADR 0018's `build`.

**The two obligations sit at opposite ends** (uniqueness.md §3, after
Marshall/Vollmer/Orchard, ESOP 2022). *Uniqueness in* — no other reference
exists — is the CALLER's promise, and at an export the caller is outside the
program, so it is **assumed**: refinements.md §6b's middle row. *Linearity
through* — moved exactly once, read freely before — is the BODY's promise, and
it is checkable from the body, which is what the code above does. **Neither
alone suffices**: uniqueness without linearity lets the body read after a store;
linearity without uniqueness lets the caller observe the write.

## 3. A BUFFER IS A TABLE FOR EVERY PURPOSE BUT ALIASING

The clearest evidence that this is one type name and not a feature is what the
four backends emit:

```go
func GenMacInto(w []int, a []byte, b []byte) []int          // go
public static long[] genMacInto(long[] w, short[] a, …)     // java
export function genMacInto(w, a, b)                         // js
gen_mac_into proc                                           // windows
```

**No backend was changed.** A buffer's element, its width, its indexing, its
bounds obligations and its representation are an array's; what differs is *who
may hold it*, and that is a compile-time question that never reaches emission.
So `ArrayElem` answers for both — `ArrayElem("buffer int 0 255")` is
`"int 0 255"` — and `IsBuffer` is the one predicate that distinguishes them,
read by the one caller that has to.

**Rule 6 — a buffer may not be an element type** — is enforced in the `array`,
`buffer` and `map` constructors, the three places a compound type is built. It
is load-bearing rather than tidy: it is what keeps the read-borrow free. `(b i)`
yields a scalar or a frozen array, so an observation **cannot alias the buffer**,
which is why *reads do not consume* needs none of Wadler's `let!` (1990) or
Odersky's observers (1992). Allow `(array (buffer V))` and that argument fails.

## 4. Cost

| | |
|---|---|
| emitted files | **65 of 65 pre-existing byte-identical**; four new — `workspace.oro` on all four targets |
| differential suite | 28 cases, four targets, green |
| unit tests, `go vet` | green |
| code | 51 non-comment lines: `core/read.go`, `emit/linearity.go`, two call sites |

**Three tests, each verified to fail against its bug**: `CheckLinear` unseeded (a
read after a store into a buffer parameter is accepted), rule 6 removed (all
three of `(array (buffer int))`, `(buffer (buffer int))` and
`(map int (buffer int))` accepted, with an array control that must still pass),
and `ArrayElem` blind to a buffer.

**No pre-existing emitted file changed**, which is right for a change that adds a
declaration and removes a refusal. Nothing about what an existing declaration
means moved.

## 5. Three honest limits

**The uniqueness half at an export is assumed and cannot be checked.** The ADR
says so and it is still true: a host caller passing the same buffer twice gets a
wrong answer, silently. Unlike an exported `where`, that is a **memory-safety**
assumption rather than an arithmetic one. This is the cost of the decision, not
an omission in the build.

**No differential case is possible, for a structural reason — the fourth
instance.** The runner's entry point is `(run n)` called with integer literals; a
buffer parameter can only be supplied by a *host* caller, so the path a case
would exercise is the one it cannot reach. scalarrange-2026-08-31 wrote such a
case and **deleted** it for exactly this reason: reduction inlines every
non-exported call, so **a declared parameter only survives at an EXPORT**.
Covered instead by unit tests, by `workspace.oro` emitting on all four targets,
and by the Go agreement test.

**`examples/kara/core.oro` could not be the demonstration**, which was tried
first. That file was written for `cmd/oro` — it reduces, and it does not *emit*
under bounded-by-default, because `pack`'s accumulator and the descriptor-driven
products are unprovable. Exporting its `mulpass` therefore fails on ADR 0019
before ADR 0020 is reached. The kernel above is purpose-written for the boundary
and says so.

## 6. What this closes

**ADR 0013 is superseded.** The allocation price is now *avoidable by
declaration* rather than accepted, which is ADR 0019's shape applied to memory.

**ADR 0018's trigger 2 is discharged**, and consequence 3 is amended on
ADR 0018's own terms: the check is still `occurrences` on the residual and is
unchanged; uniqueness now appears in a signature.

**Still open, and named rather than assumed: ergonomics.** ADR 0020's trigger is
*a program in which threading the workspace is what makes the program
unpleasant*. This kernel threads one buffer through one loop and is pleasant, so
it is evidence about the mechanism and none at all about the cost — which is the
assessment's *write an application* arriving from a third direction.
