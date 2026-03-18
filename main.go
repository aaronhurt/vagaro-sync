// Package main provides the vagaro-sync CLI entrypoint.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
)

var buildVersion = ""

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "vagaro-sync: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	root := flag.NewFlagSet("vagaro-sync", flag.ContinueOnError)
	root.SetOutput(os.Stderr)
	root.Usage = func() {
		if err := writeUsage(os.Stderr); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "vagaro-sync: %v\n", err)
		}
	}

	if err := root.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}

		return err
	}

	if root.NArg() == 0 {
		return writeUsage(os.Stdout)
	}

	name := root.Arg(0)
	command, ok, err := newCommand(name)
	if err != nil {
		return err
	}

	if !ok {
		if err := writeUsage(os.Stderr); err != nil {
			return err
		}

		return fmt.Errorf("unknown command %q", name)
	}

	return command.Run(ctx, root.Args()[1:])
}
