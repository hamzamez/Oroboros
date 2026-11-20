package stdlib

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"oroboros/emit"
)

// AN `ordered` CALLBACK IS ITS HOST'S SENTENCE (ADR 0143). `ordered` is a claim
// about a host's promise that no compiler can verify, and its first reading
// went wrong 52 times in 64, a promise about a result taken for one about the
// calls. So the evidence is kept beside the survey: callbacks-go-markers.txt
// gives each function whose documentation states the order of its calls the
// sentence that states it. Checked both ways:
//   - every host function in targets/go whose callback claim is `ordered` has a
//     row marked S, and its sentence is in `go doc` of the function;
//   - every function marked S that targets/go declares is declared `ordered`.
//
// The fault is a sentence no document says, which must be refused.
func TestEveryOrderedCallbackIsItsHostsSentence(t *testing.T) {
	rows, err := markerRows(filepath.Join(root, "gauntlet", "stdlib", "callbacks-go-markers.txt"))
	if err != nil {
		t.Fatal(err)
	}
	tg, err := emit.LoadTarget(filepath.Join(root, "targets", "go"))
	if err != nil {
		t.Fatal(err)
	}
	if errs := checkOrdered(tg, rows, goDoc); len(errs) > 0 {
		for _, e := range errs {
			t.Error(e)
		}
	}
	// the fault: the evidence for slices.EqualFunc replaced by a sentence Go's
	// documentation does not contain
	bad := map[string]markerRow{}
	for k, v := range rows {
		bad[k] = v
	}
	r := bad["slices.EqualFunc"]
	r.sentence = "the elements are compared in random order"
	bad["slices.EqualFunc"] = r
	if errs := checkOrdered(tg, bad, goDoc); len(errs) == 0 {
		t.Error("a sentence go doc does not contain was accepted as the evidence: the check cannot fail")
	}
}

type markerRow struct {
	order, sentence string
}

// markerRows reads the markers file: name, escape, count, order, sentence.
func markerRows(path string) (map[string]markerRow, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]markerRow{}
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 4 {
			return nil, fmt.Errorf("a markers row has name, escape, count and order: %q", line)
		}
		r := markerRow{order: f[3]}
		if len(f) > 4 {
			r.sentence = f[4]
		}
		out[f[0]] = r
	}
	return out, nil
}

// goDoc is `go doc NAME`'s text with its whitespace collapsed.
func goDoc(name string) (string, error) {
	out, err := exec.Command("go", "doc", name).Output()
	if err != nil {
		return "", fmt.Errorf("go doc %s: %v", name, err)
	}
	return strings.Join(strings.Fields(string(out)), " "), nil
}

// checkOrdered is the check, both ways, with doc as the host's documentation.
func checkOrdered(tg *emit.Target, rows map[string]markerRow, doc func(string) (string, error)) []string {
	var errs []string
	declared := map[string]bool{}
	var names []string
	for n := range tg.Prims {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		p := tg.Prims[n]
		for _, a := range p.Args {
			q, ok := strings.CutPrefix(a, emit.CallbackPrefix)
			if !ok {
				continue
			}
			host := strings.TrimPrefix(n, "go/")
			declared[host] = true
			c := tg.Callbacks[q]
			if !c.Ordered {
				continue
			}
			r, ok := rows[host]
			if !ok || r.order != "S" || r.sentence == "" {
				errs = append(errs, fmt.Sprintf("%s's callback %s is declared `ordered`, and the markers file "+
					"gives no sentence of its documentation stating the order", host, q))
				continue
			}
			text, err := doc(host)
			if err != nil {
				errs = append(errs, err.Error())
				continue
			}
			if !strings.Contains(text, strings.Join(strings.Fields(r.sentence), " ")) {
				errs = append(errs, fmt.Sprintf("%s: the markers file's sentence %q is not in go doc %s", host, r.sentence, host))
			}
		}
	}
	for host, r := range rows {
		if r.order == "S" && declared[host] {
			ordered := false
			for _, n := range names {
				if strings.TrimPrefix(n, "go/") != host {
					continue
				}
				for _, a := range tg.Prims[n].Args {
					if q, ok := strings.CutPrefix(a, emit.CallbackPrefix); ok && tg.Callbacks[q].Ordered {
						ordered = true
					}
				}
			}
			if !ordered {
				errs = append(errs, fmt.Sprintf("%s's documentation states its order, and targets/go does not declare it `ordered`", host))
			}
		}
	}
	return errs
}
