# Assessment: the round the plan ran out

2026-09-09. Deliberately **not** an ADR — writing "the direction is correct" as a decision would
recreate the predecessor's failure. The previous four are
[2026-09-06](assessment-2026-09-06.md), [2026-08-20](assessment-2026-08-20.md),
[2026-08-19](assessment-2026-08-19.md) and [2026-08-13](assessment-2026-08-13.md).

It is written now for one reason and not the calendar: **all five items of the last assessment are
done**, so the document that says what this project should do next has been describing finished work.

> **Short answer: yes on all three, and the criticism that has led three assessments has moved for
> the first time.** The corpus grew **275 lines against 3,169 of compiler — 11.5 to 1 at the margin,
> against 41 to 1 three days ago.** Two tools now exist and between them they found **eight bugs,
> four in the compiler and two silent wrong answers**, which is the answer to *"the corpus is not
> evidence"* that three assessments asked for and none had.
>
> **And one thing got worse, exactly where the last assessment said to watch.** `emit : core` went
> **3.62 → 4.00**, the analysis layer is **5,175 lines**, and this round added three more facts to
> it. That is the risk that has now survived four assessments.
>
> **One process failure, mine, yesterday**: the product shipped **without a spec**, against this
> repository's own first rule for adding to the language. **Written today**, and it paid at once —
> three shapes that had been accepted are refused by name, one of them emitting a Go type that does
> not exist.
>
> **And the next item is one question rather than four.** hamza: *"the fact that we can't express
> the entire go api, and the entire windows api, tells us we are still short."* File I/O on two more
> hosts is an INSTANCE of that, not a task beside it — §5.

---

## 1. What the last assessment asked for, scored

| | asked | happened |
|---|---|---|
| 1 | The backend is chosen by the flag string | **Done** — and it had two live instances, plus a worse bug behind it: the emitter was not a function of its input |
| 2 | The two format changes the surveys located | **Done** — Go's callable standard library 70.0%/19.0% → 88.0%/32.0%, and a program opens a file |
| 3 | ADR 0020 — uniqueness on parameters | **Done**, and then **corrected by a later build**: 4.5× became 1.84× when the element inference learned arithmetic |
| 4 | Write an application | **Done twice** — jsonfmt (95 lines) and freq (155), the two largest programs in the language |
| 5 | windows can print a string and cannot construct one | **Done** — and the gap turned out not to be the strings |

**Five of five.** The item that had been named in three consecutive assessments and acted on in none
was acted on twice.

## 2. What this round established

Eight results in three days —
[jsonfmt](../gauntlet/results/jsonfmt-2026-09-07.md),
[layers](../gauntlet/results/layers-2026-09-07.md),
[uniqueness](../gauntlet/results/uniqueness-2026-09-07.md),
[winstrings](../gauntlet/results/winstrings-2026-09-07.md),
[gauntlet](../gauntlet/results/gauntlet-2026-09-07.md),
[freq](../gauntlet/results/freq-2026-09-08.md),
[win32](../gauntlet/results/win32-2026-09-08.md),
[product](../gauntlet/results/product-2026-09-09.md).

**The corpus criticism has an answer for the first time.** Two tools, 250 lines of Oroboros between
them, and they found **eight bugs**: ADR 0019 vacuous inside a multi-result continuation, a
bounds-check narrowing that truncated its own source, a copied byte that could never be narrowed, a
Java local declared at the element width, a computed byte that could not reach a byte buffer, a
`let` in a loop guard emitting dead code Go refuses, and two in the programs themselves. **Four are
compiler bugs and two were silent wrong answers.** That is what *"write something awkward"* was
for, and it is the first time the argument has been made with anything but assertion.

**A CORE DEFECT WAS FOUND AND FIXED, and it was the language's primary data structure.** A table
store has three operands and the x86 scratch pool has two, so a declared prim could not always store
into a table — three programs were refused, and the third's refusal was not even in the program but
in `print-int`, unable to make a call that was always legal. A template with more spilled operands
than scratch now borrows a **callee-saved** value register through a **frame slot**, and `big-divmod`
and `merge-sort` run on windows.

**The oldest unbuilt item in the language's own plan is built.** type-algebra.md §8 listed four
things in order; sums, `match` and case-of-case were built and the product was not. It is now, and
the shape is the one tables.md §5.3 had written down years earlier without acting on: **a
statically-indexed heterogeneous `(array x y)` is a pair**. Flattening it is **currying**, no
backend changed, and rewriting `freq.oro`'s two hand-strided tables took 16 strided index
expressions to 2 and made the emitted Go **10% smaller**.

**Both surveys corrected themselves again, for the third and fourth time.** Win32 went
72.6%/33.5% → **76.1%/34.8%** and 7,813 → **8,809 generated primitives**, and neither number moved
because the language changed: 409 `void`-returning functions were declarable all along, and 665 more
were counted callable on the strength of a NULL **the language could not write**. One line of target
data. The pattern is now worth more than any instance of it:

