package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// EVERY SOURCE IS J AND READS (ADR 0068; jswitch-2026-10-16). The corpus moved to
// J in step 5: each .oro file in the repository is J, none is today's notation,
// and each reads, a target file, the language's facts or a fragment as a model's
// forms, anything else as a program. The gate compiles them; this says, first
// and fast, that none has drifted back.
func TestEveryCorpusFileReadsInJ(t *testing.T) {
	var files []string
	filepath.Walk("..", func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && (info.Name() == ".git" || info.Name() == ".check") {
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.HasSuffix(p, ".oro") {
			files = append(files, p)
		}
		return nil
	})
	programs, models := 0, 0
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		if Todays(src) {
			t.Errorf("%s is today's notation, not J", path)
			continue
		}
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
