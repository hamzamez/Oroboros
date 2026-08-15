//go:build ignore

// Conformance runner for the Go target. See README.md.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	b, err := os.ReadFile("cases.json")
	if err != nil {
		panic(err)
	}
	var cases []string
	if err := json.Unmarshal(b, &cases); err != nil {
		panic(err)
	}
	for _, s := range cases {
		f := strings.Fields(s) // the specified semantics
		out, _ := json.Marshal(f)
		fmt.Printf("%d %s\n", len(f), out)
	}
}
