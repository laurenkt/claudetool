package hook

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

func init() {
	Register("backend101", handleBackend101)
}

var (
	reFirehoseRegistrar        = regexp.MustCompile(`firehose\.NewRegistrar\(`)
	reStreamsTopic             = regexp.MustCompile(`\w+\.\w+StreamsTopic\b`)
	reHandlerFunc              = regexp.MustCompile(`func\s+(handle[A-Z]\w*)\s*\(`)
	reProtoStructLitSingleLine = regexp.MustCompile(`\b\w*proto\.\w+\s*\{\s*\S[^{}\n]*\}`)
	reMapLitSingleLine         = regexp.MustCompile(`\bmap\[[^\]]+\][\w*\[\]. ]*\{\s*\S(?:[^{}\n]|\{[^{}\n]*\})*\}`)
	reTimeParseRFC3339         = regexp.MustCompile(`\btime\.Parse\(\s*time\.RFC3339(?:Nano)?\b`)
)

// handleBackend101 enforces Backend 101 naming conventions:
//   - firehose.NewRegistrar() must be in consumer/consumer.go
//   - *.StreamsTopic usage must be in streamsconsumer/consumer.go
//   - func handleFoo() in consumer package must be in foo.go (CamelCase → snake_case)
//   - proto struct literal with fields must be multi-line
//   - map literal with entries must be multi-line
//   - time.Parse(time.RFC3339[Nano], ...) must use util.ProtoToTime
func handleBackend101(in *Input) (*Output, error) {
	if in.ToolName != "Write" && in.ToolName != "Edit" {
		return nil, nil
	}

	var filePath, text string

	switch in.ToolName {
	case "Write":
		var w WriteInput
		if err := json.Unmarshal(in.ToolInput, &w); err != nil {
			return nil, nil
		}
		filePath = w.FilePath
		text = w.Content
	case "Edit":
		var e EditInput
		if err := json.Unmarshal(in.ToolInput, &e); err != nil {
			return nil, nil
		}
		filePath = e.FilePath
		text = e.NewString
	}

	if !strings.HasSuffix(filePath, ".go") {
		return nil, nil
	}

	fileName := filepath.Base(filePath)
	pkgName := filepath.Base(filepath.Dir(filePath))

	// Rule 1: firehose.NewRegistrar() must be in consumer/consumer.go
	if reFirehoseRegistrar.MatchString(text) {
		if pkgName != "consumer" || fileName != "consumer.go" {
			return &Output{
				Decision: "block",
				Reason:   "Backend 101: firehose.NewRegistrar() must live in consumer/consumer.go",
			}, nil
		}
	}

	// Rule 2: *.StreamsTopic must be in streamsconsumer/consumer.go
	if reStreamsTopic.MatchString(text) {
		if pkgName != "streamsconsumer" || fileName != "consumer.go" {
			return &Output{
				Decision: "block",
				Reason:   "Backend 101: streams topic subscriptions (*.StreamsTopic) must live in streamsconsumer/consumer.go",
			}, nil
		}
	}

	// Rule 3: func handleFoo() in consumer package must be in snake_case.go
	if pkgName == "consumer" {
		matches := reHandlerFunc.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			funcName := m[1]                                 // e.g. "handleMandateUpdated"
			suffix := strings.TrimPrefix(funcName, "handle") // e.g. "MandateUpdated"
			wantFile := camelToSnake(suffix) + ".go"
			if fileName != wantFile {
				return &Output{
					Decision: "block",
					Reason:   fmt.Sprintf("Backend 101: %s should be in consumer/%s, not consumer/%s", funcName, wantFile, fileName),
				}, nil
			}
		}
	}

	// Rule 4: time.Parse(time.RFC3339[Nano], ...) duplicates util.ProtoToTime.
	if reTimeParseRFC3339.MatchString(text) {
		return &Output{
			Decision: "block",
			Reason: "Backend 101: don't reimplement RFC3339 proto-time parsing. " +
				"Use `util.ProtoToTime(s)` (returns (time.Time, error)) or " +
				"`util.ForceProtoToTime(s)` (returns time.Time, zero on empty/error) from " +
				"github.com/monzo/wearedev/libraries/util.\n\n" +
				"Replace:\n" +
				"    t, err := time.Parse(time.RFC3339Nano, s)\n" +
				"with:\n" +
				"    t, err := util.ProtoToTime(s)\n" +
				"or if you want zero-time on empty/error:\n" +
				"    t := util.ForceProtoToTime(s)",
		}, nil
	}

	// Rule 5: proto struct literal with fields must not be single-line.
	// Rule 6: map literal with entries must not be single-line.
	for i, line := range strings.Split(text, "\n") {
		if reProtoStructLitSingleLine.MatchString(line) {
			return &Output{
				Decision: "block",
				Reason: fmt.Sprintf(
					"Backend 101: proto struct literal should be multi-line (line %d: %q). "+
						"Put each field on its own line, e.g.\n"+
						"    bizumproto.ConfirmCreditRequest{\n"+
						"        Payment: confirmReq,\n"+
						"    }",
					i+1, strings.TrimSpace(line),
				),
			}, nil
		}
		if reMapLitSingleLine.MatchString(line) {
			return &Output{
				Decision: "block",
				Reason: fmt.Sprintf(
					"Backend 101: map literal should be multi-line (line %d: %q). "+
						"Put each entry on its own line, e.g.\n"+
						"    map[string]string{\n"+
						"        \"decision_code\": decisionCode,\n"+
						"        \"reason_code\":   reasonCode,\n"+
						"    }",
					i+1, strings.TrimSpace(line),
				),
			}, nil
		}
	}

	return nil, nil
}

// camelToSnake converts CamelCase to snake_case.
// MandateUpdated → mandate_updated, HTTPHandler → http_handler.
func camelToSnake(s string) string {
	var result []rune
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			if unicode.IsLower(prev) {
				// aB → a_b
				result = append(result, '_')
			} else if unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				// ABc → a_bc (end of acronym run)
				result = append(result, '_')
			}
		}
		result = append(result, unicode.ToLower(r))
	}
	return string(result)
}
