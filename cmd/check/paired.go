package main

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// A SUSPECT IS TIMED AGAINST THE BASELINE'S OWN BINARY, PAIRED — compiletime-2026-09-25.
//
// The sweep compares this run's CPU time with a number recorded on another day,
// under whatever else the machine was doing then. On a hybrid machine that is
// not one quantity (compiletime-2026-09-25):
//
//   - the same compile of freq costs 1.6x the CPU on an E-core as on a P-core,
//     and which one it gets depends on the rest of the machine's load;
//   - Go's collector runs idle mark workers on every idle P, so a GC-heavy
//     compile's CPU time also grows with how many cores happen to be idle.
//
// Under a neighbour's load both moved at once and freq read 1.63x the baseline
// in two consecutive sweeps, with a serial measurement of the same binaries
// identical. A second sweep cannot clear that, because it samples the same
// afternoon. A PAIRED measurement can: the baseline's binary and this one,
// alternating, serially, so that whatever the machine is doing it does to both,
// and the rule is applied to the ratio of the two minima.
//
// Serial alone is not enough: freq costs 0.68x its sweep time serially, so a
// serial time held against the SWEEP baseline would let a 1.47x regression
// through the 1.5 rule. Only a pair compares like with like.
//
// THE BASELINE'S BINARY is gen built from the commit that last wrote
// compiletime.txt — the commit whose compiler those numbers are of — in a tree
// extracted from it, where it reads its own target files and its own sources.

// pairRounds is how many (baseline, this) pairs are timed; the estimate for each
// side is the minimum, as everywhere else in this gate.
const pairRounds = 3

// pairEnv is the environment of a paired compile. gen is single-threaded, so one
// P changes how its collector is scheduled and not what it computes, and the
// idle mark workers that soak up otherwise idle cores are gone: CPU time is the
// compile's work (freq on Go: 12.3 s at GOMAXPROCS=1 against 17-19 s at 16).
var pairEnv = []string{"GOMAXPROCS=1"}

// pairTimer times one compile: the gen binary, the tree it runs in, the source
// and target. It is a variable so the decision can be tested without a compiler.
type pairTimer func(gen, root string, key timeKey) (time.Duration, error)

// pairedSide is the minimum of each side over the rounds.
type pairedSide struct{ base, now time.Duration }

// pairedTimes alternates the baseline's binary and this one over the suspects,
// round by round, and keeps each side's minimum.
func pairedTimes(run pairTimer, baseGen, baseRoot, gen, root string, keys []timeKey, rounds int) (map[timeKey]pairedSide, error) {
	out := map[timeKey]pairedSide{}
	for r := 0; r < rounds; r++ {
		for _, k := range keys {
			b, err := run(baseGen, baseRoot, k)
			if err != nil {
				return nil, fmt.Errorf("the baseline's binary on %s: %v", k, err)
			}
			n, err := run(gen, root, k)
			if err != nil {
				return nil, fmt.Errorf("this binary on %s: %v", k, err)
			}
			p, seen := out[k]
			if !seen || b < p.base {
				p.base = b
			}
			if !seen || n < p.now {
				p.now = n
			}
			out[k] = p
		}
	}
	return out, nil
}

// pairedVerdict applies the rule to each suspect's pair: those still slower are
// kept, with the paired ratio; the rest are cleared.
func pairedVerdict(suspects []slowCompile, pairs map[timeKey]pairedSide) (kept, cleared []slowCompile) {
	for _, s := range suspects {
		p, ok := pairs[s.key]
		if !ok {
			kept = append(kept, s)
			continue
		}
		s.pairBase, s.pairNow = p.base, p.now
		if len(slower(map[timeKey]time.Duration{s.key: p.base}, map[timeKey]time.Duration{s.key: p.now})) > 0 {
			kept = append(kept, s)
		} else {
			cleared = append(cleared, s)
		}
	}
	return kept, cleared
}

// baselineCommit is the commit that last wrote compiletime.txt. A baseline with
// uncommitted changes has no binary to pair against.
func baselineCommit(root string) (string, error) {
	const file = "gauntlet/check/compiletime.txt"
	st, err := exec.Command("git", "-C", root, "status", "--porcelain", "--", file).Output()
	if err != nil {
		return "", fmt.Errorf("git status: %v", err)
	}
	if len(strings.TrimSpace(string(st))) > 0 {
		return "", fmt.Errorf("%s has uncommitted changes, so no commit holds its compiler", file)
	}
	out, err := exec.Command("git", "-C", root, "log", "-1", "--format=%H", "--", file).Output()
	if err != nil {
		return "", fmt.Errorf("git log: %v", err)
	}
	h := strings.TrimSpace(string(out))
	if h == "" {
		return "", fmt.Errorf("%s was never committed", file)
	}
	return h, nil
}

