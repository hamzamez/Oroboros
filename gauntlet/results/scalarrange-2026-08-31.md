# A scalar range is a type — 2026-08-31

[ADR 0019](../../docs/decisions/0019-precision-by-declaration.md) names four things owed before
precision by declaration works, and puts one first: **a scalar range is not usable at all today**.
It is closed. This is a **capability and correctness** result and deliberately not a speed one:
every emitted file on all four targets is byte-identical across 41 programs.

---

## 0. What was wrong

```
(sig sq ((n (int 0 1000))) int)
→ sq: in argument 1 of go.*: n is int 0 1000, but int is required here
```

The syntax parsed and then the type checker refused **every use of the parameter**. A legal program
rejected by its own declaration.

The cause was one line. `core.ValueType` — *"what a range MEANS, as opposed to how it is stored"* —
was called at **exactly one site**, the table-read path. So the range language worked on array
elements and nowhere else, and a scalar's range had to be spelled as a `where`.

## 1. A range has three effects, and conflating any two is a real bug shape

| | what it means | where it lands |
|---|---|---|
| **type** | `(int LO HI)` **is** `int` | normalised at the point of **demand** |
| **premise** | `LO ≤ n ≤ HI` | desugars into the `where` it means |
| **representation** | which rung of ADR 0003's ladder | **not this change** — ADR 0019's |

A scalar is the host's own word at every finite range. Only a table's element slot consults a width,
which is what `ValueType` exists to say.

## 2. The premise half is a desugaring, and that is why it costs nothing

`(n (int LO HI))` and `(n int) (where (and (<= LO n) (<= n HI)))` have the same denotation —
γ(int LO HI) = {k | LO ≤ k ≤ HI} is exactly the satisfying set of that conjunct — so making one the
sugar of the other is a **definition, not an approximation**.

Doing it in the reader is what makes it free. `where` is already read by the refinement layer, the
interval layer and termination; a range that became a fact by its own path would need each of those
taught, and **this repository has three recorded cases of a helper that existed and was not called at
every site**. Sugar that erases in the reader is what `let`, `seq`, `and`, `cond`, `match` and
`values` all are.

**The reader is the only producer of a signature with named parameters** — checked, not assumed:
`core.Sig` is constructed in two places, and the other carries an already-desugared `Where` and no
parameters. So reader-side desugaring is complete rather than merely convenient.

A range on an **unnamed** parameter is a type and nothing else, because a refinement attaches to a
name. That is refinements.md's existing rule, not a new limitation.

## 3. The type half is one comparison point

`compatible` is the only place two types are compared — `agree` is now defined in terms of it — so
normalising there has no second site to forget.

`core.ValueType` strips only a **top-level** range, and that is load-bearing rather than incidental:
`array int 0 255` does not begin with `int `, so it passes through unchanged and `(array (int 0 255))`
stays distinct from `(array int)`. A target that declares `[]byte` still refuses an `[]int` program,
and elemwidth is untouched.

## 4. The refusal was standing in front of a SILENT WRONG ANSWER

With the checker no longer refusing, the declaration reached the emitter for the first time — and
`seedFromSig` put the declared type straight into the emitter's type map:

```go
func GenSq(n uint16) int { return int(n * n) }   // GenSq(1000) = 16960
```

`1000*1000` is 1,000,000; at `uint16` it wraps to **16,960**. Verified by running it.

The bug was **latent for as long as the range language has existed**, because nothing could ever
reach that line. That is worth recording on its own: *a refusal can hide a wrong answer, and removing
the refusal is what finds it.*

The fix is the third `ValueType` site. The test is stated as the theorem rather than as a spelling:
**a range and the `where` it means must emit the same function.** That is stronger than asserting
`int` appears, and it is the property that catches this. It fails against the reintroduced bug.

## 5. The result position was a declaration NOBODY CHECKED

postconditions.md's algebra is a swap, and a range in the result position is that swap written in the
type language: `result : (int LO HI)` is `(and (<= LO result) (<= result HI))`, an `ensures`.

