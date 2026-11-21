package main

// AN ACCEPT REUSES THE VERDICT OF A PASSING RUN OF THE SAME TREE (CLAUDE.md
// plan item 0, hamza 2026-11-20: "agree, put it in the plan").
//
// The check is a function of what it reads, the tree's files and the toolchain
// that reads them: the emitter is a function of its input, which this check
// verifies (the gauntlet step, the IR's reprinting, byte-identical emission),
// and every other step is a verdict no second run on the same input changes.
// So a run's verdicts are those of every run of the same input, and a step
// that paid a review run and then an accepting run of one tree paid twice for
// one verdict.
//
// THE INPUT is a hash: SHA-256 over every file git would track (tracked, and
// untracked but not ignored), each as its path, a NUL, its length and its bytes,
// in path order; with `go version` and the -tier2 flag. A run that writes no
// baseline records its steps' statuses under the hash taken when it began, and
// only if the hash is the same when it ends (a tree edited during a run was
// not the tree checked). An -accept whose hash is the record's, the record's
// compiler and differential steps passing, runs the emission step alone, for
// the baselines it writes from, and takes every other step's verdict from the
// record. Its emission must be the recorded one, outcome for outcome
// (formatOutcomes' digest): a difference is the emitter not a function of its
// input, and the accept is refused.

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// runRecord is a run's record, .check/verdict.json.
type runRecord struct {
	Tree     string            // the input's hash
	When     string            // when the run ended
	Steps    map[string]status // each step's status
	Details  map[string]string // and its detail line
	Emission map[string]string // each emission step's digest of its outcomes
}

// PlantVerdictAnyTree reuses a record whatever tree it was taken of
// (TestAVerdictIsReusedOnlyForItsTree).
var PlantVerdictAnyTree bool

// PlantEmissionDigestUnchecked accepts a reused verdict whatever the emission
// rerun gave (TestAVerdictIsReusedOnlyForItsTree).
var PlantEmissionDigestUnchecked bool

// inputHash is the hash of what a run reads.
func (c *checker) inputHash(tier2 bool) (string, error) {
	cmd := exec.Command("git", "ls-files", "-z", "-c", "-o", "--exclude-standard")
	cmd.Dir = c.root
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git ls-files: %v", err)
	}
	var paths []string
	seen := map[string]bool{}
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" && !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	h := sha256.New()
	var n [8]byte
	for _, p := range paths {
		b, err := os.ReadFile(filepath.Join(c.root, filepath.FromSlash(p)))
		if err != nil {
			if os.IsNotExist(err) {
				continue // deleted in the working tree, not yet in the index
			}
			return "", err
		}
		h.Write([]byte(p))
		h.Write([]byte{0})
		binary.LittleEndian.PutUint64(n[:], uint64(len(b)))
		h.Write(n[:])
		h.Write(b)
	}
	gv, err := exec.Command("go", "version").Output()
	if err != nil {
		return "", fmt.Errorf("go version: %v", err)
	}
	fmt.Fprintf(h, "\x00go %s\x00tier2 %v", strings.TrimSpace(string(gv)), tier2)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// emissionDigest is the hash of a sweep's outcomes as outcomes.txt holds them.
func emissionDigest(outs []outcome) string {
	s := sha256.Sum256([]byte(formatOutcomes(outs)))
	return hex.EncodeToString(s[:])
}

func (c *checker) verdictPath() string { return filepath.Join(c.work, "verdict.json") }

func (c *checker) readVerdict() (runRecord, bool) {
	var v runRecord
	b, err := os.ReadFile(c.verdictPath())
	if err != nil || json.Unmarshal(b, &v) != nil {
		return runRecord{}, false
	}
	return v, true
}

func (c *checker) writeVerdict(v runRecord) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.verdictPath(), b, 0o644)
}

// reusable says whether record v decides an accept of the tree whose hash is
// tree, and why not.
func reusable(v runRecord, tree string, differential string) (bool, string) {
	switch {
	case v.Tree == "":
		return false, "no run of this tree is recorded"
	case v.Tree != tree && !PlantVerdictAnyTree:
		return false, "the tree is not the one the last run checked"
	case v.Steps["compiler"] != pass:
		return false, "the last run's compiler step is " + string(v.Steps["compiler"])
	case v.Steps[differential] != pass:
		return false, "the last run's " + differential + " step is " + string(v.Steps[differential])
	case v.Steps["tooling"] == fail:
		return false, "the last run's tooling step failed"
	}
	return true, ""
}

// sameEmission says whether a rerun's emission is the recorded one.
func sameEmission(v runRecord, step, digest string) bool {
	return PlantEmissionDigestUnchecked || v.Emission[step] == digest
}

// recordRun writes this run's verdict, when it read one tree throughout.
func (c *checker) recordRun(began string, tier2 bool, emissions map[string]string) {
	end, err := c.inputHash(tier2)
	if err != nil || end != began {
		_ = os.Remove(c.verdictPath()) // the tree moved during the run: no record
		return
	}
	v := runRecord{Tree: began, When: time.Now().Format(time.RFC3339), Steps: map[string]status{},
		Details: map[string]string{}, Emission: emissions}
	for name, r := range c.results {
		v.Steps[name] = r.status
		v.Details[name] = r.detail
	}
	_ = c.writeVerdict(v)
}

// reuseRefusal is why a record does not decide this accept, "" when it does:
// the main tier's steps, and the second tier's differential when -tier2 runs.
func reuseRefusal(v runRecord, tree string, c, c32 *checker) string {
	if ok, why := reusable(v, tree, c.differentialStep()); !ok {
		return why
	}
	if c32 != nil && v.Steps[c32.differentialStep()] != pass {
		return "the last run's " + c32.differentialStep() + " step is " + string(v.Steps[c32.differentialStep()])
	}
	return ""
}
