//go:build ignore

// HOW MUCH OF EACH GO PACKAGE WE HAVE A FILE FOR IS DECLARED, AND WHAT STOPS THE REST.
//
// The universe is Go's own API manifest, $GOROOT/api/go1*.txt: every exported
// function, method, type, variable and constant, as survey.go reads it. The
// declared set is every `sig`, `type` and `const` in targets/go, read with the
// language's own reader. A variadic family counts as declared when its
// restrictions are (Fprintf, Fprintf2, Fprintf3: fmt.oro's convention).
//
// A name that is not declared gets a CAUSE. Most are read off its signature
// (`func(` is a callback, `...` a variadic). But some walls do not show in a Go
// signature at all: a returned slice that aliases the host's buffer until the
// next call looks like any other []byte. So the table `causes` below records,
// name by name, what reading Go's documentation and source found
// (gocoverage-2026-10-06). A name it does not list, and whose signature shows
// nothing, is WORK: declarable today, not written.
//
//	go run gauntlet/stdlib/coverage.go              the table, by package and by cause
//	go run gauntlet/stdlib/coverage.go -list CAUSE  every name under a cause (a prefix: -list L)
//	go run gauntlet/stdlib/coverage.go -pkg os      one package's missing names, with their causes
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

	"oroboros/core"
)

// The packages targets/go has a file for. `math/big` is not one: bigint.oro
// declares the rung above the word, not the package.
var pkgs = []string{"bufio", "encoding/binary", "encoding/hex", "fmt", "io", "math/bits", "os",
	"strconv", "strings", "unicode/utf8"}

// The causes, each with what stops it. The letter orders the report.
var causeText = map[string]string{
	"A": "a callback: a function value handed to the host (tiers 1 and 2, specified, not built)",
	"B": "implementing a host interface: the host calls the program's methods (tier 3 by design)",
	"C": "a borrow that a later call ends (the slice aliases the host's buffer)",
	"D": "reflection over Go values: interface{} encoded by struct layout (records not built)",
	"E": "complex numbers (no complex type)",
	"F": "raw handles, uintptr (platform-specific)",
	"G": "variadic over one type: CLOSABLE as Go's spread, f(xs...) over a table",
	"H": "variadic over any: CLOSABLE as restrictions f|A^n",
	"I": "out-parameters: CLOSABLE as templates that own the pointer",
	"J": "scratch the host may overwrite (a write-borrow)",
	"K": "a type from a package with no file (time, io/fs, unicode, syscall)",
	"L": "work: declarable today, not written",
}

// The names whose cause their signature does not show, read one at a time.
var causes = map[string]string{}

func put(c, names string) {
	for _, n := range strings.Fields(names) {
		causes[n] = c
	}
}

func init() {
	put("A", `strings:ContainsFunc strings:FieldsFunc strings:FieldsFuncSeq strings:IndexFunc
		strings:LastIndexFunc strings:Map strings:TrimFunc strings:TrimLeftFunc strings:TrimRightFunc
		os:Expand os:Process.WithHandle bufio:Scanner.Split bufio:SplitFunc bufio:ScanBytes
		bufio:ScanLines bufio:ScanRunes bufio:ScanWords strings:Lines strings:SplitSeq
		strings:SplitAfterSeq strings:FieldsSeq`)
	put("B", `fmt:Formatter fmt:GoStringer fmt:Scanner fmt:ScanState fmt:State fmt:FormatString`)
	put("C", `bufio:Scanner.Bytes bufio:Reader.Peek bufio:Reader.ReadSlice bufio:Reader.ReadLine
		bufio:ReadWriter.Peek bufio:ReadWriter.ReadSlice bufio:ReadWriter.ReadLine
		bufio:Writer.AvailableBuffer bufio:ReadWriter.AvailableBuffer`)
	put("D", `encoding/binary:Read encoding/binary:Write encoding/binary:Size encoding/binary:Append
		encoding/binary:Encode encoding/binary:Decode os:ProcessState.Sys os:ProcessState.SysUsage`)
	put("E", `strconv:FormatComplex strconv:ParseComplex`)
	put("F", `os:File.Fd os:NewFile`)
	put("G", `strings:NewReplacer strings:Replacer strings:Replacer.Replace strings:Replacer.WriteString
		io:MultiReader io:MultiWriter`)
	put("H", `fmt:Append fmt:Appendf fmt:Appendln`)
	put("I", `fmt:Fscan fmt:Fscanf fmt:Fscanln fmt:Scan fmt:Scanf fmt:Scanln fmt:Sscan fmt:Sscanf fmt:Sscanln`)
	put("J", `bufio:Scanner.Buffer io:CopyBuffer`)
	put("K", `strings:ToLowerSpecial strings:ToUpperSpecial strings:ToTitleSpecial os:DirEntry os:ModeIrregular`)
}

