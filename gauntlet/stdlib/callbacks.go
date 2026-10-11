//go:build ignore

// GO'S FUNCTION-TYPED API, MEASURED (docs/arrow-research.md §3).
//
//	go run gauntlet/stdlib/callbacks.go
//	go run gauntlet/stdlib/callbacks.go -list take|return|order2|named
//
// Every function and method of Go's standard library, read from Go's own API
// manifest, $GOROOT/api/go1*.txt, classified by where a function type occurs:
// a parameter (a callback the host calls: a function-space claim, Π(P).R, on
// a host parameter), a result (a function the host hands back: a value of a
// host type whose eliminator is application, not an arrow), or both. A named
// function type (`type SplitFunc func(…)`) is unfolded once to decide it. A
// callback of ORDER 2 takes a function itself (iter.Seq's yield), so its
// heading has a heading inside it. Packages under internal/ and vendor/ are
// excluded, as generics.go excludes them.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func splitTop(s string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if t := strings.TrimSpace(s[start:]); t != "" {
		out = append(out, t)
	}
	return out
}

// paramsResults splits "(params) results" at the parenthesis matching the first.
func paramsResults(s string) (string, string, bool) {
	if !strings.HasPrefix(s, "(") {
		return "", "", false
	}
	depth := 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[1:i], strings.TrimSpace(s[i+1:]), true
			}
		}
	}
	return "", "", false
}

