package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/reogac/utils/oam/client"
)

func main() {
	var (
		version = flag.Bool("version", false, "Show version information")
		noColor = flag.Bool("no-color", false, "Disable color output")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *version {
		fmt.Println("Test_Monitor CLI Client v1.0.0")
		os.Exit(0)
	}

	cli := client.NewClient(nil, nil)

	if *noColor {
		fmt.Println("Warning: no-color flag is set but not yet implemented")
	}

	cli.Run()
}