type name struct {
	pkg, key, kind, text string
	deprecated           bool
}

func main() {
	list := flag.String("list", "", "print every missing name whose cause starts with this letter")
	onePkg := flag.String("pkg", "", "print one package's missing names, with their causes")
	flag.Parse()

	goroot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		fail(err)
	}
	api := manifest(strings.TrimSpace(string(goroot)))
	have := declared("targets/go")

	type row struct {
		n     name
		cause string // "" when declared
	}
	var rows []row
	for _, n := range api {
		if n.deprecated {
			continue
		}
		if isDeclared(have, n) {
			rows = append(rows, row{n, ""})
			continue
		}
		rows = append(rows, row{n, causeOf(n)})
	}

	switch {
	case *list != "":
		for _, r := range rows {
			if r.cause != "" && strings.HasPrefix(r.cause, *list) {
				fmt.Printf("%s  %-16s %-34s %s\n", r.cause, r.n.pkg, r.n.key, trim(r.n.text, 100))
			}
		}
		return
	case *onePkg != "":
		for _, r := range rows {
			if r.n.pkg == *onePkg && r.cause != "" {
				fmt.Printf("%s  %-34s %s\n", r.cause, r.n.key, trim(r.n.text, 100))
			}
		}
		return
	}

	fmt.Printf("%-16s %6s %9s %6s\n", "package", "names", "declared", "")
	all, dec := 0, 0
	for _, p := range pkgs {
		n, d := 0, 0
		for _, r := range rows {
			if r.n.pkg == p {
				n++
				if r.cause == "" {
					d++
				}
			}
		}
		all += n
		dec += d
		fmt.Printf("%-16s %6d %9d %5.0f%%\n", p, n, d, 100*float64(d)/float64(n))
	}
	fmt.Printf("%-16s %6d %9d %5.1f%%\n\nthe missing %d, by cause:\n", "all", all, dec, 100*float64(dec)/float64(all), all-dec)
	count := map[string]int{}
	for _, r := range rows {
		if r.cause != "" {
			count[r.cause]++
		}
	}
	var cs []string
	for c := range count {
		cs = append(cs, c)
	}
	sort.Strings(cs)
	for _, c := range cs {
		fmt.Printf("  %s %4d  %s\n", c, count[c], causeText[c])
	}
	declarable := dec + count["L"] + count["K"]
	closable := declarable + count["G"] + count["H"] + count["I"]
	fmt.Printf("\ndeclarable today, with the packages K needs: %d of %d, %.1f%%\n", declarable, all, 100*float64(declarable)/float64(all))
	fmt.Printf("and with the closable G, H and I:            %d of %d, %.1f%%\n", closable, all, 100*float64(closable)/float64(all))
}

func causeOf(n name) string {
	if c, ok := causes[n.pkg+":"+n.key]; ok {
		return c
	}
	switch {
	case n.kind == "var" || n.kind == "const" || n.kind == "type":
		return "L"
	case strings.Contains(n.text, "func("):
		return "A"
	case strings.Contains(n.text, "..."):
		return "H"
	case regexp.MustCompile(`\b(time|fs|syscall|iter|unicode)\.`).MatchString(n.text):
		return "K"
	}
	return "L"
}