> When a measurement of what a language can do is written by the same hands as the language, the
> first number describes the measurer.

**And the method turned on its own result.** ADR 0020's 4.5× was re-measured at 1.84× the next day,
because a compiler change narrowed the workspace and the two forms stopped being comparable. The
unconditional half — 0 allocations against 1 — did not move, and the ratio moved exactly the way
that result's own §1.1 predicted it would.

## 3. Are we on the right track

### 3.1 The balance of effort: the trend REVERSED, and this is the headline

| | 2026-08-20 | 2026-09-06 | 2026-09-09 |
|---|---:|---:|---:|
| compiler (with tests) | 13,098 | 32,709 | 35,878 |
| Oroboros written | 572 | 1,049 | **1,324** |
| **at the margin** | — | 41 : 1 | **11.5 : 1** |
| overall | 22.9 : 1 | 31.2 : 1 | **27.1 : 1** |
| `emit` : `core` | 2.65 : 1 | 3.62 : 1 | **4.00 : 1** |
| largest program | 112 | 112 | **155** |

**3,169 lines of compiler for 275 of Oroboros.** A factor of 3.6 better than the previous round, and
the first time either ratio has improved. The last assessment said *"it was watched and it got
worse"*; it was watched again and it did not.

**The honest qualification**: some of the 275 is the product's own demonstration, so the corpus grew
partly *because* the compiler did. And `emit : core` is the one column that still moves the wrong
way — §3.3.

### 3.2 The thesis and the core: yes, and both are now measured on two ecosystems

Go **88.0% declarable, 32.0% usable**; Win32 **76.1% and 34.8%**, with 8,809 primitives that build
under MASM and run, and two acceptance programs kept in the tree so the percentage cannot drift back
into being a claim.

The core did not grow to accommodate any of it. **Seven term kinds, unchanged.** The product — the
largest addition since `loop` by type-algebra.md's own reckoning — added **one type former and one
lowering pass**, and no backend learned it exists.

### 3.3 The analysis layer: the risk that has survived four assessments

`emit/interval.go` is **3,070 lines**; with `refine.go`, `linear.go` and `monotone.go` the analysis
layer is **5,175** — larger than `core/` twice over. This round added three more facts to it: a
quotient as a linear atom, `k·(x/k) ≤ x` as a declared axiom, and one Fourier–Motzkin step.

Each of the three is small, sound, and **verified to be load-bearing** — the stride proof fails if
any one is removed — and all three were provability-only, with 70 of 70 emitted files unchanged. So
the additions were disciplined. **The concentration is still the problem**: the part of the compiler
that decides whether a program is legal is the part hardest to check, and it is where every round
adds.

The counterweight is real and should be stated with it: the containment harness generates 1,877
programs a run and had to be made to fail before it was trusted, and every rule added this round
came with a test that fails against its absence.

### 3.4 Process: one failure, mine

**The product shipped without a spec.** CLAUDE.md's first rule for adding to the language is
*"Nothing goes in without a specification saying how it behaves on every target. String literals
were added without one and docs/spec/strings.md is the correction — write the spec first."* Every
prior addition has one — `sums.md`, `match.md`, `values.md`, `maps.md`, `tables.md`. The product has
research ([products.md](products.md)) and a measurement
([product-2026-09-09](../gauntlet/results/product-2026-09-09.md)), and **neither is a spec**: neither
says what a product means independently of a target, what each target does with it, or whether they
agree.

They happen to agree — the differential case runs on four targets — but that is evidence, not a
specification, and the rule exists because `split-words` passed every mechanical check for two
months while returning different answers.

**WRITTEN THE SAME DAY, AND IT PAID BEFORE IT WAS FINISHED** —
[docs/spec/products.md](spec/products.md). Answering *"where may a product appear"* found three
shapes that had been silently accepted: a bare product **parameter**, a product **result**, and a
map whose value is a product — and that last one emitted `map[int]/*prod(int, int)?*/`, **a Go type
that does not exist**, which a host typing nothing would have taken without a word. All three are
refused by name now, with a message saying what to write instead, and each refusal is pinned by a
test that fails without it.

So the rule is not ceremony. Two of those three would have compiled on some target and one would
have compiled on none, and **the thing that found them was being made to say what each target does
with it.**

**Everything else is clear.** The gauntlet was benchmarked two days ago, all seven at parity, and
G5's portable-layer debt is closed at 0.99×; that debt was also **corrected** — it was one program,
not the three the previous result claimed.

## 4. The risks, ranked

**1. The analysis layer, for the fourth assessment running.** 5,175 lines deciding what is legal,
growing every round, and the least checkable part of the compiler. Nothing here proposes stopping —
each addition has paid — but four consecutive assessments naming a trend that continues is a fact
about the process, not about the code.

