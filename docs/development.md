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
- Send each prompt block in its own model call. Hold output until every block is sent, then generate in a final call.
- Summarize each commit from its message, description, and diff. Store all summaries in one JSON file and use that as the generation context instead of raw git data.
- Reject model output that echoes an acknowledgment, refuses, or claims no changes when repository data exists. Retry once with the repository statistics, then fall back to the template.
- Prefer the standard library and existing dependencies.
