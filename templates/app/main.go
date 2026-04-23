package main

import (
	"fmt"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	fmt.Printf("Hello flake %s (%s)\n", version, commit)
}
