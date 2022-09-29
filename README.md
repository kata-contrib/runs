# runs

Special credits:

- cmd/runs: from runc
- pkg/shim: from containerd

## Using runs

### Prerequisites

```
bundle_dir="bundle"
rootfs_dir="$bundle_dir/rootfs"
image="busybox"
mkdir -p "$rootfs_dir" && (cd "$bundle_dir" && sudo runs spec)
sudo docker export $(sudo docker create "$image") | tar -C "$rootfs_dir" -xf -
```

Note: If you use the unmodified runs spec template, this should give a sh session inside the container. However, if you use runs directly and run a container with the unmodified template, runs cannot launch the sh session because runs does not support terminal handling yet. You need to edit the process field in the config.json should look like this below with "terminal": false and "args": ["sleep", "10"].

```
"process": {
    "terminal": false,
    "user": {
        "uid": 0,
        "gid": 0
    },
    "args": [
        "sleep",
        "10"
    ],
    "env": [
        "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
        "TERM=xterm"
    ],
    "cwd": "/",
    [...]
}
```

### Running a container
Now you can go through the lifecycle operations in your shell. You need to run runk as root because runk does not have the rootless feature which is the ability to run containers without root privileges.

```
cd $bundle_dir

# Create a container
sudo runs create test

# View the container is created and in the "created" state
sudo runs list

# Start a container
sudo runs start test

# View the container is started and in the "running" state
sudo runs list

# kill the process inside the container
sudo runs kill test

# View the process inside the container is killed and in the "stopped" state
sudo runs list

# Now delete the container
sudo runs delete test
```
