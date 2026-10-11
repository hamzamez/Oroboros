package core

// ABSTRACT OROBOROS — docs/spec/abstract.md, ADR 0050.
//
// The language is the initial algebra of a many-sorted binding signature Σ, and
// a notation is a rendering of it. These are Σ's trees for the computation's
// terms: what the reader builds (μ, mu.go) from today's notation, and what
// desugaring maps into today's core (D, desugar.go). Every meaning below the
// reader is a homomorphism out of these trees, defined by one clause per
// operator.
//
// THE SORTS ARE GO TYPES. Expr ⊆ Tail (abstract.md §2.5): every expression may
// end a loop's clause, and `again` and a binding around a tail may end one and
// may stand nowhere else. An Expr slot cannot hold an *Again, so ADR 0015's tail
// rule is the type of a field rather than a walk over a term (W3, §3).
//
// Step 1 of abstract.md §13 builds the computation's sort only. Declarations
// (def, sig, variant, use, …) are still read as terms by toForm, and claims are
// carried as today's applications (`the`, a sig's `where`). Where a field holds
// something only resolution can classify — a `case` pattern names a
// constructor, and naming is resolution (W1) — it holds the tree as written,
// and Load decides it, as it does today.

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

// Lam is the λ. Its binders are ranging binders whose sets are absent: today's
// notation writes no claim on a λ (abstract.md §2.2: no set is no claim, not V).
type Lam struct {
	Params []string
	Body   Expr
}

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

// LocalBind is one binder of a local. A buffer is a definitional binder with the
// linear mode (abstract.md §2.2), a cell a state binder.
type LocalBind struct {
	Name   string
	Init   Expr
	Buffer bool
}

// BuildMap is a scoped map buffer, `(build-map m cap … body)`: local state whose
// initializer is an empty map of a capacity (maps.md).
type BuildMap struct {
	Names []string
	Caps  []Expr
	Body  Expr
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

// TRedex is a λ of one parameter applied to one argument, written by hand,
// whose body is a tail. Today's notation treats it as the binding it reduces to,
// since a `let` reads as exactly this redex; abstract.md's W3 would refuse it
// ("no λ lies between them"), and step 1 keeps today's acceptance.
type TRedex struct {
	Param string
	Arg   Expr
	Body  Tail
}

// tlam is provisional: a λ of one parameter whose body is a tail, which is a
// TRedex if it is applied to one argument and an ordinary λ otherwise. It never
// leaves μ.
type tlam struct {
	Param string
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
func (*Again) tailSort()     {}
func (*TLet) tailSort()      {}
func (*TSeq) tailSort()      {}
func (*TMonad) tailSort()    {}
func (*TRedex) tailSort()    {}
func (*tlam) tailSort()      {}

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

func (*PVar) patSort()   {}
func (*PWild) patSort()  {}
func (*PTuple) patSort() {}
func (*PLit) patSort()   {}
