## 基于IM的飞书智能协同助手

### CLI 扫描式 Agent demo（不依赖 skills，仅扫描 `lark-cli --help`）

这个 demo 不读 `SKILL.md`，而是：
- 从环境里定位 `lark-cli`（默认 `PATH`，可用 `LARK_CLI_BIN` 覆盖）
- 递归运行 `lark-cli <...> --help` 扫描出可用命令列表（含一行描述）
- 把“命令列表”作为 prompt context，让模型挑选要执行的 **一条** `lark-cli` 命令
- 执行后输出 stdout/stderr

运行：

```bash
export OPENAI_API_KEY="..."
export OPENAI_MODEL="..."
go run ./cmd/lark-cli-agent
```

可选：

```bash
export OPENAI_BASE_URL="https://api.openai.com/v1"
export LARK_CLI_BIN="lark-cli"
export LARK_CLI_SCAN_DEPTH=2
export LARK_CLI_SCAN_LIMIT=220
```