package main

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

// A PROGRAM A TEST RUNS HAS A TIME LIMIT (assessment-2026-11-18 §5 item 4): a
// built program that stalls, a loop the analysis proved and the host never
// ends, or a window that never gets its WM_QUIT, is killed at programLimit, so
// it costs its one test and not the compiler step's 40 minutes. Every program
// this package's tests run goes through program; `go`, `clang` and the
// toolchains do not, since they are the build, not the program.
var programLimit = 2 * time.Minute

// PlantNoProgramLimit runs a program with no limit, as before
// (TestAProgramThatStallsIsKilled).
var PlantNoProgramLimit bool

func program(name string, args ...string) *exec.Cmd {
	if PlantNoProgramLimit {
		return exec.Command(name, args...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), programLimit)
	cmd := exec.CommandContext(ctx, name, args...)
	// a killed program's pipes are closed this long after, so a grandchild
	// holding them cannot keep Wait from returning
	cmd.WaitDelay = 5 * time.Second
	time.AfterFunc(programLimit+time.Minute, cancel)
	return cmd
}

// TestAProgramThatStallsIsKilled: a child that sleeps past the limit is stopped
// at it, and the run reports an error. The child is this test binary, asked to
// sleep 3 s, with the limit at 300 ms. The fault, no limit, lets it run its 3 s.
func TestAProgramThatStallsIsKilled(t *testing.T) {
	if os.Getenv("ORO_TEST_SLEEP") == "1" {
		time.Sleep(3 * time.Second)
		os.Exit(0)
	}
	run := func() (time.Duration, error) {
		old := programLimit
		programLimit = 300 * time.Millisecond
		defer func() { programLimit = old }()
		cmd := program(os.Args[0], "-test.run=^TestAProgramThatStallsIsKilled$")
		cmd.Env = append(os.Environ(), "ORO_TEST_SLEEP=1")
		start := time.Now()
		err := cmd.Run()
		return time.Since(start), err
	}
	took, err := run()
	if err == nil || took > 2*time.Second {
		t.Errorf("a program sleeping 3 s under a 300 ms limit: err %v after %v; want it killed", err, took)
	}
	PlantNoProgramLimit = true
	took, err = run()
	PlantNoProgramLimit = false
	if err != nil || took < 3*time.Second {
		t.Errorf("with no limit the program should run its 3 s and succeed: err %v after %v", err, took)
	}
}
