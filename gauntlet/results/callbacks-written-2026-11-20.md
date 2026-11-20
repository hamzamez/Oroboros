# callbacks-written — a cell a λ writes, on every target, and a definition as a host (ADR 0143, 0144), 2026-11-20

hamza, 2026-11-20: "no program asks for it is no excuse. You come with a program. rewrite api doc
examples, famous open source software, famous algorithms. you just do it. on javascript, the jvm
and windows, you declare something that use it." And on our own definitions: "why?"

callbacks-2026-11-20 built the written cell on Go only, refusing it on the other three targets,
which declared no host whose calls have an order. This step declares one on each, from the host's
own documents, writes a program on each whose answer depends on the order, and builds the printers.
It also answers hamza's "why?" with ADR 0144: a definition of the program's is the strongest host.

## The programs

Each checks a law whose truth depends on the order of the λ's applications, against a loop of the
program's own.
- **`examples/callbacks/dyck.oro`, JavaScript, `Array.prototype.forEach`: balanced parentheses.** A
  word is a Dyck word iff its path ends at 0 and never goes below it. The minimum of the prefix sums
  depends on the order: ")(" and "()" have the same steps. Two cells, the path and its minimum. Of
  the 256 words of length 8, **14** are balanced (Catalan's C₄), and **256** agree with the loop.
- **`examples/callbacks/kadane.oro`, the JVM, `LongStream.forEachOrdered`: the maximum subarray**
  (Kadane, in Bentley's Programming Pearls). The recurrence c' = max(0, c + x) is not commutative.
  Bentley's example {−2, 1, −3, 4, −1, 2, 1, −5, 4} gives **6**, and **100** of 100 tables agree
  with the quadratic search over every (i, j).
- **`examples/callbacks/once.oro`, windows on both triples, `InitOnceExecuteOnce`: one-time
  initialization is idempotent.** On success e ∘ e = e, and a failure leaves the block fresh. Over
  two calls f runs once when it succeeds and twice when it fails: **1 1 1 2 0 0**.
- **`examples/callbacks/horner.oro`, every target, a definition `each`: Horner's rule** against the
  sum of powers at t = 0 … 9, and `twice` counting its applications: **89 10 7 2** on Go,
  JavaScript, the JVM and both windows triples. The cell becomes a variable of `each`'s own loop.

## The evidence

`gauntlet/stdlib/callbacks-hosts-evidence.txt`, checked by `TestEveryCallbackMarkerIsItsHostsSentence`
both ways (every `ordered` and every `calls K` other than `0..` has rows; every row names a marker
declared):
- **ECMA-262** (tc39.es, read with hamza's permission): forEach, some, every and findIndex each call
  their callback "once for each element present in the array, in ascending order". So all four are
  `ordered`, the three already declared included.
- **The JDK**, `src.zip` on this machine: forEachOrdered processes "each element … in encounter
  order", and a long[]'s spliterator reports ORDERED. These sentences are checked against the files.
- **Microsoft Learn**: InitOnceExecuteOnce, "only one thread will execute the callback function",
  so `calls 0..1`. LineDDA's page and LINEDDAPROC's promise each point "along the line" and no
  order, so LineDDA was not declared.

The faults: a JDK sentence the file does not contain, and a `calls 0..1` with its row removed. Both
are refused.

## What was built

| piece | what | its witness, the fault refused or caught |
|---|---|---|
| JavaScript | a reference no host receives (`ir.OwnReferences`) is a `let` the arrow closes over; its read a `const`, its write an assignment | dyck; the arrow writing a variable of its own changes the answer (`js.PlantCellWriteLocal`) |
| the JVM | a final one-element holder the lambda shares, `final long[] v = { x }`, uncopied | kadane; a clone of the holder changes the answer (`java.PlantJavaHolderCopied`) |
| LLVM | an `alloca i64` in the entry block, reached by the lifted function through E | once on both triples; the λ's store dropped changes the count (`llvm.PlantCellSetDropped`) |
| a unit result | `fn(x ∈ S) ()`, a callback whose value its host ignores, read (#result "prod()"); a void lambda returns nothing | forEach and forEachOrdered |
| a host cell and a λ's cell in one call | the λ's cells threaded first, the call inside it then the host's cell call | once: InitOnce's block is a host cell, the count a λ's |
| a λ widens as a loop | a λ's held references widen with its own thresholds (Blanchet et al. 2003), as the loop it is read as | once's count proven ≤ 100 after the calls; widened plainly it is [0, +inf], refused (`ir.PlantCallbackWidenedPlainly`) |
| ADR 0144 | a definition applied to a λ mentioning a cell expanded before the state translation, β on values | horner on five targets; unexpanded, refused as a closure (`core.PlantHigherNotExpanded`) |
| the layout test | a struct's C name spelled by σ_J's inverse, `INIT_ONCE` | INIT_ONCE's size, alignment and offset agree with clang on both triples |

## What failed first

- **A claim could not state a unit result.** `fn(x ∈ Int) ()` began no expression, so forEach's
  claim could not be written; a host statement's result is `(Any)` by convention. Java then refused
  `return 0` in a void lambda: a function whose one result is the unit returns nothing now, on the
  JVM and JavaScript.
- **InitOnceExecuteOnce's block and the λ's count in one call** reached the host-cell translation
  first, which passes a λ as it stands, so the λ's cell was read as its value: "the place written
  here, 0, is none". Translated the other way round it composes.
- **The count was [0, +inf] after the call.** The λ's fixpoint widened with no thresholds, so a cell
  written under `n < 100` lost its bound. Go's `prefix` had the same widening, hidden because its
  test's product was meant to be refused.
- **A table's elements are not the claim's domain.** forEach's λ parameter is `Int`, not the
  table's element set, so `d + x` needs a guard on x. The programs compare instead (`x == 1`), or
  guard. A claim over a table's element set (`fn(x ∈ elem(a))`) is owed.
- **The layout test wrote `INIT-ONCE`**, the core's spelling, which clang cannot parse.
- **A heredoc collapsed `\n` in the Go test** again (memory's note): fixed with a script written by
  the Write tool.

## The JVM's holder, measured

Kadane's pass over 10⁶ longs, hand-written, the three forms interleaved in one process, three runs
of 30 rounds after 50 warm-ups (`scratchpad/jvmcell/Bench.java`):

| form | median, ms | |
|---|---|---|
| final holder `long[] c = { 0 }` (printed) | 0.564–0.590 | 1.6–1.7× the loop |
| static field | 0.308–0.361 | the loop's speed |
| plain for loop | 0.328–0.361 | |

The holder loses, as expected: C2 does not scalar-replace an array the lambda captures through the
stream's pipeline. The static field is the loop's speed and wrong: an export called from two threads
shares it, the race that ruled out Go's slot. So the holder stands, and its price is a host's
traversal written as a callback where a loop of the program's own costs less.

## Paid and bought

| | |
|---|---|
| compile time | 1.05× the baseline on the corpus (`cmd/check`'s gate), no suspect confirmed by the serial pairs |
| run time | the JVM's holder 1.6–1.7× a plain loop (measured above); JavaScript's variable and LLVM's alloca not measured against a loop |
| code size | four new programs; every existing file byte-identical, `prefix`'s refusal on three targets changed only in the closure message's corrected text |
| proofs | 11,971 of 12,025 operations and 1,393 of 1,686 loops, from 11,859 of 11,913 and 1,367 of 1,660: the four programs' 112 operations and 26 loops, all proven; windows32 6,720 of 6,730 and 747 of 826, from 6,685 of 6,695 and 741 of 820 |
| refusals changed | a cell written in a λ under an `ordered` host is accepted on JavaScript, the JVM and windows, where it was refused by its printer; a λ given to a definition of the program's that writes a cell, refused as a closure, is expanded |
| compiler lines | `core/cell.go` +120 (`expandHigher`, `betaLams`, the cells-first rewrite), `core/jreadtarget.go` +15, `ir/interval.go` +30, `ir/js/js.go` +45, `ir/java/java.go` +50, `ir/llvm/llvm.go` +30, `core/reduce.go`, `ir/llvm/fn.go` a line each; tests +180 |
| language lines | `targets/win32/synchapi.oro` new (30), `targets/js/methods.oro` +12, `targets/java/util.oro` +12; four programs, 260 lines; the evidence file, 8 rows |
| documents | ADR 0144; the book's §14.2 (`twice` given a λ that writes a cell); local.md §3's refusal list; the inventory's `callback-set`; formers-research.md |
| survey | Win32 callable 4,480 (36.2%), linking 3,844 (31.1%), every struct declared 5,887 (47.6%), from 4,468, 3,832 and 5,878: a record the target declares now counts under any of its typedef names, and `__callback` is read as an annotation (TestTheSurveysModelCallsWhatTheTargetDeclares refused InitOnceExecuteOnce as not callable) |
| rule made less general | none |
| not built | a buffer written in the λ (T1-4), below; the slot's restore still without a witness, since no slot API is `ordered` |

## Not built: a buffer written in the λ (T1-4)

Counting sort's histogram through forEach, `count[x] := count[x] + 1`, is refused by R3 ("the
buffer count is mentioned by a function"). T1-4 is decided (lent like a cell), and the derivation
says how. The state is a table, λ_S : B × A → B × R, threaded as the cell is. Realized in place,
each store returns its own argument, so **the threaded value is one address throughout**: the
reference is constant, and the λ may capture the buffer and store into it. What is owed is the
IR's part, more than the translation's. A store `(set h i v)` is lowered only inside a build region,
where its table is the region's own ("a table-set is not a host call"), so a table read out of a
reference cannot be stored into. The design the derivation gives:
- **a store and a read through the reference**, `#ref-set(r, i, v)` and `#ref-index(r, i)`, which S
  writes for `b[i] := v` and `b[i]` inside the λ. They are in place by construction, since r holds
  the address, and they carry ADR 0119's obligation 0 ≤ i < len, len read through r;
- printed as Go's `(*r)[i] = v`, JavaScript's `r[i] = v`, the JVM's holder `r[0][i] = v` and LLVM's
  load of the table's address then a store;
- `ownReferences` counting the two as uses, not escapes, so the table's facts after the call are
  its element set's alone (no content fact survives a call that stores);
- the table's release at the call, as a captured table's is (`PlantFnReleasedEarly`).

The program is counting sort (Seward 1954), the histogram by forEach on JavaScript, forEachOrdered on
the JVM and a definition on every target. It is next.
