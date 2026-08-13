package main

import "fmt"

var a = 0.1
var b = 0.2

func main() {
	fmt.Println("const-folded  0.1+0.2 =", 0.1+0.2)
	fmt.Println("runtime       a+b     =", a+b)
	fmt.Println("const-folded  -0.0    =", -0.0)
	fmt.Println("runtime       -a*0    =", -a*0)
}
