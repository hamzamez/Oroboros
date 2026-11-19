package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

// runOn builds src for target and runs it, answering what it printed, its
// lines joined by spaces, or the build's error.
func runOn(t *testing.T, src, target string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "p.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	art := map[string]string{"js": "p.mjs", "java": "classes"}[target]
	if art == "" {
		art = "p.exe"
	}
	art = filepath.Join(dir, art)
	if err := run("../../targets", path, target, art, "../../lib", false, false, ""); err != nil {
		return "", err
	}
	var cmd *exec.Cmd
	switch target {
	case "js":
		cmd = program("node", art)
	case "java":
		cmd = program("java", "-cp", art, "Main")
	default:
		cmd = program(art)
	}
	b, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s: running it: %v\n%s", target, err, b)
	}
	return strings.Join(strings.Fields(string(b)), " "), nil
}

// A PRODUCT AS A MAP'S VALUE IS ITS COMPONENTS (ADR 0128 item 7, data.md
// §3.3): a tuple of three and a record of two, inserted, overwritten, read at
// a present and an absent key, counted by len and listed by keys, through a
// helper that takes the map unclaimed, on every target: Go a struct, the JVM a
// record, JavaScript an array, windows and windows32 w cells of a slot of
// 2 + w. The faults:
//   - the map left to lowering, which refuses several values where one is
//     wanted, the refusal before this build;
//   - a case arm's binder renamed without its projections, so `r.x` is read
//     as a module's name;
//   - a map read's payload given no shape, so `r.x` has no labels;
//   - the map's value typed by its first slot, which the verifier's W4 must
//     refuse;
//   - windows' slots given w = 1, three cells for w components, which must
//     answer otherwise.
func TestAProductIsAMapsValue(t *testing.T) {
	const tuples = "use io\n\n" +
		"fn get(m, k) { case m[k] { 'some(p) -> { (a, b, c) = p; a + b * c }; 'none -> 0 - 1 } }\n\n" +
		"fn run(n ∈ 0..16) {\n" +
		"    m = { var m := map(16); loop i = 0 { i >= n -> { m[1] := (7, 8, 9); m }; else -> { m[i] := (i * 10, i + 1, 2); again(i + 1) } } }\n" +
		"    ks = keys(m)\n" +
		"    io.print_int(get(m, 1))\n    io.print_int(get(m, 3))\n    io.print_int(get(m, 99))\n" +
		"    io.print_int(len(m))\n    io.print_int(len(ks))\n}\n\n" +
		"export fn main() { run(5); run(2) }\n"
	const wantTuples = "79 38 -1 5 5 79 -1 -1 2 2"
	const records = "use io\n\n" +
		"fn run(n ∈ 0..16) {\n" +
		"    m = { var m := map(16); loop i = 0 { i >= n -> m; else -> { m[i] := record { y : i + 1, x : i * 10 }; again(i + 1) } } }\n" +
		"    case m[2] { 'some(r) -> r.x * 100 + r.y; 'none -> 0 }\n}\n\n" +
		"export fn main() { io.print_int(run(5)) }\n"
	const wantRecords = "2003"
	targets := []string{"go", "windows", "windows32"}
	if _, err := exec.LookPath("node"); err == nil {
		targets = append(targets, "js")
	}
	if _, err := exec.LookPath("javac"); err == nil {
		targets = append(targets, "java")
	}
	for _, target := range targets {
		for src, want := range map[string]string{tuples: wantTuples, records: wantRecords} {
			if got, err := runOn(t, src, target); err != nil || got != want {
				t.Errorf("%s: %q, %v; want %s", target, got, err, want)
			}
		}
	}
	for _, c := range []struct {
		name, src, says string
		plant           *bool
	}{
		{"the map left to lowering", tuples, "values where one is wanted", &emit.PlantMapProductsUnflattened},
		{"the binder renamed without its projections", records, "is not imported", &core.PlantCaseBinderUnbound},
		{"the payload given no shape", records, "record labels are not known", &core.PlantMapRecordUnshaped},
		{"the value typed by its first slot", tuples, "W4", &ir.PlantSlotsUntyped},
	} {
		*c.plant = true
		_, err := runOn(t, c.src, "go")
		*c.plant = false
		if err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("with %s: %v, want a refusal saying %q", c.name, err, c.says)
		}
	}
	ir.PlantMapStrideOne = true
	got, err := runOn(t, tuples, "windows")
	ir.PlantMapStrideOne = false
	if err == nil && got == wantTuples {
		t.Errorf("with windows' slots of three cells for three components the answer is still %s: the case cannot fail", got)
	}
}
