# Running rkt with the *fly* stage1

The *fly* stage1 is an alternative stage1 that runs a single-application ACI with only `chroot`-isolation.


## Motivation

The motivation of the fly feature is to add the ability to run applications with full privileges on the host but still benefit from the image management and discovery from rkt.
The Kubernetes `kubelet` is one candidate for rkt fly.


## How does it work?

In comparison to the default stage1, there is no process manager involved in the stage1.
This a visual illustration for the differences in the process tree between the default and the fly stage1:

stage1-coreos.aci:

```
host OS
  └─ rkt
    └─ systemd-nspawn
      └─ systemd
        └─ chroot
          └─ user-app1
```


stage1-fly.aci:

```
host OS
  └─ rkt
    └─ chroot
      └─ user-app1
```

The rkt application sets up bind mounts for `/dev`, `/proc`, `/sys`, and the user-provided volumes.
In addition to the bind mounts, an additional *tmpfs* mount is done at `/tmp`.
After the mounts are set up, rkt `chroot`s to the application's RootFS and finally executes the application.


### Mount propagation modes

The *fly* stage1 makes use of Linux [mount propagation modes](https://www.kernel.org/doc/Documentation/filesystems/sharedsubtree.txt).
If a volume source path is a mountpoint on the host, this mountpoint is made recursively shared before the host path is mounted on the target path in the container.
Hence, changes to the mounts inside the container will be propagated back to the host.

The bind mounts for `/dev`, `/proc`, and `/sys` are done automatically and are recursive, because their hierarchy contains mounts which also need to be available for the container to function properly.
User-provided volumes are not mounted recursively.
This is a safety measure to prevent system crashes when multiple containers are started that mount `/` into the container.


## Getting started

You can either use `stage1-fly.aci` from the official release, or build rkt yourself with the right options:

```
$ ./autogen.sh && ./configure --with-stage1-flavors=fly && make
```

For more details about configure parameters, see the [configure script parameters documentation](build-configure.md).
This will build the rkt binary and the stage1-fly.aci in `build-rkt-0.16.0+git/bin/`.

### Selecting stage1 at runtime

Here is a quick example of how to use a container stage1 named `stage1-fly.aci` in `/usr/local/rkt/`:

```
# rkt run --stage1-path=/usr/local/rkt/stage1-fly.aci coreos.com/etcd:v2.0.9
```

When the image is already in the store, the `--stage1-name` or `--stage1-hash` flags can be used instead for a faster startup:

```
# rkt run --stage1-name=coreos.com/rkt/stage1-fly coreos.com/etcd:v2.0.9
# rkt run --stage1-hash=<hash> coreos.com/etcd:v2.0.9
```

## Notes on isolation and security

By design, the *fly* stage1 does not provide the same isolaton and security features as the default stage1.

Specifically, the following constraints are not available when using the *fly* stage1:
- network namespace isolation
- CPU isolators
- Memory isolators
- CAPABILITY bounding
- SELinux

### Providing additional isolation with systemd

When using systemd on the host it is possible to [wrap rkt with a systemd unit file](using-rkt-with-systemd.md#advanced-unit-file) to provide additional isolation.
For more information please consult the systemd manual.
* [systemd.resource-control](http://www.freedesktop.org/software/systemd/man/systemd.resource-control.html)
* [systemd.directives](http://www.freedesktop.org/software/systemd/man/systemd.directives.html)
