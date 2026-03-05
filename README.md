# claudetool

A companion binary for Claude Code. Provides a status line and hook handlers.

```
go install github.com/laurenkt/claudetool@latest
```

## Status line

Reads Claude Code JSON from stdin and outputs a formatted terminal status line (directory, branch, model, cost, context usage).

```json
{"hooks": {"Status": [{"hooks": [{"type": "command", "command": "claudetool statusline"}]}]}}
```

## Hooks

Hook handlers that read Claude Code hook JSON from stdin and return structured decisions on stdout.

```
claudetool hook <handler>
```

### Available handlers

| Handler | Event | Matcher | What it does |
|---|---|---|---|
| `no-cd` | PreToolUse | `Bash` | Blocks `cd` commands |
| `use-linear-mcp` | PreToolUse | `Bash` | Blocks gh/curl/wget calls to Linear, points to MCP |
| `semgrep-check` | PostToolUse | `Write\|Edit` | Runs semgrep, blocks on findings |
| `go-augment-style` | PostToolUse | `Write\|Edit` | Blocks verbose `terrors.Augment` context strings (e.g. "failed to read" → "read") |
| `redirect-writes` | PreToolUse | `Write\|Edit` | Rewrites file paths (set `REDIRECT_FROM` and `REDIRECT_TO` env vars) |

### Example settings.json

```json
{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "claudetool hook no-cd"}]},
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "claudetool hook use-linear-mcp"}]}
    ],
    "PostToolUse": [
      {"matcher": "Write|Edit", "hooks": [{"type": "command", "command": "claudetool hook semgrep-check"}]},
      {"matcher": "Write|Edit", "hooks": [{"type": "command", "command": "claudetool hook go-augment-style"}]}
    ]
  }
}
```

### Adding a hook

Create `internal/hook/handler_yourname.go`:

```go
func init() { Register("your-name", handleYourName) }

func handleYourName(in *Input) (*Output, error) {
    // return nil, nil to allow
    // return nil, fmt.Errorf("reason") to block (exit 2)
    // return &Output{...}, nil for structured responses
}
```
