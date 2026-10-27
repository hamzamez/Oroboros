// Hand-written references for examples/hash (hashbench-2026-10-27): FNV-1a 32
// and 64 as hash/fnv writes them, CRC-32 table-driven as hash/crc32's
// simpleUpdate, and the library itself for context.
//
//	go run . MODE FILE    MODE: fnv | crc32 | lib
package main

import (
	"fmt"
	"hash/crc32"
	"hash/fnv"
	"os"
)

func main() {
	b, err := os.ReadFile(os.Args[2])
	if err != nil {
		panic(err)
	}
	switch os.Args[1] {
	case "fnv":
		h32 := uint32(2166136261)
		for _, c := range b {
			h32 ^= uint32(c)
			h32 *= 16777619
		}
		h64 := uint64(14695981039346656037)
		for _, c := range b {
			h64 ^= uint64(c)
			h64 *= 1099511628211
		}
		fmt.Printf("%08x\n%016x\n", h32, h64)
	case "crc32":
		var tab [256]uint32
		for i := range tab {
			c := uint32(i)
			for j := 0; j < 8; j++ {
				if c&1 == 1 {
					c = 0xEDB88320 ^ c>>1
				} else {
					c >>= 1
				}
			}
			tab[i] = c
		}
		crc := ^uint32(0)
		for _, v := range b {
			crc = tab[byte(crc)^v] ^ crc>>8
		}
		fmt.Printf("%08x\n", ^crc)
	case "lib":
		h32, h64 := fnv.New32a(), fnv.New64a()
		h32.Write(b)
		h64.Write(b)
		fmt.Printf("%08x\n%016x\n%08x\n", h32.Sum32(), h64.Sum64(), crc32.ChecksumIEEE(b))
	}
}
