# 0044 — A partial success's error is relevant

Date: 2026-10-07
Status: Accepted (hamza: "yes, make the partial success error relevant, go ahead", on
relevance-2026-10-07 §6). Amends ADR 0040's shape for a partial success (errors.md §4.3).

## Context

ADR 0040 typed a call that succeeds in part, `io.Reader.Read` or `File.Write`, as ℕ × (1 + E), with
the 1 + E written `(option error)`. ADR 0043 made `(relevant)` mean something. `option` is not
relevant, because a map read may be ignored, so a dropped `File.Write` error compiled silently. So did
those of the other partial successes (`Read`, `ReadAll`, `Copy`, `Fprint*`, `hex.Decode*`,
`strconv.Parse*`), while dropping `os.WriteFile`'s was refused.

## Decision

1. **The error factor of a partial success is `(result (tuple) E)`**, the relevant 1 + E. As sets
   `(option E)` and `(result (tuple) E)` are one, and the retraction is the same: H = [E], so the raw
   call is unchanged and nothing new is emitted. What differs is the role. `option` is the absence of
   a value, which may be ignored. `result` is the presence of a failure, which may not.
   `Scanner.Err`, the same 1 + E alone, follows.
2. **`(option error)` is kept where the host's algebra makes the error ignorable, and the declaration
   says why**, as errors.md §7 already asked: `strings.Builder`'s writes, whose error Go documents as
   always nil (so `Builder.WriteByte`, declared a relevant unit + E in step 5b, moves to the option),
   and `bufio.Writer`'s writes, whose errors are sticky and reported by `Flush`.
3. **A write through an interface is relevant** whatever its receiver: `fmt.Fprintf` to an
   `io.Writer` does not know the writer is a `bufio.Writer`. The program writes `(ignore …)` and says
   why.
4. `fmt.Print*` is unchanged. Its declarations already forget Go's `(n, err)`, a projection fmt.oro
   states. A program that must see the error writes `Fprintln` to `os.Stdout`.

## Consequences

- 34 declarations in seven files change type, and 5 keep the option with a reason.
- Emission is byte-identical: `lines.oro` writes `(ignore …)` with its reason at four sites, and
  `ignore` erases.
- Three acceptance programs read the error through the result's constructors. One,
  `encoding-hex`'s `projection`, had dropped `DecodeString`'s error unread. It now says why, which
  is the point.
- Three emit tests that drop an error in passing write `(ignore e)`.

## Why not

- **Keep `option` and make `option` relevant**: a map read's `none` is not a failure, and a lookup
  whose absence is ignored is correct code. Relevance belongs to the role, so it belongs to the
  declaration that has the role.
- **A third variant for "an optional failure"**: `(result (tuple) E)` already is 1 + E with the
  failure role and the `success` marker, so `try` and `expect` work on it. A third type would
  duplicate it, as ADR 0040 found for 1 + E itself (unit-2026-10-06).
- **Declare every write to a stream ignorable, as Go's `errcheck` does for `fmt.Print*`**: that is a
  claim about the host's algebra, and only the sticky writer and the builder have one.
