//go:build ignore

// bench times the file programs as processes (gauntlet/files/README.md):
//
//	go run gauntlet/files/bench.go BIN IN ROUNDS > runs.csv
//
// BIN holds PROGRAM-VARIANT.exe, .mjs (run by node) or .classes (a directory of
// classes with Main, run by java as the differential runs it), IN the inputs
// (inputs.go). Every variant of a
// program is first checked to print what the windows build prints, byte for
// byte; then each round runs every (program, variant) once, in an order
// rotated each round, each run a fresh process timed wall-clock from start to
// exit, alone. CSV: round,program,variant,ms.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

var programs = []struct{ name, input string }{
	{"wc", "text.txt"}, {"count", "text.txt"}, {"freq", "words.txt"}, {"jsonfmt", "doc.json"},
	{"roundtrip", ""}, {"nop", ""},
}

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: go run gauntlet/files/bench.go BIN IN ROUNDS")
		os.Exit(2)
	}
	bin, in := os.Args[1], os.Args[2]
	rounds, _ := strconv.Atoi(os.Args[3])
	type job struct{ prog, variant, exe, input string }
	var jobs []job
	for _, p := range programs {
		var exes []string
		for _, ext := range []string{".exe", ".mjs", ".classes"} {
			m, _ := filepath.Glob(filepath.Join(bin, p.name+"-*"+ext))
			exes = append(exes, m...)
		}
		sort.Strings(exes)
		for _, e := range exes {
			v := strings.TrimPrefix(filepath.Base(e), p.name+"-")
			v = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(v, ".exe"), ".mjs"), ".classes")
			input := ""
			if p.input != "" {
				input = filepath.Join(in, p.input)
			}
			jobs = append(jobs, job{p.name, v, e, input})
		}
	}
	tmp, _ := os.MkdirTemp("", "files-bench")
	defer os.RemoveAll(tmp)
	runOnce := func(j job) ([]byte, time.Duration, error) {
		args := []string{}
		if j.prog == "roundtrip" {
			args = append(args, filepath.Join(tmp, "rt-"+j.variant+".txt"))
		} else if j.input != "" {
			args = append(args, j.input)
		}
		var cmd *exec.Cmd
		switch {
		case strings.HasSuffix(j.exe, ".mjs"):
			cmd = exec.Command("node", append([]string{j.exe}, args...)...)
		case strings.HasSuffix(j.exe, ".classes"):
			cmd = exec.Command("java", append([]string{"-Dfile.encoding=UTF-8", "-Dstdout.encoding=UTF-8",
				"-Dsun.stdout.encoding=UTF-8", "-cp", j.exe, "Main"}, args...)...)
		default:
			cmd = exec.Command(j.exe, args...)
		}
		cmd.Env = append(os.Environ(), "ORO_ROUNDTRIP=x")
		var out bytes.Buffer
		cmd.Stdout = &out
		t0 := time.Now()
		err := cmd.Run()
		return out.Bytes(), time.Since(t0), err
	}
	// AGREEMENT FIRST: every variant prints what the windows build prints.
	want := map[string][]byte{}
	for _, j := range jobs {
		if j.variant == "windows" {
			o, _, err := runOnce(j)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s windows: %v\n", j.prog, err)
				os.Exit(1)
			}
			want[j.prog] = o
		}
	}
	for _, j := range jobs {
		o, _, err := runOnce(j)
		if err != nil || !bytes.Equal(o, want[j.prog]) {
			fmt.Fprintf(os.Stderr, "%s %s does not print what the windows build prints (%v)\n", j.prog, j.variant, err)
			os.Exit(1)
		}
	}
	fmt.Fprintf(os.Stderr, "%d variants agree\n", len(jobs))
	for r := 1; r <= rounds; r++ {
		k := r % len(jobs)
		for i := range jobs {
			j := jobs[(i+k)%len(jobs)]
			_, d, err := runOnce(j)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s %s: %v\n", j.prog, j.variant, err)
				os.Exit(1)
			}
			fmt.Printf("%d,%s,%s,%.3f\n", r, j.prog, j.variant, float64(d.Microseconds())/1000)
		}
	}
}
