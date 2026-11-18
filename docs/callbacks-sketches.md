# Callbacks, sketched: every decision as programs

2026-11-18. hamza, on [callbacks-research.md](callbacks-research.md) (tier 2, the window procedure)
and [callbacks-tier1-research.md](callbacks-tier1-research.md) (tier 1): "I agree in principle. what
I need is to see it, sketch it for me in programs. same for every decision I have to make, show me
my options."

Every program here is a sketch: nothing in it is built. Words no ADR has decided are marked
*(word open)*. Win32 calls are written as ADR 0131 and 0132 now build them. `GetMessageA`,
`DispatchMessageA` and `MSG` are not declared yet; they are TinyRetroPad's 6d walls and are written
as they will be.

Sketching surfaced one question the tier 2 research left open, how the step's text names the state
after a call that re-enters it. It is §1.4b.

---

# Part 1. Tier 2: the window procedure

The program throughout is a window that counts clicks and closes when it is destroyed. TinyRetroPad's
procedure is the same shape with more messages.

## 1.1 The mechanism (research decision 1)

**The question**: who holds the window's state between messages, and how does the step reach it?

**Option A: tier 2 alone, the state by hand.** The procedure is a closed function passed by name.
The state lives where C keeps it, at `GWLP_USERDATA`, as an address the program stores and reads
back.

```
fn wndproc(h ∈ windef.HWND, m ∈ minwindef.UINT, w ∈ minwindef.WPARAM, l ∈ minwindef.LPARAM) (minwindef.LRESULT) {
    p = winuser.GetWindowLongPtrA(h, -21)            // GWLP_USERDATA: an integer that is an address
    s = view(p, S)                                    // (word open) an address read as a state
    case {
        m == 513 -> { store(p, s with { clicks : s.clicks + 1 }); 0 }   // (word open) written back by hand
        else -> winuser.DefWindowProcA(h, m, w, l)
    }
}
```

What it costs: an address the program reads back (`inttoptr`, the root region, ADR 0084 item 5), a
view protocol written by hand, and C's bug available. The program can read `s`, call `SendMessageA`,
which changes the state at `p`, and go on using the stale `s`.

**Option B: a stateful step, installed by name (recommended).** The step is a function of the state
and the message, returning the new state and the answer. It is pure in the state. The compiler
writes the C function the host calls (the trampoline) and the slot it finds the state in.

```
set S = record { clicks ∈ 0..1000 }

fn step(s ∈ S, h ∈ windef.HWND, m ∈ minwindef.UINT, w ∈ minwindef.WPARAM, l ∈ minwindef.LPARAM) (S × minwindef.LRESULT) {
    case {
        m == 513 && s.clicks < 1000 -> {             // WM_LBUTTONDOWN
            ignore(winuser.InvalidateRect(h, llvm.null(), 1))
            (s with { clicks : s.clicks + 1 }, 0)
        }
        m == 2 -> { winuser.PostQuitMessage(0); (s, 0) }   // WM_DESTROY
        else -> (s, winuser.DefWindowProcA(h, m, w, l))
    }
}
```

No address appears in the program. The state is a value, so a stale read is a binder the program
can see.

**Option C: ADR 0081's process.** It is the same mechanism as B, with the surface a process
declaration and typed messages.

```
process Pad {                                         // (word open) ADR 0081 item 6's surface
    state S = record { clicks ∈ 0..1000 }
    on 'lbuttondown(x, y) -> state with { clicks : state.clicks + 1 }
    on 'destroy -> { winuser.PostQuitMessage(0); state }
}
```

What it costs: C2, ADR 0081's process kind, is not built, and its surface is open. B is C's
mechanism, so building B first loses nothing, and C becomes B's surface when a second process kind
or the typed messages ask.

**Option D: a closure over the state.** `lpfnWndProc : fn(h, m, w, l) { … s … }` capturing a
`var s`. Refused, as ADR 0018 refuses it: the closure outlives the scope it captured, holding a
linear state.

## 1.2 One state per UI process, or one per window (research decision 2)

