# i18n-cli

[English](README.md) | 简体中文

轻量 Go CLI，通过 OpenAI-compatible Chat Completions API 增量翻译 JSON。无第三方 Go 依赖，翻译结果直接保存在 Git 仓库，生产环境不调用 LLM。

## go install 安装

需要 Go 1.24 或更高版本：

```sh
go install github.com/molesignal/i18n-cli/cmd/i18n-cli@latest
i18n-cli --help
```

安装指定版本：

```sh
go install github.com/molesignal/i18n-cli/cmd/i18n-cli@v0.2.0
```

二进制安装到 `GOBIN` 指定的目录；未设置时安装到 `$(go env GOPATH)/bin`。请将对应目录加入 `PATH`。默认路径的设置方法：

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
```

安装后的 CLI 在业务仓库的工作目录运行，读取该目录的 `.env` 和配置文件。如果要测试本仓库提供的示例，先克隆源码：

```sh
git clone https://github.com/molesignal/i18n-cli.git
cd i18n-cli
i18n-cli translate --config examples/demo/i18n.config.json --dry-run
```

## 从源码构建与运行

需要 Go 1.24+：

```sh
make build
export OPENAI_API_KEY="your-key"
export OPENAI_MODEL="gpt-4.1-mini"
export OPENAI_BASE_URL="https://api.openai.com/v1"
./bin/i18n-cli translate --dir ./locales --source en --target zh-CN,ja-JP,ko-KR
```

无配置模式使用 `<dir>/<locale>/**/*.json`，例如 `locales/en/common.json` → `locales/zh-CN/common.json`。

## 运行内置测试示例

[示例配置](examples/demo/i18n.config.json) 将以下两个英文文件翻译为简体中文（`zh-CN`）和日文（`ja`）：

- [common.json](examples/demo/locales/en/common.json)：嵌套界面文案、`{{name}}` 占位符和空字符串。
- [observability.json](examples/demo/locales/en/observability.json)：数组、`%s`、`{seconds}`、HTML 标签、数字、布尔值和 null。

共 12 条非空源文案，两个目标语言，每批 3 条。首次运行预计翻译 **24 条，分为 8 个批次，生成 4 个目标文件**。

在本工程根目录构建并预览，无需 API Key，不调用 API、不写文件：

```sh
make build
./bin/i18n-cli translate --config examples/demo/i18n.config.json --dry-run
```

复制 DeepSeek 环境变量模板，在 `.env` 中填写 API Key 后执行实际翻译：

```sh
cp -n .env.example .env
# Edit .env and replace your-deepseek-api-key with your actual key.

./bin/i18n-cli translate --config examples/demo/i18n.config.json
```

输出文件位置：

```text
examples/demo/
├── .i18n-cli-state.json
└── locales/
    ├── en/       # 原始文件，保持不变
    ├── zh-CN/    # common.json 和 observability.json
    └── ja/       # common.json 和 observability.json
```

检查输出后再次执行同一命令，预计翻译数量为 0，不调用 API。修改 `examples/demo/locales/en/common.json` 中 `navigation.overview` 的值后重跑，预计只翻译该条文案的两个目标语言版本。追加 `--force` 可全量重译；首次实际翻译和强制重译均会消耗 API 用量。

`examples/demo` 下生成的翻译和增量状态已加入忽略规则，确保新克隆的示例可从首次翻译开始测试。实际业务仓库应将翻译文件和增量状态一起提交。

## .env 与 DeepSeek

CLI 自动读取**当前工作目录**中的 `.env`，即使 `--config` 指向其他目录也是如此。不会向上查找，也不会自动读取配置文件目录的 `.env`。文件不存在时可以正常使用环境变量或配置；加载过程不会修改进程环境变量。

```dotenv
OPENAI_API_KEY=your-deepseek-api-key
OPENAI_BASE_URL=https://api.deepseek.com
OPENAI_MODEL=deepseek-flash
```

以上设置来自当前 [DeepSeek 官方文档](https://api-docs.deepseek.com/)。所有兼容提供商仍使用统一的 `OPENAI_*` 变量名，demo 配置也已改为 DeepSeek。自动验证未调用真实 DeepSeek API。

只加载上述三个键。支持空行、注释、可选 `export` 前缀、无引号值和单行单/双引号值。双引号支持 Go 风格转义，单引号内容按原文读取；无引号值中的行内注释以空白之后的 `#` 开始。不展开变量，也不执行 shell 命令。其他键忽略，同一个支持的键在文件内以最后一次赋值为准。格式错误只报告行号，不打印值。

已导出的非空环境变量优先于 `.env`。如果之前设置过 OpenAI 环境变量，可以清除后使用文件里的 DeepSeek 配置：

```sh
unset OPENAI_API_KEY OPENAI_BASE_URL OPENAI_MODEL
./bin/i18n-cli translate --config examples/demo/i18n.config.json
```

