# Research: callbacks, or a window procedure as a handler with a state

2026-11-18. Item 6 of [assessment-2026-11-17](assessment-2026-11-17.md) §5, TinyRetroPad 6d, its
second wall. hamza: "start the callbacks research, always mathematical and algebraic". **Research, no
decision; nothing built.** The measurements in §2 were made on this machine with clang 23.1.4 and
Windows' own user32; TinyRetroPad's source was read as data (commit `42f4c6c`, never built or run).

**The short version:**
- **The repository already holds the direction.** concurrency-research §2.1 and ADR 0081 say a window
  procedure is a Mealy machine, δ : S × I → O × S, and that a UI process's lowering is "the host's
  own loop, with our step as its handler". [callbacks.md](spec/callbacks.md) (tier 2: a closed
  function passed by name, the API's context word its environment) and ADR 0053 (the arrow is a
  claim on a host's callback parameter) say how a callback crosses. pointers-research §14.18 gives
  the Find dialog's struct as a protocol of views (P4).
- **What none of them settled** is where the state lives when the host, not the program, calls the
  step, and what happens when the step calls the host and the host calls the step again.
- **Theorem N (measured, §2.2): on one thread every invocation of a callback is nested inside a host
  call the program made.** So invocations form a stack, and one state, lent down the stack, is what
  the program's globals were.
- **The algebra (§3)**: the window procedure is a *parameterised deep handler* of the host's message
  effect (Plotkin and Pretnar 2009, 2013), its parameter the state. Lending the state to a host call
  that may re-enter is ADR 0048's cell passed by reference, and the handler's state is found through
  a slot, as evidence passing finds a handler (Xie et al. 2020), because the C ABI carries no
  evidence.
- **Recommended: B (§6)**, a stateful step `fn step(s ∈ S, m ∈ I) (S × O)` installed by name with an
  initial state. The compiler makes the C-ABI function (the trampoline) and the slot, and threads the
  state through every host call that may dispatch. ADR 0081's typed process (C) is B's surface,
  later. Five decisions for hamza are in §7.

## 0. What was decided before, and what was not

| document | what it says | status |
|---|---|---|
| [callbacks.md](spec/callbacks.md) | three tiers. Tier 1, a λ at the call site the host closes over. **Tier 2, a top-level function with no free variables passed by name**, "and the Win32 API is designed for exactly this": the context word (`LPARAM`, `lpParameter`) *is* the environment, closure conversion done by hand. Tier 3, an escaping closure, refused. Two hazards: a callback must not capture a buffer, and a host closure allocates | specified, not built |
| [ADR 0053](decisions/0053-the-arrow-is-a-claim-on-a-host-parameter.md), arrow-research | a host's callback parameter states its shape as a claim, a λ's header without a body, Π(x̄ ∈ P). R; fitting is contravariant in the domain, covariant in the result | decided, not built |
| [ADR 0081](decisions/0081-concurrency-is-typed-processes.md), concurrency-research §2.1, §4 | a process is a Mealy machine, a coalgebra S → (O × S)^I; **"a Windows window procedure … whose state lives in the window"**; a UI process's lowering is **the host's own loop, our step its handler**; a supervisor is a handler of the exception effect (Plotkin and Pretnar) | the direction, not built |
| [ADR 0084](decisions/0084-an-address-is-an-integer-and-ownership-is-a-view.md) P4, pointers-research §14.13, §14.18 | a code address is frozen, a value; a pointer the host keeps is a protocol of views: FindTextA moves FINDREPLACE's view to the host, the window procedure is lent it per message, the dialog's end returns it. "Declared with 6d's window procedure" | decided, not built |
| ADR 0128 | WNDCLASSA's `lpfnWndProc ∈ ptr = 0` until callbacks | built (structs-2026-11-17 §4) |

## 1. The issue

TinyRetroPad's callbacks, read from its source:

| callback | how it reaches the host | its messages | what it calls |
|---|---|---|---|
| `WndProc` | `WNDCLASSA.lpfnWndProc`, then `RegisterClassA` | `WM_CREATE`, `WM_PAINT`, `WM_COMMAND`, `WM_NOTIFY`, `WM_SIZE`, `WM_SYSCOMMAND`, `WM_DESTROY`; the registered `FINDMSGSTRING`; the rest to `DefWindowProcA` | `SendMessageA` ×16, `SetWindowPos` ×3, `CreateWindowExA` ×2, `DestroyWindow`, `MessageBoxA`, `DefWindowProcA`, the common dialogs, `DialogBoxIndirectParamA`, `FindTextA`, `ReplaceTextA` |
| `GoToProc` | `DialogBoxIndirectParamA`'s `lpDialogFunc`, a `DLGPROC` | `WM_INITDIALOG`, `WM_COMMAND`: TRUE or FALSE | `GetDlgItemInt`, `EndDialog` |

