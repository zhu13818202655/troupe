package main

import (
	"fmt"
	"os"

	"github.com/zhu13818202655/troupe/cmd/astonish"
)

func main() {
	if err := astonish.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
