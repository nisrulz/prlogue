# Installation

PRlogue requires Go 1.27 or newer. The project pins Go 1.27.1 in `go.mod`, so the build uses that exact toolchain and downloads it when your install is older.

## Install with Go

```bash
go install github.com/nisrulz/prlogue@latest
```

## Build from a clone

```bash
git clone https://github.com/nisrulz/prlogue.git
cd prlogue
make install
```

`go install` writes the binary to `$(go env GOPATH)/bin` (or `GOBIN` if you set it). `make install` copies it to `~/go/bin`. Make sure the directory for the command you used is in your `PATH`.

## Install a release binary

Download a binary from the GitHub releases page, or install the latest release:

```bash
curl -sfL https://github.com/nisrulz/prlogue/releases/latest/download/install.sh | sh
```

To pin a version, pass the tag as an argument:

```bash
curl -sfL https://github.com/nisrulz/prlogue/releases/latest/download/install.sh | sh -s -- v0.1.0
```

The tag can be `v0.1.0` or `0.1.0`.