**Option 1: one state for the process (recommended).** Two windows of one class share it, and the
step tells them apart by `h`.

```
set S = record { main ∈ windef.HWND, status ∈ windef.HWND, clicks ∈ 0..1000 }

fn step(s ∈ S, h ∈ windef.HWND, m ∈ minwindef.UINT, w ∈ minwindef.WPARAM, l ∈ minwindef.LPARAM) (S × minwindef.LRESULT) {
    case {
        m == 513 && h == s.main -> (s with { clicks : s.clicks + 1 }, 0)
        else -> (s, winuser.DefWindowProcA(h, m, w, l))
    }
}
```

There is one slot, the compiler's, and no address. TinyRetroPad has one main window, and its
dialogs have procedures of their own.

**Option 2: one state per window.** Each window gets a fresh state at `WM_CREATE`, and the step sees
only its own.

```
fn step(s ∈ S, h ∈ windef.HWND, m ∈ …) (S × minwindef.LRESULT) { … }   // the same text
ui_per_window(zero(S), step) { … }                                       // (word open)
```

What it costs: the state per window lives at `GWLP_USERDATA`, an address in the root region, which
needs ADR 0084 item 5 built. It waits on a program with two windows that do not share state.

## 1.3 Which host calls re-enter the step (research decision 3)

A call that dispatches (`SendMessageA`, `DialogBoxIndirectParamA`, `MessageBoxA`) may run the step
again before it returns. The state must be lent to it: written to the slot before the call, and
read after. A call that never dispatches, such as `WriteFile`, need not be lent the state. Lending
it anyway costs one store and one load, which LLVM removes when nothing reads the slot.

The fault every option must catch is this program, which would print a stale value:

```
fn step(s ∈ S, h, m, w, l) (S × minwindef.LRESULT) {
    case {
        m == 513 -> {
            r = winuser.SendMessageA(h, 1024, 0, 0)   // re-enters step with m = 1024 (WM_USER)
            (s, 0)                                     // which s? the one 1024's step left
        }
        m == 1024 -> (s with { clicks : s.clicks + 1 }, 0)
        else -> (s, winuser.DefWindowProcA(h, m, w, l))
    }
}
```

**Option 1: an effect per header, the default "dispatches" inside a UI process (recommended).**

```
// targets/win32/fileapi.oro: kernel32's files, no window, no dispatch
module fileapi nodispatch { … }                      // (word open)

// targets/win32/winuser.oro: says nothing, so it dispatches
module winuser { … }
```

A header that forgets to say so is lent the state, which is safe and costs nothing measurable.

**Option 2: per declaration.** `fn SendMessageA(…) (…) dispatches { … }`, with every other
declaration not dispatching. One forgotten marker gives a stale state, which is exactly the fault.
It is the unsafe default.

**Option 3: no classification.** Every host call inside the extent is lent the state. It is
correct, with a store and a load at every call, and LLVM removes them only where nothing reads the
slot. It is the measured candidate to beat, and needs no word.

## 1.4 The spelling of the installation (research decision 4)

The extent must begin before `CreateWindowExA`, because `WM_CREATE` runs the step inside that call.

**Option (a): a scope (recommended).**

```
export fn main() {
    ui(record { clicks : 0 }, step) {                 // (word open: `ui`)
        wc = zero(winuser.WNDCLASSA) with { lpfnWndProc : step, lpszClassName : "Pad" }
        a = winuser.RegisterClassA(wc)
        h = winuser.CreateWindowExA(0, "Pad", "Pad", 13565952, 100, 100, 400, 300, llvm.null(), llvm.null(), llvm.null(), llvm.null())
        ignore(winuser.ShowWindow(h, 1))
        loop ok = winuser.GetMessageA(…) {            // the message loop: each DispatchMessageA runs step
            ok > 0 -> { ignore(winuser.DispatchMessageA(…)); again(winuser.GetMessageA(…)) }
            else -> ()
        }
    }
}
```

The extent is visible in the text, and a `CreateWindowExA` outside the scope is refused at compile
time (the planted fault "installed after `CreateWindowExA`").

