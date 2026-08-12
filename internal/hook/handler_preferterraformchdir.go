package hook

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

func init() {
	Register("prefer-terraform-chdir", handlePreferTerraformChdir)
}

// terraformCdPassengers are commands whose behaviour does not depend on the
// working directory, so their presence alongside terraform does not justify a
// cd. They also keep working with an absolute redirect once the cd is gone.
var terraformCdPassengers = map[string]bool{
	"echo": true, "printf": true, "pwd": true, "true": true, "false": true, ":": true,
}

// handlePreferTerraformChdir blocks a Bash command that changes directory and
// runs terraform with an output redirection, nudging the agent to use
// terraform's own -chdir flag instead of cd-ing into the directory.
//
// A `cd <dir> && terraform ... >log` trips Claude Code's built-in "cd with an
// output redirect" gate — it can't tell which directory the redirect target
// resolves against after the cd, so it forces a manual approval prompt.
// `terraform -chdir=<dir> ...` removes the cd entirely: the gate never fires,
// the redirect on its own is harmless, and the command can match a `terraform`
// allowlist and run with no prompt at all.
//
// It never approves — it only blocks — and stays silent unless every non-cd
// segment is terraform (or an inert passenger like echo). The moment a segment
// might genuinely need the working directory (a script, a relative path, make,
// dbt) it stays silent, so legitimate cd use is untouched. It also requires an
// output redirection to be present: without one, `cd <dir> && terraform ...`
// does not trip the gate, so there is nothing to fix.
func handlePreferTerraformChdir(in *Input) (*Output, error) {
	if in.HookEventName != "PreToolUse" || in.ToolName != "Bash" {
		return nil, nil
	}
	var bash BashInput
	if err := json.Unmarshal(in.ToolInput, &bash); err != nil {
		return nil, nil
	}
	dir, ok := terraformCdIsChdirable(bash.Command)
	if !ok {
		return nil, nil
	}
	// A cd whose target is already the working directory is a no-op that
	// strip-redundant-cd rewrites away; leave it for that handler.
	if in.CWD != "" && dir != "" && filepath.Clean(dir) == filepath.Clean(in.CWD) {
		return nil, nil
	}

	target := dir
	if target == "" {
		target = "<dir>"
	}
	return nil, fmt.Errorf("blocked: a 'cd' combined with an output redirection trips Claude Code's \"cd with an output redirect\" gate (it can't resolve the redirect target against the post-cd directory), forcing a manual prompt. Drop the cd and give terraform its own working directory with -chdir: run 'terraform -chdir=%s <subcommand> ...' for each terraform call — e.g. 'terraform -chdir=%s init -backend=false -input=false >/tmp/tf_init.log 2>&1'. With the cd gone the redirect is harmless and the command matches your terraform allowlist. (This only fires when every non-cd step is terraform; cd is left alone when a later step genuinely needs the working directory.)", target, target)
}

// terraformCdIsChdirable reports whether command changes directory and runs
// terraform with an output redirection, with only inert passengers alongside,
// and returns the first cd target. It returns ok=false the moment a segment
// could depend on the working directory, or when there is no cd, no terraform,
// or no redirection.
func terraformCdIsChdirable(command string) (string, bool) {
	var cdDir string
	sawCD, sawTerraform := false, false
	for _, seg := range splitSegments(command) {
		cmd, args := commandWord(seg)
		switch {
		case cmd == "":
			// empty segment or a bare VAR=value assignment — inert
		case cmd == "cd":
			sawCD = true
			if cdDir == "" && len(args) > 0 {
				cdDir = unquote(args[0])
			}
		case cmd == "terraform":
			sawTerraform = true
		case terraformCdPassengers[cmd]:
			// inert passenger; does not need the working directory
		default:
			return "", false // something that might need the working directory
		}
	}
	if !sawCD || !sawTerraform || !hasTopLevelRedirect(command) {
		return "", false
	}
	return cdDir, true
}

// hasTopLevelRedirect reports whether command contains an output redirection
// (>, >>, or 2>&1) outside of quotes.
func hasTopLevelRedirect(command string) bool {
	masked, _ := maskQuotes(command)
	return strings.ContainsRune(masked, '>')
}
