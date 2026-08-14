//go:build ignore

package main

import "fmt"

func main() {
	for _, s := range []string{"abc", "café", "日本", "🙂", "e\u0301"} {
		fmt.Printf("%d\n", len(s))
	}
}