**Option (b): at the field, the initial state elsewhere.**

```
wc = zero(winuser.WNDCLASSA) with { lpfnWndProc : step, lpszClassName : "Pad" }
install(step, record { clicks : 0 })                  // (word open)
h = winuser.CreateWindowExA(…)
```

Its extent is "from here", so the compiler must decide where it ends, and nothing in the text
says.

**Option (c): an attribute of the definition.**

```
fn step(s ∈ S, …) (S × minwindef.LRESULT) handler(record { clicks : 0 }) { … }   // (word open)
```

The extent is then the whole program, so two message loops with different states cannot be
written.

### 1.4b How the step names the state after a re-entering call (open; the research did not say)

Theorem T says a dispatching call inside the step takes S in and gives S back. The program text
must then refer to the returned S. Sketching §1.3's program shows two ways to write it.

**Option (i): the step's state is a cell.** Writes go in program order. A read after
`SendMessageA` reads what the inner activation left, which is C's global and Theorem T's meaning.

```
fn step(s ∈ cell(S), h, m, w, l) (minwindef.LRESULT) {    // a cell parameter: only host declarations have one today
    case {
        m == 513 -> { c = s.clicks; r = winuser.SendMessageA(h, 1024, 0, 0); print.print_int(s.clicks - c); 0 }
        m == 1024 -> { s.clicks := s.clicks + 1; 0 }
        else -> winuser.DefWindowProcA(h, m, w, l)
    }
}
```

`c` is the old value by meaning, a value binder, so the stale read C allows is visible as two names.
This uses what cells already are (ADR 0047, 0048): a host writes a cell through a reference.

**Option (ii): the state threaded by hand at each dispatching call.**

```
m == 513 -> { (s1, r) = lend(s, winuser.SendMessageA(h, 1024, 0, 0)); (s1, 0) }  // (word open: `lend`)
```

Here `s` after the call is still the old value, and `s1` is the new one. It is explicit, and one
word longer at every dispatching call.

Recommendation: (i). It is the language's existing meaning of a cell (state-passing translation),
needs no new word beyond a cell parameter on our own definitions, and Theorem T is then exactly the
proof that a cell lent through the host keeps its program-order meaning.

## 1.5 The callback's claim (research decision 5)

**Option (a): the arrow as a claim, ADR 0053 (recommended, already decided).**

```
// targets/win32/winuser.oro
set WNDPROC = fn(h ∈ windef.HWND, m ∈ minwindef.UINT, w ∈ minwindef.WPARAM, l ∈ minwindef.LPARAM) (r ∈ minwindef.LRESULT)
repr WNDPROC c(h, m, w, l) stdcall                    // its C signature, as tier 1's §2.5 below
set WNDCLASSA = record { style ∈ minwindef.UINT, lpfnWndProc ∈ WNDPROC, … }
```

`step` fits WNDPROC with S added in front, and its answer is checked in `LRESULT`.

**Option (b): no claim, the field a `ptr`.** The step's own header is all there is, so a step taking
three parameters, or returning a `BOOL`, is accepted and crashes the host. Not recommended.

---

# Part 2. Tier 1: a λ the host calls during the call

## 2.1 What tier 1 is (decision 1)

**Recommended: L, a callback applied only during the call.** 105 of Go's 150 callback parameters
are of this kind. Each of these is tier 1:

```
i = strings.IndexFunc(s, fn(r) { r == 32 })                         // applied 0.. times, in order
ws = strings.FieldsFunc(s, fn(r) { r == 32 || r == 10 })            // 0.. times, in no stated order
sort.Slice(names, fn(i, j) { len(names[i]) < len(names[j]) })      // 0.. times, in no stated order
e = filepath.WalkDir(root, fn(p, d, err) { … })                    // in lexical order
```

Each of these escapes the call, and goes elsewhere, not to tier 1:

```
go worker(c)                                   // a spawn: a process (ADR 0081)
http.HandleFunc("/", fn(w, r) { … })          // an installed handler: Part 1's mechanism
sc.Split(fn(data, eof) { … })                  // a host object holding our code: waits on a program
```

