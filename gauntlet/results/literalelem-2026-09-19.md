# A literal table's elements decide its element type

2026-09-19. `emit/target.go`, `emit/golang.go`, `emit/java.go`, `emit/literalelem_test.go`,
`README.md`. Research first: [literal-elements.md](../../docs/literal-elements.md), on hamza's
question about the README's fourth example — *why the `build` chain rather than
`(array 104 105 33)`, and is this fundamental?*

It was fundamental, and the answer was a gap rather than a design: **the style is right and the
compiler could not serve it.**

## 1. What was wrong

`(array 104 105 33)` handed to a host call declaring `(array (int 0 255))`:

```
.\main.go:13:91: cannot use src (variable of type []int) as []byte value in
                 argument to func(dst, src []byte) ([]byte, int){…}
```

Our checker accepted it; **Go refused it**, naming a type nobody wrote. The cause is one line:
`typeOf` of a literal table was `"array " + typeOf(first element)`, and `typeOf` of an integer
literal is `int`. That is the **first-element** shape of the `BufferElemBytes` bug
[elemwidth-2026-08-27](elemwidth-2026-08-27.md) fixed for buffers, surviving in the safe direction —
so the same three bytes were a `[]byte` when stored into a buffer and a `[]int` when written down.

## 2. Two rules, because the question is bidirectional

**Synthesis.** A literal table's element type is the **join of its elements' exact ranges**,
`V = ⨆ᵢ [eᵢ, eᵢ]`. Sound by construction, on elemwidth's own ground: a literal is its own exact
range, so every element is inside the hull. This is the opposite of a buffer's hazard, where the
danger is a later `set` the inference did not see; a literal is an immutable value and has no later
store (ADR 0018). Unlike a buffer's, the hull does **not** start at zero — `build` zero-fills and a
graph holds exactly what is written.

**Checking.** Where the literal reaches a parameter that declares an element type, **the declaration
decides**, and the literal must fit it. That is ADR 0003's rule — declared at boundaries, inferred
for locals — and it is needed because synthesis alone disagrees with the host: the hull of
`104 105 33` is `33..105`, which on the JVM picks a **signed byte** while the declared
`(array (int 0 255))` is a `short[]`. Bidirectional typing's ordinary split (Pierce & Turner, *Local
Type Inference*), and `declaredElem` is the same rule already written for buffers.

**And the refusal moves to us.** A literal that does not fit is now refused naming the element:

```
a table written here holds 300, which is outside the declared element type (int 0 255)
of the call it is handed to (docs/literal-elements.md)
```

## 3. What it cost, and what changed

| | |
|---|---|
| `emit/target.go` | `LiteralElem` (the join), `LiteralFits` (the check), `declaredForLit` — **one** implementation, shared by both backends, because two would be two rules that can disagree |
| `emit/golang.go`, `emit/java.go` | `typeOf`, the literal emission, and the two places that know a declared element: an argument, and a `let` whose body hands the name to such a call |
| **windows** | deliberately untouched. `emitArrayLit` stores qwords, and element size travels **by name** there — wintables-2026-08-25's hazard, where losing it read a byte array as qwords and gave a wrong answer. A per-target representation is allowed to decline |
| JavaScript | no types; nothing to decide |

**Emission: 2 files changed of 194**, both predicted by the research: `examples/json/tokenize.oro`
and `examples/json/tree.oro` on Go, whose JSON documents go from `[]int{123, 34, …}` to `[]byte`,
with `int(src[i])` on each read — exactly what a narrowed buffer already emits. **Everything else is
byte-identical**, including both programs' differential twins, because those declare their element
on a signature and were already narrow. Proof counts unchanged.

**Benchmark, Go, `-benchtime=20000x`, medians of five:** `BenchmarkTreeGen` **4,747 → 4,688 ns**,
1.3% and inside the 15% noise floor, with `TestTree` still agreeing with both hand-written
implementations. Flat, as the research predicted for Go.

**A correction to the research.** §4 predicted the JVM's element width would be worth about 1.19×
here, citing json-tree-bench. It is worth **nothing measurable, because there is nothing to measure**:
`examples/json/*.oro` are Go-only sources, so no corpus program exercises the Java path at all. The
Java rule is therefore covered by a unit test rather than by a program — *a path nothing runs is a
path nothing checks*, and this is the cheap version of running it.

## 4. Witnesses

`emit/literalelem_test.go`, four tests, each shown to fail against a planted bug:

| test | what it pins | fails when |
|---|---|---|
| the hull, on Go and the JVM | `33..105` → `[]byte` / `byte[]`; `33..300` → `[]uint16` / `short[]`; `-2..3` → `[]int8` / `byte[]` | the old first-element rule is planted |
| a declared element at a boundary | the same literal is `[]byte` on Go and `short[]` on the JVM, through **both** shapes: a direct argument and a `let`-bound name | either context rule is removed |
| a literal that does not fit | our refusal, naming `300` and `int 0 255` | `LiteralFits` is planted to never refuse |
| the join itself | `{9, -3}` and `{-3, 9}` both give `int -3 9` | reading only the first or only the last element |

The two hosts disagreeing about the same literal is the point of the second test: it is what makes
the boundary rule necessary rather than a convenience.

## 5. What it buys

The README's fourth example is now what hamza asked for:

```lisp
(let (array 104 105 33) (fn (src)   ; "hi!"
  …((hex.Encode dst src) (fn (dst n) dst))…))
```

and prints `686921`. A table written as its graph is usable at a host boundary, which is what
[tables.md](../../docs/spec/tables.md) always said it was.

## 6. Not built

- **windows**, per §3.
- **Static data.** staticdata-2026-08-20 measured compile-time materialisation as never a win.
- **A literal at two boundaries that declare different elements.** None occurs; the rule would have
  to refuse rather than pick, and that refusal is not written.
