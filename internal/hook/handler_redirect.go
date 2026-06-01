package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	Register("redirect-writes", handleRedirectWrites)
}

// handleRedirectWrites intercepts file writes matching a path pattern
// and rewrites the file_path in tool_input via updatedInput.
// Use as a PreToolUse hook with matcher "Write|Edit|MultiEdit".
//
// Configuration via environment variables:
//
//	REDIRECT_FROM  - path prefix to match (e.g. "/old/generated/")
//	REDIRECT_TO    - replacement prefix (e.g. "/new/generated/")
func handleRedirectWrites(in *Input) (*Output, error) {
	from := os.Getenv("REDIRECT_FROM")
	to := os.Getenv("REDIRECT_TO")
	if from == "" || to == "" {
		return nil, nil
	}

	filePath := extractFilePath(in)
	if filePath == "" {
		return nil, nil
	}

	// Normalize paths for comparison
	from = filepath.Clean(from)
	to = filepath.Clean(to)
	filePath = filepath.Clean(filePath)

	if !strings.HasPrefix(filePath, from) {
		return nil, nil
	}

	newPath := filepath.Join(to, filePath[len(from):])

	// Build updatedInput with the redirected path
	var toolInput map[string]any
	if err := json.Unmarshal(in.ToolInput, &toolInput); err != nil {
		return nil, nil
	}
	toolInput["file_path"] = newPath

	// For Edit with edits array (MultiEdit), redirect each sub-edit too
	if edits, ok := toolInput["edits"].([]any); ok {
		for _, e := range edits {
			if edit, ok := e.(map[string]any); ok {
				if fp, ok := edit["file_path"].(string); ok {
					fp = filepath.Clean(fp)
					if strings.HasPrefix(fp, from) {
						edit["file_path"] = filepath.Join(to, fp[len(from):])
					}
				}
			}
		}
	}

	return &Output{
		HookSpecificOutput: &HookSpecificOutput{
			HookEventName:            "PreToolUse",
			PermissionDecision:       "ask",
			PermissionDecisionReason: "file path redirected from " + filePath + " to " + newPath,
			UpdatedInput:             toolInput,
			AdditionalContext:        "The file path was redirected by the redirect-writes hook.",
		},
	}, nil
}
