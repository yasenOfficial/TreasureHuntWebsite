//https://www.youtube.com/watch?v=P7dCWOjRwJA

package main

import (
	"fmt"
	"math"
)

func add(a int, b int) int { // function with parameters and return type
	return a + b
}

func sqrt(x float64) (float64, error) { // function with parameters and return type
	if x < 0 {
		return 0, Errorf("Negative number")
	}

	return math.Sqrt(x), nil // return multiple values  
	// panic("Tried to take square root of negative number") // panic make the program crash
}

type Person struct { // struct
	name string
	age int
}

type Shape interface { // interface
	area() float64
}

func printArea(s Shape) { // function with interface as parameter
	fmt.Println(s.area())
}

func (p Person) area() float64 { // method. with (p Person) we are attaching the method to the Person struct 
	fmt.Println("Hello")
}

func main() {
	fmt.Println("Hello, World!")

	var a int = 10 // one way to declare a variable
	b := 10 // another way to declare a variable

	if a == b {
		fmt.Println("a is equal to b")
	} else {
		fmt.Println("a is not equal to b")
	}

	var x [5]int // array
	x := [5]int{1, 2, 3, 4, 5} // another way to declare an array

	x := []int{1, 2, 3, 4, 5} // slice (dynamic array)
	x = append(x, 6) // append to slice

	a := make(map[string]int) // map (key-value pair)
	a["key"] = 10
	delete(a, "key") // delete key-value pair

	for i := 0; i < 5; i++ { // for loop
		fmt.Println(i)
	}

	p := Person{name: "John", age: 30} // struct

	x := 10
	b := &x // pointer
	fmt.Println(*b) // dereference pointer

}

// Tommorow:
// https://www.youtube.com/watch?v=8z8VrlVOIyM

