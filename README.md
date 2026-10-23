# i18n-cli

English | [简体中文](README.zh-CN.md)

A lightweight Go CLI for incremental JSON translation through an OpenAI-compatible
Chat Completions API. It has no third-party Go dependencies. Translations stay in
Git, and your production application does not call an LLM.

## Install

Requires Go 1.24 or later:

```sh
go install github.com/molesignal/i18n-cli/cmd/molesignal-i18n@latest
molesignal-i18n --help
```

To install a specific release:

```sh
go install github.com/molesignal/i18n-cli/cmd/molesignal-i18n@v0.1.0
```

Go installs the binary into `GOBIN` when set, otherwise into `$(go env GOPATH)/bin`.
Add that directory to your `PATH`. For the default location:

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
```

The installed CLI runs in your application's working directory and reads its `.env`
and configuration there. To try this repository's bundled demo, clone it first:

```sh
git clone https://github.com/molesignal/i18n-cli.git
cd i18n-cli
molesignal-i18n translate --config examples/demo/i18n.config.json --dry-run
```

## Build from source

Requires Go 1.24 or later. Run from the project root:

```sh
make build
./bin/molesignal-i18n --help
```

## Try the included demo

The [demo configuration](examples/demo/i18n.config.json) translates two source
files into Simplified Chinese (`zh-CN`) and Japanese (`ja`):

- [common.json](examples/demo/locales/en/common.json): nested UI strings, `{{name}}`, and an empty string.
- [observability.json](examples/demo/locales/en/observability.json): an array, `%s`, `{seconds}`, HTML tags, numbers, booleans, and null.

There are 12 non-empty source strings. With two target languages and a batch size
of three, the first run schedules **24 translations in eight batches across four target files**.

Preview the plan without an API key, API calls, or file writes:

```sh
./bin/molesignal-i18n translate --config examples/demo/i18n.config.json --dry-run
```

Copy the DeepSeek environment template, edit `.env` to set your API key, and run:

```sh
cp -n .env.example .env
# Edit .env and replace your-deepseek-api-key with your actual key.

./bin/molesignal-i18n translate --config examples/demo/i18n.config.json
```

Generated files:

```text
examples/demo/
├── .i18n-cli-state.json
└── locales/
    ├── en/       # Source files, unchanged
    ├── zh-CN/    # common.json and observability.json
    └── ja/       # common.json and observability.json
```

Inspect the output, then run the same command again. It should report zero new
translations without calling the API. To test incremental updates, change the
`navigation.overview` value in `examples/demo/locales/en/common.json` and rerun:
only that entry in each target language should be translated. Use `--force` to
translate all non-empty strings again. Both a fresh run and `--force` consume API usage.

Generated translations and state under `examples/demo` are ignored so a fresh checkout
starts with untranslated examples. In your application repository, commit translation
files and incremental state together.

## Environment file and DeepSeek

The CLI automatically reads `.env` from the **current working directory**, even
when `--config` points elsewhere. It does not search parent directories or the
configuration directory. A missing `.env` is allowed, and loading it does not
change the process environment.

```dotenv
OPENAI_API_KEY=your-deepseek-api-key
OPENAI_BASE_URL=https://api.deepseek.com
OPENAI_MODEL=deepseek-flash
```

The example follows the current [DeepSeek API documentation](https://api-docs.deepseek.com/).
The CLI continues to use the `OPENAI_*` names for all compatible providers.
The demo configuration also defaults to DeepSeek. Live DeepSeek calls have not
been made as part of automated verification.

Only these three keys are loaded. Blank lines, comments, optional `export`, unquoted
values, and single/double-quoted single-line values are supported. Double quotes
support Go-style escapes; single quotes are literal. Unquoted inline comments
start with `#` after whitespace. Variables and shell commands are never expanded.
Other keys are ignored; the last assignment to a supported key wins within the file.
Malformed supported values report a line number without printing their contents.

Existing non-empty exported variables override `.env`. If you previously exported
OpenAI settings, unset them to use the DeepSeek file:

```sh
unset OPENAI_API_KEY OPENAI_BASE_URL OPENAI_MODEL
./bin/molesignal-i18n translate --config examples/demo/i18n.config.json
```

`.env` is already ignored by Git; `.env.example` contains only placeholders. Use
`--force` if you want to retranslate existing demo output after changing providers.

## Configuration

The CLI automatically loads `i18n.config.json` from the current directory. Copy
[i18n.config.example.json](i18n.config.example.json) into your application repository,
or select a configuration explicitly:

```sh
/path/to/molesignal-i18n translate --config /path/to/app/i18n.config.json
```

