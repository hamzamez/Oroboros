//go:build ignore

// How text reaches a Windows console (win32org-2026-10-05 §2). Writes one line
// of non-ASCII text to standard output by one of the console's routes, and says
// on standard error what the handle is and which code page is in force.
//
//	go run console.go MODE [conout]     MODE: file | cp-utf8 | wide; conout writes to CONOUT$
package main

import (
	"fmt"
	"os"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	k32                = syscall.NewLazyDLL("kernel32.dll")
	getConsoleMode     = k32.NewProc("GetConsoleMode")
	getConsoleOutputCP = k32.NewProc("GetConsoleOutputCP")
	setConsoleOutputCP = k32.NewProc("SetConsoleOutputCP")
	writeConsoleW      = k32.NewProc("WriteConsoleW")
)

const text = "café € 中 \U0001F600\n"

func main() {
	h, _ := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if len(os.Args) > 2 && os.Args[2] == "conout" {
		// The console's own output device, whatever the inherited handles are.
		name, _ := syscall.UTF16PtrFromString("CONOUT$")
		h, _ = syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, syscall.FILE_SHARE_WRITE,
			nil, syscall.OPEN_EXISTING, 0, 0)
	}
	var mode uint32
	isConsole, _, _ := getConsoleMode.Call(uintptr(h), uintptr(unsafe.Pointer(&mode)))
	cp, _, _ := getConsoleOutputCP.Call()
	fmt.Fprintf(os.Stderr, "[console=%v mode=%#x outputCP=%d] ", isConsole != 0, mode, cp)
	switch os.Args[1] {
	case "file", "cp-utf8":
		if os.Args[1] == "cp-utf8" {
			setConsoleOutputCP.Call(65001)
		}
		b := []byte(text)
		var n uint32
		err := syscall.WriteFile(h, b, &n, nil)
		fmt.Fprintf(os.Stderr, "WriteFile %d of %d bytes, err=%v\n", n, len(b), err)
	case "wide":
		u := utf16.Encode([]rune(text))
		var n uint32
		r, _, err := writeConsoleW.Call(uintptr(h), uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)),
			uintptr(unsafe.Pointer(&n)), 0)
		fmt.Fprintf(os.Stderr, "WriteConsoleW ok=%v %d of %d units, err=%v\n", r != 0, n, len(u), err)
	}
}
