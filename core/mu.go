package core

import "fmt"

// μ, TODAY'S NOTATION READ INTO ABSTRACT OROBOROS — docs/spec/abstract.md §10.
//
// The reader reads a list's elements and then calls mu on them, so a tree is
// built bottom-up in exactly the order today's reader desugared, and an error
// is reported where it always was. Every check reads the IMAGE of an element,
// D of its tree (desugar.go), which is the term today's reader inspected at
// that point, so every refusal and every message is unchanged. What is new is
// what μ returns: a tree of Σ, from which D builds the core term.
//
// THE TAIL RULE IS A SORT (W3, ADR 0015). `again` builds an *Again, which is a
// Tail and not an Expr, and so are the bindings whose body is one. A constructor
// that needs an Expr where it receives a Tail DEMOTES it: the `again` becomes an
// application of the name `again`, which is what it denotes outside a tail
// position, and the result is marked STRAY. A loop refuses a clause body that is
// stray; anywhere else a stray `again` stays the free name it is, refused later
// as unbound, as today. This replaces the walkers that enforced the rule on
// terms, checkClauseBody and noAgain: the sort decides where a jump may stand,
// and only a loop consults the mark.
//
// LISTS IN BINDER POSITIONS ARE READ AS LISTS. Today's reader desugared every
// list as a term before its form looked at it, so a parameter list, a loop's
// bindings or a match's scrutinee list whose first element is a reserved word
// (`(match (not b) …)`) was first rewritten as that form. μ validates on the
// image, so every refusal is unchanged; it takes the elements from the list as
// written, so on the one shape where the two differ and today accepted — a
// match scrutinee list headed by a reserved word — it reads the scrutinees the
// program wrote.

// tree is the tree the reader built for one datum. elems are the elements of
// the list a tree was read from: a form that is its operand's identity, `(and
// x)` or `(let b)`, returns the operand's tree, and its list stays the
// operand's.
func (r *reader) tree(k *Term) Tail { return r.abs[k] }

// image is D of a datum's tree: the term today's reader built for it.
func (r *reader) image(k *Term) *Term { return r.d.term(r.abs[k]) }

// expr is a datum's tree in an expression position, demoted if it is a tail.
func (r *reader) expr(k *Term) Expr { return r.demote(r.abs[k]) }

func (r *reader) exprs(ks []*Term) []Expr {
	out := make([]Expr, len(ks))
	for i, k := range ks {
		out[i] = r.expr(k)
	}
	return out
}

// demote reads a tail in an expression position. An `again` there is the name
// `again` applied, a jump nothing can take, and the tree is stray.
func (r *reader) demote(t Tail) Expr {
	var e Expr
	switch t := t.(type) {
	case Expr:
		return t
	case *Again:
		e = &Apply{Fn: &Var{Name: "again"}, Args: t.Args}
	case *TLet:
		e = &Let{Binds: t.Binds, Body: r.demote(t.Body)}
	case *TSeq:
		e = &Seq{Xs: append(append([]Expr(nil), t.Xs...), r.demote(t.Last))}
	case *TMonad:
		e = &Monad{Form: t.Form, Items: t.Items, Body: r.demote(t.Body)}
	case *TRedex:
		e = &Apply{Fn: &Lam{Params: []string{t.Param}, Body: r.demote(t.Body)}, Args: []Expr{t.Arg}}
	case *tlam:
		e = &Lam{Params: []string{t.Param}, Body: r.demote(t.Body)}
	}
	r.stray[e] = true
	return e
}

// mark makes n stray if any of the given subtrees is.
func (r *reader) mark(n Expr, kids ...Expr) Expr {
	for _, k := range kids {
		if r.stray[k] {
			r.stray[n] = true
		}
	}
	return n
}