**Its state is global**: 13 words (`hMain`, `hEdit`, `hStatus`, `hFindDlg`, `uFindMsg`, `fDirty`,
`fWrap`, `fStatus`, `fLineNum`, `fDark`, `hInst`, …) and seven buffers (the startup path, the title,
the date and time, the find and replace texts, the status line). The language has no globals
(ADR 0018, and pointers-research §13's "Shen's globals": a view everyone holds is aliasing
everywhere). So the question is not how to pass a function to the host, which tier 2 answers. It is
how the host's calls of that function reach a state the program owns.

## 2. Measured

### 2.1 How Win32 hands a callback its environment

| API | callback | its context word, the closure's environment |
|---|---|---|
| `EnumWindows(f, lParam)`, `CreateThread(…, f, lpParameter, …)` | `WNDENUMPROC`, `LPTHREAD_START_ROUTINE` | given on every call: closure conversion exactly |
| `DialogBoxParamA(…, f, dwInitParam)` | `DLGPROC` | given once, in `WM_INITDIALOG`'s lParam, then kept by the program (`DWLP_USER`) |
| `CreateWindowExA(…, lpParam)` | `WNDPROC` | given once, in `WM_NCCREATE`'s `CREATESTRUCT.lpCreateParams`, then kept (`GWLP_USERDATA`, `cbWndExtra`) |
| `RegisterClassA` | `WNDPROC` | **none per call**: the procedure gets `(hwnd, msg, wParam, lParam)` and nothing else |

### 2.2 Where a window procedure is called: Theorem N's premise

`scratchpad/cb/nest.c`, my own program: a window procedure that records which of the program's host
calls was in progress when it was invoked, and how deep invocations nest. It creates, shows, moves,
sends to, posts to, pumps and destroys one window:

| invocations | outside any host call of the program | deepest nesting |
|---:|---:|---:|
| 32 | **0** | 2 |

Counting distinct messages per call, which is what the program records:
- **CreateWindowExA** invokes it with 4 messages (`WM_GETMINMAXINFO`, `WM_NCCREATE`,
  `WM_NCCALCSIZE`, `WM_CREATE`). That is before any message loop exists, so the state must exist
  before the window does.
- **The other calls that invoked it**: `ShowWindow` with 7 messages, `SetWindowPos` 5, `SendMessageA`
  1, `DispatchMessageA` 3, `DestroyWindow` 5.
- **Re-entry, depth 2**: `DefWindowProcA`, called from inside the procedure, invoked it again with 4
  messages, and a `SendMessageA` from inside it with 1.

This is Win32's documented rule: a thread's window procedures run only when that thread calls a
function that sends, retrieves or dispatches messages; Windows does not interrupt a thread to deliver
one.

**Theorem N (nesting).** On one thread, if every invocation of a callback is inside a host call the
program made, then the invocations and the program's host calls form a properly nested tree: an
invocation begins and ends within the call that made it, and calls made inside an invocation end
before it does.

*Proof.* A thread runs one stack. The host can run the callback only on that stack, and only while
it holds control, which is inside a call the program made (the premise, measured above). A call
returns only after everything it ran has returned. ∎

So at any moment exactly one activation, the innermost, is running, and the state is used by one
activation at a time. **That is linearity.** The globals were a linear state that C could not say
was linear.

## 3. The algebra

### 3.1 A callback is a closure converted by hand

A function value with an environment is ∃E. (E × A → R) × E (Minamide, Morrisett and Harper,
"Typed closure conversion", POPL 1996). Win32's callbacks are that pair taken apart: the code is
the function pointer, a closed function, which is tier 2, and E travels in the context word. A
closed function needs no environment at run time, so it is a code address, frozen and a value
(pointers-research §14.13), and printing one is printing a function with the C convention (§4).

### 3.2 The window procedure is a Mealy machine with host effects

Its step is δ : S × I → H(S × O): I = HWND × UINT × WPARAM × LPARAM, O = LRESULT, and H the host's
effects, since δ calls user32. concurrency-research's δ : S × I → O × S is δ with H the identity.
The behaviour is the coalgebra S → H(O × S)^I, and two window procedures are equal when bisimilar
(Rutten 2000): a refactoring of the state is free as long as every message gets the same answer and
the same effects.

### 3.3 Re-entry is the state threaded through the host: Theorem T