Before this it was decoration. The two spellings of one false claim disagreed:

| | |
|---|---|
| `(ensures (and (<= 0 result) (<= result 5)))` | **refused** — *"the body does not establish …; its result is [0, 10000]"* |
| `(sig sq ((n (int 0 100))) (int 0 5))` | **accepted in silence** |

Now both are refused, with the interval that disproves them. It rides on postconditions.md's
trichotomy unchanged: assumed on a prim, checked against the body on an exported definition, gone on
an internal one.

### 5a. And the synthesised conjunction had to be built ERASED

The first version built `(and a b)` and it came back *"outside the decidable fragment, propagated and
not proven"* — for a conjunction the layer decides perfectly well when the reader writes it.

**The connectives do not survive reading** (ADR 0017): `and`, `or`, `not` and `cond` are sugar and
nothing downstream has ever seen one. A term synthesised *after* that erasure has to be built in the
erased form, `(if a b false)`, or it is a shape no consumer knows. There is one spelling of a
conjunction now, `core.conj`, and the `and` desugaring uses it too.

**That failure mode is worth naming: `CheckEnsures` returns SUCCESS with a note when a claim is
outside its fragment.** So the wrong form did not fail — it passed while checking nothing, and a test
asserting only "not refused" would have passed with it. The test asserts the claim is **decided**.

## 6. Two smaller things

**An empty range is not a type.** `(int 100 0)` denotes ∅ — no value inhabits it, so a parameter
declared with one can never be called. It used to flow on as the string `"int 100 0"` and surface much
later as *"n is int 100 0, but int is required here"*: a mismatch reported against the wrong thing, in
the wrong place. Now refused where it is written. Pinned with a control, since the refusal must not be
implemented as *lo < hi* — `(int 5 5)` is a legal single-point range.

**A range and a `where` compose**, both conjoined; the sieve's `(where (and (<= 0 n) (< n 1048576)))`
rewritten as `(int 0 1048575)` emits a **byte-identical** file with the same 10 of 10 operations
bounded and 3 of 3 loops proven.

## 7. THE DIFFERENTIAL SUITE CANNOT REACH THIS, and a case was written and DELETED

A case was written, and it **passed with the bug reintroduced**. A harness that cannot fail proves
nothing, so it was removed rather than kept as a green row that can never go red.

The reason is structural and is the same one CLAUDE.md already records for index narrowing:
**reduction inlines every call, so a declared parameter only survives at an EXPORT** — and the
differential harness is a whole program that calls `run` with literals. The exported `GenRun` is
emitted with the wrong signature and `GenMain` never calls it, so the output the suite reads comes
entirely from folded constants.

Worth stating because the near-miss is tempting: the suite is **not** blind here for the reason it is
blind to a bad element narrowing. That blindness is *every target narrows on the same decision, so
they agree and are wrong together*; here JavaScript declares no `int-repr` at all and would have
disagreed. The suite would catch this bug — it just cannot be made to *execute* the code that has it.

`cmd/gen`, which emits an export as a function without a whole program around it, is the tool that
reaches it, and that is what the emitter-level test uses.

## 8. What it costs and what it buys

**Byte-identical output on 41 programs × 4 targets.** The differential suite passes on all four. No
program in the corpus used a scalar range, because it did not work.

So the honest ledger is: **no speed claim, one refusal of a legal program removed, one latent silent
wrong answer fixed, one silently-ignored declaration made real** — and the surface ADR 0019 needs now
exists. The declared range is preserved in `sig.Params[i].Type`; only its *consumers* normalise, so
representation selection can read it when it is built.

## 9. What is still owed

ADR 0019's remaining three, unchanged: a spelling for the unbounded rung, a **bidirectional**
representation solver (factorial is the witness — the pressure comes from the declared result), and
R3 per target. And the trigger nothing measured can settle: **how many declarations a real
application needs.**
