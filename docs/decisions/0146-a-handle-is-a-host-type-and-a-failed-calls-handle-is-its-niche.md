# 0146 — A handle is a host type, its equality the kind's, and a failed call's handle a niche of the declaration

Date: 2026-11-20
Status: Accepted. Builds [ADR 0084](0084-an-address-is-an-integer-and-ownership-is-a-view.md)'s item 10
(P3), decided 2026-10-30, by [views-finish-research.md](../views-finish-research.md) §1, its one choice
made by hamza (2026-11-19: equality "(a)", `==` on a kind). Item 5 is the derivation's, taken under
the working rule ("when the algebra decides a question already in front of hamza, take it"), and
its "Why not" says what reversing it would cost. The plan item: hamza, 2026-11-20 ("yes, reorder";
"Go ahead always mathematical and algebraic"). **Built 2026-11-20**
([handle](../../gauntlet/results/handle-2026-11-20.md)), its struct fields excepted (Consequences).

## Context

A Win32 handle is a name the host gives a thing it holds. Today every kind is `ptr` (`set HWND =
ptr`), so a window is accepted where a file is wanted, and a failure is tested by
`llvm.is_null(h)` or `llvm.is_all_ones(h)`: tests on a pointer that any program may apply to any
pointer, and forget. TinyRetroPad's file load and save is the program: CreateFileA's failure is
INVALID_HANDLE_VALUE, (HANDLE)−1; CreateWindowExA's and GlobalAlloc's is NULL.

## Decision

1. **A handle kind is a host type, its realization C's own expansion**: windows.h writes each
   STRICT kind `DECLARE_HANDLE(HWND)`, which is `struct HWND__ *`, a pointer to an incomplete struct
   of its own. So `set HWND = host "HWND__*"`, and two kinds are one type exactly when their
   realizations are (`typedef HICON HCURSOR` is `set HCURSOR = HICON`). HANDLE is `typedef void
   *HANDLE`, so `set HANDLE = host "void*"`. A kind's carrier is a set with one operation.
2. **Its one operation is equality**, `h == g` between two handles of one kind (hamza: "a"),
   `icmp eq ptr` on LLVM, and refused between two kinds. A handle is Win32's alone: its equality is
   a host operation of the kind and carries no portability claim (Tier 2).
3. **A C pointer type is a C type of the ABI**: a host type whose realization ends in `*` is passed
   and returned as the triple's pointer (ADR 0094), so a kind crosses a C call as `ptr` does.
4. **A failed call's handle is the niche of an `Option` on the declaration's result**, the niche
   the declaration's own: `(r ∈ Option(winnt.HANDLE))` with `niche -1` (CreateFileA) or `niche NULL`
   (CreateWindowExA) in its host clause. Win32 has no niche shared by a type's every function
   (`win32.oro`), so the niche is indexed by the function, n_f ∈ P ∖ K, and its retraction
   r_f : P → Option(K), n_f ↦ none, p ↦ some(p), is ADR 0040's construction (`emit/retract.go`)
   with the declaration's niche in place of the type's.
5. **An `_In_opt_` handle parameter is `Option(K)`, none passed as NULL**: the same partial
   isomorphism read the other way, its section s : Option(K) → P, none ↦ NULL, some(h) ↦ h, with
   r ∘ s = id on Option(K) where NULL is the niche. ADR 0136 decided it for a C string that may be
   NULL (`Option(String)`, NULL its niche); a handle is the same law on another carrier. The SDK's
   SAL annotation is its evidence: a handle parameter is `Option` exactly when the prototype says
   `_In_opt_`, checked by `TestWin32DeclarationsAreTheirPrototypes`.
6. `llvm.is_null` and `llvm.is_all_ones` stay for a `ptr` that is no handle (an address, ADR 0084);
   no program tests a handle with them.

## Why not

- **`ptr` for every kind, as today**: a window where a file is wanted is accepted, and a failure is
  a test a program may forget. A kind is a type because the host made it one (`HWND__*` is not
  `HDC__*`); the C compiler refuses the mix, and so should we.
- **One `HANDLE` type for every kind** (Go's `syscall.Handle`, a uintptr): the same loss, and windows.h
  does not write it so.
- **A declared `same(h, g)` per kind instead of `==`** (the research's (b)): no change to `==`, and
  a word per kind for what C writes `==`. hamza chose (a).
- **A niche per type**, as Go's `error` has (`repr error niche …`): HANDLE's failure is −1 from
  CreateFileA and NULL from CreateEventA, so no type-wide niche exists; the sentinel is the
  function's.
- **A `null` value of each kind**, passed where `_In_opt_` allows: NULL is not a handle of the kind,
  it is its absence, and a `null` constant is a handle any expecting parameter would accept. Item 5
  makes absence a value only an `Option` holds. Reversing it costs one constant per kind and the
  check item 5 adds.
- **The section as a conversion the program writes** (`opt_handle(h)`): it is the declaration's,
  derived from its prototype, as the retraction is.

## Consequences

- Every Win32 declaration whose parameter or result is a handle names its kind (about 90
  parameters), an `_In_opt_` one `Option(K)`, and each program passing `x.null()` passes `'none`.
- A failure is eliminated by `case`, `'some(h) -> …; 'none -> …`, or `expect`, and cannot be
  forgotten: an `Option` is not a handle.
- The emitted code is the comparison the program wrote before, `icmp eq ptr h, inttoptr (i64 -1 to
  ptr)`, now derived from the declaration.
- P4 (a pointer the host keeps) and ADR 0136's struct fields read a handle field the same way: a
  struct's `_In_opt_` handle field is `Option(K)`. **Not built**: until ADR 0136's optional field, a
  struct's handle field (WNDCLASSA's instance, icon, cursor and brush, MSG's window, PRINTDLGA's) is
  the layout's `ptr`, and no program reads one as a kind.
- A result's niche is stated where a program meets the failure or the SDK marks it
  `_Ret_maybenull_`: CreateFileA, CreateWindowExA, LoadLibraryA, GetModuleHandleA, GlobalAlloc,
  GlobalFree, SetFocus, GetSysColorBrush. Every other handle result is its kind, its failure owed
  to its program (GetStdHandle's two sentinels, −1 and NULL, are a two-point niche).
