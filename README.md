# PRlogue

> Your git diff, turned into a PR description.

![Header](img/github_banner.png)

PRlogue turns commits and diffs into PR descriptions. It runs locally by default, so your Git data stays on your machine. Use Ollama or any OpenAI-compatible server.

## Quick start

Install the CLI:

```bash
curl -sfL https://github.com/nisrulz/prlogue/releases/latest/download/install.sh | sh
```

Create the user config:

```bash
prlogue init
```

The default setup uses Ollama. Start the server with:

```bash
ollama pull lfm2.5:8b
ollama serve
```

Run these commands from the Git repository that contains your changes:

```bash
prlogue doctor
prlogue generate
```

> Run `prlogue generate -q` to skip the CLI banner

Save the result to a file:

```bash
prlogue generate --output pr-description.md
```

Create the GitHub pull request with the [GitHub CLI](https://cli.github.com/):

```bash
prlogue generate --publish
```

Using LM Studio, OpenAI, or another OpenAI-compatible server? See [the reference](docs/reference.md).

## Docs

- [Installation](docs/installation.md): install from Go, a clone, or a release
- [Reference](docs/reference.md): OpenAI-compatible setup, flags, config, pipeline
- [Development](docs/development.md): build, test, audit
- [Testing](docs/testing.md): unit tests and the end-to-end suite
- [Releasing](docs/releasing.md): checks, tags, release artifacts

## License

[Apache-2.0](LICENSE)
