# claudetool

A companion binary for Claude Code. Provides a status line and hook handlers.

```
go install github.com/laurenkt/claudetool@latest
```

## Setup

`go install` is all you need — no clone. It drops a `claudetool` binary in your
`GOBIN` (usually `~/go/bin`).

1. **Put `~/go/bin` on your `PATH`** (`go env GOPATH`/bin) so `claudetool` is found.
2. **Wire the hooks into `~/.claude/settings.json`** — merge the entries below
   into your existing config (don't overwrite it). If Claude Code can't find
   `claudetool` (hooks don't always inherit your shell `PATH`), use the absolute
   path, e.g. `/Users/you/go/bin/claudetool hook …`.

For the permission-prompt-reduction chain, see
[below](#permission-prompt-reduction). Hooks take effect immediately, including
in a running session.

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
| `no-git-c` | PreToolUse | `Bash` | Blocks `git -C <path>`; tells the agent to run plain `git ...` since the command already runs in the working directory (no `-C`, no `cd` needed) |
| `use-ripgrep` | PreToolUse | `Bash` | Blocks `grep`/`egrep`/`fgrep`; points at `rg`, which recurses and respects `.gitignore` by default |
| `use-linear-mcp` | PreToolUse | `Bash\|WebFetch` | Blocks gh/curl/wget/`linear-cli` calls and WebFetch to `linear.app`, points to MCP |
| `no-shared-pr-body` | PreToolUse | `Bash` | Blocks commands touching the fixed `/tmp/pr-body.md` path — concurrent agents clobber each other's PR body; points to `mktemp` / `--body-file -` |
| `no-prod` | PreToolUse | `Bash` | Blocks `£ -e prod`/`--environment production` invocations of the Monzo CLI against prod; tells the agent to have the human operator run it and paste results back. Lower envs (`£ -e s101 ...`) are allowed |
| `semgrep-check` | PostToolUse | `Write\|Edit` | Runs semgrep, blocks on findings |
| `go-augment-style` | PostToolUse | `Write\|Edit` | Blocks verbose `terrors.Augment` context strings (e.g. "failed to read" → "read") |
| `redirect-writes` | PreToolUse | `Write\|Edit` | Rewrites file paths (set `REDIRECT_FROM` and `REDIRECT_TO` env vars) |
| `backend101` | PreToolUse | `Write\|Edit` | Enforces Backend 101 naming: firehose consumers in `consumer/consumer.go`, streams consumers in `streamsconsumer/consumer.go`, handler functions in matching snake_case files |
| `go-fix` | PostToolUse | `Write\|Edit` | Runs `go fix` on the package containing the edited `.go` file; applies modernizations and reports the diff back to Claude |
| `go-makeslice` | PostToolUse | `Write\|Edit` | Advisory (non-blocking): suggests `var x []T` instead of `make([]T, 0, n)` |
| `go-named-func` | PostToolUse | `Write\|Edit` | Advisory (non-blocking): flags named anonymous functions (`name := func(...) {...}`); skips `_test.go` |
| `go-cmp-or` | PostToolUse | `Write\|Edit` | Advisory (non-blocking): flags hand-rolled first-non-empty-string selectors (a value returned under an `x != ""` guard with a `return ""` fallback); points at `cmp.Or` |
| `valuable-comments` | PostToolUse | `Write\|Edit` | **Async** ([see below](#async-review-hooks)): dispatches the changed `.go` code to a headless `claude` reviewer in the background; wakes the agent only when a comment restates the code, the name, or something that belongs in a validator |
| `change-detector-tests` | PostToolUse | `Write\|Edit` | **Async** ([see below](#async-review-hooks)): reviews changed `_test.go` files and pushes back on change-detector tests (coupled to the implementation, not behaviour); leaves legitimate ones alone (codegen golden files, parsing/round-trip, characterization, security tripwires) |
| `rpc-wrapper` | PostToolUse | `Write\|Edit` | **Async** ([see below](#async-review-hooks)): flags functions that are pointless thin wrappers around a single generated-client RPC call (`Request{…}.Send(ctx).DecodeResponse()` + trivial error-wrap + return a field); leaves wrappers that transform, orchestrate, or back an interface alone; skips `_test.go` |
| `ci-watch` | PostToolUse | `Bash` | **Async** ([see below](#ci-watch)): after a `git push` / `gh pr create` / `gh pr ready`, watches the PR's CI checks in the background for up to 30 min and wakes the agent only if a check fails |
| `auto-approve-readonly` | PreToolUse | `Bash` | Auto-approves a command when *every* segment is read-only: cd/ls/cat/grep/find/jq, `awk`/`sed` (guarded — no write/exec/`-i`), git read subcommands, `gh` read subcommands + GET-only `gh api`, `bq` metadata `show`/`ls`/`head`, `docker` read subcommands, read-only data-shell verbs. Quote-aware (pipes inside a `jq`/`awk` program don't fool it); a standalone `VAR=value` assignment is treated as inert. See [below](#permission-prompt-reduction) |
| `strip-redundant-cd` | PreToolUse | `Bash` | Rewrites the command to drop a leading `cd <dir>` when `<dir>` already equals the working directory (a no-op that otherwise trips the "changes directory before running git / cd + output redirection" gate) |
| `no-shell-loops` | PreToolUse | `Bash` | Blocks `for`/`while`/`until` loops; points at running each iteration as its own command or using the Read/Grep tools |
| `no-inline-python` | PreToolUse | `Bash` | Blocks inline `python[23] -c '…'`; points at `jq` for JSON and the Read/Grep tools for files |
| `no-inline-file` | PreToolUse | `Bash` | Blocks heredocs and content-building command substitution `$(cat/printf/echo …)`; points at writing the content with a file tool and referencing it (`< file`, `git commit -F`, `gh pr create --body-file`) |
| `no-command-substitution` | PreToolUse | `Bash` | Blocks command substitution `$(…)`/backticks (arithmetic `$((…))` allowed); points at running the inner command as its own step and using its output explicitly (e.g. `docker ps -q …` then `docker inspect <ids>`) |
| `no-exit-code-check` | PreToolUse | `Bash` | Blocks `$?` exit-code scaffolding (e.g. `echo "EXIT=$?"`); points at running the command and reading its output |

### Permission-prompt reduction

`auto-approve-readonly` plus the `no-*`/`strip-*` handlers above form a chain that
cuts Claude Code's manual-approval prompts down to the ones actually worth a look.
The idea: an interactive agent generates a lot of *unanalyzable* shell — `cd`
prefixes, `for` loops, inline `python -c`, heredocs, `$(cat …)`/`$(printf …)`,
`$?` checks — and each defeats the permission allowlist even when the underlying
tool is trusted. So:

- **Reads are auto-approved.** `auto-approve-readonly` returns an `allow` decision
  when the whole command is read-only, so harmless exploration never prompts. It
  can only ever turn a prompt into an approval, never block, and never approves a
  write, a redirect to a real file, command substitution, or an unknown command.
- **Unanalyzable anti-patterns are blocked with a fix.** The `no-*` handlers refuse
  the roundabout form and name the analyzable rewrite — which matches the allowlist
  and runs without a prompt.
- **A redundant `cd` is rewritten away** by `strip-redundant-cd` (a no-op `cd` only).

Chain them in one entry (order matters — put `strip-redundant-cd` first):

```json
{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "claudetool hook strip-redundant-cd auto-approve-readonly no-shell-loops no-inline-python no-inline-file no-command-substitution no-exit-code-check"}]}
    ]
  }
}
```

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

Two such checks ship today:

- `valuable-comments` judges whether comments in the changed code add value
  (explain *why*, cite scheme/schema docs or examples, clarify what names and
  validators can't) versus restate the implementation, the name, or a rule that
  belongs in a proto/go validator.
- `change-detector-tests` reviews changed `_test.go` files and pushes back on
  change-detector tests — ones coupled to the implementation rather than
  observable behaviour, which break on any refactor and catch no real bug. It
  deliberately leaves the legitimate cases alone (codegen golden files,
  message/wire-format round-trips, characterization tests, security/config
  tripwires).
- `rpc-wrapper` flags functions that are pointless thin wrappers around a
  single generated-client RPC call — build one request, `Send().DecodeResponse()`,
  trivial error-wrap, return a field — where inlining the call at the caller
  would lose nothing. It leaves wrappers that transform inputs/outputs,
  orchestrate multiple calls, add retry/caching, or implement an interface
  method alone.

Because `asyncRewake` is configured **per hook**, an async handler must be its
own hook entry, not appended to a synchronous chain:

```json
{
  "hooks": {
    "PostToolUse": [
      {"matcher": "Write|Edit", "hooks": [
        {"type": "command", "command": "claudetool hook go-augment-style backend101 go-fix"},
        {"type": "command", "command": "claudetool hook valuable-comments", "asyncRewake": true},
        {"type": "command", "command": "claudetool hook change-detector-tests", "asyncRewake": true},
        {"type": "command", "command": "claudetool hook rpc-wrapper", "asyncRewake": true}
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

### ci-watch

`ci-watch` reuses the same `asyncRewake` delivery as the review hooks, but
instead of dispatching a reviewer it watches the PR's CI. When the agent runs a
`git push`, `gh pr create`, or `gh pr ready`, the hook looks up the PR for the
current branch (`gh pr view`) and blocks on `gh pr checks --watch --fail-fast`
in the background. CI at Monzo takes 8–30+ minutes, so without this the agent
moves on and the human finds the red build later; here the hook wakes the agent
(exit 2) on the **first** failing check while the context is still fresh.

- **Failure-only**: silent on success — only a failed check wakes the agent.
- **No PR yet**: a bare push before `gh pr create` finds no PR and does nothing.
- **Timeout**: gives up silently after 30 min — it never reports a false failure.
- **Preemption**: a new push for the same branch kills the previous watcher (via
  a pidfile under `~/.claude/ci-watch/`), so only the latest push is watched.

Because `asyncRewake` is per hook, `ci-watch` is its own `Bash` entry with a
timeout matching its 30-min watch window (not chained onto a synchronous hook):

```json
{
  "hooks": {
    "PostToolUse": [
      {"matcher": "Bash", "hooks": [
        {"type": "command", "command": "claudetool hook ci-watch", "asyncRewake": true, "timeout": 1800}
      ]}
    ]
  }
}
```

Requires `gh` on `PATH` and authenticated. A PR opened via the GitHub web UI
(rather than `gh`) after a bare push is not detected, and branch CI without a PR
is not watched — both by design.

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
