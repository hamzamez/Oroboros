package j

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"oroboros/core"
)

// canonTarget is a target source's forms up to the laws a target file obeys: a
// file is the glue of its forms, so the target wrapper goes, an inclusion of a
// type of a module of the file stands in that module (ADR 0064 item 9), and a
// module's forms are a multiset; a sig binds its own names (keyed by position,
// its parameters by their sets alone); a template's holes are keyed by the
// parameter they splice, whatever their style; an import's alias is present.
func canonTarget(forms []*core.Term) string {
	var flat []*core.Term
	for _, f := range forms {
		if head(f) == "target" {
			flat = append(flat, f.Kids[2:]...)
			continue
		}
		flat = append(flat, f)
	}
	return canonLevel(flat)
}

func canonLevel(forms []*core.Term) string {
	modules := map[string][]*core.Term{}
	var order []string
	var rest []*core.Term
	for _, f := range forms {
		if head(f) == "module" {
			p := f.Kids[1].Name
			if _, ok := modules[p]; !ok {
				order = append(order, p)
			}
			modules[p] = append(modules[p], f.Kids[2:]...)
		}
	}
	for _, f := range forms {
		switch head(f) {
		case "module":
		case "implements":
			name := f.Kids[1].Name
			moved := false
			for p := range modules {
				if strings.HasPrefix(name, p+".") && !strings.Contains(name[len(p)+1:], ".") {
					rel := app(append([]*core.Term{f.Kids[0], core.Name(name[len(p)+1:])}, f.Kids[2:]...)...)
					modules[p] = append(modules[p], rel)
					moved = true
					break
				}
			}
			if !moved {
				rest = append(rest, f)
			}
		default:
			rest = append(rest, f)
		}
	}
	var keys []string
	for _, f := range rest {
		keys = append(keys, canonForm(f))
	}
	sort.Strings(order)
	for _, p := range order {
		keys = append(keys, "(module "+p+"\n"+canonLevel(modules[p])+")")
	}
	sort.Strings(keys)
	return strings.Join(keys, "\n")
}

func canonForm(f *core.Term) string {
	switch head(f) {
	case "provides":
		return fmt.Sprintf("(provides %s %s\n%s)", f.Kids[1].Name, f.Kids[2].Name, canonLevel(f.Kids[3:]))
	case "use":
		path := f.Kids[1].Name
		alias := path[strings.LastIndex(path, "/")+1:]
		if len(f.Kids) == 4 {
			alias = f.Kids[3].Name
		}
		return "(use " + path + " as " + alias + ")"
	case "sig":
		return canonSig(f)
	case "def":
		fm, err := core.ToForm(f)
		if err != nil {
			return "(def ? " + err.Error() + ")"
		}
		return "(def " + fm.Name + " " + key(fm.Term) + ")"
	}
	return key(f)
}

// withMeantDefs is raw with each `def` replaced by meant's, the same datum read
// with its sugar desugared: the two readings have one shape above a term.
func withMeantDefs(raw, meant *core.Term, has *bool) *core.Term {
	switch head(raw) {
	case "def":
		*has = true
		return meant
	case "provides", "module", "target":
		c := *raw
		c.Kids = make([]*core.Term, len(raw.Kids))
		for i, k := range raw.Kids {
			c.Kids[i] = withMeantDefs(k, meant.Kids[i], has)
		}
		return &c
	}
	return raw
}

