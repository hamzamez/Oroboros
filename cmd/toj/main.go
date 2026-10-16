// Command toj prints a program of today's notation in J (spec/j.md §7), or,
// with -w, rewrites each file in J in place: the corpus's one-time move to J
// (step 5, jswitch-2026-10-16).
//
//	go run ./cmd/toj FILE.oro
//	go run ./cmd/toj -w FILE.oro…
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"oroboros/j"
)

func main() {
	write := flag.Bool("w", false, "rewrite each file in J, in place")
	flag.Parse()
	if flag.NArg() == 0 || !*write && flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: toj FILE.oro | toj -w FILE.oro…")
		os.Exit(2)
	}
	failed := 0
	for _, path := range flag.Args() {
		r, err := translate(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "toj: %s: %v\n", path, err)
			failed++
			continue
		}
		if !*write {
			fmt.Print(r.J)
			if r.Uncertain > 0 {
				fmt.Fprintf(os.Stderr, "toj: %d one-argument applications of a local name printed as applications\n", r.Uncertain)
			}
			continue
		}
		if err := os.WriteFile(path, []byte(r.J), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "toj: %s: %v\n", path, err)
			failed++
		}
	}
	if failed > 0 {
		os.Exit(1)
	}
}

// translate is a file in J: a target file, the language's facts or a fragment
// as a model's forms, anything else as a program.
func translate(path string) (*j.Result, error) {
	slash := filepath.ToSlash(path)
	if strings.HasPrefix(slash, "targets/") || strings.HasSuffix(slash, "emit/lang-facts.oro") {
		return j.TranslateTargetFile(path, ".")
	}
	r, err := j.TranslateFile(path, ".")
	if err != nil && strings.Contains(err.Error(), "step 2d") {
		return j.TranslateTargetFile(path, ".")
	}
	return r, err
}
