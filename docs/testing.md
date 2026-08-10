# Testing

## Unit tests

Unit tests live next to the code they test. Tests that only need the exported API use an external package (`package foo_test`), so they can only reach the public surface. Tests that exercise unexported behavior stay in-package (`package foo`). Both kinds live in the same directory.

```bash
make test
```

`make test` uses `-count=1`, so it never serves you a cached result.

## End-to-end tests

`make test-live` runs `scripts/live-test.sh`. It builds the binary, creates throwaway git repos under `.temp-test/`, and runs `prlogue generate` in each one. Then it greps the output to check the behavior each scenario expects.

The scenarios cover the awkward cases:

- Outside a git repository
- A branch with no changes
- Staged-only changes, and `--staged` with nothing staged
- JSON output
- Binary files, unicode filenames, renames, and deletes
- Special characters in the diff
- Large diffs that force chunking

The script requires Ollama with the default model (`lfm2.5:8b`) when run locally. It does not use the template fallback. Start Ollama and pull the model before you run `make test-live`.

CI sets `PRLOGUE_LIVE_TEST_PROVIDER=mock`. This starts a local OpenAI-compatible mock server for the end-to-end tests. The mock is a small Go program (`scripts/mock-openai-server.go`) with no external dependencies, so the suite needs no Python runtime. Do not set this variable locally when you want to test against Ollama.

## Test the fallback

The template fallback is covered by an OpenAI-compatible client stub in unit tests. To see it from the binary, point PRlogue at an unused local endpoint by editing the config:

```bash
# edit $PRLOGUE_CONFIG_DIR/prlogue/config.yaml
base_url: http://127.0.0.1:65535/v1
```

```bash
prlogue generate -v
```

The command prints a warning and returns a template description. It needs no live model server.
