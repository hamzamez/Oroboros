# Research: concurrency, and letting it crash

2026-10-04. **Research, not a decision.** This is the one question
[design-direction.md §8](design-direction.md) left open (item 3), and the second of the 10-03 round.
hamza's framing: the Erlang model, Joe Armstrong's "let it crash", fault tolerance, approached through
the literature, the mathematics and candidates. It follows ADR 0007: each candidate is named with the
measurement or program that would kill it. One measurement is taken
([conc-ring-2026-10-04](../gauntlet/results/conc-ring-2026-10-04.md)); the rest are proposed in §6.

The short version:
- an Erlang process is a **Mealy machine**;
- a crash is the **exception effect**, a link is its propagation, and a supervisor is its **handler**,
  with a restart policy;
- in a language with no recursion, **a process that receives anywhere compiles to a step function**,
  with no stack;
- because values are immutable and buffers linear, **isolation is a typing property**, not a heap
  layout;
- determinacy, like portability, can be **computed and reported**.

---

## 0. The question, and what is already fixed

The question is which model of concurrency the language should have, and what "let it crash" means in
it. These are already decided, and any candidate must satisfy them:

| constraint | source | what it forces on concurrency |
|---|---|---|
| a construct promoted to the language works on every target | CLAUDE.md, the `values` precedent | Go, JavaScript, Java **and x86** each implement it. x86 has no runtime, and JavaScript has one thread per agent |
| values are immutable; the one mutable thing is a scoped linear buffer | ADR 0018, ADR 0020 | nothing to race on. A buffer can only be *moved* between processes |
| a closure may not survive staging | closures-direction.md; callbacks.md tiers 1–3 | no `spawn(fun)` with a manufactured closure. A process is named, or written at the spawn site (tier 1), and captures no buffer (callbacks.md §4) |
| no recursion | ADR 0014 | Erlang's server loop, a tail call, is a `loop` |
| effects are a side condition on β | ADR 0010 | `send`, `receive` and `spawn` are impure primitives: let-bound, never substituted, never folded |
| staging never changes an answer | ADR 0009 | nothing concurrent happens at compile time |
| no boxing or hidden allocation; every map has a declared capacity | design-direction §2, maps.md | **an unbounded mailbox is hidden allocation**. A mailbox has a declared capacity |
| bounded by default: what is not proven is refused, or trapped by request | ADR 0019, ADR 0026 | "let it crash" must be reconciled with "refuse what is not proven" (§3) |
| structured control only | CLAUDE.md | no `goto`, and no unstructured `go` statement either (§2.4, structured concurrency) |
| legality and portability are per (program, target), computed and reported | ADR 0026 | determinacy can be treated the same way (§2.6) |

The applications the language is for are event-driven on every platform (general-purpose.md §2.5): a
Windows message loop, Android's main thread, the browser's event loop, a backend's requests.

---

## 1. What Erlang is

Erlang's model has five parts (Armstrong 2003, ch. 2–4; Armstrong, HOPL 2007):

1. **Processes.** Each process is isolated, with its own heap. They are cheap enough to create one per
   activity: 2.7 KB idle on OTP 29, measured.
2. **Asynchronous send.** `Pid ! Msg` never blocks. Delivery between any one pair of processes is
   FIFO; there is no order across pairs (Svensson, Fredlund and Earle 2010).
3. **A mailbox and selective receive.** `receive P₁ -> … ; Pₙ -> … after T -> … end` takes the first
   message in arrival order matching some pattern, and leaves the others in order. An `after` clause
   fires if nothing matches within T.
4. **Links, monitors and exit signals.** A link is symmetric: when either process dies, the other gets
   an exit signal and dies too, unless it traps exits, in which case the signal arrives as a message.
   A monitor is one-way and always delivers a message.
