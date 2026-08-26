# Differential conformance — 2026-08-26

Seven programs, four targets, built and **run**, outputs required byte-identical and required to be
the right answer. [`gauntlet/differential/`](../differential/).

It was built because emitting is not the same as being right, and the pass condition was set before
writing it: **it must reproduce both of the previous day's silent wrong-answer bugs when their fixes
are reverted.** It does.

It also found three things on its first run, none of which was the kind of bug it was written for.

---

## 1. The pass condition

| revert | what the suite says |
|---|---|
| the skip in `changedArgs` ([loopshape §3](loopshape-2026-08-25.md)) | `go 0 1 2 3 4 4 4 4 4` / **`js 0 1 1 2 2 2 2 2 2`** / **`java 0 1 1 2 2 2 2 2 2`** / `windows 0 1 2 3 4 4 4 4 4` |
| the term case in `elemOf` ([wintables §4a](wintables-2026-08-25.md)) | `go 0 2 5` / `js 0 2 5` / `java 0 2 5` / **`windows 0 1 2`** |

Both bugs compiled cleanly, ran, and returned numbers. The example sweep and the existing
conformance runner both pass on both.

---

## 2. Two checks, because agreement is not correctness

**Agreement** across targets is the differential half, and no target is the oracle.

**`; expect:`** is the other half, and it is not decoration: four backends can be wrong in the same
way, and the one bug a purely differential test cannot see is a bug in the **reader** or the
**reducer**, which all four share. A case with no `; expect:` line fails.

Both checks were verified by breaking them — a deliberately wrong `; expect:` prints
*"every target agrees, and all of them are wrong"*.

---

## 3. What it covers, and what that is worth

`loop`/`again`, `match` with `when` guards, `sum` and `case` with a **dynamic** tag, `values`
consumed in place, `alloc`, `build`/`set`, `len`, indexing by application, and the `array` literal
with an index that cannot fold.

Every one of those is a **Tier 1 language construct** built or changed in the last four days, on
four backends, and until now the only thing checking them across targets was
[the inventory audit noticing they had nothing](../../docs/spec/inventory.md).

The answers are independently right rather than merely consistent: `table-alloc` gives Σi² for
i < 10 = 285, `values-product` gives 3n+1, `array-literal` sums a prefix of `10 20 30 40 50`.

---

## 4. Three findings on the first run

None of these is a codegen disagreement, which is what the suite was written to catch. All three
are things nobody had a reason to look at.

### 4a. The native Java target could not build a program at all

`targets/java/java.oro` declared `(build "javac -d %s %s")`. `cmd/build` fills the second hole with
the source **directory** and runs the command inside it — which `go build` accepts and `javac` does
not, because javac wants source files:

```
error: invalid flag: C:\…\oro-build-2265161361
```

`portable-java.oro` had it right — `javac -encoding UTF-8 -d %s Main.java` — and the native target
did not. **Nothing noticed because every Java result in this repository came through `cmd/gen` plus
`javac` invoked by hand**, including all seven gauntlet programs the day before. Fixed; the native
Java target builds and runs a program now, and `-encoding UTF-8` came with it, which javac needs
because it otherwise defaults to the platform charset ([strings.md §5](../../docs/spec/strings.md)).

### 4b. `win/fmt.print-int` prints a blank line for a negative number

Found by a case whose `err` branch returned `(@sub 0 e)`. windows printed six lines where the other
three printed nine.

This is **not** a bug in the implementation. `lib/win/fmt.oro` declares

```lisp
(sig print-int ((n int)) any (where (and (<= 0 n) (< n 9007199254740991))))
```

so a negative argument is outside its stated domain and it makes no claim there. The digit loop
exits immediately on `(x64.setg m 0)` and writes the one byte it had already stored — the newline.

The bug is that **the precondition was not enforced**, which is 4c.

### 4c. A `where` on a DEFINITION is not checked at any call site

```lisp
(sig safe ((n int)) int (where (<= 0 n)))
(def safe (fn (n) (go.+ n 1)))
(def f (fn (x) (safe (go.- 0 5))))      ; accepted
```

`refinements.md` says an obligation "is discharged at every call site" — and that is true of a
**primitive's** `where`. A definition's is not checked anywhere, so
`(sig print-int … (where (<= 0 n)))` is documentation rather than a check.

The mechanism is worth stating because it is not an oversight in the refiner: **reduction inlines
the call**, so by the time the residual reaches `Refine` there is no call site left and no name to
attach the obligation to. The check has to happen before or during reduction, which is a different
place from where every other refinement lives.

> **Corrected the same day, after testing it properly.** The paragraph above is right that the
> clause is not checked and wrong about what follows. Three things came out of pushing on it, and
> they are written up in [refinements.md §6b](../../docs/spec/refinements.md):
>
> **The `where` is DROPPED, not assumed** — a missing check, not an unsound one. Nothing is ever
> told something false.
>
> **Inlining is the enforcement mechanism, and it is stronger than the declaration.**
> `(safe a (go.- 0 5))` IS refused, because `(a -5)` lands in the residual and the indexing
> obligation catches it on the concrete value. And a `where` is only a conservative summary, so
> checking it instead would be *less* precise: a declared `(< n 100)` on a body that needs
> `n < len a` rejects a legal `(get a 400)` against a 500-element array, which the propagated
> obligation accepts. **A naive fix would be a regression.**
>
> **What is genuinely lost is one nameable category**: a precondition that states MEANING rather
> than guarding an obligation, where the body is total and merely wrong outside its domain.
> `print-int` is exactly that, and it is the same shape as SAL's `_Success_`.
>
> So this is not one item needing an ADR. It is a documentation gap — three meanings on one syntax,
> now written down — and one open question, which belongs beside the SAL work rather than ahead of
> it.

---

## 4b. And an eighth case, added the same day

[`json-tokenize`](../differential/cases/json-tokenize.oro) — a JSON tokeniser, and the one case here
that is a **program** rather than a construct ([json-2026-08-26](json-2026-08-26.md)).

It is worth recording what that bought, because the seven construct cases had all passed and every
construct the tokeniser uses was already among them. It found **four** things anyway: `typeOf` of a
`build` assumed the body hands the buffer back, `any` demanded something in two checks that had a
`compatible` helper and did not call it, and x86 emitted a memory-to-memory `mov` because nothing
before had enough live values to spill the destination of a `len`.

**All four are about the program being bigger, not about a construct being new.** A construct suite
and a program are different tests, and this suite should keep at least one of each.

## 5. What it does not do

It runs **integers only**, because float formatting differs per host by design — `3.328335e+08` on
Go, `332833500` on JavaScript, `3.328335E8` on Java — and a differential test of a deliberate
divergence would only ever fail.

It builds four artifacts per case, so it is slow: seven cases is about four minutes. It is not part
of `go test`; it is run deliberately.

It cannot see a bug that is identical on all four targets **and** matches the expected answer — but
that is the case where the expected answer is wrong, which is a person's mistake and not a
compiler's.
