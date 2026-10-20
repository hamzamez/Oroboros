package core

// ABSTRACT OROBOROS — docs/spec/abstract.md, ADR 0050.
//
// The language is the initial algebra of a many-sorted binding signature Σ, and
// a notation is a rendering of it. These are Σ's trees for the computation's
// terms: what J's reader builds (jparse.go), and what desugaring maps into
// the core term (D, desugar.go). Every meaning below the
// reader is a homomorphism out of these trees, defined by one clause per
// operator.
//
// THE SORTS ARE GO TYPES. Expr ⊆ Tail (abstract.md §2.5): every expression may
// end a loop's clause, and `again` and a binding around a tail may end one and
// may stand nowhere else. An Expr slot cannot hold an *Again, so ADR 0015's tail
// rule is the type of a field rather than a walk over a term (W3, §3).
//
// Step 1a built the computation's sort and step 1b the declarations' (Decl,
// below; ADR 0051): a definition is a name bound to a value, and a λ carries its
// claims. Where a field holds something only resolution can classify — a `case`
// pattern names a constructor, and naming is resolution (W1) — it holds the tree
// as written, and Load decides it, as it does today.

// Tail is a term that may end a loop's clause: an expression, a back edge, or a
// binding whose body is a tail.
type Tail interface{ tailSort() }

// Expr is a term of the computation. Every Expr is a Tail.
type Expr interface {
	Tail
	exprSort()
}

// Pat is a pattern: what a binder destructures and what an arm tests (§2.3).
type Pat interface{ patSort() }

// ---- Expressions: the primitive operators (abstract.md §2.4) ----

// Var is a name: a variable, a definition, an owned name of Σ₀, or a host's.
type Var struct{ Name string }

// Lit is a literal: an integer, a float, a string, a boolean, or an integer past
// the word, which the reader writes as Horner's form over 10¹⁵ (bigLiteral).
type Lit struct{ T *Term }

// Lam is the λ. Its binders are ranging binders (abstract.md §2.2), and its
// Claim, when one is written, gives their sets, the result's set and the pre-
// and postcondition (ADR 0051: a definition is a named λ, so the λ carries what
// a definition claims). With no claim a binder has no set, which is no claim,
// not V (settypes §10).
type Lam struct {
	Params []string
	Body   Expr
	Claim  *Claim
	// Borrows are the parameters that are places in the body (ADR 0052 item 3,
	// spec/local.md §2.1): a buffer the caller lends for the call, written in
	// program order. The λ's value is then these buffers first, then the body's.
	// Only a rendering in program order builds one; today's notation threads.
	Borrows []string
}

// Claim is what a λ claims (ADR 0051, D2): each parameter's set, the result's
// set, a precondition φ over the parameters and a postcondition ψ over them and
// the result. Their meaning is two sets, P = {x̄ ∈ S₁ × … × Sₙ : φ} for the
// arguments and R = {result ∈ U : ψ} for the value; the tree keeps a set per
// binder because a value's representation is chosen from its own set (ρ) and a
// parameter's set is checked on its argument (ADR 0028).
//
// The result binder is `result`, today's spelling (ResultName); several results
// are a tuple set, whose components a notation may name with a tuple pattern
// (ADR 0051, D3). Today's notation writes a claim apart from its λ, as a `sig`
// that binds its own names, so Params keeps the claim's spelling of each binder,
// α-equivalent to the λ's, and At where the claim was written: both are what D
// needs to give today's forms back, and neither is meaning.
type Claim struct {
	Params []ClaimParam
	Result Expr
	Pre    Expr
	Post   Expr
	At     int
	// ResultModes are the result binder's usage, one per component of a tuple
	// result or one for a single result, and empty when none is written: a
	// write-borrow hands its buffer back as a linear component (ADR 0050,
	// decision 2; ADR 0051, D3: the result binder's pattern may be a tuple).
	ResultModes []Mode
}

// ClaimParam is one parameter's set, its mode, and the claim's name for it. The
// name is "" for a parameter given only its set, positionally.
type ClaimParam struct {
	Name string
	Set  Expr
	Mode Mode
}

// Mode is how a binder is used or passed, which is not what its values are
// (ADR 0050, decision 2; settypes §2.5). A buffer is a table bound linearly; a
// cell the host writes is passed by value-result, inout (ADR 0048). On a
// variadic declaration's list, inout is each trailing argument's.
type Mode string

const (
	NoMode Mode = ""
	Linear Mode = "linear"
	InOut  Mode = "inout"
)

// Apply is application: of a definition, a host name, a λ, or an owned name.
// `tuple` is one of Σ₀'s names (abstract.md §1.3), the product former, and D
// realizes its definition, the Church tuple.
type Apply struct {
	Fn   Expr
	Args []Expr
}