// mu reads one list. `kids` are its elements, each already read.
func (r *reader) mu(kids []*Term, line int) (Tail, error) {
	head := ""
	if kids[0].Kind == KName {
		head = kids[0].Name
	}
	switch {
	case head == "again":
		return &Again{Args: r.exprs(kids[1:])}, nil
	case head == "let":
		return r.muLet(kids, line)
	case head == "seq":
		return r.muSeq(kids, line)
	case head == "and" || head == "or":
		return r.muConnective(head, kids), nil
	case head == "not":
		if len(kids) != 2 {
			return nil, fmt.Errorf("line %d: not takes one term", line)
		}
		x := r.expr(kids[1])
		return r.mark(&Not{X: x}, x), nil
	case head == "cond":
		return r.muCond(kids, line)
	case head == "values":
		return nil, fmt.Errorf("line %d: `values` is spelled `tuple` (spec/data.md §3)", line)
	case head == "tuple":
		if len(kids) == 2 {
			return nil, fmt.Errorf("line %d: tuple takes no terms, the unit, or two or "+
				"more; a tuple of one is just the value", line)
		}
		args := r.exprs(kids[1:])
		return r.mark(&Apply{Fn: &Var{Name: "tuple"}, Args: args}, args...), nil
	case head == "loop":
		return r.muLoop(kids, line)
	case head == LocalName:
		return r.muLocal(kids, line)
	case (head == "build" || head == "build-map") && len(kids) >= 4:
		if head == "build" {
			return nil, fmt.Errorf("line %d: a scope of local state is `local`, whose binder takes its "+
				"initial value: (local b (table n 0) body) for a buffer of n zeros (spec/local.md); "+
				"the core form (build n (fn (b) body)) is unchanged", line)
		}
		return r.muBuildMap(kids, line)
	case head == "match":
		return r.muMatch(kids, line)
	case head == "fn" || head == "λ":
		return r.muLam(kids, line)
	case head == "if" && len(kids) == 4:
		c, a, b := r.expr(kids[1]), r.expr(kids[2]), r.expr(kids[3])
		return r.mark(&If{C: c, A: a, B: b}, c, a, b), nil
	case head == "case" && len(kids) >= 4 && len(kids)%2 == 0:
		n := &Case{Scrut: r.expr(kids[1])}
		r.mark(n, n.Scrut)
		for i := 2; i < len(kids); i += 2 {
			arm := CaseArm{Pat: r.expr(kids[i]), Body: r.expr(kids[i+1])}
			r.mark(n, arm.Pat, arm.Body)
			n.Arms = append(n.Arms, arm)
		}
		return n, nil
	case (head == "try" || head == "expect") && len(kids) >= 4:
		items := r.exprs(kids[1 : len(kids)-1])
		body := r.tree(kids[len(kids)-1])
		if b, ok := body.(Expr); ok {
			return r.mark(r.mark(&Monad{Form: head, Items: items, Body: b}, items...), b), nil
		}
		return &TMonad{Form: head, Items: items, Body: body}, nil
	case head == AscribeName && len(kids) == 3:
		s, e := r.expr(kids[1]), r.expr(kids[2])
		return r.mark(&The{Set: s, E: e}, s, e), nil
	}
	return r.muApp(kids), nil
}

// muApp reads an application. A λ of one parameter whose body is a tail,
// applied to one argument, is the binding it reduces to (TRedex). The core loop
// form's body is its own loop's, so only its initial values can be stray
// (iteration.md: an `again` there jumps to that loop).
func (r *reader) muApp(kids []*Term) Tail {
	fn := r.tree(kids[0])
	args := r.exprs(kids[1:])
	if l, ok := fn.(*tlam); ok && len(args) == 1 {
		return &TRedex{Param: l.Param, Arg: args[0], Body: l.Body}
	}
	n := &Apply{Fn: r.demote(fn), Args: args}
	if v, ok := n.Fn.(*Var); ok && v.Name == "loop" && len(args) >= 1 && r.d.term(args[0]).Kind == KFn {
		return r.mark(n, args[1:]...)
	}
	return r.mark(r.mark(n, n.Fn), args...)
}

// muLam reads `(fn (x…) body)`.
func (r *reader) muLam(kids []*Term, line int) (Tail, error) {
	if len(kids) != 3 {
		return nil, fmt.Errorf("line %d: fn takes a parameter list and one body", line)
	}
	params, err := paramList(r.image(kids[1]), line)
	if err != nil {
		return nil, err
	}
	body := r.tree(kids[2])
	if _, ok := body.(Expr); !ok && len(params) == 1 {
		return &tlam{Param: params[0], Body: body}, nil
	}
	b := r.demote(body)
	return r.mark(&Lam{Params: params, Body: b}, b), nil
}

