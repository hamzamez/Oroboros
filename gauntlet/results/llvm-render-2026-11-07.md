# llvm-render-2026-11-07: an integer above the word printed on windows

6b′ step 3, its third part (hamza, 2026-11-07: "go ahead with step 3. always mathematical and
algebraic"). Of the seven differential cases that skipped windows for `big_str`, five run there:
`big-mul`, `big-negmid`, `big-select`, `big-subdiv` and `word-wide`. The other two are walls. With
them, codecs is proven on windows without `checked:`, by per-loop widening thresholds. ADR 0095.

## The algebra

**render : ℤ → Σ\*** is the sign of n and the digits of |n|. Positional notation in base 10 is a
bijection between ℕ and the digit strings with no leading zero, by Euclidean division, so the digits
are the remainders of repeated division by 10 (printing-research §2.2). That makes render a fold into
the free monoid. Its efficient realization writes the digits into one table and makes text once.

**ι : (B₇)\* ↪ Σ\*.** The language could not make text from a table. ASCII* is the free monoid on the
128 scalars below 128, a submonoid of Σ*, and UTF-8 is the identity on it. So the map from a table of
such scalars to the string it spells is the inclusion: total, a monoid homomorphism, needing no
validation, a copy on every host. Its domain is a table whose elements are in 0..127, which the
decision proves from the table's one element interval. That is `ascii(b, lo, n)`, a word of Σ_L.
- **It observes a live buffer by its meaning.** ι(b[lo, lo+n)) is a value of Σ*, so its result holds
  no part of the buffer on any host. That is Theorem L's condition (ADR 0069), true of the name; no
  other string-valued host call gains it.
- **Why not the general map, or d.** The universal map from tables of scalars has a union for a
  domain, Scalar = [0, D7FF] ∪ [E000, 10FFFF], which one interval cannot state. The decode d
  (`os.text_of`) is ι on ASCII*, but it validates every byte and lives in `os`.

**Rendering is ℤ's at every sort**, as `<` at F64 is `fcmp` (llvm.md §3b). The value's
representation chooses the realization:
- on a host bignum, the host's numeral (Go, the JVM and JavaScript, unchanged);
- on fixed limbs, the limb library's `render`. Every seventh digit position takes c = q mod 10⁷ and
  q ← q div 10⁷ by `rem_small` and `div_small`, the one-pass remainder and quotient by a word below
  2²⁸. The six positions after it take c's digits. The field is static,
  w ≥ ⌊24n·log₁₀2⌋ + 1 by 30103/100000 > log₁₀2, rounded up to a multiple of 7, so every index is
  w − 1 − k, linear in k. The numeral is ι(b[top, w)), with `top` the least index of a non-zero digit;
- in the signed word, a term over the word's operations, `print_int`'s loop over LLVM's `sdiv` and
  `srem` (10 is neither 0 nor −1), then ι: `realize big_str = lang.render`.

On a target with no bignum, `big_str`'s argument keeps the representation its facts give it.

**Per-loop widening thresholds.** The intervals widen a loop's variable up to a threshold, and an end
descends one threshold per round. With the function's constants as every loop's thresholds, a
function that inlines render three times beside three printings has so many constants that 16 rounds
end before render's `top` reaches 0. Plain widening then sends it to −∞, and `w − top` is unproven.
codecs had the same failure (llvm64-2026-11-06), so this is the trigger CLAUDE.md named. A loop's
fixed point can stop only where its own transfer can (Blanchet et al. 2003; Lakhdar-Chaouch, Jeannet
and Girault 2011), so

    T(L) = B ∪ N(C_L) ∪ N(E_L) ∪ G_L,   N(x) = {x − 1, x, x + 1}

with B the base (0, the word's ends, U's top), C_L the constants L uses, E_L the finite ends at entry
of the facts L reads from outside, and G_L L's guards' images under its scalings. Any threshold set
is sound, since the result is checked as a post-fixpoint. E_L keeps what the function-wide set gave
by accident: an outer `n < 50` bounds an inner `k < n` at 49.

## What was built

- **ι** (`ascii`, Σ_L):
  - registered in `core/reduce.go` and `emit/target.go`;
  - realized on Go, the JVM, JavaScript and windows (`win/str.ascii`, a helper beside `concat`);
  - an observer by its meaning (`emit/observer.go`);
  - specified in strings.md §10, with an inventory row (176 words);
  - a differential case, `ascii.oro`, on four targets.
- **The limb library's `render`** (`emit/bignum.oro`, checked once per width). The theory check
  decides a host call's contract in a body (`emit/limbfunc.go`), and `LowerLimbs` lowers `big-str` on
  a limb value to it where the target has no bignum.
- **The word's term**: `lang.render` and `realize big_str = lang.render` in the LLVM layer. `ir/big.go`
  lets `big-str`'s argument keep its representation on a target with no bignum.
- **The bound's crash.** The LLVM layer's `trap_if` crashed silently with `llvm.trap`; it now crashes
  with the bound's reason, "bignum overflow: the declared range is too small", as every host's does
  (ADR 0029: one set, one outcome). `big-negmid`'s trap at 6 is then an outcome the harness reads on
  windows.
- **Per-loop thresholds** (`ir/interval.go`, `loopThresholds`).
- **The cases**: the five un-skipped, four of them given `prints: string` (the windows driver prints
  `run`'s value as an integer otherwise), and `codecs` without `checked: windows`.

## Witnesses

- **`TestASCIIsDomainIsDecided`** (cmd/build): ι of [48, 57, 48, 48] prints `0900` on windows. A
  table that may hold 200 is refused: "a table takes the declared element 0..127, but its elements
  are only known to lie in [0, 200]".
- **`TestASCIIObservesALiveBuffer`**: two ι of one live buffer, joined by `++`, print `-00`. With
  **`PlantObserverFault`** the second is refused as a use after the buffer was handed on.
- **`TestTheLimbRenderIsTheHostsNumeral`**: on Go with limbs and `PlantRenderEverywhere`, 50!
  renders as its 65 digits, and the emitted Go imports no `math/big`. With the plant off, that last
  check fails ("still converts through math/big"), so the test can tell the two paths apart.
- **`TestALimbValueRendersOnWindows`**: big-negmid's driver shape (three renderings and printings in
  one `main`) builds and prints on windows. With **`PlantLoopThresholdFault`** (the function's
  thresholds) `w − top` is refused, the failure that shipped as a trap.
- **`TestACheckedFormTrapsExactlyOutsideTheWord`**, kept true. `trap_if` now crashes through Σ_OS,
  so the IR interpreter states windows' `ExitProcess` as a call into ∅ and the reason's write as a
  value it does not observe, and reads a string constant as opaque.
- **The differential**: `ascii` on four targets; `big-mul`, `big-negmid`, `big-select`, `big-subdiv`
  on seven configurations (host and limbs on Go, JavaScript and the JVM, and windows); `word-wide`
  and `codecs` on four.

## What failed first

- **The probe of ι.** A segment of a rule passed to a host is never allocated ("a table rule that was
  never allocated"), a lowering gap. So ι takes (b, lo, n), as `WriteFile` does, which is also the
  one-copy form. A borrowed parameter (`linear`) would not take a frozen table.
- **ι was not found on two targets**: the registration looked for one argument, and the declarations
  have three.
- **A host parameter `Nat`** is above the word, and windows read it as asking for a bignum. ι's
  offset and count are word ranges, as `WriteFile`'s are.
- **The second ι of one buffer was refused on windows** only, as a use after handing on. A pure host
  call observes a buffer only when its results are scalars. The differential inlines `run` into a
  driver `main` there, and the other targets' shape never met the rule. ι's observer comes from its
  meaning.
- **The cases printed a string as an integer on windows**: four lacked `prints: string`, which no
  other target needed.
- **render's theory check carried an undecided contract**, ι's `where`. The library's other functions
  call nothing with a `where`.
- **big-negmid's trap was no outcome on windows**: `llvm.trap` says nothing, and the harness reads the
  bound's reason.
- **Three cases trapped where every operation should be proven**, at `w − top`. Confirmed by
  experiment to be the threshold rounds: 32 rounds proved all 236 operations, and was reverted.
- **word-wide asked a word value for arbitrary precision.** `big_str`'s parameter is `big`. A first
  term over ℤ was refused by ADR 0089: a realization is a term over host operations, and its `/` and
  `%` are realized by terms. It is now over `sdiv` and `srem` at the signed sort.
- **The threshold witness took four tries to fail.** Through `ir.Entry` by hand it passed with the
  fault planted, because the hand-built pipeline missed `cmd/build`'s contracts, its exit
  composition and the os layer, and so had fewer constants. It runs through `cmd/build`'s own `run`.
- **The heredoc ate backslashes four more times**, in Go string literals written through bash; each
  was redone with the Edit tool.

## What was measured

- **The cases**, each agreeing on every configuration it runs on: 50!, 65 digits in 13 limbs; the
  limb subtraction and division; a negative intermediate trapping at the same input with the same
  reason everywhere.
- **Counts**: 5,120 of 5,174 operations proven (was 5,009 of 5,063), 522 of 546 loops (was 497 of
  521). The +111 operations and +25 loops are the new and un-skipped programs, all proven: the 54
  left unproven are the same 54.
- **Emission**, 14 changes:
  - `ascii` on four targets;
  - `big-mul`, `big-negmid`, `big-select`, `big-subdiv`, `word-wide` and `big-fib` now emit on
    windows (`big-fib`'s `run` is not exported, so its file has no function, and its driver still
    meets the unbounded-ℤ wall);
  - `fact-limbs`, `big-divmod` and `render` on windows carry the bound's crash on the cold arm, its
    reason, `WriteFile` and `ExitProcess`, with the comparison per operation unchanged.

  The per-loop thresholds changed no other emitted program and lost no proof. `big-sign` is refused
  for its reason, a signed range above the word on limbs. The gauntlet is byte-identical, and
  compile time is 0.97×.
- **Not measured**: render's speed. No gauntlet program renders a big value, and printing is not
  timed.

## Not done here

- **Two walls, brought to hamza**: `big-fib` declares an unbounded ℤ, which on windows needs a
  bignum written in the language (ADR 0019's debt); `big-sign` a signed range above the word, which
  needs signed limbs (ADR 0029 decision 4, "deliberately not next").
- **The u128 cases** test `lib/num/u128`, written over Go's `math/bits` and `strconv`, so they skip
  every target but Go. Whether they count against 6b′'s "fully" is hamza's question.
- **`big_str` of a U value past 2⁶³ on windows**: the word-sort term is S's. `print_int`, a
  definition, renders U.
- **Rendering per representation on Go, the JVM and JavaScript**, where a word value is still widened
  to the host's bignum to be printed (printing's P3).
- **A segment passed to a host by value**, ν on ι, and the general map from tables of scalars.
