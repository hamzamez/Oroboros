package emit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"oroboros/core"
)

// THE INVENTORY IS A SET, AND THIS TEST IS ITS EQUALITY (docs/spec/inventory.md).
//
// Let W_code be the words the compiler accepts, refuses by name, or injects, and
// W_doc the words the inventory's tables list. The document's claim is
// W_doc = W_code, with every word marked `specified` mentioned by a spec it links.
// Both directions are checked, as TestHandDeclarationsAgreeWithTheHost checks a
// hand declaration against its host: a word the code accepts and the document
// omits is an unexplained word, and a word the document lists and the code no
// longer accepts is a stale claim. The audit had been retaken by hand twice and
// was stale within a week each time.
//
// W_code is read MECHANICALLY, two ways.
//
//  1. Syntactically, with go/ast, from the reader and the loaders: every string
//     literal a form's head is compared against — a `case` of a switch on a
//     `.Name`, a `.Kind`, a `formWord(…)` or an `op`, the right side of `==`/`!=`
//     against one of those, and the choices handed to a loader's `word(…)`.
//  2. From the compiler's word tables, referenced here directly, so an entry added
//     to one is caught at the next run: the reserved and respelled forms, the sig
//     and host clause words, the type formers, the injected language names and
//     kinds, the integer operators, the big-integer names a target declares, the
//     backends, and `lang`'s variant.
//
// The limit, stated: a word recognised by a NEW mechanism — neither a comparison
// of that shape nor an entry in these tables — is invisible to both.

// inventorySources are the files a word can be accepted in: the reader, the
// module loader and reducer, and the target loaders.
var inventorySources = []string{
	"../core/read.go", "../core/reduce.go", "../core/sum.go", "../core/hygiene.go", "../core/term.go",
	"target.go", "companion.go", "alias.go", "constend.go", "fact.go",
}

// headExpr reports whether an expression holds a form's head: a `.Name`, a
// `.Kind`, a `formWord(…)`, or a variable assigned one of those in the same
// function (`op, args := t.Kids[0].Name, …`). A variable merely NAMED `op` is not
// one — the analyses switch on their own canonical operator names (`add`, `le`),
// which no source file can spell.
func headExpr(e ast.Expr, vars map[string]bool) bool {
	switch x := e.(type) {
	case *ast.SelectorExpr:
		return x.Sel.Name == "Name" || x.Sel.Name == "Kind"
	case *ast.CallExpr:
		id, ok := x.Fun.(*ast.Ident)
		return ok && id.Name == "formWord"
	case *ast.Ident:
		return vars[x.Name]
	}
	return false
}

// notWords are literals the extractor finds that name something internal rather
// than a word a source file can contain. Each must still be extracted, or the
// entry is stale and the test says so.
var notWords = map[string]string{
	"term":        "core.Form's kind for a top-level term; no source spells it",
	"table-build": "the kind the compiler gives the injected `build`; parseStructural refuses it in a target file",
	"table-set":   "the kind the compiler gives the injected `set`; parseStructural refuses it in a target file",
}

// dispatchWords collects the literal words a Go source compares a form against.
func dispatchWords(src string, name string) (map[string]bool, error) {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, name, src, 0)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	lit := func(e ast.Expr) {
		l, ok := e.(*ast.BasicLit)
		if !ok || l.Kind != token.STRING {
			return
		}
		s, err := strconv.Unquote(l.Value)
		if err != nil || !isWord(s) {
			return
		}
		out[s] = true
	}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		// The variables holding a head in this function.
		vars := map[string]bool{}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if as, ok := n.(*ast.AssignStmt); ok && len(as.Lhs) == len(as.Rhs) {
				for i, r := range as.Rhs {
					if id, ok := as.Lhs[i].(*ast.Ident); ok && headExpr(r, nil) {
						vars[id.Name] = true
					}
				}
			}
			return true
		})
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SwitchStmt:
				if x.Tag != nil && headExpr(x.Tag, vars) {
					for _, st := range x.Body.List {
						for _, v := range st.(*ast.CaseClause).List {
							lit(v)
						}
					}
				}
			case *ast.BinaryExpr:
				if (x.Op == token.EQL || x.Op == token.NEQ) && headExpr(x.X, vars) {
					lit(x.Y)
				}
			case *ast.CallExpr:
				if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "word" {
					for _, a := range x.Args {
						lit(a)
					}
				}
				// A form refused by its head inside a printed term: strings.Contains(s, "(forall ").
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Contains" && len(x.Args) == 2 {
					if l, ok := x.Args[1].(*ast.BasicLit); ok && l.Kind == token.STRING {
						if v, err := strconv.Unquote(l.Value); err == nil && strings.HasPrefix(v, "(") && strings.HasSuffix(v, " ") {
							if w := strings.TrimSpace(v[1:]); isWord(w) {
								out[w] = true
							}
						}
					}
				}
			}
			return true
		})
	}
	return out, nil
}

