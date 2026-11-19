# Research: finishing the memory model (ADR 0084), each part with its program

2026-11-19. hamza: "let's finish the memory model, if programs is what you need, find them, write
them, make them up." ADR 0084 (an address is an integer, ownership a view; hamza 2026-10-30)
built its portable part (views-2026-10-30), and P2 came with ADR 0129–0132. This document takes
every part still unbuilt, derives it from what is decided, names the program that demands it, and
says what is a choice.

| part | ADR 0084 | program | state |
|---|---|---|---|
| P5, a family indexed by a constant | item 12 | TinyRetroPad's status line, `examples/native/status-win.oro` | **built** (ADR 0141, family-2026-11-19) |
| P3, a handle a host type with equality, its failure a niche | item 10 | TinyRetroPad's file load (CreateFileA's −1, GlobalAlloc's NULL) and window creation | §1 |
| P6, memory the host returns | item 13 | the persistent kind: `lib/os`'s Args (built, decoded at the boundary); the epoch: a clipboard reader, made up | §2 |
| two segments of one buffer at once, proven disjoint | item 6 | wc's tail, `copy(b, b[k:n])`, as RtlMoveMemory | §3 |
| a formula with ↦ and ∗ where no binder says it | item 6 | §3's program, and P4's protocol | §3, §4 |
| P4, a pointer the host keeps | item 11 | TinyRetroPad's find dialog, FindTextA keeping FINDREPLACEA | §4 |
| a segment moved out as a linear result | item 7 | a parser handing back the rest of its input | §5 |
| the root region | item 5 | none in the plan: Tier 2 | §6 |

## 1. P3: a handle is a host type with equality, and its failure value a niche

**What it is.** A handle is a name the host gives a thing it holds. The program has no view of
the thing, so its only operation is equality, and its kinds are the host's own newtypes.
windows.h writes each STRICT kind as `DECLARE_HANDLE(HWND)`, which expands to `struct HWND__ *`,
a pointer to an incomplete struct of its own: distinct per kind, and comparable.

**Derivation of the spelling.** A target already writes a host type as its realization,
`set big = host "*big.Int"`, and two host types are one exactly when their realizations are one
(`SameHostType`). So C's own expansion is the handle's realization:

    set HWND = host "HWND__*"
    set HDC = host "HDC__*"
    set HCURSOR = HICON          // typedef HICON HCURSOR: one type, as in C

No word is added (pointers-research §10: "P1, P3, P5 and P6 add no construct"). The LLVM printer
spells any `T*` realization as `ptr`. **No choice here: the derivation takes C's expansion.**

**Equality**: `h == g` between two handles of one kind, `icmp eq ptr`, and refused between two
kinds. This widens the language's `==` past integers, which the language reserves for what every
target agrees on. A handle is Win32's alone, so its equality is a host operation and carries no
portability claim (Tier 2), as `go.==` does.
**A choice**:
- (a) `==` on a handle kind (what C writes);
- (b) a declared `same(h, g)` per kind (no change to `==`).