**The other option: E in tier 1 too.** Every callback would then be a heap closure, its captures
copied, nothing lent, and no cell written. It loses everything §2.3 and §2.4 admit, and gains only
programs that the other three mechanisms serve better.

## 2.2 The markers on a declaration (decision 2)

A callback parameter states how many times the host applies it, and whether in a fixed order.

**Option (a): `calls K` and `ordered`, after the arrow as `pure` is (recommended).**

```
fn IndexFunc(s ∈ go.bytestring, f ∈ fn(r ∈ Rune) (Bool) pure calls 0.. ordered) (Int) pure { host expr "strings.IndexFunc(${s}, ${f})" import "strings" }
fn FieldsFunc(s ∈ go.bytestring, f ∈ fn(r ∈ Rune) (Bool) calls 0..) (Array(go.bytestring)) { … }
fn Map(f ∈ fn(r ∈ Rune) (Rune) calls 0.. ordered, s ∈ go.bytestring) (go.bytestring) { … }
fn Do(o ∈ Once, f ∈ fn() () calls 0..1) () { … }                     // sync.Once: at most once
fn Run(f ∈ fn() () calls 1) () { … }                                 // exactly once
```

What a program may then do, by K:

```
r = must_use()                                  // r a relevant value (ADR 0043)
o.Do(fn() { ignore(r) })                        // calls 0..1: refused, r may go unused
Run(fn() { ignore(r) })                         // calls 1: accepted, r is used on every path
```

**The default, when a declaration says nothing.**
- (a1) **Conservative, `calls 0..` and unordered (recommended).** The body must be pure and may
  consume nothing relevant or linear. A declaration that says nothing admits the fewest programs,
  and is never wrong.
- (a2) **No default.** A callback parameter without both markers is refused when the target loads.
  It is stricter, and every Go declaration must state them.
- (a3) **Permissive, `calls 1 ordered`.** A forgotten marker admits a written cell under a host that
  calls in any order. This is unsound, Theorem D's fault, and not offered.

**The words.** Each spelling says the same thing:
- (w1) `calls 0.. ordered`, the count a set of naturals as a range is (ADR 0077);
- (w2) `× 0.. in order`;
- (w3) Kotlin's `callsInPlace(AT_MOST_ONCE)`, which has no form for `0..` and no order.

(w1) is recommended: it reuses ranges, and `calls` reads as what it states.

## 2.3 A cell written in the λ (decision 3)

```
var n := 0
t = strings.Map(fn(r) { n := n + 1; r }, s)            // Map is `ordered`
print.print_int(n)                                      // the number of runes in s
```

- **Option (a) (recommended): threaded under `ordered`, refused without it.** The program above is
  accepted, and `n` is threaded through the call as an extra result (Theorem T₁). Under
  `FieldsFunc`, unordered, it is refused, saying the order is the host's:

  ```
  ws = strings.FieldsFunc(s, fn(r) { n := n + 1; r == 32 })   // refused (Theorem D)
  ```

- **Option (b): also admit writes that commute.** `n := n + 1` gives the same `n` in every order,
  the action of a commutative monoid, so (b) accepts the `FieldsFunc` line too. It is a rule about
  which writes commute, with its own planted fault, and waits on a program that counts under an
  unordered host.
- **Option (c): no writes in any λ.** The count is then a loop of the program's own after the call,
  and `strings.Map`'s λ is pure. It is simplest, and refuses what Go programs write.

## 2.4 A buffer in the λ (decision 4)

```
var hist := array { 0 : k ∈ 0..15 }
t = strings.Map(fn(r) { hist[r % 16] := hist[r % 16] + 1; r }, s)
```

