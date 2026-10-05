//go:build ignore

// The ring: N processes, T tokens, each token passed M times round the ring.
// Two lowerings of one process (receive an integer, send it to the next):
//
//	native    — a goroutine per process, a buffered channel as its mailbox;
//	stackless — a step function per process, run by one scheduler over a run
//	            queue, each mailbox a fixed ring buffer (no stack per process).
//
// Usage: go run ring.go MODE N T M
package main

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"time"
)

const capMbox = 16

func native(n, t, m int) time.Duration {
	chans := make([]chan int, n)
	for i := range chans {
		chans[i] = make(chan int, capMbox)
	}
	done := make(chan struct{})
	for i := 1; i < n; i++ {
		go func(in, out chan int) {
			for v := range in {
				out <- v
			}
		}(chans[i], chans[(i+1)%n])
	}
	start := time.Now()
	go func() {
		in, out := chans[0], chans[1%n]
		for k := 0; k < t; k++ {
			out <- m
		}
		live := t
		for v := range in {
			if v == 1 {
				live--
				if live == 0 {
					close(done)
					return
				}
				continue
			}
			out <- v - 1
		}
	}()
	<-done
	el := time.Since(start)
	for i := 1; i < n; i++ {
		close(chans[i])
	}
	return el
}

type actor struct {
	mbox       [capMbox]int
	head, size int
	next       int
}

func stackless(n, t, m int) time.Duration {
	as := make([]actor, n)
	for i := range as {
		as[i].next = (i + 1) % n
	}
	runq := make([]int, n)
	rh, rs := 0, 0
	push := func(a, v int) {
		x := &as[a]
		x.mbox[(x.head+x.size)%capMbox] = v
		x.size++
		if x.size == 1 {
			runq[(rh+rs)%n] = a
			rs++
		}
	}
	start := time.Now()
	for k := 0; k < t; k++ {
		push(1%n, m)
	}
	live := t
	for rs > 0 {
		a := runq[rh]
		rh = (rh + 1) % n
		rs--
		x := &as[a]
		for x.size > 0 { // drain: the step function, once per message
			v := x.mbox[x.head]
			x.head = (x.head + 1) % capMbox
			x.size--
			if a == 0 {
				if v == 1 {
					live--
					continue
				}
				v--
			}
			push(x.next, v)
		}
	}
	if live != 0 {
		panic("lost a token")
	}
	return time.Since(start)
}

func main() {
	mode := os.Args[1]
	n, _ := strconv.Atoi(os.Args[2])
	t, _ := strconv.Atoi(os.Args[3])
	m, _ := strconv.Atoi(os.Args[4])
	run := native
	if mode == "stackless" {
		run = stackless
	}
	run(n, t, m/10) // warm up
	var ds []float64
	for r := 0; r < 7; r++ {
		runtime.GC()
		el := run(n, t, m)
		ds = append(ds, float64(el.Nanoseconds())/float64(n*t*m))
	}
	sort.Float64s(ds)
	fmt.Printf("go %s N=%d T=%d M=%d: %.1f ns/hop (min %.1f max %.1f)\n", mode, n, t, m, ds[3], ds[0], ds[6])
}
