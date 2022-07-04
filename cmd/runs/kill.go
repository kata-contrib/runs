package main

import (
	sctx "context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/urfave/cli"
	"golang.org/x/sys/unix"

	"github.com/containerd/containerd/namespaces"
	"github.com/kata-contrib/runs/pkg/shim"
)

var killCommand = cli.Command{
	Name:  "kill",
	Usage: "kill sends the specified signal (default: SIGTERM) to the container's init process",
	ArgsUsage: `<container-id> [signal]

Where "<container-id>" is the name for the instance of the container and
"[signal]" is the signal to be sent to the init process.

EXAMPLE:
For example, if the container id is "ubuntu01" the following will send a "KILL"
signal to the init process of the "ubuntu01" container:

       # runc kill ubuntu01 KILL`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "send the specified signal to all processes inside the container",
		},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, minArgs); err != nil {
			return err
		}
		if err := checkArgs(context, 2, maxArgs); err != nil {
			return err
		}

		ctx := namespaces.WithNamespace(sctx.Background(), "default")

		path, err := os.Getwd()
		if err != nil {
			return err
		}
		bundle := &shim.Bundle{
			ID:        "abc",
			Path:      path,
			Namespace: "default",
		}

		s, err := shim.LoadShim(ctx, bundle, func() {})
		if err != nil {
			return err
		}
		state, err := s.State(ctx)
		if err != nil {
			// return err
		}

		// FIXME check state.

		fmt.Printf("state error: %+v\n", err)
		fmt.Printf("state: %+v\n", state)

		// step 1: kill
		err = s.Kill(ctx, 9, false)
		if err != nil {
			// return err
		}
		fmt.Printf("kill error: %+v\n", err)

		state, err = s.State(ctx)
		if err != nil {
			// return err
		}

		wait, err := s.Wait(ctx)
		if err != nil {
			// return err
		}

		fmt.Printf("wait error: %+v\n", err)
		fmt.Printf("wait: %+v\n", wait)

		err = s.Shutdown(ctx)
		if err != nil {
			// return err
		}
		fmt.Printf("Shutdown error: %+v\n", err)

		state, err = s.State(ctx)
		if err != nil {
			// return err
		}
		fmt.Printf("state error: %+v\n", err)
		fmt.Printf("state: %+v\n", state)

		if err != nil {
			return err
		}

		// step 2: delete shim
		exit, err := s.Delete(ctx, false, func(ctx sctx.Context, id string) {})
		fmt.Printf("exit error: %+v\n", err)
		fmt.Printf("exit: %+v\n", exit)

		if err != nil {
			return err
		}

		return nil
		// sigstr := context.Args().Get(1)
		// if sigstr == "" {
		// 	sigstr = "SIGTERM"
		// }

		// signal, err := parseSignal(sigstr)
		// if err != nil {
		// 	return err
		// }
		// return container.Signal(signal, context.Bool("all"))
	},
}

func parseSignal(rawSignal string) (unix.Signal, error) {
	s, err := strconv.Atoi(rawSignal)
	if err == nil {
		return unix.Signal(s), nil
	}
	sig := strings.ToUpper(rawSignal)
	if !strings.HasPrefix(sig, "SIG") {
		sig = "SIG" + sig
	}
	signal := unix.SignalNum(sig)
	if signal == 0 {
		return -1, fmt.Errorf("unknown signal %q", rawSignal)
	}
	return signal, nil
}