// isWord admits what a source file can spell as a word: no spaces, no template
// holes, not a generated name (`#k`, `#tag`).
func isWord(s string) bool {
	return s != "" && !strings.HasPrefix(s, "#") && !strings.ContainsAny(s, " %(){}.\"\t\n")
}

func codeWords(t *testing.T) map[string]bool {
	t.Helper()
	w := map[string]bool{}
	for _, f := range inventorySources {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		got, err := dispatchWords(string(src), f)
		if err != nil {
			t.Fatal(err)
		}
		for k := range got {
			w[k] = true
		}
	}
	for k := range coreNames {
		w[k] = true
	}
	// An injected name is a word; its KIND (`table-set`, `map-keys`, `ascribe`) is
	// not, since parseStructural refuses every kind but structuralKinds.
	for _, p := range coreStructural {
		w[p.Name] = true
	}
	for k := range structuralKinds {
		w[k] = true
	}
	for k := range respelled {
		w[k] = true
	}
	for k := range sigWords {
		w[k] = true
	}
	for k := range hostWords {
		w[k] = true
	}
	for k := range langTypes {
		w[k] = true
	}
	for _, op := range langOps {
		w[op.name] = true
	}
	for _, op := range bigOps {
		w[op.name] = true
	}
	for _, b := range Backends {
		w[b] = true
	}
	for _, k := range []string{"array", "tuple", "buffer", "map", "int", "fn", "record", "prod"} {
		if core.IsTypeFormer(k) {
			w[k] = true
		}
	}
	opt := core.OptionSum()
	w[opt.Name] = true
	for _, v := range opt.Variants {
		w[v.Name] = true
	}
	w[core.ResultName] = true
	w[core.AscribeName] = true
	// The boolean literals are tokens the reader recognises before any form is
	// built, so no comparison of the shapes above sees them; they are admitted
	// here only after the reader is asked and says they are literals.
	for _, lit := range []string{"true", "false"} {
		if tm, err := core.ReadTerm(lit); err != nil || tm.Kind != core.KBool {
			t.Errorf("the reader no longer reads %s as a boolean literal", lit)
			continue
		}
		w[lit] = true
	}
	for k, why := range notWords {
		if !w[k] {
			t.Errorf("notWords[%q] (%s) is no longer extracted; delete the entry", k, why)
		}
		delete(w, k)
	}
	return w
}

// docRow is one row of an inventory table: the words in its first cell, its
// status, and the documents its last cell links.
type docRow struct {
	words  []string
	status string
	links  []string
	line   int
}

var (
	backticked = regexp.MustCompile("`([^`]+)`")
	mdLink     = regexp.MustCompile(`\]\(([^)#]+)(#[^)]*)?\)`)
)

// inventoryStatuses are the only statuses a row may carry.
var inventoryStatuses = map[string]bool{
	"specified": true, "recorded": true, "reserved": true, "refused": true, "undocumented": true,
}

func docRows(t *testing.T, text string) []docRow {
	t.Helper()
	var rows []docRow
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || strings.HasPrefix(line, "|---") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) < 3 || strings.TrimSpace(cells[0]) == "word" {
			continue
		}
		var r docRow
		r.line = i + 1
		for _, m := range backticked.FindAllStringSubmatch(cells[0], -1) {
			r.words = append(r.words, m[1])
		}
		if len(r.words) == 0 {
			t.Errorf("inventory.md:%d: a row with no word in its first cell: %s", i+1, line)
			continue
		}
		r.status = strings.Fields(strings.TrimSpace(cells[1]) + " ?")[0]
		for _, m := range mdLink.FindAllStringSubmatch(cells[len(cells)-1], -1) {
			r.links = append(r.links, m[1])
		}
		rows = append(rows, r)
	}
	return rows
}

// docMentions reports whether a document states a word: as a token of a code
// span, where a token is what lies between spaces and brackets — so `(repr big host)`
// states `repr`, `big` and `host`, and `+ - * / %` states `%`.
//
// A fenced block is code too, so its lines are tokenised the same way; a `;`
// begins a comment there, as it does in a source file.
func docMentions(doc, word string) bool {
	has := func(code string) bool {
		for _, tok := range strings.FieldsFunc(code, func(r rune) bool {
			return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '(' || r == ')' ||
				r == '[' || r == ']' || r == ',' || r == '…' || r == ';'
		}) {
			if tok == word {
				return true
			}
		}
		return false
	}
	for i, seg := range strings.Split(doc, "```") {
		if i%2 == 1 {
			if has(seg) {
				return true
			}
			continue
		}
		for _, m := range backticked.FindAllStringSubmatch(seg, -1) {
			if has(m[1]) {
				return true
			}
		}
	}
	return false
}

