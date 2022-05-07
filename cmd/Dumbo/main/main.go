package main

import (
	"fmt"
)

func main() {
	m := map[int]int{}
	m[0] = 1

	go func() {
		for {
			fmt.Println(m[0])
		}

	}()
	go func() {
		for {
			m[0] = 2
			//fmt.Println(m[0] + 1)
		}
	}()

	fmt.Println("yes")
	for {

	}
}
