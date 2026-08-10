# Installation

PRlogue requires Go 1.25 or newer.

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

Both commands install the binary in `$(go env GOPATH)/bin`. Make sure that directory is in your `PATH`.

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
