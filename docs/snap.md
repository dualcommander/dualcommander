# Linux Snap installation

Download `dualcommander-linux-amd64.snap` or `dualcommander-linux-arm64.snap` from
[GitHub Releases](https://github.com/dualcommander/dualcommander/releases).
With `snapd` installed, install the file matching your architecture:

```sh
sudo snap install --dangerous --classic ./dualcommander-linux-amd64.snap
dualcommander
```

Classic confinement grants normal desktop filesystem and external-application
access; this is not a sandboxed package. `--dangerous` accepts a local package
without a Snap Store assertion. Only install trusted downloads. GitHub packages
do not receive Snap Store automatic updates; install newer downloads manually.