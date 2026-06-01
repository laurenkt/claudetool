package hook

import "encoding/json"

// Input is the JSON sent by Claude Code on stdin to a hook command.
// Common fields are always present; event-specific fields are decoded per-event.
// RawJSON contains the original unparsed bytes (set by Run, not from JSON).
type Input struct {
	RawJSON        json.RawMessage `json:"-"`
	Args           []string        `json:"-"`
	SessionID      string          `json:"session_id"`
	TranscriptPath string          `json:"transcript_path"`
	CWD            string          `json:"cwd"`
	PermissionMode string          `json:"permission_mode"`
	HookEventName  string          `json:"hook_event_name"`

	// Tool events (PreToolUse, PostToolUse, PostToolUseFailure, PermissionRequest)
	ToolName  string          `json:"tool_name,omitempty"`
	ToolInput json.RawMessage `json:"tool_input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`

	// PostToolUse
	ToolResponse json.RawMessage `json:"tool_response,omitempty"`

	// PostToolUseFailure
	Error       string `json:"error,omitempty"`
	IsInterrupt bool   `json:"is_interrupt,omitempty"`

	// UserPromptSubmit
	Prompt string `json:"prompt,omitempty"`

	// Stop, SubagentStop
	StopHookActive bool `json:"stop_hook_active,omitempty"`

	// SubagentStart, SubagentStop
	AgentID             string `json:"agent_id,omitempty"`
	AgentType           string `json:"agent_type,omitempty"`
	AgentTranscriptPath string `json:"agent_transcript_path,omitempty"`

	// SessionStart
	Source string `json:"source,omitempty"`
	Model  string `json:"model,omitempty"`

	// SessionEnd
	Reason string `json:"reason,omitempty"`

	// Notification
	Message          string `json:"message,omitempty"`
	Title            string `json:"title,omitempty"`
	NotificationType string `json:"notification_type,omitempty"`

	// PreCompact
	Trigger            string `json:"trigger,omitempty"`
	CustomInstructions string `json:"custom_instructions,omitempty"`
}

// BashInput is the tool_input for Bash tool calls.
type BashInput struct {
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
}

// WriteInput is the tool_input for Write tool calls.
type WriteInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

// EditInput is the tool_input for Edit tool calls.
type EditInput struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

// WebFetchInput is the tool_input for WebFetch tool calls.
type WebFetchInput struct {
	URL    string `json:"url"`
	Prompt string `json:"prompt,omitempty"`
}
