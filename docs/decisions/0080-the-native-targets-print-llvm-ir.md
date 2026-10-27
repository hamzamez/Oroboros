# 0080 — The native targets print LLVM IR, stating the IR's proofs; MASM is retired at parity

Date: 2026-10-26
Status: Accepted (hamza, 2026-10-26: "do the research. let's try to reach the best decision", on the
10-24 plan's item 6a). Measured in [backend-2026-10-26](../../gauntlet/results/backend-2026-10-26.md).
Answers [ADR 0004](0004-first-targets.md)'s deferral of C; amends [ADR 0039](0039-the-targets-are-go-windows-android-and-the-browser.md)'s
Windows target's backend. Not built.

## Context

The windows target prints MASM (`ir/x86`, ADR 0032 step 3), built on 2026-08-19 to ask whether the
target-file format survives a host with no expressions; it answered that, and it was never chosen as the
native backend. The next native work is TinyRetroPad (32-bit), then ARM, Linux and macOS when a program
asks. hamza put the question: keep MASM, which needs a new printer per instruction set, or print LLVM IR,
which reaches every major architecture, weighing that it might duplicate work and reintroduce checks the
type system made unnecessary. C is a third candidate by ADR 0004's own trigger ("native desktop").

A backend is a map h_T : IR_P → L_T, correct when ⟦h_T(p)⟧_T = ⟦p⟧ for every program the IR accepts. Two
facts decide the choice of L_T:

1. **LLVM IR's undefined behaviour is what the IR proves absent.** Each precondition an LLVM operation
   the language reaches carries (no signed overflow under `nsw`, no unsigned under `nuw`, a non-zero divisor
   and no INT_MIN / −1 for `sdiv`, an address inside its object under `inbounds` and for a load) is an
   obligation the IR discharges or refuses (ADR 0019, integers.md §5, the index obligations). So h_L adds no
   check, and it can state every proof as an assertion the optimizer uses. If every obligation is proven,
   h_L(p) has no undefined behaviour, and preservation holds by LLVM's poison semantics (Lee et al., PLDI
   2017).
2. **A target language is worth what it can state.** Measured on one compiler, clang, given two printings
   of one IR_P: the LLVM IR, carrying `nsw`, `nuw` and `inbounds`, runs the merge sort at 0.35 of MASM; the
   C, which can say only "signed", at 0.55.

**Measured** (two programs in the language's own operators, one IR_P each, one driver, 11 interleaved
rounds): MASM is dominated on both. LLVM IR at `-O2` runs at 0.63 and 0.35 of MASM's time; LLVM IR with
`minsize` has 0.77 and 0.51 of MASM's code at 0.59 and 0.98 of its time. C runs at 0.63 and 0.44 under
`cl` and at 1.03 and 0.55 under clang, so its speed is one compiler's heuristics. One printed `.ll`
compiled for x86-64 and i686 Windows, ARM64 Windows, x86-64 and AArch64 Linux, ARM64 macOS and wasm32.

## Decision

1. **The native targets print LLVM IR**, a backend `ir/llvm` beside the other printers, reading IR_P
   alone (ADR 0038). The windows target is the first: triple `x86_64-pc-windows-msvc`, the Win32 API as
   `dllimport` declarations from the target files as today.
2. **The printer states the IR's proofs, and only them**, each a function of a fact the IR proved:
   - an `exact` operation is `nsw`, and `nuw` when both operands are proven non-negative;
   - an index is `getelementptr inbounds`;
   - a branch whose arms both go round the loop with pure arguments is one back edge through `select`
     (sound under poison: `select` takes none from the operand it does not choose);
   - a function is `nounwind`; a target that asks for size gets `minsize optsize`;
   - what the type system guarantees is the next to state, measured when printed: distinct buffers never
     alias (R0–R4), so `noalias`.
   A build that prints none of these flags exists, for diagnosing a proof.
3. **32-bit is a triple, not a printer**: `i686-pc-windows-msvc`, and a Win32 call is `x86_stdcallcc`. This
   decides 6c's third wall, 32-bit against x86-64, for TinyRetroPad.
