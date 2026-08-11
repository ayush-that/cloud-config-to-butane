// Command cc2butane converts a cloud-init #cloud-config document into a Flatcar Butane config.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ayush-that/cloud-config-to-butane/internal/transpile"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "cc2butane: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	opts := transpile.Options{Warn: os.Stderr}
	flag.BoolVar(&opts.Strict, "strict", false, "turn runcmd/bootcmd into hard errors")
	flag.BoolVar(&opts.WarnUnsupported, "warn-unsupported", false, "warn and drop unsupported keys instead of failing")
	flag.Parse()

	input, err := readInput(flag.Arg(0))
	if err != nil {
		return err
	}
	out, err := transpile.Transpile(input, opts)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(out)
	return err
}

func readInput(path string) ([]byte, error) {
	if path == "" || path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}
