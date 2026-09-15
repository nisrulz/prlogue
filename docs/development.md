# Development

PRlogue needs Go 1.27 or newer. The exact version is pinned in `go.mod`
(`toolchain go1.27.1`) and `.go-version`. CI reads it from `go.mod`, and the
`make` targets force `GOTOOLCHAIN=go1.27.1` so a newer local Go cannot drift.

## Set up

```bash
make build       # build the binary
make test        # unit tests, cache off
make test-live   # end-to-end suite, see docs/testing.md
make audit       # race detector, vet, govulncheck
```

Run `make audit` before opening a PR.

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
- Retry transient provider failures with backoff. Classify errors by HTTP status and transport type, not by message text.
- Prefer the standard library and existing dependencies.
