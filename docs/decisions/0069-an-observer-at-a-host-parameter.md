# 0069 — An observer at a host parameter; a binder's claim is an obligation

Date: 2026-10-21
Status: Accepted (hamza, 2026-10-21: "go ahead with 1–3", borrow-research.md §8 revised). Extends
[ADR 0020](0020-uniqueness-on-parameters.md)'s read-borrow from `b[i]` to a host call. Leaves ADR 0052
(and G5–G7), ADR 0058 and host-buffers.md's write-borrow and consume exactly as they were.

## Context

`examples/lines/shape.oro` met coverage cause C, a borrow that a later call ends (borrow-research.md).
The measurement that followed (ownbuf-2026-10-21) found that ownership does not decide the cost. A
64 KB buffer the program owns, searched by segment with the host's vectorized `bytes.IndexByte` and
`bytes.Count`, is as fast as the lent `ReadSlice`. What stopped the language from writing it was
narrower: a pure host call could not read a live buffer. `buffer(0..255)` was refused where
`Array(0..255)` was declared; through a `let` the buffer was handed on, a move, and the store after it
refused.

ADR 0020 had already given the program's own buffers Wadler's `let!` (1990) and Odersky's observers
(1992): *"The read-borrow is already free … an observation produces a scalar or a frozen array and
therefore cannot alias the buffer."* host-buffers.md §4 typed an aspect-3 host parameter as *"a read of
B that does not consume it"*. Neither was built for a host's parameter.

Writing `shape` in its caller-owned form also needed a claim on a value, `b ∈ S = e` (spec/j.md §5), to
give the IR a range the refinement layer proves relationally. That form existed in today's reader and
was lost with it. Re-adding it showed that the core's ascription `the(S; e)` was **trusted**: J could
write `the("int 0 5", 10)` directly, and a binder's claim desugared to it compiled when false.

## Decision

1. **An observer is derived, never declared.** A host declaration is an observer when it is pure, has a
   table parameter, and every result is a scalar (an integer, a boolean, a float, the unit). At such a
   parameter a live buffer stands for the frozen array it would be at the call: the type checker meets
   `Buffer(V)` with `Array(V)`, and linearity reads the argument as `(b i)`, a read, refused after a
   move and moving nothing. The borrow ends when the call returns.
   *Theorem.* The call is pure, so nothing writes the buffer while it runs, and its result has no
   extent, so it holds no part of the buffer afterwards. Its value is therefore the same whether the
   argument is the live buffer or a copy taken at the call. (borrow-research.md's Theorem L, with the
   epoch the call itself.)
2. **effects.md §7c generalized.** "A term that reads a table through a bound variable is not
   substituted into an impure body" now counts an observer applied to a bound variable as such a read.
   Without it the observation moves past the buffer's next store; linearity then refuses the program,
   which is sound but refuses a correct program.
3. **A segment is the declaration's business.** `segment.F(b, lo, hi, x̄) = F(b[lo:hi], x̄)`: F composed
   with the restriction of b to [lo, hi), its precondition the restriction's domain, and its template F's
   with the slice in the table's place. It is checked against F, which is checked against the manifest
   (`targets/go/bytes.oro`).
4. **A binder's claim is an obligation.** `b ∈ S = e` with S a range marks e as a declared parameter's
   range marks an argument (ADR 0028). The mark is decided by a literal, the intervals or the refinement
   layer, or the program is refused. A set naming a constant is marked once reduction has unfolded it.
   Only a proven claim reaches the IR, which reads it as the value's set (ρ from a proven set, ADR 0049)
   and erases it after deciding. **`the` is not surface**: a call to it in J is refused (ADR 0062).
5. **The analysis this needed, each demanded by the program and each sound by construction.** In the
   refinement layer:
   - a host call's `ensure` may name the components of a tuple result by position, as io.Reader's law
     on `os.File.Read`, `n <= len(q) && len(q) == len(p)`;
   - a component's stated length is a buffer's length through a loop;
   - Houdini's templates gain v ≤ len t;
   - a join's templates gain its tails and their names;
   - a claim and an obligation's where-mark are read as the values they carry;
   - a scope's tails are its body's;
   - a goal over a conditional is split when it is not linear.

   Each has a planted fault that makes `shape` refused (observer-2026-10-21 §2).

## Why not

- **A mode `lent`, or an annotation `observes`, on the declaration.** Whether a host call may observe a
  buffer follows from what the declaration already states: purity and the result's representation. A
  keyword would claim it a second time, and could disagree with the first (ADR 0023's reason for not
  claiming what is not checked). `lent` stays a decision for borrow-research.md, for `Peek` and
  `Scanner.Bytes`, which have no caller-owned form.
- **Admitting a string result.** A Go string is immutable and copies, so `string(b)` cannot alias. But
  "cannot alias" is then a fact about each target's string representation. Scalars carry no extent on
  any target. A string result can be admitted when a program asks, with that fact stated per target.
- **An impure observer**, `os.Stdout.Write(b)`. The call may run arbitrary host code, and whether it can
  write the buffer is the receiver-reachability question of borrow-research.md §5 (E1). It is not
  needed by `shape`.
- **A language-level segment, `b[lo..hi]`.** A view of a live buffer is the lent algebra again
  (borrow-research.md §9). On Java a segment is (array, offset, length), not a value, so a language
  segment would be a cross-target design. The declaration-level composition needs neither.
- **Keeping `the` trusted and documenting it.** A trusted claim is an assumption the program text
  makes and nothing checks. Under ADR 0049 every claim is proven or refused.
- **Three more rules, built on the way and removed.** Entailment on nested subsets of the facts, ex
  falso on small variable sets, and a claim's bounds assumed of its binder. Each is sound, but once
  `shape` was finished none was load-bearing: the program compiles with each faulted, and a rule no
  program exercises is a rule nothing checks. The Omega test as Fourier–Motzkin's fallback was tried
  first and ran out of memory after 7 minutes.

## Consequences

- `shape` reads its own buffer with `os.File.Read`, no `bufio`, and is at parity with the same program
  written by hand (1.11×, inside the noise floor), and with `ReadSlice` at the same size (1.00×), with
  no garbage collection.
- Every pure host call with scalar results now reads live buffers. `utf8.Valid`, `RuneCount` and
  `bytes.Count` were the first.
- A claim inside the word is a proof obligation. Above the word an ascription stays ADR 0029's
  directive. A definition's declared result range inside the word is still not proven, only dropped at
  inlining (ADR 0062's logic, not built). It is recorded in ownbuf-2026-10-21, and nothing downstream
  reads it.
- `lent` waits for `Peek` and `Scanner.Bytes`.
