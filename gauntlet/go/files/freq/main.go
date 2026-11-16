// freq: the twenty most frequent words, as a person writes it in Go: a map
// from the folded word to its count and first spelling, then one sort, ties
// by the folded word, the order examples/io/freq.oro's stable sorts give.
package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
)

func letter(c byte) bool { return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' }

type entry struct {
	folded, first string
	count         int
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: freq FILE")
		return
	}
	s, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Println("freq: cannot read that file")
		return
	}
	if len(s) >= 1<<16 {
		fmt.Println("freq: file is larger than this tool accepts")
		return
	}
	seen := map[string]*entry{}
	buf := make([]byte, 0, 64)
	for i, nw := 0, 0; i < len(s) && nw < 32768; {
		if !letter(s[i]) {
			i++
			continue
		}
		j := i + 1
		for j < len(s) && letter(s[j]) {
			j++
		}
		buf = buf[:0]
		for _, c := range s[i:j] {
			if c >= 'A' && c <= 'Z' {
				c += 32
			}
			buf = append(buf, c)
		}
		if e, ok := seen[string(buf)]; ok {
			e.count++
		} else {
			k := string(buf)
			seen[k] = &entry{k, string(s[i:j]), 1}
		}
		nw++
		i = j
	}
	es := make([]*entry, 0, len(seen))
	for _, e := range seen {
		es = append(es, e)
	}
	sort.Slice(es, func(a, b int) bool {
		if es[a].count != es[b].count {
			return es[a].count > es[b].count
		}
		return es[a].folded < es[b].folded
	})
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	for r := 0; r < len(es) && r < 20; r++ {
		fmt.Fprintf(w, "%d %s\n", es[r].count, es[r].first)
	}
}