// muLet reads the flat binding form, `(let LHS VALUE … BODY)` (binding.md). The
// pairs are checked right to left, the order today's reader bound them in.
func (r *reader) muLet(kids []*Term, line int) (Tail, error) {
	rest := kids[1:]
	// Pairs, then one body: an ODD number of elements. An even one is either
	// the old spelling or a body someone forgot.
	if len(rest)%2 == 0 {
		if len(rest) == 2 {
			return nil, fmt.Errorf("line %d: let binds a name to a value: "+
				"`(let x VALUE BODY)`; the old `(let VALUE (fn (x) …))` spelling is "+
				"refused (spec/binding.md)", line)
		}
		return nil, fmt.Errorf("line %d: let takes NAME VALUE pairs and ONE body "+
			"(spec/binding.md)", line)
	}
	body := r.tree(rest[len(rest)-1])
	if len(rest) == 1 {
		return body, nil // (let b) is b: the body is nesting's identity (binding.md §4)
	}
	binds := make([]LetBind, len(rest)/2)
	for i := len(rest) - 3; i >= 0; i -= 2 {
		p, err := r.letPat(rest[i], line)
		if err != nil {
			return nil, err
		}
		binds[i/2] = LetBind{Pat: p, Value: r.expr(rest[i+1])}
	}
	if b, ok := body.(Expr); ok {
		n := &Let{Binds: binds, Body: b}
		for _, bd := range binds {
			r.mark(n, bd.Value)
		}
		return r.mark(n, b), nil
	}
	return &TLet{Binds: binds, Body: body}, nil
}

// letPat reads a let's left-hand side: a name, `_`, or `(tuple a b …)`. A name
// obeys exactly the rules a λ's parameter does, checked in one place.
func (r *reader) letPat(lhs *Term, line int) (Pat, error) {
	img := r.image(lhs)
	if img.Kind == KName {
		ps, err := paramListOf("let", &Term{Kind: KApp, Kids: []*Term{img}}, line)
		if err != nil {
			return nil, err
		}
		if ps[0] == "_" {
			return &PWild{}, nil
		}
		return &PVar{Name: ps[0]}, nil
	}
	names, ok := patternNames(img)
	if !ok {
		return nil, fmt.Errorf("line %d: let binds a name or `(tuple a b …)`; a pattern that "+
			"may not match belongs in `case` (spec/binding.md §5)", line)
	}
	list := &Term{Kind: KApp}
	for _, n := range names {
		if n.Kind != KName {
			return nil, fmt.Errorf("line %d: a tuple pattern binds names; write a second "+
				"`let` for a nested one (spec/binding.md §5)", line)
		}
		list.Kids = append(list.Kids, n)
	}
	ps, err := paramListOf("a tuple pattern", list, line)
	if err != nil {
		return nil, err
	}
	return &PTuple{Names: ps}, nil
}

// muSeq reads `(seq a b …)`, a binding whose name is discarded.
func (r *reader) muSeq(kids []*Term, line int) (Tail, error) {
	if len(kids) < 3 {
		return nil, fmt.Errorf("line %d: seq takes two or more terms", line)
	}
	pre := r.exprs(kids[1 : len(kids)-1])
	last := r.tree(kids[len(kids)-1])
	if l, ok := last.(Expr); ok {
		xs := append(pre, l)
		return r.mark(&Seq{Xs: xs}, xs...), nil
	}
	return &TSeq{Xs: pre, Last: last}, nil
}

// muConnective reads `and` and `or`. Each is a monoid's operation: with no
// operand it is the unit, and with one it is that operand.
func (r *reader) muConnective(head string, kids []*Term) Tail {
	if len(kids) == 2 {
		return r.tree(kids[1])
	}
	xs := r.exprs(kids[1:])
	if head == "and" {
		return r.mark(&And{Xs: xs}, xs...)
	}
	return r.mark(&Or{Xs: xs}, xs...)
}

