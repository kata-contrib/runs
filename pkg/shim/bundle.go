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
	"path/filepath"

	"github.com/containerd/containerd/identifiers"
	"github.com/containerd/typeurl"
	//	"github.com/containerd/containerd/namespaces"
)

const configFilename = "config.json"

// NewBundle returns a new bundle on disk
func NewBundle(ctx context.Context, state, id string, spec typeurl.Any) (b *Bundle, err error) {
	if err := identifiers.Validate(id); err != nil {
		return nil, fmt.Errorf("invalid task id %s: %w", id, err)
	}

	//	ns, err := namespaces.NamespaceRequired(ctx)
	// if err != nil {
	// 	return nil, err
	// }

	path, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	work := filepath.Join("/run/runs/", id)
	b = &Bundle{
		ID:        id,
		Path:      path,
		Namespace: "default",
	}
	/*
		if err := os.Symlink("/home/vagrant/test/rootfs", filepath.Join(b.Path, "rootfs")); err != nil {
			return nil, err
		}
	*/
	var paths []string
	defer func() {
		if err != nil {
			for _, d := range paths {
				os.RemoveAll(d)
			}
		}
	}()
	// // create state directory for the bundle
	// if err := os.MkdirAll(filepath.Dir(b.Path), 0711); err != nil {
	// 	return nil, err
	// }
	// if err := os.Mkdir(b.Path, 0700); err != nil {
	// 	return nil, err
	// }
	// if typeurl.Is(spec, &specs.Spec{}) {
	// 	if err := prepareBundleDirectoryPermissions(b.Path, spec.GetValue()); err != nil {
	// 		return nil, err
	// 	}
	// }
	paths = append(paths, b.Path)
	// // create working directory for the bundle
	// if err := os.MkdirAll(filepath.Dir(work), 0711); err != nil {
	// 	return nil, err
	// }
	rootfs := filepath.Join(b.Path, "rootfs")
	if err := os.MkdirAll(rootfs, 0711); err != nil {
		return nil, err
	}
	paths = append(paths, rootfs)
	if err := os.Mkdir(work, 0711); err != nil {
		if !os.IsExist(err) {
			return nil, err
		}
		// 	os.RemoveAll(work)
		// 	if err := os.Mkdir(work, 0711); err != nil {
		// 		return nil, err
		// 	}
	}
	paths = append(paths, work)
	// // symlink workdir
	if err := os.Symlink(work, filepath.Join(b.Path, "work")); err != nil {
		return nil, err
	}
	if spec := spec.GetValue(); spec != nil {
		// write the spec to the bundle
		err = os.WriteFile(filepath.Join(b.Path, configFilename), spec, 0666)
		if err != nil {
			return nil, fmt.Errorf("failed to write %s", configFilename)
		}
	}
	return b, nil
}

// Bundle represents an OCI bundle
type Bundle struct {
	// ID of the bundle
	ID string
	// Path to the bundle
	Path string
	// Namespace of the bundle
	Namespace string
}

func (b *Bundle) Delete() error {
	return nil
}