5. **OTP behaviours.** The generic part of a server, a state machine or a supervisor is a library, and
   the programmer writes only callbacks. `gen_server`'s `handle_call(Req, From, State) -> {reply, R,
   State′}` is the whole server. A **supervisor** is a process that starts children, traps their
   exits, and restarts them by a strategy:
   - `one_for_one`: restart the child that died;
   - `one_for_all`: restart them all;
   - `rest_for_one`: restart it and the children started after it.

   It is bounded by an *intensity*: at most R restarts in T seconds (OTP's default is 1 in 5),
   beyond which the supervisor itself exits and its own supervisor decides.

**"Let it crash"** is the claim of Armstrong's thesis, ch. 4:
- error recovery belongs to *another* process;
- a process that meets an error it was not written to handle should fail fast, not limp on;
- the code that does the work and the code that recovers from its failure should be separate. The
  thesis names this "the error kernel": a small part of the system that must be correct, and
  everything above it allowed to fail.

Its premise is Gray's (1985): most failures in production software are **transient**. Restarting
from a known state clears them, and Tandem's process pairs masked most software faults that way.
Candea and Fox (2003) made it a design rule, *crash-only software*: the only way to stop a component
is to crash it, and the only way to start it is recovery.

Two parts of Erlang are not candidates here:
- **hot code loading**, which needs code as a runtime value (a staged language has none);
- **transparent distribution**. Location transparency hides partial failure, which is the property a
  distributed program must handle (Waldo et al. 1994).

Both are named in §8.

---

## 2. The algebra

### 2.1 A process is a coalgebra

Let I be the messages a process accepts, O what it emits (sends, replies, host calls) and S its state.
A deterministic process is a **Mealy machine** (S, s₀, δ) with

  δ : S × I → O* × S,

that is, a coalgebra S → (O* × S)^I of the functor F(X) = (O* × X)^I (Rutten 2000). Its behaviour is
the causal function I^ω → (O*)^ω it computes from s₀. **Two processes are equal when they are
bisimilar**, meaning they have the same behaviour, however differently their states are spelled. The
final coalgebra exists and is the set of causal stream functions, so equality is behavioural, which
is what a refactoring must preserve.

This is exactly OTP's `gen_server`: `init : 1 → S` and `handle_call : I × S → O × S`. It is also:
- Akka Typed's `Behavior[T]`, a function from a message to the next behaviour;
- the Elm architecture's `update : Msg → Model → (Model, Cmd Msg)`;
- a **Windows window procedure**: `WndProc(hwnd, msg, wParam, lParam)`, whose state lives in the
  window. `PostMessage` is `gen_server:cast` and `SendMessage` is `gen_server:call`.

The Windows application, item 3 of the plan, is a Mealy machine before it is anything else.

**The step δ is a first-order function of data**, which the language's dynamic level already is
(closures-direction.md). A process needs no new kind of term. It needs a state type, a message type
(a closed variant, sums.md) and a step.

### 2.2 Receiving anywhere is a Mealy machine: the state-machine translation

Erlang lets `receive` appear anywhere in a process's code, deep inside its loops. Does the language
need stackful coroutines for that? JavaScript has none, and x86 would have to switch stacks itself.

**Claim.** Let P be a process body in the residual: first-order, recursion-free, with structured loops
(ADR 0014, ADR 0015), in which `receive` is an operation recv : 1 → I, an algebraic effect (Plotkin
and Power 2003). Then P is a Mealy machine whose state set is

  S = Σ_{r ∈ R} Π_{v ∈ live(r)} ⟦τ(v)⟧,

where R is the finite set of receive points and live(r) the SSA values live across r. Its step
resumes at the point the state names, with that point's live values bound, and runs to the next
receive point.

*Why.* At a suspension, the configuration is the control point plus the environment, and only the
live values of the environment can affect what follows. With no recursion and every call inlined
(reduction inlines every non-exported definition), control is a finite graph with no call stack. The
continuations at the receive points are therefore a finite family of closures over live values.
Defunctionalizing them (Reynolds 1972) gives a variant: one constructor per receive point, carrying
that point's live values. **That variant is closed, finite and non-recursive, which is exactly what
the language's variants already are** (sums.md). The IR already computes exact live sets, for x86's
register colouring (spec/ir.md §9.6). Conversely, every Mealy machine is a loop with one receive at
its head. So the two forms are interconvertible by a compile-time translation, and the choice
between them is a surface question, not a semantic one.

This is how C# and Rust compile `async` functions into state machines, how Protothreads (Dunkels et al.
2006) run stackless on sensors, and how Esterel compiles to automata (Berry and Gonthier 1992). What
those systems must restrict (no recursion through a suspension point, which is why Rust's recursive
`async fn` needs boxing), this language has as a law. **Two consequences:**

- **No function colouring.** Nystrom's "what colour is your function?" is a problem of separately
  compiled functions: a caller must know whether a callee suspends. Inside a program everything is
  inlined, so a `receive` in a helper is a receive in the process. Only an export's signature would
  carry the colour.
- **A process costs its live set, not a stack.** Measured
  ([conc-ring](../gauntlet/results/conc-ring-2026-10-04.md)): a stackless process is 76–152 bytes,
  against 9.3 KB for an idle goroutine and 2.7 KB for a BEAM process. A hop through the stackless
  ring takes 5.7 ns on Go, against 275 ns for a goroutine and 428 ns for BEAM.

### 2.3 Composition: three answers, and the line between them

How processes compose is where the classical models differ:

| model | communication | algebra | determinacy |
|---|---|---|---|
| **CSP** (Hoare 1978, 1985) | synchronous rendezvous on named channels; external choice □ | process algebra with laws (□ commutative, associative, idempotent; ‖ by synchronisation sets); the failures–divergences model (Brookes, Hoare and Roscoe 1984), checked by refinement (FDR) | nondeterministic by □ |
| **actors** (Hewitt et al. 1973; Agha 1986; Agha et al. 1997) | asynchronous send to an address; unbounded buffers; fairness (every message is eventually delivered) | configurations and their transitions; observational equivalence | nondeterministic: arrival order |
| **Kahn process networks** (Kahn 1974; Kahn and MacQueen 1977) | FIFO channels, one writer and one reader each; a read blocks; no test for emptiness | each process is a **continuous function on streams**, ordered by prefix; a network is the **least fixed point** of its equations (Kleene) | **determinate**: the output is a function of the input, whatever the schedule |

The line between the last two is exact. **An Erlang mailbox is a fair merge of every sender's
stream.** Merge is not a function of its input histories, and Brock and Ackerman (1981) showed that
no semantics of histories is compositional once a nondeterministic merge is admitted. So:

- a network in which **every mailbox has one sender**, receives block, and there is no timeout is a
  Kahn network, hence determinate. Selective receive on a single-sender mailbox is still a function
  of the stream;
- a mailbox with two senders, an `after`, or a choice among channels makes the network
  nondeterminate.

Go's `select` is CSP's external choice. Erlang's mailbox is the actors' merge. Go's channels with one
sender and one receiver are Kahn's.

### 2.4 Failure is an effect, a link propagates it, a supervisor handles it

Give a step the exception monad: δ : S × I → (O* × S) + E. A crash is crash : E → 0, an operation with
no continuation, as `exit` already is. Then:

- **A link** is failure propagation as a relation: if p and q are linked and p's step returns inr e, q
  receives exit(p, e), which kills q unless q traps exits, in which case it is a message in q's
  I. A **monitor** is the one-way, always-a-message version.
- **A supervisor** is a *handler* for E in the sense of Plotkin and Pretnar (2009), over a vector of
  children (c₁ … cₙ). Each child is a Mealy machine with an initial state. On inr e from cᵢ:
  - `one_for_one` resets {i} to its initial state;
  - `one_for_all` resets every child;
  - `rest_for_one` resets {j | j ≥ i}.

  A restart is the re-initialisation of a coalgebra: the child's state is discarded and s₀ is put
  back. The message that caused the crash was consumed, so it is not delivered again.
- **Intensity is the termination argument of escalation.** If a supervisor exceeds R restarts in a
  window, it exits with e′ and its parent handles that. The supervision tree is finite, of depth d,
  so escalation from any leaf reaches the root in at most d steps. A supervisor that exhausts its
  intensity at the root stops the application. It is a well-founded descent on depth, the same shape
  as the size-change argument for loops (ir/sct.go), so restart storms are bounded per window.
- **Structured concurrency is the degenerate supervisor.** A nursery (Sústrik 2016; Smith 2018, "Go
  statement considered harmful"; Java's JEP 453) is a supervisor whose children it waits for, with
  strategy `one_for_all` and intensity 0. The first failure cancels the siblings and propagates. **Fork–join**
  over pure children (Cilk: Frigo, Leiserson and Randall 1998; scheduled by work stealing, Blumofe and
  Leiserson 1999) is a nursery whose children return values, joined as a **product**.

So supervision trees, nurseries and fork–join are one construct: **a tree of lifetimes with a policy
on failure**. Erlang's `spawn` without a link is the unstructured `go` statement. `spawn_link` under
a supervisor restores the structure. "Structured control only" admits the tree and refuses the bare
spawn.

### 2.5 Isolation is a typing property here

Erlang isolates processes by giving each its own heap and copying every message between heaps. A
crash then cannot leave another process's state inconsistent, because no state is shared. Pony
(Clebsch et al. 2015) gets the same guarantee on a shared heap from *reference capabilities*: `val`
(immutable) and `iso` (uniquely owned) are sendable, and nothing else is.

**This language already has both kinds, and only these:** every value is immutable (`val`), and the
one mutable thing, a buffer, is linear (`iso`) (ADR 0018, ADR 0020; memory-model.md §2.7, §9). So:

- a message of immutable values can be **shared by reference**, with no copy;
- a buffer can be sent only by **move**. `send` consumes it, as `set` consumes and returns one, and
  linearity is checked on the residual as it is now. JavaScript's `Transferable` (`postMessage(m,
  [buf])`, which detaches the sender's `ArrayBuffer`) is exactly a move;
- **a crash cannot corrupt another process's state**, because there is no shared mutable state to
  corrupt. Isolation is a theorem of the type discipline, not a property of the heap.

**What the type discipline does not isolate is resources.** On Go an unrecovered panic in any
goroutine ends the whole program, and some runtime faults cannot be recovered at all. On the JVM an
`OutOfMemoryError` is everyone's. On x86 nothing is isolated. Erlang shares one VM too, and limits a
runaway with a per-process heap ceiling (`max_heap_size`). So a process's crash must be caught at the
process boundary on every host: `defer`/`recover` on Go, `catch` on JavaScript and Java, and a return
to the scheduler on x86. A process's memory should be bounded the way a map's capacity is.

### 2.6 Determinacy is computed, like portability

ADR 0026 made portability a computed property of a program: every target that accepts it computes the
same integer, and which targets accept it is reported. The same move applies here:

- **A determinate program** (§2.3) has one observable behaviour. Every target must produce exactly
  it, and the differential suite can check it as it checks everything now.
- **A nondeterminate program** has a *set* of behaviours. Each target must produce a member of the
  set (trace refinement, as in CSP's refinement order), and the set is the same on every target.
  Testing it needs control of the schedule:
  - a deterministic, seeded scheduler (deterministic simulation, as FoundationDB tests);
  - systematic exploration (CHESS: Musuvathi et al. 2008);
  - PCT's randomized scheduler, which finds a bug of depth d with probability at least
    1/(n·k^(d−1)) (Burckhardt et al. 2010).

**The compiler can compute which a program is, from linearity.** A mailbox address (a pid) used
linearly has one sender, so a program whose every pid is linear, and which has no `after` or choice,
is a Kahn network, and determinate by Kahn's theorem. A shared pid is a merge. The occurrence counting
that checks buffers today (ADR 0018) is the analysis.

**Determinacy can also come from algebra.** If a receiver's step is the action of a **commutative
monoid** on its state (counting into a map, taking a maximum, a set union), its final state does not
depend on arrival order: s·m₁·m₂ = s·m₂·m₁. That is the principle of LVars (Kuper and Newton 2013)
and CRDTs (Shapiro et al. 2011). A word-frequency aggregator fed by many workers is nondeterminate in
its trace and determinate in its result.

### 2.7 Resources: a mailbox has a capacity

An unbounded mailbox is the classic way an Erlang system fails under load, and it is hidden
allocation, which the language forbids. **A mailbox declares its capacity**, as a map does. Then a
send to a full mailbox has three possible meanings:

1. **block the sender** (backpressure). A cycle of full mailboxes deadlocks. Deadlock is undecidable
   in general, and detectable at run time when every process is blocked;
2. **crash the sender** (overload is an error: let it crash, and shed load);
3. **return a status**, as a fallible host call returns `(tuple v err)` (ADR 0027).

Erlang chose none, with unbounded mailboxes. Go chose (1). Win32's `PostMessage` chooses (3): it fails
when the thread's queue (10,000 messages) is full. This is a decision for hamza (§7).

### 2.8 Time

`after T` is the only way an Erlang process can detect a peer that is slow but not dead. In an
asynchronous system, slow and dead cannot be told apart (Fischer, Lynch and Paterson 1985); failure
detectors are the theory (Chandra and Toueg 1996). **On one machine a link detects death exactly**,
so timeouts are needed only for the outside world and for a call whose server may hang. Time makes a
program nondeterminate (§2.6), and a time-based supervisor intensity does too. Intensity counted in
restarts per *messages handled* would keep a supervisor determinate. Erlang counts in seconds.

---

## 3. Why "let it crash" fits a language that proves

The language refuses what it cannot prove (ADR 0019): an index out of range, an overflow, an
unsatisfied contract. "Let it crash" sounds like the opposite: do not handle the error, die. **They
divide the failures between them, by Gray's taxonomy.**

- **Bohrbugs** are deterministic: the same input fails the same way. Restarting does not help, because
  a step that crashes on (s, m) crashes again whenever s recurs. The compiler removes the largest
  classes statically: indices (refinements.md §3a), arithmetic outside the word (ADR 0026), contracts
  (ADR 0028), use-after-free and aliasing (ADR 0018). These are crashes Erlang has to recover from
  and this language does not have.
- **Heisenbugs** are the environment's: a file that vanished, a socket that reset, memory exhausted, a
  host call that failed. No proof can exclude them, because they are not properties of the program.
  Restarting from a known state is exactly the right response, and these are what Gray found most
  production failures to be.

So **prove what is a property of the program, and crash on what the environment does**. The crashes
left in a proven program are:
- the traps a program asked for (`-checked`);
- host failures a program chooses not to handle;
- explicit `crash`;
- exhausted resources.

All four are what a supervisor is for. Today a trap ends the program. That is a supervision tree of
one process with intensity 0, so "let it crash" is already here, at a granularity of one.

The second half of Armstrong's argument, separate the doing from the recovering, is what the language
does with contracts already. A `where` states what the body assumes (ADR 0028), and the caller
discharges it. A supervisor states what to do when an assumption about the *world* fails, and the
worker does not.

---

## 4. What each host provides

| | process | mailbox / channel | choice | failure isolation | link / monitor | measured cost per hop (ring, T = 1) |
|---|---|---|---|---|---|---|
| **BEAM** (reference, not a target) | process, preemptive, per-process GC | mailbox, unbounded | selective `receive` | per process | links, monitors | 428 ns |
| **Go** | goroutine, preemptive since 1.14, 8 KB stack | `chan`, bounded or synchronous | `select` (CSP □, uniform among the ready) | **none**: an unrecovered panic ends the program; `recover` in the goroutine's `defer` | none: built from `recover` and a channel | 275 ns; 180 at GOMAXPROCS = 1 |
| **JavaScript** | one thread per agent; async functions (state machines); Workers (separate heaps) | promises, a microtask queue; `postMessage` between Workers, FIFO per port, structured clone or **transfer** | `Promise.race` | per Worker; within one, `catch` | a Worker's `error` and `exit` events | 53 ns (async) |
| **JVM** | platform threads (JDK 17); virtual threads (JDK 21, JEP 444); structured concurrency (JEP 453) | `BlockingQueue`s | none built in | per thread: an uncaught exception ends only its thread | `UncaughtExceptionHandler`, `Future`s | 4,220 ns (platform thread, JDK 17) |
| **Win32 / x86** | `CreateThread`, fibers, thread pools, IOCP | a **message queue per thread**: `PostMessage` (async, fails when full) and `SendMessage` (synchronous) | `MsgWaitForMultipleObjects` | structured exception handling per thread | none | not measured: our own scheduler is the only stackless option |
| **stackless, any host** (§2.2) | a step function | a fixed ring per process | by the scheduler | a `catch` at the step boundary | the scheduler's | 5.7 ns (Go), 13 (JS), 7.0 (JVM) |

**The applications' own frameworks are already single-threaded Mealy machines.** The Windows message
loop and `WndProc`, Android's `Looper` and `Handler`, and the browser's event loop each deliver one
message at a time to a handler that updates state. "Emit at the highest layer the target natively
provides" says a UI process *is* the host's loop, with our step as its handler.

**Go is the one host where the native unit is worth its cost.** Its runtime parks a goroutine on a
blocking system call and keeps the others running, and `os.File.Read` blocks. A stackless scheduler
that calls a blocking host function stalls every process it runs. On JavaScript the question does not
arise: a host call's I/O completes as a callback, which in §2.2's terms is the *receive of its
completion message*. A host call's continuation is already a tail position (ADR 0027), which is
exactly the shape of a suspension point.

---

## 5. Candidates

Each is stated with what it would cost, how it lowers, and what would kill it.

### C1. Erlang, faithfully

`spawn`, `!`, selective `receive … after`, `link`, `monitor`, exit trapping, with supervisors as a
library written in the language.

- **Cost:** untyped messages; unbounded mailboxes; `spawn(fun)`.
- **Lowers** by §2.2 to step functions everywhere, or to goroutines on Go.
- **Killed by the constraints, not by a measurement.** Unbounded mailboxes are hidden allocation, a
  manufactured closure cannot survive staging, and the language has no dynamically typed message. C1
  adapted to the constraints is C2. Its value is as the reference semantics.

### C2. Typed processes: Mealy machines, receive as their surface, supervision trees as data

- **A process** is declared by its message type (a variant), its state, its initial state and its
  step, or written with `receive` in a loop and translated by §2.2. Both spellings mean one thing.
- **An address** has a type, `(pid M)`, and is a capability: who can send to a process is who holds
  its pid (Miller 2006).
- **`send` is asynchronous**, into a mailbox of declared capacity (§2.7, policy open).
- **A call** is a send carrying a one-shot reply capability `(reply R)`, which is **linear**: the
  server must use it exactly once. A step that drops it, or replies twice, is refused, as a buffer
  used twice is now. That is the session type !R.end (Honda 1993), and it is a promise in Liskov and
  Shrira's sense (1988). Win32's `SendMessage` and `gen_server:call` are this, without the check.
- **Failure:** `crash` of type 0; links and monitors; **a supervision tree declared as data**: its
  children, a strategy and an intensity. A nursery and fork–join are the instances of §2.4.
- **Lowering:** stackless step functions over a scheduler by default (measured: 5.7–13 ns per hop,
  76–152 B per process). A host's own loop for a UI process. Goroutines on Go where a step makes a
  blocking host call (M4). On JavaScript, an async host call becomes a receive.

Open inside C2:
- **(a) Selective receive.** Erlang's scan, or typed ports: a process owns several mailboxes, and a
  receive point names which it waits on, as Go's `select` and the join calculus do.
- **(b) Preemption.** A cooperative scheduler is starved by a step that never ends. Two answers:
  - **refuse a step whose loops are not proven to terminate.** Size-change termination proves 347 of
    365 loops today. A server's own receive loop is the one loop meant not to terminate; it is
    productive (Turner 2004), and the translation removes it;
  - **count reductions at back edges**, as BEAM does, making each back edge a suspension point the
    translation already handles. It costs a counter per iteration, to be measured.
- **(c)** The full-mailbox policy and intensity in time or in messages (§2.7, §2.8).

**What would kill C2:**
- a program whose live set at a receive point is large enough that a suspension costs more than a
  stack switch (M3);
- a program in which blocking host calls are pervasive, so that the hybrid lowering is a goroutine
  per process anyway (M4);
- the translation multiplying code size past the host's inlining budget (design-direction §8, item 9).

### C3. Kahn process networks

Typed FIFO channels with one writer and one reader, blocking reads, no choice and no timeout. A crash
ends a process's output streams with exit(e).

- **Gain:** determinacy by theorem, so the differential suite works unchanged. Lee and Messerschmitt's
  synchronous dataflow (1987) and Parks's bounded scheduling (1995) are its engineering.
- **Killed as the whole model** by any server with more than one client, whose requests are a merge.
  Kept as **C2's determinate fragment, computed and reported** (§2.6): linear pids, no `after`, no
  choice.

### C4. Fork–join over immutable tables

Parallel `table` and nurseries of pure children, joined as a product. It is deterministic,
data-race-free by ADR 0018, and `(alloc (table n f))` is already embarrassingly parallel
(memory-model.md §9).

- **It is parallelism, not concurrency:** no communication, no failure handling beyond `one_for_all`
  with intensity 0.
- **Nothing kills it.** It is C2's degenerate supervisor, and it can be built first, independently.

### C5. CSP: synchronous channels and `select`

Go's own model.
- **Lowering:** a rendezvous blocks both sides, so on JavaScript and x86 it needs §2.2's translation
  anyway.
- **Killed as the base** by failure: CSP has no propagation, so a crashed partner leaves the other
  blocked forever, which is a deadlock and not a crash. That is Smith's argument against the `go`
  statement. Go's channels remain the lowering of C2's mailboxes on Go.

### C6. Session-typed linear channels

Caires and Pfenning (2010) and Wadler (2012): session types are linear logic's propositions, and a
well-typed program cannot deadlock, by cut elimination.

- **Killed as the base** by its topology. Deadlock freedom holds for tree-shaped networks, and a
  server shared by many clients needs shared channels, with which deadlock freedom is lost again
  (Balzer and Pfenning 2017).
- **Kept as the typing of C2's reply capability.** That is the one place where a protocol's shape is
  short, fixed and worth checking.

### C7. async / await and promises

JavaScript's model, and Rust's and C#'s.
- **It is a lowering, not a model:** it gives sequencing of asynchronous calls, and no processes,
  isolation or failure handling. Function colouring does not apply inside a program (§2.2).
- **Killed as the base**; it is C2's lowering on JavaScript.

### Where this points

**C2, with C3 and C4 as its computed and degenerate fragments.** One construct (a process is a Mealy
machine), one failure algebra (crash, links, supervisors as handlers), the structure of a tree of
lifetimes, and two properties the compiler computes: determinacy and the linearity of replies. It is
the hypothesis to try to kill, not a decision. CLAUDE.md's rule applies: **a program first**.

---

## 6. Measurements and programs that decide

| | what | kills or decides | status |
|---|---|---|---|
| M1 | a process's cost per hop and per idle process, native and stackless, on each host | whether stackless is worth having; whether "processes are cheap" holds on every host | **done** ([conc-ring](../gauntlet/results/conc-ring-2026-10-04.md)): stackless is 4–600× cheaper per hop, and 76–152 B against 0.7–9.3 KB |
| M2 | **a program**: a supervised pipeline. A reader per input, workers that crash on a host failure and are restarted, and an aggregator whose step is a commutative monoid action (§2.6). Written in C2's proposed surface, with a hand-written Go, Erlang and JavaScript version as the bar | whether C2's surface writes it; whether the result is determinate; whether the bar is met | proposed |
| M3 | the live sets at the receive points of M2 and of the corpus's loops, as the IR computes them | the state size per suspension (C2's kill condition 1) | proposed: the IR has exact live sets |
| M4 | a step making a blocking host call (`os.File.Read`) under a stackless scheduler, against a goroutine, on Go | the hybrid lowering on Go | proposed |
| M5 | a seeded deterministic scheduler on all four targets, giving one trace per seed | whether the differential suite extends to nondeterminate programs | proposed |
| M6 | reductions counted at back edges, against none | the cost of preemption (C2(b)) | proposed |

---

## 7. Questions for hamza

Each changes what is built:

1. **Determinacy:** report it as a property, as portability is reported (§2.6)? Or keep Erlang's
   nondeterminism with no report?
2. **A full mailbox:** block, crash the sender, or return a status (§2.7)?
3. **Selective receive:** Erlang's scan of one mailbox, or typed ports (C2(a))?
4. **Preemption:** require every step's loops to be proven to terminate, or count reductions
   (C2(b))?
5. **Supervisor intensity** in seconds, as Erlang counts it, or in messages, which keeps a program
   determinate (§2.8)?
6. **The first program** (M2): the pipeline proposed, or one of yours? The Windows application is a
   Mealy machine (§2.1), and could be the first program instead.

---

## 8. Deliberately not here

- **Hot code loading.** Code is not a runtime value in a staged language.
- **Distribution and location transparency** (Waldo et al. 1994). A remote pid is a different type,
  when a program needs one.
- **Shared-memory concurrency**: locks, atomics, `SharedArrayBuffer`. There is nothing shared and
  mutable to protect (§2.5).
- **A BEAM target.** It would be the highest layer for C2, Core Erlang emitted from the IR, and
  ADR 0004's question, not this one's.
- **Effect handlers as a language feature.** §2.4 uses Plotkin and Pretnar's algebra to describe a
  supervisor. It does not propose user-defined handlers.

---

## References

- Agha, G. *Actors: A Model of Concurrent Computation in Distributed Systems*. MIT Press, 1986.
- Agha, G., Mason, I. A., Smith, S. F., Talcott, C. L. "A foundation for actor computation". *JFP* 7(1),
  1997.
- Armstrong, J. *Making reliable distributed systems in the presence of software errors*. PhD thesis,
  KTH, 2003.
- Armstrong, J. "A history of Erlang". HOPL III, 2007.
- Balzer, S., Pfenning, F. "Manifest sharing with session types". ICFP 2017.
- Berry, G., Gonthier, G. "The Esterel synchronous programming language: design, semantics,
  implementation". *Science of Computer Programming* 19(2), 1992.
- Blumofe, R. D., Leiserson, C. E. "Scheduling multithreaded computations by work stealing". *JACM*
  46(5), 1999.
- Brock, J. D., Ackerman, W. B. "Scenarios: a model of non-determinate computation". LNCS 107, 1981.
- Brookes, S. D., Hoare, C. A. R., Roscoe, A. W. "A theory of communicating sequential processes".
  *JACM* 31(3), 1984.
- Burckhardt, S., Kothari, P., Musuvathi, M., Nagarakatte, S. "A randomized scheduler with probabilistic
  guarantees of finding bugs". ASPLOS 2010.
- Caires, L., Pfenning, F. "Session types as intuitionistic linear propositions". CONCUR 2010.
- Candea, G., Fox, A. "Crash-only software". HotOS 2003.
- Chandra, T. D., Toueg, S. "Unreliable failure detectors for reliable distributed systems". *JACM*
  43(2), 1996.
- Clebsch, S., Drossopoulou, S., Blessing, S., McNeil, A. "Deny capabilities for safe, fast actors".
  AGERE 2015.
- Dunkels, A., Schmidt, O., Voigt, T., Ali, M. "Protothreads: simplifying event-driven programming of
  memory-constrained embedded systems". SenSys 2006.
- Fischer, M. J., Lynch, N. A., Paterson, M. S. "Impossibility of distributed consensus with one faulty
  process". *JACM* 32(2), 1985.
- Frigo, M., Leiserson, C. E., Randall, K. H. "The implementation of the Cilk-5 multithreaded
  language". PLDI 1998.
- Gray, J. "Why do computers stop and what can be done about it?". Tandem TR 85.7, 1985.
- Hewitt, C., Bishop, P., Steiger, R. "A universal modular ACTOR formalism for artificial
  intelligence". IJCAI 1973.
- Hoare, C. A. R. "Communicating sequential processes". *CACM* 21(8), 1978; and the book, 1985.
- Honda, K. "Types for dyadic interaction". CONCUR 1993.
- Kahn, G. "The semantics of a simple language for parallel programming". IFIP 1974.
- Kahn, G., MacQueen, D. B. "Coroutines and networks of parallel processes". IFIP 1977.
- Kuper, L., Newton, R. R. "LVars: lattice-based data structures for deterministic parallelism". FHPC
  2013.
- Lee, E. A., Messerschmitt, D. G. "Synchronous data flow". *Proc. IEEE* 75(9), 1987.
- Liskov, B., Shrira, L. "Promises: linguistic support for efficient asynchronous procedure calls in
  distributed systems". PLDI 1988.
- Miller, M. S. *Robust composition: towards a unified approach to access control and concurrency
  control*. PhD thesis, Johns Hopkins, 2006.
- Musuvathi, M., Qadeer, S., Ball, T., Basler, G., Nainar, P. A., Neamtiu, I. "Finding and reproducing
  Heisenbugs in concurrent programs". OSDI 2008.
- Nystrom, B. "What color is your function?". 2015.
- Parks, T. M. *Bounded scheduling of process networks*. PhD thesis, UC Berkeley, 1995.
- Plotkin, G., Power, J. "Algebraic operations and generic effects". *Applied Categorical Structures*
  11, 2003.
- Plotkin, G., Pretnar, M. "Handlers of algebraic effects". ESOP 2009.
- Reynolds, J. C. "Definitional interpreters for higher-order programming languages". ACM Annual
  Conference, 1972.
- Rutten, J. J. M. M. "Universal coalgebra: a theory of systems". *TCS* 249(1), 2000.
- Shapiro, M., Preguiça, N., Baquero, C., Zawirski, M. "Conflict-free replicated data types". SSS 2011.
- Smith, N. J. "Notes on structured concurrency, or: Go statement considered harmful". 2018.
- Sústrik, M. "Structured concurrency". 2016.
- Svensson, H., Fredlund, L.-Å., Earle, C. B. "A unified semantics for future Erlang". Erlang Workshop
  2010.
- Turner, D. A. "Total functional programming". *JUCS* 10(7), 2004.
- Wadler, P. "Propositions as sessions". ICFP 2012.
- Waldo, J., Wyant, G., Wollrath, A., Kendall, S. "A note on distributed computing". Sun TR-94-29, 1994.
