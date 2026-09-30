package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"oroboros/emit"
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
	var sigma []string
	for name := range tg.Prims {
		if strings.HasPrefix(name, "os.") || strings.HasPrefix(name, "io.") {
			sigma = append(sigma, name)
		}
	}
	sort.Strings(sigma)
	if len(sigma) < 9 {
		t.Fatalf("Σ has %d names, %v; the loader lost the library layer", len(sigma), sigma)
	}
	for _, name := range sigma {
		if !strings.Contains(string(text), "("+name+" ") && !strings.Contains(string(text), "("+name+")") {
			t.Errorf("%s is in Σ and roundtrip.oro does not call it, so no host runs it", name)
		}
	}

	const want = "hi €\nok\n7\n"
	hosts := []struct {
		target, artifact string
		tools            []string
		run              func(artifact string) *exec.Cmd
	}{
		{"go", "rt.exe", []string{"go"}, func(a string) *exec.Cmd { return exec.Command(a) }},
		{"js", "rt.mjs", []string{"node"}, func(a string) *exec.Cmd { return exec.Command("node", a) }},
		{"java", "classes", []string{"javac", "java"}, func(a string) *exec.Cmd {
			return exec.Command("java", "-Dfile.encoding=UTF-8", "-Dstdout.encoding=UTF-8",
				"-Dsun.stdout.encoding=UTF-8", "-cp", a, "Main")
		}},
	}
	for _, h := range hosts {
		t.Run(h.target, func(t *testing.T) {
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
