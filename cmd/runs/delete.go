package main

import (
	sctx "context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/kata-contrib/runs/pkg/shim"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/urfave/cli"

	"golang.org/x/sys/unix"
)

func killContainer(container *libcontainer.Container) error {
	_ = container.Signal(unix.SIGKILL, false)
	for i := 0; i < 100; i++ {
		time.Sleep(100 * time.Millisecond)
		if err := container.Signal(unix.Signal(0), false); err != nil {
			destroy(container)
			return nil
		}
	}
	return errors.New("container init still running")
}

var deleteCommand = cli.Command{
	Name:  "delete",
	Usage: "delete any resources held by the container often used with detached container",
	ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container.

EXAMPLE:
For example, if the container id is "ubuntu01" and runc list currently shows the
status of "ubuntu01" as "stopped" the following will delete resources held for
"ubuntu01" removing "ubuntu01" from the runc list of containers:

       # runc delete ubuntu01`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "Forcibly deletes the container if it is still running (uses SIGKILL)",
		},
	},
	Action: func(context *cli.Context) error {
		var (
			id string
		)

		id = context.Args().First()
		if context.NArg() > 1 {
			return fmt.Errorf("with spec config file, only container id should be provided: %w", errdefs.ErrInvalidArgument)
		}

		if id == "" {
			return fmt.Errorf("container id must be provided: %w", errdefs.ErrInvalidArgument)
		}

		ctx := namespaces.WithNamespace(sctx.Background(), "default")

		path, err := os.Getwd()
		if err != nil {
			return err
		}
		bundle := &shim.Bundle{
			ID:        id,
			Path:      path,
			Namespace: "default",
		}

		s, err := shim.LoadShim(ctx, bundle, func() {})
		if err != nil {
			return err
		}

		_, err = s.Delete(ctx, false, func(ctx sctx.Context, id string) {})
		if err != nil {
			return err
		}

		path = filepath.Join(context.GlobalString("root"), id)
		if e := os.RemoveAll(path); e != nil {
			fmt.Fprintf(os.Stderr, "remove %s: %v\n", path, e)
		}

		return nil
	},
}
