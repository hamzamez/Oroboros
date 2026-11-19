package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

// EVERY PORTABLE NAME RUNS ON EVERY HOST THAT PROVIDES IT (lib/os/README.md).
//
// `emit`'s TestTheUnprefixedModulesShareOneInterface checks that the three
// cells declare the same names at the same types. That is a claim about
// signatures. A template is text the host compiles, and nothing read it: the
// portable WriteFile compiled on Go only, for as long as it existed, because
// no program called it anywhere else (libos-2026-09-30). The three tools in
// examples/io/ were measured byte-identical once, by hand, and never write.
//
// So one program calls every name of Σ, and this builds it on each host,
// runs it, and requires one output. The guard below keeps it from going
// vacuous: a name added to `os` or `io` and not called here fails.
func TestEveryPortableNameRunsOnEveryHost(t *testing.T) {
	const src = "../../examples/io/roundtrip.oro"
	text, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	tg, err := emit.LoadTargetLayers("go", []string{"../../targets"}, []string{"../../lib"})
	if err != nil {
		t.Fatal(err)
	}
	// Σ IS os's INTERFACE AND io's PRIMITIVES. `os` is a library module, one
	// definition over each host's cell os/host (oskinds-2026-10-06), so its
	// names are what lib/os.oro marks `pub` (ADR 0078); `io` is still a cell per host.
	var sigma []string
	for name := range tg.Prims {
		if strings.HasPrefix(name, "io.") {
			sigma = append(sigma, name)
		}
	}
	osSrc, err := os.ReadFile("../../lib/os.oro")
	if err != nil {
		t.Fatal(err)
	}
	osForms, err := core.ReadModule(string(osSrc), "../../lib/os.oro", "os")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range osForms {
		if f.Kind == "pub" {
			for _, n := range f.Names {
				sigma = append(sigma, "os."+n)
			}
		}
	}
	sort.Strings(sigma)
	if len(sigma) < 9 {
		t.Fatalf("Σ has %d names, %v; the loader lost the library layer", len(sigma), sigma)
	}
	for _, name := range sigma {
		// A NAME IS EXERCISED WHERE IT OCCURS AS A WORD: a function called, or a
		// nullary constructor of os's kinds matched as a `case` pattern, which is
		// how a sum is eliminated.
		// In J, a call `io.print_line(…)` and a kind matched as a tag
		// `os.'not_found` (σ_J, spec/j.md §1.5; ADR 0059).
		spelled := core.Spell(name)
		tag := strings.Replace(spelled, ".", ".'", 1)
		if !regexp.MustCompile(`(^|[^\w.'])(` + regexp.QuoteMeta(spelled) + `|` + regexp.QuoteMeta(tag) + `)(\W|$)`).MatchString(string(text)) {
			t.Errorf("%s is in Σ and roundtrip.oro does not call it, so no host runs it", name)
		}
	}

	// The last line is the kinds' claim: a missing path is not-found on every
	// host (spec/errors.md §6).
	const want = "hi €\nok\n7\n4 226\nnot-found\n"
	hosts := []struct {
		target, artifact string
		tools            []string
		run              func(artifact string) *exec.Cmd
	}{
		{"go", "rt.exe", []string{"go"}, func(a string) *exec.Cmd { return program(a) }},
		{"js", "rt.mjs", []string{"node"}, func(a string) *exec.Cmd { return program("node", a) }},
		{"java", "classes", []string{"javac", "java"}, func(a string) *exec.Cmd {
			return program("java", "-Dfile.encoding=UTF-8", "-Dstdout.encoding=UTF-8",
				"-Dsun.stdout.encoding=UTF-8", "-cp", a, "Main")
		}},
		// windows: its cells are J over kernel32 (lib/os/windows.oro, ADR 0112).
		{"windows", "rt.exe", nil, func(a string) *exec.Cmd { return program(a) }},
	}
	// NOTHING WAITS: on windows roundtrip's chain of nine J definitions passed
	// the reducer's depth bound (oswin-2026-11-15) until ADR 0113 kept them as
	// functions; it runs there since (probes-2026-11-19 found the skip stale).
	waits := map[string]string{}
	for _, h := range hosts {
		t.Run(h.target, func(t *testing.T) {
			if why, ok := waits[h.target]; ok {
				t.Skip(why)
			}
			for _, tool := range h.tools {
				if _, err := exec.LookPath(tool); err != nil {
					t.Skipf("%s is not on the path", tool)
				}
			}
			dir := t.TempDir()
			art := filepath.Join(dir, h.artifact)
			if err := run("../../targets", src, h.target, art, "../../lib", false, false, ""); err != nil {
				t.Fatalf("build: %v", err)
			}
			cmd := h.run(art)
			cmd.Args = append(cmd.Args, "put.txt")
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "ORO_ROUNDTRIP=ok")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			got, err := cmd.Output()
			if err != nil {
				t.Fatalf("run: %v\n%s", err, stderr.String())
			}
			if string(got) != want {
				t.Errorf("printed %q, want %q", got, want)
			}
			file, err := os.ReadFile(filepath.Join(dir, "put.txt"))
			if err != nil || string(file) != "hi €\n" {
				t.Errorf("the file holds %q (%v), want %q", file, err, "hi €\n")
			}
		})
	}
}