```json
{
  "version": 1,
  "provider": {
    "id": "openai-compatible",
    "baseUrl": "https://api.openai.com/v1",
    "model": "gpt-4o-mini",
    "jsonMode": true,
    "prompt": "Translate from {source} to {target}. Preserve technical terms, placeholders and variable names. Use concise terminology for enterprise observability."
  },
  "locale": { "source": "en", "targets": ["zh-CN", "zh-TW", "ja", "ko", "fr", "de"] },
  "buckets": { "json": { "include": ["src/locales/[locale].json"] } },
  "batchSize": 40
}
```

- `version` is the integer `1`. Unknown fields are rejected.
- `provider.id` accepts `openai` or `openai-compatible`; both use the same HTTP protocol.
- `{source}` and `{target}` in `provider.prompt` are replaced with the current locales. This guidance follows the required structure and placeholder preservation instructions.
- Include patterns are relative to the configuration directory. Each must contain exactly one `[locale]` and match at least one source JSON file. Supported wildcards are `*`, `?`, and `**`.
- Single-file layout: `src/locales/[locale].json`. Directory layout: `src/i18n/[locale]/**/*.json`. Multiple patterns can be combined.
- Locale names are case-sensitive paths; `en` and `en-us` are not mapped automatically.
- Precedence: explicit CLI flags > non-empty process environment variables > non-empty `.env` values > configuration > built-in defaults.
- `--dir` overrides the pattern root. The state and lock files live at that root.
- API keys are read from `OPENAI_API_KEY` in the process environment or `.env`, never from configuration or state.

Without a configuration file, the default layout is `<dir>/<locale>/**/*.json`:

```sh
./bin/molesignal-i18n translate --dir ./locales --source en --target zh-CN,ja-JP,ko-KR
```

## MoleSignal configuration

[examples/molesignal.config.json](examples/molesignal.config.json) matches
`web/src/i18n/en-us/*.json`. Copy it into the MoleSignal repository root as
`i18n.config.json`, then run:

```sh
/path/to/molesignal-i18n translate --config /path/to/molesignal/i18n.config.json --dry-run
/path/to/molesignal-i18n translate --config /path/to/molesignal/i18n.config.json --adopt-existing
```

`--adopt-existing` trusts valid existing translations without recorded source hashes,
translates missing or invalid entries, and establishes a baseline. It cannot detect
whether an existing translation is already outdated. Omit this flag to retranslate
all entries on the first run. Later source or prompt changes still trigger translation.

## Incremental updates and reliability

- Commit `.i18n-cli-state.json` alongside the translated files. It records source/target locales, file paths, JSON Pointers, and hashes of source text and translation guidance.
- Each string is tracked separately. Missing translations, blank translations, and broken placeholders are translated again. Valid manual edits are preserved when their source fingerprint is unchanged.
- Nested objects and arrays are supported. Numbers, booleans, and null come from the source. Empty strings do not call the API. Object keys are sorted deterministically; the initial run can introduce formatting changes.
- Deleted keys are removed from corresponding target documents. Target files whose entire source file was deleted remain and must be removed manually.
- `{{name}}`, `${name}`, `{name}`, common printf placeholders, and HTML tags are checked for matching content and counts. Complex ICU plural/select messages are rejected. Validation is not a complete HTML, Markdown, or ICU parser.
- Model output must contain exactly the requested string keys. Duplicate keys, non-string values, truncated responses, and altered placeholders are rejected.
- All input documents are checked before API calls. Files are written only after all responses pass validation. Each file is replaced using a temporary file and rename; state is written last. Disk failures or interruptions can leave partial output. Rerunning recovers it but may repeat API usage.
- A root-directory lock prevents concurrent runs. Remove a stale `.i18n-cli.lock` only after confirming that no translator is running.
- HTTP 429 and 5xx responses use bounded exponential backoff. Other HTTP errors fail immediately. Invalid translation JSON, key sets, value types, or placeholders trigger at most two corrective retries (three model responses total). Persistent mismatches report missing and unexpected keys without printing translated values. Corrective attempts consume additional API usage; HTTP retry limits apply to each attempt. Error bodies are not printed, and HTTP redirects are disabled to protect credentials.
- Batches are limited by string count, not tokens. Reduce `--batch-size` for long strings. Provider-specific model, parameter, and context limits still apply.

## CLI options

