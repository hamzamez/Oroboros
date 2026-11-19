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

## 5. In our type system (hamza, 2026-11-19: "this is a typing problem, isn't our type system powerful enough to express this?")

All three readings below denote the same object, the instances of Π(m), and differ in **where the
table of instances lives** and **what proves a call**.

**What the language already has.**
- **Two levels** (closures-direction.md): a static level, unrestricted and higher-order, erased by
  staging. A Π over a *static* index is exactly what staging computes away.
- **Sets are static values** (ADR 0063): a declaration's head binds static parameters,
  instantiation is application reduced by δ to `f#[T̄]`, and the instance is named by its argument.
- **Telescopes** (ADR 0062): a binder's set may name the binders before it,
  `fn get(a ∈ Array(Int), i ∈ 0..len(a)-1)`.
- **A definition over sets is a λ returning a λ** (ADR 0066).

So W(m), L(m), R(m) are set-valued functions of m. A singleton {m} is a set, and therefore a
static value. Π(m) is in reach of the static level.

**What it lacks for each reading.**

| | the index is | the table lives in | proves a call | not built |
|---|---|---|---|---|
| (1) a declaration per message, `ExGetSel(w)` | a name | the target file, one declaration each | the generator, each against the prototype and the message's documentation | a constant among a C call's arguments |
| (2) one name, `SendMessageA(hEdit, EM_EXGETSEL, 0)` | a static value | the target file, a case over the message | δ selecting the instance at staging | a static *value* argument (ADR 0063 binds sets: the singleton {m}, inferred from the literal, its item 7), and a header defined by cases, since the instances differ in shape (EM_EXGETSEL has an out-result, EM_REPLACESEL a C string), not only in their sets |
| (3) the program's wrapper over the erased declaration | a value in the program | the declaration's contract, a formula over m | a proof at each call: `m = EM_EXGETSEL ⇒` lp is the address of a fresh CHARRANGE region the host writes | the root region's addresses as integers (ADR 0084 item 5, Tier 2, `ptrtoint`, not offered on Windows yet) and the logic of views over a dependent contract (ADR 0084's P4 protocols) |

**The derivation.** (2) is (1) with its names replaced by the message's value. Its table holds the
same instances, written as the arms of a case, and δ picks the arm, as a target picks a realization
by the facts (ADR 0088). So (2) can be built later *over* (1): `SendMessageA[EM_EXGETSEL] =
ExGetSel`, the case's arms naming (1)'s declarations, and no program written against (1) changes.

(3) is the type C erases, stated in the logic. It is the truest reading, since it is SendMessageA's
real contract, but each call still owes the per-message facts, now as proof obligations, and it
needs the most machinery (the root region and views). It moves the table from the declaration into
the contract, which is where it is hardest to check.

**Recommendation**: (1) now, buildable with one form. (2) as the dependent reading on top of (1)
when static value parameters exist. (3) when the logic of views does.

## 6. The variadic of places (fmt.Scan), and `xs...`

A variadic is f : X × T* → B (ADR 0046). A call writes the list as its trailing arguments, or
spreads a table, `f(xs...)` (G2): spread : Array(T) → T*, the identity on words, defined because
a table of values is a value.

`fmt.Sscan`'s list is not of values: Sscan : X × (Place T)* → B. A place is a region with a
permission, and the pair (region, address) is never a value (ADR 0084 item 1). So there is no table
of places, and (Place T)* exists only as n binders at a call, a telescope of `xᵢ ∈ T inout`.

The four spellings, read against that:
- **(d) `a ∈ Array(go.Value inout)`** names a set of tables of places. No such value exists, so the
  set is uninhabited. `Sscan(s, xs...)` could be written but never satisfied, and the declaration is
  callable only by the trailing form the set does not describe. It uses a set to say what is a mode.
- **(c′) `a ∈ go.Value inout` with `variadic`**, every variadic written as its one trailing binder.
  The declaration names the binder, and `xs...` fills it from a table where its elements are values:
  `Println(xs...)` is accepted, and `Sscan(s, xs...)` is refused, since a table of values is no
  list of places. The spread and the mode meet at one rule.
- (c), the same for cells only, and (b), the mode on the whole table, as before.

So `...` decides it in favour of (c′): it is the bridge between a table and a list of binders, and
it is defined only where the list's elements are values. (d) is consistent only as notation, read
not as the set it writes. hamza prefers (d), with (c′) "good as well": the choice is his, and this
is the derivation's reading of it.