// CMP RUNS ON EVERY HOST (examples/io/cmp.oro, the 10-24 plan's Go-library
// program): two files compared a chunk at a time through `os.ReadAt` into two
// lent buffers. A difference is reported by its byte, a file that ends first
// by an EOF on it, equal files by nothing, and a missing file by its kind,
// one output on Go, JavaScript, the JVM and windows.
func TestCmpComparesOnEveryHost(t *testing.T) {
	const src = "../../examples/io/cmp.oro"
	files := map[string]string{"a.txt": "hello world\n", "b.txt": "hello there\n", "c.txt": "hello world\nmore\n", "d.txt": "hello world\n"}
	runs := [][]string{{"a.txt", "b.txt"}, {"c.txt", "a.txt"}, {"a.txt", "d.txt"}, {"a.txt", "nope.txt"}}
	const want = "a.txt b.txt differ: byte 7\ncmp: EOF on a.txt\ncmp: nope.txt: not-found\n"
	hosts := []struct {
		target, artifact string
		tools            []string
		run              func(artifact string) *exec.Cmd
	}{
		{"go", "cmp.exe", []string{"go"}, func(a string) *exec.Cmd { return program(a) }},
		{"js", "cmp.mjs", []string{"node"}, func(a string) *exec.Cmd { return program("node", a) }},
		{"java", "classes", []string{"javac", "java"}, func(a string) *exec.Cmd {
			return program("java", "-Dfile.encoding=UTF-8", "-Dstdout.encoding=UTF-8", "-cp", a, "Main")
		}},
		{"windows", "cmp.exe", nil, func(a string) *exec.Cmd { return program(a) }},
	}
	for _, h := range hosts {
		t.Run(h.target, func(t *testing.T) {
			for _, tool := range h.tools {
				if _, err := exec.LookPath(tool); err != nil {
					t.Skipf("%s is not on the path", tool)
				}
			}
			dir := t.TempDir()
			for n, c := range files {
				if err := os.WriteFile(filepath.Join(dir, n), []byte(c), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			art := filepath.Join(dir, h.artifact)
			if err := run("../../targets", src, h.target, art, "../../lib", false, false, ""); err != nil {
				t.Fatalf("build: %v", err)
			}
			var got bytes.Buffer
			for _, r := range runs {
				cmd := h.run(art)
				cmd.Args = append(cmd.Args, r...)
				cmd.Dir = dir
				out, err := cmd.Output()
				if err != nil {
					t.Fatalf("run %v: %v", r, err)
				}
				got.Write(out)
			}
			if got.String() != want {
				t.Errorf("printed %q, want %q", got.String(), want)
			}
		})
	}
	// THE FAULT: the ensure of ReadAt's components ignored, so its count is
	// its declared set's, 0..2^53-1 on JavaScript, and the offset plus it
	// leaves JavaScript's word: the build must be refused there
	ir.PlantComponentEnsuresIgnored = true
	defer func() { ir.PlantComponentEnsuresIgnored = false }()
	if err := run("../../targets", src, "js", filepath.Join(t.TempDir(), "cmp.mjs"), "../../lib", false, false, ""); err == nil {
		t.Errorf("with ReadAt's ensure ignored cmp still builds on JavaScript: the case cannot fail")
	}
}
