# The Go standard library: what is declared, what stops the rest, and the order to close it

2026-10-06. The standing goal is the whole Go standard library, package by package. This measures where
the ten packages `targets/go` has a file for stand. For every exported name it records what stops the
undeclared ones, and why each cause was not closed. The tool is `gauntlet/stdlib/coverage.go`, so the
table can be regenerated as names close.

## 1. Method

- **The universe** is Go's own API manifest, `$GOROOT/api/go1*.txt` (Go 1.27): every exported function,
  method, type, variable and constant of the ten packages, once each. Deprecated names (4) are not
  counted, and neither are struct fields (21) or interface methods (68): an interface's methods are
  checked against the manifest when the interface is declared (tooling_test.go).
- **The declared set** is every `sig`, `type` and `const` in `targets/go`, read with the language's
  reader. A variadic family counts as declared when its restrictions are (`Fprintf`, `Fprintf2`,
  `Fprintf3`).
- **A cause** is read off the signature where it shows (`func(` is a callback, `...` a variadic). Where
  it does not, the name was read against Go's documentation and source one at a time: a borrowed slice
  looks like any other `[]byte`. Those readings are a table in the tool.

## 2. Where it stands

| package | names | declared | |
|---|---:|---:|---:|
| unicode/utf8 | 19 | 19 | 100% |
| math/bits | 50 | 49 | 98% |
| encoding/hex | 15 | 14 | 93% |
| strconv | 40 | 35 | 88% |
| strings | 82 | 62 | 76% |
| io | 67 | 37 | 55% |
| encoding/binary | 22 | 11 | 50% |
| fmt | 29 | 11 | 38% |
| os | 189 | 35 | 19% |
| bufio | 77 | 11 | 14% |
| **all** | **590** | **284** | **48.1%** |

## 3. The 306 missing names, by cause

| | cause | names | examples | why it was not closed |
|---|---|---:|---|---|
| **L** | **work**: declarable today, not written | **205** | most of `os` (118: the rest of `*File`, `Stat`, `OpenFile`, the process API, `Root`); `bufio`'s `Reader` and `ReadWriter` (50); `io`'s section and offset readers (27); 26 package variables, such as `strconv.ErrRange` and `os.ErrNotExist`, declarable as `io.EOF` is (a nullary `sig`); 27 constants (`os.O_RDONLY`, `ModeDir`) | **program first**: ADR 0022 counts a name supported when it is declared by hand from Go's source, with its algebra stated, checked against the host, and exercised by a program. And **the error model**: 73 of these 122 functions and methods return an error, and so do 69 declared ones. errors-research found the product `(tuple T error)` leaks across hosts and left the model undecided |
| **K** | a type from a package with no file | 33 | `time` (the deadlines, `Chtimes`, `UserTime`); `io/fs` (`ReadDir`, `DirFS`, `FileInfo`, `Root.Stat`); `unicode` (`SpecialCase`); `syscall`; `iter` | those packages are next, each program first. `time` is an algebra of its own: instants, durations, a monotonic clock |
| **A** | **a callback**: a function value handed to the host | 21 | `strings.FieldsFunc`, `Map`, `TrimFunc`, `IndexFunc`; `bufio.Scanner.Split` and the four split functions; `os.Expand`; `strings.SplitSeq`, `Lines`, `FieldsSeq`, whose `iter.Seq` needs the program to supply `yield` | tiers 1 and 2 are specified (callbacks.md) and not built on any backend; deferred to the round of questions |
| **C** | **a borrow that a later call ends** | 9 | `Scanner.Bytes`, `Reader.Peek`, `ReadSlice`, `ReadLine`, `Writer.AvailableBuffer` | the slice aliases the host's buffer until the next call, and the language has no such type. On the question list. The copying forms exist (`Text`, `ReadString`) |
| **I** | out-parameters | 9 | the `fmt.Scan` family | **closable** (§4) |
| **D** | reflection over Go values | 8 | `encoding/binary.Read`, `Write`, `Size`, `Encode`, `Decode`; `ProcessState.Sys` | `interface{}` encoded by struct layout needs records, not built. `ByteOrder`'s methods (work) cover the common case |
| **B** | **implementing a host interface**: the host calls the program's methods | 6 | `fmt.Formatter`, `GoStringer`, `State`, `fmt.Scanner` | a program-made object handed to the host is tier 3, refused by design. A statically named method set is the tier-1 form, the same work as callbacks |
| **G** | a variadic over one type | 6 | `strings.NewReplacer` and its `Replacer`; `io.MultiReader`, `MultiWriter` | **closable** (§4). strings.oro called it "the variadic wall" |
| **H** | a variadic over `any` | 3 | `fmt.Append`, `Appendf`, `Appendln` | closable as restrictions per arity, as `Print` is |
| **E** | complex numbers | 2 | `strconv.ParseComplex`, `FormatComplex` | no complex type, and no program asked |
| **F** | raw handles (`uintptr`) | 2 | `os.File.Fd`, `NewFile` | platform-specific; no program asked |
| **J** | scratch the host may overwrite | 2 | `io.CopyBuffer`, `Scanner.Buffer` | a write-borrow whose contents afterwards mean nothing; `Buffer` would also make the declared token bound false |

