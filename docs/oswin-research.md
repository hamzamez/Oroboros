# `os` on windows: the portable operating system over Win32

Date: 2026-11-15. hamza: "go ahead with os on windows, always mathematical and algebraic". The
6b′ item CLAUDE.md names: `os` on windows (`read-loop`, `text-of`), then the gauntlet's programs
there. This document is the algebra of each name of `os/host` on Win32, the walls it meets, and
which of them earlier decisions already settle.

## 1. What is asked

`lib/os.oro` is one definition over a cell `os/host` per host (oskinds-2026-10-06). The portable
call is the host's call composed with the classification of its failure:

  os.ReadFile = (id + h) ∘ host.ReadFile,   h : E_host → K.

So windows needs a cell, `lib/os/windows.oro`, declaring exactly the names Go's declares, at the
same sets (`emit/portable_test.go`, both directions):
- `ReadFile`, `WriteFile`, `ReadAt`;
- the three predicates `not_found?`, `exists?`, `permission?` over `error`;
- `text_of`, `Args`, `Getenv`.

Its neighbour `io` (`print_line`, `print`, `print_int`) needs a windows cell too, since the gauntlet's
programs print through it. That is `lib/win/print`'s arithmetic over `WriteFile` to standard
output, with no wall.

**Win32 has no `ReadFile(path)`.** Its files are handles, so each name is a short program over
kernel32, which is what the README of `lib/os` predicted: a library written in the language. Every
other cell is a host template; this one is the first written in J, which ADR 0089 prefers in any case
(a realization is a term over the host's operations).

## 2. The algebra of each name

Write F : Path ⇀ B\* for the store.
- **ReadFile** = get. It is the composite
  open ; size ; read\* ; close
  of the handle API, where open is `CreateFileA(p, GENERIC_READ, FILE_SHARE_READ, NULL,
  OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, NULL)`, size is `GetFileSizeEx`, read\* is `ReadFile` until
  the table is full or a read answers 0 bytes, and close is `CloseHandle`. Each step is partial,
  and the composite fails at the first step that fails, after closing what it opened. So it is a
  Kleisli composite in the exception monad A + E, with close as the handler's finalizer. That is
  `try` (ADR 0040) and a `case`, nothing new.
- **WriteFile** = put, the same shape with `CREATE_ALWAYS` and `GENERIC_WRITE`. Put-get holds as on
  the other hosts: `CREATE_ALWAYS` truncates, so after a put of d a get gives d.
- **ReadAt** = pread: open; `SetFilePointerEx(h, off, NULL, FILE_BEGIN)`; read\* into b[0 .. m),
  m = min(len b, size − off); close. Its claim `n ≤ len b′ ∧ len b′ = len b` is the loop's
  invariant, so the definition proves it rather than declares it.
- **The predicates**: Win32's error is a DWORD from `GetLastError`. Go's own windows port
  classifies it (`src/syscall/syscall_windows.go`, `Errno.Is`), and the cell takes Go's map, so a
  failure lands in the kind Go-on-Windows gives it:
  - not found: `ERROR_FILE_NOT_FOUND` 2, `ERROR_PATH_NOT_FOUND` 3, `ERROR_BAD_NETPATH` 53;
  - exists: `ERROR_FILE_EXISTS` 80, `ERROR_ALREADY_EXISTS` 183, `ERROR_DIR_NOT_EMPTY` 145;
  - permission: `ERROR_ACCESS_DENIED` 5.

  These are three finite subsets of the DWORDs, disjoint, so h is a function of the code, and its
  case order fixes nothing (as lib/os.oro says of the three hosts).
- **Getenv** = `GetEnvironmentVariableA(k, buf, n)`, a two-call protocol. A first call with n = 0
  answers the size with its NUL, or 0 with `ERROR_ENVVAR_NOT_FOUND` (203), which is the absent
  point, ↦ "". A second call fills a table of that size. The variable can change between the calls,
  so the fill is retried while its answer exceeds the table; that is a loop whose variant is the
  environment's, bounded by a count.
- **Args** = parse ∘ `GetCommandLineA`. The command line is one C string, and argv is its parse by
  the Microsoft C runtime's rules, which Go's `os.Args` on windows also implements (`commandLineToArgv`):
  - white space separates arguments outside quotes, and `"` toggles quoting;
  - 2n backslashes before a `"` are n backslashes and the quote's toggle, 2n + 1 are n backslashes
    and a literal `"`, and backslashes before anything else are themselves;
  - `""` inside quotes is a literal `"`;
  - argv[0], the program, is read to the next quote or white space with no escapes.

  It is a fold over the bytes, a Mealy machine whose state is (quoting, pending backslashes): a
  counter automaton, so a `loop`, not a finite transducer.
- **text_of** = d, the Unicode Standard's maximal-subpart decode B\* → Σ\* (ADR 0030), the same
  function on every host. §3.3 is its wall.

## 3. The walls, and what already decides them

### 3.1 A host call into a sum: decided, by ADR 0040 and 0089

Go's `(T, error)` is T × (1 + E), with `nil` the niche of its error type (errors.md §4.1). Win32 has
no type-wide niche. Each function names its own sentinel (`INVALID_HANDLE_VALUE`, `FALSE`, 0), and
the error is a second read, `GetLastError`. errors.md §4.4 sketched a per-declaration form for it,
`(fails (sentinel s) (error GetLastError))`.

**It is not needed.** The retraction r : B × DWORD → B + E is r(b, e) = err e if b = s, else ok b,
and in a cell written in J it is a term:

    h = kernel32.CreateFileA(...)
    if h == invalid { 'err(kernel32.GetLastError()) } else { 'ok(h) }

The two calls are impure, so the effect discipline (ADR 0010) keeps `GetLastError` after the call
and before any other, which is exactly Win32's contract. ADR 0089's rule is that a realization is a
term over the host's operations, and a template only for an operation the host has. r is no host
operation, so it is a term, and §4.4's `fails` form would be a template of a term.

The error type is a set: `set error = 0..2^32-1` in the windows target, the DWORDs. 0
(`ERROR_SUCCESS`) is included because Win32 does not promise a failing call sets the code. The
failure is the sentinel's, not the code's.

### 3.2 Out-parameters: decided, by ADR 0048

`ReadFile`'s `lpNumberOfBytesRead` and `GetFileSizeEx`'s `PLARGE_INTEGER` are pointers the callee
writes. ADR 0048 settled this for Go: a C function f : X × T\* → R that keeps no address is
observably X × T → R × T (Reynolds' `new`, read operationally), a cell passed to a host is the
address of a fresh copy, and `out τ` is a cell for one call. Its item 6 left windows open ("Win32's
fixed-type cells need a stack slot"). That is the realization, not a decision:
- **On the IR**, `cell-fresh`, `cell-ref` and `cell-get` already exist. ir/llvm prints each cell as
  an `alloca` of its C type in the function's entry block, which is Go's "declared once at the
  function's top" for the same measured reason (ADR 0048's Why not). Then `store`, the call, and a
  `load` with its `zext`/`sext`.
- **In the C ABI** (ADR 0094), a parameter `cell(T)` with T a C integer type is `T*`, `ptr` to that
  slot. And a byte table `Array(0..255)` is its elements' address, s + 8 (llvm.md §4), as a C
  string is but with no NUL asked. It is a borrow, read-only, or a write-borrow when `linear`
  (ADR 0070), and its post-state is the caller's again after the call.
- **A buffer at an offset** (`ReadFile` into b[lo ..)) stays a template, as `WriteFile`'s already
  is (kernel32.oro): the address b + 8 + lo is no C type, and ADR 0089 allows a template for the
  host's own operation. Its count becomes a cell, and the target's `@__written` global goes.

A C string *from* Win32 (`GetCommandLineA`'s answer) is the other half of ADR 0091, waiting on a
trigger, and `Args` is that trigger: a helper of the string layer, the bytes up to the NUL copied
into a fresh byte table (strlen and a copy). It is total, and it answers B\*, not Σ\*, so it needs
no claim about the bytes. They become text through text_of, as on Go, where `Args` decodes by d.

### 3.3 Text: the wall

On windows Σ\* is represented by its UTF-8: a string is a byte table (strings.oro), and the
abstraction function A : V → Σ\* is a monoid isomorphism from V ⊂ B\*, the valid UTF-8 (UTF-8 is a
code, so V is free on the scalars' encodings). d is then A ∘ r, where r : B\* → V is the
maximal-subpart repair: each maximal ill-formed subpart becomes EF BF BD. On this representation
text_of is r followed by the identity ι_V : V ↪ Σ\*.

r is a loop over bytes, which the language writes. **ι_V is a primitive whose domain is V, and
nothing in the language decides V.** That is the wall. Every other host's text_of is a template
declared to be d (Go writes d in Go, JavaScript has `TextDecoder`, Java writes it after a lenient
decode), so they meet V inside the host and never state it.

The algebra of V. V is a regular language: Table 3-7 of the Unicode Standard is a small DFA
(Hoehrmann's has nine states). So V = φ⁻¹(F) for the homomorphism φ : B\* → M into the DFA's transition monoid, which is
finite (Eilenberg: V is recognizable). ν of ADR 0091 was the special case M = (𝔹, ∨). So V can be
decided as ADR 0091 decided `nulfree`: by φ on a term (a literal, a concatenation, whose φ is the
product in M) and by a guard at run time. Three ways to meet it:

- **V1, text_of as a target helper**, d written in LLVM IR beside `concat` and `ascii`, its claim
  the declaration's (ADR 0022), checked by the differential case. This is the other hosts' level of
  trust, and nothing enters the language. But d is about 60 lines of hand-written IR that no
  analysis reads, the kind of kernel hamza prefers in the language (2026-11-15, Q1).
- **V2, `valid` in Σ_L and ι_V its witness** (ADR 0091's pattern). `utf8valid(b)` would be φ's
  acceptance, decided on a term by φ and at run time by a guard that runs the DFA, and
  `utf8(b) where utf8valid(b)` would be the inclusion ι_V. text_of on windows is then
  `r = repair(b); if utf8valid(r) { utf8(r) } else { abandon(…) }`, everything but the identity
  written in J. The guard's false arm is unreachable when r is right, which the differential case
  checks, and a fault planted in r reaches it. Its cost is a second linear pass, the DFA again.
  Valid input, the common case, can take the guard first: `if utf8valid(b) { utf8(b) } else {
  utf8 of the repair }`, one pass, as Go's and Java's fast paths are. It adds two names to Σ_L and
  a realization of each on every target: on Go, `utf8.Valid` and the conversion; on JavaScript and
  the JVM, their decoders' strict mode.
- **V3, the decision proves r's image in V.** r is a rational transduction, and the image of a
  regular language under one is regular, so r(B\*) ⊆ V is decidable (Berstel 1979). On the IR it
  would be a new abstract domain, the DFA state of a buffer's written prefix, carried by the store
  chain. It is the most exact, and the most to build. The guard of V2 would then be proven, so V3
  extends V2 rather than replacing it.

**Recommendation: V2**, with V3 as its later strengthening when a program pays for the guard's
pass. It is ADR 0091's construction for the next recognizable language, it keeps the kernel in the
language, and the representation's soundness is checked at the one place it is assumed. V1 is the
cheaper option, and the algebra does not prefer it: a claim about bytes that no check reads.

## 4. What it should measure

- `read-loop` and `text-of` un-skipped: windows agrees with the three hosts and the expect lines.
- `examples/io/roundtrip.oro` on windows, the portable names' behaviour test
  (`TestEveryPortableNameRunsOnEveryHost`), so a missing path is `not_found` there too.
- Then the gauntlet's file programs on windows (`wc`, `freq`, …) at hand-written speed, against
  hand-written C over kernel32, or Go on windows: under the standing measure, faster than Go.

## References

- The Unicode Standard 15, §3.9, Table 3-7 (well-formed UTF-8) and "U+FFFD Substitution of Maximal
  Subparts".
- Microsoft, "Parsing C++ command-line arguments" (the C runtime's rules); Go's
  `src/os/exec_windows.go`, `commandLineToArgv`, and `src/syscall/syscall_windows.go`, `Errno.Is`.
- Win32: `CreateFileA`, `GetFileSizeEx`, `ReadFile`, `SetFilePointerEx`, `CloseHandle`,
  `GetCommandLineA`, `GetEnvironmentVariableA`, `GetLastError`.
- Reynolds, J. C. "The essence of Algol", 1981 (`new`); ADR 0048.
- Eilenberg, S. *Automata, Languages and Machines* A, 1974 (recognizable sets, the syntactic
  monoid); Berstel, J. *Transductions and Context-Free Languages*, 1979 (rational transductions).
