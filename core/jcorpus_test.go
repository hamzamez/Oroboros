package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// corpusFiles is the corpus: the .oro files the repository tracks, read from
// git's index. A walk of the working tree read whatever was there at that
// instant, other packages' tests' temporary programs included (cmd/gen writes
// them under the repository, where lib/ is), so it failed one run in five when
// one vanished mid-walk, and would have reported a test's deliberately invalid
// program as corpus (the book's finding 78). PlantCorpusWalk walks again.
func corpusFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	if plantCorpusWalk {
		filepath.Walk("..", func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(p, ".oro") {
				files = append(files, p)
			}
			return nil
		})
		return files
	}
	cmd := exec.Command("git", "ls-files", "--", "*.oro")
	cmd.Dir = ".."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("the corpus is the repository's tracked .oro files, and git ls-files failed: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files = append(files, filepath.Join("..", filepath.FromSlash(line)))
		}
	}
	return files
}

// plantCorpusWalk: TestTheCorpusIsTheRepositorysFiles must catch it.
var plantCorpusWalk bool

// THE CORPUS IS THE REPOSITORY'S FILES (finding 78): an untracked .oro file in
// the tree, as another package's test writes one, is not read, and with the
// working tree walked again it is.
func TestTheCorpusIsTheRepositorysFiles(t *testing.T) {
	stray, err := os.MkdirTemp("..", "corpusstray")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(stray)
	p := filepath.Join(stray, "p.oro")
	if err := os.WriteFile(p, []byte("this is not J"), 0o644); err != nil {
		t.Fatal(err)
	}
	has := func(files []string) bool {
		for _, f := range files {
			if filepath.Clean(f) == filepath.Clean(p) {
				return true
			}
		}
		return false
	}
	if has(corpusFiles(t)) {
		t.Errorf("an untracked program in the tree was read as corpus")
	}
	plantCorpusWalk = true
	defer func() { plantCorpusWalk = false }()
	if !has(corpusFiles(t)) {
		t.Errorf("with the working tree walked again the stray program should be read, so the test reaches the rule")
	}
}

// EVERY SOURCE IS J AND READS (ADR 0068; jswitch-2026-10-16). The corpus moved to
// J in step 5: each .oro file in the repository is J, none is today's notation,
// and each reads, a target file, the language's facts or a fragment as a model's
// forms, anything else as a program. The gate compiles them; this says, first
// and fast, that none has drifted back.
func TestEveryCorpusFileReadsInJ(t *testing.T) {
	files := corpusFiles(t)
	programs, models := 0, 0
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		slash := filepath.ToSlash(path)
		if strings.Contains(slash, "/targets/") || strings.HasSuffix(slash, "emit/lang-facts.oro") || IsFragment(src) {
			if _, err := ReadTargetSource(src, path, ""); err != nil {
				t.Errorf("%s: %v", path, err)
			}
			models++
			continue
		}
		if _, err := ReadEntry(src, path); err != nil {
			t.Errorf("%s: %v", path, err)
		}
		programs++
	}
	if programs+models < 200 {
		t.Fatalf("%d programs and %d models: the corpus has more", programs, models)
	}
	t.Logf("%d programs and %d target files and fragments read in J", programs, models)
}
