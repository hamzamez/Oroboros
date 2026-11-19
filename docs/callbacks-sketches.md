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

**Decided so far** (hamza, 2026-11-18): T2-1 B; T2-2 one state per process; T2-3 option 3, no
classification; T2-4 no new word, the `var` scope is the extent (§1.4c); T2-4b the cell; T2-5 the
arrow; T1-1 L. Open: T1-2 to T1-5, each derived in Part 3 (2026-11-20), T1-3 first; T1-6
settled by events (the window procedure built).

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
it anyway costs one store and one load. *Corrected 2026-11-18*: this said LLVM removes them when
nothing reads the slot. It cannot remove them here, because the trampoline reads the slot, and its
address is given to Windows, so to LLVM any external call might call it.

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

**Option 1: an effect per header, the default "dispatches" inside a UI process.**

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

**Option 3: no classification (recommended, corrected 2026-11-18).** Every host call inside the
extent is lent the state. It is correct and needs no word. Its costs:
- a store and a load at every host call in the extent, which LLVM keeps (above), unmeasured;
- what the analysis knows about the state's fields is lost at every host call, not only at the
  dispatching ones. It falls back to the set S, so a guard read before `TextOutA` does not bound
  the same field read after it. A program keeps such a fact by binding the field to a value first,
  `n = s.clicks`.

Option 1 is option 3 plus a word that says "this header does not dispatch", and buys back exactly
those two costs on the calls it marks. It was recommended against option 2, the unsafe default,
and never argued against option 3. By the standing rule, the analysis grows only for a refusal or a
measurement a program names: build option 3, and add option 1's word when TinyRetroPad's step, or
a measurement of it, asks.

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

### 1.4c The extent is the cell's own scope (hamza: "doesn't our language already handle these scopes? why do we need a new word ui()?")

It does. With 1.4b's cell, `var s := …` already has a beginning and an end. All that `ui(s, step)`
added was the link between the step and the cell, and that is written with what exists: the field
holds a λ that applies the step to the cell.

```
export fn main() {
    var s := record { clicks : 0 }
    wc = zero(winuser.WNDCLASSA) with { lpfnWndProc : fn(h, m, w, l) { step(s, h, m, w, l) }, lpszClassName : "Pad" }
    a = winuser.RegisterClassA(wc)
    h = winuser.CreateWindowExA(…)                   // WM_CREATE runs the λ: s exists, by scope
    … the message loop …
    print.print_int(s.clicks)                         // the state the last message left
}
```

- **The extent before `CreateWindowExA` is lexical scoping.** The λ can only be written where s is
  in scope, so the state exists before any call that can run it. The planted fault "installed after
  `CreateWindowExA`" cannot be written at all.