// clauses checks the shape every guarded chain shares (ADR 0015, McCarthy 1960):
// pairs, the last guard `else`, and no `else` before it. `body` checks each
// pair's body as the chain is walked, which a loop needs and a cond does not.
func (r *reader) clauses(items []*Term, what string, line int, body func(i int) error) error {
	if len(items) < 2 {
		return fmt.Errorf("line %d: %s needs at least an `else` clause", line, what)
	}
	if len(items)%2 != 0 {
		return fmt.Errorf("line %d: %s clauses come in pairs — a condition and a "+
			"result — and the last condition must be `else`", line, what)
	}
	if !r.isElse(items[len(items)-2]) {
		return fmt.Errorf("line %d: the last clause of %s must be `else`, so that every "+
			"path out is written down; got %s", line, what, r.image(items[len(items)-2]))
	}
	for i := 0; i < len(items); i += 2 {
		if i < len(items)-2 && r.isElse(items[i]) {
			return fmt.Errorf("line %d: `else` must be the last clause; the ones after "+
				"it could never be reached", line)
		}
		if body != nil {
			if err := body(i + 1); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *reader) isElse(k *Term) bool {
	img := r.image(k)
	return img.Kind == KName && img.Name == "else"
}

// muCond reads `(cond c e … else e)`. A cond of one clause is its body.
func (r *reader) muCond(kids []*Term, line int) (Tail, error) {
	items := kids[1:]
	if err := r.clauses(items, "cond", line, nil); err != nil {
		return nil, err
	}
	if len(items) == 2 {
		return r.tree(items[1]), nil
	}
	n := &Cond{}
	for i := 0; i < len(items); i += 2 {
		arm := CondArm{Body: r.expr(items[i+1])}
		if i < len(items)-2 {
			arm.Guard = r.expr(items[i])
			r.mark(n, arm.Guard)
		}
		r.mark(n, arm.Body)
		n.Arms = append(n.Arms, arm)
	}
	return n, nil
}

// muLoop reads `(loop ((x z)…) c e … else e)` (iteration.md §2). A clause body
// is a Tail; its `again` takes one argument per loop variable.
func (r *reader) muLoop(kids []*Term, line int) (Tail, error) {
	if len(kids) < 4 {
		return nil, fmt.Errorf("line %d: loop takes a binding list and at least one "+
			"clause ending in `else`", line)
	}
	names, _, err := loopBindings(r.image(kids[1]), line)
	if err != nil {
		return nil, err
	}
	n := &Loop{Vars: names}
	for _, b := range r.elems[r.tree(kids[1])] {
		n.Inits = append(n.Inits, r.expr(r.elems[r.tree(b)][1]))
	}
	items := kids[2:]
	if err := r.clauses(items, "loop", line, func(i int) error {
		return r.tailOK(r.tree(items[i]), len(names), line)
	}); err != nil {
		return nil, err
	}
	for i := 0; i < len(items); i += 2 {
		if i == len(items)-2 {
			n.Arms = append(n.Arms, LoopArm{Else: true, Body: r.tree(items[i+1])})
			continue
		}
		n.Arms = append(n.Arms, LoopArm{Guard: r.expr(items[i]), Body: r.tree(items[i+1])})
	}
	return r.mark(n, n.Inits...), nil
}

// tailOK is the tail rule at a loop: a clause body is a back edge with one
// argument per loop variable, a binding whose body is one, or an expression
// with no `again` in it. Let binds; if branches. Binding may wrap an `again`,
// branching may not (iteration.md §2).
func (r *reader) tailOK(t Tail, arity, line int) error {
	switch t := t.(type) {
	case *Again:
		if len(t.Args) != arity {
			return fmt.Errorf("line %d: again takes %d argument(s), one per loop variable, "+
				"given %d", line, arity, len(t.Args))
		}
		return nil
	case *TLet:
		if err := r.tailOK(t.Body, arity, line); err != nil {
			return err
		}
		for i := len(t.Binds) - 1; i >= 0; i-- {
			if r.stray[t.Binds[i].Value] {
				return strayAgain(line)
			}
		}
		return nil
	case *TSeq:
		if err := r.tailOK(t.Last, arity, line); err != nil {
			return err
		}
		return r.noStray(line, t.Xs...)
	case *TMonad:
		if err := r.tailOK(t.Body, arity, line); err != nil {
			return err
		}
		return r.noStray(line, t.Items...)
	case *TRedex:
		if err := r.tailOK(t.Body, arity, line); err != nil {
			return err
		}
		return r.noStray(line, t.Arg)
	case *tlam:
		return strayAgain(line)
	case Expr:
		return r.noStray(line, t)
	}
	return nil
}

func (r *reader) noStray(line int, es ...Expr) error {
	for _, e := range es {
		if r.stray[e] {
			return strayAgain(line)
		}
	}
	return nil
}

func strayAgain(line int) error {
	return fmt.Errorf("line %d: `again` may be a clause body, or sit under a `let`, but "+
		"not under an `if` or inside an expression — write another clause instead, so "+
		"the clause list stays the loop's whole control flow", line)
}

// muMatch reads `(match (e…) p… (when c) body … else body)`, a loop whose
// scrutinees are its state and whose arms have patterns over it (match.md).
func (r *reader) muMatch(kids []*Term, line int) (Tail, error) {
	if len(kids) < 3 {
		return nil, fmt.Errorf("line %d: match takes a scrutinee list and at least one "+
			"clause ending in `else`", line)
	}
	if scrut := r.image(kids[1]); scrut.Kind != KApp || len(scrut.Kids) == 0 {
		return nil, fmt.Errorf("line %d: match takes a LIST of scrutinees — (match (a b) …) — "+
			"so that clauses need no tuple built and taken apart; got %s", line, scrut)
	}
	scrut := r.elems[r.tree(kids[1])]
	n := len(scrut)

	// A scrutinee that is a bare NAME becomes the loop variable under that same
	// name: `s` and `i` are the state, and a fresh name would leave a clause
	// reading the value the loop started from (match.md §4). Anything else gets
	// a fresh `#m` name, which no program can write.
	vars := make([]string, n)
	seen := map[string]bool{}
	for i, e := range scrut {
		if img := r.image(e); img.Kind == KName && !seen[img.Name] && img.Name != "_" && img.Name != "else" {
			vars[i] = img.Name
			seen[img.Name] = true
			continue
		}
		vars[i] = fmt.Sprintf("#m%d", i)
	}

	loop := &Loop{Vars: vars, Inits: r.exprs(scrut), Match: true}
	rest := kids[2:]
	for i := 0; i < len(rest); {
		if r.isElse(rest[i]) {
			if i+2 != len(rest) {
				return nil, fmt.Errorf("line %d: `else` must be the last clause of match; "+
					"the ones after it could never be reached", line)
			}
			loop.Arms = append(loop.Arms, LoopArm{Else: true, Body: r.tree(rest[i+1])})
			break
		}
		if i+n >= len(rest) {
			return nil, fmt.Errorf("line %d: match has %d scrutinees, so every clause is %d "+
				"pattern(s), an optional (when …), and a body; the last clause is short",
				line, n, n)
		}
		pats, err := r.matchPats(rest[i:i+n], line)
		if err != nil {
			return nil, err
		}
		arm := LoopArm{Pats: pats}
		k := i + n
		// An optional `(when c)` between the patterns and the body.
		if k < len(rest) {
			if w := r.image(rest[k]); w.Kind == KApp && len(w.Kids) == 2 &&
				w.Kids[0].Kind == KName && w.Kids[0].Name == "when" {
				arm.When = r.expr(rest[k].Kids[1])
				k++
			}
		}
		if k >= len(rest) {
			return nil, fmt.Errorf("line %d: a match clause needs a body after its patterns", line)
		}
		arm.Body = r.tree(rest[k])
		loop.Arms = append(loop.Arms, arm)
		i = k + 1
	}
	if last := loop.Arms[len(loop.Arms)-1]; !last.Else {
		guard, _ := r.d.matchArm(last, vars)
		return nil, fmt.Errorf("line %d: the last clause of %s must be `else`, so that every "+
			"path out is written down; got %s", line, "match", guard)
	}
	for _, arm := range loop.Arms {
		if err := r.tailOK(arm.Body, n, line); err != nil {
			return nil, err
		}
	}
	return r.mark(loop, loop.Inits...), nil
}

// matchPats reads one match clause's patterns: `_`, a name that binds,
// `true`/`false`, or an integer (match.md §2).
func (r *reader) matchPats(pats []*Term, line int) ([]Pat, error) {
	out := make([]Pat, len(pats))
	binds := map[string]bool{}
	for i, p := range pats {
		img := r.image(p)
		switch {
		case img.Kind == KName && img.Name == "_":
			out[i] = &PWild{}
		case img.Kind == KName && img.Name == "else":
			return nil, fmt.Errorf("line %d: `else` is a clause of its own and takes no "+
				"patterns; it cannot appear in position %d", line, i+1)
		case img.Kind == KName:
			if binds[img.Name] {
				return nil, fmt.Errorf("line %d: %s is bound twice in one match clause; "+
					"a repeated name would be an equality test, which patterns do not do",
					line, img.Name)
			}
			binds[img.Name] = true
			out[i] = &PVar{Name: img.Name}
		case img.Kind == KBool || img.Kind == KInt:
			out[i] = &PLit{T: img}
		default:
			return nil, fmt.Errorf("line %d: %s is not a pattern. A pattern is `_`, a name "+
				"that binds, `true`/`false`, or an integer; float and string patterns are absent "+
				"because the language has no portable equality", line, img)
		}
	}
	return out, nil
}

// muLocal reads `(local x₁ e₁ … xₖ eₖ body)`, sequential like `let`. The
// initializer's form decides what a binder is (spec/local.md §1): a table
// gives a buffer, anything else a cell.
func (r *reader) muLocal(kids []*Term, line int) (Tail, error) {
	rest := kids[1:]
	if len(rest) < 3 || len(rest)%2 == 0 {
		return nil, fmt.Errorf("line %d: local binds NAME VALUE pairs and ONE body, "+
			"`(local x e body)` (spec/local.md)", line)
	}
	params, err := r.scopeNames(LocalName, rest, line)
	if err != nil {
		return nil, err
	}
	n := &Local{}
	for i, p := range params {
		init := r.expr(rest[2*i+1])
		n.Binds = append(n.Binds, LocalBind{Name: p, Init: init, Buffer: isBufferInit(r.image(rest[2*i+1]))})
		r.mark(n, init)
	}
	n.Body = r.expr(rest[len(rest)-1])
	return r.mark(n, n.Body), nil
}

// isBufferInit is ADR 0047's rule on an initializer's form: a table, written
// `(table n f)` or `(array …)`, makes a buffer.
func isBufferInit(e *Term) bool {
	head := func(name string) bool {
		return e.Kind == KApp && len(e.Kids) > 0 && e.Kids[0].Kind == KName && e.Kids[0].Name == name
	}
	return head("table") && len(e.Kids) == 3 || head("array")
}

// muBuildMap reads `(build-map m cap … body)`, a scoped map buffer.
func (r *reader) muBuildMap(kids []*Term, line int) (Tail, error) {
	head := kids[0].Name
	rest := kids[1:]
	if len(rest)%2 == 0 {
		return nil, fmt.Errorf("line %d: %s binds NAME SIZE pairs and ONE body, "+
			"`(%s b n body)`; or it is the core form `(%s n (fn (b) body))` "+
			"(spec/tables.md §2.4)", line, head, head, head)
	}
	params, err := r.scopeNames(head, rest, line)
	if err != nil {
		return nil, err
	}
	n := &BuildMap{Names: params}
	for i := range params {
		c := r.expr(rest[2*i+1])
		n.Caps = append(n.Caps, c)
		r.mark(n, c)
	}
	n.Body = r.expr(rest[len(rest)-1])
	return r.mark(n, n.Body), nil
}

// scopeNames reads a scope's binder names, which are one parameter list: the
// scope's state is alive together, so a repeated name would hide the first.
func (r *reader) scopeNames(head string, rest []*Term, line int) ([]string, error) {
	var names []*Term
	for i := 0; i+1 < len(rest); i += 2 {
		names = append(names, r.image(rest[i]))
	}
	return paramListOf(head, &Term{Kind: KApp, Kids: names}, line)
}
