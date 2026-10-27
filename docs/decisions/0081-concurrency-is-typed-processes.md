# 0081 — Concurrency is typed processes: a Mealy machine, failure an effect, supervisors its handlers

Date: 2026-10-27
Status: Accepted as the direction (hamza, 2026-10-27: "I agree to the recommendations. when the time
comes, we will answer the open questions and design the surface"), on
[concurrency-research.md](../concurrency-research.md) §5's recommendation. **Not built; the surface and
the questions of §7 are open.** Settles [design-direction.md](../design-direction.md) §8's last open
question as a direction, not a design.

## Context

Concurrency was the one question the founding argument left open. concurrency-research.md (2026-10-04)
derived it from Erlang's model and "let it crash", named seven candidates with what would kill each, and
measured one thing (conc-ring-2026-10-04: a stackless step is 4–600× cheaper per hop than a native
thread, 76–152 bytes per process against 0.7–9.3 KB).

## Decision

1. **C2, typed processes.** A process is a **Mealy machine**: a message type (a variant), a state, an
   initial state and a step, or written with `receive` in a loop and translated to the step (the
   state-machine translation, §2.2, which a language without recursion makes stackless). An address is a
   typed capability, `send` is asynchronous into a mailbox of declared capacity, and a call carries a
   **linear one-shot reply capability**, checked as a buffer's use is (the session type !R.end).
2. **Failure is the exception effect**: a crash is a term of type 0, a link propagates it, and **a
   supervisor is its handler** with a restart policy, its tree declared as data.
3. **Isolation is a typing property** (immutable values, linear buffers), not a heap layout, and
   **determinacy is computed and reported**, as portability is.
4. **C3, Kahn process networks, is C2's determinate fragment**, computed (linear pids, no `after`, no
   choice); **C4, fork–join over immutable tables, is its degenerate supervisor** and may be built first,
   independently.
5. **The lowering** is stackless step functions over a scheduler by default, a host's own loop for a UI
   process, goroutines on Go where a step blocks in a host call, a receive for JavaScript's asynchronous
   host call.
6. **Open, answered when a program meets them** (§7 and C2's (a)–(c)): determinacy reported or not; a
   full mailbox's policy; selective receive or typed ports; preemption by proven termination or counted
   reductions; supervisor intensity in seconds or messages; the first program; and then the surface.

## Why not

The research's own: Erlang faithfully (C1, untyped and dynamic where the language proves); Kahn networks
as the whole model (C3, killed by any server with two clients); CSP (C5, no failure propagation: a crashed
partner is a deadlock); session-typed channels as the base (C6, deadlock freedom lost with shared
servers); async/await (C7, a lowering, not a model). Each is kept where it fits: C3 and C4 as fragments,
CSP's channels as the lowering on Go, session types as the reply's typing, async as the lowering on
JavaScript.

## Consequences

- concurrency-research.md joins CLAUDE.md's primary documents until it is built.
- Nothing is built until a program asks; the measurements M2–M6 of §6 are what the build starts from.
- The Windows application's window procedure is a Mealy machine too (§2.1): when 6d's callbacks wall is
  designed, it is designed against this decision.
