# Research: ℤ on a target with no bignum, and the storage it needs

2026-11-08. **Research; the decision is hamza's** (hamza, 2026-11-08: "yes. do the research, always
mathematical and algebraic. and reach for the literature", on the two walls llvm-render-2026-11-07
left: `big-fib` and `big-sign`). 6b′ step 3.

**The short version:**
- **Nothing enters the language.** Its integers are ℤ and a range is a set. What windows lacks is a
  **representation**: every one it has is finite, and a set with no bound has no finite
  representation (§2). Since a target may not decline ℤ, the LLVM layer owes one whose image is all
  of ℤ, and on a machine the only kind is **variable length** (§3).
- **Signed and unbounded are one construction.** ℤ is the group completion of the monoid (ℕ, +), and
  its normal form is sign and magnitude. So any representation M of magnitudes gives one, Z(M), of
  signed integers, its operations derived from M's (§4). Two instances settle both walls:
  - Z(V_n), the fixed limbs with a sign, is `big-sign`;
  - Z(ℕ*), variable-length magnitudes with a sign, is all of ℤ, `big-fib`.
- **Storage comes first.** Nothing on the LLVM layer frees memory, so every table leaks. Measured:
  10⁶ limb additions hold 252 MB at their peak and 10⁷ hold 2.7 GB (§5.1). An unbounded
  representation would make the leak unbounded too. The values are immutable, first-order and
  acyclic, so the IR knows exactly when each is dead. **Free at death, with same-size reuse**, decided
  by the IR's liveness, reclaims all of it with no counts and no collector (§5).
- **The views (ADR 0084) decide safety and an exclusive region's death; they leave a frozen value's
  storage**, whose persistent permission is linear logic's !A, droppable and so never freed. Counted
  fractions in its place make free at death the views' own `free` rule (§5.6). Goose's model adds
  where storage lives: by size class, so a fixed limb value needs no allocation at all (§5.7).
- **Recommended order** (§8): reclamation, then Z(V_n), then the variable-length magnitudes and Z
  over them.

---

## 1. The issue

### 1.1 The two walls

| case | its set | why windows refuses it today |
|---|---|---|
| `big-fib` | ℕ: `fib(n)` for an unbounded n | no representation of an unbounded set |
| `big-sign` | a signed range above the word | the fixed limbs hold magnitudes only (ADR 0029 decision 4) |

On Go, the JVM and JavaScript both cases run on the host's bignum (`math/big`, `BigInteger`,
`BigInt`), which the language parasitizes (ADR 0001).

### 1.2 What the language already says

- An `Int` is an integer (ADR 0026), and a declared set is a claim; the facts choose each value's
  representation (ADR 0079).
- A representation is a pair (H, d): a host type H and an abstraction function d : H → ℤ (Hoare 1972;
  ADR 0083). A set S can be held in it when S ⊆ d(H). Every conversion between representations is
  the identity on ℤ, so no choice of representation changes an answer.
