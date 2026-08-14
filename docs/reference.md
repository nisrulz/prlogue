# PRlogue reference

## Install

See [Installation](installation.md).

## Start here

Run `init` from the Git repository that contains the changes:

```bash
prlogue init
```

`prlogue init` creates the user config. It does not install or start a model server. The default config uses Ollama on `http://localhost:11434/v1` with `lfm2.5:8b`.

For a local Ollama setup:

```bash
ollama pull lfm2.5:8b
ollama serve
```

Run `doctor` next. It checks the config, endpoint, model, and Git repository. Fix any failed check before you generate a description:

```bash
prlogue doctor
prlogue generate
```

Save the result to a file with `--output`:

```bash
prlogue generate --output pr-description.md
```

Create the GitHub pull request with the [GitHub CLI](https://cli.github.com/):

```bash
prlogue generate --publish
```

## Use another model server

PRlogue accepts any server that implements the OpenAI chat API. Set `provider`, `base_url`, and `model` in the user config. The `name` value is a label for `init`, `doctor`, and `config` output.

Here are common settings:

| Server | `name` | `base_url` | Example `model` |
| --- | --- | --- | --- |
| Ollama | `Ollama` | `http://localhost:11434/v1` | `lfm2.5:8b` |
| LM Studio | `LM Studio` | `http://localhost:1234/v1` | `LiquidAI/LFM2.5-8B-A1B-MLX-4bit` |
| OpenAI | `OpenAI` | `https://api.openai.com/v1` | `gpt-5.6-luna` |

```yaml
# edit $PRLOGUE_CONFIG_DIR/prlogue/config.yaml
name: Ollama
provider: openai_compat
base_url: http://localhost:11434/v1
model: lfm2.5:8b
```

For LM Studio, replace the server values with:

```yaml
name: LM Studio
provider: openai_compat
base_url: http://localhost:1234/v1
model: LiquidAI/LFM2.5-8B-A1B-MLX-4bit
```

Local servers do not need an API key. Remote servers that require authentication use `PRLOGUE_OPENAI_COMPAT_API_KEY`:

```yaml
base_url: https://api.openai.com/v1
model: gpt-5.6-luna
```

```bash
export PRLOGUE_OPENAI_COMPAT_API_KEY=<token>
prlogue generate
```

PRlogue reads API keys from `PRLOGUE_OPENAI_COMPAT_API_KEY` only. Config files cannot store or set them.

## Generate flags

| Flag | Short | Default | What it does |
| --- | --- | --- | --- |
| `--staged` | `-s` | `false` | Reads staged changes only |
| `--output` | `-o` | stdout | Writes the result to a file |
| `--publish` | | `false` | Creates a GitHub PR with the [GitHub CLI](https://cli.github.com/) |
| `--verbose` | `-v` | `false` | Prints the resolved endpoint and git details |
| `--quiet` | `-q` | `false` | Suppresses the startup banner |
| `--config` | | user config | Loads an explicit trusted config file |

Set all other options in the config file. Commands reject unexpected positional arguments. Branch values must be valid Git branch names, not revision expressions such as `HEAD~10`.

## OpenAI-compatible endpoint

Set `provider` to `openai_compat`, then set `base_url` and `model` for the server you use.

`response_max_tokens` sets the response limit for PR generation. It defaults to `8192`.
You can set it from `8192` to `1048576`.
Choose a lower value when your provider has a smaller limit, but keep it at least `8192`.
The limit applies to the final generation call. Per-commit summaries use the configured context length as their token budget.

PRlogue accepts plain HTTP only for `localhost` and loopback IP addresses. Remote endpoints must use HTTPS, and HTTP redirects are not followed.

## User config

PRlogue stores user config at `$PRLOGUE_CONFIG_DIR/prlogue/config.yaml`. `PRLOGUE_CONFIG_DIR` defaults to `~/.config`. The first run creates the directory and the config file. After that, the file is the source of truth. Older files without `response_max_tokens` use `8192`, and files without `staged_context` default to `true`.

The generated config looks like this:

```yaml
name: Ollama                           # optional label shown in CLI output
provider: openai_compat                # required provider type
model: lfm2.5:8b                       # required model name
base_url: http://localhost:11434/v1    # required OpenAI-compatible endpoint
response_max_tokens: 8192              # maximum response tokens sent to the provider
no_think: true                         # optional, defaults to false when omitted
staged_context: true                   # optional, defaults to true when omitted
extra_body: {}                         # optional provider-specific request fields

context:                               # required context settings
  mode: auto                           # required: auto or manual
  manual: 131072                       # required manual context size
  max_auto: 1000000                    # required maximum automatic context
  min_auto: 4096                       # required minimum automatic context

chunking:                              # required diff chunking settings
  strategy: two-tier                   # required: two-tier, file, or hunk
  file_summary_threshold: 200          # required file-size threshold
  hunk_split_threshold: 500            # required hunk-size threshold

git:                                   # optional
  default_branch: ""                   # optional, auto-detected when empty

output:                                # required output settings
  format: markdown                     # required: markdown or json

system:                                # optional
  os_reservation_gb: 0                 # optional reserved RAM for the OS
  model_size_gb: 5.2                   # optional model size used for context sizing
```

`prlogue init` copies the bundled style prompt to `$PRLOGUE_CONFIG_DIR/prlogue/output_style_prompt.txt`.

PRlogue keeps the task, security, sanitization, and commit-summary prompts outside the config file.

`staged_context` controls how PRlogue sends the prompt. With `true` (the default), each context block goes in its own model call and the model holds its output until the final call. Set it to `false` to send every block in one call. Staged delivery helps models that lose track of context in a single large request. If a small model keeps misbehaving, try `staged_context: false`.

An explicit `--config <path>` is treated as trusted user config. PRlogue validates the whole file before doing any work.

### Provider name

`name` is a free-form label for the server you point the config at. It has no functional effect and only appears in `prlogue init` and `prlogue config` output. The scaffolded config uses `Ollama`; set it to whatever you run:

```yaml
name: Ollama
```

Check the current value with `prlogue config get name`.

## Output style prompt

The embedded task prompt tells the model how to review the collected repository data. The bundled style prompt defines the `Title:` and `### PR Description` format.

PRlogue sends the branch context and the per-commit summaries in a separate user message.

`prlogue init` copies the bundled prompt to:

```text
$PRLOGUE_CONFIG_DIR/prlogue/output_style_prompt.txt
```

Edit that file to change the output format. PRlogue creates it when a command needs it and the file does not exist.

PRlogue loads the file before it uses the embedded fallback. Empty, oversized, or unrelated content uses the fallback instead.

The file may contain format rules only. PRlogue ignores content about commands, tools, secrets, repository actions, or prompt policies.

PRlogue sends the style prompt as a system message. The file has a 64 KiB limit. Immutable task, security, sanitization, and commit-summary prompts remain separate.

Tell the model to put `Title:` on the first output line. PRlogue uses that line as the PR title; otherwise it uses the current branch name.

The style prompt only affects the model path. The template fallback builds its description from Git data.

Check the file path with `prlogue config get output_style_prompt_file`. The `prlogue config` command also shows the path.

If a provider rejects the response limit, lower `response_max_tokens` in the config file and run `prlogue generate` again.

### Repository config

An implicit `.prlogue.yaml` in the current repository has a small allowlist:

```yaml
git:
  default_branch: develop
output:
  format: json
```

Any other key returns an error. Move trusted endpoint settings to the user config, or pass the file explicitly with `--config` if you intend to trust the whole file.

### Request extras

`extra_body` adds server-specific JSON fields:

```yaml
extra_body:
  chat_template_kwargs:
    enable_thinking: false
```

`extra_body` cannot override the API fields `model`, `messages`, `max_tokens`, `temperature`, or `stream`.
Set the response limit with `response_max_tokens`.
PRlogue owns the other request fields. The immutable security and sanitization policies are also not configurable.

## Config commands

```bash
# Show user config
prlogue config

# Read one value
prlogue config get provider
prlogue config get output_style_prompt_file

# Back up the current config and restore defaults
prlogue reset-config
```

Edit settings directly in the config file. Edit the style file separately. `api_key` is never stored in config; use `PRLOGUE_OPENAI_COMPAT_API_KEY` instead.

`reset-config` writes a backup next to the config before it writes a new default config. The backup name has this form: `config.yaml.YYYYMMDD-HHMMSS.NNNNNNNNN.bak`. The command uses the configured `$PRLOGUE_CONFIG_DIR` path, or the path passed with `--config`.

## Doctor

Run this before you generate:

```bash
prlogue doctor
```

```sh

'||'''|, '||'''|, '||`                                
 ||   ||  ||   ||  ||                                 
 ||...|'  ||...|'  ||  .|''|, .|''|, '||  ||` .|''|, 
 ||       || \\    ||  ||  || ||  ||  ||  ||  ||..|| 
.||      .||  \\. .||. `|..|' `|..||  `|..'|. `|...  
                                  ||                 
                               `..|' 

  ✔ config            ~/.config/prlogue/config.yaml
  ✔ provider          Ollama · openai_compat
  ✔ api key           not required
  ✔ endpoint          http://localhost:11434/v1
  ✔ model             lfm2.5:8b loaded
  ✔ llm connection    operational
  ✔ git               ready

All checks passed.
```

It checks the setup:

- the config file is loaded and valid
- the provider, including its `name` label
- whether an API key is needed
- whether the endpoint is reachable
- whether your model is listed on the endpoint
- whether the model responds to a test request
- the repo's `.prlogue.yaml` and the git work tree

The test request sends "Hi" and gives the model your full configured context as its token budget. Thinking models can use that space to finish their `<think>` block before they answer.

Pass `--config` to check an explicit config file instead of the user config.

Every check prints `✔` for a pass, `⚠` for a warning, and `✗` for a problem. Failures repeat at the end under "Errors found:". The command exits non-zero when any check fails, so it works as a CI gate.

## Context and input limits

Auto mode estimates the context size from available RAM and the configured model size. Manual mode uses `context.manual`.

The resolved context size controls how much repository data PRlogue includes in the request. The repository-data prompt is capped at 1 MiB and always keeps the closing untrusted-data marker. Each per-commit diff is capped at 1 MiB before it is summarized. The output style prompt is capped at 64 KiB. Git diff collection stops at 8 MiB. Larger diffs return a clear error instead of exhausting memory.

This setting does not reconfigure your model server. Configure the server context separately, for example with `lms load --context-length`.

## Thinking blocks

PRlogue removes `<think>...</think>` blocks by default. Set `no_think: false` in the config to keep them.

The parser handles malformed or out-of-order tags without getting stuck in a loop.

## Publish a PR

```bash
prlogue generate --publish
```

Publishing requires the [GitHub CLI](https://cli.github.com/) and an authenticated GitHub session. Only the command-line flag can publish. User and repository config cannot enable it automatically.

## Commit summaries

PRlogue reads the range between the base branch and the current branch, then summarizes each commit in its own model call. The prompt for one commit is its message, its description, and its bounded diff. The model returns a JSON object with the summary, key changes, and impact.

In an interactive terminal, PRlogue lists every commit while it works. Each row starts with a braille spinner, then turns into a check mark when that commit is summarized.

PRlogue stores all summaries in one JSON file in the system temp directory. `prlogue generate -v` prints the file path. The final generation call reads the summaries instead of the raw diff, so the model sees a compact and complete digest of every commit.

A failed summary call does not stop the run. PRlogue fills in a fallback entry with the commit subject, description, and changed file paths, so no commit is dropped.

Small models sometimes return a summary that is wrong but well formed. The checks in the final generation call catch the common cases:

- Output that echoes an acknowledgment (for example, "OK" or "ACK").
- Output that refuses or claims there are no changes when the diff has files.
- Output that repeats the diff instead of describing it.
- Output that skips the configured output format, such as the `### PR Description` and `### Key Changes` headings.

PRlogue rejects that output, retries once with the repository statistics, and falls back to the local template if the retry still fails.

## Pipeline

1. Resolve and validate the trusted config.
2. Detect the base branch and collect up to 50 commits (with message bodies).
3. Read a bounded Git diff with external diff drivers disabled.
4. Summarize each commit from its message, description, and per-commit diff, and store all summaries in one JSON file in the temp directory.
5. Classify and chunk changes locally for JSON and template output.
6. Send each bounded context block (security, sanitization, output style, commit summaries) as its own model call, asking the model to hold its output until all blocks are sent.
7. Release the collected context in a final generation call for the PR title and description. Output that echoes an acknowledgment, refuses, or claims there are no changes is rejected and retried once against the repository statistics.
8. Use the local template if the server is unavailable or the output stays unusable.
9. Format the result as Markdown or JSON, then publish only when requested.

`prlogue generate -v` prints the path of the commit-summary JSON file so you can inspect what the model was given.
