package main

import (
	"fmt"
)

func main() {
	//	a := []byte{'\001', '\002', '\100'}
	//	var i int = int(a[2])

	//	fmt.Println(a)
	//	fmt.Println("A: ", i)

	//	a := make([]int, 10)
	//	a[0] = 10
	//	fmt.Println("Len A: ", len(a))

	//	a = a[:9]
	//	fmt.Println("A: ", a)
	//	fmt.Println("Len A: ", len(a))

	//	a := make([]int, 0)
	//	fmt.Println("A Len: ", len(a))
	//	fmt.Println("A Cap: ", cap(a))

	//	a = append(a, 1)
	//	fmt.Println("A Len: ", len(a))
	//	fmt.Println("A Cap: ", cap(a))

	a := []string{"hello", "hell1"}
	fmt.Println("Last Element: ", a[len(a)-1])
}
