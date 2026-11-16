// count: lines, words and bytes, as a person writes it in Go.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: count FILE")
		return
	}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Println("count: cannot read that file")
		return
	}
	if len(b) >= 1<<24 {
		fmt.Println("count: file is larger than this tool accepts")
		return
	}
	lines, words, in := 0, 0, false
	for _, c := range b {
		if c == '\n' {
			lines++
		}
		if c == ' ' || c >= 9 && c <= 13 {
			in = false
		} else if !in {
			in = true
			words++
		}
	}
	fmt.Printf("%d\n%d\n%d\n", lines, words, len(b))
}
