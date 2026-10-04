//go:build ignore

// Memory per idle process: K goroutines, each blocked receiving on its own
// buffered channel (the native lowering's process and mailbox).
package main

import (
	"fmt"
	"runtime"
)

func main() {
	const k = 100000
	var a, b runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&a)
	chans := make([]chan int, k)
	for i := range chans {
		chans[i] = make(chan int, 16)
		go func(c chan int) { <-c }(chans[i])
	}
	runtime.Gosched()
	runtime.GC()
	runtime.ReadMemStats(&b)
	fmt.Printf("go native: %d bytes per idle process (stack %d, heap %d)\n",
		(b.Sys-a.Sys)/k, (b.StackInuse-a.StackInuse)/k, (b.HeapAlloc-a.HeapAlloc)/k)
	runtime.KeepAlive(chans)
}
