# Relevance, and `ignore`

2026-10-07. Step 6 of errors.md §12: a value of a `(relevant)` type must be used, and `ignore` is
the one way to drop it. ADR 0043; the specification is errors.md §7.

## 1. What it is

**The rule.** Weakening is denied for one declared type (Anderson and Belnap; Walker, ATTAPL ch. 1).
The eliminators are additive: [f, g] : A + B → C takes f and g in one context. So a variable is
used by `(if c a b)` when c uses it or both a and b do. `(abandon w)` uses everything vacuously
(0 is initial). So does `(again …)`, since a terminating execution leaves a loop by an exit.

**Where.** At β, where a binder the program wrote meets its value. The body checked is the λ as
written, and the value is normalised if impure (ADR 0010) or taken as written if pure. Relevance is
not invariant under the equations the reducer uses after that point (η for products, the
conditional's idempotence), so the residual cannot decide it (ADR 0043, "Why not").

**What is relevant.** A value whose tail is a relevant constructor's tuple, binder `#k!`, or a tuple
containing one. Retractions build a fallible host call's result with the model's constructors, so
the host call and every definition over it, the portable `os` included, are relevant with no
annotation.

**Hand-offs.** `(drop r)` uses r exactly when `drop`'s parameter is used on every path of its body.
That is usage inference, terminating because no definition reaches itself (ADR 0014).

**`ignore` is !_A : A → 1.** On a tuple-valued term its reduction is the eliminator with a constant
body, by the universal property. η for products extends to n = 0, η for the terminal object.

## 2. What it measured

- **The corpus discards nothing.** Of 524 runs, none was refused by the new rule, and the emission
  gate shows one change: `lib/result.oro` compiled alone, a refusal whose text prints the term with
  the new binder `#k!`. Step 5 had already moved every fallible call site to `case`.
- **So the corpus gets a witness.** `examples/errors/discard.oro` drops `os.WriteFile`'s result and
  is refused on Go, JavaScript and Java with *the result of os.WriteFile is a value of a relevant type
  (spec/errors.md §7), and it is discarded: use it, or discard it with (ignore …)*. Windows refuses
  it earlier, having no `os` cell. `examples/errors/ignore.oro` is the same program with the discard
  written down. Compiled by this commit, it is **byte-identical** on all three hosts to `discard.oro`
  compiled by the commit before.
- Compile time: the sweep read 1.12× and 1.15× the baseline in two runs, inside the gate's rule (1.5× and 250 ms).

## 3. What failed first

1. **A hand-off through a let-bound name was accepted.** `(let r (write n) (drop r))`. The reducer
   normalises a λ's body before β, so `drop`'s β had already reduced `(drop r)` to `0` with `r`
   abstract, and no later step saw `r` handed on. A scoped set of relevant names did not help, for the
   same reason. What fixed it is the inference above, read off the source.
2. **Two mechanisms were caught by no fault**: the scoped names, and η carrying the mark on the
   tuple that replaces a relevant binding. Under normal order beneath binders every hand-off is
   reduced while its variable is abstract, so both were dead. They were removed, not kept unwitnessed.
3. **The first witness for the slot exemption did not reach the slot.** An impure payload arrives as
   a let-bound name. A pure relevant payload, `(case (ok (ok n)) …)`, does.
4. **`emit/libos_test.go` dropped `Setenv`'s result**, and the rule refused it. It now writes
   `(ignore …)`.

## 4. Found on the way

- **`cmd/check` never ran `ir/plan`, `ir/golang`, `ir/js`, `ir/java` or `ir/x86`'s tests.** The step
  ran `go test ./ir/`, the package alone. `ir/plan`'s `TestABignumReadIsNotMovedPastAWriteIntoIt` had
  failed since step 5b, because its target had no library path for `lib/result`. The step now runs
  `./ir/...`, and the test passes `lib/`.
- **The type checker named a raw call**, *in argument 2 of #raw:os/host:WriteFile*. It now names the
  declaration, `os/host.WriteFile`, as the contract diagnostics have since step 5b.

## 5. Witnesses

| rule | witness (`core/relevance_test.go`) | planted fault, caught |
|---|---|---|
| a relevant value is recognised | `(seq (write n) 1)` and two more refused | `relevant` answers no |
| `if` is additive | `(if c (check r) 0)`, `and`'s short circuit, `try`'s failure arm refused | `||` for `&&` |
| a crash uses every variable | `(if c (check r) (abandon "x"))` accepted | abandon not vacuous |
| `again` continues | a loop whose exit checks r accepted | again not a use |
| a hand-off uses as the receiver does | `(drop r)` refused, `(check r)` accepted | every hand-off a use |
| a pure constructor, as written | `(seq (ok 1) 1)` refused | pure arguments unchecked |
| the body is the λ as written | `(seq (ignore r) 1)` accepted | the reduced body |
| the compiler's binders are exempt | a relevant payload in a `case` accepted | every binder checked |
| η for the unit; `ignore` erases | `(seq (ignore (write n)) 1)` reduces as `(seq (opt n) 1)` | η only for n ≥ 2 |

## 6. Not built, and one proposal

- Named in errors.md §7.4: a relevant value from a pure computation passed unnormalised; a relevant
  payload in a `case` arm; a loop variable; a hand-off to a function computed at run time.
- **Proposal, for hamza: a partial success's error should be relevant.** §4.3 types `io.Writer.Write`
  as ℕ × (1 + E) with `(option error)`, and `option` is not relevant, so a dropped `File.Write`
  error is silent today, as are those of the other partial successes (`Read`, `ReadAll`, `Fprint*`,
  `ParseInt`…). As a set, 1 + E is `(option E)` and `(result (tuple) E)` alike. What separates them
  is the role: absence of a value (a map read, which may be ignored) against presence of a failure
  (which may not). The algebra says the factor is the relevant 1 + E, `(result (tuple) E)`. The cost:
  every caller of those declarations uses or ignores the error, and a decision for `fmt.Print*`,
  whose errors Go's own `errcheck` excludes by default. Like `bufio.Writer.Write` (§7), those could
  be declared not fallible, if the reason is stated.
