# The words audit, first pass: which words bought nothing

2026-11-19. Plan item 6 of [assessment-2026-11-18](assessment-2026-11-18.md) §5 (hamza, 2026-11-18: "ask
what other words did we add, that didn't need to be, didn't buy us anything"). **Research: verdicts,
nothing withdrawn.** A withdrawal is a reversal ADR, and each is hamza's to decide (§4).

## 1. The method

Each word gets three questions:
- **The program**: write the smallest program that uses it, then the same program with only the
  other words. What fails, exactly?
- **ADR 0130's test**: does a program observe what the word states (meaning, in the header or the
  language), or not (representation, in the host clause or the `repr`)?
- **What it bought**: a refusal it enables, a fact it carries, a representation it chooses, each
  named with the program that needs it.

**The measurement first** (`gauntlet/surface/words.py`, §5): every keyword's uses across the
corpus's `.oro` files, with comments and string literals removed. A count describes the measurer
(CLAUDE.md), so a low count is where to look, not a verdict.

## 2. The two found before this pass

| word | what it said | what the language already had | outcome |
|---|---|---|---|
| `in`, after a struct parameter's set | `_In_`'s const T* | the call's argument term, `&x` (`ref(x)` then) | withdrawn, ADR 0130 |
| `ui(s₀, step) { … }` | the window procedure's extent | the cell's own `var` scope | never added (sketches §1.4c, ADR 0133) |

Both were representation or structure put in the language, and both came from not asking what the
language already expressed.

## 3. Verdicts

### Candidates: a word that buys nothing today

