package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// semgrepHandler returns a Handler that runs semgrep with the given YAML rule
// against files written/edited by Claude. Use as a PostToolUse hook.
func semgrepHandler(ruleYAML string) Handler {
	return func(in *Input) (*Output, error) {
		filePath := extractFilePath(in)
		if filePath == "" {
			return nil, nil
		}

		findings, err := runSemgrep(ruleYAML, filePath)
		if err != nil {
			// semgrep not installed or failed; don't block
			return nil, nil
		}
		if findings == "" {
			return nil, nil
		}

		return &Output{
			Decision: "block",
			Reason:   fmt.Sprintf("semgrep found issues in %s:\n%s", filePath, findings),
		}, nil
	}
}

func extractFilePath(in *Input) string {
	if len(in.ToolInput) == 0 {
		return ""
	}
	var obj struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(in.ToolInput, &obj); err != nil {
		return ""
	}
	return obj.FilePath
}

func runSemgrep(ruleYAML, filePath string) (string, error) {
	f, err := os.CreateTemp("", "semgrep-rule-*.yml")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())

	if _, err := f.WriteString(ruleYAML); err != nil {
		f.Close()
		return "", err
	}
	f.Close()

	cmd := exec.Command("semgrep", "scan", "--quiet", "--json", "--config", f.Name(), filePath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", err
	}
	return parseSemgrepOutput(stdout.Bytes()), nil
}

func parseSemgrepOutput(data []byte) string {
	var result struct {
		Results []struct {
			CheckID string `json:"check_id"`
			Path    string `json:"path"`
			Start   struct {
				Line int `json:"line"`
			} `json:"start"`
			Extra struct {
				Message string `json:"message"`
			} `json:"extra"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return string(data)
	}
	if len(result.Results) == 0 {
		return ""
	}

	var b strings.Builder
	for _, r := range result.Results {
		fmt.Fprintf(&b, "  %s:%d: [%s] %s\n", r.Path, r.Start.Line, r.CheckID, r.Extra.Message)
	}
	return b.String()
}
