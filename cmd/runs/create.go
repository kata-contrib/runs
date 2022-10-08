package main

import (
	sctx "context"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/protobuf"
	"github.com/containerd/containerd/runtime"
	"github.com/kata-contrib/runs/pkg/shim"
	"golang.org/x/sys/unix"

	securejoin "github.com/cyphar/filepath-securejoin"
)

type stdinCloser struct {
	stdin  *os.File
	closer func()
}

func (s *stdinCloser) Read(p []byte) (int, error) {
	n, err := s.stdin.Read(p)
	if err == io.EOF {
		if s.closer != nil {
			s.closer()
		}
	}
	return n, err
}

var createCommand = cli.Command{
	Name:  "create",
	Usage: "create a container",
	ArgsUsage: `<container-id>

Where "<container-id>" is your name for the instance of the container that you
are starting. The name you provide for the container instance must be unique on
your host.`,
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

		containerRoot, err := securejoin.SecureJoin("/run/runs", id)
		if err != nil {
			return err
		}
		os.Stat(containerRoot)
		os.MkdirAll(containerRoot, 0711)
		os.Chown(containerRoot, unix.Geteuid(), unix.Getegid())

		ctx := namespaces.WithNamespace(sctx.Background(), "default")

		shimManager, err := shim.NewShimManager(ctx, &shim.ManagerConfig{
			State:        "/var/run/runs",
			Address:      "/run/containerd/containerd.sock",
			TTRPCAddress: "/run/containerd/containerd.sock.ttrpc",
		})
		if err != nil {
			return err
		}
		spec, err := loadSpec(specConfig)
		if err != nil {
			return err
		}

		specAny, err := protobuf.MarshalAnyToProto(spec)
		if err != nil {
			return err
		}

		// stdinC := &stdinCloser{
		// 	stdin: os.Stdin,
		// }

		// ioOpts := []cio.Opt{cio.WithFIFODir(context.String("fifo-dir"))}
		// ioCreator := cio.NewCreator(append([]cio.Opt{cio.WithStreams(stdinC, os.Stdout, os.Stderr)}, ioOpts...)...)

		// i, err := ioCreator(id)
		// cfg := i.Config()

		opts := runtime.CreateOpts{
			Spec: specAny,
			// IO: runtime.IO{
			// 	Stdin:    cfg.Stdin,
			// 	Stdout:   cfg.Stdout,
			// 	Stderr:   cfg.Stderr,
			// 	Terminal: cfg.Terminal,
			// },
		}

		opts.Runtime = "io.containerd.kata.v2"

		taskManager := shim.NewTaskManager(shimManager)
		taskManager.Create(ctx, id, opts)
		if err != nil {
			return err
		}

		return nil
	},
}