A host call h : X → Y made inside δ may run δ again (Theorem N's nesting). If δ's state is a value
threaded in program order, the inner activation must see the state as the outer one left it before
the call, and the outer one must continue with what the inner one left. So h is, observably,

  h_S : S × X → S × Y,

the state monad's lifting of h (Moggi 1991): S lent into the call and returned by it.

**Theorem T.** Under Theorem N, a program whose step threads S, and lends S to every host call that
may dispatch, has the meaning of the C program whose S is one global, read and written in place.
*Proof.*
- **The global's history is the sequence of activations.** By Theorem N the activations nest, so that
  sequence is a depth-first walk of the tree.
- **The threaded program passes that history in the same order.** The state is handed into each
  dispatching call and taken out of its return, and each nested activation receives the state its
  caller lent and returns the state it leaves.
- **Lending is ADR 0084's Theorem P at each level**: a view lent for one call is a value passed and a
  result returned.
- By induction on the tree's depth, the two histories are equal. ∎

**The bug it excludes** is the one C makes easy. A C window procedure caches a field of the state in
a local, calls `SendMessage`, which changes the state underneath, and uses the stale local. Threaded,
the local is the old state's value by meaning, and the new state is a new binder, so the reuse is
visible in the source.

### 3.4 The handler's reading: a parameterised deep handler of the message effect

Treat the host's sending of a message as an algebraic operation, `msg : I ⇝ O`, which a host call may
perform any number of times. The window procedure is then a handler for `msg`, and three standard
properties of handlers are exactly Win32's (Plotkin and Pretnar, "Handlers of algebraic effects",
ESOP 2009, and LMCS 2013):
- **deep**: the handler is in place for the operations its own clauses perform. That is re-entry: a
  `SendMessage` inside the procedure is handled by the procedure;
- **parameterised**: a handler with a parameter p, h_p(msg(i, k)) = let (p′, o) = δ(p, i) in h_{p′}(k o).
  That is the state, threaded from one operation to the next;
- **installed for an extent**: the UI process's lifetime, from before `CreateWindowExA` to after the
  loop ends.

ADR 0081 already reads a supervisor as a handler of the exception effect. So one algebra covers
failure and callbacks: **a process's step and a window procedure are both handlers, and a UI
process is the handler whose operations the host performs.**

### 3.5 How the handler finds its state: evidence through a slot

An implementation of handlers passes evidence: a pointer to the handler, its parameter included, to
every operation, so performing one is a call through it ("Effect handlers, evidently", Xie,
Brachthäuser, Hillerström, Schuster and Leijen, ICFP 2020). Win32's C ABI carries no evidence, since
the procedure gets four words. So the evidence goes where the operation can find it without being
passed it: **a slot**, the place an implementation keeps its context when the calling convention
cannot carry it.
- **The slot holds the state's region.** The trampoline, the C function the host calls, reads it,
  runs the step, and writes the result back.
- **Lending S to a dispatching call writes the slot before the call and reads it after.** That is
  ADR 0048's cell through a reference: the reference is the slot, and the call may write it.
- **The program never sees an address.** The region's address is the compiler's, as a cell's slot
  is, so no `inttoptr` and no root region (ADR 0084 item 5) are needed, unlike `GWLP_USERDATA`.

**Per process, or per window?** The slot holds the UI process's state. Two windows of one class in
one process share it, and the step tells them apart by `hwnd`, as one Mealy machine whose input
names the window. A state per window, ∏ over windows, is the per-window slot, `GWLP_USERDATA`,
an address in the root region, and is waiting on a program with two windows that do not share
state. TinyRetroPad has one main window, and its dialogs have their own procedures.

### 3.6 Which host calls dispatch: an effect on declarations

Theorem T needs to know which calls may dispatch, and lending to a call that never dispatches costs
one store and one load, which LLVM removes when nothing reads the slot. So the classification is an
effect, `dispatches`, stated per header as data:
- **dispatching**: user32, comdlg32, shell32's dialogs;
- **not dispatching**: kernel32's files, memory and processes, and gdi32's drawing, which take no
  window. Win32's rule says only message functions dispatch, and the measurement of §2.2 is its
  witness for the calls it made.

A call wrongly declared non-dispatching that does dispatch gives the inner activation a stale state.
That is the planted fault §5 names, and why the default inside a UI process is "dispatches" unless
the header says otherwise.

### 3.7 DLGPROC, and the Find dialog's struct

- **A dialog procedure** is the same handler with O = INT_PTR, TRUE or FALSE, installed for the
  modal loop's extent by `DialogBoxIndirectParamA`, itself a dispatching call. GoToProc's state is
  nothing; it answers through `EndDialog`'s result.
- **FINDREPLACE (P4)** lives in the process's state, its view moved to the host by `FindTextA` and
  lent back by each `FINDMSGSTRING` message, whose lParam is its address. Because it is in the
  state's region, the step reads it there, and the lParam is compared, not dereferenced. While it is
  moved, the program may not write it: a typing rule on the state's component, P4's.