- Above the word, the enforceable sets form the lattice (ADR 0029)

      H = { N_k = [0, 2ᵏ) } ∪ { Z_k = (−2ᵏ, 2ᵏ) } ∪ { ℤ },

  decided by two O(1) observables of a sign-magnitude integer: its sign and its magnitude's bit
  length. Every host bignum is sign-magnitude (ADR 0029's survey, §4.4 below).
- ADR 0029 decision 4: "fixed limbs realize N_k only". That is the wall `big-sign` meets.

---

## 2. Why windows has no representation of ℕ

windows' integer representations:

| representation | H | d(H) |
|---|---|---|
| S₆₄ | `i64` read signed | [−2⁶³, 2⁶³) |
| U₆₄ | `i64` read unsigned (llvm.md §3a) | [0, 2⁶⁴) |
| V_n, fixed limbs (ADR 0035) | n limbs of 24 bits | [0, 2²⁴ⁿ), one n per program |

**Proposition 1.** No representation with a finite carrier holds an infinite set.

*Proof.* d(H) is the image of a finite set, so it is finite, and an infinite S is not contained in a
finite set. ∎

Every carrier above is finite, so `big-fib`'s ℕ is held by none of them. The program is refused
because the target's representations do not cover its set, not because the language lacks anything.

**What the target owes.** CLAUDE.md: "Anything promoted to the language works on every target, and
the compiler finds the implementation. A target may neither decline nor declare a language
construct." ℤ is the language's integer sort, so a complete target needs a representation (H, d)
with d(H) = ℤ. A machine's carrier is a set of finite bit strings, and for d(H) to be infinite, H
must be. So H must contain strings of every length: **a representation of ℤ is variable length**,
its size chosen at run time by the value. Each host bignum is one. LLVM has none, so the LLVM layer
must provide one, as it provides the fixed limbs: a library written in the language and checked as
a theory (ADR 0035). Programs never name it.

---

## 3. ℕ by positional numerals of any length

### 3.1 The algebra

**Theorem (positional notation).** For every base B ≥ 2, each m ∈ ℕ has exactly one representation
m = Σ_{i<ℓ} aᵢ·Bⁱ with 0 ≤ aᵢ < B and ℓ = 0 or a_{ℓ−1} ≠ 0.

Existence and uniqueness follow from Euclidean division by B, by induction on m (Knuth, TAOCP 2,
§4.1). So with H = [0, B)* the finite sequences of digits, and d(a) = Σ aᵢ·Bⁱ:
- d is **surjective** onto ℕ;
- d is **injective on the normal forms** N = { a : ℓ = 0 or a_{ℓ−1} ≠ 0 }, and d|N is a bijection
  N ≅ ℕ.

ADR 0083's abstraction function is injective, so the representation is N, the normal forms. An
operation must answer a normal form, which costs one test on its top digit.

**In the language**, a digit sequence is a table: `Array(0..B−1)`, its length ℓ. A table's length is
fixed when it is made, and an operation's result length is known only after it runs: a sum has
max(ℓa, ℓb) or one more digit, depending on the final carry. Two answers in the literature:
- **a used length beside a capacity**: GMP's `mpz_t` holds `_mp_alloc` and `_mp_size`, the sign in
  the size's sign. In the language that is a pair (t, m), with m ≤ len(t) and
  d(t, m) = Σ_{i<m} tᵢ·Bⁱ, normal when m = 0 or t_{m−1} ≠ 0;
- **an exact result, by a second pass or a bound**: allocate the bound, then make a table of the
  exact length if the bound was not met.

The first is the one every bignum library uses, and the second copies. Recommended: (t, m), a
product, which the IR already carries (ADR 0031).

### 3.2 The base

Knuth's Algorithm M computes each inner step t = uᵢ·vⱼ + w_{i+j} + k, with each term below B. Its
greatest value is

    (B − 1)² + (B − 1) + (B − 1) = B² − 1,

so the inner step is exact in any word holding [0, B²). The base is therefore a function of the
target's largest exact integer:
- **LLVM's U₆₄**, [0, 2⁶⁴): B = 2³², since (2³²)² − 1 = 2⁶⁴ − 1. The bound is proven by the decision
  on the library's own body, as the fixed library's bounds are.
- **JavaScript's 2⁵³**: B ≤ 2²⁶. The fixed library chose 2²⁴ for headroom (emit/bignum.oro), and
  JavaScript has `BigInt`, so it never takes the new library.
- **Native i128 on LLVM** (ADR 0083's "native iN above the word"): B = 2⁶⁴, with the product in
  `mul i128`. That halves the digits and quarters the digit products. It is ADR 0083's deferred
  item, and is measured against B = 2³² rather than assumed.

### 3.3 The operations

All from Knuth, TAOCP 2, §4.3.1, on normal forms:

| operation | algorithm | cost | note |
|---|---|---|---|
| a + b | A, carry propagation | O(max(ℓa, ℓb)) | result length max + 1, normalized |
| a − b, a ≥ b | S, borrow propagation | O(ℓa) | leading zeros stripped |
| a · b | M, schoolbook | O(ℓa·ℓb) | inner step exact by §3.2 |
| a div b, a mod b | D | O(ℓb·(ℓa − ℓb + 1)) | normalization, then q̂ by two digits |
| a ≤ b | length, then digits from the top | O(1) usually | normal forms make the length decisive |
| render | division by 10⁹ in one pass, nine digits per word | O(ℓ²) | 10⁹ < 2³², and r·B + digit < 10⁹·2³² < 2⁶² |

**Algorithm D** is the one the fixed library lacks: it divides only by a word (`div_small`), and a
quotient by a bignum is refused there ("needs a quotient estimate"). For ℤ to be complete, `/` and
`%` must take any divisor. D normalizes the divisor so that its top digit is at least B/2, after
which the two-digit estimate q̂ satisfies **q̂ − 2 ≤ q ≤ q̂** (Knuth's Theorem B). So at most two
corrections per quotient digit are needed, and D's correctness is a theorem the library's checks
can cite.

**Faster algorithms**, each later and measured, never assumed:
- Karatsuba's O(n^log₂3) multiplication (Karatsuba and Ofman 1962), above a threshold. GMP measures
  its thresholds per machine, typically tens of digits.
- Divide-and-conquer radix conversion for render (Brent and Zimmermann 2010, §1.7).
- Division by an invariant word by multiplication (Granlund and Montgomery 1994; Möller and
  Granlund 2011), which LLVM already does for a constant divisor such as 10⁹.

---

## 4. ℤ from ℕ: the group completion

### 4.1 The construction

(ℕ, +, 0) is a commutative monoid with cancellation (a + c = b + c ⇒ a = b). Its **group
completion** (Bourbaki, Algebra I, ch. I, "symmetrization"; later the Grothendieck group K(M)) is

    K(ℕ) = ℕ × ℕ / ~,    (a, b) ~ (c, d)  ⟺  a + d = b + c,

where (a, b) stands for a − b. It is a group, the universal one into which ℕ embeds, and K(ℕ) ≅ ℤ.
Cancellation is what makes ~ transitive.

**Normal form.** Every class contains exactly one pair with a zero: (m, 0) for m − 0 ≥ 0, and
(0, m) with m > 0 for a negative value. Writing that as a sign and a magnitude,

    ℤ ≅ { (s, m) : s ∈ {+, −}, m ∈ ℕ } / ((+, 0) ~ (−, 0)),

which is **sign-magnitude** with one zero, its sign chosen +.

### 4.2 The operations, derived

Each operation of ℤ is computed on normal forms through operations of ℕ:

| ℤ | on normal forms | ℕ's operations used |
|---|---|---|
| x + y, equal signs | the sign, \|x\| + \|y\| | + |
| x + y, different signs | the larger magnitude's sign; the larger minus the smaller | ≤, − |
| −x | the sign flipped (0 stays +) | none |
| x − y | x + (−y) | as + |
| x · y | the product of the signs; \|x\|·\|y\| | · |
| x / y, x % y (truncated) | the quotient's sign is the product of the signs, the remainder's the dividend's; the magnitudes from \|x\| div \|y\|, \|x\| mod \|y\| | div, mod |
| x ≤ y | by sign, then by magnitude (reversed when both are negative) | ≤ |
| render(x) | "−" when negative, then render(\|x\|) | render |

The derivation of the second row: (m, 0) + (0, n) = (m, n), whose normal form is (m − n, 0) when
m ≥ n and (0, n − m) otherwise. Truncated division is the language's on every host (integers.md §4;
Boute 1992 calls it T-division), and on sign-magnitude it is exactly the magnitudes' Euclidean
division with the signs above. Every ℕ operation the table uses is in the fixed library already, for
each n: `add`, `sub`, `cmp`, `mul`, `div_small`, `rem_small`, `render`.

### 4.3 One construction over any magnitude representation

Nothing in §4.2 depends on how ℕ is represented, only on its operations. So for any representation M
of magnitudes, Z(M) = {+, −} × M with the derived operations represents the group completion of
d(M)'s monoid, cut to the same bound:

| magnitudes M | Z(M) | the set of ADR 0029's lattice | the case |
|---|---|---|---|
| V_n, n fixed limbs | Z(V_n) | Z_k, k ≤ 24n | `big-sign` |
| ℕ*, variable length (§3) | Z(ℕ*) | ℤ | `big-fib` |

- **The bound is enforced on the magnitude.** Z_k is |x| < 2ᵏ, so Z(V_n)'s check is the fixed
  library's magnitude guard, unchanged. ADR 0029 decision 3's two observables are then literally the
  representation's two components.
- **A value of N_k does not take the sign layer.** ρ chooses from the facts (ADR 0079), so an
  unsigned program's arithmetic is unchanged, branch for branch.
- **ADR 0029 decision 4 is amended**, not reversed: fixed limbs still hold magnitudes, and Z(V_n) is
  how they realize Z_k.

### 4.4 The alternative: two's complement fixed limbs

The residue map r : ℤ → ℤ/Bⁿ is a ring homomorphism, injective on [−Bⁿ/2, Bⁿ/2). So a signed
bounded set can be held as residues read signed, as S₆₄ is `i64` read signed (llvm.md §3a):
- **+, − and · are the existing magnitude code**, unchanged and without a sign branch: the
  homomorphism makes them exact wherever the result is in range;
- **order, division, render and the bound check need signed forms**: the top digit's sign, then
  negation and the magnitude algorithms for division and render;
- **it does not extend to ℤ**: ℤ is not a residue ring. The 2-adic integers, infinite two's
  complement, extend it, which is what Java's `BigInteger` emulates for its bitwise operations over a
  sign-magnitude store.

What the hosts chose, all for variable length:

| host | store |
|---|---|
| Go `math/big` | `Int{neg bool, abs nat}` |
| Java `BigInteger` | `signum` and `mag[]` |
| V8 `BigInt` | a sign bit and digits |
| GMP `mpz_t` | the sign in `_mp_size`, the magnitude in limbs |

Every one is sign-magnitude, because the magnitude algorithms (Knuth works on non-negative integers
throughout §4.3.1) are the expensive part, and sign handling is O(1) beside them.

**So Z(M) is recommended**: one construction for both walls, over operations that exist. Two's
complement fixed limbs would be a second mechanism for the bounded case, and are worth building only
if a program's mixed-sign addition is measured paying for its comparison.

### 4.5 The bits above the word

ADR 0082's operations `& | ⊕ ~` on a negative value are defined 2-adically: ~x = −x − 1, and
x & y on the infinite two's complement expansions. On sign-magnitude they are computed as Java's
`BigInteger` does, converting each negative operand's magnitude on the fly. That was CLAUDE.md's
waiting item "the Boolean operations above the word", unchanged by this choice. *Built 2026-11-19 on
K(ℕ\*) by De Morgan from operations on naturals, a negative a being ~(A − 1)
([bigbits-2026-11-19](../gauntlet/results/bigbits-2026-11-19.md)).*

---

## 5. Storage

### 5.1 Measured: the LLVM layer never frees

The table allocator is a term over Σ_OS's `region` (ADR 0092), and nothing releases a region: there
is no `HeapFree` or `VirtualFree` in `targets/`. The fixed library makes a fresh table per operation
(`var o := array { … }`), so each limb addition allocates one. A loop of limb additions, on windows,
in a program declaring `(0..2^200)` (9 limbs):

| iterations | peak working set | time |
|---|---|---|
| 10⁶ | 252 MB | 0.83 s |
| 10⁷ | 2,714 MB | 2.0 s |

That is about 264 bytes per iteration, linear and never returned. The leak is not specific to big
values: any loop that makes a table each iteration leaks the same way on LLVM. On Go, the JVM and
JavaScript the host's collector reclaims, so nothing in the differential suite noticed.

### 5.2 What the project has already decided

- **No collector of the language's own** (pointers-research §13.6): a collector is a hidden allocation
  policy and a runtime, which the core forbids. A host's collector is a region whose `free` is the
  host's.
- **Values are immutable and buffers linear** (ADR 0018, 0020), and a scope's end consumes what it
  owns (pointers-research §13.3, Tofte and Talpin's `letregion`).
- **A table's storage is an operation of the IR** (ADR 0092), `alloc(n, w)`, realized by a term over
  the host's.

### 5.3 The facts that decide it

**Proposition 2 (the heap is acyclic).** In a program whose values are immutable once made, the
graph of references between heap values is a DAG, ordered by creation time.

*Proof.* A value's contents are fixed when it is made (a buffer, while being written, holds only
values that already exist, and is frozen on the way out, ADR 0018). So if v refers to w, then w
existed when v was made: t(w) < t(v). A cycle v₀ → … → v₀ would give t(v₀) < t(v₀). ∎

So **reference counting would be complete**: its one failure, cycles (Collins 1960; Jones, Hosking
and Moss 2011, ch. 5), cannot occur. That is the observation behind Lean 4's counts (Ullrich and de
Moura 2019), whose values are immutable too.

**But counts are not needed for most values here**, because the language is first-order, has no
recursion, and no closure survives staging:
- A heap value can be referenced from three places only: **SSA names** in the IR, an **element of
  another value** (a table of strings), and **a host**.
- SSA names are static: in structured SSA, liveness is exact, so the point after a name's last use is
  known (Appel 1998, ch. 10).
- **Containment is rare, and made by hosts.** In the corpus, 4 of 153 files hold a table of heap
  values (`Array(String)`, `Array(bytestring)`), and every one is a host's result: `os.Args` in
  `examples/pe` and the three `os` cells. No program builds one.
- **A host retains nothing on LLVM.** Win32's functions read a borrowed table during the call
  (ADR 0070), and nothing on the layer stores a table in a host object.

**Proposition 3 (free at death is exact).** Let v be a heap value referenced by no other heap value
and retained by no host. Then v is unreachable exactly at the program point after the last use of
the last SSA name bound to it. Releasing its storage there frees it neither early nor late.

*Proof.* Its only references are SSA names. A name past its last use is never read again, by
liveness, and no other reference exists, by hypothesis. ∎

### 5.4 The options

| | what it is | here |
|---|---|---|
| **L0** a tracing collector (McCarthy 1960; Boehm and Weiser 1988) | find the live set from the roots | a runtime and a hidden policy: refused by §13.6 |
| **L1** free at death, decided statically | release at the point Proposition 3 names: ASAP (Proust 2017), Rust's drop at a scope's end | exact for every value that is not contained; no counts, no run-time cost but the release |
| **L1 + reuse** | a dead value's storage becomes a new value's of the same size, at the same point (Perceus's reuse, Reinking et al. 2021; Lorenzen and Leijen 2022; FP², Lorenzen, Leijen and Swierstra 2023) | allocation disappears where sizes match |
| **L2** precise reference counting (Collins 1960; Ullrich and de Moura 2019; Perceus) | a count per value, increments at a share and decrements at a death, most of them removed by borrow inference | complete by Proposition 2, and a count word and an update per share, for what L1 already decides |
| **L3** regions per scope or per loop iteration (Tofte and Talpin 1997; Cyclone, Grossman et al. 2002) | a region freed whole at a scope's end, carried values copied out | each iteration copies the values it carries |

**L1 with reuse is the one the facts choose**, and it needs no counts in the common case:
- **Fixed limbs**: every limb value of a program has the same width n (one width per program,
  ADR 0029), so a dead limb value's table always fits the next one. A loop carrying k limb values
  then holds at most about k + 1 tables in steady state, whatever its trip count. Fibonacci's
  (a, b) → (b, a + b) writes a + b into a's storage, the shape the destination forms `big+!` were
  made for on Go.
- **Variable length** reuses where the dead table's capacity holds the result, as GMP's `mpz_add` does
  when its destination aliases an operand.
- **A contained value**, the case L1 does not decide, has an owner, its container, and an element
  read out of it is a second reference. Either the read is a **borrow** of the container, when the
  element's uses end before the container's death (a comparison of two liveness intervals, ADR 0070's
  caller's view of a borrow), or it is a **copy**, which is unobservable because values are immutable
  and the language has no identity comparison. Counted views (pointers-research §14.16) stay
  available if a program measures a copy as too dear. No program in the corpus reaches that case
  today.

### 5.5 Realizing release

Release is an operation of the IR, as `alloc` is (ADR 0092), and each target realizes it:
- **LLVM**: a term over Σ_OS. Σ_OS gains `release(p, b)` beside `region(b)`. Since `region` chooses
  `VirtualAlloc` for 2¹⁶ bytes or more and `HeapAlloc` below, by the size alone, `release` chooses
  `VirtualFree` or `HeapFree` by the same rule. Then release ∘ region is the identity on the heap,
  by construction.
- **Go, the JVM, JavaScript**: nothing. The host's collector owns the region's `free`, so release is
  realized by the empty term, and their emission does not change.

### 5.6 The views (ADR 0084): what they decide, and what they leave

ADR 0084 made ownership a **view**: a claim in the logic, erased, that a scope may access a range of
a region with a permission. Views form a separation algebra (Calcagno, O'Hearn and Yang 2007); a
linear view must be consumed. The permissions are exclusive, fraction, persistent and device.
`free` is an axiom that takes the full permission (pointers-research §12):

    { r[0..n) ↦₁ _ }   free(r)   { emp }

**What the views decide:**
- **Safety of every access.** There is no load or store without a view, so a use after free, a
  double free or an access out of range cannot be written (pointers-research §14.1).
- **When an exclusive region dies.** A `var` buffer's view is consumed at its scope's end, which is
  Tofte and Talpin's `letregion`: the end of the scope is the point of `free`. Nothing implements it
  on LLVM yet, which is part of the leak.
- **No leak of an exclusive region.** A linear view cannot be dropped (§13.3), so for every region a
  program owns exclusively the reclamation point exists and is unique.
- **What memory means**: under an exclusive view memory is a value (Theorem V), which is why every
  fact about buffers survives.
- **The host's side**: what a host lends for a call, keeps, fills, or returns as owned (P2–P6).

**What they leave: the persistent permission.** Freezing a buffer makes a value, and its view becomes
**persistent**: readable and duplicable (ADR 0084's table: "a frozen buffer, a constant"). In linear
logic that is the exponential !A, which admits contraction (copy) and weakening (drop) (Girard 1987).
Weakening means a persistent view may vanish without being consumed, so the logic never reaches a
point where `free`'s full permission is in hand. **Every value the leak of §5.1 holds is behind a
persistent view**: a limb result, a frozen table, a string. The views were designed for safety, and
for values they say "never freed", as Rust's `'static` does.

**Closing it inside the algebra.** A frozen region does not need !A. It needs **counted fractions**
(Boyland 2003; counting permissions, Bornat, Calcagno, O'Hearn and Parkinson 2005):
- freezing splits ↦₁ into one fraction per reference;
- each reference's death returns its fraction;
- the fractions sum to 1 again at the last death, and `free`'s precondition holds there.

The count is static where the references are SSA names. Their number and their deaths are the IR's
liveness, and that is Proposition 3: **free at death is the views' own `free` rule, applied to a
frozen region whose fractions are counted at compile time.** Where a reference's death is not static
(a value inside another value, read out and kept), the count is a run-time number, and that is
reference counting (L2): the same fraction algebra, executed. So L1 and L2 are not two models, but
one algebra decided statically or dynamically. pointers-research §14.16's counted view is that run-time
case. **Persistent** stays for what is never freed: constants and static data.

**And where the region lives** is the one question views do not ask: a region is a name, and its
allocator a term (ADR 0092). The heap is one realization. §5.7's are cheaper.

### 5.7 Goose's memory model, evaluated

[Goose](https://github.com/aardappel/goose) (van Oortmerssen; its spec is `docs/goose_spec.md`) is a
whole-program, memory-safe systems language that compiles to C, "no heap allocator, garbage collector,
or lifetime annotations". Its model, from the spec:

| Goose | the rule | in this document's terms |
|---|---|---|
| **N data stacks** (§1.2, §1.3) | each a reserved range with a bump pointer; values pushed in lifetime order and popped together at scope exit; "the only deallocation is resetting stack pointers at scope exit" | regions as a stack of regions, Tofte and Talpin's `letregion`, with bump allocation |
| **at most one resizable value per stack, on top** | growth is a pointer bump with no capacity check, and never moves memory | a growable region must end its stack |
| **N computed statically** (§10.3) | each simultaneously live resizable gets the lowest index not in use; an index frees at its scope's exit | greedy colouring of an interval graph, which is optimal: interval graphs are perfect, so the colours used equal the largest set of simultaneously live values (Gavril 1972; Golumbic 1980), as in SSA register allocation (Hack, Grund and Goos 2006) |
| **size classes** (§1.1, §1.3 item 5) | fixed-size values on the native stack or in registers; variable ones on data stacks | storage chosen by whether the size is static |
| **construction in place** (§4.3, §7.3) | a variable-size result is built in its receiver's storage, through calls, and the callee is specialized by the destination stack | destination-passing style (Minamide 1998; Shaikhha et al. 2017) |
| **roots** (§9.2) | every reference has static roots; a store is allowed only if the pointee outlives the container; roots inferred per specialization | region lifetimes and Cyclone's "outlives" (Grossman et al. 2002), inferred by monomorphization |
| **no exclusivity** (§9.1) | aliased mutation allowed, since threads share nothing | none: this language keeps exclusivity |
| **pools** (§5.4) | a freelist over a typed array for lifetimes that do not nest; a stale reference reads "a same-typed value", "type-safe reuse, never memory corruption" | pointers-research §14.3's arena with integer links, which concluded the same: a reused slot is a logic error, not a memory error |
| **relative references** (§3.9) | 1-, 2- or 4-byte offsets, position-independent, checked on load | ADR 0084 item 1: an address is an integer offset into a region |
| **the address-space bound** (§10.4) | every length, index and offset in [0, 2⁴⁸], so "size arithmetic cannot overflow" | a fact of the target |
| **threads** (§11.2) | separate programs, no shared mutable memory, flat values through typed queues | ADR 0081's processes |
| **recursion** (§7.8) | no call into a cycle while a non-fixed local is live, so the stack count stays static | this language has no recursion at all (ADR 0014) |

**Where Goose and this project already agree**, reached independently: offsets instead of addresses,
type-safe reuse of a pool slot as a logic error, threads that share nothing, and no collector. That is
evidence for ADR 0084 and ADR 0081, not something to take.

**What not to take:**
- **Aliased mutation without exclusivity.** Goose is sound without it because it gives up equational
  reasoning about memory. This project's facts rest on Theorem V, which needs the exclusive view.
- **Specialization per root and per destination.** It exists because Goose compiles functions
  separately. Here β inlines every call of the program into its entry point (ADR 0071), so every
  value's destination is already in the one function the IR sees.
- **Goose's open items** name its unsolved cases: "a move operation for resizables" (B.3), lifetime
  precision across long-distance returns (B.0d), pool garbage collection (B.7). Its stacks do not
  handle a loop-carried value whose size changes: assigning to a resizable is "clear-then-construct",
  and the right-hand side "may not use the array being rebuilt" (§4.4). That is exactly `big-fib`'s
  (a, b) → (b, a + b) on variable-length digits. So the heap with free at death (§5.4) stays for that
  case.

**What to take, three things:**
1. **Storage by size class.** The heap is the realization of last resort, not the default.
   - **A table whose length is static after staging** is a fixed-size value, an element of Vⁿ.
     Every fixed-limb value is one, since there is one width per program. Its storage is a frame slot
     (LLVM `alloca`), which a loop carries as it carries any loop variable (a join is a slot,
     llvm.md), and the frame frees it. That is **zero allocation calls**, where §5.4's reuse still
     calls `HeapAlloc` once per live value. §5.1's loop needs none at all.
   - **A table of run-time length that dies inside the IR region that made it** goes on a data stack:
     a bump to allocate, and the region's exit pops it with everything made after it. The IR's regions
     are structured (ADR 0032), so lifetimes that end at a region's exit nest, which is Goose's §1.3
     invariant. Tables here never grow after they are made (a table is born full, ADR 0084 item 8), so
     one stack suffices where Goose needs N for its resizables.
   - **A value that escapes its region**, yielded to a join or carried by a loop, with a run-time
     length: the heap, freed at death (Proposition 3), reused where it fits. That is the case Goose
     leaves open.
2. **Construction in place at a join.** A value yielded out of a region can be built in the outer
   region's storage when its length is known on entry (Goose §4.3). That removes the copy a join
   makes today.
3. **The address-space bound as a fact.** x64's user space is 2⁴⁷ bytes, so on windows every table has
   len(t) ≤ 2⁴⁷ − 9, a fact of the host, as the JVM declares `fact max_len … len(a) <= (2^31-1)`
   (targets/java/java.oro). Then n·w + 8 is in the word for every table, by the facts alone. ADR 0092's
   guard on the allocator becomes a theorem, not a case. On i686, user space is at most 2³² bytes.
   windows declares no such fact today.

---

## 6. What each target does

| | Go, the JVM, JavaScript | LLVM (windows, and every LLVM target after it) |
|---|---|---|
| N_k, Z_k above the word | the host's bignum, or V_n by `-big-repr` | V_n, Z(V_n) |
| ℤ, ℕ unbounded | the host's bignum | Z(ℕ*), B = 2³² over U₆₄ |
| a dead value's storage | the host's collector | released at death, reused where it fits |
| i686 | — | the word is still S₆₄ (ADR 0083), so B = 2³² over LLVM's i64 |

---

## 7. Measurements owed, each with a time limit

1. **M1, the leak** (an hour): the loop of §5.1 with release and reuse, peak memory flat across 10⁶ and
   10⁷ iterations, and its time against today's 0.83 s and 2.0 s.
2. **M2, the sign layer** (an hour): Z(V_n) against V_n on the same unsigned program. Its emission
   should be identical (ρ does not take the layer), and a signed one is timed against Go's `math/big`.
3. **M3, the variable-length library** (two hours): `fib` and `fact` on Z(ℕ*) on windows against Go's
   `math/big`, the same digits, timed. Measured, not predicted: biglimb-2026-09-02 found the fixed
   library 2.8× to 75× slower than the hosts' bignums.
4. **M4, the base** (an hour): B = 2³² over U₆₄ against B = 2⁶⁴ over i128, on M3's programs.

---

## 8. Recommendations

1. **R1, reclamation first, by size class and lifetime** (§5.4, §5.6, §5.7), on the LLVM layer:
   - a table of static length, every fixed limb value among them, in a frame slot: no allocation;
   - a table of run-time length that dies in its region, on a data stack, popped at the region's exit;
   - one that escapes its region, on the heap, freed at death by the IR's liveness (the views' `free`
     with statically counted fractions) and reused where it fits;
   - a value inside another, borrowed when liveness proves the borrow, copied otherwise, and counted
     only if a program measures the copy as too dear.

   `release` is an IR operation, a term over Σ_OS's new `release` on LLVM and the empty term on the
   collected hosts. With it, the target fact len(t) ≤ 2⁴⁷ − 9 on x64. It fixes today's leak for every
   table and every fixed limb value before anything new is allocated.
2. **R2, Z(M), the group completion over magnitudes**, one construction. Its first instance is Z(V_n)
   over the existing fixed limbs, which amends ADR 0029 decision 4 and runs `big-sign`. **Built**
   (ADR 0098, [signlimbs-2026-11-09](../gauntlet/results/signlimbs-2026-11-09.md)).
3. **R3, variable-length magnitudes ℕ\***: normal forms (t, m), base 2³² over U₆₄ on LLVM, Knuth's
   Algorithms A, S, M and D, and render by 10⁹; schoolbook first, with Karatsuba and native i128 only
   after M3 and M4. Z(ℕ*) is ℤ on LLVM, and runs `big-fib`. **Built**, D included (ADR 0099,
   [natnum-2026-11-10](../gauntlet/results/natnum-2026-11-10.md), [natdiv-2026-11-10](../gauntlet/results/natdiv-2026-11-10.md)).
4. **R4, nothing in the language**: no new type, keyword or set. The rung above the word (ADR 0034)
   gains two representations on LLVM, and ρ chooses among them from the facts, as it does today.

**Why this order**: R1 is a defect today, in every program on the layer. R2 is small: a sign layer
over operations that exist. R3 is a library, and without R1 it would leak without bound.

**Questions for hamza**:
- **Q1**: R1, storage by size class (frame, data stack, heap) with free at death and reuse for what
  escapes, rather than reference counts (L2) or regions copied per iteration (L3)? And the persistent
  permission narrowed to constants, a frozen value's view being counted fractions (§5.6)?
- **Q2**: R2, sign-magnitude Z(M), rather than two's complement fixed limbs?
- **Q3**: R3's base, 2³² over U₆₄ first, with i128 measured after?
- **Q4**: the order, R1 then R2 then R3, each with its ADR, result document and planted faults?

---

## References

- Appel, A. W. *Modern Compiler Implementation in ML*, Cambridge 1998, ch. 10 (liveness).
- Bornat, R., Calcagno, C., O'Hearn, P. and Parkinson, M. "Permission accounting in separation logic",
  POPL 2005.
- Boyland, J. "Checking interference with fractional permissions", SAS 2003.
- Boehm, H.-J. and Weiser, M. "Garbage collection in an uncooperative environment", SP&E 18(9), 1988.
- Bourbaki, N. *Algebra I*, ch. I (the symmetrization of a commutative monoid).
- Boute, R. T. "The Euclidean definition of the functions div and mod", TOPLAS 14(2), 1992.
- Brent, R. P. and Zimmermann, P. *Modern Computer Arithmetic*, Cambridge 2010.
- Calcagno, C., O'Hearn, P. and Yang, H. "Local action and abstract separation logic", LICS 2007.
- Collins, G. E. "A method for overlapping and erasure of lists", CACM 3(12), 1960.
- Gavril, F. "Algorithms for minimum coloring, maximum clique, minimum covering by cliques, and maximum
  independent set of a chordal graph", SIAM J. Comput. 1(2), 1972.
- Girard, J.-Y. "Linear logic", Theoretical Computer Science 50, 1987.
- Golumbic, M. C. *Algorithmic Graph Theory and Perfect Graphs*, Academic Press 1980.
- Granlund, T. and Montgomery, P. L. "Division by invariant integers using multiplication", PLDI 1994.
- Granlund, T. et al. *The GNU Multiple Precision Arithmetic Library*, manual, "Integer Internals".
- Grossman, D. et al. "Region-based memory management in Cyclone", PLDI 2002.
- Hack, S., Grund, D. and Goos, G. "Register allocation for programs in SSA-form", CC 2006.
- Hoare, C. A. R. "Proof of correctness of data representations", Acta Informatica 1, 1972.
- Jones, R., Hosking, A. and Moss, E. *The Garbage Collection Handbook*, CRC 2011.
- Karatsuba, A. and Ofman, Yu. "Multiplication of many-digital numbers by automatic computers",
  Doklady Akad. Nauk SSSR 145, 1962.
- Knuth, D. E. *The Art of Computer Programming*, vol. 2, 3rd ed., 1997, §4.1 and §4.3.1
  (Algorithms A, S, M and D; Theorem B).
- Lorenzen, A. and Leijen, D. "Reference counting with frame limited reuse", ICFP 2022.
- Lorenzen, A., Leijen, D. and Swierstra, W. "FP²: Fully in-place functional programming", ICFP 2023.
- McCarthy, J. "Recursive functions of symbolic expressions and their computation by machine, part I",
  CACM 3(4), 1960.
- Minamide, Y. "A functional representation of data structures with a hole", POPL 1998.
- Möller, N. and Granlund, T. "Improved division by invariant integers", IEEE TC 60(2), 2011.
- Proust, R. L. "ASAP: As Static As Possible memory management", Cambridge UCAM-CL-TR-908, 2017.
- Reinking, A., Xie, N., de Moura, L. and Leijen, D. "Perceus: garbage free reference counting with
  reuse", PLDI 2021.
- Shaikhha, A., Fitzgibbon, A., Peyton Jones, S. and Vytiniotis, D. "Destination-passing style for
  efficient memory management", FHPC 2017.
- Tofte, M. and Talpin, J.-P. "Region-based memory management", Information and Computation 132(2),
  1997.
- Ullrich, S. and de Moura, L. "Counting immutable beans: reference counting optimized for purely
  functional programming", IFL 2019.
- van Oortmerssen, W. Goose, https://github.com/aardappel/goose, `docs/goose_spec.md` (§1–§11,
  Appendices B and C), read 2026-11-08.
