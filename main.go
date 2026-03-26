// Package main provides the vagaro-sync CLI entrypoint.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "vagaro-sync: %v\n", err)
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
	command, err := findCommandSpec(name)
	if err != nil {
		if errors.Is(err, errUnknownCommand) {
			if usageErr := writeUsage(os.Stderr); usageErr != nil {
				return usageErr
			}
		}

		return err
	}

	instance, err := command.New()
	if err != nil {
		return err
	}

	return instance.Run(ctx, root.Args()[1:])
}