## 4. The targets

- **The C function** the host calls is printed with the prototype's convention: `CALLBACK` is
  `__stdcall`, so `x86_stdcallcc` on i686 and the C convention on x86-64 (ADR 0094). Its parameters
  are the prototype's C types, assumed in their whole sets as an export's are (ADR 0028), and its
  result is the C type's. Inside it, a crash is `abandon` (ADR 0090), which exits without unwinding
  through user32's frames.
- **Tier 2 on the other hosts** (callbacks.md): a function value on Go, a function on JavaScript, a
  static method behind a functional interface on the JVM. Passing a closed function by name is
  portable.
- **The UI process is portable in shape** (concurrency-research §4): Android's `Looper` and
  `Handler.handleMessage`, and the browser's event loop with its listeners, are the same handler with
  a state, the host's loop dispatching. Each realization is its host's; Win32's is the trampoline
  and the slot.

## 5. Planted faults, before any witness is trusted

| rule | the fault | what must fail |
|---|---|---|
| Theorem T | a dispatching call not lent the state | a program whose step changes S inside a `SendMessage` to itself, then reads it: the stale value prints |
| the convention | the trampoline printed `cdecl` on i686 | the stack unbalanced after the first message: the program crashes |
| the slot | the trampoline reads a fresh zeroed region | the step sees the initial state on every message |
| the extent | the state installed after `CreateWindowExA` | `WM_CREATE`'s step runs with no state; refused at compile time, not crashing |
| P4 | a write to FINDREPLACE while the dialog holds it | refused, naming the protocol |

## 6. Candidates

- **A. Tier 2 alone, the state by hand.** The program passes `wndproc(h, m, w, l)` by name and keeps
  its state at `GWLP_USERDATA`, through `SetWindowLongPtrA` and `GetWindowLongPtrA`.
  - *Against*: an address the program reads back, `inttoptr`, the root region and a view protocol
    written by hand, which is C's idiom with C's bug of §3.3 available.
- **B. A stateful step, installed by name (recommended).** The program writes
  `fn step(s ∈ S, h ∈ HWND, m ∈ UINT, w ∈ WPARAM, l ∈ LPARAM) (S × LRESULT)`, a first-order function
  whose host calls are threaded, and installs it with an initial state for an extent around
  `CreateWindowExA` and the loop. The compiler makes the trampoline and the slot, and lends the state
  to every dispatching call in the extent. It is §3.4's handler with §3.5's evidence, and Theorem T
  is its correctness.
- **C. ADR 0081's process.** A `process` declaration with a state, a message variant and a step,
  realized on Windows as B.
  - Its messages are typed by P5's per-message declarations (`windowsx.h`'s crackers): `WM_SIZE(w, h)`
    rather than four words.
  - *Against, now*: C2 is not built, and its surface is open (ADR 0081 item 6).
  - B is C's mechanism, and C is B's surface when a second process kind or the typed messages ask.
- **D. Tier 3, a closure over the state.** Refused, as callbacks.md and ADR 0018 refuse it: the
  closure would capture a linear state.

## 7. Decisions for hamza

1. **B as the mechanism**, with C its surface later. Recommended.
2. **The state per UI process**, one slot, with per-window state waiting on a program that needs
   it. Recommended.
3. **`dispatches` as an effect on host declarations, per header**, the default inside a UI process
   being "dispatches". Recommended.
4. **The spelling of the installation.** Candidates:
   - `ui(s₀, step) { … }`, a scope;
   - an initial state given at the field, `wc with { lpfnWndProc : step }` with s₀ elsewhere;
   - a definition attribute.

   I recommend a scope, because the extent is the handler's and must begin before `CreateWindowExA`.
   The scope's word is for hamza.
5. **The callback's claim** (ADR 0053): the header declares `set WNDPROC = fn(h ∈ HWND, m ∈ UINT,
   w ∈ WPARAM, l ∈ LPARAM) (r ∈ LRESULT)`, the arrow as a claim, and the step fits it with S added.
   Recommended, since ADR 0053 already decided it.

## 8. Not here

- **Tier 1** (a λ at the call site, Go's `sort.Slice`) and **tier 3**.
- **Threads** (`CreateThread`), whose callback is a process's spawn (ADR 0081).
- **Android's and the browser's realizations**, each its own step.
- **Typed messages** (P5), C's surface.
- **Per-window state.**
- **Hooks in the common dialogs.**

## Paid and bought (this research)

- **Compiler, emission, proofs**: unchanged; nothing built.
- **The measurement**: `scratchpad/cb/nest.c`, one program and one run, under an hour, with the
  repository's documents read first.
