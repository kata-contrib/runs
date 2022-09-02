package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/containerd/containerd/runtime"
	"github.com/kata-contrib/runs/pkg/shim"
	"github.com/opencontainers/runc/libcontainer/user"
	"github.com/urfave/cli"
)

type containerState struct {
	// ID is the container ID
	ID string `json:"id"`
	// InitProcessPid is the init process id in the parent namespace
	InitProcessPid int `json:"pid"`
	// Status is the current status of the container, running, paused, ...
	Status string `json:"status"`
	// Bundle is the path on the filesystem to the bundle
	Bundle string `json:"bundle"`
	// Created is the unix timestamp for the creation time of the container in UTC
	Created time.Time `json:"created"`
	// The owner of the state directory (the owner of the container).
	Owner string `json:"owner"`
}

var listCommand = cli.Command{
	Name:  "list",
	Usage: "list the containers",
	Action: func(context *cli.Context) error {
		s, _ := loadStates(context)

		w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
		fmt.Fprint(w, "ID\tPID\tSTATUS\tBUNDLE\tCREATED\tOWNER\n")
		for _, item := range s {
			fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\t%s\n",
				item.ID,
				item.InitProcessPid,
				item.Status,
				item.Bundle,
				item.Created.Format(time.RFC3339Nano),
				item.Owner,
			)

			if err := w.Flush(); err != nil {
				return err
			}
		}
		return nil
	},
}

func loadStates(context *cli.Context) ([]containerState, error) {
	root := context.GlobalString("root")
	list, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && context.IsSet("root") {
			// Ignore non-existing default root directory
			// (no containers created yet).
			return nil, nil
		}
		// Report other errors, including non-existent custom --root.
		return nil, err
	}

	var s []containerState
	for _, item := range list {
		if !item.IsDir() {
			continue
		}
		st, err := item.Info()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Possible race with runc delete.
				continue
			}
			return nil, err
		}
		// This cast is safe on Linux.
		uid := st.Sys().(*syscall.Stat_t).Uid
		owner, err := user.LookupUid(int(uid))
		if err != nil {
			owner.Name = fmt.Sprintf("#%d", uid)
		}

		id := item.Name()
		parent := filepath.Join(root, id)
		stateFilePath := filepath.Join(parent, "state.json")

		f, err := os.Open(stateFilePath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintln(os.Stderr, "the path is wrong")
				return nil, os.ErrNotExist
			}
			return nil, err
		}
		defer f.Close()

		var state *shim.State
		if err := json.NewDecoder(f).Decode(&state); err != nil {
			return nil, err
		}

		status := ""
		switch state.Status {
		case runtime.CreatedStatus:
			status = "CreatedStatus"
		case runtime.RunningStatus:
			status = "RunningStatus"
		case runtime.StoppedStatus:
			status = "StoppedStatus"
		case runtime.DeletedStatus:
			status = "DeletedStatus"
		case runtime.PausedStatus:
			status = "PausedStatus"
		case runtime.PausingStatus:
			status = "PausingStatus"
		default:
			status = "wrong parameter"
		}

		s = append(s, containerState{
			ID:             id,
			InitProcessPid: state.InitProcessPid,
			Status:         status,
			Bundle:         state.Bundle,
			Created:        state.Created,
			Owner:          owner.Name,
		})
	}

	return s, nil
}
