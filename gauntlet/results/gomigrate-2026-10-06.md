# The fallible Go declarations, in the error model

2026-10-06. Step 5 of errors.md §12, its second half: every fallible declaration in `targets/go`
states the shape Go's documentation gives it, and the programs that call them follow.

## 1. Three shapes, read from the documentation

Which shape a call has is a claim about what its value means beside an error, and only the
documentation says. So these are hand claims, checked against Go's types (ADR 0022), and the survey's
generated declarations keep Go's `(T, error)` (ADR 0023).

| shape | type | declarations |
|---|---|---|
| **sum** | `(result.result T error)`: the value means nothing beside an error | `os.ReadFile`, `Create`, `MkdirTemp`, `Getwd`, `Hostname`, `Executable`; `io.ByteReader.ReadByte`; `binary.ReadUvarint`, `ReadVarint`; `strconv.ParseBool`, `Unquote`, `QuotedPrefix`, `UnquoteChar` (a tuple payload); `strings.Reader.ReadByte`, `ReadRune` (a tuple payload), `Seek` |
| **unit + E** | `(result.result (tuple) error)` | `os.WriteFile`, `Remove`, `RemoveAll`, `Rename`, `Mkdir`, `MkdirAll`, `Chdir`, `Setenv`, `Unsetenv`; every `Close`, `CloseWithError`, `Flush`; `WriteByte`; `UnreadByte`, `UnreadRune` |
| **partial success** | `(tuple … (option error))`: the value comes with the error | every `Write`, `WriteString` (io.Writer's contract); `io.Copy`, `CopyN`, `ReadAll`, `ReadFull`, `ReadAtLeast`, every `Read` and `ReadAt`; `fmt.Fprint*`; `hex.Decode`, `DecodeString`, `AppendDecode` ("the bytes decoded before the error"); `strconv.ParseInt`, `ParseUint`, `Atoi`, `ParseFloat` (a documented 0, saturated, or ±Inf value); `WriteTo` |
| **option** | `(option error)` | `bufio.Scanner.Err`, "the first non-EOF error" |

Not fallible, unchanged: the sentinels (`io.EOF`, …), `fmt.Errorf`, and `NumError`'s constructor and
`Unwrap`, which construct or carry an error. A tuple payload composes: H(T) concatenates its factors,
so `UnquoteChar`'s raw call is Go's `(rune, bool, string, error)`.

## 2. What the migration found

1. **A partial file's check skipped every raw call.** The hand-declaration check collects a module's
   declarations by prefix, and `#raw:go/os:Open` has none, so `Open`, `File.Read` and `File.Close` had
   gone unchecked since step 4. Raw calls are now compared under their declared names
   (`emit.DeclaredName`); a planted wrong result type on `Open` fails.
2. **Views compared one result and never several.** A view between methods with two or more results
   compared none of them. They are compared now, with a write-borrow's leading buffer (the argument
   handed back, host-buffers.md) removed, because a view compares the host's methods.
3. **An included method must be retracted in its companion.** Inclusion is a morphism on
   declarations, so retraction now runs after `expandCompanions`. An included copy resolves its variant
   in the module that wrote it (`RetractIn`), and that module's `use` comes with it.
4. **Derived types name manifest types** (`go.int64`), which the unfolding pass had already passed;
   it is idempotent, and runs again after retraction.
5. **A diagnostic named a raw call**, `#raw:go/os:WriteFile's parameter perm…`. It names the declared
   `go/os.WriteFile`, which is what the program wrote.
6. **An unread product binding reached lowering as a closure.** η for products had been applied only
   to a binding that is read. r = (π₁ r, …, πₙ r) holds of every r, so it applies to all of them, and
   a discarded sum keeps its effects and drops its components.
7. **Two laws of the conditional**, on bool = 1 + 1: η, `(if c true false) → c`, and idempotence,
   `(if c a a) → a` for a pure c. They arrived with `(case o none true (some e) false)` and with a
   discard that fails the same way on both arms.
8. **`Term.Equal` called any two bound variables equal.** It had no case for a bound index, so two
   leaves with no kids compared equal. The idempotence law then collapsed `freq`'s
   `(if (< n m) n m)`, a min, to `n`. The gate caught it as 18 fewer operations and a changed loop
   bound. `Equal` now compares indices, and it is α-equivalence: a binder's hints are ignored, so two
   copies of one definition named apart by hygiene are equal.

## 3. The programs

- `examples/lines/lines.oro`: `Scanner.Err` is an option, `Flush` a unit result, `ParseInt` and `Atoi`
  partial successes read through `ok?`. Its behaviour test passes. What it emits changed: the niche
  test is shared, and the commuting conversion pushes the `and` in `summed` into the niche's branches,
  so the range test appears at each of `summed`'s two uses, where it was one connective.
- `acceptance/strconv`, `encoding-binary`, `encoding-hex` reproduce their reference outputs exactly:
  `ok?` and `print-err` read an option as Go prints an error, a nil one `<nil>`.
- The emit tests that used these declarations as examples of multi-result host calls name the raw
  call, which is the host call they test.

## 4. Witnesses

| claim | witness | planted fault |
|---|---|---|
| raw calls are checked against Go | `TestHandDeclarationsAgreeWithTheHost` | `Open`'s result as `bytestring`: refused |
| the conditional's laws, and only where they hold | `TestTheConditionalsLaws`: η, idempotence, a min, an impure condition | `Equal` ignoring indices: the min collapses |
| the migrated programs' outputs | the acceptance suite and `TestLinesFoldsTheLineSplit` | — |

## 5. Not built

- `ignore`, and relevance (step 6): a discarded result is accepted today, as it was.
- Merging a unit slot or two equal slots at a boundary.