// Index is a table's or map's application to a point (ADR 0050, decision 6).
// Today's notation does not mark it, so μ never builds one: whether `(t i)`
// indexes data is known only once t's value is, after reduction. A rendering
// that marks data application builds it, and D reads it as application.
type Index struct{ Table, At Expr }

// Let binds definitional binders in sequence around an expression.
type Let struct {
	Binds []LetBind
	Body  Expr
}

// LetBind is one binder of a let: a name, `_`, or a tuple of names.
type LetBind struct {
	Pat   Pat
	Value Expr
}

// Case is the one eliminator of a sum (ADR 0050, decision 5). Its patterns
// name constructors, so they are kept as written and resolved in Load.
type Case struct {
	Scrut Expr
	Arms  []CaseArm
}

// CaseArm is a pattern, as written, and its body.
type CaseArm struct {
	Pat  Expr
	Body Expr
}

// Loop is iteration: state binders, and arms whose bodies are tails
// (ADR 0015). A `match` is a loop whose arms have patterns over its state
// (match.md): Match marks the spelling, and Vars are then the names the
// scrutinees became.
type Loop struct {
	Vars  []string
	Inits []Expr
	Arms  []LoopArm
	Match bool
}

// LoopArm is one guarded alternative. A loop's clause has a Guard; a match's
// clause has one pattern per state variable and an optional When. Else is the
// last arm, which tests nothing (W4).
type LoopArm struct {
	Guard Expr
	Pats  []Pat
	When  Expr
	Else  bool
	Body  Tail
}

// Local is a scope of local state (ADR 0047): each binder is a buffer, decided
// by its initializer's form, or a cell.
type Local struct {
	Binds []LocalBind
	Body  Expr
}

// LocalBind is one binder of a local. In today's notation a buffer is a
// definitional binder with the linear mode (abstract.md §2.2), threaded, and a
// cell a state binder. In program order (ADR 0052, spec/local.md §2.1) a buffer
// is a state binder whose value is a table, a Place, written by Write.
type LocalBind struct {
	Name   string
	Init   Expr
	Buffer bool
	Place  bool
}

// Write is assignment to state, in program order (abstract.md §2.4's assign;
// ADR 0047, ADR 0052): each place takes the value in the same position, every
// value evaluated in the state before any is written, Dijkstra's concurrent
// assignment when there are several. One place with no index is a cell's write
// or a buffer's whole value.
type Write struct {
	Places []Place
	Values []Expr
}

// Place is what a write assigns: a state binder, an element of a buffer
// `b[i]`, or a component of a record of a table of tuples `b[i][j]`.
type Place struct {
	Name string
	At   []Expr
}

// BuildMap is a scoped map buffer, `(build-map m cap … body)`: local state whose
// initializer is an empty map of a capacity (maps.md).
type BuildMap struct {
	Names []string
	Caps  []Expr
	Body  Expr
	// Place says the map buffers are places, written in program order
	// (ADR 0058): only a rendering in program order builds one.
	Place bool
}

// The is a claim: e's value lies in S (abstract.md §2.4). It is carried to the
// logic as today's ascription and erased at emission (ADR 0037's pattern).
type The struct{ Set, E Expr }

// EmptyList is `()` where a parameter list may be empty and the form around it
// is still read as a term: a top-level `(def f () body)` or `(sig f () …)`,
// until declarations have their own sort.
type EmptyList struct{}

// ---- Derived operators (abstract.md §2.7): each is an equation over the
// primitives, kept because it names a law ----

// If is 𝔹's eliminator.
type If struct{ C, A, B Expr }

// And, Or and Not are the connectives as 𝔹's eliminator.
type And struct{ Xs []Expr }
type Or struct{ Xs []Expr }
type Not struct{ X Expr }

// Cond is guarded choice: the first true guard selects its body, and the last
// guard is else.
type Cond struct{ Arms []CondArm }

// CondArm is a guard, absent for else, and its body.
type CondArm struct {
	Guard Expr
	Body  Expr
}

// Seq is sequencing: a binding whose name is discarded (ADR 0010).
type Seq struct{ Xs []Expr }

// Monad is `try` or `expect`, the exception monad's bind and its give-up
// (ADR 0040). Items are the pattern, the value and `expect`'s reason, as
// written; Load checks their arity and resolves the pattern.
type Monad struct {
	Form  string
	Items []Expr
	Body  Expr
}

// ---- Tails that are not expressions (abstract.md §2.5) ----

// Again is the back edge, with one argument per state variable of the nearest
// loop.
type Again struct{ Args []Expr }

// TLet is a let whose body is a tail.
type TLet struct {
	Binds []LetBind
	Body  Tail
}

// TSeq is a sequence whose last term is a tail.
type TSeq struct {
	Xs   []Expr
	Last Tail
}

// TMonad is a `try` or `expect` whose success arm is a tail (ADR 0041).
type TMonad struct {
	Form  string
	Items []Expr
	Body  Tail
}

