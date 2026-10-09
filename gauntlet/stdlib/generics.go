//go:build ignore

// GO'S GENERIC API, MEASURED (docs/typevars-research.md §4).
//
//	go run gauntlet/stdlib/generics.go
//
// Every function, method and type of Go's standard library with type
// parameters, read from Go's own API manifest, $GOROOT/api/go1*.txt, where a
// type parameter is printed $0, $1, …: classified by where its variables occur
// (a name counts once per position) and by what bounds them.
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func main() {
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "go env GOROOT:", err)
		os.Exit(1)
	}
	root := strings.TrimSpace(string(out))
	files, _ := filepath.Glob(filepath.Join(root, "api", "go1*.txt"))
	seen := map[string]bool{}
	var lines []string
	for _, f := range files {
		fh, _ := os.Open(f)
		sc := bufio.NewScanner(fh)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			l := sc.Text()
			if i := strings.Index(l, " #"); i >= 0 {
				l = l[:i]
			}
			if strings.Contains(l, "$0") && !seen[l] && !strings.Contains(l, "internal") {
				seen[l] = true
				lines = append(lines, l)
			}
		}
		fh.Close()
	}
	sort.Strings(lines)
	funcRe := regexp.MustCompile(`^pkg ([^,]+)(?:, [^,]+)*, func ([A-Za-z0-9_]+)\[(.*?)\]\((.*)\)(.*)$`)
	methRe := regexp.MustCompile(`^pkg ([^,]+)(?:, [^,]+)*, method \(\*?([A-Za-z0-9_]+)\[[^\]]*\]\) ([A-Za-z0-9_]+)\((.*)\)(.*)$`)
	typeRe := regexp.MustCompile(`^pkg ([^,]+)(?:, [^,]+)*, type ([A-Za-z0-9_]+)\[(.*?)\] (.*)$`)
	pos := map[string]int{}
	bounds := map[string]int{}
	pkgs := map[string]int{}
	funcs, meths, types := 0, 0, map[string]bool{}
	needsCallback, noCallback, resultOnly := 0, 0, 0
	splitTop := func(s string) []string {
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
	classify := func(params, results string) {
		where := map[string]bool{}
		for _, p := range splitTop(params) {
			switch {
			case strings.Contains(p, "func(") && strings.Contains(p, "$"):
				where["in a callback's type"] = true
			case strings.HasPrefix(p, "...") && strings.Contains(p, "$"):
				where["a variadic list's element"] = true
			case strings.HasPrefix(p, "*$"):
				where["a pointer the host writes (*T)"] = true
			case strings.HasPrefix(p, "[]") && strings.Contains(p, "$") || strings.HasPrefix(p, "map[") && strings.Contains(p, "$"):
				where["a table's or map's element"] = true
			case strings.HasPrefix(p, "$"):
				where["an ordinary parameter"] = true
			case strings.Contains(p, "$"):
				where["a host type's argument"] = true
			}
		}
		if strings.Contains(results, "$") {
			if strings.Contains(results, "Seq") || strings.Contains(results, "func(") {
				where["the result, as an iterator or function"] = true
			} else {
				where["the result"] = true
			}
		}
		for w := range where {
			pos[w]++
		}
		if where["in a callback's type"] || where["the result, as an iterator or function"] {
			needsCallback++
		} else {
			noCallback++
			if !where["an ordinary parameter"] && !where["a table's or map's element"] && !where["a pointer the host writes (*T)"] && !where["a variadic list's element"] && !where["a host type's argument"] && where["the result"] {
				resultOnly++
			}
		}
	}
	for _, l := range lines {
		if m := funcRe.FindStringSubmatch(l); m != nil {
			funcs++
			pkgs[m[1]]++
			for _, tp := range splitTop(m[3]) {
				f := strings.Fields(tp)
				b := strings.Join(f[1:], " ")
				switch {
				case b == "interface{}":
					b = "none (any)"
				case strings.Contains(b, "~"):
					b = "a type set with ~ (an approximation)"
				}
				bounds[b]++
			}
			classify(m[4], m[5])
			continue
		}
		if m := methRe.FindStringSubmatch(l); m != nil {
			meths++
			pkgs[m[1]]++
			classify(m[4], m[5])
			continue
		}
		if m := typeRe.FindStringSubmatch(l); m != nil {
			types[m[1]+"."+m[2]] = true
		}
	}
	fmt.Printf("generic functions %d, methods of generic types %d, generic types %d\n\n", funcs, meths, len(types))
	fmt.Println("by package (functions and methods):")
	var ps []string
	for p := range pkgs {
		ps = append(ps, p)
	}
	sort.Slice(ps, func(i, j int) bool { return pkgs[ps[i]] > pkgs[ps[j]] })
	for _, p := range ps {
		fmt.Printf("  %-22s %3d\n", p, pkgs[p])
	}
	fmt.Println("\nwhere a type variable occurs (a name counts once per position):")
	var ws []string
	for w := range pos {
		ws = append(ws, w)
	}
	sort.Slice(ws, func(i, j int) bool { return pos[ws[i]] > pos[ws[j]] })
	for _, w := range ws {
		fmt.Printf("  %-40s %3d\n", w, pos[w])
	}
	fmt.Printf("\nwithout a callback or an iterator: %d; needing one: %d; the variable only in the result: %d\n", noCallback, needsCallback, resultOnly)
	fmt.Println("\nwhat bounds a function's type parameter:")
	var bs []string
	for b := range bounds {
		bs = append(bs, b)
	}
	sort.Slice(bs, func(i, j int) bool { return bounds[bs[i]] > bounds[bs[j]] })
	for _, b := range bs {
		fmt.Printf("  %-45s %3d\n", b, bounds[b])
	}
	fmt.Println("\ngeneric types:")
	var ts []string
	for t := range types {
		ts = append(ts, t)
	}
	sort.Strings(ts)
	fmt.Println("  " + strings.Join(ts, ", "))
}
