# 0107 — A region is answered by its capacity class

Date: 2026-11-12
Status: Accepted (hamza, 2026-11-12: "go ahead", the next step after ADR 0106, storage for a growing
numeral). **Amends [ADR 0100](0100-a-dead-region-serves-a-request-it-fits.md)**: a dead region serves
a request of its class, a longer one included, where ADR 0100 required b′ ≤ b.

## Context

After ADR 0106, K(ℕ\*)'s product of a numeral by a word is the machine's row (`mulq`, `add`, `adc`),
and `factm`, the product 20,000! mod 10⁹ + 7, was still 1.6× Go's process. Each step makes a table one
digit longer than the one that dies, so no dead region ever fits the next request: ADR 0100's token
compares n ≤ k and gives the region back, and the heap is asked for a block 8 bytes larger than the
one just freed.

Measured in C on the same sequence (20,000 tables, 2 to 4,102 digits, each freed after its
successor is made; `HeapAlloc` with the process heap, cold): **66–81 ms at exact sizes, 19 ms at
power-of-two sizes**, the latter what writing the digits costs. At exact sizes the block just freed
is always one digit short of the next request; at classes the next request is of the freed block's
class until its size crosses a power of two.

## Decision

1. **A heap request of b bytes takes a block of c(b) = max(64, 2^⌈log₂ b⌉)**, segregated fits by
   powers of two (Knowlton 1965's buddy classes; Wilson et al. 1995's survey). c is monotone, idempotent
   and extensive (b ≤ c(b) = c(c(b))), and c(b) < 2b for b > 64, so the internal fragmentation is
   below 2×. A block freed at class c is on the heap's own free list for every later request of class
   c. Windows writes c as a case on b's thresholds (`heapclass`, targets/windows/os.oro), so its range
   is the decision's, the join of its arms' constants.
2. **Σ_OS's `class` is the capacity**: class(b′) = class(b) means a region answered for b holds b′
   bytes and release(p, b′) gives it back. It is the kernel of b ↦ (allocator, capacity), an
   equivalence whose classes are the blocks' sizes. Windows: c(b) on the heap, −⌈b/4096⌉ (committed
   pages, negated so the two allocators' classes are disjoint) on VirtualAlloc; HeapFree and
   VirtualFree with MEM_RELEASE free a whole block whatever the size given.
3. **A dead region serves every request of its class**:

        release(p, b) ; q ← region(b′)   ≡   q ← zero(p, b′)     whenever class(b′) = class(b).

   *Proof*: region(b′) answers a fresh block of class(b′)'s capacity whose first b′ bytes are 0, and
   nothing reads past b′ (a table's view is its n·w + 8 bytes, ADR 0084). p is such a block, and its
   address is unobservable after W11; zero(p, b′) makes its first b′ bytes 0. ∎ A reuse keeps the
   class, so a block's capacity stays the class of the count it holds, and its eventual release is
   by the right rule. `reuse_fit` tests class equality alone (targets/llvm/lang.oro); the IR's token
   and W12 are unchanged, since the IR hands the token's count k and the request n and the target
   decides the fit.

## Why not

- **Exact sizes**: the measured 3.5–4.2× of the allocation's cost, on every growing numeral.
- **An allocator of our own in the layer** (a free list per class). The process heap already keeps one
  per size, and the classes alone reach the writes' cost; it would lower further than the target
  requires.
- **A capacity in the table's header** (a growable vector, `realloc`'s doubling). It changes every
  table's representation (the header is 2m + s, ADR 0099) for what a function of the size gives
  without a field, and a K(ℕ\*) value is immutable: no table grows in place.
- **Keep ADR 0100's n ≤ k** beside the classes. It is true but weaker than the law, which is class
  equality. Measured inside the noise (below); kept as the law says, so a numeral one digit longer than
  its dead predecessor takes its region without the heap.
- **Classes of ratio 1.5 or Fibonacci sizes** (less overhead, more classes). Not measured; powers of
  two already reach the writes' cost.
- **Static-size tokens across sizes of one class**: the IR would compute class itself, and every R1b
  token is between equal sizes. No program asks.

## Consequences

- The product of a numeral by a word 1.6× → 0.99× Go's; `fact` 3.2× → 2.7×, its gap now printing;
  `fib` 1.9×, unchanged, its gap the sum's carry; the 2,000 quotients 0.92× (class-2026-11-12).
- Every heap table up to 64 KiB takes at most twice its bytes.