**Declarable today**, counting the packages K needs: 522 of 590, **88.5%**, which agrees with the Go
survey's 87.8% for the whole library, an independent check. **With G, H and I closed: 540, 91.5%.**
The language's real walls are A, B, C and D (44 names), plus E, F and J (6).

## 4. Two walls that are not walls, measured

Both were built and run as fragments in a program's own directory (a layer, target-system.md):

- **A variadic over one type is a function of a list**, f : A* → B, and the language has tables. Go's
  own spread passes one:

  ```lisp
  (sig NewReplacerOf ((oldnew (array string))) Replacer pure
       (host expr "strings.NewReplacer(%s...)" (import "strings")))
  ```

  The emitted Go is `strings.NewReplacer(v6...)`, and `(Replacer.Replace r "abcabc")` with the pairs
  a→1, b→2, c→3 printed `123123`.
- **An out-parameter is a result in disguise.** A template that owns the pointer turns it back:

  ```lisp
  (sig SscanInt ((s bytestring)) (tuple int (int 0 1) error) pure
       (host expr "func(s string) (int, int, error) { var x int; n, err := fmt.Sscan(s, &x); return x, n, err }(%s)" (import "fmt")))
  ```

  `(fmt.SscanInt "  42 rest")` gave `42`. It is how `strconv.UnquoteChar`'s template and Win32's
  `__written` cell already work. It costs one declaration per type and arity, a family like `Fprintf`.

**Two headers overstate their walls**: strings.oro ("the variadic wall … the one thing in this package
it blocks") and fmt.oro ("each stores what it reads through a POINTER, and the language has none … That
is a wall"). io.oro's "MultiReader and MultiWriter … are VARIADIC" is the same.

## 5. The order to close it

1. **Decide the error model for host declarations, and implement it**, because 142 declarations depend
   on it (73 to write, 69 written) and the product's leak is a live bug (errors-2026-10-04). It is
   errors-research's questions 1 to 4: sums for host failures, error content, relevance, the partial
   eliminator. Spec first, an ADR.
2. **The closable walls, G, H and I** (18 names), and the three headers corrected: no language work,
   with the fallible ones in the new model.
3. **The work, package by package, program first, in the new model**:
   - `os` first (118 names). It is the largest gap, and where the portable `os` (ADR 0039's
     consequence) and the error kinds (`ErrNotExist`, `ErrPermission`) meet;
   - then `bufio`'s `Reader` (50) and `io` (27).

   The process API (`StartProcess`, `Process.Wait`, `Pipe`) waits for concurrency-research's
   decisions: a child process is a process.
4. **The packages K needs** (33), each program first: `io/fs` (for `ReadDir`, `Stat`), `time`,
   `unicode`.
5. **The language's walls, by what they unlock**:
   - **callbacks, tier 1** (A, 21 names, and B's 6 through static method sets). The largest wall,
     specified in callbacks.md, and the entry point concurrency will need too;
   - then **borrows** (C, 9), which need their research;
   - then records (D), complex numbers (E), raw handles (F) and scratch (J), each when a program asks.
