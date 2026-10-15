# 0064 — A file is a theory named by its path; a declaration is a header and one body; a variant is a set; visibility is public and `export` is an entry point

Date: 2026-10-15
Status: Accepted (hamza, 2026-10-15, on surface-design.md §6.7 and §6.8: items 1, 2, 5–8, then 9 and
10; J's set notation, items 11 and 12, is §6.9's). Amends the surface ADR 0021 drew from its theories, and modules.md §0's `export` row.
Not built: the surface is built when it is chosen, by the reader and the translator.


> **Item 7's sentence on `let` is corrected** by ADR 0054 item 3: J writes no scope head, so `let`
> is no word of J ([spec/j.md](../spec/j.md) §0.2, C2).

## Context

surface-design installment 6 designed declarations and modules, tested on a real program,
`examples/os/du.oro` (du-2026-10-14), and three host packages declared for it.
`gauntlet/surface/decls.go` counted how the corpus declares, and the misfits were restatements the
loader checks and words that name two things:
- a library file's `(module X)` restates the path it was found at, 5 of 5, and the loader refuses any
  other X; a target file's `(target go …)` restates its directory, 35 of 35, also refused otherwise;
- 149 exported names are written twice, at their definition and in a list, and an export with no
  definition is refused;
- 38 of 244 imports restate the default alias; 40 import a companion by its own path, because
  `fs.DirEntry.Name`, theories.md §3.4's spelling, does not resolve;
- `type` names a carrier the model supplies (80) and a set definition (8); theories.md's one record
  x : A [= t] [↦ ρ] is spelled with five words, and the variant with a sixth form;
- `implements` (13) sits apart from its type, a verb for an inclusion;
- `export` means visibility in a library and an entry point in the entry file (modules.md §0).
`gauntlet/surface/visibility.go` measured the public share of Go packages: user-facing ones 76%; our
libraries export 21 of 23 definitions.

## Decision

1. **A library or entry file is one theory, and its path is its name**: no `module` header (theories.md
   §3.1, now to be built). A child module, a companion, or an entry file's further modules are nested:
   J `module X { … }`, D TLA+'s `---- MODULE X ---- … ====`.
2. **A target file is a set of fragments of one model**, each naming its module by absolute path,
   `module go/io/fs { … }`, since a target file's location carries nothing (37 of its 52 outermost
   modules are not its path). **The target is the directory**: no wrapper. Outside a target's directory
   a fragment names its target, `provides go os/host { … }`.
3. **An import is use(path; alias), the alias always present**: the reader fills the path's last segment
   when the source omits it, and the printer omits an alias equal to it, so a restated alias is
   normalized, not refused. **`alias.T.m` names member m of T's companion**, as theories.md §3.4
   writes it; a companion's own import stays legal for a short alias.
4. **Visibility is public by default, and `private` (J) or `LOCAL` (D) marks a hidden name.** A private
   set hides its tags. Visibility is the reduct m|S and carries no code: every call is resolved and
   inlined (ADR 0011).
5. **`export` marks an entry point, and only that**: `export fn main()`. In the entry file the build
   compiles it into a host function and the host takes on its claim (ADR 0028, ADR 0020); in a file
   reached by `use` it is ignored (modules.md §0). An entry point is the dual of a host declaration: one
   brings a host operation in with its claim an axiom, the other sends a definition out with its claim
   the host's obligation.
6. **A declaration is a header and one body, an expression or a host** (theories.md §2's four cells as the
   body's shape). `type` goes: a carrier is `set NumError = host "*strconv.NumError"`, a manifest type a
   set definition. **A host constant's value is a claim**, `MaxRune ∈ {1114111} = host "utf8.MaxRune"`:
   R2 never unfolds it, and the analysis reads the claim.
7. **The top-level forms** (J; D writes the same trees with its own keywords):
   - a value is `cap = 4096`, the block's binder; **`let` leaves the top level**, where it marks nothing;
     it stays the head of a scope with a body (ADR 0054);
   - `fn f(…) (U) { e }` is the same tree as `f = fn(…) (U) { e }`, both read, the named form printed;
   - **`set S = e` is `S = e` with the claim S ∈ subsets(any)**, and `set F(T̄) = e` is
     `fn F(T̄) (subsets(any)) { e }`. **Optional**, as every claim (ADR 0062), except on a host body,
     where it is what makes the body a carrier and not a constant. The translator writes it on every set
     definition. D writes `SET`, with the same rule.
8. **A variant is a set definition, a union of tagged sets**:
   `set os_error = 'not_found ∪ 'exists ∪ 'permission ∪ 'other(error)`.
   - 'c(S) is {'c} × S; a tag with no payload is the unit tagged, {'c} × {()} ≅ {'c}, one value.
     Distinct tags make the union disjoint, so it is the coproduct; ADR 0042's representation is ρ of it.
   - **A tag is an element.** In a set position a value denotes its singleton, η(v) = {v}, and a tag
     applied to a set its image, 𝒫(c)(S) = {c(s) : s ∈ S}: the powerset monad's unit and functor, which
     agree by naturality, 𝒫(c) ∘ η = η ∘ c. In a value position a tag applied to a set is refused (Law S).
   - Nothing is generated: a constructor is a tag applied. Type parameters make a static function on
     sets, `set result(T, E) = 'ok(T) ∪ 'err(E)`, applied as ADR 0063 instantiates. The role markers are
     clauses after the set, `success 'ok relevant`. Closed, finite and non-recursive (theories.md §1.3);
     a tag belongs to one set definition per module.
