package hook

// Output is the JSON a hook command writes to stdout on exit 0.
type Output struct {
	// Universal fields
	Continue       *bool  `json:"continue,omitempty"`
	StopReason     string `json:"stopReason,omitempty"`
	SuppressOutput bool   `json:"suppressOutput,omitempty"`
	SystemMessage  string `json:"systemMessage,omitempty"`

	// Top-level decision (PostToolUse, Stop, UserPromptSubmit, etc.)
	Decision string `json:"decision,omitempty"`
	Reason   string `json:"reason,omitempty"`

	// Event-specific output
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// HookSpecificOutput carries event-specific decision fields.
type HookSpecificOutput struct {
	HookEventName string `json:"hookEventName"`

	// PreToolUse
	PermissionDecision       string         `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string         `json:"permissionDecisionReason,omitempty"`
	UpdatedInput             map[string]any `json:"updatedInput,omitempty"`

	// PreToolUse, PostToolUse, SessionStart, UserPromptSubmit, etc.
	AdditionalContext string `json:"additionalContext,omitempty"`
}
