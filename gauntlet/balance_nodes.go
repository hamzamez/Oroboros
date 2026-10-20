//go:build ignore

// THE LANGUAGE'S SIZE IN NODES, the count of gauntlet/balance.py --nodes.
//
//	go run gauntlet/balance_nodes.go
//
// A cross-check on lines of Oroboros across a change of notation. J writes one
// statement to a line: the language's lines of code went from 1,745 to 1,869
// (+7%) and freq's from 163 to 194 across the switch, with no definition added;
// and balance.py counted J's `//` comment lines as code until it was taught
// them, which read freq as 424 (assessment-2026-10-20 §3.1). What a program
// means does not depend on its rendering: D gives the same core term from
// either notation. So the size counted here is the nodes of the core terms
// D gives every definition of every file under examples/ and lib/ that the
// revision's own core.Read reads; a target fragment's definitions are not
// counted.
//
// IT IS INVARIANT TO THE RENDERING, NOT TO ADR 0052: J writes a buffer in
// program order, and D's image of a place is not today's threaded term, which
// the state translation S makes later (+613 nodes, 6%, across the switch, with
// no program added). The after-S terms are not invariant either: S's
// translation is larger than the threaded source and reduces to the same
// residual. Between steps 5a and 5b (2a88c5d–98c3712) core.Read read today's
// notation while the corpus was J, so those revisions count nothing.
// balance.py copies this file into a worktree of the revision and runs it.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"oroboros/core"
)

func nodes(t *core.Term) int {
	if t == nil {
		return 0
	}
	n := 1
	for _, k := range t.Kids {
		n += nodes(k)
	}
	return n
}

func main() {
	total, files, unread := 0, 0, 0
	largest, lname := 0, ""
	for _, root := range []string{"examples", "lib"} {
		filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".oro") {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			forms, err := core.Read(string(b))
			if err != nil {
				unread++ // a fragment, or a program meant to be refused by its reader
				return nil
			}
			n := 0
			for _, f := range forms {
				if f.Kind == "def" {
					n += nodes(f.Term)
				}
			}
			total += n
			files++
			if root == "examples" && n > largest {
				largest, lname = n, filepath.ToSlash(p)
			}
			return nil
		})
	}
	fmt.Printf("nodes %d  files %d unread %d  largest %d %s\n", total, files, unread, largest, lname)
}