9. **An inclusion is an axiom, and only a model states one**: `axiom NumError ⊆ error`, a list on the
   right its intersection, `axiom File ⊆ Reader ∩ Writer ∩ Closer`. Legal only in a target file or a
   fragment: a program is extended by definitions and proven lemmas only (ADR 0049), and a model's
   axioms are checked against the host (ADR 0022).
10. **The model's data keeps its words** (`repr`, `structural`, `backend`, `link`, `fact`, `underlying`,
    `max-len`, `big-repr`, `int-repr`), written with the notation's sets: `repr 0..255 host "byte"`.
11. **A companion is a nested module**, its constructor `new` included (ADR 0063). `self` is an ordinary
    parameter name, the conventional one for the value an operation acts on, Go's receiver as the first
    parameter.

## Why not

- **Keep the library header.** It restates the path, and the loader must check that it does; theories.md
  §3.1 had already said a library file declares none.
- **Make a target file one module, its path.** 37 of the 52 outermost modules in target files are
  grouped by theme (`targets/java/lang.oro` holds ten); a file is the glue of its forms, so the
  location of a fragment carries nothing, and forcing it would split files for no meaning.
- **Refuse a restated alias.** It reads to the same tree; refusing a spelling the printer can normalize
  makes a program fail for nothing.
- **Keep the export list.** It writes each name twice and is where the refusal of an export with no
  definition comes from. The interface at a glance is computable from the markers.
- **Default private (Rust, Java).** The errors are asymmetric: a forgotten `export` is caught at the
  first use elsewhere, a forgotten `private` is silent. Here a leaked name carries no code and no
  binary interface, the reduct can be listed by a tool, and libraries hide 2 of 23 definitions, so the
  marker belongs on those.
- **One word for visibility and the entry point**, with everything public. An entry point is compiled
  and its claim crosses to the host: exporting du's helpers makes the build refuse two of them (a sum
  parameter, a function parameter), so "everything public" would mean "nothing builds".
- **Keep `type`.** It names two things, a carrier and a set definition; a carrier is a host body and a
  set definition a definition.
- **A host constant's value as its definiens.** R2 already keeps a realized declaration from being
  unfolded; the value is what the analysis reads, which is a claim, and the survey reads it so ("a
  primitive of no arguments whose result is the EXACT range [v, v]").
- **`let` at the top level.** It marks nothing the binder does not; inside a block J writes `x = e`.
- **`set` required on every set definition.** It would be the one mandatory claim, a restatement the
  compiler must check after resolution; Law S already refuses a set where a value is needed, so leaving
  it out cannot produce a wrong program.
- **A variant as its own form with generated constructors.** ADR 0059 made a tag a value, so the union of
  tagged sets says everything the form said, and nothing needs generating.
- **The inclusion as the set definition's header claim**, `NumError ∈ subsets(error) == HOST "…"`. It
  covers 11 of the 13 (two include a language set defined elsewhere), and a header's claim is an
  obligation proven on the body, which a host body cannot discharge.
- **A section variable for `self`** (Coq's `Variable`, Lean's `variable`). Two of the 63 companion
  operations, the constructors, take none.

## Consequences

- Today's notation's `(module X)` headers, `(target T …)` wrappers, `(export …)` lists in libraries,
  `type`, `const`, `implements` and `variant` all become forms the translator rewrites; the corpus's
  meaning is unchanged, which the byte-identical gate checks when the reader is built.
- The loader's refusals of a mismatched header, a mismatched wrapper and an export with no definition
  disappear with the restatements they checked.
- Resolution gains `alias.T.m` through a companion, which theories.md §3.4 specified and the loader does
  not do today (`fs.DirEntry.Name` is "not bound").
- A program's own sets still wait for types-as-values step 2; until then a `sig` naming a definition as a
  set is refused where it names it (setdef-2026-10-15).
- J's set operators and the ASCII spelling of their symbols are decided separately (surface-design §6.9).
