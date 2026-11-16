//go:build ignore

// inputs writes the file programs' inputs (gauntlet/files/README.md), each
// deterministic, each under its program's own cap:
//
//	go run gauntlet/files/inputs.go DIR
//
//	text.txt   8 MiB of words in lines, for wc and count (cap 2^24)
//	words.txt  60 KiB of mixed-case words, for freq (cap 2^16)
//	doc.json   60 KiB of minified JSON, for jsonfmt (cap 2^16)
package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

func vocab(r *rand.Rand, n int) []string {
	v := make([]string, n)
	for i := range v {
		b := make([]byte, 2+r.Intn(8))
		for j := range b {
			b[j] = byte('a' + r.Intn(26))
		}
		v[i] = string(b)
	}
	return v
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run gauntlet/files/inputs.go DIR")
		os.Exit(2)
	}
	dir := os.Args[1]
	write := func(name string, s string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(s), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	// text.txt: lines of 1–14 words, spaces and a tab between them
	r := rand.New(rand.NewSource(11))
	v := vocab(r, 2000)
	var b strings.Builder
	for b.Len() < 8<<20 {
		n := 1 + r.Intn(14)
		for k := 0; k < n; k++ {
			if k > 0 {
				if r.Intn(10) == 0 {
					b.WriteByte('\t')
				} else {
					b.WriteByte(' ')
				}
			}
			b.WriteString(v[r.Intn(len(v))])
		}
		b.WriteByte('\n')
	}
	write("text.txt", b.String()[:8<<20])

	// words.txt: 500 words, a third capitalized, punctuation between
	r = rand.New(rand.NewSource(12))
	v = vocab(r, 500)
	b.Reset()
	for b.Len() < 60<<10 {
		w := v[r.Intn(len(v))]
		if r.Intn(3) == 0 {
			w = strings.ToUpper(w[:1]) + w[1:]
		}
		b.WriteString(w)
		switch r.Intn(8) {
		case 0:
			b.WriteString(", ")
		case 1:
			b.WriteString(".\n")
		default:
			b.WriteByte(' ')
		}
	}
	write("words.txt", b.String()[:60<<10])

	// doc.json: records of the tokenizer's shape, minified, closed
	b.Reset()
	b.WriteString(`{"items":[`)
	rec := `{"id":1234,"name":"a b\"c","tags":["x","y","z"],"score":-12.5e3,"ok":true,"prev":null,"meta":{"depth":2,"flag":false}}`
	for i := 0; b.Len()+len(rec)+3 < 60<<10; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(rec)
	}
	b.WriteString("]}")
	write("doc.json", b.String())
}
