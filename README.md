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
claudetool hook <handler> [<handler>...] [flags]
```

Multiple handlers run in sequence against the same stdin. The chain stops at
the first error or `Decision: "block"`; otherwise non-blocking outputs are
merged (e.g. `additionalContext` strings are concatenated). Positional args
before the first `-flag` are treated as handler names; everything from the
first flag onward is the flag tail, passed to every handler so flag-driven
ones like `dump -o <path>` keep working. Handler names cannot begin with `-`.

### Available handlers

| Handler | Event | Matcher | What it does |
|---|---|---|---|
| `no-cd` | PreToolUse | `Bash` | Blocks `cd` commands |
| `use-linear-mcp` | PreToolUse | `Bash\|WebFetch` | Blocks gh/curl/wget/`linear-cli` calls and WebFetch to `linear.app`, points to MCP |
| `semgrep-check` | PostToolUse | `Write\|Edit` | Runs semgrep, blocks on findings |
| `go-augment-style` | PostToolUse | `Write\|Edit` | Blocks verbose `terrors.Augment` context strings (e.g. "failed to read" → "read") |
| `redirect-writes` | PreToolUse | `Write\|Edit` | Rewrites file paths (set `REDIRECT_FROM` and `REDIRECT_TO` env vars) |
| `backend101` | PreToolUse | `Write\|Edit` | Enforces Backend 101 naming: firehose consumers in `consumer/consumer.go`, streams consumers in `streamsconsumer/consumer.go`, handler functions in matching snake_case files |
| `go-fix` | PostToolUse | `Write\|Edit` | Runs `go fix` on the package containing the edited `.go` file; applies modernizations and reports the diff back to Claude |
| `go-makeslice` | PostToolUse | `Write\|Edit` | Advisory (non-blocking): suggests `var x []T` instead of `make([]T, 0, n)` |
| `go-named-func` | PostToolUse | `Write\|Edit` | Advisory (non-blocking): flags named anonymous functions (`name := func(...) {...}`); skips `_test.go` |
| `valuable-comments` | PostToolUse | `Write\|Edit` | **Async** ([see below](#async-review-hooks)): dispatches the changed `.go` code to a headless `claude` reviewer in the background; wakes the agent only when a comment restates the code, the name, or something that belongs in a validator |

### Example settings.json

```json
{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "claudetool hook no-cd no-diff-master"}]},
      {"matcher": "Bash|WebFetch", "hooks": [{"type": "command", "command": "claudetool hook use-linear-mcp"}]}
    ],
    "PostToolUse": [
      {"matcher": "Write|Edit", "hooks": [{"type": "command", "command": "claudetool hook go-augment-style backend101 go-fix"}]}
    ]
  }
}
```

### Async review hooks

Most handlers decide inline and the agent waits for them. An *async review* hook
instead dispatches the change to a separate, headless `claude` instance for
evaluation **in the background**, and only interrupts the working agent if that
reviewer has something to say.

This relies on Claude Code's per-hook [`asyncRewake`](https://code.claude.com/docs/en/hooks):
the hook runs without blocking the agent, and exiting with code 2 wakes the
agent with the hook's stderr as a system reminder. So the handler returns:

- **no findings** → exit 0, silent — the agent is never interrupted;
- **findings** → exit 2 with feedback → the agent is woken (a few seconds later)
  and can go back and revise.

`valuable-comments` is the first such check. It judges whether comments in the
changed code add value (explain *why*, cite scheme/schema docs or examples,
clarify what names and validators can't) versus restate the implementation, the
name, or a rule that belongs in a proto/go validator.

Because `asyncRewake` is configured **per hook**, an async handler must be its
own hook entry, not appended to a synchronous chain:

```json
{
  "hooks": {
    "PostToolUse": [
      {"matcher": "Write|Edit", "hooks": [
        {"type": "command", "command": "claudetool hook go-augment-style backend101 go-fix"},
        {"type": "command", "command": "claudetool hook valuable-comments", "asyncRewake": true}
      ]}
    ]
  }
}
```

The reviewer model is a configurable tier — `FastCheap` (haiku),
`MediumBalanced` (sonnet, the default), `SlowAccurate` (opus) — overridable per
invocation with a flag: `claudetool hook valuable-comments -tier fast`.

New async checks are a few lines: configure an `asyncReview` (file suffix,
default tier, a cheap `precheck` gate, and a `rubric`) and register its
`handler()`. See `internal/hook/handler_asyncreview.go` and
`internal/hook/handler_valuable_comments.go`.

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