var (
	reLine   = regexp.MustCompile(`^pkg ([^ ,]+), (.*)$`)
	reFunc   = regexp.MustCompile(`^func (\w+)`)
	reMethod = regexp.MustCompile(`^method \(\*?(\w+)\) (\w+)`)
	reDecl   = regexp.MustCompile(`^(type|var|const) (\w+)`)
	reMember = regexp.MustCompile(`^type \w+ (struct|interface), `)
)

// manifest reads every exported name of the packages, once each, from the
// release files. A platform-specific line ("pkg os (linux-386), …") is not
// matched; a name marked //deprecated in any file is deprecated.
func manifest(goroot string) []name {
	in := map[string]bool{}
	for _, p := range pkgs {
		in[p] = true
	}
	files, _ := filepath.Glob(filepath.Join(goroot, "api", "go1*.txt"))
	sort.Strings(files)
	byKey := map[string]*name{}
	var order []string
	for _, f := range files {
		fh, err := os.Open(f)
		if err != nil {
			fail(err)
		}
		sc := bufio.NewScanner(fh)
		for sc.Scan() {
			m := reLine.FindStringSubmatch(sc.Text())
			if m == nil || !in[m[1]] {
				continue
			}
			pkg, rest := m[1], m[2]
			dep := strings.HasSuffix(rest, "//deprecated")
			rest = strings.TrimSuffix(rest, " //deprecated")
			if reMember.MatchString(rest) {
				continue // a struct field or an interface method, not counted here
			}
			var key, kind string
			if mm := reFunc.FindStringSubmatch(rest); mm != nil {
				key, kind = mm[1], "func"
			} else if mm := reMethod.FindStringSubmatch(rest); mm != nil {
				key, kind = mm[1]+"."+mm[2], "method"
			} else if mm := reDecl.FindStringSubmatch(rest); mm != nil {
				key, kind = mm[2], mm[1]
			} else {
				continue
			}
			k := pkg + ":" + key
			if n, ok := byKey[k]; ok {
				n.deprecated = n.deprecated || dep
				continue
			}
			byKey[k] = &name{pkg, key, kind, rest, dep}
			order = append(order, k)
		}
		fh.Close()
	}
	out := make([]name, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	return out
}

// declared is every (module path, name) a `sig`, `type` or `const` in the
// target's files introduces. A child module's path is its parent's followed
// by its own (target-files.md §1a).
func declared(dir string) map[string]map[string]bool {
	have := map[string]map[string]bool{}
	var walk func(t *core.Term, path string)
	walk = func(t *core.Term, path string) {
		if t == nil || t.Kind != core.KApp || len(t.Kids) == 0 {
			return
		}
		head := t.Kids[0]
		if head.Kind == core.KName && len(t.Kids) > 1 && t.Kids[1].Kind == core.KName {
			switch head.Name {
			case "module":
				p := t.Kids[1].Name
				if strings.HasPrefix(p, "go") || path == "" {
					path = p
				} else {
					path = path + "/" + p
				}
			case "sig", "type", "const":
				if have[path] == nil {
					have[path] = map[string]bool{}
				}
				have[path][t.Kids[1].Name] = true
			}
		}
		for _, k := range t.Kids {
			walk(k, path)
		}
	}
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".oro") {
			return err
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		forms, err := core.ReadRaw(string(src))
		if err != nil {
			fail(fmt.Errorf("%s: %v", p, err))
		}
		for _, f := range forms {
			walk(f, "")
		}
		return nil
	})
	return have
}

func isDeclared(have map[string]map[string]bool, n name) bool {
	path, member := "go/"+n.pkg, n.key
	if i := strings.IndexByte(n.key, '.'); i >= 0 {
		path, member = path+"/"+n.key[:i], n.key[i+1:]
	}
	family := regexp.MustCompile(`^` + regexp.QuoteMeta(member) + `[0-9]$`)
	for d := range have[path] {
		if d == member || family.MatchString(d) {
			return true
		}
	}
	return false
}

func trim(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
