package j

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/core"
)

func corpus(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, dir := range []string{"../examples", "../lib", "../gauntlet/stdlib/acceptance", "../gauntlet/differential/cases"} {
		filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(p, ".oro") {
				files = append(files, p)
			}
			return nil
		})
	}
	return files
}

// EVERY PROGRAM PRINTS IN J, EVERY COMMENT CARRIED (spec/j.md §7): over the
// corpus, each program today's reader reads is printed, once and the same way
// twice, with no two names spelled alike, and every comment of the source
// appears in the J text in the source's order. Target fragments wait for step
// 2d and are counted.
func TestEveryProgramPrintsInJWithEveryComment(t *testing.T) {
	printed, fragments, uncertain := 0, 0, 0
	for _, path := range corpus(t) {
		b, _ := os.ReadFile(path)
		src := string(b)
		if _, err := core.ReadTrees(src); err != nil {
			continue
		}
		r, err := Translate(src, filepath.Dir(path), "..")
		if err != nil {
			if strings.Contains(err.Error(), "step 2d") {
				fragments++
				continue
			}
			t.Errorf("%s: %v", path, err)
			continue
		}
		again, _ := Translate(src, filepath.Dir(path), "..")
		if again.J != r.J {
			t.Errorf("%s: two translations differ", path)
		}
		// EVERY BUFFER IN PROGRAM ORDER (ADR 0052): unthreading is total on the
		// corpus, so no threaded spelling is left. A map buffer stays threaded
		// until ADR 0058 is built.
		for _, threaded := range []string{"store(", " linear = array", " linear = {"} {
			if strings.Contains(r.J, threaded) {
				t.Errorf("%s: a buffer is still threaded, %q", path, threaded)
			}
		}
		c, _ := core.ReadConcrete(src)
		at := 0
		for _, g := range c.Gaps {
			want := "//" + strings.TrimRight(g.Text, " \t\r")
			i := strings.Index(r.J[at:], want)
			if i < 0 {
				t.Errorf("%s: the comment %q is missing or out of order", path, want)
				break
			}
			at += i + len(want)
		}
		printed++
		if dir := os.Getenv("J_DUMP"); dir != "" {
			name := strings.ReplaceAll(strings.TrimPrefix(filepath.ToSlash(path), "../"), "/", "_")
			os.MkdirAll(dir, 0o755)
			os.WriteFile(filepath.Join(dir, strings.TrimSuffix(name, ".oro")+".j"), []byte(r.J), 0o644)
		}
		uncertain += r.Uncertain
		if os.Getenv("J_UNCERTAIN") != "" && r.Uncertain > 0 {
			t.Logf("%s: %v", path, r.UncertainNames)
		}
	}
	if printed < 140 {
		t.Fatalf("printed %d programs; the corpus has more", printed)
	}
	t.Logf("%d programs printed in J, %d target fragments for step 2d, %d uncertain applications", printed, fragments, uncertain)
}

// EVERY TARGET FILE AND FRAGMENT PRINTS IN J, EVERY COMMENT CARRIED (spec/j.md
// §2.1–2.5, §2.11–2.12): each file under targets/, and each program file that
// holds a fragment, prints once and the same way twice, every comment in order,
// and no positional hole survives in a host declaration's template.
func TestEveryTargetPrintsInJWithEveryComment(t *testing.T) {
	var files []string
	filepath.Walk("../targets", func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(p, ".oro") {
			files = append(files, p)
		}
		return nil
	})
	for _, path := range corpus(t) {
		b, _ := os.ReadFile(path)
		if _, err := Translate(string(b), filepath.Dir(path), ".."); err != nil && strings.Contains(err.Error(), "step 2d") {
			files = append(files, path)
		}
	}
	printed := 0
	for _, path := range files {
		b, _ := os.ReadFile(path)
		src := string(b)
		r, err := TranslateTarget(src, "..")
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		again, _ := TranslateTarget(src, "..")
		if again.J != r.J {
			t.Errorf("%s: two translations differ", path)
		}
		rc, _ := core.ReadRawConcrete(src)
		at := 0
		for _, g := range rc.Gaps {
			want := "//" + strings.TrimRight(g.Text, " \t\r")
			i := strings.Index(r.J[at:], want)
			if i < 0 {
				t.Errorf("%s: the comment %q is missing or out of order", path, want)
				break
			}
			at += i + len(want)
		}
		for _, line := range strings.Split(r.J, "\n") {
			code := strings.TrimSpace(line)
			if strings.HasPrefix(code, "//") || !strings.Contains(code, "fn ") {
				continue
			}
			if (strings.Contains(code, "host expr") || strings.Contains(code, "host stmt")) && strings.Contains(code, "%s") {
				t.Errorf("%s: a host declaration keeps a positional hole: %s", path, line)
			}
		}
		if dir := os.Getenv("J_DUMP"); dir != "" {
			name := strings.ReplaceAll(strings.TrimPrefix(filepath.ToSlash(path), "../"), "/", "_")
			os.MkdirAll(dir, 0o755)
			os.WriteFile(filepath.Join(dir, strings.TrimSuffix(name, ".oro")+".j"), []byte(r.J), 0o644)
		}
		printed++
	}
	if printed < 40 {
		t.Fatalf("printed %d target files; there are more", printed)
	}
	t.Logf("%d target files and fragments printed in J", printed)
}
