package main

import (
	"crypto/sha256"
	"encoding/hex"
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
	"time"
)

// THE EMISSION SWEEP, AND WHAT A BASELINE RECORDS.
//
// Every .oro file under examples/, lib/ and gauntlet/differential/cases/ is
// compiled for every target, and the OUTCOME is recorded whether it emits or
// not. The sweep this replaces deleted a file that failed to emit, so a program
// that STOPPED emitting simply vanished from the comparison; and it excluded
// examples/tally, which is how tally's refusal went unseen.
//
// An outcome is:
//   - whether the program emitted,
//   - the SHA-256 of what it emitted,
//   - every line the compiler printed: the notes, which carry the per-program
//     proof counts ("N of M integer operations bounded; K of L loops proven
//     terminating"), and, for a refusal, the refusal itself.
//
// So a weaker proof shows as a change in the notes before it shows as a
// refusal, a new refusal shows as a change of outcome, and a diagnostic that
// changed its wording shows too.

var targets = []struct{ name, ext string }{
	{"go", "go"}, {"js", "mjs"}, {"java", "java"}, {"windows", "asm"},
}

type outcome struct {
	Source  string
	Target  string
	Emitted bool
	Hash    string   // hex SHA-256 of the emitted text; "" when refused
	Lines   []string // everything the compiler printed on stderr, normalised

	// What the compile COST, which is not part of an outcome: outcomes.txt must
	// be byte-identical across runs, and a time never is. See compiletime.go.
	CPU  time.Duration // user + system time of the gen process
	Wall time.Duration
}

func (o outcome) key() string { return o.Source + " " + o.Target }

// emittedName is the file an outcome's text is stored under.
func emittedName(source, target string) string {
	stem := strings.ReplaceAll(strings.TrimSuffix(source, ".oro"), "/", "_")
	for _, t := range targets {
		if t.name == target {
			return stem + "." + target + "." + t.ext
		}
	}
	return stem + "." + target
}

// needsChecked says whether a source is compiled with -checked.
//
// Two sources of truth, and no guess. The differential runner builds every case
// with -checked (gauntlet/differential/run.go), so its cases are swept the same
// way. An example that needs it says so on its FIRST line, in the form four of
// them already use: "; BUILT WITH `-checked`".
func needsChecked(source string, firstLine string) bool {
	if strings.HasPrefix(source, "gauntlet/differential/cases/") {
		return true
	}
	return strings.Contains(firstLine, "BUILT WITH `-checked`")
}

