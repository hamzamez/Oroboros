package stdlib

import (
	"archive/zip"
	"fmt"
	"io"
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

// EVERY MARKER OF THE OTHER HOSTS IS ITS HOST'S SENTENCE (ADR 0143): `ordered`
// and `calls K` on targets/js, targets/java and the windows family, against
// callbacks-hosts-evidence.txt. Checked both ways:
//   - every callback claim declared `ordered`, or with a count other than the
//     default 0.., has rows giving that marker, and every row names a
//     declaration whose claim carries the marker it gives;
//   - a row whose source is the JDK's src.zip has its sentence in that file's
//     javadoc. ECMA-262's and Microsoft Learn's are web pages, kept as data.
//
// The faults: a JDK sentence its file does not contain, and a marker with its
// rows removed, each of which must be refused.
func TestEveryCallbackMarkerIsItsHostsSentence(t *testing.T) {
	rows, err := evidenceRows(filepath.Join(root, "gauntlet", "stdlib", "callbacks-hosts-evidence.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var tgs []*emit.Target
	for _, d := range []string{"js", "java", "windows"} {
		tg, err := emit.LoadTarget(filepath.Join(root, "targets", d))
		if err != nil {
			t.Fatal(err)
		}
		tgs = append(tgs, tg)
	}
	src, err := jdkSource()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range checkMarkers(tgs, rows, src) {
		t.Error(e)
	}
	// the faults
	bad := append([]evidenceRow(nil), rows...)
	for i := range bad {
		if strings.HasPrefix(bad[i].source, "src.zip:") {
			bad[i].sentence = "the elements are processed in random order"
			break
		}
	}
	if len(checkMarkers(tgs, bad, src)) == 0 {
		t.Error("a sentence the JDK does not contain was accepted as the evidence: the check cannot fail")
	}
	var less []evidenceRow
	for _, r := range rows {
		if r.decl != "windows/synchapi.InitOnceExecuteOnce" {
			less = append(less, r)
		}
	}
	if len(checkMarkers(tgs, less, src)) == 0 {
		t.Error("a `calls 0..1` with no row was accepted: the check cannot fail")
	}
}

type evidenceRow struct {
	decl, marker, source, sentence string
}

func evidenceRows(path string) ([]evidenceRow, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []evidenceRow
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 4 {
			return nil, fmt.Errorf("an evidence row has declaration, marker, source and sentence: %q", line)
		}
		out = append(out, evidenceRow{f[0], f[1], f[2], f[3]})
	}
	return out, nil
}

// markerOf is a claim's markers as an evidence row writes them: `ordered`,
// `calls K` for a count other than the default, K as a range.
func markerOf(c emit.CallbackSig) []string {
	var out []string
	if c.Ordered {
		out = append(out, "ordered")
	}
	if c.Calls != "" {
		f := strings.Fields(c.Calls) // "int lo hi" or "int lo"
		k := ""
		switch {
		case len(f) == 3 && f[1] == f[2]:
			k = f[1]
		case len(f) == 3:
			k = f[1] + ".." + f[2]
		case len(f) == 2:
			k = f[1] + ".."
		default:
			k = c.Calls
		}
		if k != "0.." {
			out = append(out, "calls "+k)
		}
	}
	return out
}

func checkMarkers(tgs []*emit.Target, rows []evidenceRow, src func(string) (string, error)) []string {
	var errs []string
	declared := map[string]bool{} // decl + "\x00" + marker
	for _, tg := range tgs {
		var names []string
		for n := range tg.Prims {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			for _, a := range tg.Prims[n].Args {
				q, ok := strings.CutPrefix(a, emit.CallbackPrefix)
				if !ok {
					continue
				}
				for _, m := range markerOf(tg.Callbacks[q]) {
					declared[n+"\x00"+m] = true
				}
			}
		}
	}
	given := map[string]bool{}
	for _, r := range rows {
		given[r.decl+"\x00"+r.marker] = true
		if !declared[r.decl+"\x00"+r.marker] {
			errs = append(errs, fmt.Sprintf("the evidence gives %s `%s`, and no claim it takes is declared so", r.decl, r.marker))
		}
		if file, ok := strings.CutPrefix(r.source, "src.zip:"); ok {
			text, err := src(file)
			if err != nil {
				errs = append(errs, err.Error())
				continue
			}
			if !strings.Contains(text, strings.Join(strings.Fields(r.sentence), " ")) {
				errs = append(errs, fmt.Sprintf("%s: the sentence %q is not in the JDK's %s", r.decl, r.sentence, file))
			}
		} else if !strings.HasPrefix(r.source, "https://") {
			errs = append(errs, fmt.Sprintf("%s: a source is the JDK's src.zip or a page's URL, not %q", r.decl, r.source))
		}
	}
	var keys []string
	for k := range declared {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !given[k] {
			d, m, _ := strings.Cut(k, "\x00")
			errs = append(errs, fmt.Sprintf("%s's callback is declared `%s`, and callbacks-hosts-evidence.txt gives "+
				"no sentence of its host stating it", d, m))
		}
	}
	return errs
}

// jdkSource reads a file of the JDK's src.zip, the java on PATH's own, as its
// javadoc's text: each line's leading `*` dropped and whitespace collapsed.
func jdkSource() (func(string) (string, error), error) {
	out, err := exec.Command("java", "-XshowSettings:properties", "-version").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("java -XshowSettings: %v", err)
	}
	home := ""
	for _, line := range strings.Split(string(out), "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok && strings.TrimSpace(k) == "java.home" {
			home = strings.TrimSpace(v)
		}
	}
	if home == "" {
		return nil, fmt.Errorf("java reports no java.home")
	}
	z, err := zip.OpenReader(filepath.Join(home, "lib", "src.zip"))
	if err != nil {
		return nil, fmt.Errorf("the JDK's source archive, the JVM markers' evidence: %v", err)
	}
	return func(name string) (string, error) {
		f, err := z.Open(name)
		if err != nil {
			return "", fmt.Errorf("%s in the JDK's src.zip: %v", name, err)
		}
		defer f.Close()
		b, err := io.ReadAll(f)
		if err != nil {
			return "", err
		}
		var words []string
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "*")
			words = append(words, strings.Fields(line)...)
		}
		return strings.Join(words, " "), nil
	}, nil
}
