# Development

PRlogue needs Go 1.25 or newer.

## Set up

```bash
make build       # build the binary
make test        # unit tests, cache off
make test-live   # end-to-end suite, see docs/testing.md
make audit       # race detector, vet, govulncheck
```

Run `make audit` before opening a PR. It runs the race detector, `go vet`, and `govulncheck`.

To build and install from source:

```bash
make install
```

The command copies the binary to `~/go/bin`.

## Project conventions

- Shell out to git with argument arrays. Do not invoke a shell.
- Disable external diff drivers and cap command output.
- Treat repository config, git metadata, diffs, and model output as untrusted.
- Keep API keys in `PRLOGUE_OPENAI_COMPAT_API_KEY`. Config files never store or set them.
- Use one model call for the normal path.
- Prefer the standard library and existing dependencies.
