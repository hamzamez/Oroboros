// Command toj prints a program of today's notation in J (spec/j.md §7).
//
//	go run ./cmd/toj FILE.oro
package main

import (
	"fmt"
	"os"

	"oroboros/j"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: toj FILE.oro")
		os.Exit(2)
	}
	r, err := j.TranslateFile(os.Args[1], ".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "toj: %s: %v\n", os.Args[1], err)
		os.Exit(1)
	}
	fmt.Print(r.J)
	if r.Uncertain > 0 {
		fmt.Fprintf(os.Stderr, "toj: %d one-argument applications of a local name printed as applications\n", r.Uncertain)
	}
}
