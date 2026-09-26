package main

import (
	"archive/tar"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// THE EXTRACTION DRAINS ITS PIPE (paired.go, extractTar). A tar reader stops
// at the end-of-archive marker, and whatever the writer prints after it —
// git's padding to a 10 KiB record — must still be read, or the writer blocks
// on a full pipe and Wait waits for it forever. The helper below prints a
// one-file archive and then 1 MiB of zeros, which no pipe buffer holds; the
// extraction must finish, and the file must be there.

func TestHelperTarWriter(t *testing.T) {
	if os.Getenv("ORO_TAR_HELPER") != "1" {
		return
	}
	w := tar.NewWriter(os.Stdout)
	body := []byte("(def x 1)\n")
	w.WriteHeader(&tar.Header{Name: "a/b.oro", Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
	w.Write(body)
	w.Close()
	os.Stdout.Write(make([]byte, 1<<20))
	os.Exit(0)
}

func TestExtractionDrainsItsPipe(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperTarWriter$")
	cmd.Env = append(os.Environ(), "ORO_TAR_HELPER=1")
	dir := t.TempDir()
	done := make(chan error, 1)
	go func() { done <- extractTar(cmd, dir) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(30 * time.Second):
		cmd.Process.Kill()
		t.Fatal("the extraction did not finish: Wait is waiting on a writer blocked on an undrained pipe")
	}
	if b, err := os.ReadFile(filepath.Join(dir, "a", "b.oro")); err != nil || string(b) != "(def x 1)\n" {
		t.Fatalf("the extracted file: %q, %v", b, err)
	}
}
