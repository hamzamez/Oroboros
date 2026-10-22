package emit

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"oroboros/core"
)

// targetDecls is every declaration in every target file: each (sig …) and
// (const …), at the top of a file, inside (target …), (module …) or a
// (provides …) fragment, however deep modules nest.
func targetDecls(t *testing.T) map[string][]*core.Term {
	t.Helper()
	out := map[string][]*core.Term{}
	var visit func(path string, f *core.Term)
	visit = func(path string, f *core.Term) {
		switch formWord(f) {
		case "sig", "const":
			out[path] = append(out[path], f)
		case "target", "module", "provides":
			for _, k := range f.Kids[1:] {
				if k.Kind == core.KApp {
					visit(path, k)
				}
			}
		}
	}
	err := filepath.Walk("../targets", func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".oro") {
			return err
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		// The target is its directory, or its file's name (ADR 0064 item 9).
		rel := strings.TrimPrefix(filepath.ToSlash(p), "../targets/")
		name := strings.TrimSuffix(strings.SplitN(rel, "/", 2)[0], ".oro")
		ts, err := core.ReadTargetSource(string(src), p, name)
		if err != nil {
			return err
		}
		for _, f := range ts {
			visit(p, f)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// primText renders a primitive with its terms printed, so two primitives
// compare by what they say rather than by which term pointers they hold.
func primText(p Prim) (Prim, []string) {
	var ts []string
	for _, x := range []*core.Term{p.Where, p.Ensures, p.Retract} {
		if x != nil {
			ts = append(ts, x.String())
		} else {
			ts = append(ts, "-")
		}
	}
	p.Where, p.Ensures, p.Retract = nil, nil, nil
	return p, ts
}

// A HOST DECLARATION IS A DEFINITION WITH A HOST BODY (ADR 0051, D4), and
// Theorem D3 holds for it: on every declaration in every target file, the
// primitive read off the `sig` D writes back from the tree is the primitive
// today's reader made of the declaration, field for field. Usage reads as a
// mode, never as a set (ADR 0050, decision 2).
func TestHostDeclarationsAreDefinitions(t *testing.T) {
	n, deep, modes := 0, 0, map[core.Mode]int{}
	for path, fs := range targetDecls(t) {
		for _, f := range fs {
			want, err := parseSig(f, path)
			if err != nil {
				continue // refused as a declaration: deferred or not one, read elsewhere
			}
			def, err := hostDef(f, want, path)
			if err != nil {
				t.Errorf("%s: %v", path, err)
				continue
			}
			got, err := parseSig(core.HostSig(def), path)
			if err != nil {
				t.Errorf("%s: D's sig is refused: %v\n%s", path, err, core.HostSig(def))
				continue
			}
			g, gt := primText(got)
			w, wt := primText(want)
			if !reflect.DeepEqual(g, w) || !reflect.DeepEqual(gt, wt) {
				t.Errorf("%s: %s\n the tree gives %+v %v\n today's reader %+v %v", path, want.Name, g, gt, w, wt)
			}
			for _, p := range core.HostLam(def).Claim.Params {
				modes[p.Mode]++
				if a, ok := p.Set.(*core.Apply); ok && p.Mode != core.NoMode {
					if v, ok := a.Fn.(*core.Var); ok && (v.Name == "buffer" || v.Name == "cell") {
						t.Errorf("%s: %s: a parameter with a mode still spells its usage in its set", path, want.Name)
					}
				}
			}
			for _, m := range core.HostLam(def).Claim.ResultModes {
				modes[m]++
			}
			if r := core.HostSig(def).Kids[3].String(); strings.Contains(r, "buffer") && core.HostLam(def).Claim.ResultModes == nil {
				deep++ // a buffer inside a result's set deeper than a component keeps its former
			}
			n++
		}
	}
	t.Logf("%d declarations read; modes %v; %d results keep a buffer former deeper than a component", n, modes, deep)
	if n < 800 {
		t.Fatalf("only %d declarations read: the walk found too little to test", n)
	}
	if modes[core.Linear] < 60 || modes[core.InOut] < 9 || deep > 0 {
		t.Errorf("modes read: %v, %d left in a set; the targets have 60 buffers, 30 parameters and 30 results, and 9 cell lists", modes, deep)
	}
}

// THEOREM D3 ON THE SHAPES THE TARGETS LACK: a single cell, a buffer result
// alone, a positional buffer, a nullary declaration, a constant, a unit result,
// and a variadic list of cells beside a fixed cell.
func TestHostDeclarationShapes(t *testing.T) {
	for _, src := range []string{
		`fn Get(c ∈ Int inout) (()) { host stmt "get(${c})" }`,
		`fn Fill(b ∈ Array(0..255) linear) (b' ∈ buffer(0..255)) { host expr "fill(${b})" }`,
		`fn Put(a ∈ Array(0..255) linear, b ∈ Int) (n ∈ Int) pure ensure len(a') == len(a) { host expr "put(${a}, ${b})" }`,
		`fn Now() (Int) { host expr "now()" }`,
		`fn Two(x ∈ Int, y ∈ Int) (r ∈ Int) pure
    where x < y
    ensure r > 0 {
    host expr "${x}+${y}" import "m"
}`,
		`fn Scan(s ∈ bytestring, n ∈ Int inout, a ∈ Array(cell(go.Value))) (()) variadic {
    host expr "scan(${s}, ${n}, ${a}...)"
}`,
		`Max ∈ set { 9 } = host "m.Max" import "m"`,
	} {
		ts, err := core.ReadAll(src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		f := ts[0]
		want, err := parseSig(f, "shape")
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		def, err := hostDef(f, want, "shape")
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		got, err := parseSig(core.HostSig(def), "shape")
		if err != nil {
			t.Fatalf("%s: D's sig %s is refused: %v", src, core.HostSig(def), err)
		}
		g, gt := primText(got)
		w, wt := primText(want)
		if !reflect.DeepEqual(g, w) || !reflect.DeepEqual(gt, wt) {
			t.Errorf("%s:\n the tree gives %+v %v\n today's reader %+v %v", src, g, gt, w, wt)
		}
	}
	ts, _ := core.ReadAll(`fn Get(c ∈ Int inout) (()) { host stmt "get(${c})" }`)
	p, _ := parseSig(ts[0], "shape")
	def, _ := hostDef(ts[0], p, "shape")
	if c := core.HostLam(def).Claim.Params[0]; c.Mode != core.InOut {
		t.Errorf("a cell is the inout mode on its set, got %+v", c)
	}
}

// ADR 0070: the header is the caller's view of a borrow. A linear parameter
// with no p' among the results is written in place, and the core's threaded
// result is derived, the buffer first; a result p' is storage handed back. The
// old threaded spelling, and a p' naming no linear parameter, are refused.
func TestHostHeaderIsTheCallersView(t *testing.T) {
	read := func(src string) Prim {
		t.Helper()
		ts, err := core.ReadAll(src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		p, err := parseSig(ts[0], "view")
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		return p
	}
	put := read(`fn Put(a ∈ Array(0..255) linear, b ∈ Int) (n ∈ Int) ensure len(a') == len(a) { host expr "put(${a}, ${b})" }`)
	if !reflect.DeepEqual(put.InPlace, []InPlace{{Result: 0, Arg: 0}}) || len(put.Results) != 2 {
		t.Errorf("a write-borrow threads the buffer first and records it in place, got %v %v", put.Results, put.InPlace)
	}
	if hr := HostResults(put); len(hr) != 1 {
		t.Errorf("the host's own results leave the in-place buffer out, got %v", hr)
	}
	fill := read(`fn Fill(b ∈ Array(0..255) linear, x ∈ Int) (b' ∈ buffer(0..255)) { host expr "fill(${b}, ${x})" }`)
	if len(fill.InPlace) != 0 || len(HostResults(fill)) != 1 {
		t.Errorf("a primed result is storage the host hands back, got %v %v", fill.Results, fill.InPlace)
	}
	for _, c := range []struct{ src, why string }{
		{`fn Old(a ∈ Array(0..255) linear) (buffer(0..255) × Int) { host expr "old(${a})" }`, "caller's view"},
		{`fn Odd(a ∈ Array(0..255) linear, q ∈ Int) (q' ∈ buffer(0..255)) { host expr "odd(${a}, ${q})" }`, "linear"},
	} {
		ts, err := core.ReadAll(c.src)
		if err == nil {
			_, err = parseSig(ts[0], "view")
		}
		if err == nil || !strings.Contains(err.Error(), c.why) {
			t.Errorf("%s: want a refusal naming %q, got %v", c.src, c.why, err)
		}
	}
}