The derivation leans to (a): the kind's equality is the type's (ADR 0084 item 10's "with
equality"). **Decided (hamza, 2026-11-19): (a).**

**The niche.** A failure is a value of a marked sum (ADR 0040). CreateFileA's failure is
INVALID_HANDLE_VALUE, (HANDLE)−1. GlobalAlloc's and CreateWindowExA's is NULL. Win32 has no niche
shared by every function (`win32.oro`), so each goes on its declaration's result:
`(r ∈ Option(winnt.HANDLE))`, with the declaration stating its absent point, as a type states
its own (`repr error niche host expr "%s == nil"`):

    fn CreateFileA(…) (r ∈ Option(winnt.HANDLE)) where nulfree(p) {
        host expr call CreateFileA stdcall lib "kernel32" niche -1
    }

The program writes `case fileapi.CreateFileA(…) { 'some(h) -> …; 'none -> … }`. That replaces
`lib/os`'s `llvm.is_all_ones(h)`, a test on a `ptr` that any program can apply to any pointer.
**Built for Go only** (`emit/retract.go`, niche-2026-10-06). On LLVM it is owed: the sum's tag is
the comparison with the niche at the call, and the payload the value.

**Program**: TinyRetroPad's LoadStartupFile and SaveFile:
- CreateFileA, `'none` its −1;
- GetFileSize, 0xFFFFFFFF;
- ReadFile and WriteFile into a buffer of the language's, since GlobalAlloc's GMEM_FIXED memory is
  a buffer (P6);
- CloseHandle.

## 2. P6: memory the host returns

pointers-research §8 P6 gives three kinds:
- **persistent** (GetCommandLineA): the host's string, entered by a total decode at the boundary
  (ADR 0030). *Built in the language*: `lib/os`'s Args copies its bytes (lstrlenA, lstrcpynA) and
  decodes them, and the program never views the host's memory;
- **owned** (GlobalAlloc with GMEM_FIXED): *not offered*, since it is a buffer, and the printer
  chooses its allocator;
- **lent for an epoch** (GlobalLock … GlobalUnlock): borrow-research's `lent`, a borrow a later
  call ends, waiting on its trigger.

The epoch is the one left. Its program, made up because TinyRetroPad uses WM_COPY and WM_PASTE
and never locks memory itself: **a clipboard reader**, OpenClipboard, GetClipboardData(CF_TEXT),
GlobalLock, its C string decoded, GlobalUnlock, CloseClipboard.

The derivation: GlobalLock's pointer is a view of the global object, lent from the lock until the
unlock. A program that uses it after GlobalUnlock reads freed memory. So the view is a region
whose extent two calls bound. That is exactly borrow-research's question (a borrow a later call
ends), researched and not decided.
**Researched further** in [scopeforms-research.md](scopeforms-research.md), at hamza's question
(a `keyword options { block }` form, a macro): the epoch's soundness is on the declarations, and the
scope form is a trailing block over a definition. **A choice for hamza** (borrow-research's options):
- the epoch as a scope, `lock(h) { p -> … }`, the unlock at its end;
- the epoch as a linear token GlobalLock gives and GlobalUnlock consumes.

The derivation leans to the scope, the `var` scope's own rule. That is how ADR 0133 settled the
window procedure's extent.

## 3. Two segments of one buffer at once, and the formula where no binder says it

**What it is.** A segment `b[lo:hi]` is b's function restricted (ADR 0074), and lent it is a view
of [lo, hi) (ADR 0084 item 6: the splitting law, b ↦ v ⊣⊢ b[0:k] ↦ v₁ ∗ b[k:n] ↦ v₂). Two segments
of one buffer may be lent to one call at once when their ranges are disjoint: the separating
conjunction ∗ is exactly that.

**Program**: wc's tail. After a chunk, its last t < 4 bytes move to the front:
`copy(b[0:t], b[k:k + t])`. Today wc writes it as three guarded stores. As RtlMoveMemory(dst, src,
n), two segments of one buffer, `b[0:t]` written and `b[k:k+t]` read, are disjoint when t ≤ k,
which the guard proves. memmove allows overlap; `RtlCopyMemory` (memcpy) needs the disjointness.

**The derivation.** The lend of two segments is legal iff the decision proves [lo₁, hi₁) ∩
[lo₂, hi₂) = ∅, an obligation like any other, hi₁ ≤ lo₂ ∨ hi₂ ≤ lo₁. It is a disjunction, decided
by proving one side. No formula is written: the binders (two `linear` segment parameters) say it.
ADR 0084 item 6 keeps the formula for what no binder says. **No choice.**

**The formula ↦ ∗ itself** is needed only where no binder says it: contents promised (an `ensure`
about a buffer's elements, CLAUDE.md's owed "an invariant over a buffer's elements"), a protocol
across calls (§4), a split kept across calls. Its first program is P4's protocol (§4), so it is
built there, if the derivation needs it.

## 4. P4: a pointer the host keeps

**What it is.** FindTextA(&fr) shows a modeless find dialog, and the host keeps &fr: it writes the
find text into fr's buffer as the user types, and sends the window the registered message
FINDMSGSTRING, with &fr as lParam, at each Find Next. The pointer outlives the call.
pointers-research §8 P4 gave D1 (a region over the message loop), D2 (an epoch) and D3 (a
process), and said "decide it in 6d".

**6d decided the window procedure since** (ADR 0133). Its state is a `var` cell in main's scope,
the λ at WNDCLASSA's field captures it, and the cell is its static slot (Theorem T's global), live
for the scope, which holds the message loop. That is D1. So FINDREPLACEA is a `var` struct cell in
the same scope:
- FindTextA(&fr) lends the cell's address for the cell's scope, not for one call (the view moved to
  the host, ADR 0084 item 11);
- the window procedure reads fr's fields (Flags, the find text) when FINDMSGSTRING arrives;
- the dialog's end (FR_DIALOGTERM) returns the view.

**The derivation.** A pointer the host keeps for the extent of a scope is that scope's slot. The
slot is what ADR 0133 built, so the host's keeping is sound exactly when the address is the
slot's and the scope outlives every message. ADR 0133 proved that by the message loop sitting in
the scope. What is new: a struct cell whose address goes to a call that keeps it, marked on the
declaration as the host's keeping (`_Inout_` held past the call; SAL has no annotation for it, so
the declaration says it). A program cannot pass a non-slot cell to it: refused.
**A choice for hamza**: the word on the declaration for "the host keeps this address" (`kept`).
Or none: a FINDREPLACEA parameter is always kept by FindTextA and ReplaceTextA, so the loader may
know it from the function. The derivation says a keeping is a mode, as `inout` is (ADR 0135: a
mode is what a program observes of a binder's relation to the caller's place). So a mode word.
**Decided (hamza, 2026-11-19): `kept`**, since the repo has the protocol (pointers-research §14.18,
callbacks-research §3.7: a move to the host, a lend back per FINDMSGSTRING, a return at
FR_DIALOGTERM) and no spelling for the declaration's side.

**Program**: TinyRetroPad's find and replace, with P5's members (EM_FINDTEXTEXA, EM_EXSETSEL,
EM_REPLACESEL) over the control.

## 5. A segment moved out as a linear result

views-2026-10-30: "a segment as a moved result waits for a program". **Program, made up**: a
tokenizer that hands back the rest of its input, `fn next(b ∈ Array(0..255) linear) (tok ∈ Token,
rest' ∈ …)`. It is the lent buffer handed back (ADR 0140) restricted to a segment. The derivation:
a primed result that is a segment of the parameter, `(b'[k:] …)`. It waits on a program that is not
made up, since no program in the corpus or TinyRetroPad needs it.

## 6. The root region

The whole address space, origin 0, belongs to a target that offers it (ADR 0084 item 5, Tier 2),
where `ptrtoint` and `inttoptr` are coordinates. No program in the plan needs an absolute address.
P5's integer carrying a pointer is a conversion at a call, not an address the program holds.
Waits on an OS target's program (a page table, a memory-mapped device).

## 7. Order

1. P5, built.
2. P3, its spelling taken from the derivation, with (a) or (b) for equality to hamza; the niche
   on LLVM; TinyRetroPad's file load and save.
3. Two segments at once (§3), no choice: wc's tail by RtlMoveMemory.
4. P4 (§4), after hamza's word for the keeping mode: find and replace.
5. P6's epoch (§2), after hamza's choice of scope or token: the clipboard reader.
6. §5 and §6 wait on programs.
