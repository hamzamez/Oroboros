package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// THE ASSEMBLY STEP: every windows file the sweep emitted goes through clang,
// at the flags the target's own build uses (emit/target.go's build.bat), into
// an object. The emission step compares TEXT, and text can be wrong in a way
// no comparison sees: `search`'s windows file was accepted printing
// `icmp sgt double`, which clang refuses (llvmf64-2026-11-06). So an emitted
// .ll is checked by the one reader that decides whether it is LLVM IR. A file
// clang refuses fails the step, baseline or none.

// clangPath is the official LLVM release's clang, where it installs it, or
// clang on PATH, as build.bat finds it; "" when there is none.
func clangPath() string {
	if p := filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "LLVM", "bin", "clang.exe"); fileExists(p) {
		return p
	}
	if p, err := exec.LookPath("clang"); err == nil {
		return p
	}
	return ""
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// assembleOne compiles one module to an object at the build's flags, and
// returns clang's diagnostics when it refuses.
func assembleOne(clang, ll, obj string) error {
	cmd := exec.Command(clang, "-O2", "-Wno-override-module", "-c", "-o", obj, ll)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v\n%s", err, out)
	}
	return nil
}

func (c *checker) assemble() result {
	if !c.emissionRan {
		return result{status: skip, detail: "needs the emission sweep"}
	}
	clang := clangPath()
	if clang == "" {
		return result{status: skip, detail: "no clang: the windows target cannot build here either"}
	}
	objs := filepath.Join(c.work, "objects")
	if err := os.RemoveAll(objs); err != nil {
		return result{status: fail, detail: err.Error()}
	}
	if err := os.MkdirAll(objs, 0o755); err != nil {
		return result{status: fail, detail: err.Error()}
	}
	var files []string
	for _, o := range c.emitted {
		if o.Target == "windows" && o.Emitted {
			files = append(files, emittedName(o.Source, o.Target))
		}
	}
	errs := make([]string, len(files))
	var wg sync.WaitGroup
	sem := make(chan struct{}, max(c.jobs/2, 1))
	for i, name := range files {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ll := filepath.Join(c.work, "emitted", name)
			if err := assembleOne(clang, ll, filepath.Join(objs, strings.TrimSuffix(name, ".ll")+".obj")); err != nil {
				errs[i] = fmt.Sprintf("%s: %s", name, firstLines(err.Error(), 4))
			}
		}()
	}
	wg.Wait()
	var refused []string
	for _, e := range errs {
		if e != "" {
			refused = append(refused, e)
		}
	}
	if len(refused) > 0 {
		return result{status: fail, log: "   " + strings.Join(refused, "\n   "),
			detail: fmt.Sprintf("clang refuses %d of %d windows file(s)", len(refused), len(files))}
	}
	return result{status: pass, detail: fmt.Sprintf("all %d windows files assemble (clang -O2 -c)", len(files))}
}
