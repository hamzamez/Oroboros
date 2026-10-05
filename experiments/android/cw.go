//go:build ignore

// A class file written by hand: version 50 (Java 6), no StackMapTable, one
// loop. Does D8 take it, and does ART run it? (android-2026-10-05 §3)
// go run cw.go Main.class && java -cp . Main; then d8 and dalvikvm64.
package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"strconv"
)

type pool struct {
	b bytes.Buffer
	n uint16
	m map[string]uint16
}

func (p *pool) add(key string, w func(*bytes.Buffer)) uint16 {
	if i, ok := p.m[key]; ok {
		return i
	}
	p.n++
	w(&p.b)
	p.m[key] = p.n
	return p.n
}
func u2(b *bytes.Buffer, v uint16) { binary.Write(b, binary.BigEndian, v) }
func u4(b *bytes.Buffer, v uint32) { binary.Write(b, binary.BigEndian, v) }
func (p *pool) utf8(s string) uint16 {
	return p.add("U"+s, func(b *bytes.Buffer) { b.WriteByte(1); u2(b, uint16(len(s))); b.WriteString(s) })
}
func (p *pool) class(s string) uint16 {
	i := p.utf8(s)
	return p.add("C"+s, func(b *bytes.Buffer) { b.WriteByte(7); u2(b, i) })
}
func (p *pool) nat(n, t string) uint16 {
	a, c := p.utf8(n), p.utf8(t)
	return p.add("N"+n+t, func(b *bytes.Buffer) { b.WriteByte(12); u2(b, a); u2(b, c) })
}
func (p *pool) ref(tag byte, cl, n, t string) uint16 {
	c, nt := p.class(cl), p.nat(n, t)
	return p.add(strconv.Itoa(int(tag))+cl+n+t, func(b *bytes.Buffer) { b.WriteByte(tag); u2(b, c); u2(b, nt) })
}

func main() {
	p := &pool{m: map[string]uint16{}}
	this, super := p.class("Main"), p.class("java/lang/Object")
	out := p.ref(9, "java/lang/System", "out", "Ljava/io/PrintStream;")
	println := p.ref(10, "java/io/PrintStream", "println", "(I)V")
	mname, mdesc, code := p.utf8("main"), p.utf8("([Ljava/lang/String;)V"), p.utf8("Code")

	// s = 0; i = 0; while (i < 10) { s += i; i++ }; System.out.println(s)
	var c bytes.Buffer
	c.Write([]byte{0x03, 0x3c, 0x03, 0x3d}) // iconst_0 istore_1 iconst_0 istore_2
	loop := c.Len()
	c.Write([]byte{0x1c, 0x10, 10}) // iload_2 bipush 10
	ifAt := c.Len()
	c.Write([]byte{0xa2, 0, 0})                   // if_icmpge END (patched)
	c.Write([]byte{0x1b, 0x1c, 0x60, 0x3c})       // iload_1 iload_2 iadd istore_1
	c.Write([]byte{0x84, 2, 1})                   // iinc 2 1
	gotoAt := c.Len()
	c.Write([]byte{0xa7, 0, 0}) // goto LOOP (patched)
	end := c.Len()
	c.WriteByte(0xb2) // getstatic out
	u2(&c, out)
	c.WriteByte(0x1b) // iload_1
	c.WriteByte(0xb6) // invokevirtual println
	u2(&c, println)
	c.WriteByte(0xb1) // return
	bs := c.Bytes()
	binary.BigEndian.PutUint16(bs[ifAt+1:], uint16(end-ifAt))
	binary.BigEndian.PutUint16(bs[gotoAt+1:], uint16(int16(loop-gotoAt)))

	var f bytes.Buffer
	u4(&f, 0xCAFEBABE)
	u2(&f, 0)
	u2(&f, 50) // Java 6: no StackMapTable required
	u2(&f, p.n+1)
	f.Write(p.b.Bytes())
	u2(&f, 0x0021) // public super
	u2(&f, this)
	u2(&f, super)
	u2(&f, 0) // interfaces
	u2(&f, 0) // fields
	u2(&f, 1) // methods
	u2(&f, 0x0009) // public static
	u2(&f, mname)
	u2(&f, mdesc)
	u2(&f, 1) // attributes: Code
	u2(&f, code)
	u4(&f, uint32(2+2+4+len(bs)+2+2))
	u2(&f, 2) // max_stack
	u2(&f, 3) // max_locals
	u4(&f, uint32(len(bs)))
	f.Write(bs)
	u2(&f, 0) // exception table
	u2(&f, 0) // code attributes: none, so no StackMapTable
	u2(&f, 0) // class attributes
	os.WriteFile(os.Args[1], f.Bytes(), 0o644)
}
