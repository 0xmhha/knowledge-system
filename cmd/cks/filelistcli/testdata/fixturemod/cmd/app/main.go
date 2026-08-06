package main

import (
	_ "embed"
	"fmt"

	"example.com/fixture/lib"
)

//go:embed asset.txt
var asset string

func main() { fmt.Println(lib.Answer(), asset) }