// canonSig keys a sig: its parameters by their sets, its names by position, and
// its template's holes by the parameter each splices.
func canonSig(f *core.Term) string {
	g := positional(f)
	params := g.Kids[2]
	var sets []*core.Term
	for _, prm := range params.Kids {
		if prm.Kind == core.KApp && len(prm.Kids) == 2 && prm.Kids[0].Kind == core.KName && strings.HasPrefix(prm.Kids[0].Name, "$") {
			sets = append(sets, prm.Kids[1])
		} else {
			sets = append(sets, prm)
		}
	}
	kids := append([]*core.Term(nil), g.Kids...)
	kids[2] = app(sets...)
	// Several results are one product, written `(int int)` or `(tuple int int)`.
	if r := kids[3]; r.Kind == core.KApp && len(r.Kids) > 1 && !isTypeHead(r) && head(r) != "int" ||
		r.Kind == core.KApp && len(r.Kids) > 1 && head(r) == "int" && !(len(r.Kids) == 3 && isBound(r.Kids[1])) {
		kids[3] = app(append([]*core.Term{core.Name("tuple")}, r.Kids...)...)
	}
	for i, k := range kids {
		if head(k) == "host" {
			h := append([]*core.Term(nil), k.Kids...)
			for j, x := range h {
				if x.Kind == core.KStr {
					h[j] = core.Str(canonHoles(x.Str, len(sets)))
				}
				if head(x) == "jump" && len(x.Kids) == 3 && x.Kids[2].Kind == core.KStr {
					h[j] = app(x.Kids[0], x.Kids[1], core.Str(canonHoles(x.Kids[2].Str, len(sets))))
				}
			}
			kids[i] = app(h...)
		}
	}
	return key(app(kids...))
}

// canonHoles writes a template's holes as the parameter each fills: `%s` the
// next, cyclically (emit/target.go, fill), `%n` the n-th, as <n>.
func canonHoles(tpl string, n int) string {
	var b strings.Builder
	seq := 0
	for i := 0; i < len(tpl); i++ {
		if tpl[i] != '%' || i+1 >= len(tpl) {
			b.WriteByte(tpl[i])
			continue
		}
		c := tpl[i+1]
		switch {
		case c == 's' && n > 0:
			fmt.Fprintf(&b, "<%d>", seq%n+1)
			seq++
			i++
		case c >= '1' && c <= '9':
			fmt.Fprintf(&b, "<%c>", c)
			i++
		default:
			b.WriteByte('%')
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// EVERY TARGET FILE READS BACK TO ITS FORMS (spec/j.md §2.1–2.5, §2.11–2.12):
// J's reading of each target file's translation gives the raw forms today's
// reading does, up to the laws canonTarget applies. A program's definition in a
// fragment (tally's two files) is compared as its meaning, D of its datum.
func TestEveryTargetReadsBackToItsForms(t *testing.T) {
	var files []string
	filepath.Walk("../targets", func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(p, ".oro") {
			files = append(files, p)
		}
		return nil
	})
	for _, path := range corpus(t) {
		b, _ := os.ReadFile(path)
		if _, err := Translate(string(b), filepath.Dir(path), ".."); err != nil && strings.Contains(err.Error(), "step 2d") {
			files = append(files, path)
		}
	}
	agree, withDefs := 0, 0
	for _, path := range files {
		b, _ := os.ReadFile(path)
		src := string(b)
		want, err := core.ReadRaw(src)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		// A program's definition in a fragment is read by the loader as D of its
		// datum (emit/target.go reads every target file with core.ReadAll).
		meant, err := core.ReadAll(src)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		hasDefs := false
		for i := range want {
			want[i] = withMeantDefs(want[i], meant[i], &hasDefs)
		}
		r, err := TranslateTarget(src, "..")
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		got, err := ReadTarget(r.J, "", "..")
		if err != nil {
			t.Errorf("%s: J does not read: %v", path, err)
			continue
		}
		if hasDefs {
			withDefs++
		}
		w, g := canonTarget(want), canonTarget(got)
		if w != g {
			wl, gl := strings.Split(w, "\n"), strings.Split(g, "\n")
			for i := 0; i < len(wl) || i < len(gl); i++ {
				var a, b string
				if i < len(wl) {
					a = wl[i]
				}
				if i < len(gl) {
					b = gl[i]
				}
				if a != b {
					t.Errorf("%s: the forms differ:\n  today: %.300s\n  J:     %.300s", path, a, b)
					break
				}
			}
			continue
		}
		agree++
	}
	if agree != len(files) || agree < 40 || withDefs < 2 {
		t.Fatalf("%d of %d target files read back (%d hold a program's definitions)", agree, len(files), withDefs)
	}
	t.Logf("%d of %d target files read back to their forms; %d hold a program's definitions", agree, len(files), withDefs)
}
