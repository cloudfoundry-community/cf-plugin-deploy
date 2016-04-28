package main

import (
	"fmt"
	"os"
)

func main() {
	m, err := ParseManifest(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	d := &Deployer{ manifest: &m }
	d.Deploy()
}
