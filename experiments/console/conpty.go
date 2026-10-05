//go:build ignore

// Run a command attached to a REAL console, a pseudo-console (ConPTY), and print
// what the console shows: ConPTY renders its buffer back as UTF-8 with VT
// sequences, so the text below is what a user's terminal would display, after
// the console has decoded every write by its own rules (win32org-2026-10-05 §2).
//
//	go run conpty.go COMMAND LINE…
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	k32                               = syscall.NewLazyDLL("kernel32.dll")
	createPseudoConsole               = k32.NewProc("CreatePseudoConsole")
	closePseudoConsole                = k32.NewProc("ClosePseudoConsole")
	initializeProcThreadAttributeList = k32.NewProc("InitializeProcThreadAttributeList")
	updateProcThreadAttribute         = k32.NewProc("UpdateProcThreadAttribute")
)

const (
	procThreadAttributePseudoconsole = 0x00020016
	extendedStartupinfoPresent       = 0x00080000
)

type startupInfoEx struct {
	syscall.StartupInfo
	attrs uintptr
}

func must(r uintptr, err error, what string) {
	if r != 0 {
		fmt.Fprintf(os.Stderr, "%s: %#x %v\n", what, r, err)
		os.Exit(1)
	}
}

func main() {
	var inR, inW, outR, outW syscall.Handle
	if err := syscall.CreatePipe(&inR, &inW, nil, 0); err != nil {
		panic(err)
	}
	if err := syscall.CreatePipe(&outR, &outW, nil, 0); err != nil {
		panic(err)
	}
	var hpc uintptr
	size := uintptr(120) | uintptr(30)<<16
	r, _, err := createPseudoConsole.Call(size, uintptr(inR), uintptr(outW), 0, uintptr(unsafe.Pointer(&hpc)))
	must(r, err, "CreatePseudoConsole")

	var n uintptr
	initializeProcThreadAttributeList.Call(0, 1, 0, uintptr(unsafe.Pointer(&n)))
	list := make([]byte, n)
	ok, _, err := initializeProcThreadAttributeList.Call(uintptr(unsafe.Pointer(&list[0])), 1, 0, uintptr(unsafe.Pointer(&n)))
	if ok == 0 {
		panic(err)
	}
	ok, _, err = updateProcThreadAttribute.Call(uintptr(unsafe.Pointer(&list[0])), 0, procThreadAttributePseudoconsole,
		hpc, unsafe.Sizeof(hpc), 0, 0)
	if ok == 0 {
		panic(err)
	}
	var si startupInfoEx
	si.Cb = uint32(unsafe.Sizeof(si))
	si.attrs = uintptr(unsafe.Pointer(&list[0]))
	var pi syscall.ProcessInformation
	cmd, _ := syscall.UTF16PtrFromString(strings.Join(os.Args[1:], " "))
	if err := syscall.CreateProcess(nil, cmd, nil, nil, false, extendedStartupinfoPresent, nil, nil,
		&si.StartupInfo, &pi); err != nil {
		panic(err)
	}
	syscall.CloseHandle(inR)
	syscall.CloseHandle(outW)

	var out []byte
	done := make(chan bool)
	go func() {
		buf := make([]byte, 4096)
		for {
			var got uint32
			if err := syscall.ReadFile(outR, buf, &got, nil); err != nil || got == 0 {
				break
			}
			out = append(out, buf[:got]...)
		}
		done <- true
	}()
	syscall.WaitForSingleObject(pi.Process, syscall.INFINITE)
	time.Sleep(300 * time.Millisecond) // let the console render the last write
	closePseudoConsole.Call(hpc)
	<-done
	vt := regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]|\x1b\][^\x07]*\x07|\x1b[()][A-Z0-9]`)
	for _, l := range strings.Split(vt.ReplaceAllString(string(out), ""), "\n") {
		if l = strings.TrimRight(l, "\r "); l != "" {
			fmt.Println(l)
		}
	}
}
