package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	authclearcmd "github.com/aaronhurt/vagaro-sync/internal/command/authclear"
	authlogincmd "github.com/aaronhurt/vagaro-sync/internal/command/authlogin"
	synccmd "github.com/aaronhurt/vagaro-sync/internal/command/sync"
	"github.com/aaronhurt/vagaro-sync/internal/platform"
	"github.com/aaronhurt/vagaro-sync/internal/state"
	"github.com/aaronhurt/vagaro-sync/internal/storage"
)

type runner interface {
	Run(context.Context, []string) error
}

type commandSpec struct {
	Name     string
	Synopsis string
	New      func() (runner, error)
}

var errUnknownCommand = errors.New("unknown command")

func commandSpecs() []commandSpec {
	return []commandSpec{
		{
			Name:     "auth-login",
			Synopsis: "Launch browser-assisted authentication and store the resulting session",
			New: func() (runner, error) {
				return authlogincmd.NewCommand(
					storage.NewKeychainStore(storage.DefaultKeychainService, storage.DefaultKeychainAccount),
				), nil
			},
		},
		{
			Name:     "auth-clear",
			Synopsis: "Remove any stored authentication bundle",
			New: func() (runner, error) {
				return authclearcmd.NewCommand(
					storage.NewKeychainStore(storage.DefaultKeychainService, storage.DefaultKeychainAccount),
				), nil
			},
		},
		{
			Name:     "sync",
			Synopsis: "Fetch Vagaro appointments and synchronize Calendar.app",
			New: func() (runner, error) {
				statePath, err := platform.DefaultStatePath()
				if err != nil {
					return nil, err
				}

				return synccmd.NewCommand(
					storage.NewKeychainStore(storage.DefaultKeychainService, storage.DefaultKeychainAccount),
					state.NewFileStore(statePath),
				), nil
			},
		},
	}
}

func findCommandSpec(name string) (commandSpec, error) {
	for _, command := range commandSpecs() {
		if command.Name == name {
			return command, nil
		}
	}

	return commandSpec{}, fmt.Errorf("%w %q", errUnknownCommand, name)
}

func writeUsage(w io.Writer) error {
	if _, err := fmt.Fprintln(w, "Usage: vagaro-sync <command>"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "Commands:"); err != nil {
		return err
	}

	for _, command := range commandSpecs() {
		if _, err := fmt.Fprintf(w, "  %-12s %s\n", command.Name, command.Synopsis); err != nil {
			return err
		}
	}

	return nil
}
