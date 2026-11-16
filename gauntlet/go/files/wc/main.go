// wc: the lines of a file, as a person writes it in Go (gauntlet/files/README.md).
package main

import (
	"bytes"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: wc FILE")
		return
	}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Println("wc: cannot read that file")
		return
	}
	if len(b) >= 1<<24 {
		fmt.Println("wc: file is larger than this tool accepts")
		return
	}
	fmt.Println(bytes.Count(b, []byte{'\n'}))
}
