// The reference for examples/pe/pe.oro: the same report from Go's debug/pe and
// from Windows' own checksum, imagehlp's MapFileAndCheckSumW.
package main

import (
	"debug/pe"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: ref FILE")
		return
	}
	f, err := pe.Open(os.Args[1])
	if err != nil {
		fmt.Println("ref:", err)
		return
	}
	defer f.Close()
	h := f.FileHeader
	fmt.Printf("machine %04x\nsections %04x\ntimestamp %08x\ncharacteristics %04x\n",
		h.Machine, h.NumberOfSections, h.TimeDateStamp, h.Characteristics)
	switch o := f.OptionalHeader.(type) {
	case *pe.OptionalHeader64:
		fmt.Printf("magic %04x\nentry %08x\nimagebase %016x\n", o.Magic, o.AddressOfEntryPoint, o.ImageBase)
	case *pe.OptionalHeader32:
		fmt.Printf("magic %04x\nentry %08x\nimagebase %016x\n", o.Magic, o.AddressOfEntryPoint, o.ImageBase)
	}
	// THE RAW NAME, eight bytes NUL-padded, as the program prints it: debug/pe
	// resolves a long name, "/4", through the string table.
	raw, _ := os.ReadFile(os.Args[1])
	lfanew := int(raw[0x3c]) | int(raw[0x3d])<<8 | int(raw[0x3e])<<16 | int(raw[0x3f])<<24
	table := lfanew + 24 + int(h.SizeOfOptionalHeader)
	for i, s := range f.Sections {
		name := ""
		for _, c := range raw[table+40*i : table+40*i+8] {
			if c == 0 {
				break
			}
			name += string(rune(c))
		}
		fmt.Printf("section %s %08x %08x %08x %08x %08x\n", name, s.VirtualSize, s.VirtualAddress,
			s.Size, s.Offset, s.Characteristics)
	}
	var stored, computed uint32
	p, _ := syscall.UTF16PtrFromString(os.Args[1])
	proc := syscall.NewLazyDLL("imagehlp.dll").NewProc("MapFileAndCheckSumW")
	r, _, _ := proc.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&stored)), uintptr(unsafe.Pointer(&computed)))
	if r != 0 {
		fmt.Println("ref: MapFileAndCheckSumW failed", r)
		return
	}
	fmt.Printf("checksum %08x %08x\n", stored, computed)
}
