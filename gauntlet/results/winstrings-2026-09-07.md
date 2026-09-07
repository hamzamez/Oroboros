# windows can construct a string

2026-09-07. `targets/windows/strings.oro`,
[assessment-2026-09-06](../../docs/assessment-2026-09-06.md)'s item 5 — the last
capability gap in the integer work.

Until now this host could **print** a string and could not **make** one. Literals
are emitted as `db …,0` in the data section; nothing could produce a new one. So
`render.oro` — decimal rendering of an arbitrary-precision value, which is
repeated division and then *text* — ran on three targets of four.

> **`concat` and `string-of` are two assembly templates.** `render` now runs on
> **all four**, and the differential suite's `render` case goes from six variants
> to seven.
>
> And the gap turned out not to be the strings. Once they existed, `render` was
> still refused on windows — by a **one-conjunct bug in the limb rung** that had
> switched off the fixpoint on the only target that has no host bignum to fall
> back on.

---

## 1. What is declared

Two `prim`s, found by **spelling** the way `=` and `+` are, so they are the
LANGUAGE's names and this file only says how this host performs them.

**`string-of`** — η : Scalar → Scalar\*, the generator injection. Four cases,
because UTF-8 is four cases. The other three hosts hide them behind
`string(rune(c))`, `String.fromCodePoint` and `Character.toChars`; this one has
no library to hide behind, so the encoder is written out — a compare-and-branch
ladder on 128 / 2048 / 65536 and then the byte assembly, with `%u` giving each
expansion its own labels.

**`concat`** — the monoid's operation. Copy until NUL, twice, which fuses
`strlen` with the copy and is what a C programmer writes.

**No `malloc`.** Both could call it, and calling from inside a template is where
this gets delicate: a call clobbers the volatile registers and `%1`, `%2` and
`%r` are whichever registers the allocator chose. A **static bump arena** needs
no call, so both templates are straight-line code and two loops.

Never freed — which is this target's existing memory story rather than a new one.
[windows-target.md](../../docs/spec/windows-target.md) records one `VirtualAlloc`
per `alloc`, never released, and says changing that is a target-file edit. The
arena is the same decision for text, and its size is the same kind of stated
limit as `tree.oro`'s node cap.

**The one discipline that matters is reading both operands before touching any
scratch.** `concat` saves `%2` into the home space at `[rsp+32]` — the 48 bytes
`emit/asm.go` reserves so a template can write there without knowing anything
about the frame — because the scratch it needs (rax, rcx, rdx, r8) may *be* `%1`
or `%2`.

## 2. It is byte-identical to Go at every UTF-8 width

Checked directly rather than inferred, one scalar of each width — `A`, `é`, `日`,
`🙂`:

```
Go:      A 303 251 346 227 245 360 237 231 202
windows: A 303 251 346 227 245 360 237 231 202
```

and decoding those bytes gives back `0x41, 0xe9, 0x65e5, 0x1f642` exactly.

Pinned as the differential case **`utf8-widths`**, which is deliberately **two
targets**: `System.out` on the JVM encodes in the platform charset, so a printed
`é` is one byte on a Windows console and two on Go — a property of *printing*,
which is why `string-escapes` is ASCII-only and says so. What remains is the
comparison that matters: windows' hand-written encoder against Go's own
`string(rune(c))`.

Nothing else in the corpus reaches past the first width — `render.oro` only ever
asks for a decimal digit — which is why the case exists.

## 3. THE GAP WAS NOT THE STRINGS

With both operations declared, `render.oro` was still refused on windows:

```
render: in a loop's guard: in argument 1 of `=`: v is array int,
        but int is required here.
```

**Reproduced with no strings in the program at all**, which is what identified
it: the same loop shape, on windows and on Go forced to `-big-repr=limbs`,
succeeded on Go and failed on windows. One line in `emit/interval.go`:

```go
if p.bigOK() && p.tgt.HasBig() && (p.loopTail || len(p.big) > 0) {
```

`bigOK()` is already `limbs || HasBig()`. The extra conjunct means the loop's
**representation fixpoint never ran on a target with no host bignum** — so supply
never reached a loop variable through its initialiser, `v` stayed a machine word
although `x` is a declared arbitrary-precision parameter, and the checker refused
the guard by name.

Two things make this worth recording rather than just fixing.

**`emit/biglimb.go` warns about exactly this shape**, in as many words: *"every
gate below asks `bigOK` rather than `HasBig`, or a target with nothing to fall
back to would promote nothing and then be refused for producing a machine
word."* The warning was written and then not applied at this gate.

**And the comment directly beneath the gate names the program.** It says the rule
exists because *"the first program to hit it was decimal rendering, whose whole
shape is a big value walked down to nothing while the answer is a string"* — and
the gate switched that rule off on the one target where decimal rendering had no
alternative. The rule was written for this program and disabled for it.

It went unnoticed because the differential case carried `; skip: windows` for a
capability reason that was true when written, so the path had never run. **A path
nothing runs is a path nothing checks**, for the fourth time in this repository.

## 4. And one in the harness

With everything building, `render` produced the right digits on windows and ran
them together:

```
windows  15511210043330985984000000265252859812191058636308480000000
```

The string driver used `msvcrt.printf`, which appends no newline, where the other
three use `Println`/`console.log`/`out-println`, which do. The note beside it
explained the choice: *"`printf` rather than `puts`, which does not link"* — true
when written, and **fixed by string-literals.md**, which added `ucrt.lib
vcruntime.lib` to the generated `build.bat` and made every CRT stdio entry point
reachable. Nothing revisited the note. `puts` appends the newline.

Invisible until a case printed **more than one** string, which `render` is the
first to do.

## 5. Cost

| | |
|---|---|
| emitted files | **no pre-existing file changes**; one new — `render` on windows |
| differential suite | 28 cases (was 27), four targets, green |
| `render` | six variants → **seven**: go/host, go/limbs, js/host, js/limbs, java/host, java/limbs, **windows** |
| unit tests, `go vet` | green |

One test, verified to fail against the gate: the same program on a host-bignum
target forced to limbs and on a target with no host bignum must promote
identically, because the storage is the same.

## 6. What this closes, and what it does not

**Closes:** the last capability gap named in the integer work. windows has
arbitrary precision (`emit/bignum.oro`, the fixed-limb library), and now it can
render one as text. ADR 0019 item 4 is complete.

**Does not close:** a windows string is still a bare NUL-terminated byte pointer
and not a table, deliberately. Making it a byte table would decide
[string-operations.md §3](../../docs/string-operations.md)'s *is a string a
table* on one host, ahead of the measurement that decides it — and
[jsonfmt-2026-09-07](jsonfmt-2026-09-07.md) has since produced half of that
measurement in the other direction. `length`, indexing and string `=` remain
absent here as everywhere.

The arena is also never freed and has a fixed size, so a windows program that
builds text in an unbounded loop will exhaust it. That is stated rather than
solved, and it is the same shape as every other capacity in this project.
