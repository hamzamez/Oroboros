package main

import "fmt"

func main() {
	vals := []float64{1.0, 0.1 + 0.2, 1e8, 1e21, 1.0 / 3.0, -0.0}
	for _, v := range vals {
		fmt.Println(v)
	}
}
