package main

import (
	sctx "context"
	"errors"
	"fmt"
	"os"

	"github.com/containerd/containerd/namespaces"
	"github.com/opencontainers/runc/libcontainer"

	//	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/errdefs"
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
		// if err := checkArgs(context, 1, exactArgs); err != nil {
		// 	return err
		// }
		var (
			id  string
			ref string
		//      config = context.IsSet("config")
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
			// return fmt.Errorf("container id must be provided: %w", errdefs.ErrInvalidArgument)
		}
		// container, err := getContainer(context)
		// if err != nil {
		// 	return err
		// }
		// status, err := container.Status()
		// if err != nil {
		// 	return err
		// }
		status := libcontainer.Created
		switch status {
		case libcontainer.Created:

			ctx := namespaces.WithNamespace(sctx.Background(), "default")

			//			id := context.GlobalString("id")

			fmt.Printf("id: %+v\n", id)

			path, err := os.Getwd()
			if err != nil {
				return err
			}

			fmt.Printf("id: %+v\n", id)
			bundle := &shim.Bundle{
				ID:        id,
				Path:      path,
				Namespace: "default",
			}

			fmt.Printf("id: %+v\n", id)
			task, err := shim.LoadShim(ctx, bundle, func() {})
			if err != nil {
				return err
			}
			state, err := task.State(ctx)
			if err != nil {
				// return err
			}

			// FIXME check state.

			fmt.Printf("state error: %+v\n", err)
			fmt.Printf("state: %+v\n", state)

			// task, err := findTask(context)
			if err != nil {
				return err
			}

			err = task.Start(ctx)
			if err != nil {
				return err
			}
			pid, err := task.PID(ctx)
			if err != nil {
				return err
			}
			fmt.Printf("pid %d\n", pid)

			return nil
		case libcontainer.Stopped:
			return errors.New("cannot start a container that has stopped")
		case libcontainer.Running:
			return errors.New("cannot start an already running container")
		default:
			return fmt.Errorf("cannot start a container in the %s state", status)
		}
	},
}
