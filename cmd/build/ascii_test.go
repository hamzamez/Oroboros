package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/emit"
	"oroboros/ir"
)

// buildAndRun builds src for target and runs it, answering its output.
func buildAndRun(t *testing.T, src, target, bigRepr string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "p.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "p.exe")
	if err := run("../../targets", path, target, exe, "../../lib", false, false, bigRepr); err != nil {
		return "", err
	}
	out, err := exec.Command(exe).CombinedOutput()
	if err != nil {
		t.Fatalf("running it: %v\n%s", err, out)
	}
	return string(out), nil
}

// ι : (B₇)* ↪ Σ* TAKES ITS DOMAIN FROM THE FACTS (ADR 0095): a table whose
// elements are proven in 0..127 is text; one that may hold 200 is refused,
// saying which argument, before any target is asked.
func TestASCIIsDomainIsDecided(t *testing.T) {
	const ok = "use win/print\n" +
		"export fn main() {\n" +
		"    var b := array { 48 : _ ∈ 0..3 }\n" +
		"    b[1] := 57\n" +
		"    print.print_str(ascii(b, 0, 4))\n}\n"
	out, err := buildAndRun(t, ok, "windows", "")
	if err != nil || strings.TrimSpace(out) != "0900" {
		t.Errorf("ι of [48, 57, 48, 48]: %q, %v", out, err)
	}
	bad := strings.Replace(ok, "b[1] := 57", "b[1] := 200", 1)
	if _, err := buildAndRun(t, bad, "windows", ""); err == nil || !strings.Contains(err.Error(), "0..127") || !strings.Contains(err.Error(), "[0, 200]") {
		t.Errorf("ι of a table that may hold 200: want its domain 0..127 refused against [0, 200]; got %v", err)
	}
}

// ι OBSERVES A LIVE BUFFER BY ITS MEANING (ADR 0095, emit/observer.go): two of
// them on one buffer, joined by ++. THE PLANTED FAULT takes the observer away,
// and the second is a use after the buffer was handed on.
func TestASCIIObservesALiveBuffer(t *testing.T) {
	const src = "use win/print\n" +
		"export fn main() {\n" +
		"    var b := array { 48 : _ ∈ 0..3 }\n" +
		"    b[0] := 45\n" +
		"    print.print_str(ascii(b, 0, 1) ++ ascii(b, 2, 2))\n}\n"
	if out, err := buildAndRun(t, src, "windows", ""); err != nil || strings.TrimSpace(out) != "-00" {
		t.Errorf("two ι of one live buffer: %q, %v", out, err)
	}
	emit.PlantObserverFault = true
	defer func() { emit.PlantObserverFault = false }()
	if _, err := buildAndRun(t, src, "windows", ""); err == nil || !strings.Contains(err.Error(), "handed on") {
		t.Errorf("with ι not an observer, want the second refused as a use after handing on; got %v", err)
	}
}

// THE LIMB LIBRARY'S RENDER IS A HOST'S NUMERAL (ADR 0095): 50!, 65 digits in
// 13 limbs, rendered by render on Go's fixed limbs where the host's bignum
// would have been converted through, against math/big's String.
func TestTheLimbRenderIsTheHostsNumeral(t *testing.T) {
	const src = "use go/fmt\n" +
		"fn fact(n ∈ 0..50) (0..2^300) { loop k = 1, a = 1 { k > n -> a; else -> again(k + 1, a * k) } }\n" +
		"export fn main() { fmt.Println(big_str(fact(50))) }\n"
	ir.PlantRenderEverywhere = true
	defer func() { ir.PlantRenderEverywhere = false }()
	out, err := buildAndRun(t, src, "go", "limbs")
	if want := "30414093201713378043612608166064768844377641568960512000000000000"; err != nil || strings.TrimSpace(out) != want {
		t.Errorf("render(50!) on Go's limbs: %q, %v; want %s", out, err, want)
	}
	// AND IT IS THE LIBRARY'S, not the host's by another road: the emitted Go
	// imports no math/big, which the conversion through the host would.
	if emitted := keptGo(t, src); strings.Contains(emitted, "math/big") {
		t.Errorf("render through the limb library still converts through math/big")
	}
}

// keptGo builds src for Go on limbs, keeping the emitted source, and answers it.
func keptGo(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "p.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	err = run("../../targets", path, "go", filepath.Join(dir, "p.exe"), "../../lib", true, false, "limbs")
	w.Close()
	os.Stdout = stdout
	if err != nil {
		t.Fatal(err)
	}
	var printed strings.Builder
	buf := make([]byte, 4096)
	for {
		n, e := r.Read(buf)
		printed.WriteString(string(buf[:n]))
		if e != nil {
			break
		}
	}
	_, after, ok := strings.Cut(printed.String(), "source kept in ")
	if !ok {
		t.Fatalf("no kept source in %q", printed.String())
	}
	work := strings.TrimSpace(strings.SplitN(after, "\n", 2)[0])
	defer os.RemoveAll(work)
	var all strings.Builder
	files, _ := filepath.Glob(filepath.Join(work, "*.go"))
	for _, f := range files {
		b, _ := os.ReadFile(f)
		all.Write(b)
	}
	if all.Len() == 0 {
		t.Fatalf("no Go source kept in %s", work)
	}
	return all.String()
}
