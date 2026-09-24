# The host's string is the host's

2026-09-24. The build of [ADR 0030](../../docs/decisions/0030-a-hosts-string-is-the-hosts.md), from
hamza's question: in `(sig Quote ((s string)) string …)`, is `string` ours or Go's? It is ours: a
sequence of scalar values, Σ*, stored on Go as a Go `string`. A Go `string` is any byte sequence, so
every result declared `string` claimed the host returns valid UTF-8, and nothing checked that claim.
Spec: [strings.md §8](../../docs/spec/strings.md).

## 1. What was measured first

- **A false claim.** `strconv.Unquote("\xff")` is the byte 0xFF, typed `string`. `Unquote("\xe6")` and
  `Unquote("\x97\xa5")` were each invalid, and their `concat` printed `日`. So the free monoid
  manufactured a scalar neither operand contains (Theorem 2).
- **Three decoders.** Over 26 ill-formed and edge sequences:
  - JS's `TextDecoder` is the Unicode Standard's maximal-subpart substitution d, in every case;
  - Java's `new String(b, UTF_8)` differs from it only on encoded surrogates, one U+FFFD where d gives
    three;
  - Go's portable `text-of` did not decode at all, and Go's `range` decode gives one U+FFFD per bad
    byte.
- **57 hand-declared Go results mention `string`**, read one at a time against Go's source:
  - 41 provably return valid UTF-8 on valid input: ASCII output, escaping, Theorem 1's
    synchronization, concatenation, rune-wise case maps, or slices of ours;
  - 11 can return arbitrary bytes;
  - 5 are map types.

## 2. What is built

- **`go.bytestring`**, a type owned by module `go` and realized as Go `string`, with
  `(implements string go.bytestring)`: ours is a subset, and the coercion emits nothing.
- **`go.text`**, which is d, and **`go.valid`**, which decides valid UTF-8. `go.text`'s fast path is
  `utf8.ValidString`, since d is the identity there.
- **The 11 results** are `go.bytestring`:
  - `Unquote`;
  - the six `fmt.Sprint` forms;
  - `go/os`'s raw `text-of`, `Args` and `Getenv`;
  - `string-of-bytes`.

  **Eight parameters** take `go.bytestring`, because the host reads arbitrary bytes there: the five
  `unicode/utf8` string functions, and `Quote`, `QuoteToASCII` and `QuoteToGraphic`.
- **The portable `os.text-of` is d on Go, JS and Java**, and the portable `os.Args` and `os.Getenv` on
  Go decode by d:
  - **JS:** unchanged;
  - **Go:** writes out d from Table 3-7;
  - **Java:** keeps the lenient decoder's answer when it holds no U+FFFD, and runs d otherwise.
- **The checker.** `SameHostType` identified two names that realize one host spelling. That now never
  applies to a language type, so host bytes cannot pass as ours.
- **The generator** spells every Go `string` as `go.bytestring` (ADR 0023). A string constant is
  `string` when its value is valid UTF-8.
- **The differential harness** runs the JVM with UTF-8 output.

## 3. What failed first

- **`SameHostType` would have undone the design.** `agree` accepts any two names that share a
  realization, in both directions, before it consults subtyping. So `go.bytestring` and `string` would
  have been interchangeable, and `go.text` never required. Found by reading the checker before
  building, and pinned by a test.
- **The type's first home made it unnameable.** Declared at the target's root, it was keyed
  `bytestring`, and a program's `go.bytestring` did not resolve. Moved into module `go`, where
  `go/io.Writer`-style ownership gives it the qualified key. `implements` is legal only at the root.
- **Two acceptance programs relied on the unsoundness.** `encoding-hex` fed binary data to a hex
  encoder through the portable `text-of`, and `unicode-utf8` asked `ValidString` about invalid bytes
  made that way. With `text-of` now d, both printed differently from Go. They meant the host's string,
  and now use `go/os`'s raw `text-of`.
- **The generator change reached a binding.** `tally`'s Go binding is written against generated
  declarations, and its `Itoa`, `Split` and capture results are now the host's. It decodes each where
  it enters the portable core.
- **Java's first fast path was slow.** The strict decoder lacks the lenient one's ASCII intrinsic, and
  cost 3.6× on 1 KB of ASCII. The lenient decoder plus a check for U+FFFD is exact and costs 1.00×.
- **A Java template failed to parse, and the loader said the module did not exist.** A bad escape in
  `lib/os/java.oro` gave "`(use os)` matched no file on the search path" for every program. It is
  reproduced and filed as its own task.
- **Heredocs ate backslashes** in edit scripts four more times. Each was caught before a file changed.

## 4. Checked, and how each check fails

- **The witness:** the `Unquote`-then-`concat` program is refused ("s is go.bytestring, but string is
  required here"). Decoding each half gives three U+FFFD, all valid, and decoding the whole gives `日`.
- **`emit/hoststring_test.go`**, four tests: host bytes are not ours, ours is the host's, the way back
  is d, and a language type is not a host type by realization.
- **`text-of`**, a differential case over 18 sequences on Go, JS and Java. Its expected answer comes
  from an implementation of d written from Table 3-7, checked against `TextDecoder` and taken from no
  host.
- **`utf8-widths`** now runs on four targets, where it ran on two.

| planted | caught by |
|---|---|
| identity by realization for a language type | two unit tests |
| no edge `string ≤ go.bytestring` | two unit tests |
| `Unquote` returning ours again | `TestTheHostsStringIsNotOurs` |
| Go's `text-of` keeping raw bytes | `text-of`: the targets disagree |
| Go's `text-of` decoding per byte | `text-of` |
| Java's `text-of` as the lenient decoder alone | `text-of` |
| the JVM printing in the platform charset | `text-of` |

## 5. What it cost

- **Emission:** 5 files change, each only by the new decoder: Go's `text-of` and `Args` in `freq`,
  `jsonfmt` and `wc`, and Java's `text-of` in `freq` and `jsonfmt`. The new case adds 4. The proof
  counts are unchanged.
- **Run time, on valid input:**

  | | before | after | ratio |
  |---|---:|---:|---:|
  | Go `text-of`, 1 KB ASCII | 185.6 ns | 202.2 ns | 1.09× |
  | Go `text-of`, 64 KB ASCII | 5,938 ns | 8,575 ns | 1.44× |
  | Go `text-of`, 1 KB non-ASCII | 182.8 ns | 849.9 ns | **4.65×** |
  | Java `text-of`, 1 KB and 64 KB | | | 1.00× to 1.02× |

  Go's non-ASCII cost is the validity scan, about 0.65 ns per byte. The old version was cheaper
  because it checked nothing.
- **Compile time:** 1.09× median.
- **The full check** passes, tooling included.

## 6. Not built, and named

- **JS and Java strings** are UTF-16 code-unit sequences, and a lone surrogate is their analogue of a
  stray byte. Their hand-declared string results are to be read against the same rule when a program
  first needs one. Windows has no string results.
- **A lossless path for Go file names** that are not UTF-8. The portable `os.Args` decodes, so a
  program cannot reopen a file whose name holds a stray byte; `go/os`'s raw `Args` can.
- **Concatenation of host strings** (Go's `+` on `go.bytestring`) and host path operations are not
  declared. The one program that built a path decodes the directory first.
- **The loader's diagnostic** for a fragment that fails to parse, filed separately.
