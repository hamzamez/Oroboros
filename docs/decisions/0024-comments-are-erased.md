# 0024 — A comment never carries meaning; documentation is a term

Date: 2026-09-20
Status: Accepted

## Context

Comments have been in the language since the first commit and were never specified: one grammar line
in [core-0.md §1.2](../spec/core-0.md), which was itself wrong about where a comment ends. hamza
asked whether that was the whole of it. It was.

Writing [spec/comments.md](../spec/comments.md) forced one question that is a decision rather than a
description: **may a comment ever mean something?** Two shapes were on the table — a doc comment
attached to a `def`, and a pragma such as `; nolint` or `; @doc` — and both are ordinary in
neighbouring languages.

The question is not academic here. **Half the corpus is comment** — 2,032 comment-only lines of 4,047 in
`examples/` and `lib/`, and 217 of `freq.oro`'s 425 — and the assessments' balance count excludes
them, so the channel carrying most of the project's reasoning is the one the compiler is defined not
to read.

## Decision

**A comment never carries meaning.** It is gap between tokens, erased by the lexer, with no position,
no attachment and no representation past `core.Read`.

**There are no doc comments and no pragmas.** If documentation is ever to be read by a program, it
arrives as a **form** — data the reader keeps, a declaration about a name beside `sig` — not as a
comment.

Nothing is built, because nothing consumes it.

## Why not

**A comment map, as Go, Java and Rust have.** The parser keeps the comments and a convention says
which construct each one belongs to.

- For a comment attached to a **term** this is not merely expensive, it is undefined: there is no
  transport rule under reduction. δ copies a definition into every one of its uses, β deletes the
  branch that is not taken, and call-by-need let-binds a value at a site the programmer did not
  write. A comment on a term has **no image** under any of them, so the reducer would have to invent
  one for an object with no semantics — and the residual is what every backend sees.
- For a comment attached to a **top-level form** it is coherent: godoc works on the AST, before
  anything reduces. But the cost is real and it is paid in the *language*, not in a tool: the
  attachment rule ("the comment group immediately preceding, with no blank line") becomes part of
  what a program means, and every tool that reads or writes source must agree on it. Go's own
  `ast.CommentMap` and gofmt's attachment behaviour exist because that rule is not as simple as it
  sounds.
- And it buys nothing today: there is no documentation generator, no package site and no editor
  integration to consume it. [ADR 0023](0023-a-generator-does-not-make-a-claim-it-cannot-justify.md)
  is the neighbouring instinct — do not build a channel nothing reads.

**A docstring in the body, as Lisp, Python and Elixir have.** `(defun f (x) "doc" body)` is a term,
so it needs no comment map, and that is the right *idea* — it is this ADR's second half. But it does
not fit the surface: `def` takes **one** body ([def.md §4](../spec/def.md)), and adding an optional
leading string would recreate exactly the ambiguity the `def` shorthand was designed to refuse, where
a form's meaning depends on the shape of an element rather than on its position. When documentation
is wanted, a separate form is both cheaper and more honest.

**A pragma — a comment the compiler reads.** `; nolint`, `; @doc`, `//go:build`. This is the worst of
the three: a lexical construct that changes compilation is a second language hidden inside the one
that is specified, and it is invisible to the grammar that defines the first. This project's rule is
the opposite one — **anything that changes what is legal is a declaration the checker reads**
([ADR 0021](0021-declarations-are-theories.md)) — and `where`, `ensures`, `sig` and `provides` are
where that lives.

**A datum comment, `#;`, which is a smaller version of the same mistake.** Refused with a reason
specific to this language: our forms are discriminated by arity and parity — `def`'s shorthand by
"a list of names", `let`'s grammar by odd or even, `cond`/`loop`/`match` by clauses in pairs — so
`#;` would let a comment change *which construct a form is*. Scheme can afford it because nothing in
Scheme is parity-discriminated in that way. [comments.md §6](../spec/comments.md).

## Consequences

**Easy.** A comment is free, in the strongest sense: `parse` is not injective, so no pass can be
affected by one, and no emission can differ because of one. The emission baseline proves it on every
run.

**Hard, and it is the one real cost.** Because the compiler cannot see a comment, **the only thing
that can lose one is a program that rewrites source** — and one did, twice, on 2026-09-19
([letflat §6](../../gauntlet/results/letflat-2026-09-19.md)). The obligation is now written down
([comments.md §5](../spec/comments.md)): *a source rewriter preserves every gap, or refuses the edit
and names the site.*

**Committed to.** No documentation generation until something wants it, and when it does, the
decision is which consumer it serves — not whether to teach the lexer to mean something. A diagnostic
can never quote the programmer's own words about the line it is refusing.
