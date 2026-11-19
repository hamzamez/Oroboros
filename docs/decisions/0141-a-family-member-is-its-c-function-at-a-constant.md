# 0141 — A family's member is its C function called at a constant, typed by the erased declaration

Date: 2026-11-19
Status: Accepted (hamza, 2026-11-19, on sendmessage-research.md: "option1 seems like something the
language can do now and the name of each function is clear and is documentation", then "let's
finish the memory model … sendmessage seems to be such a program"). Builds
[ADR 0084](0084-an-address-is-an-integer-and-ownership-is-a-view.md)'s P5, decided 2026-10-30, in
[ADR 0131](0131-an-out-parameter-is-a-result.md)'s notation. **Built 2026-11-19**
([family](../../gauntlet/results/family-2026-11-19.md)).

## Context

ADR 0084 item 12 (P5): *a family indexed by a constant is one declaration per member (SendMessage's
messages, as windowsx.h's crackers); the integer carrying a pointer is the template's.* SendMessageA
is Π(m). HWND × W(m) × L(m) → R(m), which C erases to integers (sendmessage-research.md). ADR 0131
later fixed how a C call's arguments are written, `call F(x, &x, &r, &b[lo], NULL)`, and listed no
constant, so no member could be declared.

## Decision

1. **A constant among a C call's arguments makes the declaration a member of its C function's
   family**: `fn ExGetSel(w ∈ HWND) (cr ∈ CHARRANGE, r ∈ LRESULT) { host expr call SendMessageA(w,
   1076, 0, &cr) … }`, named after its message, as windowsx.h names its crackers.
2. **A member is typed by the function's erased declaration**, the one that calls the same symbol
   with its parameters in order, position by position:
   - a constant by the prototype's parameter set there, and refused outside it;
   - a pointer where the prototype takes an integer, as LPARAM, by that integer: the integer
     carrying it, converted in the call by `ptrtoint`, after its own conversion.
3. So **every member prints the erased declaration's prototype**, one `declare` for every call of
   the function, on each triple.
4. **The prototype check** reads a member against the SDK's declaration of the function it calls.

## Why not

- **One name chosen by the message's value** (sendmessage-research §5 (2)): it needs a static
  value parameter and a header defined by cases. It can be built later over the members, its arms
  naming them.
- **A wrapper in the language over the erased declaration** (§5 (3)): it needs the root region's
  addresses and the logic of views for a contract that depends on m.
- **A member's own prototype, its types from its own parameters**: two declarations of one symbol
  with different types are two `declare`s, which LLVM refuses, and on i686 a different stdcall
  frame.
- **A constant's C type written with it**, `(UINT)1076`: C's cast, which ADR 0131 keeps out; the
  prototype already says it.

## Consequences

- Rich Edit's messages are declarations of `windows/richedit` (EM_EXGETSEL, EM_EXSETSEL,
  EM_EXLINEFROMCHAR, EM_LINEINDEX, EM_GETLINECOUNT, EM_REPLACESEL), and TinyRetroPad's status line
  runs on both triples (`examples/native/status-win.oro`).
- A message's name is a comment beside its constant until a target module can declare constants.
