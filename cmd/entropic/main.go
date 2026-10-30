package main

import (
	"fmt"
	"os"

	"github.com/umbralux1/entropic/internal/cli"
)

func main() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "entropic: %v\n", err)
		os.Exit(cli.ExitError)
	}
	os.Exit(cli.Run(os.Args[1:], dir, os.Stdout, os.Stderr))
}
