package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// THE BOOK STEP: every committed chapter of docs/book is re-run against the
// compiler by docs/book/check.go, which compares each example's file, output,
// refusal and emitted code with what the chapter prints. A chapter is a set of
// claims about the language, and a claim nothing re-runs goes stale the first
// time a message changes: chapters 1, 5 and 6 each quoted a refusal the
// compiler no longer gave (chapters56-2026-11-15). Committed chapters only: a
// chapter being written lives in the working tree, and is its author's to run.

var chapterRe = regexp.MustCompile(`^docs/book/[0-9][0-9]-[^/]*\.md$`)

// chapters is every committed chapter, in order.
func (c *checker) chapters() ([]string, error) {
	cmd := exec.Command("git", "ls-files", "docs/book")
	cmd.Dir = c.root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files docs/book: %v", err)
	}
	var list []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if chapterRe.MatchString(l) {
			list = append(list, l)
		}
	}
	sort.Strings(list)
	return list, nil
}

func (c *checker) book() result {
	list, err := c.chapters()
	if err != nil {
		return result{status: fail, detail: err.Error()}
	}
	if len(list) == 0 {
		return result{status: fail, detail: "no committed chapter in docs/book"}
	}
	type verdict struct {
		ok             bool
		summary, fails string
	}
	vs := make([]verdict, len(list))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 3) // each run builds four tools of its own
	for i, ch := range list {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			cmd := exec.Command("go", "run", filepath.FromSlash("docs/book/check.go"), ch)
			cmd.Dir = c.root
			out, err := cmd.CombinedOutput()
			text := strings.TrimSpace(string(out))
			last := text
			if k := strings.LastIndex(text, "\n"); k >= 0 {
				last = text[k+1:]
			}
			vs[i] = verdict{ok: err == nil, summary: last}
			if err != nil {
				vs[i].fails = firstLines(text, 12)
			}
		}()
	}
	wg.Wait()
	blocks := regexp.MustCompile(`(\d+) block\(s\) checked`)
	var failed []string
	checked := 0
	for i, v := range vs {
		if m := blocks.FindStringSubmatch(v.summary); m != nil {
			var n int
			fmt.Sscan(m[1], &n)
			checked += n
		}
		if !v.ok {
			failed = append(failed, list[i]+":\n      "+strings.ReplaceAll(v.fails, "\n", "\n      "))
		}
	}
	if len(failed) > 0 {
		return result{status: fail, log: "   " + strings.Join(failed, "\n   "),
			detail: fmt.Sprintf("%d of %d chapter(s) do not hold", len(failed), len(list))}
	}
	return result{status: pass, detail: fmt.Sprintf("%d chapters, %d blocks, each what the compiler gives", len(list), checked)}
}
