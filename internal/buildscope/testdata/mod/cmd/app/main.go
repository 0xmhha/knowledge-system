package main

import (
	"fmt"

	"example.com/bs/lib"
	"example.com/ext"
)

func main() { fmt.Println(lib.Name(), ext.Hello()) }
