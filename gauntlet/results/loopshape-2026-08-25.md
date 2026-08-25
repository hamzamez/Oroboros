# The loop shape — 2026-08-25

[tables-write-2026-08-25](tables-write-2026-08-25.md) found the emitted sieve **1.4x** hand-written
Go and isolated the cause to our loop shape rather than to tables. This is the fix and what it cost
to get right.

**Result: the sieve is at parity.** 322,877 ns against hand-written 315,973 — **1.02x**, down from
447,000–484,000.

---

## 1. The shape, and which half of it mattered

We emitted:

```go
i := 2
for {
	if i*i >= n { break }
	if c[i] { i = i + 1; continue }
	…
	i = i + 1
	continue
}
```

against a person's `for i := 2; i*i < n; i++`. The increment is duplicated into every clause, so the
loop has several back edges.

Both halves of the difference were candidates — the condition living in a `break`, and the
increment living in the clauses. Measured separately, hand-written, nothing else changed:

| | ns/op (min of 5, pinned) |
|---|---|
| hand-written | 340,894 |
| **our shape — both halves** | **470,461** |
| post hoisted only (`for i := 2; ; i = i + 1`) | 347,937 |
| condition hoisted only (`for i := 2; i*i < n;`) | 352,193 |

**Either one alone recovers the whole cost.** Go needs only one of the two to recognise a counted
loop; with both in the body it sees an infinite loop with several exits and several back edges.

So the fix is the cheaper one: hoist the increment. The condition hoist is not built, and does not
need to be.

---

## 2. What is hoisted, and the condition that makes it sound

A loop variable moves into the post clause when **every `again` passes the same term for it**, and
that term **reads no other loop variable**.

The second condition is the soundness one and it is easy to miss. `again`'s arguments are evaluated
*simultaneously*, with every variable's old value. A post clause runs *after* the body. So:

```lisp
(again (go.+ i j) (go.+ j 1))
```

`j`'s update reads only `j` and hoists. `i`'s update reads `j`, which the body has already
assigned by then — hoisting it would read the new value. It stays in the body.
`TestAnUpdateReadingAnotherVariableIsNotHoisted` pins both halves.

The sieve's outer loop qualifies: `i` gets `(go.+ i 1)` in both `again`s. Its buffer `c` does not —
one clause passes it through and the other passes the result of the inner loop — so `c` stays an
in-body assignment, which is exactly the hand-written shape.

---

## 3. The bug, which shipped wrong answers and still compiled

The first version patched each backend's `emitAgain` separately. **JavaScript's routes through the
shared `changedArgs` instead**, so the skip never applied there and the increment was emitted
*twice* — once in the post clause and once in the body:

```js
for (;; i = (i + 1)) {
	…
	if (c2[i]) {
		i = (i + 1);      // and again here
		continue;
	}
```

The sieve advanced `i` by two per iteration and got **1984 of 2000 answers wrong**. It compiled, it
ran, and it returned a number.

Found by running the emitted code against a reference, not by reading it. The fix put the skip in
`changedArgs` — the one place all three backends share — and
`TestTheHoistedUpdateAppearsExactlyOnce` checks the count on each.

Patching three backends by hand is how one of them ends up different. That is the second time in
two days: `match` found the JavaScript backend alone had never seeded its fresh-name set from the
parameters.

---

## 4. Per target, and one divergence that is not actionable

**Go: 1.4x recovered.** 322,877 against hand-written 315,973 — 1.02x, and the whole distribution
overlaps (317–322 against 322–328).

**`dot` is unchanged**, and this matters: it was already 1.00x with the old shape, its loop has a
single back edge, and after the change its machine code is still **byte-identical** to the
hand-rolled version. 472.9 ns against 473.2. So the hoist is not a general speed-up — it removes a
cost that only appears when a loop has several back edges.

**JavaScript: no benefit, and possibly a small cost.** Minimum of nine timed rounds, three
repetitions, one process each:

| | µs |
|---|---|
| hand-written | 870 |
| emitted, **without** the hoist | 846 |
| emitted, with the hoist | 871 |

V8 was already at 0.97x without it and is at 1.00x with it. **That 3% is far inside this machine's
~15% noise floor**, which this project does not decide on, so no per-target knob was added — the
difference is recorded rather than acted on. If JavaScript ever shows a real loop regression, this
is the first place to look.

**Java: correct, unmeasured.** The emitted sieve compiles under `javac` and agrees with a
hand-written reference over 2000 sizes. There is no Java sieve benchmark, and the host most likely
to disagree with the other two remains the one with the least evidence.

**x86-64: not applied, deliberately.** `PostVars` exists to make Go, V8 and C2 recognise a counted
loop. On x86 the labels and the back jump *are* the emitted code and there is no host compiler to
convince, so `changedArgs` is called with a nil post set and the comment says why.

---

## 5. Method

Correctness was checked by running the emitted code on all three hosts against a hand-written
reference — 2000–3000 sizes each, plus n = 200000 — because §3's bug was invisible in the output
and in the type checker.

Residuals are untouched by construction: this is an emission change and nothing in `core/` moved.
