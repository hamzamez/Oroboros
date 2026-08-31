# Arrays, re-examined — 2026-08-31

Research. **No decision and no ADR** — this document re-opens
[ADR 0018](decisions/0018-immutable-values-linear-buffers.md) on purpose, eleven days and about a
dozen programs after it was written, and asks the question hamza put: *are our decisions still
sound, or would a freely accessible array — mutable or immutable by default — be better?*

ADR 0018 was **argued, not measured**. It says so itself: *"This ADR is that hypothesis argued; it
is not yet that hypothesis measured."* Everything since has been evidence, and none of it was
collected to answer this question. So the honest thing is to point it all at the ADR and see what
survives.

**Verdict up front: ADR 0018 survives, and it survives for reasons that did not exist when it was
written.** But one of its four reopening triggers has now **fired with a number**, and it is not the
one the ADR said to watch.

---

## 0. What would falsify it, stated before looking

ADR 0018 names four reopening triggers, which is the useful thing about it. They are the falsifiers,
so this document tests them rather than re-arguing the decision.

| | trigger | how it is tested here |
|---|---|---|
| 1 | Occurrence counting rejects programs that are obviously fine — *"the one to watch, and it can only be found by writing awkward programs"* | **write the most awkward program available in Oroboros** |
| 2 | A measured case needing buffer reuse across an exported API | **measure it** |
| 3 | The linearity diagnostic proves untranslatable | not tested — no instance has appeared |
| 4 | Closures are added to the language | not fired — closures are still refused ([callbacks.md](spec/callbacks.md)) |

Trigger 5 (a target whose allocator we own and whose footprint matters) is about *reclamation*, which
ADR 0018 already delegates to the target, so it cannot falsify the language decision.

---

## 1. Trigger 1: NOT FIRED, tested on the hardest program we have

The ADR says this can only be found by writing awkward programs. Since it was written, the awkward
programs got written — the JSON tokeniser, `tree.oro` with two live buffers (one nested inside the
other, one frozen and read back), and **Karatsuba**, which is the most aliasing-hostile thing in the
repository: one arena, a descriptor table driving computed offsets, and two of every node's three
children being *subranges of the parent*.

Karatsuba exists here hand-written in Go, Java, JavaScript and MASM, and **not in Oroboros**. That
is the experiment ADR 0018 asked for and nobody had run.

### 1.1 It compiles, and it is right

Its structural core — a schoolbook multiply at offsets read out of a descriptor table, over one
arena — was written in Oroboros and produces the correct product. Every shape that would break a
linear discipline is in it:

- **three live buffers** at once (arena read, descriptors read, product written);
- **nested loops**, the buffer threaded through both;
- **a buffer crossing a binder** (`macrow` takes `out` and hands it back);
- **every index computed from a read of another buffer**;
- and separately, **read and write of ONE buffer at two different computed offsets** — `b[4+i] -= b[i]`
  over overlapping ranges, which is exactly the tiled combine's `z2 -= z0`.

`occurrences` accepted all of it. **No linearity error, on any of it.**

### 1.2 What DID complain, and why it is a different subject

```
build: (out (go.+ i j)) is an indexing, and (< (go.+ i j) (len out)) does not follow
  known: 0 <= i, assumed (go.lt i (desc 2)), 0 <= j, assumed (go.lt j (desc 2))
```

The **refinement layer**, not the memory model. `(desc 2)` is a value read out of a buffer, which is
stratum 0 and therefore ⊤ — the exact limitation [frozen-2026-08-28](../gauntlet/results/frozen-2026-08-28.md)
records for `tree.oro`'s `d`, and which that result explicitly says an octagon would *not* fix,
because bounding a value read out of a table needs a quantified array invariant.

Clamping, as both parsers already do, makes it compile and run. So the honest sentence is:

> **The memory model expresses Karatsuba. The analysis cannot yet prove its indices, and that is a
> known, separately-tracked, orthogonal gap.**

Conflating the two would have produced a false verdict in either direction.

### 1.3 And reuse inside a program is free, as claimed

ADR 0018 asserts that the decision *"gets the same power inside a program for free"* because
reduction removes every non-exported boundary. Tested rather than taken: two multiplies threaded
through **one** workspace buffer, in a loop.

It works, and the emitted Go allocates the product **once** for both multiplies. Threading a buffer
through `again` is exactly what ADR 0018's linearity is for, and it is the same shape `tree.oro`
already uses for `nodes`.

---

## 2. Trigger 2: FIRED, and here is the number

ADR 0018 predicts precisely one residual gap: *"A portable program still allocates where a
caller-supplied buffer would not, at an **exported** boundary."* It has never been measured. It can
be, because `KWork2` is exactly a caller-supplied workspace: allocated once, reused across every
multiply.

A **buffer is not a nameable type** — there is no `buffer` type name anywhere in the compiler — so an
exported Oroboros multiply must build its workspace inside itself, every call. Measured on Go,
medians of three, the only difference being where the workspace comes from:

| | workspace reused | fresh per call | cost | allocs/op |
|---|---|---|---|---|
| 1024 limbs, D=5 | 721,064 ns | 1,195,545 | **1.66×** | 10 / 504 KB |
| 1024 limbs, D=3 | 1,101,785 | 1,637,509 | **1.49×** | 10 / 198 KB |
| 256 limbs, D=4 | 107,230 | 142,964 | **1.33×** | 10 / 95 KB |
| 256 limbs, D=2 | 131,587 | 140,586 | **1.07×** | 10 / 30 KB |

(1024 D=7 spans 1,023k–1,498k on the reuse side — a 46% spread against a ~15% noise floor — so it is
not quoted.)

**1.07× to 1.66×, growing with the size of the workspace relative to the work.**

### 2.1 Why this one matters more than the stencil's did

ADR 0013 accepted 1.79×/2.01× on the stencil, and this is a smaller number. But
[bigarith-2026-08-28](../gauntlet/results/bigarith-2026-08-28.md)'s entire result is that ours beats
`math/big` **because we allocate nothing** — `math/big`'s inner loops are hand-written assembly and
better than ours, and what we avoid is its per-operation allocation overhead. Naive `math/big` is
4–5× worse than careful `math/big` for exactly this reason.

So an Oroboros bignum that allocates 10 objects and half a megabyte per multiply **becomes the thing
it beats.** That is a qualitatively different situation from the stencil, where the price was a
constant factor on a program that was winning anyway.

### 2.2 But it is trigger 2, which has a named answer that is not free mutation

ADR 0018 already says what firing this means: *"it should produce an ADR adopting uniqueness on
parameters rather than a workaround"* — option (b), Clean/Futhark/Cogent style, *"the only option that
reaches buffer reuse across an exported boundary"*, and chosen by the two languages with our exact
constraints.

**Nothing about this points at free mutation.** It points at letting a *linear* buffer be named in a
signature.

---

## 3. What now DEPENDS on linearity, and did not when the ADR was written

This is the part that has changed most, and it changed in ADR 0018's favour. Seven things now rest on
it, and four of them are **measured results** that postdate the decision.

**(1) The buffer element theorem** — [containment-2026-08-27](../gauntlet/results/containment-2026-08-27.md).
The property quantifies over *reads* and the harness checks *writes*, and that is a theorem rather
than a shortcut: a slot holds either the zero fill or the most recent `set`, **there being no third
source** — `build` is the only allocator, `set` the only store, *and linearity means nothing else can
have written it*. That is what makes checking the stores **sufficient**. Free mutation deletes the
third clause, so `ElemType` becomes unsound, so `[]byte`/`short[]` selection goes.

**(2) The frozen-buffer read theorem** — [frozen-2026-08-28](../gauntlet/results/frozen-2026-08-28.md).
A read out of a frozen buffer carries `γ(ElemType(b))`, justified by a **stratification**: stratum 0
inside the build lambda, stratum 1 after the freeze. Free mutation means there is no freeze, so there
is no stratum 1, so the theorem does not exist. `tree.oro`'s 92.7% and its 74-of-74 `go.-` depend on
it.

**(3) β itself.** `(array V)` reads are pure, which is what lets reduction substitute a table. Under
free mutation every read is impure, so ADR 0010 never substitutes one, so every read is let-bound —
and worse, ADR 0018 §(f)'s point stands: *substitution would change meaning*. This is not a slowdown,
it is a soundness loss in the reducer.

**(4) η-tab.** `(alloc (table (len a) (fn (i) (a i)))) = a` is a rewrite the compiler may take only
because tables are immutable. It was ADR 0013's fifth reopening trigger and ADR 0018 fired it. Free
mutation un-fires it.

**(5) `modifies` is syntactically the buffer** — [lowstar-lessons.md](lowstar-lessons.md). Low\*'s
HyperStack needs a `modifies` clause per function and it is **HACL\*'s largest proof burden**. ADR
0018 gets it for free, and lowstar-lessons calls this ADR 0018's best argument. Free mutation
reintroduces HACL\*'s worst problem, in a project that has no proof assistant to absorb it.

**(6) Data races are not expressible.** Tables are Pony's `val`, buffers are Pony's `iso` —
`Send`/`Sync` without writing them down. Free mutation makes races expressible and drags in the Go,
Java and C++11 memory models, which **disagree**, across four hosts. That is a Tier 2 catastrophe in
the core.

**(7) The parallel/sequential distinction is in the source.** `(alloc (table n f))` is parallel by
construction; `(build …)` is sequential by construction. That is Futhark's design and the reason its
parallel programs are deterministic. Free mutation erases the distinction from the source, so it can
only be recovered by analysis — which is option (a), already rejected because *an optimisation that
silently does not fire is the failure mode requirement 5 does not tolerate*.

**Four of these seven were not available as arguments in ADR 0018**, because (1), (2) and their
supporting measurements did not exist. The decision is better supported now than when it was made.

---

## 4. The alternative, taken seriously

