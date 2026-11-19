# Research: SendMessageA, a dependent product over the message number

2026-11-19. TinyRetroPad's body (assessment-2026-11-18 plan item 1) talks to its Rich Edit control
through `SendMessageA` 44 times, with 25 messages. Find, replace, the status line, printing and the
font all go through it. Each message gives wParam and lParam their own types:
- EM_EXGETSEL's lParam points to a CHARRANGE the control writes;
- EM_REPLACESEL's is a C string;
- EM_FINDTEXTEXA's is a FINDTEXTEXA, read and written;
- WM_GETTEXT's is a buffer and its size.

The declaration `winuser.SendMessageA(w ∈ HWND, m ∈ UINT, wp ∈ WPARAM, lp ∈ LPARAM)` erases all of
that into integers. This document says what the function is, and where the language stands.
Nothing is built.

## 1. What it is

SendMessageA is a **dependent function**,

    SendMessageA : Π(m : UINT). HWND × W(m) × L(m) → R(m),

with W, L and R families over the message numbers that a header's `#define`s and documentation
fix. C has no dependent types, so it takes the union of every L(m) as `LPARAM`, an integer wide
enough for a pointer. A cast at every call recovers the instance. A program that passes a
CHARRANGE's address for EM_EXGETSEL is writing the instance m = 1076 by hand.

The language has the instances already: **a family of declarations, one per message**, each the
same C function called at its constant. That is how C programs that do not want the cast write
it, Windows' own `windowsx.h` included: `Edit_GetSel`, `Edit_ReplaceSel` and the rest are
macros, one per message. Each instance is an ordinary C call with an `_Out_`, an `_In_` string or
a segment. ADR 0131 and 0132 already give those forms.

    fn ExGetSel(w ∈ windef.HWND) (cr ∈ CHARRANGE, r ∈ minwindef.LRESULT) {
        host expr call SendMessageA(w, 1076, 0, &cr) stdcall lib "user32"
    }
    fn ReplaceSel(w ∈ windef.HWND, undo ∈ minwindef.BOOL, s ∈ String) (r ∈ minwindef.LRESULT) where nulfree(s) {
        host expr call SendMessageA(w, 194, undo, s) stdcall lib "user32"
    }

## 2. The wall

**A C call's argument may not be a constant.** ADR 0131's notation is a parameter, `&x`,
`&b[lo]`, `len(s)` or `NULL`. ADR 0118 reads it as a linear term over the parameters, and the
reader refuses `1076`: *a C argument is a parameter, &x, &b[lo], len(s) or NULL*. The instance
needs one more form, the constant m. `NULL` is already a constant, the null pointer.

Algebraically, the instance is SendMessageA ∘ ⟨w, const m, …⟩. The arguments are an **affine**
map of the parameters rather than a linear one. ADR 0118's linearity was about each parameter
appearing once, so a constant is no second use of anything.

## 3. Options, for hamza

1. **A constant argument** (recommended): `call SendMessageA(w, 1076, 0, &cr)`, an integer literal
   of the parameter's C type, checked against it as an argument is. Better, the name the header
   defines, `EM_EXGETSEL`, declared as a constant of the module (`EM_EXGETSEL = 1076`). The one
   new form is the constant; every message is then a declaration of the header that defines it,
   richedit.h's and winuser.h's.
2. **A wrapper in the language**: `fn ExGetSel(w) { SendMessageA(w, 1076, 0, …) }` over the erased
   declaration. It cannot pass &cr, an address, as an LPARAM: a pointer is representation (ADR
   0130), and the language has no address of a record to give. Refused by the memory model.
3. **The erased declaration with an address cast**: the same objection, a place as an integer.

Option 1 is the only one the memory model admits.

## 4. What TinyRetroPad needs from it

| messages | lParam | form |
|---|---|---|
| EM_EXGETSEL, EM_EXSETSEL | CHARRANGE* | `&cr` out; `ref(cr)` in (ADR 0130) |
| EM_REPLACESEL | C string | `s`, `where nulfree(s)` |
| WM_GETTEXT, WM_GETTEXTLENGTH | a buffer and its size | a segment (ADR 0132) |
| EM_FINDTEXTEXA | FINDTEXTEXA*, read and written | an `inout` struct cell with a C string field (ADR 0136) |
| EM_SETCHARFORMAT | CHARFORMATW* | `ref(cf)`, an inline array field `szFaceName[32]` (ADR 0136 item 3) |
| EM_FORMATRANGE | FORMATRANGE* | `ref(fr)`, nested structs by value |
| EM_LINEINDEX, EM_GETLINECOUNT, EM_EXLINEFROMCHAR, EM_SCROLLCARET, … | integers | constants and parameters |

So once option 1 is decided, the status line, find and replace, and the edit commands are written
from the forms already built and ADR 0136's.
