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

Now run `doctor`. It checks the config, endpoint, model, and Git repository. Fix any failed check before you generate the description:

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

PRlogue accepts any server that implements the OpenAI chat API. Set `provider`, `base_url`, and `model` in the user config. The `name` value is only a label shown by `init`, `doctor`, and `config`.

Common settings:

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

Everything else is set in the config file. Commands reject unexpected positional arguments. Branch values must be valid git branch names, not revision expressions such as `HEAD~10`.

## OpenAI-compatible endpoint

Set `provider` to `openai_compat`, then set `base_url` and `model` for the server you use.

PRlogue accepts plain HTTP only for `localhost` and loopback IP addresses. Remote endpoints must use HTTPS, and HTTP redirects are not followed.

## User config

PRlogue stores user config at `$PRLOGUE_CONFIG_DIR/prlogue/config.yaml`. `PRLOGUE_CONFIG_DIR` defaults to `~/.config`. The first run creates the directory and the config file. After that, the file is the source of truth. The CLI does not apply defaults when it loads the file.

The generated config looks like this:

```yaml
name: Ollama                           # optional label shown in CLI output
provider: openai_compat                # required provider type
model: lfm2.5:8b                       # required model name
base_url: http://localhost:11434/v1    # required OpenAI-compatible endpoint
no_think: true                         # optional, defaults to false when omitted
output_style_prompt: |                 # optional, uses the bundled format prompt when empty
  Output only a PR description in this format and nothing else:

  Title: <one-line PR title>

  ### PR Description
  Write a 2-3 sentence summary of what the PR does and why.

  ### Key Changes
  Use a bullet list for relevant changes. Omit this section if there are no relevant changes.

  Use the configured headings and bullet style. Omit any section that has no relevant information. Do not add placeholder categories or filler text.
  Keep one blank line between sections. When you mention syntax from the diff, copy the complete syntax exactly. Do not truncate delimiters or invent a partial code token. If you cannot copy it exactly, describe the change without the token.
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

The generated config contains the bundled fallback format prompt in `output_style_prompt`. PRlogue keeps the security and sanitization prompts outside the config file.

An explicit `--config <path>` is treated as trusted user config. PRlogue validates the whole file before doing any work.

### Provider name

`name` is a free-form label for the server you point the config at. It has no functional effect and only appears in `prlogue init` and `prlogue config` output. The scaffolded config uses `Ollama`; set it to whatever you run:

```yaml
name: Ollama
```

Check the current value with `prlogue config get name`.

## Output style prompt

The bundled output style prompt defines the `Title:` and `### PR Description` format. Set `output_style_prompt` to change the format or presentation. PRlogue sends branch, issue, commit, and diff data in a separate user message.

PRlogue sends the output style prompt as a system message. The value has a 64 KiB limit. PRlogue adds immutable security and sanitization policies after it. The configured prompt cannot change those policies.

```yaml
# edit $PRLOGUE_CONFIG_DIR/prlogue/config.yaml
output_style_prompt: |
  Use concise headings for a Go codebase.
  Start with "Title:" on the first line.
```

Tell the model to put `Title:` on the first output line. PRlogue uses that line as the PR title; otherwise it falls back to the current branch name.

The output style prompt only affects the model path. The template fallback ignores it and always builds the description from the git data.

Edit the config file to set or clear it. An empty string uses the bundled output style prompt:

```yaml
output_style_prompt: ""
```

Check the current value with `prlogue config get output_style_prompt`. `prlogue config` prints whether it is set.

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

It cannot replace `model`, `messages`, `max_tokens`, `temperature`, or `stream`. Those fields are owned by PRlogue. The immutable security and sanitization policies are also not configurable.

## Config commands

```bash
# Show user config
prlogue config

# Read one value
prlogue config get provider
prlogue config get output_style_prompt

# Back up the current config and restore defaults
prlogue reset-config
```

Settings are edited directly in the config file. `api_key` is never stored in config; use `PRLOGUE_OPENAI_COMPAT_API_KEY` instead.

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

The test request sends "Hi" and gives the model your full configured context as its token budget. That leaves thinking models room to finish their `<think>` block before answering.

Pass `--config` to check an explicit config file instead of the user config.

Every check prints `✔` for a pass, `⚠` for a warning, and `✗` for a problem. Failures repeat at the end under "Errors found:". The command exits non-zero when any check fails, so it works as a CI gate.

## Context and input limits

Auto mode estimates a context size from available RAM and the configured model size. Manual mode uses `context.manual`.

The resolved context size controls how much repository data PRlogue includes in the request. The repository-data prompt is capped at 1 MiB and always keeps the closing untrusted-data marker. The output style prompt is capped at 64 KiB. Git diff collection stops at 8 MiB. Larger diffs return a clear error instead of exhausting memory.

This setting does not reconfigure your model server. Configure the server context separately, for example with `lms load --context-length`.

## Thinking blocks

PRlogue removes `<think>...</think>` blocks by default. Set `no_think: false` in the config to keep them.

The parser handles malformed or out-of-order tags without getting stuck in a loop.

## Publish a PR

```bash
prlogue generate --publish
```

Publishing requires the [GitHub CLI](https://cli.github.com/) and an authenticated GitHub session. Only the command-line flag can publish. User and repository config cannot enable it automatically.

## Pipeline

1. Resolve and validate trusted config.
2. Detect the base branch and collect up to 50 commits.
3. Read a bounded git diff with external diff drivers disabled.
4. Classify and chunk changes locally for JSON and template output.
5. Send one bounded request to the selected model.
6. Fall back to the local template if the server is unavailable.
7. Format the result as Markdown or JSON, then publish only when requested.

The normal path makes one model call.
