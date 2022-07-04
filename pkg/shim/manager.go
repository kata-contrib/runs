/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package shim

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd/api/runtime/task/v2"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/pkg/timeout"
	"github.com/containerd/containerd/platforms"
	"github.com/containerd/containerd/protobuf"
	"github.com/containerd/containerd/runtime"
	shimbinary "github.com/containerd/containerd/runtime/v2/shim"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type ManagerConfig struct {
	State        string
	Address      string
	TTRPCAddress string
}

// NewShimManager creates a manager for v2 shims
func NewShimManager(ctx context.Context, config *ManagerConfig) (*ShimManager, error) {
	// for _, d := range []string{ config.State} {
	// 	if err := os.MkdirAll(d, 0711); err != nil {
	// 		return nil, err
	// 	}
	// }

	log.G(ctx).Errorf("AAAAA NewShimManager config %+v", config)

	m := &ShimManager{
		state:                  config.State,
		containerdAddress:      config.Address,
		containerdTTRPCAddress: config.TTRPCAddress,
		shims:                  runtime.NewTaskList(),
	}
	log.G(ctx).Errorf("AAAAA NewShimManager ShimManager %+v", config)

	// if err := m.loadExistingTasks(ctx); err != nil {
	// 	return nil, err
	// }

	return m, nil
}

// ShimManager manages currently running shim processes.
// It is mainly responsible for launching new shims and for proper shutdown and cleanup of existing instances.
// The manager is unaware of the underlying services shim provides and lets higher level services consume them,
// but don't care about lifecycle management.
type ShimManager struct {
	state                  string
	containerdAddress      string
	containerdTTRPCAddress string
	shims                  *runtime.TaskList
}

// Start launches a new shim instance
func (m *ShimManager) Start(ctx context.Context, id string, opts runtime.CreateOpts) (_ ShimProcess, retErr error) {
	path, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	bundle := &Bundle{
		ID:        id,
		Path:      path,
		Namespace: "default",
	}

	log.G(ctx).Errorf("AAAAA ShimManager Start bundle %+v", bundle)
	log.G(ctx).Errorf("AAAAA ShimManager Start opts %+v", opts)

	shim, err := m.startShim(ctx, bundle, id, opts)
	if err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			m.cleanupShim(shim)
		}
	}()

	// NOTE: temporarily keep this wrapper around until containerd's task service depends on it.
	// This will no longer be required once we migrate to client side task management.
	shimTask := &shimTask{
		shim: shim,
		task: task.NewTaskClient(shim.client),
	}

	if err := m.shims.Add(ctx, shimTask); err != nil {
		return nil, fmt.Errorf("failed to add task: %w", err)
	}

	return shimTask, nil
}

func (m *ShimManager) startShim(ctx context.Context, bundle *Bundle, id string, opts runtime.CreateOpts) (*shim, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}
	log.G(ctx).Errorf("AAAAA ShimManager startShim ns %+v", ns)

	topts := opts.TaskOptions
	if topts == nil || topts.GetValue() == nil {
		topts = opts.RuntimeOptions
	}
	log.G(ctx).Errorf("AAAAA ShimManager startShim opts %+v", opts)

	runtimePath, err := m.resolveRuntimePath(opts.Runtime)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve runtime path: %w", err)
	}

	b := shimBinary(bundle, shimBinaryConfig{
		runtime:      runtimePath,
		address:      m.containerdAddress,
		ttrpcAddress: m.containerdTTRPCAddress,
	})
	shim, err := b.Start(ctx, protobuf.FromAny(topts), func() {
		log.G(ctx).WithField("id", id).Info("shim disconnected")

		cleanupAfterDeadShim(context.Background(), id, ns, m.shims, b)
		// Remove self from the runtime task list. Even though the cleanupAfterDeadShim()
		// would publish taskExit event, but the shim.Delete() would always failed with ttrpc
		// disconnect and there is no chance to remove this dead task from runtime task lists.
		// Thus it's better to delete it here.
		m.shims.Delete(ctx, id)
	})
	if err != nil {
		return nil, fmt.Errorf("start failed: %w", err)
	}

	log.G(ctx).Errorf("AAAAA new shim %+v", shim)

	return shim, nil
}

