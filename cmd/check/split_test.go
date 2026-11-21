package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// LPT IS A PARTITION, AND WITHIN GRAHAM'S BOUND. Every job is on exactly one
// machine, and the makespan is at most (4/3 − 1/(3m))·OPT, here checked against
// the lower bound max(maxᵢ wᵢ, Σw/m) ≤ OPT.
func TestLPTIsAPartitionWithinGrahamsBound(t *testing.T) {
	w := map[string]float64{"a": 7, "b": 5, "c": 4, "d": 4, "e": 3, "f": 3, "g": 2, "h": 1}
	var jobs []string
	total, heaviest := 0.0, 0.0
	for j, x := range w {
		jobs = append(jobs, j)
		total += x
		heaviest = max(heaviest, x)
	}
	for m := 1; m <= 5; m++ {
		parts, loads := lpt(jobs, w, m)
		var got []string
		span := 0.0
		for i, p := range parts {
			got = append(got, p...)
			span = max(span, loads[i])
		}
		sort.Strings(got)
		want := append([]string(nil), jobs...)
		sort.Strings(want)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("m=%d: the schedule is not a partition: %v", m, parts)
		}
		lb := max(heaviest, total/float64(m))
		if span > (4.0/3.0-1.0/(3.0*float64(m)))*lb+1e-9 {
			t.Errorf("m=%d: makespan %.2f past Graham's bound over the lower bound %.2f", m, span, lb)
		}
	}
}

// THE SPLIT COMPILER STEP RUNS EVERY TEST, end to end on a module of three
// tests weighed to need three processes: it passes, every test reported; and
// with one test left out of its group (PlantGroupDropsATest) it fails naming it.
func TestTheSplitCompilerStepRunsEveryTest(t *testing.T) {
	root := t.TempDir()
	write := func(p, s string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module splitcheck\n\ngo 1.22\n")
	write("p/p.go", "package p\n")
	write("p/p_test.go", "package p\n\nimport \"testing\"\n\n"+
		"func TestA(t *testing.T) {}\nfunc TestB(t *testing.T) {}\nfunc TestC(t *testing.T) {}\n")
	write("gauntlet/check/testtimes.txt", "splitcheck/p TestA 10\nsplitcheck/p TestB 10\nsplitcheck/p TestC 10\n")
	old := compilerPackages
	compilerPackages = []string{"./p/"}
	defer func() { compilerPackages = old }()
	c := &checker{root: root, work: filepath.Join(root, ".check"), testProcs: 3}
	if err := os.MkdirAll(filepath.Join(c.work, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := c.compilerStep()
	if r.status != pass || !strings.Contains(r.detail, "3 tests in 3 processes") {
		t.Fatalf("three tests on three processes: %s %s\n%s", r.status, r.detail, r.log)
	}
	PlantGroupDropsATest = true
	r = c.compilerStep()
	PlantGroupDropsATest = false
	if r.status != fail || !strings.Contains(r.detail, "did not run") || !strings.Contains(r.detail, "p.TestA") {
		t.Errorf("with a test left out of its group the step must fail naming it: %s %s", r.status, r.detail)
	}
}

// A VERDICT IS REUSED ONLY FOR ITS TREE. The input's hash changes with a
// tracked file and with an untracked one, and not with an ignored one; a
// record decides an accept only for its own hash with its compiler and
// differential passing (PlantVerdictAnyTree reuses it anywhere); and the
// emission rerun must be the recorded one (PlantEmissionDigestUnchecked takes
// any).
func TestAVerdictIsReusedOnlyForItsTree(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(p, s string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, p), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	write(".gitignore", "ignored.txt\n")
	write("a.oro", "one")
	git("add", ".")
	c := &checker{root: root}
	hash := func() string {
		t.Helper()
		h, err := c.inputHash(false)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	h0 := hash()
	if hash() != h0 {
		t.Fatal("the input's hash is not a function of the tree")
	}
	write("ignored.txt", "x")
	if hash() != h0 {
		t.Error("an ignored file changed the input's hash")
	}
	write("a.oro", "two")
	h1 := hash()
	if h1 == h0 {
		t.Error("a tracked file's edit left the input's hash unchanged")
	}
	write("new.oro", "untracked")
	if hash() == h1 {
		t.Error("an untracked file left the input's hash unchanged")
	}
	if h, _ := c.inputHash(true); h == hash() {
		t.Error("-tier2 left the input's hash unchanged")
	}

	v := runRecord{Tree: "H1", Steps: map[string]status{"compiler": pass, "differential": pass, "tooling": pass},
		Emission: map[string]string{"emission": "D1"}}
	if ok, why := reusable(v, "H1", "differential"); !ok {
		t.Errorf("the record of this tree, passing: want reused, got %s", why)
	}
	if ok, _ := reusable(v, "H2", "differential"); ok {
		t.Error("a record of another tree was reused")
	}
	v.Steps["compiler"] = fail
	if ok, _ := reusable(v, "H1", "differential"); ok {
		t.Error("a record whose compiler step failed was reused")
	}
	v.Steps["compiler"] = pass
	PlantVerdictAnyTree = true
	ok, _ := reusable(v, "H2", "differential")
	PlantVerdictAnyTree = false
	if !ok {
		t.Error("with the tree unchecked, another tree's record should be reused, so the witness can fail")
	}
	if !sameEmission(v, "emission", "D1") || sameEmission(v, "emission", "D2") {
		t.Error("the emission rerun is compared with the recorded digest")
	}
	PlantEmissionDigestUnchecked = true
	same := sameEmission(v, "emission", "D2")
	PlantEmissionDigestUnchecked = false
	if !same {
		t.Error("with the digest unchecked, any emission should pass, so the witness can fail")
	}
}
