// Command wininputs writes the windows gauntlet's inputs and Go's answers on them
// (gauntlet/windows/README.md). Go's math/rand makes the random vectors, so a C
// harness cannot draw the same ones; it reads them instead, and checks every case
// against the answer the hand-written Go computes before timing anything.
//
//	cd gauntlet/go && go run ./wininputs DIR
//
// DIR/vec.bin is vecA then vecB, NVec little-endian doubles each (smallA and
// smallB are their first NSmall). DIR/answers.txt is one `name value` per line,
// a double as its bits in hex, so nothing is lost to printing.
package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"

	g "oroboros/gauntlet"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./wininputs DIR")
		os.Exit(2)
	}
	dir := os.Args[1]
	a, b := g.MakeVec(g.NVec, 1), g.MakeVec(g.NVec, 2)
	buf := make([]byte, 0, 16*g.NVec)
	for _, v := range append(append([]float64{}, a...), b...) {
		buf = binary.LittleEndian.AppendUint64(buf, math.Float64bits(v))
	}
	must(os.WriteFile(filepath.Join(dir, "vec.bin"), buf, 0o644))

	const nSmall = 1024
	smooth := make([]float64, 100000)
	for i := range smooth {
		smooth[i] = float64(i % 97)
	}
	search := make([]float64, 100000)
	for i := range search {
		search[i] = float64(i)
	}
	fresh := g.SmoothFresh(smooth)
	var stencil float64 // the sum of every output, in order: one number for n - 2 of them
	for i := 1; i < len(smooth)-1; i++ {
		stencil += fresh[i]
	}
	tok := g.TokDoc(64)
	tree := g.TokDocInts(g.TokDoc(20))
	lines := []string{
		hex("dot1024", g.DotRange(a[:nSmall], b[:nSmall])),
		hex("dot65536", g.DotRange(a, b)),
		hex("centroid", g.CentroidSumRef(a, b)),
		hex("sum", g.SumF64(a)),
		hex("stencil", stencil),
		fmt.Sprintf("search5 %d", g.FindFirstRef(search, 5)),
		fmt.Sprintf("search99998 %d", g.FindFirstRef(search, 99998)),
		fmt.Sprintf("tokens %d %d", len(tok), g.TokBytes(tok)),
		fmt.Sprintf("tree %d %d", len(tree), g.TreeFlat(tree)),
	}
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	must(os.WriteFile(filepath.Join(dir, "answers.txt"), []byte(out), 0o644))
}

func hex(name string, v float64) string { return fmt.Sprintf("%s %016x", name, math.Float64bits(v)) }

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