### 4.1 The real axis is not mutable-vs-immutable, it is ALIASED-vs-not

*Mutable or immutable by default* is a question about **defaults**, and it is not the load-bearing
one. Everything in §3 breaks on one condition and one only: **can two live names refer to storage one
of them writes?**

There are exactly three known ways to guarantee they cannot, and all three are in the literature:

| | mechanism | who | what it costs |
|---|---|---|---|
| **immutability** | there is nothing to alias | Haskell, SISAL | cannot express scatter at any speed |
| **linearity** | there is only ever one name | Clean, Futhark, Cogent, ADR 0018 | threading; and reuse stops at a boundary |
| **ownership + borrowing** | many names, one writer, checked | Rust | a borrow checker, lifetimes, and a much larger language |

ADR 0018 takes **immutability for values and linearity for buffers** — two of the three, applied where
each is cheapest. "Mutable by default" without one of these three is ADR 0018 §(f), *unchecked
portable mutation*, which the ADR rejects without argument and §3 above now says why in seven places.
"Mutable by default" *with* one of them is either what we already have, or Rust.

> **So the proposal does not survive being made precise: made precise, it is either what we have, or
> it is Rust, or it is the thing that costs seven measured properties.**

### 4.2 What free mutation would actually buy

Against those seven, the honest ledger of gains:

| candidate gain | status |
|---|---|
| Karatsuba's aliasing shape | **already expressible** — §1.1 |
| workspace reuse inside a program | **already free** — §1.3 |
| workspace reuse across an exported boundary | **real, 1.07–1.66×** — but option (b) buys it too, losing none of the seven |
| ergonomics: not threading `b` through `again` | **real, unmeasured** |

Three of four are zero or available more cheaply. Free mutation is **strictly dominated** by
uniqueness types: it buys one thing that (b) also buys, and pays seven things (b) does not.

### 4.3 The one honest cost that remains

Threading a buffer through `again` and through helper functions is genuine syntactic noise, and the
Karatsuba core written for §1 shows it — the buffer appears in every loop variable list and every
helper's parameters. This is exactly the **ergonomics** thread already open twice in CLAUDE.md
against ADR 0014, and it has the same shape: nobody has measured it, and it is the kind of thing that
only a real application decides.

It is worth naming that it is *not* the same as trigger 1. Trigger 1 is *"occurrence counting rejects
programs that are obviously fine"* — a correctness/expressiveness failure, and it has not happened.
Ergonomic noise is a different complaint with a different remedy, and the remedy is probably sugar
rather than a memory model.

---

## 5. Verdict

**ADR 0018 stands.** Not on the argument it was made with, but on evidence collected since and not
for this purpose:

1. **Trigger 1 has not fired**, tested against the hardest aliasing shape available. Karatsuba's
   core compiles, runs, and is right.
2. **Trigger 2 has fired, at 1.07×–1.66×**, and its named answer is uniqueness on parameters, not
   free mutation.
3. **Seven properties now depend on linearity**, four of them measured after the ADR was written.
4. **Free mutation is strictly dominated** by the option ADR 0018 already deferred.

The one thing that changes is the *urgency* of option (b). ADR 0018 deferred it saying *"no measured
case in this repository needs it."* **That sentence is now false**, and the counterexample is
Karatsuba.

---

## 6. What this means for maps

The map question that prompted this — *is a map a value or a buffer?* — now has a derived answer
rather than a chosen one.

**A map is both, on exactly the same terms as an array, because the discipline is about aliasing and
aliasing does not care what the index set is.** A growing map is a linear buffer inside `build`; a
frozen map is an immutable value. `(map K V)` reads are pure, `(mapbuf K V)` reads are impure, and
`occurrences` is the check. Nothing new.

Two things fall out that are worth having:

- **Go's map is a reference type** — `m[k] = v` mutates in place and there is no `append`-style
  reassignment — so `(set-map m k v)` returning `m` is *exactly* Go's own semantics, and linearity is
  what makes threading it safe. The host that looks least like our model turns out to need the least
  translation, which is the same surprise `(T, error)` gave sums.
- growth.md §1.1's asymmetry — append keeps an **equation**, insert keeps only an **interval** — is
  about what the *analysis* can prove, not about the memory discipline. It survives this unchanged.

---

## 7. What to do next, in order

1. **Nothing here blocks the map spec.** §6 settles its open question, so
   [maps.md](spec/maps.md) can be written against a derived answer.
2. **Uniqueness on parameters is now owed an ADR**, not a build — ADR 0018 says firing trigger 2
   *"should produce an ADR adopting uniqueness on parameters rather than a workaround."* The
   measurement it asked for exists now.
3. **Not yet done and worth doing**: the *full* Karatsuba level-walk in Oroboros, not just its core.
   §1 tested the shapes; it did not test the whole algorithm, and the descriptor pass is where the
   remaining doubt is.
4. **Still unmeasured, and it is the one that could still move this**: ergonomics. §4.3. It needs a
   real application, which is the same thing precision-by-declaration is waiting for.