func (c *checker) sources() ([]string, error) {
	var out []string
	for _, dir := range []string{"examples", "lib", "gauntlet/differential/cases"} {
		err := filepath.WalkDir(filepath.Join(c.root, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(p, ".oro") {
				rel, err := filepath.Rel(c.root, p)
				if err != nil {
					return err
				}
				out = append(out, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

// sweep compiles every source for every target into dir and returns the outcomes.
func (c *checker) sweep(dir string) ([]outcome, error) {
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := os.RemoveAll(irDir(c.work)); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(irDir(c.work), 0o755); err != nil {
		return nil, err
	}
	gen := filepath.Join(c.work, "bin", "gen")
	if runtime.GOOS == "windows" {
		gen += ".exe"
	}
	build := exec.Command("go", "build", "-o", gen, "./cmd/gen")
	build.Dir = c.root
	if out, err := build.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("building cmd/gen: %v\n%s", err, out)
	}
	srcs, err := c.sources()
	if err != nil {
		return nil, err
	}

	type job struct {
		i       int
		source  string
		target  string
		checked bool
	}
	var jobs []job
	for _, s := range srcs {
		first := ""
		if b, err := os.ReadFile(filepath.Join(c.root, s)); err == nil {
			first, _, _ = strings.Cut(string(b), "\n")
		}
		for _, t := range targets {
			jobs = append(jobs, job{len(jobs), s, t.name, needsChecked(s, first)})
		}
	}
	outs := make([]outcome, len(jobs))
	work := make(chan job)
	var wg sync.WaitGroup
	n := c.jobs
	if n < 1 {
		n = 1
	}
	for w := 0; w < n; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range work {
				outs[j.i] = c.compile(gen, dir, j.source, j.target, j.checked)
			}
		}()
	}
	for _, j := range jobs {
		work <- j
	}
	close(work)
	wg.Wait()
	return outs, nil
}

func (c *checker) compile(gen, dir, source, target string, checked bool) outcome {
	o := outcome{Source: source, Target: target}
	file := filepath.Join(dir, emittedName(source, target))
	var args []string
	if checked {
		args = append(args, "-checked")
	}
	args = append(args, "-ir", filepath.Join(irDir(c.work), irName(source, target)))
	args = append(args, source, target, file)
	cmd := exec.Command(gen, args...)
	cmd.Dir = c.root
	var stderr strings.Builder
	cmd.Stderr = &stderr
	start := time.Now()
	err := cmd.Run()
	o.Wall = time.Since(start)
	if cmd.ProcessState != nil {
		o.CPU = cmd.ProcessState.UserTime() + cmd.ProcessState.SystemTime()
	}
	o.Lines = normalise(stderr.String(), c.root, dir)
	if err != nil {
		_ = os.Remove(file)
		return o
	}
	text, rerr := os.ReadFile(file)
	if rerr != nil {
		o.Lines = append(o.Lines, "check: emitted file unreadable: "+rerr.Error())
		return o
	}
	sum := sha256.Sum256(text)
	o.Emitted, o.Hash = true, hex.EncodeToString(sum[:])
	return o
}

// normalise makes compiler output comparable across machines and runs: line
// endings, trailing space, blank lines, and any absolute path of this checkout
// or of the work directory.
func normalise(s, root, work string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	for _, p := range []string{work, root} {
		for _, form := range []string{p, filepath.ToSlash(p)} {
			if form != "" {
				s = strings.ReplaceAll(s, form, "<root>")
			}
		}
	}
	var out []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimRight(l, " \t")
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// ---------------------------------------------------------------- the baseline

const outcomesHeader = `# gauntlet/check/outcomes.txt — written by "go run ./cmd/check -accept". Do not edit by hand.
# One block per source and target: "== SOURCE TARGET emitted HASH" or "== SOURCE TARGET refused",
# then every line the compiler printed. The emitted text itself is in gauntlet/check/testdata/emitted/.
`

func formatOutcomes(list []outcome) string {
	var b strings.Builder
	b.WriteString(outcomesHeader)
	for _, o := range sorted(list) {
		if o.Emitted {
			fmt.Fprintf(&b, "== %s %s emitted %s\n", o.Source, o.Target, o.Hash)
		} else {
			fmt.Fprintf(&b, "== %s %s refused\n", o.Source, o.Target)
		}
		for _, l := range o.Lines {
			b.WriteString("   " + l + "\n")
		}
	}
	return b.String()
}

func parseOutcomes(text string) ([]outcome, error) {
	var out []outcome
	for n, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		switch {
		case raw == "" || strings.HasPrefix(raw, "#"):
		case strings.HasPrefix(raw, "== "):
			f := strings.Fields(raw[3:])
			if len(f) < 3 {
				return nil, fmt.Errorf("outcomes.txt line %d: malformed header %q", n+1, raw)
			}
			o := outcome{Source: f[0], Target: f[1]}
			switch {
			case f[2] == "emitted" && len(f) == 4:
				o.Emitted, o.Hash = true, f[3]
			case f[2] == "refused" && len(f) == 3:
			default:
				return nil, fmt.Errorf("outcomes.txt line %d: malformed header %q", n+1, raw)
			}
			out = append(out, o)
		case strings.HasPrefix(raw, "   "):
			if len(out) == 0 {
				return nil, fmt.Errorf("outcomes.txt line %d: a line before any header", n+1)
			}
			out[len(out)-1].Lines = append(out[len(out)-1].Lines, raw[3:])
		default:
			return nil, fmt.Errorf("outcomes.txt line %d: unrecognised %q", n+1, raw)
		}
	}
	return out, nil
}

func sorted(list []outcome) []outcome {
	order := map[string]int{}
	for i, t := range targets {
		order[t.name] = i
	}
	cp := append([]outcome(nil), list...)
	sort.Slice(cp, func(i, j int) bool {
		if cp[i].Source != cp[j].Source {
			return cp[i].Source < cp[j].Source
		}
		return order[cp[i].Target] < order[cp[j].Target]
	})
	return cp
}

// ---------------------------------------------------------------- comparing

type changeKind string

const (
	textChanged  changeKind = "emitted text changed"
	notesChanged changeKind = "compiler output changed"
	nowRefused   changeKind = "now REFUSED"
	nowEmitted   changeKind = "now emits"
	added        changeKind = "new source"
	removed      changeKind = "source removed"
)

type change struct {
	key  string
	kind changeKind
	old  outcome
	new  outcome
}

// compare lists every difference between two sweeps, in a total order, so two
// runs of the check on one tree print the same report.
func compare(old, new []outcome) []change {
	om, nm := map[string]outcome{}, map[string]outcome{}
	var keys []string
	for _, o := range old {
		om[o.key()] = o
		keys = append(keys, o.key())
	}
	for _, o := range new {
		if _, ok := om[o.key()]; !ok {
			keys = append(keys, o.key())
		}
		nm[o.key()] = o
	}
	sort.Strings(keys)
	var out []change
	for _, k := range keys {
		o, inOld := om[k]
		n, inNew := nm[k]
		switch {
		case !inOld:
			out = append(out, change{k, added, o, n})
		case !inNew:
			out = append(out, change{k, removed, o, n})
		case o.Emitted && !n.Emitted:
			out = append(out, change{k, nowRefused, o, n})
		case !o.Emitted && n.Emitted:
			out = append(out, change{k, nowEmitted, o, n})
		default:
			if o.Hash != n.Hash {
				out = append(out, change{k, textChanged, o, n})
			}
			if strings.Join(o.Lines, "\n") != strings.Join(n.Lines, "\n") {
				out = append(out, change{k, notesChanged, o, n})
			}
		}
	}
	return out
}

var proofRE = regexp.MustCompile(`(\d+) of (\d+) integer operations bounded; (\d+) of (\d+) loop\(s\) proven terminating`)

// proofTotals sums the proof counts every program's notes report.
func proofTotals(list []outcome) (bounded, ops, proven, loops int) {
	for _, o := range list {
		for _, l := range o.Lines {
			if m := proofRE.FindStringSubmatch(l); m != nil {
				a, _ := strconv.Atoi(m[1])
				b, _ := strconv.Atoi(m[2])
				c, _ := strconv.Atoi(m[3])
				d, _ := strconv.Atoi(m[4])
				bounded, ops, proven, loops = bounded+a, ops+b, proven+c, loops+d
			}
		}
	}
	return
}

func countEmitted(list []outcome) (emitted, refused int) {
	for _, o := range list {
		if o.Emitted {
			emitted++
		} else {
			refused++
		}
	}
	return
}

// ---------------------------------------------------------------- the step

func (c *checker) emission() result {
	baseDir := filepath.Join(c.root, "gauntlet", "check")
	newDir := filepath.Join(c.work, "emitted")
	outs, err := c.sweep(newDir)
	if err != nil {
		return result{status: fail, detail: err.Error()}
	}
	c.emissionRan, c.emitted = true, outs
	if err := os.WriteFile(filepath.Join(c.work, "outcomes.txt"), []byte(formatOutcomes(outs)), 0o644); err != nil {
		return result{status: fail, detail: err.Error()}
	}
	e, r := countEmitted(outs)
	b, ops, p, loops := proofTotals(outs)
	summary := fmt.Sprintf("%d runs: %d emitted, %d refused; %d of %d integer operations bounded, %d of %d loops proven",
		len(outs), e, r, b, ops, p, loops)
	c.summary = summary

	// COMPILE TIME, before the outcomes: it has to leave c.times set for -accept
	// whether or not an outcomes baseline exists yet.
	note, slow, err := c.compileTime(baseDir, outs)
	if err != nil {
		return result{status: fail, detail: err.Error()}
	}
	c.timeNote, c.slow = note, slow

	baseText, err := os.ReadFile(filepath.Join(baseDir, "outcomes.txt"))
	switch {
	case os.IsNotExist(err):
		c.noBaseline = true
		return result{status: review, detail: summary + " — no baseline yet; create one with -accept \"initial baseline\""}
	case err != nil:
		return result{status: fail, detail: err.Error()}
	}
	old, err := parseOutcomes(string(baseText))
	if err != nil {
		return result{status: fail, detail: err.Error()}
	}
	c.changes = compare(old, outs)
	ob, oops, op, oloops := proofTotals(old)
	if ob != b || oops != ops || op != p || oloops != loops {
		fmt.Printf("   proof counts: %d of %d operations and %d of %d loops, against %d of %d and %d of %d in the baseline\n",
			b, ops, p, loops, ob, oops, op, oloops)
	}
	for _, ch := range c.changes {
		fmt.Printf("   %-24s %s\n", ch.kind, ch.key)
		if ch.kind == textChanged {
			name := emittedName(ch.new.Source, ch.new.Target)
			fmt.Printf("   %-24s git diff --no-index gauntlet/check/testdata/emitted/%s .check/emitted/%s\n", "", name, name)
		}
	}
	fmt.Printf("   %s\n", note)
	for _, s := range slow {
		fmt.Printf("   %-24s %s  %d → %d ms, %.2fx%s\n", "compiles SLOWER", s.key,
			s.base/time.Millisecond, s.now/time.Millisecond, s.ratio(), s.pairedText())
	}
	for _, s := range c.fast {
		fmt.Printf("   %-24s %s  %d → %d ms, %.2fx\n", "compiles faster", s.key,
			s.base/time.Millisecond, s.now/time.Millisecond, s.ratio())
	}
	var why []string
	if len(c.changes) > 0 {
		why = append(why, fmt.Sprintf("%d change(s) against the baseline", len(c.changes)))
	}
	if len(slow) > 0 {
		why = append(why, fmt.Sprintf("%d compile(s) slower than the baseline", len(slow)))
	}
	if c.noTimeBaseline {
		why = append(why, "no compile-time baseline yet")
	}
	if len(why) > 0 {
		return result{status: review, detail: summary + " — " + strings.Join(why, "; ")}
	}
	return result{status: pass, detail: summary + " — byte-identical to the baseline; " + note}
}

// acceptBaseline makes this run's emission the committed baseline and appends
// the reason to the log.
func (c *checker) acceptBaseline(reason string, tooling status) error {
	baseDir := filepath.Join(c.root, "gauntlet", "check")
	newDir := filepath.Join(c.work, "emitted")
	// Under testdata/, which the go tool never builds: the baseline holds one Go
	// file per program, all in `package main`, and `go vet ./...` read them as one
	// package with a hundred `GenMain`s.
	emittedDir := filepath.Join(baseDir, "testdata", "emitted")
	if err := os.RemoveAll(emittedDir); err != nil {
		return err
	}
	if err := os.MkdirAll(emittedDir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(newDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(newDir, e.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(emittedDir, e.Name()), b, 0o644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(baseDir, "outcomes.txt"), []byte(formatOutcomes(c.emitted)), 0o644); err != nil {
		return err
	}
	// THE COMPILE-TIME BASELINE IS A MINIMUM OF TWO SWEEPS. One sweep is an upper
	// bound under positive noise, and a baseline that happens to be slow HIDES a
	// regression: at the 1.21x noise measured between identical sweeps, a single
	// slow baseline would let freq's 1.67x through the 1.5 rule.
	if err := c.secondSweep(c.emitted); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(baseDir, "compiletime.txt"), []byte(formatTimes(c.times)), 0o644); err != nil {
		return err
	}

	head := "unknown"
	if out, err := exec.Command("git", "-C", c.root, "rev-parse", "--short", "HEAD").Output(); err == nil {
		head = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "-C", c.root, "status", "--porcelain").Output(); err == nil && len(out) > 0 {
		head += ", with uncommitted changes"
	}
	e, r := countEmitted(c.emitted)
	bnd, ops, p, loops := proofTotals(c.emitted)
	var b strings.Builder
	fmt.Fprintf(&b, "\n## %s — on %s\n\n**Reason:** %s\n\n", time.Now().UTC().Format("2006-01-02"), head, reason)
	fmt.Fprintf(&b, "%d runs: %d emitted, %d refused; %d of %d integer operations bounded, %d of %d loops proven. "+
		"compiler pass, differential pass, tooling %s.\n\n", len(c.emitted), e, r, bnd, ops, p, loops, tooling)
	if c.noBaseline {
		b.WriteString("The initial baseline.\n")
	} else if len(c.changes) > 0 {
		fmt.Fprintf(&b, "%d change(s):\n\n", len(c.changes))
		for _, ch := range c.changes {
			fmt.Fprintf(&b, "- %s — `%s`\n", ch.kind, ch.key)
		}
	}
	switch {
	case c.noTimeBaseline:
		b.WriteString("\nThe initial compile-time baseline, the minimum of two sweeps.\n")
	case c.retime && len(c.slow) == 0:
		fmt.Fprintf(&b, "\nThe compile-time baseline RE-RECORDED (-retime), the minimum of two sweeps — %s.\n", c.timeNote)
	case len(c.slow) > 0:
		fmt.Fprintf(&b, "\n%d compile(s) accepted SLOWER — %s:\n\n", len(c.slow), c.timeNote)
		for _, s := range c.slow {
			fmt.Fprintf(&b, "- `%s` %d → %d ms, %.2fx%s\n", s.key, s.base/time.Millisecond, s.now/time.Millisecond, s.ratio(), s.pairedText())
		}
	}
	if len(c.fast) > 0 {
		fmt.Fprintf(&b, "\n%d compile(s) recorded FASTER:\n\n", len(c.fast))
		for _, s := range c.fast {
			fmt.Fprintf(&b, "- `%s` %d → %d ms, %.2fx\n", s.key, s.base/time.Millisecond, s.now/time.Millisecond, s.ratio())
		}
	}
	logPath := filepath.Join(baseDir, "ACCEPTED.md")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if fi, err := f.Stat(); err == nil && fi.Size() == 0 {
		if _, err := f.WriteString("# Accepted emission changes\n\nEvery change to the baseline in this directory, with its reason. " +
			"Written by `go run ./cmd/check -accept`; the rule is in [README.md](README.md).\n"); err != nil {
			return err
		}
	}
	_, err = f.WriteString(b.String())
	return err
}
