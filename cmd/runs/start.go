package main

import (
	sctx "context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/fifo"

	//	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/errdefs"
	"github.com/kata-contrib/runs/pkg/cio"
	"github.com/kata-contrib/runs/pkg/shim"
	"github.com/urfave/cli"
)

var startCommand = cli.Command{
	Name:  "start",
	Usage: "executes the user defined process in a created container",
	ArgsUsage: `<container-id>

Where "<container-id>" is your name for the instance of the container that you
are starting. The name you provide for the container instance must be unique on
your host.`,
	Description: `The start command executes the user defined process in a created container.`,
	Action: func(context *cli.Context) error {
		var (
			id  string
			ref string
		)

		fmt.Println("number: %w\n", context.NArg())

		if 1 == 1 {
			id = context.Args().First()
			if context.NArg() > 1 {
				return fmt.Errorf("with spec config file, only container id should be provided: %w", errdefs.ErrInvalidArgument)
			}
		} else {
			id = context.Args().Get(1)
			ref = context.Args().First()
			if ref == "" {
				return fmt.Errorf("image ref must be provided: %w", errdefs.ErrInvalidArgument)
			}
		}
		if id == "" {
			id = context.GlobalString("id")
			fmt.Println("number: %w\n", id)
		}

		s, _ := loadStates(context)

		path := ""
		for _, item := range s {
			if item.ID == id {
				path = item.Bundle
			}
		}

		ctx := namespaces.WithNamespace(sctx.Background(), "default")
		bundle := &shim.Bundle{
			ID:        id,
			Path:      path,
			Namespace: "default",
		}

		tasks, err := shim.LoadShim(ctx, bundle, func() {})
		if err != nil {
			return err
		}

		err = tasks.Start(ctx)
		if err != nil {
			return err
		}

		pid, err := tasks.PID(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("pid %d\n", pid)

		state, err := tasks.State(ctx)
		if err != nil {
			return err
		}

		if err := shim.SaveContainerState(ctx, id, state.Status, state.Pid); err != nil {
			return err
		}

		return nil
	},
}

func attachExistingIO(cfg cio.Config) *cio.FIFOSet {
	fifos := []string{
		cfg.Stdin,
		cfg.Stdout,
		cfg.Stderr,
	}
	closer := func() error {
		var (
			err  error
			dirs = map[string]struct{}{}
		)
		for _, f := range fifos {
			if isFifo, _ := fifo.IsFifo(f); isFifo {
				if rerr := os.Remove(f); err == nil {
					err = rerr
				}
				dirs[filepath.Dir(f)] = struct{}{}
			}
		}
		for dir := range dirs {
			// we ignore errors here because we don't
			// want to remove the directory if it isn't
			// empty
			os.Remove(dir)
		}
		return err
	}

	return cio.NewFIFOSet(cio.Config{
		Stdin:    cfg.Stdin,
		Stdout:   cfg.Stdout,
		Stderr:   cfg.Stderr,
		Terminal: cfg.Terminal,
	}, closer)
}