// ---- Patterns ----

// PVar binds a name; PWild binds nothing; PTuple binds a product's components;
// PLit tests a literal, an integer or a boolean (match.md §2).
type PVar struct{ Name string }
type PWild struct{}
type PTuple struct{ Names []string }
type PLit struct{ T *Term }

func (*Var) tailSort()       {}
func (*Lit) tailSort()       {}
func (*Lam) tailSort()       {}
func (*Apply) tailSort()     {}
func (*Index) tailSort()     {}
func (*Let) tailSort()       {}
func (*Case) tailSort()      {}
func (*Loop) tailSort()      {}
func (*Local) tailSort()     {}
func (*BuildMap) tailSort()  {}
func (*The) tailSort()       {}
func (*EmptyList) tailSort() {}
func (*If) tailSort()        {}
func (*And) tailSort()       {}
func (*Or) tailSort()        {}
func (*Not) tailSort()       {}
func (*Cond) tailSort()      {}
func (*Seq) tailSort()       {}
func (*Monad) tailSort()     {}
func (*Write) tailSort()     {}
func (*Again) tailSort()     {}
func (*TLet) tailSort()      {}
func (*TSeq) tailSort()      {}
func (*TMonad) tailSort()    {}

func (*Var) exprSort()       {}
func (*Lit) exprSort()       {}
func (*Lam) exprSort()       {}
func (*Apply) exprSort()     {}
func (*Index) exprSort()     {}
func (*Let) exprSort()       {}
func (*Case) exprSort()      {}
func (*Loop) exprSort()      {}
func (*Local) exprSort()     {}
func (*BuildMap) exprSort()  {}
func (*The) exprSort()       {}
func (*EmptyList) exprSort() {}
func (*If) exprSort()        {}
func (*And) exprSort()       {}
func (*Or) exprSort()        {}
func (*Not) exprSort()       {}
func (*Cond) exprSort()      {}
func (*Seq) exprSort()       {}
func (*Monad) exprSort()     {}
func (*Write) exprSort()     {}

func (*PVar) patSort()   {}
func (*PWild) patSort()  {}
func (*PTuple) patSort() {}
func (*PLit) patSort()   {}

// ---- Declarations (abstract.md §2.6, ADR 0051) ----

// Decl is a top-level form of a module. At is where it was written, which D
// needs to give today's forms back in their order.
type Decl interface{ declSort() }

// DefDecl binds a name to a value. A definition is a named λ (ADR 0051, D1): its
// claims are its λ's, and a constant is a name bound to any other value.
type DefDecl struct {
	Name  string
	Value Expr
	At    int
	// Host is a host declaration's body (abstract.md §2.6, ADR 0051, D4): the
	// definition's Value is then a λ with no body, whose claim is an axiom
	// checked against the host (ADR 0022), realized by this template.
	Host *HostBody
}

// HostBody is the host's realization of a declaration: the template's kind and
// text, the host clauses (import, lib, checked, jump), and the markers the
// declaration states of the operation (pure, index, variadic).
type HostBody struct {
	Kind     string
	Template string
	Clauses  []Expr
	Markers  []string
}

// ClaimDecl is a claim written for a name with no λ to carry it: a `sig` whose
// name has no definition in the file, more than one, or one that is not a λ of
// its arity. Today's notation allows it and Load decides it; no program in the
// corpus writes one (abstract1b-2026-10-11).
type ClaimDecl struct {
	Name  string
	Claim *Claim
}

// VariantDecl declares a closed sum: its name, its type parameters, ranging over
// sets of values, and its constructors and role markers in the order written.
type VariantDecl struct {
	Name   string
	Params []string
	Items  []VariantItem
	At     int
}

// VariantItem is a constructor, with its payload's set or none, or a role
// marker: `success` naming a constructor, or `relevant` (errors.md §2).
type VariantItem struct {
	Ctor     string
	Payload  Expr
	Success  string
	Relevant bool
}

// ModuleDecl, UseDecl and ExportDecl are resolution (ADR 0011); ForeignDecl is a form another
// reader owns or Load refuses — a target fragment `provides`, or `prim` and
// `target`, which a program may not write — kept as written.
type ModuleDecl struct {
	Path string
	At   int
}
type UseDecl struct {
	Path, As string
	At       int
}
type ExportDecl struct {
	Names []string
	At    int
}
type ForeignDecl struct {
	E  Expr
	At int
}

// TermDecl is a top-level term that is no declaration.
type TermDecl struct {
	E  Expr
	At int
}

func (*DefDecl) declSort()     {}
func (*ClaimDecl) declSort()   {}
func (*VariantDecl) declSort() {}
func (*ModuleDecl) declSort()  {}
func (*UseDecl) declSort()     {}
func (*ExportDecl) declSort()  {}
func (*ForeignDecl) declSort() {}
func (*TermDecl) declSort()    {}
