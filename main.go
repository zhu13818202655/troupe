package main

import (
	"fmt"
	"os"

	"github.com/zhu13818202655/troupe/cmd/troupe"
)

func main() {
	if err := troupe.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
