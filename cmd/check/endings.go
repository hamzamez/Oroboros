package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// THE LINE-ENDINGS STEP: no carriage return in a tracked text file whose
// attribute is eol=lf, in the WORKING TREE (assessment-2026-11-16 §3.2).
// .gitattributes keeps the repository LF, but the tests and the compiler read
// the working tree, and a file a tool rewrote with CRLF stays clean to git:
// its index copy is LF and the conversion maps one to the other. Python's text
// mode wrote CRLF on thirteen files that way, and a remembered rule did not
// stop it. So the rule is a check.
//
// The set is git's: `git ls-files --eol` names each tracked file with its
// index content's class (i/…) and its attributes (attr/…). A file is in the
// set when its attribute says eol=lf and its index content is text (not
// i/-text, which is git's binary). Each is read from the working tree and
// must hold no byte 0x0D: the kernel of ν_CR : bytes* → (𝔹, ∨), true on CR,
// the homomorphism ADR 0091 uses for NUL. The two kinds of file whose
// attribute is eol=crlf (.bat, .ps1) are outside the set, and so is the
// emission baseline (-text), which is compared byte for byte instead.

// crlfFiles is each path in the listing, `git ls-files --eol` output, that is
// in the set and whose bytes, as read, hold a carriage return.
func crlfFiles(listing string, read func(string) ([]byte, error)) ([]string, error) {
	var bad []string
	for _, line := range strings.Split(listing, "\n") {
		tab := strings.IndexByte(line, '\t')
		if tab < 0 {
			continue
		}
		fields, path := strings.Fields(line[:tab]), line[tab+1:]
		lf, binary := false, false
		for _, f := range fields {
			switch {
			case f == "eol=lf":
				lf = true
			case f == "i/-text" || f == "attr/-text":
				binary = true
			}
		}
		if !lf || binary {
			continue
		}
		b, err := read(path)
		if os.IsNotExist(err) {
			continue // deleted in the working tree: nothing to read
		}
		if err != nil {
			return nil, err
		}
		if bytes.IndexByte(b, '\r') >= 0 {
			bad = append(bad, path)
		}
	}
	sort.Strings(bad)
	return bad, nil
}

func (c *checker) endings() result {
	out, err := exec.Command("git", "-C", c.root, "-c", "core.quotepath=off", "ls-files", "--eol").Output()
	if err != nil {
		return result{status: fail, detail: "git ls-files --eol: " + err.Error()}
	}
	bad, err := crlfFiles(string(out), func(p string) ([]byte, error) {
		return os.ReadFile(filepath.Join(c.root, filepath.FromSlash(p)))
	})
	if err != nil {
		return result{status: fail, detail: err.Error()}
	}
	n := strings.Count(string(out), "eol=lf")
	if len(bad) > 0 {
		return result{status: fail, detail: fmt.Sprintf("%d tracked text file(s) hold a carriage return; "+
			"rewrite each with LF (rm it and `git checkout -- PATH` restores the index's LF)", len(bad)),
			log: "   " + strings.Join(bad, "\n   ")}
	}
	return result{status: pass, detail: fmt.Sprintf("no carriage return in the %d tracked eol=lf files", n)}
}
