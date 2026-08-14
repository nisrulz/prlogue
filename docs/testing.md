# Testing

## Unit tests

Unit tests live next to the code they test. Tests that need only the exported API use an external package such as `package foo_test`. Tests for unexported behavior stay in the package, such as `package foo`.

Both test types live in the same directory.

```bash
make test
```

`make test` uses `-count=1`, so Go does not use cached results.

## End-to-end tests

`make test-live` runs the Go end-to-end suite in `e2e/`:

```bash
go test -tags e2e -count=1 ./e2e/
```

The suite builds the binary, creates throwaway git repositories under the
system temp directory, and runs `prlogue generate` in each one. It checks the
output each scenario expects. Tests are grouped by concern (repository state,
generation, chunking, output format, config validation, config limits,
repository config, doctor), so you can run one group at a time:

```bash
go test -tags e2e -count=1 -run TestConfigValidation ./e2e/
```

The scenarios cover the awkward cases:

- Outside a git repository
- A branch with no changes
- Staged-only changes, and `--staged` with nothing staged
- JSON output
- Binary files, unicode filenames, renames, and deletes
- Special characters in the diff
- Large diffs that force chunking

Local runs need Ollama with the default model (`lfm2.5:8b`). Start Ollama and
pull the model before you run `make test-live`.

CI sets `PRLOGUE_LIVE_TEST_PROVIDER=mock`. This starts an in-process
OpenAI-compatible mock server with `httptest`. The suite then needs no live
model server or external binary. Do not set this variable when testing Ollama.

The mock answers commit-summary calls with a fixed JSON summary and PR
generation calls with a fixed completion, so the full pipeline runs end to
end. The per-commit summarizer accepts any well-formed JSON, so the exact
subject in the mock response does not matter.

## Test the fallback

Unit tests cover the template fallback with an OpenAI-compatible client stub. To test it from the binary, point PRlogue at an unused local endpoint:

```bash
# edit $PRLOGUE_CONFIG_DIR/prlogue/config.yaml
base_url: http://127.0.0.1:65535/v1
```

```bash
prlogue generate -v
```

The command prints a warning and returns a template description. It does not need a live model server.