- **What changes is a rule, not a word.** The research refused D, a closure over the state, because
  the closure outlives the scope. Here the host keeps the λ (RegisterClassA holds the pointer)
  beyond the scope, so the rule is about what happens then. At the cell's scope end the compiler
  clears the slot, and a message that arrives after it, for a class still registered, is answered
  by `DefWindowProcA`. That is defined, with no crash and no refusal, and needs no knowledge of
  which calls dispatch (T2-3's option 3).
- **What is built**: the λ printed with WNDPROC's C convention, its environment the cell's slot,
  Theorem T for the cell lent to every host call in the scope, and the clearing at the scope's end.
  It revises the research's candidate D (refused for escaping its scope) to a λ held by the host
  for its cell's scope, and the ADR says so.

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

*Corrected 2026-11-20* (§3.7): `IndexFunc` and `Map` are not `ordered`. Their documentation
promises a result, not the order of the calls. Written by what Go promises, they are
`calls 0..` alone, and `filepath.WalkDir`'s `fn(p ∈ go.bytestring, d ∈ DirEntry, e ∈ error)
(error) calls 0.. ordered` is the `ordered` example.

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

*Corrected 2026-11-20* (Part 3, §3.7): `Map` is not `ordered`. Its documentation promises a
result, not the order or the number of its calls, so this program is refused under every option.
The example of an `ordered` host is `filepath.WalkDir`, "walked in lexical order".

- **Option (a) (recommended): threaded under `ordered`, refused without it.** The program above is
  accepted, and `n` is threaded through the call as an extra result (Theorem T₁). Under
  `FieldsFunc`, unordered, it is refused, saying the order is the host's:

  ```
  ws = strings.FieldsFunc(s, fn(r) { n := n + 1; r == 32 })   // refused (Theorem D)
  ```

- **Option (b): also admit writes that commute.** `n := n + 1` gives the same `n` in every order,
  the action of a commutative monoid, so (b) accepts the `FieldsFunc` line too. It is a rule about
  which writes commute, with its own planted fault, and waits on a program that counts under an
  unordered host. *Withdrawn 2026-11-20* (Part 3, §3.2): the count under an unordered host is the
  host's too, and n depends on it.
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
| T2-3 dispatch | per header with default dispatches, per declaration, none | none; per header at a program's refusal |
| T2-4 installation | scope `ui(s₀, step) { … }`, at the field, attribute | decided: the `var` scope, no word (§1.4c) |
| T2-4b state after re-entry | a cell parameter, `lend(s, …)` by hand | the cell |
| T2-5 claim | the arrow (ADR 0053), `ptr` | the arrow |
| T1-1 scope | L only, E too | L |
| T1-2 markers | `calls K ordered`, `× K in order`, Kotlin's; default conservative, none, permissive | `calls K ordered`, conservative |
| T1-3 written cell | threaded under `ordered`, commuting writes too, none | threaded under `ordered` |
| T1-4 buffer | decide now, build at a program; keep refused | decide now |
| T1-5 context word | `ctx`/`ctx(f)`, `&f`, in the header | `ctx(f)` |
| T1-6 order | shared piece, WndProc, tier 1; tier 1 first; together | shared piece, WndProc, tier 1 |

---

# Part 3. Where the derivation leads, decision by decision

2026-11-20. hamza, on Part 2: "2.1 agree. 2.2 I don't see it yet. what does the derivation say. 2.3
you mean a cell from outside the lambda? my first instinct is no, remember we started this language
as s-expression functional language. don't let the syntax, and the targets confuse you. push back
or explain more. 2.4 same, this looks like a closure, am I wrong? … for every decision I have to
make on these, show me where the derivation leads, and how each option agrees or disagrees with
the derivation."

Everything below is read in the core, the λ-calculus the J surface desugars to. A target appears
only where it realizes a term.

## 3.0 Two facts from the core that the sketches hid behind J

**There is no cell in the core.** `var n := 0` is surface. The state translation S (local.md §3)
makes it a value passed along: "Moggi's state monad compiled away", in the spec's words. A cell
written in a loop body is accepted today, and it is the plainest case:

```
var n := 0                                  loop n = 0, i = 0 {
loop i = 0 {                         S        i < 5 -> again(n + i, i + 1)
    i < 5 -> { n := n + i; again(i + 1) }  ─────→   else -> n
    else -> n                               }
}
```

The loop's body is a λ (ADR 0055). After S it has one more parameter and one more result, and
nothing is mutated. So "a cell written in a λ" means, in the core, **a λ with the state as an
extra parameter and an extra result**. It is a pure function. A mutable variable appears only on a
target, where Go prints the loop variable as one too.

**Every λ given to a host is a closure, in every option.** 2.1's `fn(r) { r == c }` already pairs
a λ with an environment holding `c`. So does our `loop` body, which captures the scope's values. A
closure is not what the language refuses. It refuses a closure that **survives** as a value:
stored, returned, or applied later. Tier 1's λ is second-class (Strachey 1967): it is applied only
inside the call, so its environment is the caller's frame, as a loop body's is. 2.1 (L) is the
decision that admits it. 2.3 and 2.4 are not "closure or no closure". They ask **what the
environment may hold**: values only, or state to be threaded.

**The probe that wrote the second fact down found a wrong answer.** The rule "a λ may not mention
a cell" (local.md §3) was checked for a λ passed as an argument, and not for one bound by a name:

```
var n := 0
add = fn(x) { n := n + x }
add(1)
add(2)
io.print_int(n)            // printed 0; now refused, as a passed λ is
```

`get = fn() { n }` read the cell's first value, also printing 0. Both are fixed, with a planted
fault (callbacks-derivation-2026-11-20). So hamza's "no" is the rule today, and the rule had a hole.

## 3.1 What a host must tell us, and why (decision 2)

A host call h(ā, λȳ. e) runs e an unknown number of times, in an unknown order, inside a body we
cannot read. Two checks the language already makes need to see across that body, and nothing else
does:

1. **Effects** (ADR 0010, the purity bit). An impure body (a host call that prints, or state
   threaded by S) has a meaning only if the host's activations form one sequence.
2. **Usage** (ADR 0043 relevance, at least once; linearity, at most once). abstract.md §8.1's 𝒰
   counts a name's uses in 𝕌, the interval semiring. A name the body uses is used k times its use
   in the body, where k is the host's count: 𝒰ₓ(h(ā, λȳ.e)) = Σᵢ 𝒰ₓ(aᵢ) + k · 𝒰ₓ(e).

**Theorem D** (research §3.3) says what happens when neither check is in play. If the body is pure
and consumes nothing relevant or linear, the call's meaning is h's function applied to the λ's
function, **whatever the count and the order**. So:

> **A pure λ that consumes nothing needs no marker at all.** A marker exists only to let one of the
> two checks through the host's body, and each marker is the host's half of a check we already make.

- **The count is k ∈ 𝕌, for usage.** A relevant name consumed in the body needs k ≥ 1, and a
  linear one k ≤ 1. Not knowing k is k = [0, ∞], the top of 𝕌.
- **The order is the existence of a lift, for effects.** An impure body is a Kleisli arrow
  Y → T Z, and the host was declared at (Y → Z) → W. The program has a meaning only if there is an
  h_T : (Y → T Z) → T W extending h. That h_T exists when h is a **traversal**, its activations a
  sequence its arguments determine. Haskell's `map` and `mapM` are this distinction: a `map` takes
  only a pure function, and `mapM` (`traverse`) needs a fixed order of effects. `ordered` is the
  claim "this host function is a traversal, in this documented order". A sequence fixes the count
  too.

Two consequences:
- A count of at most one makes the order trivial, since one activation has one order. So the
  markers are a pair (k, order) with k ≤ 1 implying ordered.
- **Decision 2 depends on decision 3.** If no λ given to a host may be impure, which is what
  hamza's instinct leads to, then `ordered` has nothing to admit and is not needed. Only `calls`
  remains, for a relevant value under `calls 1`, which waits on a program (`sync.Once.Do`,
  `pprof.Do`). Decide 3 first.

**How each option agrees with the derivation:**

| option | the derivation |
|---|---|
| `calls K` with K a range (w1) | **agrees**: K is an element of 𝕌, and J writes intervals as ranges (ADR 0077) |
| `× K in order` (w2) | the same algebra, other letters |
| Kotlin's `callsInPlace(…)` (w3) | four named points of 𝕌 and no order: a subset |
| default `calls 0..`, unordered (a1) | **agrees**: an unstated claim is the weakest, ⊤, which admits exactly Theorem D's case |
| no default (a2) | the same meaning; it differs only in refusing silence when the target loads |
| default `calls 1 ordered` (a3) | **contradicts**: it claims a lift the host never promised; unsound |

## 3.2 A cell written in the λ (decision 3)

Yes: the sketch means a `var` declared in the enclosing scope and written in the λ's body.

**In the core**, after S, `strings.Map(fn(r) { n := n + 1; r }, s)` is the function

```
λ(r, n). (r, n + 1)   :   Rune × ℤ → Rune × ℤ
```

handed to a host whose parameter is `Rune → Rune`. `Map` does not pass `n` along, so the program has
a meaning only through the lift of 3.1:

```
Map_S : (Rune × S → Rune × S) × String × S  →  String × S
```

This is `mapAccumL`, the stateful traversal, a pure function. It exists when `Map` visits the runes
in a documented order. Its meaning is n folded through the runes in that order (Theorem T₁). Go's
`n++` inside a closure is how one target realizes that fold, as its mutable loop variable realizes
our loop's. *Corrected 2026-11-20*: this first said Go's documentation states `Map`'s order. It does
not (§3.7), so `Map` is Theorem D's case and this program is refused. The lift is real for a host
that does state its order, `filepath.WalkDir(root, fn(p, d, err) { n := n + 1; nil })`, which
counts the entries in lexical order.

So **the derivation pushes back on "no"**. Option (a) is not a mutable variable captured from
outside. It is the state monad, which the language already compiles away for its own loops (§3.0),
applied to a host function that is a traversal. Two pieces already in the language compose into it:
- a host writing a cell, f′ : X × T → R × T (ADR 0048, local.md §5);
- a λ applied by a primitive in order (the `loop` body).

**What (a) does cost**, which is the real reason to hesitate: the meaning now rests on a claim about
the host, `ordered`, written by hand. A wrong `ordered` is a wrong answer. A pure λ needs nothing
from the host (Theorem D). That dependence is the price of (a), and it is the same price as every
`pure` and every `where` on a host declaration (ADR 0022: written by someone who read the host's
source).

**Each option against the derivation:**

| option | the derivation |
|---|---|
| (a) threaded under `ordered`, refused without it | **agrees**: exactly the lift where it exists, and a refusal where it does not |
| (b) commuting writes under no order too | **contradicts.** The sketch says `n := n + 1` gives the same `n` in every order. That is true, but the result needs the count as well as the order. Under no stated order the count is the host's too: `sort.Slice`'s number of comparisons is its algorithm's, and `FieldsFunc` promises no call per rune. The final n = n₀ + (the number of activations), which is not a function of the arguments. A commutative monoid's action is independent of order, not of count. Even an idempotent write (`seen := seen ∪ {r}`, a max) needs the set of arguments the host calls with. **So (b) is withdrawn as stated**, and §2.3 and research §3.4 are corrected. It returns, narrower and sound, where a host documents its count without its order (§3.7) |
| (c) no writes in any λ | **a sound subset** of (a), which the derivation does not require. Its own consequence: a print in the body is an effect just as a cell write is, both impure in the core, so (c) consistently means "a λ given to a host is pure", with no host IO either. That is coherent, and `ordered` then disappears (3.1) |

**What (c) refuses**, measured by what the hosts are built for:
- the visitors, whose only output is the effect: `filepath.WalkDir` collecting paths, `ast.Inspect`,
  `EnumWindows`, and Win32's other enumerators;
- Win32's 291 context words (research §2.2), which exist to carry the state the callback writes:
  C's own hand-written state passing, the very pair (a) would fill.

Under (c), `EnumWindows(fn(h) { … })` can only answer 1 or 0 to the host, and computes nothing.

**Staging is monotone.** Every program (c) accepts, (a) accepts with the same meaning. So (c) now
and (a) at the first program that writes a cell is a sound order of building. It is not a different
design.

**Recommended, by the derivation: (a), decided now and built with tier 1**, with the λ's host
effects (a print) under the same rule as its cell writes. It is the functional reading. (c) is the
safe start of the same design, if hamza prefers to wait for the program.

**One asymmetry (a) makes visible.** Our own higher-order definition is refused the same λ today:
`twice(fn(x) { n := n + x })` is a closure refusal, though after β its applications are in program
order. The refusal comes from the translation's order, S before β, not from the meaning. Under (a),
a host's `ordered` call would admit what our own definition refuses. The cure, if a program asks, is
the same lift for a definition β can see. It is not proposed here.

## 3.3 A buffer in the λ (decision 4)

**In the target it is a closure. So is 2.1's `fn(r) { r == c }`**, and so is every λ here (§3.0).
**In the core** a buffer is a state binder holding a table (ADR 0052), translated by the same S as a
cell; local.md says it outright: "a cell is a buffer of length one". So

```
strings.Map(fn(r) { hist[r % 16] := hist[r % 16] + 1; r }, s)
```

is, after S, λ(r, h). (r, set(h, r % 16, h[r % 16] + 1)), the same lift as 3.2 with the state a
table. The environment holds no mutable thing: the table is a parameter, threaded. Linearity holds
by the threading. Each activation consumes the table once and returns it once, one chain, whatever
the count, because 𝒰's scaling applies to a *captured* name, and a threaded state is not captured.
On LLVM the table is a pointer in the environment, lent for the call and returned (ADR 0084's
Theorem P).

**The derivation makes 2.4 the same decision as 2.3**, its state of a different sort. The old
refusal (callbacks.md §4, R3) gave its reason as "two goroutines could hold it": aliasing across
threads, which L excludes and E and C do not.

| option | the derivation |
|---|---|
| (a) lent like a cell | **agrees**, if 3.2 is (a): one translation for both state binders |
| (b) keep refused | **agrees** if 3.2 is (c). If 3.2 is (a), it draws a line between a cell and a buffer of one that S does not draw |

So 2.4 takes 2.3's answer.

## 3.4 The context word (decision 5)

Closure conversion (Minamide, Morrisett and Harper 1996) is ∃E. (E × A → R) × E, and Win32's
(callback, `LPARAM`) is that pair. E's address is never observed by the program, so it is
representation, and representation is written in `repr` and in a call's argument terms (ADR 0130).
This holds under every answer to 3.2: even a pure λ's environment holds `c`.

| option | the derivation |
|---|---|
| (a) `repr … c(h, ctx)`, `ctx(f)` | **agrees**: an argument term, as `ref(x)` and `elem(b, lo)` are |
| (b) `&f` | **disagrees**: `&x` means the address of a fresh copy of a value (ADR 0130, 0131); f's environment is not a copy of f. It gives one symbol two meanings, told apart by the set |
| (c) in the header | **contradicts**: a representation in the meaning's place, the fault ADR 0130 removed from `in` |

## 3.5 The order of building (decision 6)

Settled by events. The shared piece and the window procedure are built (ADR 0133, cbfield, msgloop,
slot), so tier 1 comes next, at its trigger, a Go-library program.

## 3.6 The decisions, in the order the derivation needs them

| | the question | where the derivation leads |
|---|---|---|
| 3 | may a λ given to a host have effects: threaded state, a host's IO? | **(a)**: yes, exactly where the host is a traversal (`ordered`), as the state monad our loops already use. (c) is its sound first stage. (b) as stated is withdrawn, and sound where a host documents each element once without an order (§3.7). By Go's documents, (a) reaches the 12 visitors (`WalkDir`, `ast.Inspect`, `flag.Visit` …), and not `Map` or `IndexFunc` |
| 4 | a buffer | **the same answer as 3** |
| 2 | the markers | **`calls K` with K a range, and `ordered`**, default the top, `calls 0..` unordered. Under 3 = (c), `ordered` is not needed and `calls` waits on a relevant value |
| 5 | the context word | **`ctx(f)`**, independent of 3 |
| 6 | the order | done: the window procedure is built; tier 1 at its trigger |

## 3.7 Why `Map` and `FieldsFunc` differ, measured (hamza, 2026-11-20)

hamza: "show me why `t = strings.Map(fn(r) { n := n + 1; r }, s)` is accepted while
`ws = strings.FieldsFunc(s, fn(r) { n := n + 1; r == 32 })` is refused."

**By the derivation, they do not differ: both are refused.** What decides is what the host promises,
and the sketch had read `Map`'s promise wrongly.

**What each does today, Go 1.27's source:** both call f once per rune, left to right, `for i, c :=
range s { r := mapping(c) … }` and `for end, rune := range s { if f(rune) { … } }`. No experiment on
one Go release tells them apart.

**What each promised before.** `FieldsFunc` in Go 1.9 made two passes, a counting loop
(`inField = !f(rune)`) and a creating loop (`if f(rune) {`). So it called f twice per rune.
`n := n + 1` in its λ then counts 2·len(s) on Go 1.9 and len(s) on Go 1.27: one program, two
answers. That is what "not a function of the arguments" means, and why its documentation says "makes
no guarantees about the order in which it calls f(c) and assumes that f always returns the same
value for a given c". That sentence is Theorem D's premise, in Go's words.

**What each documents.**
- `Map`: "a copy of the string s with all its characters modified according to the mapping
  function". That is a promise about the result. It says nothing of the order or the number of the
  calls, so a Go release may change either, as `FieldsFunc`'s did.
- `FieldsFunc`: no order, explicitly.

So `Map` has `FieldsFunc`'s standing. A declaration states what the host **promises**, which holds
on every release under Go 1's compatibility promise. What one release **does** is an accident of its
implementation. Declaring `Map` `ordered` from its source would tie the program's meaning to one Go
release.

**What the documents do promise, all 89 read again** (the markers file, corrected): 12 state the
sequence of their calls, the visitors:
- `filepath.WalkDir`, `filepath.Walk` and `fs.WalkDir`: "walked in lexical order";
- `ast.Inspect`: "depth-first order: it starts by calling f(node)"; `ast.PreorderStack`;
- the four `flag.Visit` forms: "in lexicographical order, calling fn for each";
- `ring.Do`: "in forward order"; `token.FileSet.Iterate`: "in ascending Base order";
- `slices.EqualFunc`: "compared in increasing index order".

The other 77 promise a result: `IndexFunc`'s "first code point satisfying f", `Map`'s copy,
`TrimFunc`, `ReplaceAllFunc`, and the sorts.

**The documents sort the functions by purpose.** A function whose purpose is its result promises no
sequence, and its λ should be pure: `Map`, `IndexFunc`, `TrimFunc`, `sort.Slice`. A function whose
purpose is the visiting promises one, and its λ is useless unless it has an effect: `WalkDir`,
`Inspect`, `flag.Visit`. So decision 3 is narrower than it looked. It asks whether the visitors are
usable, not whether string functions may count.

**(b) returns, in the form the algebra allows.** A commutative write (`n := n + 1`, a histogram
cell) needs the multiset of the calls, not their order. Three hosts document exactly that:
- `sync.Map.Range`: "calls f sequentially for each key and value … no key will be visited more than
  once";
- `expvar.Do`: "calls f for each exported variable";
- `slog.Record.Attrs`: "calls f on each Attr".

Their count is the number of elements, and their order is open. So the markers are three strengths
of one promise, about the history of the calls:

| the host promises | it admits in the λ | examples |
|---|---|---|
| nothing (the default) | a pure body (Theorem D) | `Map`, `IndexFunc`, `FieldsFunc`, `sort.Slice` |
| each element once, in no order | writes forming a commutative monoid's action | `sync.Map.Range`, `expvar.Do`, `slog.Record.Attrs` |
| the sequence (`ordered`) | any effect, threaded in that order | `WalkDir`, `ast.Inspect`, `flag.Visit`, `EqualFunc` |

The middle row is (b) made sound. It needs a rule for which writes commute, with its planted fault,
and it waits on a program.

**Win32 read the same way.** `EnumWindows`: "Enumerates all top-level windows on the screen by
passing the handle to each window, in turn, to an application-defined callback function." That is
each window once, in an order the page does not state, so it is the middle row: counting the
windows is determined, and collecting their handles in order is not. §2.5's `calls 0.. ordered` on
`WNDENUMPROC` overstated it. One more condition belongs to the middle row: the λ must not stop the
enumeration early (`EnumWindows` stops at FALSE, `sync.Map.Range` at false), since which elements
were visited before the stop depends on the order.