- **Option (a) (recommended): decide now, build when a program writes one.** The buffer is lent for
  the call, as a cell is (ADR 0084's Theorem P). This narrows callbacks.md §4's refusal, "two
  goroutines could hold it", to the escaping callbacks, where its reason holds. The program above
  is then accepted under `ordered`, and its writes are refused under an unordered host.
- **Option (b): keep the refusal.** The program counts in a cell, or with its own loop over `s`.
  callbacks.md §4 stays as written.

## 2.5 The context word on a C declaration (decision 5)

The program is the same under every option. A λ counts the top-level windows:

```
var n := 0
ok = winuser.EnumWindows(fn(h) { n := n + 1; 1 })     // `calls 0.. ordered`: threaded
print.print_int(n)
```

On windows the λ becomes a C function with `EnumWindows`'s convention. Its environment (`n`'s cell)
is a record in the caller's frame, and its address travels in the API's `LPARAM`, which no program
observes. It is representation, so it is written in the declaration's host clause and `repr`, as a
struct's layout and its `&x` are.

- **Option (a) (recommended): `ctx` in the callback's C signature, `ctx(f)` in the call.**

  ```
  set WNDENUMPROC = fn(h ∈ windef.HWND) (minwindef.BOOL) calls 0.. ordered
  repr WNDENUMPROC c(h, ctx) stdcall
  fn EnumWindows(f ∈ WNDENUMPROC) (r ∈ minwindef.BOOL) { host expr call EnumWindows(f, ctx(f)) stdcall lib "user32" }
  ```

- **Option (b): C's own notation, `&f`.** The call reads `call EnumWindows(f, &f)`, `&f` being the
  address of f's environment, as C writes `(LPARAM)&env`. It is closer to ADR 0131's "as C writes
  it", but `&x` already means a struct's fresh copy, and here x is a function. Two meanings of `&`,
  told apart by the set of x.
- **Option (c): in the header, rejected.** `fn EnumWindows(f ∈ WNDENUMPROC, l ∈ LPARAM = ctx(f))`.
  This is the first draft, corrected after ADR 0130: a determined parameter of the header is a
  representation in the place of meaning, the fault `in` was.

**An API with no context word** (33 of 349, and `qsort`) has a `repr` without `ctx`:
`repr CMPFN c(a, b) cdecl`. Its environment is found through a slot, saved and restored around the
call, and the program is unchanged.

## 2.6 The order of building (decision 6)

Both tiers share one piece: a function of ours printed with the host's C convention, its address a
value. It is the lifted λ in tier 1 and the trampoline in tier 2.

- **Option (a) (recommended): the shared piece, then the window procedure, then tier 1 at its
  trigger.** TinyRetroPad (plan item 6) needs §1's program. Tier 1's trigger is a Go-library program
  under the standing goal, such as

  ```
  use go/strings
  export fn main() { print.print_int(strings.IndexFunc(read_line(), fn(r) { r == 32 })) }
  ```

- **Option (b): tier 1 first.** Its region on the IR is the larger construct, with every analysis
  taught to read it as a loop. It serves the Go library, and TinyRetroPad waits for it.
- **Option (c): both together.** One step, twice the size, and a single result document that cannot
  say which rule a failure belongs to.

---

## What each option set asks of you

| decision | the options | recommended |
|---|---|---|
| T2-1 mechanism | A by hand, B step, C process, D closure | B (C its surface later) |
| T2-2 state | per process, per window | per process |
| T2-3 dispatch | per header with default dispatches, per declaration, none | per header |
| T2-4 installation | scope `ui(s₀, step) { … }`, at the field, attribute; and the scope's word | scope; the word is yours |
| T2-4b state after re-entry | a cell parameter, `lend(s, …)` by hand | the cell |
| T2-5 claim | the arrow (ADR 0053), `ptr` | the arrow |
| T1-1 scope | L only, E too | L |
| T1-2 markers | `calls K ordered`, `× K in order`, Kotlin's; default conservative, none, permissive | `calls K ordered`, conservative |
| T1-3 written cell | threaded under `ordered`, commuting writes too, none | threaded under `ordered` |
| T1-4 buffer | decide now, build at a program; keep refused | decide now |
| T1-5 context word | `ctx`/`ctx(f)`, `&f`, in the header | `ctx(f)` |
| T1-6 order | shared piece, WndProc, tier 1; tier 1 first; together | shared piece, WndProc, tier 1 |
