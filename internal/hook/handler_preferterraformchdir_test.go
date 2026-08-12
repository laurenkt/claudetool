package hook

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPreferTerraformChdirBlocks(t *testing.T) {
	cmds := []string{
		// The screenshot case: cd into a worktree to fmt/init/validate, with a
		// redirect on the init step — trips the cd-with-output-redirect gate.
		`cd "/Users/x/src/github.com/monzo/worktrees/analytics/pdg-3900/infrastructure/terraform/projects/connected-accounts-data" && terraform fmt -check -diff service.looker-connected-accounts.tf && terraform init -backend=false -input=false >/tmp/tf_init.log 2>&1 && terraform validate 2>&1`,
		"cd /repo && terraform init >/tmp/x.log 2>&1",
		"cd /repo && terraform validate 2>&1",
		"cd /repo && terraform plan > /tmp/plan.txt",
		// Inert passenger alongside terraform is fine.
		`cd /repo && echo "==" && terraform validate >/tmp/v.log 2>&1`,
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: c})
			code, stderr := runHandler(t, "prefer-terraform-chdir", input)
			if code != 2 {
				t.Errorf("exit = %d, want 2 (block)", code)
			}
			if !strings.Contains(stderr, "-chdir") {
				t.Errorf("stderr = %q, want -chdir guidance", stderr)
			}
		})
	}
}

func TestPreferTerraformChdirAllows(t *testing.T) {
	cmds := []string{
		// The desired rewrite — no cd, so no gate; the redirect is harmless.
		"terraform -chdir=/repo init -backend=false >/tmp/x.log 2>&1",
		"terraform -chdir=/repo validate 2>&1",
		// No redirect: cd + terraform does not trip the gate, so nothing to fix
		// (a terraform allowlist handles it).
		"cd /repo && terraform validate",
		"cd /repo && terraform fmt -check && terraform validate",
		// cd is genuinely needed by a non-terraform step — stay silent, do not
		// tell the agent to drop the cd.
		"cd /repo && terraform init >/tmp/x.log 2>&1 && ./post.sh",
		"cd /repo && cat main.tf && terraform validate 2>&1",
		"cd /repo && make plan >/tmp/x.log",
		// Not terraform at all (git is no-avoidable-cd's job).
		"cd /repo && git status >/tmp/x.log 2>&1",
		// Redirect but no terraform.
		"cd /repo && ls >/tmp/x.log",
		// Bare cd, or terraform with no cd.
		"cd /repo",
		"terraform validate",
	}
	for _, c := range cmds {
		t.Run(c, func(t *testing.T) {
			input := makeToolInput("PreToolUse", "Bash", BashInput{Command: c})
			code, _ := runHandler(t, "prefer-terraform-chdir", input)
			if code != 0 {
				t.Errorf("exit = %d, want 0 (allow/silent)", code)
			}
		})
	}
}

// A redundant cd (target == cwd) is strip-redundant-cd's job; this handler must
// stay silent so its rewrite isn't clobbered by a block.
func TestPreferTerraformChdirSkipsRedundantCd(t *testing.T) {
	in := Input{
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		CWD:           "/repo",
	}
	ti, _ := json.Marshal(BashInput{Command: "cd /repo && terraform validate 2>&1"})
	in.ToolInput = ti
	data, _ := json.Marshal(in)

	var stdout, stderr bytes.Buffer
	code := Run([]string{"prefer-terraform-chdir"}, strings.NewReader(string(data)), &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d, want 0 (silent; cd target == cwd); stderr=%q", code, stderr.String())
	}
}