func checkInventory(t *testing.T, code map[string]bool, text, dir string) []string {
	var problems []string
	listed := map[string]int{}
	for _, r := range docRows(t, text) {
		if !inventoryStatuses[r.status] {
			problems = append(problems, "inventory.md:"+strconv.Itoa(r.line)+": status "+
				strconv.Quote(r.status)+" is not one of specified, recorded, reserved, refused, undocumented")
		}
		for _, w := range r.words {
			if prev, dup := listed[w]; dup {
				problems = append(problems, "inventory.md:"+strconv.Itoa(r.line)+": `"+w+
					"` is listed twice (also line "+strconv.Itoa(prev)+")")
			}
			listed[w] = r.line
			if r.status != "specified" {
				continue
			}
			said := false
			for _, l := range r.links {
				if !strings.HasSuffix(l, ".md") {
					continue
				}
				b, err := os.ReadFile(filepath.Join(dir, l))
				if err != nil {
					problems = append(problems, "inventory.md:"+strconv.Itoa(r.line)+": link "+l+" does not resolve")
					continue
				}
				if docMentions(string(b), w) {
					said = true
				}
			}
			if !said {
				problems = append(problems, "inventory.md:"+strconv.Itoa(r.line)+": `"+w+
					"` is marked specified, and no document the row links mentions it")
			}
		}
	}
	var missing, stale []string
	for w := range code {
		if _, ok := listed[w]; !ok {
			missing = append(missing, w)
		}
	}
	for w := range listed {
		if !code[w] {
			stale = append(stale, w)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	for _, w := range missing {
		problems = append(problems, "the compiler accepts `"+w+"` and inventory.md does not list it")
	}
	for _, w := range stale {
		problems = append(problems, "inventory.md lists `"+w+"` (line "+strconv.Itoa(listed[w])+
			") and the compiler no longer knows it")
	}
	return problems
}

func TestTheInventoryIsTheWordsTheCompilerKnows(t *testing.T) {
	code := codeWords(t)
	b, err := os.ReadFile("../docs/spec/inventory.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range checkInventory(t, code, string(b), "../docs/spec") {
		t.Error(p)
	}
	if t.Failed() && os.Getenv("INVENTORY_WORDS") != "" {
		var ws []string
		for w := range code {
			ws = append(ws, w)
		}
		sort.Strings(ws)
		t.Log(strings.Join(ws, " "))
	}
}

// THE CHECK MUST BE ABLE TO FAIL, in each direction and on each rule.
func TestTheInventoryCheckFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte("the `alpha` form, and `(beta x)`\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code := map[string]bool{"alpha": true, "beta": true}
	good := "| word | status | where |\n|---|---|---|\n" +
		"| `alpha` | specified | [spec](spec.md) |\n| `beta` | specified | [spec](spec.md) |\n"
	if p := checkInventory(t, code, good, dir); len(p) != 0 {
		t.Fatalf("the control must pass: %v", p)
	}
	cases := []struct {
		name, doc string
		code      map[string]bool
		want      string
	}{
		{"a word the code accepts is missing", good,
			map[string]bool{"alpha": true, "beta": true, "gamma": true}, "does not list it"},
		{"a listed word the code no longer knows", good,
			map[string]bool{"alpha": true}, "no longer knows it"},
		{"specified, and the spec does not say it",
			strings.Replace(good, "`beta`", "`beta`, `delta`", 1),
			map[string]bool{"alpha": true, "beta": true, "delta": true}, "no document the row links mentions it"},
		{"an unknown status", strings.Replace(good, "| `alpha` | specified", "| `alpha` | fine", 1),
			code, "is not one of"},
		{"a word listed twice", good + "| `alpha` | recorded | code |\n", code, "listed twice"},
	}
	for _, c := range cases {
		p := checkInventory(t, c.code, c.doc, dir)
		found := false
		for _, s := range p {
			if strings.Contains(s, c.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want a problem containing %q, got %v", c.name, c.want, p)
		}
	}

	// And the extractor sees a new dispatch, of each recognised shape.
	src := `package p
func f(t T, op2 string) {
	switch t.Name { case "one": }
	if formWord(t) == "two" {}
	op := t.Kids[0].Name
	switch op { case "three": }
	switch op2 { case "eight": }
	if t.Kind != "four" {}
	word("five", "six")
	if strings.Contains(s, "(nine ") {}
	if t.Other == "seven" {}
}`
	got, err := dispatchWords(src, "p.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"one", "two", "three", "four", "five", "six", "nine"} {
		if !got[w] {
			t.Errorf("the extractor missed %q", w)
		}
	}
	if got["seven"] || got["eight"] {
		t.Errorf("the extractor took a comparison against a field that is not a word")
	}
}
