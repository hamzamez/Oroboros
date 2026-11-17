# 0125 — A dead region crosses a call as the callee's token

Date: 2026-11-17
Status: Accepted (hamza, 2026-11-17: "Build both fixes first", on
[alloc-2026-11-17](../../gauntlet/results/alloc-2026-11-17.md) §3.2's design). Extends
[ADR 0100](0100-a-dead-region-serves-a-request-it-fits.md) (R1d) and
[ADR 0107](0107-a-region-is-answered-by-its-capacity-class.md) across the boundary of
[ADR 0101](0101-a-library-instance-is-a-function-of-the-program.md)'s instances. Built with
[reusecall-2026-11-17](../../gauntlet/results/reusecall-2026-11-17.md).

## Context

ADR 0100's equation, by ADR 0107's classes:

  release(p, b) ; q ← region(b′)   ≡   q ← p   when class(b′) = class(b)

R1b applies it inside a region, and R1d across a loop's back edge, with a token that is linear on
every path. Both need the release and the allocation in one function. A K(ℕ\*) instance
(ADR 0101) allocates its result inside itself, as W13 requires, while its argument dies in the
caller. So in `loop acc, i { … again(acc * i, i + 1) }`, every step's `mul_small` makes a fresh
region and the caller frees the last one: 20,000 allocations where the classes allow about 15.
windows32's 32-bit heap returns each freed block as fresh pages, 50,921 faults on 20000!
(alloc §2).

## Decision

1. **A release commutes with a call that does not read the released region**:

     release(p) ; f(x)   =   f′(x, p),

   where f′ takes p as the token for its result's region, and ADR 0100's equation holds inside f′.
   It holds because a released region has no reference (W11), so it is not among f's arguments
   and f reads nothing in it. It is destination-passing style (Shaikhha, Fitzgibbon, Peyton Jones
   and Vytiniotis, FHPC 2017), and Perceus's reuse tokens passed to a callee (Reinking, Xie, de
   Moura and Leijen, PLDI 2021; Lean 4's reset/reuse, Ullrich and de Moura, IFL 2019).
2. **An instance has a token variant, `f~tok`**: f's parameters, then the token `tok` (a pointer,
   null or a dead region) and its table's element count `k_tok`. Its result's storage, the one
   allocation whose table every return yields, outside any loop, is
   `reuse-fit(tok, k_tok, n, w)`. That takes the token when the classes agree, and otherwise
   gives it back and allocates (ADR 0107). An instance whose returns yield more than one table, or
   whose result is built in a loop, has no variant.
3. **The caller hands the token as R1d hands one to an allocation**: a call of an instance with a
   variant is a request of the run's size, by fit, at the result's width. Its consumer is the call,
   rewritten to `f~tok(x, tok, k)`. The other arms give the token back by `discard`, as before.
4. **W12 checks both sides**:
   - in the caller, the call is the token's one consumer on its path, and the pair (tok, k) is a
     fitting token at every source, as for a `reuse-fit`;
   - in the variant, the token parameters are a legal source, since the caller's W12 proved what it
     hands, and their `reuse-fit` lies on every path that returns. A path into ∅ is exempt, as an
     unreachable arm is.
5. **A variant's call is impure**: it releases or reuses memory, so it keeps its place among its
   caller's effects (ADR 0010).

## Why not

- **In place, with the result written into the argument's own region as it is read** (Go's z = z·x,
  FP²'s fully in-place functions, Lorenzen, Leijen and Swierstra, ICFP 2023). The result is one
  digit longer and the argument may be shared, so it needs uniqueness of the argument at the call
  and a capacity proof, and each instance's body changes. The dead predecessor's region is free by
  the time of the call, and needs neither.
- **A free list in Σ_OS's allocator**: hidden state the decision cannot see, the language's
  allocation becoming the host's heuristic. ADR 0096's releases are the program's, placed and
  checked (W11, W12), and the token keeps them so.
- **Inline the instance where its argument dies**: ADR 0102's size argument, Σ|f|·π(f), the reason
  instances exist.
- **Fix it in the host's heap** (a heap option, a low-fragmentation heap): it covers one host's 32-bit
  heap and none of the alternation the algebra gives for free.

## Consequences

- The fact loop's steady state is two regions in turn, alloc §3.2's emulation: 0.676 of the product's
  time with ADR 0097's tail, and 1,255 page faults where there were 50,921.
- W12's statement grows by a consumer kind and a source kind. Each gets a planted fault.
- Only instances get variants. A program's own kept definition (ADR 0113) allocating its result
  could take the same rule. It is not built until a program pays for it.