func main() {
	list := flag.String("list", "", "print the members of one class: take, return, order2, named")
	flag.Parse()
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "go env GOROOT:", err)
		os.Exit(1)
	}
	root := strings.TrimSpace(string(out))
	files, _ := filepath.Glob(filepath.Join(root, "api", "go1*.txt"))
	// One line per declaration: the GOOS/GOARCH qualifier and the issue
	// number are dropped, so a name declared per platform counts once.
	qual := regexp.MustCompile(`^pkg ([^ ,]+)(?: \([^)]*\))?, `)
	seen := map[string]bool{}
	var lines []string
	for _, f := range files {
		fh, _ := os.Open(f)
		sc := bufio.NewScanner(fh)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			l := sc.Text()
			if !strings.HasPrefix(l, "pkg ") {
				continue
			}
			if i := strings.Index(l, " #"); i >= 0 {
				l = l[:i]
			}
			l = qual.ReplaceAllString(l, "pkg $1, ")
			if strings.Contains(l, "internal") || strings.HasPrefix(l, "pkg vendor/") || seen[l] {
				continue
			}
			seen[l] = true
			lines = append(lines, l)
		}
		fh.Close()
	}
	sort.Strings(lines)

	// Named function types, by package and by name, with their underlying func(…).
	typeRe := regexp.MustCompile(`^pkg ([^,]+), type ([A-Za-z0-9_]+)(\[[^\]]*\])? (func\(.*)$`)
	named := map[string]map[string]string{}
	var namedList []string
	for _, l := range lines {
		if m := typeRe.FindStringSubmatch(l); m != nil {
			if named[m[1]] == nil {
				named[m[1]] = map[string]string{}
			}
			named[m[1]][m[2]] = m[4]
			namedList = append(namedList, m[1]+"."+m[2])
		}
	}
	// underlying returns the func(…) type t denotes, if it is one, in package pkg.
	underlying := func(pkg, t string) (string, bool) {
		t = strings.TrimPrefix(strings.TrimPrefix(t, "..."), "*")
		if strings.HasPrefix(t, "func(") {
			return t, true
		}
		base := t
		if i := strings.Index(base, "["); i >= 0 {
			base = base[:i]
		}
		p, n := pkg, base
		if i := strings.LastIndex(base, "."); i >= 0 {
			p, n = base[:i], base[i+1:]
			for q := range named {
				if q == p || strings.HasSuffix(q, "/"+p) {
					p = q
					break
				}
			}
		}
		u, ok := named[p][n]
		return u, ok
	}
	// takesFunc reports whether a func(…) type has a function-typed parameter.
	takesFunc := func(pkg, fn string) bool {
		ps, _, ok := paramsResults(strings.TrimPrefix(fn, "func"))
		if !ok {
			return false
		}
		for _, p := range splitTop(ps) {
			if _, ok := underlying(pkg, p); ok {
				return true
			}
		}
		return false
	}

	declRe := regexp.MustCompile(`^pkg ([^,]+), (func|method \(\*?([A-Za-z0-9_]+)[^)]*\)) ([A-Za-z0-9_]+)(.*)$`)
	total := 0
	members := map[string][]string{}
	byPkg := map[string]int{}
	literal, onlyNamed, both, methodsOnNamed := 0, 0, 0, 0
	for _, l := range lines {
		m := declRe.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		total++
		pkg := m[1]
		if m[3] != "" && named[pkg][m[3]] != "" {
			methodsOnNamed++
		}
		// A type parameter list is skipped by its matching bracket, since a
		// bound may hold brackets itself: [$0 interface{ ~[]$1 }, $1 …].
		rest := m[5]
		if strings.HasPrefix(rest, "[") {
			depth := 0
			for i, r := range rest {
				if r == '[' {
					depth++
				} else if r == ']' {
					depth--
					if depth == 0 {
						rest = rest[i+1:]
						break
					}
				}
			}
		}
		ps, rs, ok := paramsResults(rest)
		if !ok {
			continue
		}
		name := pkg + "." + m[4]
		if m[3] != "" {
			name = pkg + "." + m[3] + "." + m[4]
		}
		take, lit, order2 := false, false, false
		for _, p := range splitTop(ps) {
			if u, ok := underlying(pkg, p); ok {
				take = true
				if strings.HasPrefix(strings.TrimPrefix(p, "..."), "func(") {
					lit = true
				}
				if takesFunc(pkg, u) {
					order2 = true
				}
			}
		}
		ret := false
		rlist := []string{rs}
		if strings.HasPrefix(rs, "(") {
			rlist = splitTop(rs[1 : len(rs)-1])
		}
		for _, r := range rlist {
			if r == "" {
				continue
			}
			if _, ok := underlying(pkg, r); ok {
				ret = true
			}
		}
		if take {
			byPkg[pkg]++
			members["take"] = append(members["take"], name)
			if lit {
				literal++
			} else {
				onlyNamed++
			}
		}
		if order2 {
			members["order2"] = append(members["order2"], name)
		}
		if ret {
			members["return"] = append(members["return"], name)
		}
		if take && ret {
			both++
		}
	}
	members["named"] = namedList
	if *list != "" {
		for _, n := range members[*list] {
			fmt.Println(n)
		}
		return
	}
	fmt.Printf("functions and methods                     %5d\n", total)
	fmt.Printf("take a function (a callback)              %5d  (%.1f%%)\n", len(members["take"]), 100*float64(len(members["take"]))/float64(total))
	fmt.Printf("  through a literal func type             %5d\n", literal)
	fmt.Printf("  only through a named func type          %5d\n", onlyNamed)
	fmt.Printf("  of order 2 (the callback takes one)     %5d\n", len(members["order2"]))
	fmt.Printf("return a function                         %5d\n", len(members["return"]))
	fmt.Printf("both                                      %5d\n", both)
	fmt.Printf("named func types                          %5d\n", len(namedList))
	fmt.Printf("methods declared on a named func type     %5d\n", methodsOnNamed)
	var ps []string
	for p := range byPkg {
		ps = append(ps, p)
	}
	sort.Slice(ps, func(i, j int) bool {
		if byPkg[ps[i]] != byPkg[ps[j]] {
			return byPkg[ps[i]] > byPkg[ps[j]]
		}
		return ps[i] < ps[j]
	})
	fmt.Println("\ncallbacks by package:")
	for _, p := range ps {
		fmt.Printf("  %-26s %3d\n", p, byPkg[p])
	}
	fmt.Println("\nnamed func types:\n  " + strings.Join(namedList, ", "))
}
