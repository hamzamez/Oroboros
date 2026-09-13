// Tests for the survey tooling — assessment-2026-09-11 item 2.
//
// Four programs in this directory publish numbers that CLAUDE.md quotes as
// findings, and until this file none of them had a test. Twelve corrections to
// those numbers were found by reading and by the next question; the assessment
// counted four of them that a test would have caught before they were published.
//
// Three properties, each one a thing a published number has already been wrong
// about:
//
//	A SURVEY IS A FUNCTION OF ITS INPUT. Run twice, the report and every emitted
//	file are byte-identical. Two emits of the Go generator once differed in five
//	files (structlit-2026-09-10), and on its first run this test found the
//	REPORTS still differing: every ranked list sorted ties by map order.
//
//	EVERY NAME COUNTED DECLARABLE IS EMITTED, and the report's count of emitted
//	primitives is the count in the files. "A name counted declarable and never
//	emitted is a claim" (win32-2026-09-08) was found by hand three times — the
//	Go generator's 1,007 against 4,331, Win32's voids, Win32's stale arity cap.
//	This is that sentence as arithmetic, and it needs no pinned number.
//
//	THE PUBLISHED COUNTS ARE THE COUNTS. Each figure a result document or
//	CLAUDE.md quotes is pinned against the host version it was measured on, so a
//	change is a failing test rather than the next question. On its first run
//	this found two JVM figures that no version of the committed tool produces.
//
// And a fourth, which is the reason the first three matter: THE ACCEPTANCE
// PROGRAMS RUN. A percentage that does not build is a claim, and a witness that
// cannot fail proves nothing — so these are the twelve programs in acceptance/,
// built from THIS run's generated declarations and checked against the host.
//
// Slow — every survey runs twice, and twelve programs go through four toolchains —
// so `-short` skips all of it. A host whose toolchain is absent is skipped BY
// NAME; nothing is skipped for any other reason.
package stdlib

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

var root = func() string {
	abs, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		panic(err)
	}
	return abs
}()

// scratch is shared by every test, because a survey is run once per host and
// read by all four properties.
var scratch string

func TestMain(m *testing.M) {
	var err error
	if scratch, err = os.MkdirTemp("", "oro-tooling-"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(scratch)
	os.Exit(code)
}

// A host is a tool, optionally a dumper that writes its manifest, and the
// executable either needs.
type host struct {
	tool  string   // gauntlet/stdlib/<tool>.go
	dump  []string // the command that writes the manifest, or nil
	needs string   // an executable the host's toolchain must provide
}

var hosts = map[string]host{
	"go":    {tool: "survey"},
	"win32": {tool: "win32"},
	"jvm":   {tool: "jvm", dump: []string{"java", "gauntlet/stdlib/jdk/Dump.java"}, needs: "java"},
	"js":    {tool: "js", dump: []string{"node", "gauntlet/stdlib/jsdump.mjs"}, needs: "node"},
}

type run struct {
	manifest []byte // nil for a host whose manifest is on disk already
	report   string // with the emit directory replaced by DIR
	emit     string
}

type survey struct {
	runs [2]run
	skip string
	err  error
}

var (
	surveyMu sync.Mutex
	surveys  = map[string]*survey{}
	once     = map[string]*sync.Once{}
)

// surveyOf runs a host's survey TWICE, from the manifest up, and remembers it.
func surveyOf(t *testing.T, name string) *survey {
	t.Helper()
	if testing.Short() {
		t.Skip("the surveys are slow; -short skips them")
	}
	surveyMu.Lock()
	o, ok := once[name]
	if !ok {
		o = &sync.Once{}
		once[name] = o
		surveys[name] = &survey{}
	}
	s := surveys[name]
	surveyMu.Unlock()
	o.Do(func() { s.runs, s.skip, s.err = runSurvey(name) })
	if s.skip != "" {
		t.Skip(s.skip)
	}
	if s.err != nil {
		t.Fatal(s.err)
	}
	return s
}

func runSurvey(name string) (runs [2]run, skip string, err error) {
	h := hosts[name]
	if h.needs != "" {
		if _, err := exec.LookPath(h.needs); err != nil {
			return runs, h.needs + " is not on PATH", nil
		}
	}
	for i := range runs {
		dir := filepath.Join(scratch, fmt.Sprintf("%s-%d", name, i))
		args := []string{"run", "gauntlet/stdlib/" + h.tool + ".go"}
		if h.dump != nil {
			man, err := command(root, h.dump...)
			if err != nil {
				return runs, "", err
			}
			runs[i].manifest = man
			path := filepath.Join(scratch, fmt.Sprintf("%s-%d.api", name, i))
			if err := os.WriteFile(path, man, 0o644); err != nil {
				return runs, "", err
			}
			args = append(args, "-api", path)
		}
		out, err := command(root, append(append([]string{"go"}, args...), "-emit", dir)...)
		if err != nil {
			if name == "win32" && strings.Contains(err.Error(), "no Windows SDK found") {
				return runs, "no Windows SDK is installed", nil
			}
			return runs, "", err
		}
		runs[i].report = strings.ReplaceAll(string(out), dir, "DIR")
		runs[i].emit = dir
	}
	return runs, "", nil
}

// command runs argv in dir and returns its standard output. Standard error is
// kept apart because node writes deprecation warnings there, which are the
// host's business and not the manifest's.
func command(dir string, argv ...string) ([]byte, error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %v\n%s%s", strings.Join(argv, " "), err, out, stderr.String())
	}
	return out, nil
}