// extractCommit writes the tree of commit into dir, from `git archive`: no
// worktree, so nothing in .git changes.
func extractCommit(root, commit, dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return extractTar(exec.Command("git", "-C", root, "archive", "--format=tar", commit), dir)
}

// extractTar writes the tar that cmd prints into dir.
func extractTar(cmd *exec.Cmd, dir string) error {
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	// THE PIPE IS DRAINED BEFORE Wait, on every path. The tar reader stops at
	// the end-of-archive marker, but git pads the archive to a 10 KiB record
	// and blocks writing the rest into a pipe nobody reads; Wait then waits for
	// git forever. os/exec says as much ("it is incorrect to call Wait before
	// all reads from the pipe have completed"), and Windows' small pipe buffer
	// made it happen: `check` sat idle for 53 minutes with every file already
	// extracted (irstep4b-2026-09-26).
	defer func() {
		io.Copy(io.Discard, pipe)
		cmd.Wait()
	}()
	tr := tar.NewReader(pipe)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		p := filepath.Join(dir, filepath.FromSlash(h.Name))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(p, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return err
			}
			f, err := os.Create(p)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, tr)
			f.Close()
			if err != nil {
				return err
			}
		}
	}
	if _, err := io.Copy(io.Discard, pipe); err != nil {
		return err
	}
	return cmd.Wait()
}

// timeCompile is the real pairTimer: one gen process, serially, with pairEnv.
func (c *checker) timeCompile(gen, root string, key timeKey) (time.Duration, error) {
	first := ""
	if b, err := os.ReadFile(filepath.Join(root, key.source)); err == nil {
		first, _, _ = strings.Cut(string(b), "\n")
	} else {
		return 0, err
	}
	var args []string
	if needsChecked(key.source, first) {
		args = append(args, "-checked")
	}
	out := filepath.Join(c.work, "paired", emittedName(key.source, key.target))
	args = append(args, key.source, key.target, out)
	cmd := exec.Command(gen, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), pairEnv...)
	_ = cmd.Run() // a refusal costs time too, and the sweep has already judged the outcome
	if cmd.ProcessState == nil {
		return 0, fmt.Errorf("did not run")
	}
	return cmd.ProcessState.UserTime() + cmd.ProcessState.SystemTime(), nil
}

// pairSuspects times every suspect against the baseline's binary and returns
// the ones the pair confirms, the ones it clears, and a sentence for the note.
// When no pair can be made the suspects stand, and the note says why.
func (c *checker) pairSuspects(suspects []slowCompile) (kept, cleared []slowCompile, note string) {
	commit, err := baselineCommit(c.root)
	if err != nil {
		return suspects, nil, "; not paired against the baseline's binary: " + err.Error()
	}
	tree := filepath.Join(c.work, "baseline-tree")
	defer os.RemoveAll(tree)
	if err := extractCommit(c.root, commit, tree); err != nil {
		return suspects, nil, "; not paired: extracting " + commit[:7] + ": " + err.Error()
	}
	baseGen := filepath.Join(c.work, "bin", "gen-baseline")
	if runtime.GOOS == "windows" {
		baseGen += ".exe"
	}
	build := exec.Command("go", "build", "-o", baseGen, "./cmd/gen")
	build.Dir = tree
	if out, err := build.CombinedOutput(); err != nil {
		return suspects, nil, fmt.Sprintf("; not paired: building gen at %s: %v %s", commit[:7], err, strings.TrimSpace(string(out)))
	}
	gen := filepath.Join(c.work, "bin", "gen")
	if runtime.GOOS == "windows" {
		gen += ".exe"
	}
	if err := os.MkdirAll(filepath.Join(c.work, "paired"), 0o755); err != nil {
		return suspects, nil, "; not paired: " + err.Error()
	}
	var keys []timeKey
	for _, s := range suspects {
		keys = append(keys, s.key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	pairs, err := pairedTimes(c.timeCompile, baseGen, tree, gen, c.root, keys, pairRounds)
	if err != nil {
		return suspects, nil, "; not paired: " + err.Error()
	}
	kept, cleared = pairedVerdict(suspects, pairs)
	note = fmt.Sprintf("; paired serially against the baseline's binary (%s), %d confirmed",
		commit[:7], len(kept))
	for _, s := range cleared {
		note += fmt.Sprintf(", %s cleared at %.2fx paired", s.key, s.pairRatio())
	}
	return kept, cleared, note
}

// pairedText is the pair's figure for a report line, or nothing if none was made.
func (s slowCompile) pairedText() string {
	if s.pairBase <= 0 && s.pairNow <= 0 {
		return ""
	}
	t := fmt.Sprintf("; paired %d → %d ms", s.pairBase/time.Millisecond, s.pairNow/time.Millisecond)
	if s.pairBase > 0 {
		t += fmt.Sprintf(", %.2fx", s.pairRatio())
	}
	return t
}
