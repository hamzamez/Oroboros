// The hand-written reference for examples/num/ledger.oro: the same file format
// and limits, the subtotal in int64 cents, the tax 8.25% rounded to the cent
// half away from zero, written as a Go programmer would. Run:
//
//	go run . FILE           the three lines, as the ledger prints them
//	go run . -gen N FILE    write N random lines, prices with 0 to 2 places
package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strings"
)

func cents(c int64) string {
	sign := ""
	if c < 0 {
		sign, c = "-", -c
	}
	return fmt.Sprintf("%s%d.%02d", sign, c/100, c%100)
}

func main() {
	if len(os.Args) == 4 && os.Args[1] == "-gen" {
		var n int
		fmt.Sscanf(os.Args[2], "%d", &n)
		f, _ := os.Create(os.Args[3])
		w := bufio.NewWriter(f)
		r := rand.New(rand.NewSource(1))
		for i := 0; i < n; i++ {
			switch r.Intn(3) {
			case 0:
				fmt.Fprintf(w, "%d %d\n", r.Intn(10000), r.Intn(100))
			case 1:
				fmt.Fprintf(w, "%d.%d %d\n", r.Intn(10000), r.Intn(10), r.Intn(100))
			default:
				fmt.Fprintf(w, "%d.%02d %d\n", r.Intn(10000), r.Intn(100), r.Intn(100))
			}
		}
		w.Flush()
		f.Close()
		return
	}
	src, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Println("ledger: cannot read that file")
		return
	}
	var sum int64
	for _, line := range strings.Split(strings.TrimRight(string(src), "\n"), "\n") {
		var whole, frac, qty int64
		places := 0
		f := strings.Fields(line)
		p := f[0]
		if i := strings.IndexByte(p, '.'); i >= 0 {
			fmt.Sscanf(p[:i], "%d", &whole)
			fmt.Sscanf(p[i+1:], "%d", &frac)
			places = len(p) - i - 1
		} else {
			fmt.Sscanf(p, "%d", &whole)
		}
		fmt.Sscanf(f[1], "%d", &qty)
		c := whole * 100
		switch places {
		case 1:
			c += frac * 10
		case 2:
			c += frac
		}
		sum += c * qty
	}
	tax := (sum*825 + 5000) / 10000 // half away from zero, for sum ≥ 0
	fmt.Println("subtotal " + cents(sum))
	fmt.Println("tax      " + cents(tax))
	fmt.Println("total    " + cents(sum+tax))
}