func hostNames() []string {
	var ns []string
	for n := range hosts {
		ns = append(ns, n)
	}
	sort.Strings(ns)
	return ns
}

// ---------------------------------------------------------------------------
// 1. A survey is a function of its input.

func TestSurveysAreFunctionsOfTheirInput(t *testing.T) {
	for _, name := range hostNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := surveyOf(t, name)
			a, b := s.runs[0], s.runs[1]
			if a.manifest != nil && !bytes.Equal(a.manifest, b.manifest) {
				t.Errorf("the dumper wrote two different manifests from one host")
			}
			if a.report != b.report {
				t.Errorf("two runs printed different reports:\n%s", firstDiff(a.report, b.report))
			}
			fa, fb := tree(t, a.emit), tree(t, b.emit)
			for p := range union(fa, fb) {
				if fa[p] != fb[p] {
					t.Errorf("two runs emitted different %s", p)
				}
			}
		})
	}
}

func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	m := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		m[rel] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func union(a, b map[string]string) map[string]bool {
	u := map[string]bool{}
	for k := range a {
		u[k] = true
	}
	for k := range b {
		u[k] = true
	}
	return u
}

func firstDiff(a, b string) string {
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(la) || i < len(lb); i++ {
		var x, y string
		if i < len(la) {
			x = la[i]
		}
		if i < len(lb) {
			y = lb[i]
		}
		if x != y {
			return fmt.Sprintf("line %d:\n  first:  %q\n  second: %q", i+1, x, y)
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// 2. Every name counted declarable is emitted.

var primLine = regexp.MustCompile(`(?m)^\s*\(prim `)

// emittedInFiles counts the primitives actually written, which is the number
// the report has to agree with — the files are what a program loads.
func emittedInFiles(t *testing.T, dir string) int {
	n := 0
	for p, body := range tree(t, dir) {
		if strings.HasSuffix(p, ".oro") {
			n += len(primLine.FindAllStringIndex(body, -1))
		}
	}
	return n
}

// num reads the first integer a pattern captures, and fails loudly when the
// report no longer says it: a reworded report must reword this test too.
func num(t *testing.T, report, pattern string) int {
	t.Helper()
	m := regexp.MustCompile(pattern).FindStringSubmatch(report)
	if m == nil {
		t.Fatalf("the report no longer matches %q", pattern)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestEveryDeclarableNameIsEmitted(t *testing.T) {
	const declared = `declarable as a \(prim …\):\s+(\d+)`
	for _, name := range hostNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := surveyOf(t, name)
			r := s.runs[0].report
			emitted := num(t, r, `emitted (\d+) primitives`)
			if inFiles := emittedInFiles(t, s.runs[0].emit); inFiles != emitted {
				t.Errorf("the report says %d primitives were emitted and the files hold %d", emitted, inFiles)
			}
			// What each host's arithmetic is, stated rather than inferred: a
			// declarable name with no template must be COUNTED as having none,
			// and the Go generator adds its own constructors, which are ours
			// and not the host's surface.
			var want int
			var why string
			switch name {
			case "go":
				decl := num(t, r, declared)
				voids := num(t, r, `\((\d+) void with no argument\)`)
				ctors := num(t, r, `(\d+) get a generated constructor`)
				consts := num(t, r, `CONSTANTS: \d+ exported, (\d+) declarable`)
				want, why = decl-voids+ctors+consts, fmt.Sprintf("%d declarable - %d voids + %d constructors + %d constants",
					decl, voids, ctors, consts)
			case "win32":
				want, why = num(t, r, declared), "declarable"
			case "jvm":
				decl := num(t, r, declared)
				voids := num(t, r, `(\d+) declarable names have no template`)
				want, why = decl-voids, fmt.Sprintf("%d declarable - %d voids", decl, voids)
			case "js":
				want, why = num(t, r, `DECLARABLE: (\d+) of`), "declarable"
			}
			if emitted != want {
				t.Errorf("emitted %d, and %s is %d: a name counted declarable and never emitted is a claim",
					emitted, why, want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 3. The published counts are the counts.

// The version each host was measured on. A manifest is the host's own account
// of itself, so a new toolchain is a new manifest and every number below has to
// be re-measured — and re-published, which is the part that has been skipped
// before. A mismatch FAILS rather than skipping, because a test that quietly
// stops running after an upgrade is a witness that cannot fail.
var measuredOn = map[string][2]string{
	"go":    {"go env GOVERSION", "go1.27.0"},
	"win32": {"", "WINDOWS SDK 10.0.26100.0"}, // the report's own first line
	"jvm":   {"java -version", `"17.0.12"`},
	"js":    {"node --version", "v26.7.0"},
}

// Each line is one a result document or CLAUDE.md quotes, with the document
// named. Runs of spaces are collapsed on both sides, so a column moving is not a
// finding and a number moving is.
var published = map[string][]string{
	"go": { // gomethods, interfaces.md §5, coercion, structlit, surveys-2026-09-10
		"CALLABLE SURFACE (func + method): 4932",
		"declarable as a (prim …): 4331 87.8%",
		"usable by a program: 2978 60.4%",
		"method 3098 total 2862 declarable (92.4%) 1670 usable (53.9%)",
		"HOST TYPES A PROGRAM CAN OBTAIN: 568",
		"1651 candidate edges, 1466 accepted by the host",
		"702 struct types; 462 get a generated constructor",
		"238 have no exported field and get NOTHING",
		"52 types have BOTH",
		"USABLE GOES 2710 -> 2978 (+268)",
		// gostd-utf8-2026-09-13. A constant is a zero-argument pure prim whose
		// result is its own exact range, and these are what refuses the rest.
		"CONSTANTS: 2989 exported, 509 declarable",
		"constant of a named type 2425",
		"integer constant whose value is per-platform 50",
		"integer constant outside the portable window 5",
		"emitted 5282 primitives",
	},
	"win32": { // win32-2026-09-08, win32enum-2026-09-11, structval-2026-09-12
		"FLAT C API: 11575",
		"declarable as a (prim …): 10479 90.5%",
		"callable by a program: 4093 35.4%",
		"and the call says it all: 3756 32.4%",
		"(widest is 17 arguments)",
		"ENUMS: 10109 enum type names; 6031 of them",
		"struct by value 140 1.2%",
		"floating point 29 0.3%",
		"emitted 10479 primitives",
		// structval-2026-09-12. The bucket that was the largest refusal at
		// 945, and what the ABI does with what is left of it.
		"STRUCT BY VALUE: 24 aggregate types across 140 refusals",
		"1/2/4/8 bytes — one register 12 77 7",
		"other — by reference / RCX 12 55 1",
		// And the ceiling nobody had measured: a callable name in a library
		// the link line does not name is a claim. linkline-2026-09-13 computes
		// the line from the declarations, and these are what that buys and
		// what it deliberately refuses.
		"by a library the target always links: 666 16.3%",
		"bound to ONE DLL by every library: 2927 71.5%",
		"refused: bound to two or more DLLs: 155 3.8%",
		"refused: only through an API set: 61 1.5%",
		"refused: listed and bound to no DLL: 17 0.4%",
		"in no import library at all: 267 6.5%",
		"CALLABLE AND IT LINKS: 3593 31.0% of the flat API",
	},
	"jvm": { // surveys-2026-09-10, corrected by tooling-2026-09-11
		"4508 public types",
		"286 functional interfaces",
		"CALLABLE SURFACE (func + method + ctor): 38042",
		"declarable as a (prim …): 30959 81.4%",
		"usable by a program: 23170 60.9%",
		"java.lang.Object 1891 5.0%",
		"type not exported 958 2.5%",
		"3026 names mention one (8.0% of the surface)",
		"1872 are a member of a GENERIC CLASS",
		"1154 are a variable in the member's OWN signature",
		"emitted 30940 primitives",
		"4948 of them are OVERLOADS",
		"19 declarable names have no template",
		"10095 subsumption edges over 3225 subtypes",
	},
	"js": { // surveys-2026-09-10 as corrected 2026-09-11
		"DECLARABLE: 2326 of 2340 callable",
		"10 have a key the HOST accepts and OUR READER does not",
		"604 of 2326 callable members report arity ZERO (26.0%)",
		"emitted 2326 primitives",
	},
}

var spaces = regexp.MustCompile(`[ \t]+`)

func TestPublishedCountsAreTheCounts(t *testing.T) {
	for _, name := range hostNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := surveyOf(t, name)
			report := spaces.ReplaceAllString(s.runs[0].report, " ")
			v := measuredOn[name]
			got := report
			if v[0] != "" {
				argv := strings.Fields(v[0])
				cmd := exec.Command(argv[0], argv[1:]...)
				out, _ := cmd.CombinedOutput()
				got = string(out)
			}
			if !strings.Contains(got, v[1]) {
				t.Fatalf("the published %s counts were measured on %s and this host is not it — re-measure, "+
					"re-pin, and correct every document that quotes them:\n%s", name, v[1], got)
			}
			for _, line := range published[name] {
				if !strings.Contains(report, line) {
					t.Errorf("published %q, and the %s survey no longer says it", line, name)
				}
			}
			// A PIN SAYS A NUMBER HAS NOT MOVED, NOT THAT IT WAS RIGHT, and
			// the ABI-class rows read from a committed size table. A new SDK
			// can add a blocking aggregate the table does not cover, and the
			// report would then say "size not measured" — honest, and useless
			// as a finding. So the absence of that row is pinned too.
			if name == "win32" && strings.Contains(report, "size not measured") {
				t.Errorf("an aggregate blocks an entry point and has no measured size; " +
					"regenerate with `go run gauntlet/stdlib/win32.go -sizes " +
					"gauntlet/stdlib/win32-sizes.txt`")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 4. The acceptance programs run.

// A program, the generated files it needs, and what the host must print.
type accept struct {
	host   string            // the survey that generates its declarations
	target string            // the cmd/build target
	layer  string            // where in the project the generated files go
	files  map[string]string // project-relative path -> generated file
	flags  []string
	want   []string
	// An APPLICATION rather than a one-file witness: its sources (repo-relative,
	// the entry first), its command line ("{proj}" is the project directory),
	// and any input files it reads. Empty for the twelve acceptance programs.
	srcs   []string
	args   []string
	inputs map[string]string
}

// tallyReference is what `tally PATTERN FILE` must print, computed by
// HAND-WRITTEN Go rather than by anything this repository compiles: capture group
// 1 (or the whole match) of every matching line, counted, most frequent first,
// ties in ascending byte order. A group that takes no part is "" — Go's answer,
// and the one the JVM binding had to be taught (tally-2026-09-11).
func tallyReference(text, pattern string) []string {
	re := regexp.MustCompile(pattern)
	count := map[string]int{}
	for _, line := range strings.Split(text, "\n") {
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		v := m[0]
		if len(m) > 1 {
			v = m[1]
		}
		count[v]++
	}
	var vs []string
	for v := range count {
		vs = append(vs, v)
	}
	sort.Slice(vs, func(i, j int) bool { return byCountThenValue(count, vs[i], vs[j]) })
	var out []string
	for _, v := range vs {
		out = append(out, fmt.Sprintf("%d\t%s", count[v], v))
	}
	return out
}

func byCountThenValue(count map[string]int, a, b string) bool {
	if count[a] != count[b] {
		return count[a] > count[b]
	}
	return a < b
}

func acceptance() map[string]accept {
	gomod, err := os.Stat(filepath.Join(root, "go.mod"))
	size := "?"
	if err == nil {
		size = strconv.FormatInt(gomod.Size(), 10)
	}
	checked := []string{"-checked"}
	// THE APPLICATION: examples/tally, one core over six host operations, bound on
	// each host by generated declarations — assessment-2026-09-11 item 3. Two
	// inputs: a sample access log, and the one place the hosts DIVERGED, an
	// optional group that takes no part (Go "", the JVM null).
	logText := ""
	if b, err := os.ReadFile(filepath.Join(root, "examples", "tally", "access.log")); err == nil {
		logText = string(b)
	}
	const logPat = `"[A-Z]+ ([^ ?]*)`
	const optText, optPat = "ab\nb\n", `(a)?b`
	tallyOn := func(host, target, entry string, files map[string]string) []accept {
		srcs := []string{"examples/tally/" + entry, "examples/tally/tally.oro"}
		return []accept{
			{host: host, target: target, layer: "tg", files: files, srcs: srcs,
				args: []string{logPat, "examples/tally/access.log"}, want: tallyReference(logText, logPat)},
			{host: host, target: target, layer: "tg", files: files, srcs: srcs,
				inputs: map[string]string{"opt.txt": optText},
				args:   []string{optPat, "{proj}/opt.txt"}, want: tallyReference(optText, optPat)},
		}
	}
	goTally := tallyOn("go", "go", "tally-go.oro", map[string]string{
		"tg/go/regexp-gen.oro": "regexp.oro", "tg/go/strings-gen.oro": "strings.oro",
		"tg/go/strconv-gen.oro": "strconv.oro"})
	jvmTally := tallyOn("jvm", "java", "tally-java.oro", map[string]string{
		"tg/java/regex-gen.oro": "java-util-regex.oro", "tg/java/lang-gen.oro": "java-lang.oro"})
	return map[string]accept{
		"tally-go":            goTally[0],
		"tally-go-optional":   goTally[1],
		"tally-java":          jvmTally[0],
		"tally-java-optional": jvmTally[1],
		// Win32: a seven-argument call with a NULL, a void statement, a SIGNED
		// result — the one that could fail and did (win32enum-2026-09-11) — and
		// an enum whose two values must give two different errors.
		"wide-call": {host: "win32", target: "windows", layer: ".", flags: checked, want: []string{"2"},
			files: map[string]string{"windows/fileapi.oro": "fileapi.oro", "windows/errhandlingapi.oro": "errhandlingapi.oro"}},
		"void-stmt": {host: "win32", target: "windows", layer: ".", want: []string{"1"},
			files: map[string]string{"windows/fileapi.oro": "fileapi.oro", "windows/errhandlingapi.oro": "errhandlingapi.oro"}},
		"signed-result": {host: "win32", target: "windows", layer: ".", flags: checked, want: []string{"1", "79"},
			files: map[string]string{"windows/WinBase.oro": "WinBase.oro"}},
		"enum-param": {host: "win32", target: "windows", layer: ".", flags: checked, want: []string{"0", "24", "87"},
			files: map[string]string{"windows/WinBase.oro": "WinBase.oro"}},
		// And a parameter written `TYPE *name`, which the survey read as a
		// value by value: `IsBadReadPtr(NULL, 1)` is TRUE and
		// `IsBadReadPtr(NULL, 0)` is FALSE, so one declaration gives two
		// answers and the program cannot be right by accident
		// (structval-2026-09-12).
		"pointer-param": {host: "win32", target: "windows", layer: ".", flags: checked, want: []string{"1", "0"},
			files: map[string]string{"windows/WinBase.oro": "WinBase.oro"}},
		// And two libraries the target does not link by default, reached only
		// because a generated declaration says `(lib "…")` and the link line is
		// computed from what the program calls (linkline-2026-09-13). Against
		// the old constant line this does not build at all.
		"link-line": {host: "win32", target: "windows", layer: ".", flags: checked, want: []string{"1", "0", "12", "28"},
			files: map[string]string{"windows/WinUser.oro": "WinUser.oro", "windows/securitybaseapi.oro": "securitybaseapi.oro"}},
		// A WHOLE PACKAGE, unicode/utf8: every function and every constant,
		// each called with a value the program computed rather than a literal
		// (gostd-utf8-2026-09-13). The expected lines are computed by the real
		// package, not copied from a run.
		"unicode-utf8": {host: "go", target: "go", layer: "tg", flags: checked, want: utf8Reference(),
			files: map[string]string{"tg/go/utf8-gen.oro": "unicode-utf8.oro"}},
		// Go: a method, a coercion to an interface, and a nested struct literal.
		// The generated files go under tg/ and under a name that is not the
		// module's, because the source's directory is also the LIBRARY path —
		// and os-methods' own recipe put its file at go/os.oro, where `(use
		// go/os)` finds it as a module and reads it with the wrong grammar. The
		// README said so for io-reader, and the recipe beside it had not been run
		// since; this test is the first thing that ran it.
		"os-methods": {host: "go", target: "go", layer: "tg", want: []string{"64"},
			files: map[string]string{"tg/go/os-gen.oro": "os.oro"}},
		"io-reader": {host: "go", target: "go", layer: "tg", want: []string{size},
			files: map[string]string{"tg/go/os-gen.oro": "os.oro", "tg/go/io-gen.oro": "io.oro"}},
		"struct-literal": {host: "go", target: "go", layer: "tg", want: []string{"8", "4"},
			files: map[string]string{"tg/go/image-gen.oro": "image.oro"}},
		// The JVM: 30 is what `new java.util.Random(42).nextInt(100)` prints —
		// the host's answer, not ours.
		"jvm-object": {host: "jvm", target: "java", layer: "tg", want: []string{"a, b, c", "30"},
			files: map[string]string{"tg/java/java-util-gen.oro": "java-util.oro", "tg/java/java-lang-gen.oro": "java-lang.oro"}},
		// JavaScript: the single dot is `path.join()`, all the generated
		// declaration can call because the host reported arity 0.
		"js-shape": {host: "js", target: "js", layer: "tg", want: []string{"7", "ABC", "."},
			files: map[string]string{"tg/js/global-gen.oro": "globalThis.oro", "tg/js/path-gen.oro": "node-path.oro"}},
	}
}

// What each target's artifact is called and how it runs — the differential
// suite's table, for the same reasons: Windows will not exec a file without
// `.exe`, node will not load one without `.mjs`, and javac writes a directory.
var artifact = map[string]struct {
	name string
	run  func(o string) []string
}{
	"go":      {"out.exe", func(o string) []string { return []string{o} }},
	"windows": {"out.exe", func(o string) []string { return []string{o} }},
	"js":      {"out.mjs", func(o string) []string { return []string{"node", o} }},
	"java":    {"classes", func(o string) []string { return []string{"java", "-cp", o, "Main"} }},
}

var (
	buildOnce sync.Once
	buildBin  string
	buildErr  error
)

func oroBuild(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		buildBin = filepath.Join(scratch, "oro-build.exe")
		_, buildErr = command(root, "go", "build", "-o", buildBin, "./cmd/build")
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return buildBin
}

func TestAcceptanceProgramsRun(t *testing.T) {
	progs := acceptance()
	var names []string
	for n := range progs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		a := progs[name]
		t.Run(name, func(t *testing.T) {
			if a.target == "windows" && runtime.GOOS != "windows" {
				t.Skip("the windows target builds and runs only on Windows")
			}
			s := surveyOf(t, a.host)
			bin := oroBuild(t)
			proj := t.TempDir()
			for dst, src := range a.files {
				b, err := os.ReadFile(filepath.Join(s.runs[0].emit, src))
				if err != nil {
					t.Fatalf("the %s survey did not generate %s: %v", a.host, src, err)
				}
				p := filepath.Join(proj, filepath.FromSlash(dst))
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, b, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			srcs := a.srcs
			if len(srcs) == 0 {
				srcs = []string{"gauntlet/stdlib/acceptance/" + name + ".oro"}
			}
			for _, s := range srcs {
				b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(s)))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(proj, filepath.Base(s)), b, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			for n, body := range a.inputs {
				if err := os.WriteFile(filepath.Join(proj, n), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			src := filepath.Join(proj, filepath.Base(srcs[0]))
			layers := filepath.Join(proj, a.layer) + string(os.PathListSeparator) + filepath.Join(root, "targets")
			out := filepath.Join(proj, artifact[a.target].name)
			argv := append([]string{bin}, a.flags...)
			argv = append(argv, "-target="+a.target, "-targets", layers, "-o", out, src)
			if _, err := command(root, argv...); err != nil {
				t.Fatalf("build: %v", err)
			}
			// Run from the repository root: os-methods and io-reader read go.mod.
			cmdline := artifact[a.target].run(out)
			for _, arg := range a.args {
				cmdline = append(cmdline, strings.ReplaceAll(arg, "{proj}", proj))
			}
			got, err := command(root, cmdline...)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			lines := strings.Split(strings.TrimRight(strings.ReplaceAll(string(got), "\r\n", "\n"), "\n"), "\n")
			if strings.Join(lines, "\n") != strings.Join(a.want, "\n") {
				t.Errorf("printed %q, and the host's answer is %q", lines, a.want)
			}
		})
	}
}

// utf8Reference is unicode-utf8.oro's expected output, computed by calling the
// real unicode/utf8 in the order the program does. It is hand-written Go and
// never a copy of a run, so a wrong declaration or a wrong conversion shows up
// as a difference from the HOST rather than as agreement with ourselves.
func utf8Reference() []string {
	var out []string
	p := func(v any) { out = append(out, fmt.Sprint(v)) }
	acc := []byte{104}
	for _, r := range []rune{104, 233, 26085, 128578, -1, 55296, 1114112} {
		p(utf8.RuneLen(r))
		p(utf8.ValidRune(r))
		p(utf8.EncodeRune(make([]byte, 4), r))
		acc = utf8.AppendRune(acc, r)
	}
	pair := func(r rune, n int) { p(r); p(n); p(int(r) + n) }
	p(acc)
	p(utf8.RuneCount(acc))
	p(utf8.Valid(acc))
	p(utf8.FullRune(acc))
	pair(utf8.DecodeRune(acc))
	pair(utf8.DecodeLastRune(acc))
	starts := 0
	for _, x := range acc {
		if utf8.RuneStart(x) {
			starts++
		}
	}
	p(starts)
	s := string(acc)
	p(utf8.RuneCountInString(s))
	p(utf8.ValidString(s))
	p(utf8.FullRuneInString(s))
	pair(utf8.DecodeRuneInString(s))
	pair(utf8.DecodeLastRuneInString(s))
	bad := []byte{240, 159, 153}
	p(utf8.FullRune(bad))
	p(utf8.Valid(bad))
	p(utf8.RuneCount(bad))
	pair(utf8.DecodeRune(bad))
	p(utf8.ValidString(string(bad)))
	p(utf8.FullRuneInString(string(bad)))
	p(utf8.RuneCountInString(string(bad)))
	p(utf8.MaxRune)
	p(utf8.RuneError)
	p(utf8.RuneSelf)
	p(utf8.UTFMax)
	p(utf8.ValidRune(utf8.MaxRune + 1))
	p(utf8.RuneLen(utf8.RuneError))
	return out
}