`.env` 已被 Git 忽略，`.env.example` 只包含占位值。切换提供商后，如需重新翻译已有 demo 输出，请追加 `--force`。

## 配置文件

复制 `i18n.config.example.json` 为业务仓库根目录的 `i18n.config.json`，然后在业务仓库执行：

```sh
/path/to/i18n-cli translate --dry-run
/path/to/i18n-cli translate
# 也可以从任何目录显式指定配置
/path/to/i18n-cli translate --config /path/to/app/i18n.config.json
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

- 配置结构独立定义，`version` 为整数 `1`，未知字段报错。
- `provider.id` 支持 `openai` / `openai-compatible`，都使用相同 HTTP 协议。
- `provider.prompt` 中的 `{source}`、`{target}` 会替换为当前语言；它附加在强制结构/占位符保留规则后面。
- `include` 相对于配置文件所在目录；每条规则必须含一个 `[locale]`。支持 `*`、`?`、`**`，不支持其他 glob 扩展。每条规则至少匹配一个源 JSON。
- 单文件：`src/locales/[locale].json`；多文件：`src/i18n/[locale]/**/*.json`；可同时写多条规则。
- 语言名保留大小写，必须与实际目录/文件一致，不自动映射 `en` 和 `en-us`。
- 优先级：显式 CLI 参数 > 非空进程环境变量 > `.env` 中的非空值 > 配置文件 > 内置默认值。
- `--dir` 可覆盖规则解析根目录；没有配置时默认 `locales`。状态和运行锁保存在该根目录。
- API Key 从进程环境变量或 `.env` 中的 `OPENAI_API_KEY` 读取，不写入配置或状态。

## MoleSignal

`examples/molesignal.config.json` 对应当前 `web/src/i18n/en-us/*.json` 结构。把它复制为 MoleSignal 根目录的 `i18n.config.json` 后执行：

```sh
/path/to/i18n-cli translate --config /path/to/molesignal/i18n.config.json --dry-run
/path/to/i18n-cli translate --config /path/to/molesignal/i18n.config.json --adopt-existing
```

`--adopt-existing` 在首次接入时信任现有目标文案，只翻译缺失或占位符不一致的条目，并建立源文案基线。它无法判断现有翻译是否已经过时；有此疑虑时省略该选项，首次全部翻译。后续源文案或提示词变化仍会触发翻译。

## 增量与可靠性

- `.i18n-cli-state.json` 记录源语言、目标语言、文件、JSON Pointer、源文案及提示词哈希。请与翻译 JSON 一起提交。
- 每个字符串单独判断变更；缺失目标、空翻译或占位符损坏会重新翻译。人工修订的有效目标文本保留。
- 支持嵌套对象和数组。数字、布尔值、null 继承源文件；空字符串不调用 API。对象键按 Go JSON 编码器稳定排序，首次运行可能产生格式差异。
- 源文件中已删除的键会从相应目标 JSON 移除；已删除的整个源文件所对应的目标文件保留，需手动清理。
- `{{name}}`、`${name}`、`{name}`、常见 printf 占位符和 HTML 标签按数量与内容校验。复杂 ICU plural/select 明确拒绝；该校验并非完整的 HTML/Markdown/ICU 解析器。
- API 输出必须是精确的 JSON 键集合，重复键、非字符串、截断响应、占位符变化均报错。
- 调用前预检所有文档，所有 API 响应成功并校验后开始写入。单文件采用同目录临时文件 + rename；状态最后写入。磁盘故障/进程中止可能造成部分文件已写，重新运行可恢复，但可能重复消费 API。
- 根目录锁阻止同根并发运行。强制终止后，仅在确认无运行进程时删除 `.i18n-cli.lock`。
- HTTP 429/5xx 使用有上限的指数退避；其他 HTTP 错误直接失败。翻译 JSON、键集合、值类型或占位符不合要求时最多纠正重试两次（共三次模型响应），仍失败则报告缺失键和多余键，不打印翻译值。纠正重试会增加 API 用量，每次尝试分别遵守 HTTP 重试上限。错误响应体不打印；禁止 HTTP 重定向以保护 API Key。
- 批次按字符串数量划分，不估算 token；长文案请降低 `--batch-size`。不同提供商的模型、参数和上下文限制需自行确认。

## 参数

| 参数 | 用途 |
|---|---|
| `--config` | 配置路径，默认当前目录 `i18n.config.json` |
| `--source`, `--target` | 覆盖源语言和逗号分隔目标语言 |
| `--dir` | 文件规则根目录 |
| `--dry-run` | 统计待翻译条目，不请求 API、不写文件、无需 Key |
| `--adopt-existing` | 首次信任现有有效翻译 |
| `--force` | 重译全部非空字符串，与 adopt 互斥 |
| `--instructions` | 附加 UTF-8 术语表/风格提示文件，相对当前工作目录 |
| `--base-url`, `--model` | 覆盖提供商配置 |
| `--json-mode=false` | 不发送 response_format，兼容不支持 JSON mode 的服务 |
| `--allow-no-key` | 显式允许本地无认证服务 |
| `--batch-size` | 每批 1–200 条，默认 40 |
| `--timeout` | 单请求超时，默认 90s |
| `--retries` | HTTP 429/5xx 重试次数，默认 3，最多 6 |

协议使用 [`POST /chat/completions`](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)，附加在 Base URL 后。Base URL 需要自行包含服务所需的 `/v1`。DeepSeek、Qwen、vLLM、LiteLLM 等服务可通过 Base URL/Model 切换；本项目测试使用本地模拟 API，未宣称逐一通过真实提供商验证。

## GitHub Actions 与 Pull Request

```yaml
- uses: actions/checkout@v4
- uses: molesignal/i18n-cli@v0.2.0
  with:
    config: i18n.config.json
    api-key: ${{ secrets.OPENAI_API_KEY }}
    base-url: https://api.deepseek.com
    model: deepseek-flash
```

Action 支持 `config`、`api-key`、`base-url`、`model` 和 `dry-run` 输入。设置 `dry-run: 'true'` 可仅预览，不调用 API 或写文件。Action 翻译检出的仓库文件，自动提交可使用下方 PR 工作流。

工程提供 `action.yml` composite action 和 `examples/translate.workflow.yml` 消费端工作流。使用公开仓库 `molesignal/i18n-cli`，然后：

1. 将配置保存为业务仓库 `i18n.config.json`，工作流复制到 `.github/workflows/translate.yml`。
2. 设置仓库 Secret `OPENAI_API_KEY`；Variables `I18N_CLI_REPOSITORY`（`molesignal/i18n-cli`）、`I18N_CLI_REF`（`v0.2.0` 或固定 commit SHA）。可选 `OPENAI_BASE_URL`、`OPENAI_MODEL`。
3. 按实际配置修改工作流源路径、恢复目标路径、PR add-paths 和默认分支。示例已匹配 MoleSignal 的 en-us/zh-cn/ja-jp/ko-kr。
4. 允许 GitHub Actions 创建 Pull Request，确保组织策略允许 contents/pull-requests 写权限。私有 CLI 仓库需要具有读取权限的单独 checkout token。
5. 推送源 JSON 或手动触发工作流。工作流恢复尚未合并的翻译分支状态，增量更新同一个 `automation/i18n` PR。

不要在不可信 PR 的代码上运行带 API Key 的工作流。示例仅由 main push 或手动触发。`GITHUB_TOKEN` 创建的 PR 通常不会触发后续 PR 工作流，需要该行为时配置权限适当的 GitHub App token。生产使用建议将第三方 Actions 固定到审核过的 commit SHA。

CLI 不隐式执行 git commit/push。Actions 的 PR 步骤会自动提交指定的翻译文件和状态；本地可正常用 Git 审阅后提交。远程仓库发布、Secret 设置和实际翻译需要分别完成。

## 开发验证

```sh
make test
make build
```

测试覆盖嵌套结构、增量、删除键、已有翻译接入、配置覆盖、dry-run、失败不落盘、占位符校验、重复 JSON key、HTTP 重试和重定向保护。

## 终端进度

macOS/Linux 的交互终端显示原地刷新的紧凑面板：已校验字符串的进度条、当前目标语言和文件、批次位置及耗时。API 工作期间每 100 毫秒更新动画和计时。过长路径会根据终端宽度截断。完成后面板替换为保存成功的汇总；dry-run 和失败状态使用各自的汇总信息。

```text
  i18n-cli
  ⠹ Translating  ·  en → 2 languages

  ━━━━━━━━━━━━────────────   50%  12 / 24 strings
  ja  ·  file 3/4  ·  batch 1/2
  locales/ja/common.json
  00:03 elapsed  ·  0.9s batch
```

```text
  i18n-cli
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

翻译数量按每种目标语言的字符串条目计数，文件数量按目标文件任务计数。保留的既有翻译和空字符串不计入新增翻译数量。只有翻译文件和增量状态保存成功后才显示成功。

完成后的表格按目标语言汇总：`Files` 仅统计本次有新增翻译且完成全部批次的文件，跳过的文件不计入；`Strings` 为本次通过校验的翻译条目数；`Time` 为 API 批次耗时之和，包含提供商重试，不含预检和写盘时间。没有变化的语言显示零值。该表格仅在成功保存后显示，终端面板和纯文本日志均支持。

重定向、CI、不支持的平台和 `TERM=dumb` 使用逐行纯文本日志，等待期间每 5 秒输出心跳。`NO_COLOR=1` 关闭颜色，在支持的终端上仍保留动态面板。所有进度文案均为英文。

## 许可证

Copyright (c) 2026 MoleSignal Authors.

本项目采用 Apache License, Version 2.0，完整英文许可条款见 [LICENSE](LICENSE)。