| Flag | Purpose |
|---|---|
| `--config` | Configuration path; defaults to `i18n.config.json` in the current directory |
| `--source`, `--target` | Override source and comma-separated target locales |
| `--dir` | Override the file-pattern root; defaults to `locales` without a config |
| `--dry-run` | Report pending entries without API calls, writes, or credentials |
| `--adopt-existing` | Trust valid existing translations without a baseline |
| `--force` | Retranslate all non-empty strings; mutually exclusive with adoption |
| `--instructions` | Append a UTF-8 glossary/style file, relative to the working directory |
| `--base-url`, `--model` | Override provider configuration |
| `--json-mode=false` | Omit `response_format` for providers without JSON mode |
| `--allow-no-key` | Explicitly allow an unauthenticated local provider |
| `--batch-size` | Strings per request, 1–200; defaults to 40 |
| `--timeout` | Per-request timeout; defaults to 90s |
| `--retries` | HTTP 429/5xx retry count, 0–6; defaults to 3 |

The CLI appends [`/chat/completions`](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)
to the Base URL. Include `/v1` in the Base URL when required by your provider.
DeepSeek, Qwen, vLLM, and LiteLLM can be selected through Base URL and model settings,
subject to their compatibility. Automated tests use a local mock API; individual
providers have not all been verified with live requests.

## Terminal progress

Interactive macOS/Linux terminals show a compact panel that refreshes in place:
a validated-string progress bar, current target language and file, batch position,
and elapsed time. A spinner and timer update every 100 ms while the API is working.
Long paths are shortened to fit the terminal width. Completion replaces the live
panel with a saved-output summary; dry runs and failures have distinct summaries.

```text
  MOLESIGNAL  /  I18N
  ⠹ Translating  ·  en → 2 languages

  ━━━━━━━━━━━━────────────   50%  12 / 24 strings
  ja  ·  file 3/4  ·  batch 1/2
  locales/ja/common.json
  00:03 elapsed  ·  0.9s batch
```

```text
  MOLESIGNAL  /  I18N
  ✓ Complete  ·  en → 2 languages

  ━━━━━━━━━━━━━━━━━━━━━━━━  100%  24 / 24 strings
  24 translated  ·  4 files processed
  Translation files and state saved.
  Total time  00:09

  Language    Files    Strings        Time
  ────────────────────────────────────────
  zh-CN           2         12       3.57s
  ja              2         12       5.86s
```

Counts refer to string entries per target language and target file jobs. Existing
translations and empty strings are excluded from newly translated counts. Success
is shown only after translation files and incremental state have been saved.

The completion table groups results by target language. `Files` counts only files
with newly translated strings; skipped files are excluded. `Strings` counts validated
translations in this run. `Time` sums API batch durations, including provider retries,
and excludes planning and disk writes. Unchanged languages show zero values.
The table is shown only after successful persistence, in both terminal and plain logs.

Redirected output, CI, unsupported platforms, and `TERM=dumb` use plain sequential
logs with five-second heartbeat messages. `NO_COLOR=1` disables colors while keeping
the live panel on supported terminals. Progress messages remain in English.

## GitHub Actions and pull requests

The project provides a composite action in [action.yml](action.yml) and a consumer
workflow in [examples/translate.workflow.yml](examples/translate.workflow.yml).
Use the public `molesignal/i18n-cli` repository, then:

1. Save your application configuration as `i18n.config.json` and copy the workflow to `.github/workflows/translate.yml`.
2. Set Secret `OPENAI_API_KEY` and Variables `I18N_CLI_REPOSITORY` (`molesignal/i18n-cli`) and `I18N_CLI_REF` (`v0.1.0` or a fixed commit SHA). Optionally set `OPENAI_BASE_URL` and `OPENAI_MODEL`.
3. Adjust source paths, restored target paths, PR `add-paths`, and the default branch. The example uses MoleSignal's en-us/zh-cn/ja-jp/ko-kr layout.
4. Allow GitHub Actions to create pull requests and ensure repository/organization policies permit contents and pull-requests write access. A private CLI repository requires a separate checkout token with read access.
5. Push source changes or trigger the workflow manually. It restores pending translation branch state and updates the same `automation/i18n` PR.

Do not expose API keys to untrusted PR code. The example runs on main pushes or
manual dispatches. PRs created with `GITHUB_TOKEN` generally do not trigger further
PR workflows; use an appropriately scoped GitHub App token if that behavior is
required. Pin third-party actions to reviewed commit SHAs for production use.

The CLI does not implicitly commit or push. The workflow commits only configured
translation paths and state; local changes can be reviewed and committed normally.
Repository publishing, secret configuration, and live translation are separate setup steps.

## Development checks

```sh
make test
make build
```

Tests cover nested structures, incremental updates, key deletion, adoption,
configuration overrides, dry runs, failure handling, placeholder validation,
duplicate keys, HTTP retries, redirect protection, and progress reporting.

## License

Copyright (c) 2026 MoleSignal Authors.

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for the full text.
