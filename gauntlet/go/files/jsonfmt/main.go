// jsonfmt: the pretty-printer's layout in one streaming pass, as a person
// writes it in Go: a bufio.Writer, the same rule as examples/io/jsonfmt.oro.
package main

import (
	"bufio"
	"fmt"
	"os"
)

func numeric(c byte) bool {
	return c >= '0' && c <= '9' || c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E'
}
func alpha(c byte) bool { return c >= 'a' && c <= 'z' }

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: jsonfmt FILE")
		return
	}
	s, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Println("jsonfmt: cannot read that file")
		return
	}
	if len(s) >= 1<<16 {
		fmt.Println("jsonfmt: file is larger than this tool accepts")
		return
	}
	w := bufio.NewWriterSize(os.Stdout, 1<<16)
	defer w.Flush()
	nl := func(d int) {
		w.WriteByte('\n')
		for k := 0; k < 2*d; k++ {
			w.WriteByte(' ')
		}
	}
	i, d, fresh := 0, 0, false
	for i < len(s) && d < 64 {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '{' || c == '[':
			if fresh {
				nl(d)
			}
			w.WriteByte(c)
			i, d, fresh = i+1, d+1, true
		case c == '}' || c == ']':
			if d > 0 {
				d--
			}
			if !fresh {
				nl(d)
			}
			w.WriteByte(c)
			i, fresh = i+1, false
		case c == ',':
			w.WriteByte(',')
			nl(d)
			i, fresh = i+1, false
		case c == ':':
			w.WriteString(": ")
			i++
		case c == '"' || numeric(c) || alpha(c):
			j := i + 1
			switch {
			case c == '"':
				for j < len(s) {
					if s[j] == '\\' {
						if j+1 >= len(s) {
							j++
							break
						}
						j += 2
						continue
					}
					if s[j] == '"' {
						j++
						break
					}
					j++
				}
			case numeric(c):
				for j < len(s) && numeric(s[j]) {
					j++
				}
			default:
				for j < len(s) && alpha(s[j]) {
					j++
				}
			}
			if j > len(s) {
				j = len(s)
			}
			if fresh {
				nl(d)
			}
			w.Write(s[i:j])
			i, fresh = j, false
		default:
			i++
		}
	}
	w.WriteByte('\n')
}