func (m *ShimManager) resolveRuntimePath(runtime string) (string, error) {
	if runtime == "" {
		return "", fmt.Errorf("no runtime name")
	}

	// Custom path to runtime binary
	if filepath.IsAbs(runtime) {
		// Make sure it exists before returning ok
		if _, err := os.Stat(runtime); err != nil {
			return "", fmt.Errorf("invalid custom binary path: %w", err)
		}

		return runtime, nil
	}

	// Check if relative path to runtime binary provided
	if strings.Contains(runtime, "/") {
		return "", fmt.Errorf("invalid runtime name %s, correct runtime name should be either format like `io.containerd.runc.v1` or a full path to the binary", runtime)
	}

	// Preserve existing logic and resolve runtime path from runtime name.

	name := shimbinary.BinaryName(runtime)
	if name == "" {
		return "", fmt.Errorf("invalid runtime name %s, correct runtime name should be either format like `io.containerd.runc.v1` or a full path to the binary", runtime)
	}

	var (
		cmdPath string
		lerr    error
	)

	binaryPath := shimbinary.BinaryPath(runtime)
	if _, serr := os.Stat(binaryPath); serr == nil {
		cmdPath = binaryPath
	}

	if cmdPath == "" {
		if cmdPath, lerr = exec.LookPath(name); lerr != nil {
			if eerr, ok := lerr.(*exec.Error); ok {
				if eerr.Err == exec.ErrNotFound {
					self, err := os.Executable()
					if err != nil {
						return "", err
					}

					// Match the calling binaries (containerd) path and see
					// if they are side by side. If so, execute the shim
					// found there.
					testPath := filepath.Join(filepath.Dir(self), name)
					if _, serr := os.Stat(testPath); serr == nil {
						cmdPath = testPath
					}
					if cmdPath == "" {
						return "", fmt.Errorf("runtime %q binary not installed %q: %w", runtime, name, os.ErrNotExist)
					}
				}
			}
		}
	}

	cmdPath, err := filepath.Abs(cmdPath)
	if err != nil {
		return "", err
	}

	return cmdPath, nil
}

// cleanupShim attempts to properly delete and cleanup shim after error
func (m *ShimManager) cleanupShim(shim *shim) {
	dctx, cancel := timeout.WithContext(context.Background(), cleanupTimeout)
	defer cancel()

	_ = shim.delete(dctx)
	m.shims.Delete(dctx, shim.ID())
}

func (m *ShimManager) Get(ctx context.Context, id string) (ShimProcess, error) {
	proc, err := m.shims.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	log.G(ctx).Errorf("AAAAA ShimManager Get %+v", id)

	shimTask := proc.(*shimTask)
	return shimTask, nil
}

// Delete a runtime task
func (m *ShimManager) Delete(ctx context.Context, id string) error {
	log.G(ctx).Errorf("AAAAA ShimManager Delete %+v", id)
	proc, err := m.shims.Get(ctx, id)
	if err != nil {
		return err
	}

	shimTask := proc.(*shimTask)
	err = shimTask.shim.delete(ctx)
	m.shims.Delete(ctx, id)

	return err
}

func parsePlatforms(platformStr []string) ([]ocispec.Platform, error) {
	p := make([]ocispec.Platform, len(platformStr))
	for i, v := range platformStr {
		parsed, err := platforms.Parse(v)
		if err != nil {
			return nil, err
		}
		p[i] = parsed
	}
	return p, nil
}

// TaskManager wraps task service client on top of shim manager.
type TaskManager struct {
	manager *ShimManager
}

// NewTaskManager creates a new task manager instance.
func NewTaskManager(shims *ShimManager) *TaskManager {
	return &TaskManager{
		manager: shims,
	}
}

// // ID of the task manager
// func (m *TaskManager) ID() string {
// 	return fmt.Sprintf("%s.%s", plugin.RuntimePluginV2, "task")
// }

// Create launches new shim instance and creates new task
func (m *TaskManager) Create(ctx context.Context, taskID string, opts runtime.CreateOpts) (runtime.Task, error) {
	log.G(ctx).Errorf("AAAAA TaskManager Create %+v", opts)
	process, err := m.manager.Start(ctx, taskID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to start shim: %w", err)
	}

	// Cast to shim task and call task service to create a new container task instance.
	// This will not be required once shim service / client implemented.
	shim := process.(*shimTask)
	log.G(ctx).Errorf("AAAAA call shim Create start")
	t, err := shim.Create(ctx, opts)
	log.G(ctx).Errorf("AAAAA call shim Create end")
	if err != nil {
		// NOTE: ctx contains required namespace information.
		m.manager.shims.Delete(ctx, taskID)

		dctx, cancel := timeout.WithContext(context.Background(), cleanupTimeout)
		defer cancel()

		sandboxed := false
		_, errShim := shim.Delete(dctx, sandboxed, func(context.Context, string) {})
		if errShim != nil {
			if errdefs.IsDeadlineExceeded(errShim) {
				dctx, cancel = timeout.WithContext(context.Background(), cleanupTimeout)
				defer cancel()
			}

			shim.Shutdown(dctx)
			shim.Close()
		}

		return nil, fmt.Errorf("failed to create shim task: %w", err)
	}
	log.G(ctx).Errorf("AAAAA call shim Create end ok")

	return t, nil
}

// Get a specific task
func (m *TaskManager) Get(ctx context.Context, id string) (runtime.Task, error) {
	return m.manager.shims.Get(ctx, id)
}

// Tasks lists all tasks
func (m *TaskManager) Tasks(ctx context.Context, all bool) ([]runtime.Task, error) {
	return m.manager.shims.GetAll(ctx, all)
}

// Delete deletes the task and shim instance
func (m *TaskManager) Delete(ctx context.Context, taskID string) (*runtime.Exit, error) {
	item, err := m.manager.shims.Get(ctx, taskID)
	if err != nil {
		return nil, err
	}

	sandboxed := false
	shimTask := item.(*shimTask)
	exit, err := shimTask.Delete(ctx, sandboxed, func(ctx context.Context, id string) {
		m.manager.shims.Delete(ctx, id)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to delete task: %w", err)
	}

	return exit, nil
}
