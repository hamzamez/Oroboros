//go:build ignore

// HOW HOST TEMPLATES WRITE THEIR HOLES (docs/surface-design.md §7.9).
//
//	go run gauntlet/surface/templates.go
//
// Over every `.oro` file in targets/, lib/, examples/ and the acceptance and
// differential programs, comments stripped: each `(host expr|stmt "…")`
// template, its holes by kind (`%s` in order, `%1` by position, `%r` and its
// views on x86, `%t1` a static argument, `%%` a literal), how many templates
// fill two or more holes by order, and how many contain `{` or `$`, the
// characters an antiquote could collide with.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	comment  = regexp.MustCompile(`;[^\n]*`)
	template = regexp.MustCompile(`\(host\s+(?:expr|stmt)\s+"((?:[^"\\]|\\.)*)"`)
	hole     = regexp.MustCompile(`%(s|t[0-9]|[0-9]|r|br|er|b[0-9]|e[0-9]|%)`)
)

func main() {
	c := map[string]int{}
	n, ordered, brace, dollar := 0, 0, 0, 0
	for _, dir := range []string{"targets", "lib", "examples", "gauntlet/stdlib/acceptance", "gauntlet/differential/cases"} {
		filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".oro") {
				return nil
			}
			b, _ := os.ReadFile(p)
			src := comment.ReplaceAllString(string(b), "")
			for _, m := range template.FindAllStringSubmatch(src, -1) {
				t := m[1]
				n++
				s := 0
				for _, h := range hole.FindAllStringSubmatch(t, -1) {
					c["%"+h[1]]++
					if h[1] == "s" {
						s++
					}
				}
				if s >= 2 {
					ordered++
				}
				if strings.Contains(t, "{") {
					brace++
				}
				if strings.Contains(t, "$") {
					dollar++
				}
			}
			return nil
		})
	}
	fmt.Printf("templates %d; filling two or more holes by order %d; containing { %d; containing $ %d\n",
		n, ordered, brace, dollar)
	var ks []string
	for k := range c {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool { return c[ks[i]] > c[ks[j]] })
	for _, k := range ks {
		fmt.Printf("%6d  %s\n", c[k], k)
	}
}