| word | uses | what it says | what fails without it | verdict |
|---|---:|---|---|---|
| `inout` vs `cell(T)` | 0 vs 4 | the same declaration twice: `x ∈ T inout` and `x ∈ cell(T)` (abstract.md's D: `D(x ∈ S inout) = (x (cell ⌜S⌝))`) | nothing: they are synonyms (fit-research's F1) | **`cell(T)` is the leak, `inout` the word** (hamza, 2026-11-19: "we use linear and not buffer()"). A set ranges over values, and a cell is a place, not a value (ADR 0049, 0084). `T` is the set and `inout` the mode, as `Array(V)` and `linear` are; `cell` and `buffer` are the core's types, D's image. The count, 4 against 0, measured the leak: the target files, and ADR 0133's `s ∈ cell(S)`, wrote the core's form. *Corrected 2026-11-19*: this pass first kept `cell(T)` for its uses |
| `jump` | 0 | `(jump "l")`, a predicate that branches instead of computing, on MASM's templates (windows-target.md) | nothing: the MASM backend is deleted (ADR 0080, mastretire-2026-11-03), and no file writes `jump`. The reader still parses it | **dead**: its backend retired |
| `underlying` | 0 | `(underlying T (int LO HI))`, a named host type over an integer set, read by the hand-declaration checker and ignored by the compiler (target-files.md §2a) | nothing today: no declaration writes it | **unused**, though its job may still be wanted (Go's `type FileMode uint32`); hamza's whether to keep it for that or drop it until a declaration needs it |
| `cdecl` | 0 | a C function's convention, after its call | nothing: the printer treats every convention but `stdcall` as C's default, so `call F cdecl` and `call F` print the same | **a synonym of its absence**. As documentation of the prototype it says what the prototype's absence of `__stdcall` says |

### Reserved for what is not built

`spec`, `lemma`, `assert` (0 uses): the logic's definitions (ADR 0062) and assertions, reserved so a
program cannot take them as names before they are built. Their reservation is decided, so the
question is not whether they buy something today.

### The header modes, where `in` was: each states a meaning

The two found before were a mode (`in`) and a construct (`ui`). So the modes get the three questions
in full.

- **`linear`** (`s ∈ Array(0..255) linear`, ReadFile's). *Without it*, the table is a value: the
  host's writes into it are writes into a copy no program can see again, so `(got, r) =
  ReadFile(h, b[lo:hi])` would read nothing into b, and every buffer the program fills by a host
  would have to be returned as a fresh table. *Observed*: yes, the program reads the bytes the host
  wrote (ADR 0070, a header is the caller's view of a borrow), and a call that lends b may not be
  reordered past a read of b. *Bought*: in-place reads into a lent buffer (`os`, `freq`, `jsonfmt`
  write through it) and R1's uniqueness. **Meaning; keep.**
- **`pure`** (on 736 host declarations). *Without it*, a host call is impure by default (ADR 0010):
  never dropped, duplicated or reordered, and its argument let-bound. A call whose value is unused
  stays, `strings.ToUpper(s)` included. *Observed*: no single answer changes, but which programs
  reduce to which residuals does. A pure call substituted into one use is work moved, never repeated
  (ADR 0071), and a pure host call may observe a live buffer (ADR 0069), which an impure one may not.
  It cannot be derived: a host's body is opaque. **Meaning (an effect bit); keep.**
- **`out`** (`fmt.Sscan(s, out Int, out go.bytestring)`). *Without it*, a cell parameter needs a
  cell in scope, `var a := 0; fmt.Sscan(s, a)`, whose set is ℤ and cannot choose what Sscan parses
  into. `out τ` is a fresh cell whose set the program chooses, and Go's `Sscan` reads τ from it.
  *Observed*: yes, τ chooses the parse. ADR 0131 removed it where the declaration already knows τ (an
  `_Out_`), which was the redundant half, and kept it where the program chooses. **Meaning; keep.**
- **`inout`**: the mode of a value-result parameter, as `linear` is of a borrow. *Without it*, the
  program writes the core's `cell(T)`, a place where J writes sets. *Observed*: yes, the value comes
  back written. **Meaning; keep, and refuse its synonym `cell(T)`** (above).

### In use, with the job each does

Each of these is written by a program or a declaration, and removing it loses the job named. ADR
0130's test is answered by where each word stands: the host clause's and the target model's words are
representation, and they stand in the host clause, the `repr` and the target files, where ADR 0130
puts representation. None is a mode or a construct that restates what another word says. That was
the shape of `in` and `ui`, and of three of the four candidates above.

| word | uses | the job | where it is needed |
|---|---:|---|---|
| `out` | 2 | a fresh cell at a host's cell parameter, its set the program's choice | `roundtrip`'s `out set { … }` and `pe`'s `out Head` at `os.ReadAt`, and the emit tests' `fmt.Sscan`; ADR 0131 kept it exactly there. (The counter's 76 are mostly a parameter named `out`.) |
| `linear` | 79 | a borrowed buffer, uniqueness the program keeps (ADR 0020, 0070) | every buffer parameter of `os`, `freq`, `jsonfmt` |
| `pure` | 736 | the effect bit β reads (ADR 0010) | every host declaration that can be dropped or reordered |
| `index` | 5 | a host array read lowered to the IR's own index, so its bounds are proven (ir/lower.go) | the JVM's and JavaScript's array reads |
| `variadic` | 26 | Go's `f(xs...)`, one declaration over a table (ADR 0046) | `fmt.Print` and its family |
| `checked` | 21 | each operation's trapping form under `-checked` (ADR 0088) | `+ − × / <<` on every target |
| `narrow` | 1 | Go's `q = q[:n]`, the host's bounds-check proof (1.96×, bce-2026-08-15) | Go's tables |
| `structural` | 6 | a kind the book's tutorial targets bind | the book's normal-form chapters |
| `comma_ok` | 1 | Go's `(v, ok)` read as an `Option` (typevalues.md §7) | `errors.AsType` |
| `success`, `relevant` | 2, 1 | a variant's roles: what `try` continues on, what must be used (ADR 0040, 0043) | `lib/result` |
| `invariant` | 1 | a loop's invariant, Hoare's rule | `lib/os/windows.oro`'s read loop |
| `as`, `with`, `mod`, `record` | 75, 11, 47, 25 | an import's alias; a record's override; the residue; a record's former | throughout |
| `where`, `ensure` | 153, 80 | a declaration's domain and postcondition, obligations and facts | throughout the targets |
| `host`, `expr`, `stmt`, `import`, `lib`, `call`, `stdcall`, `NULL` | many | the host clause: representation, where ADR 0130 puts it | every declaration |
| `repr`, `word`, `niche`, `realize`, `glue`, `triple`, `runtime`, `c`, `pack`, `size`, `len`, `read`, `write`, `include`, `data`, `fact`, `link`, `build`, `artifact`, `backend` | 1–149 | the target model's data: representations, realizations, layout, the toolchain | the target files |

**Tonight's additions are no new words.** ADR 0133's surface reuses `set Q = …` (a claim as a set's
value, ADR 0053), `repr Q c(…) stdcall` (the struct layout's form), and `cell(S)` at a definition's
parameter. Its reader forms `callback-set` and `conv` are the inventory's, not J's.

## 4. For hamza

1. **`cell(T)` in J**: refuse it at a parameter, naming `x ∈ T inout`; amend W7 so `inout` may stand
   on a program definition's parameter (ADR 0133 item 6); respell item 6, `s ∈ S inout`, and the
   uses (SetFilePointer, PrintDlgA, the tests, `wndproc-win.oro`, `cell-param`). The emitted code is
   unchanged, both spellings being one core term.
2. **`jump`**: withdraw, with the MASM backend it served. If yes, the reader's clause, the
   inventory, windows-target.md marked historical.
3. **`underlying`**: withdraw until a declaration needs it, or keep for Go's named integer types.
4. **`cdecl`**: withdraw as a synonym of no convention, or keep as the prototype's explicit word.
5. **The rest of §3's table** has its job named and its place checked against ADR 0130's test. A
   word-by-word program for each of the host clause's and the model's words is not written. They are
   representation in representation's places, the class neither of the two earlier finds came from.
   It is owed only if hamza wants the pass exhaustive.

## 5. The measurement

`python gauntlet/surface/words.py .`, 2026-11-19, on commit `3ae3d996` plus
ADR 0133's work in progress. Columns are uses outside comments and strings, by area.

| word | targets | lib | examples | cases | stdlib |
|---|---:|---:|---:|---:|---:|
| `inout` | 0 | 0 | 0 | 0 | 0 |
| `jump` | 0 | 0 | 0 | 0 | 0 |
| `underlying` | 0 | 0 | 0 | 0 | 0 |
| `cdecl` | 0 | 0 | 0 | 0 | 0 |
| `spec`, `lemma`, `assert` | 0 | 0 | 0 | 0 | 0 |
| `invariant` | 0 | 1 | 0 | 0 | 0 |
| `comma_ok` | 1 | 0 | 0 | 0 | 0 |
| `relevant` | 0 | 1 | 0 | 0 | 0 |
| `success` | 0 | 1 | 0 | 1 | 0 |
| `narrow` | 1 | 0 | 0 | 0 | 2 |
| `index` | 5 | 0 | 0 | 0 | 0 |
| `structural` | 6 | 0 | 0 | 0 | 0 |
| `checked` | 21 | 0 | 0 | 0 | 0 |
| `variadic` | 26 | 0 | 0 | 0 | 0 |
| `out` (the word, and a parameter so named) | 0 | 0 | 68 | 0 | 8 |
| `linear` | 40 | 10 | 23 | 6 | 0 |
| `pure` | 724 | 12 | 0 | 0 | 0 |
