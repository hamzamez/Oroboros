# Types: a direction, not a decision

**Not an ADR and not a specification.** Nothing here is settled, and the language has no types
today — they live in `emit/`, which is a
[measured result](../gauntlet/results/js-2026-08-14.md) rather than an accident: `targets/js.oro`
declares **zero** types, because JavaScript needs none.

This exists because the question was raised deliberately early, while the interface can still
change, and because one measurement taken while answering it is worth keeping regardless of what
we decide.

---

## 1. The measurement that frames everything

The performance argument for a strong type system is that a proof lets you delete a runtime check.
Bounds checking is the cleanest case, so it was measured. Go, 4096-element dot product, 2s × 4:

| | ns/op | per-iteration check |
|---|---|---|
| indexed loop — **what `cmd/gen` emits today** | 1,005 | `IsInBounds` on the second array |
| same loop, reshaped so Go's own BCE fires | **525** | none; one `IsSliceInBounds` before the loop |

**1.94×.** Larger than every host-idiom difference this project has measured — larger than JS's
`Map` vs null-prototype object (3.25× but on a slower path), larger than Java's fused vs unfused
dictionary (2.6×), and it applies to the innermost loop of every array program in the gauntlet.
`go build -gcflags="-d=ssa/check_bce/debug=1"` confirms `generated_dot.go` and
`generated_centroid.go` both carry the check today.

So the premise is **right**: proving `i < len(a)` is worth roughly a factor of two.

## 2. Three corrections to "a proof removes a check"

### 2.1 Our proofs do not transfer. The host must re-prove them.

We emit host source. Go's bounds-check elimination runs on Go's own analysis and has never heard
of us. A theorem in our type system removes exactly zero instructions unless the code we emit is
**shaped so that the host proves it again**:

```go
b = b[:len(a)]          // one IsSliceInBounds, outside the loop
for i := range a {      // Go now knows i is in range for both
```

> **A type system that does not change what we emit buys nothing.**

That is the parasite thesis arriving at the type system, and it is a much harder constraint than
it sounds. Each host re-proves differently: Go has BCE, the JVM has its own, JavaScript engines
have hidden classes and range analysis that we cannot address at all. A proof we can state and no
host can re-derive is decoration.

### 2.2 The easy 1.94× needs no type system

The residual for `dot` is

```lisp
(fold-range 0.0 (alen p) (fn (acc i) (add acc (mul (aindex p i) (aindex q i)))))
```

Everything needed to emit the reshaped loop is **syntactically visible**: the loop bound is
`(alen p)`, and `q` is indexed by the same `i`. An emitter pattern — *narrow every array indexed
by the loop variable to the length that bounds the loop* — collects the whole factor of two with
no types anywhere.

So the type system must not be justified by the cases the emitter can already see. Its value is
the cases where the relationship is **not** syntactically evident: an index computed by
arithmetic, a length carried across a function boundary, a bound established by a caller. Those
are real, and they are a much smaller and more honest claim than "types make it fast."

### 2.3 "The most powerful type system" is the minimality trap wearing a hat

[CLAUDE.md](../CLAUDE.md) already warns against minimising *constructs needed to express all
computation*, because that is not the property this project needs. Maximising *propositions
expressible about a program* is the same error with the sign flipped. The criterion should mirror
the one already adopted for the core:

> **As powerful as necessary subject to (i) every proof being re-provable by some host, and
> (ii) checking terminating.**

Sequent-calculus type systems in the Shen sense — user-supplied inference rules discharged by
proof search — fail (ii) by construction. Shen answers with a depth limit and backtracking, which
means type checking may fail for reasons that are not about the program. This project's core has
one hard-won property, that **reduction terminates at compile time and yields a residual**; adding
a second compile-time process with no termination guarantee fights it directly, and
[ADR 0009](decisions/0009-staging-preserves-results.md) is about exactly that class of hazard.

And it is worth saying plainly: that type system belongs to the predecessor that stalled. The wall
was the boxing in the portability layer, not the types — but adopting the stalled project's most
distinctive component into its successor deserves an argument, not an inheritance.

## 3. What looks right instead

**Refinement types over the range-typed integers already decided in
[ADR 0003](decisions/0003-range-typed-integers.md).**

ADR 0003 committed to integers carrying ranges with mathematical semantics and machine
representation. A range *is* a refinement — `{ i : int | 0 ≤ i < n }` — so half of this is already
a decision, and the type system would be its generalisation rather than a new idea.

Constrain the refinement language to a **decidable fragment**: linear arithmetic over integers,
which is Presburger, which is what Liquid Types uses and what every array-bounds obligation
actually needs. Checking is then a small solver, terminating, with no user-visible search
behaviour.

That buys, in order of how well it is evidenced:

1. **Array bounds** — measured above, and re-provable by Go and the JVM.
2. **ADR 0003's ranges** — overflow and representation selection, which is a *correctness*
   obligation we have already taken on and are not currently discharging.
3. **Signatures that mean something.** This is the argument I did not expect to be the strongest.

### 3.1 The first job is modules, not correctness or speed

[modules.md §2](spec/modules.md) says a signature "names a set of exports and specifies each one's
behaviour". Today a signature is a **list of names** — `(export split-words)` — and nothing more.
The conformance suite exists precisely because the signature cannot state anything a checker could
verify.

A type system is what turns a signature from a list into a claim. And that gives the first
increment a bounded, checkable requirement that has nothing to do with dependent types:

> A signature should be able to say what a name's arguments and result are, so that a target's
> native implementation and a library's fallback can be **checked against the same statement**.

That is [modules.md T2](spec/modules.md)'s substitution soundness becoming machine-checked instead
of asserted, and it is the one job no backend can do for us — because the two implementations
being compared live on *different targets* and no single host compiler ever sees both.

## 4. Order, when the time comes

Nothing here is scheduled. If it were:

1. **Collect the 1.94× with an emitter pattern first**, and measure it on the real gauntlet. It is
   the cheapest large win available and it requires no design.
2. **Signature types** — argument and result types on module exports, checked across targets.
   Small, and it makes something we already built stronger.
3. **Refinements on integers**, decidable fragment, discharging ADR 0003.
4. Anything beyond that only when a gauntlet program demands it, per
   [ADR 0007](decisions/0007-exploration-over-specification.md).

## 5. What would change this document

- A measurement showing the emitter pattern of §2.2 does **not** collect most of §1's factor —
  which would mean the interesting bounds cases are the non-syntactic ones, and move refinements
  up the list.
- A gauntlet program whose correctness cannot be stated without dependent types.
- Evidence that JavaScript engines can be addressed by emitted shape at all. Right now the entire
  performance argument is Go and JVM only, and JS is the target that most needs it.