**2. THE APPLICATIONS ARE SINGLE-HOST AND THE KERNELS ARE PORTABLE, which is backwards.** All three
programs with input, output and error handling — `wc`, `jsonfmt`, `freq` — run on Go alone, and the
reason is one missing target file: `targets/go/os.oro` is the only file-I/O binding that exists. The
thesis is that *portability is a property a program may or may not have, computed and reported*; the
only programs anyone would call programs currently have it set to "no" for a reason that is not a
language limit. This is new to this list and it is the sharpest thing wrong.

**3. The product has no spec.** §3.4. **Fixed 2026-09-09** —
[docs/spec/products.md](spec/products.md), and writing it found three holes that had been accepted:
a bare product parameter, a product result, and a map of products, the last emitting
`map[int]/*prod(int, int)?*/` — **a Go type that does not exist**, which a host typing nothing would
have taken silently. All three are refused by name now. That is the rule earning its place in one
afternoon.

**4. Struct by value, and it is smaller than 13.9%.** The largest remaining Win32 refusal, but
measured: **675 of 1,610 are enums the tool misreads**, and of the rest the calling convention is
never the obstacle — an aggregate of ≤8 bytes is a register and a wider one is a pointer. What is
missing is *construction*, which splits into one blocked decision (composing a packed word needs
bitwise, refused on V8's int32 coercion) and one genuine design question (a named layout).

## 5. What is next

**Ordered by what the risks above say, not by what is interesting.**

**1. ~~A spec for the product.~~ DONE, 2026-09-09** — [spec/products.md](spec/products.md), and it
paid immediately: three shapes that had been accepted are refused by name, one of them emitting a Go
type that does not exist. Writing the spec is what found them, which is the argument for the rule.

**2. THE ECOSYSTEM, AND IT IS ONE ITEM RATHER THAN FOUR.** This list first had *file I/O on
JavaScript and Java* here, as two target files. hamza corrected the framing and the correction is
the more valuable half:

> *"I think the I/O on js and java is the same problem of supporting the api — we have yet to
> support it on go and windows, it is going to be the same question to ask about js and java: can we
> or not? And I think this is the direction we push towards. It will inform the language design
> (express everything) and the compiler (translate). The fact that we can't express the entire go
> api, and the entire windows api, tells us we are still short."*

That is right, and it reorganises this list. **File I/O on two more hosts is an INSTANCE of the
question, not a task beside it**, and the question is: *can this language express a host's whole
API, and where it cannot, is that the format, the compiler, or the language?* The surveys already
answer it as a number — **Go 88.0% declarable, Win32 76.1%** — so what is short is measured rather
than felt, and every point of the residue is a specific thing to name.

**The residue is the roadmap, and it is already itemised.** On Go: *cannot build the argument* at
43.6%, which is interfaces and structs. On Win32: struct by value 13.9% (of which 675 names are the
tool misreading enums), unresolved typedefs 6.9%, function pointers 2.7%. Each has been priced;
none has been attacked since the two format changes.

**And it is the right direction for the reason the quote gives**: it pushes on both halves at once.
*Express everything* is the language question — what a `prim` can say, which is where several
results and a declared result range came from, and where a layout would come from next. *Translate*
is the compiler's — and this week's product is exactly that shape: one type former, one lowering
pass, no backend.

So the item is **close the residue, host by host, and let what it refuses name the next language
question.** File I/O on JavaScript and Java is where to start, because it is small, it is the same
question, and it makes three programs portable — and because it walks straight into Java's `Path`,
which is the *cannot build the argument* wall arriving as a specific thing to construct rather than
as a percentage. It also forces one design question this project has not had to answer: **how does a
target declare a host call that fails by THROWING?** `(T, error)` is Go's idiom alone.

**3. AoS against SoA, per host.** [products.md §11](products.md) item 3, and the only measurement
that could show part of what was built this week to be worth nothing. If every host prefers the flat
layout, the compiler's freedom to choose buys nothing and the record is legibility alone — which is
a result worth having, and a withdrawal this project has made before.

**4. The Win32 enum classification.** 675 names, the same shape as the `void` fix, and it finishes a
survey that has now corrected itself four times.

### Deliberately not next

**The heterogeneous product and its layout.** It is what `GUID` needs and nothing in this repository
wants it. Building ahead of demand is how the `values` reversion happened.

**The termination gap.** Real, isolated to the geometric step, and every level-walk will hit it —
but it is analysis work, and risk 1 says that is the direction to be most careful about, not least.

**Another survey.** Two was a finding; four self-corrections is a habit worth stopping.

**More gauntlet programs.** Seven is enough, they are at parity, and the corpus that needs growing is
the applications.

## 6. The one sentence

The method is right, the core is right, the thesis is measured on two ecosystems — and for the first
time the project spent a round making more language than compiler, which is what the last three
assessments asked for; the next round should be judged by **how much of a host's API this language
can express**, because the residue is the honest measure of what is missing, every point of it names
a specific thing, and the fact that no application yet runs on two hosts is the smallest instance of
exactly that gap.
