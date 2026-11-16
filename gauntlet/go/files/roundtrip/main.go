// roundtrip: put, get, the variable, the length, a read at an offset, and a
// missing file's kind, as a person writes it in Go.
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: roundtrip FILE")
		os.Exit(1)
	}
	path := os.Args[1]
	if err := os.WriteFile(path, []byte("hi €\n"), 0o644); err != nil {
		fmt.Println("roundtrip: cannot write that file: other")
		os.Exit(1)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("roundtrip: cannot read that file: other")
		os.Exit(1)
	}
	os.Stdout.Write(b)
	fmt.Println(os.Getenv("ORO_ROUNDTRIP"))
	fmt.Println(len(b))
	f, _ := os.Open(path)
	buf := make([]byte, 4)
	n, _ := f.ReadAt(buf, 3)
	f.Close()
	fmt.Printf("%d %03d\n", n%10, buf[0])
	if _, err := os.ReadFile(path + ".missing"); errors.Is(err, fs.ErrNotExist) {
		fmt.Println("not-found")
	} else {
		fmt.Println("found")
	}
}
