package main

import "fmt"

type Person struct {
	Name string
	Age  int
}

func (p Person) InfoPrint() {
	fmt.Printf("Hello! My name is %s and I am %d years old.", p.Name, p.Age)
}

func main() {
	person := Person{
		Name: "Deyan",
		Age:  21,
	}

	person.InfoPrint()
}
