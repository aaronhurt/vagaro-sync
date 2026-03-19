package main

import (
	"context"
	"fmt"
	"io"

	authclearcmd "github.com/aaronhurt/vagaro-sync/internal/command/authclear"
	authlogincmd "github.com/aaronhurt/vagaro-sync/internal/command/authlogin"
	synccmd "github.com/aaronhurt/vagaro-sync/internal/command/sync"
	versioncmd "github.com/aaronhurt/vagaro-sync/internal/command/version"
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
}

var availableCommands = []commandSpec{
	{
		Name:     "auth-login",
		Synopsis: "Launch browser-assisted authentication and store the resulting session",
	},
	{
		Name:     "auth-clear",
		Synopsis: "Remove any stored authentication bundle",
	},
	{
		Name:     "sync",
		Synopsis: "Fetch Vagaro appointments and synchronize Calendar.app",
	},
	{
		Name:     "version",
		Synopsis: "Print build version information",
	},
}

func newCommand(name string) (runner, bool, error) {
	switch name {
	case "auth-login":
		return authlogincmd.NewCommand(storage.NewKeychainStore(storage.DefaultKeychainService, storage.DefaultKeychainAccount)), true, nil
	case "auth-clear":
		return authclearcmd.NewCommand(storage.NewKeychainStore(storage.DefaultKeychainService, storage.DefaultKeychainAccount)), true, nil
	case "sync":
		statePath, err := platform.DefaultStatePath()
		if err != nil {
			return nil, true, err
		}

		return synccmd.NewCommand(
			storage.NewKeychainStore(storage.DefaultKeychainService, storage.DefaultKeychainAccount),
			state.NewFileStore(statePath),
		), true, nil
	case "version":
		return &versioncmd.Command{Version: buildVersion}, true, nil
	default:
		return nil, false, nil
	}
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

	for _, command := range availableCommands {
		if _, err := fmt.Fprintf(w, "  %-12s %s\n", command.Name, command.Synopsis); err != nil {
			return err
		}
	}

	return nil
}
