package emit_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// RELEVANT USAGE AGREES ON THE CORPUS (abstract.md §8.1, step 2). For every
// definition of every program that loads on the Go target, and each of its
// parameters, the tree's usage (core.UsageEnv) and the reducer's usage inference
// (Env.ParamUsed, what the relevance check reads) answer the same question —
// is the parameter used on every path that ends normally — and must agree.
func TestRelevantUsageAgreesOnTheCorpus(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	params, programs, unusedSomewhere := 0, 0, 0
	var disagree []string
	for _, dir := range []string{"../examples", "../lib", "../gauntlet/stdlib/acceptance"} {
		filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".oro") {
				return nil
			}
			src, _ := os.ReadFile(p)
			forms, err := core.Read(string(src))
			if err != nil || core.OnlyFragments(forms) {
				return nil
			}
			prog, _, err := tg.LoadProgram(forms, filepath.Dir(p), "../lib")
			if err != nil {
				return nil // not a Go program, or one meant to be refused
			}
			env, err := tg.Env(prog)
			if err != nil {
				return nil
			}
			decls, err := core.ReadTrees(string(src))
			if err != nil {
				t.Errorf("%s: %v", p, err)
				return nil
			}
			// A file may hold several modules: each definition is its module's,
			// and is handed values by the names its module calls them.
			type mod struct {
				path  string
				defs  map[string]*core.Lam
				names []string
			}
			var mods []*mod
			cur := &mod{defs: map[string]*core.Lam{}}
			mods = append(mods, cur)
			for _, d := range decls {
				switch d := d.(type) {
				case *core.ModuleDecl:
					cur = &mod{path: d.Path, defs: map[string]*core.Lam{}}
					mods = append(mods, cur)
				case *core.DefDecl:
					if l, ok := d.Value.(*core.Lam); ok {
						cur.defs[d.Name] = l
						cur.names = append(cur.names, d.Name)
					}
				}
			}
			for _, m := range mods {
				u := &core.UsageEnv{Defs: m.defs}
				for _, n := range m.names {
					q := n
					if m.path != "" {
						q = m.path + "." + n
					}
					for j := range m.defs[n].Params {
						tree := u.ParamUsage(n, j).Relevant()
						today := env.ParamUsed(q, j)
						params++
						if !today {
							unusedSomewhere++
						}
						if tree != today {
							disagree = append(disagree, fmt.Sprintf("%s: %s parameter %d (%s): tree %v, reducer %v",
								p, q, j, m.defs[n].Params[j], tree, today))
						}
					}
				}
			}
			programs++
			return nil
		})
	}
	t.Logf("%d parameters of %d programs compared; %d not used on every path", params, programs, unusedSomewhere)
	for _, d := range disagree {
		t.Error(d)
	}
	if programs < 40 {
		t.Fatalf("only %d programs loaded: the walk found too little to test", programs)
	}
}