4. **The toolchain is LLVM at or above 15** (the opaque-pointer syntax the printer uses) and the
   platform's linker: on Windows, Visual Studio's "C++ Clang Compiler for Windows" component beside the
   MSVC it already requires, or LLVM's release. The printed subset is small and its syntax has been stable
   since LLVM 15; LLVM promises no textual compatibility, so `cmd/check` builds with the installed version
   and reports it, and bitcode is the fallback if a version in the supported range breaks the text.
5. **MASM is retired at parity**, as the term backends were (irstep4a): `ir/x86` and `x64.oro`'s
   instruction vocabulary stay until `ir/llvm` builds and runs every windows case of the differential suite
   and the gauntlet's Windows sieve, then they go. The programs that name x86 instructions
   (`examples/native/*-win*.oro`, `lib/win/fmt.oro`) move to the language's operators.
6. **C is not the native backend.** ADR 0004 deferred it until a native desktop was wanted; one is, and
   the measurement chose LLVM IR, the layer below C in every LLVM toolchain.
7. **LLVM IR is a host like any other, and the target offers every primitive it has** (hamza, 2026-10-27:
   "because llvm ir is just another target, we will try to support every primitive it has, when you hit a
   wall or something that can't be expressed let me know, so we can decide it together"). Its signature
   Σ_LLVM, the instructions and intrinsics over its types, is declared by hand in a target file under the
   host's own names (`llvm.ctpop`, `llvm.umul.with.overflow`), as Go's and x86's are (ADR 0022, 0025); an
   operation's undefined behaviour or poison condition is its declaration's `where`, an obligation
   discharged or refused, never assumed. A primitive the language cannot yet state (a type, a form or an
   effect it lacks) is a wall, put to hamza with its algebra before anything is built for it.

## Why not

- **Keep MASM, and improve `ir/x86`.** Dominated on both axes. Closing the sort's 2.9× is reimplementing
  division by invariants (Granlund and Montgomery 1994), if-conversion and the rest of an optimizer, and
  ARM64 and i686 would each need another printer: lowering further than the target requires.
- **C.** It is the highest layer Windows provides natively, ADR 0001's first candidate. Measured, it cannot
  carry the IR's facts (0.55 against 0.35 under one compiler: no `nuw`, no `inbounds`, no per-operation
  choice, aliasing only as `restrict`), and its speed depends on which C compiler meets it (`cl` 0.63
  against clang 1.03 on one file). Its semantics is the larger set of undefined behaviours a printer must
  avoid (strict aliasing, sequencing). Lowering one layer further is necessary to transmit what the IR
  knows, which is the clause ADR 0001 leaves room for.
- **Both printers, MASM for size.** `minsize` gave smaller code than MASM on both programs.
- **LLVM bitcode.** Stable since 3.0, but a bitcode writer is a second encoder of the IR, and the text
  has been stable for the printed subset since LLVM 15. Taken if the text breaks inside the supported
  range.
- **QBE, Cranelift, MLIR.** QBE (design-direction §9's "small SSA backend") targets "amd64 (linux and
  osx), arm64, and riscv64" (its page, 2026-10-26): not Windows, the first native target; Cranelift is a Rust library with no stable textual input for an
  external producer; MLIR is LLVM's front-end infrastructure, more than a backend needs.
- **WebAssembly as the native layer.** It reaches the browser (ADR 0039's fourth target), not a native
  executable; LLVM emits it as one more triple.

## Consequences

- **A plan item: `ir/llvm`, spec first** (ir.md's printer section), at the corpus's windows parity, then
  `ir/x86` and `x64.oro` retired and their programs ported; the Windows sieve re-measured against
  hand-written MASM.
- **6b and 6c proceed against LLVM**: a window procedure is a function pointer LLVM gives the right
  calling convention (callbacks tier 2); a Win32 struct is an LLVM struct type laid out by the target's
  data layout, as MSVC lays it out (records). Both walls shrink to the language's part.
- **ARM, Linux and macOS become a triple and a target file each** when a program asks, not a printer each.
- **The repository needs clang** wherever `cmd/check` builds the windows target; the doctor (build.md §6)
  reports it.
- **Compile time** gains about 80 ms per native file.
