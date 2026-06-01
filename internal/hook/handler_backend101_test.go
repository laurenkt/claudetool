package hook

import (
	"strings"
	"testing"
)

func TestBackend101FirehoseWrongFile(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/main.go",
		Content:  `reg := firehose.NewRegistrar(ctx)`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out == nil || out.Decision != "block" {
		t.Errorf("want block, got %+v", out)
	}
}

func TestBackend101FirehoseWrongPkg(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/notconsumer/consumer.go",
		Content:  `reg := firehose.NewRegistrar(ctx)`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out == nil || out.Decision != "block" {
		t.Errorf("want block, got %+v", out)
	}
}

func TestBackend101FirehoseCorrect(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/consumer/consumer.go",
		Content:  `reg := firehose.NewRegistrar(ctx)`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow, got %+v", out)
	}
}

func TestBackend101StreamsTopicWrongPkg(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/consumer/consumer.go",
		Content:  `proto.FooStreamsTopic`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out == nil || out.Decision != "block" {
		t.Errorf("want block, got %+v", out)
	}
}

func TestBackend101StreamsTopicWrongFile(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/streamsconsumer/main.go",
		Content:  `proto.FooStreamsTopic`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out == nil || out.Decision != "block" {
		t.Errorf("want block, got %+v", out)
	}
}

func TestBackend101StreamsTopicCorrect(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/streamsconsumer/consumer.go",
		Content:  `proto.FooStreamsTopic`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow, got %+v", out)
	}
}

func TestBackend101HandlerWrongFile(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/consumer/consumer.go",
		Content:  `func handleMandateUpdated(ctx context.Context) error {`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out == nil || out.Decision != "block" {
		t.Errorf("want block, got %+v", out)
	}
	if out != nil && !strings.Contains(out.Reason, "mandate_updated.go") {
		t.Errorf("reason = %q, want mention of mandate_updated.go", out.Reason)
	}
}

func TestBackend101HandlerCorrectFile(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/consumer/mandate_updated.go",
		Content:  `func handleMandateUpdated(ctx context.Context) error {`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow, got %+v", out)
	}
}

func TestBackend101HandlerOutsideConsumerPkg(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/whatever.go",
		Content:  `func handleMandateUpdated(ctx context.Context) error {`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow (not consumer pkg), got %+v", out)
	}
}

func TestBackend101EditScansNewStringOnly(t *testing.T) {
	input := makeToolInput("PreToolUse", "Edit", EditInput{
		FilePath:  "/src/service/main.go",
		OldString: `reg := firehose.NewRegistrar(ctx)`,
		NewString: `// removed firehose registrar`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow (bad pattern only in OldString), got %+v", out)
	}
}

func TestBackend101NonGoFileIgnored(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/main.py",
		Content:  `firehose.NewRegistrar(ctx)`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow (non-.go file), got %+v", out)
	}
}

func TestBackend101NonWriteEditToolIgnored(t *testing.T) {
	input := makeToolInput("PreToolUse", "Bash", BashInput{
		Command: `firehose.NewRegistrar(ctx)`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow (non-Write/Edit tool), got %+v", out)
	}
}

func TestBackend101ProtoStructLitSingleLineSend(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/payment.go",
		Content:  `rsp, err := bizumproto.ConfirmCreditRequest{Payment: confirmReq}.Send(ctx).DecodeResponse()`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out == nil || out.Decision != "block" {
		t.Errorf("want block, got %+v", out)
	}
	if out != nil && !strings.Contains(out.Reason, "multi-line") {
		t.Errorf("reason = %q, want mention of multi-line", out.Reason)
	}
}

func TestBackend101ProtoStructLitSingleLineFirehosePublish(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/publish.go",
		Content:  `eventproto.MandateUpdatedEvent{ID: id}.FirehosePublish(ctx)`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out == nil || out.Decision != "block" {
		t.Errorf("want block, got %+v", out)
	}
}

func TestBackend101ProtoStructLitSingleLinePublish(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/publish.go",
		Content:  `streamsproto.Message{Body: b}.Publish(ctx)`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out == nil || out.Decision != "block" {
		t.Errorf("want block, got %+v", out)
	}
}

func TestBackend101ProtoStructLitMultiLine(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/payment.go",
		Content: `rsp, err := bizumproto.ConfirmCreditRequest{
	Payment: confirmReq,
}.Send(ctx).DecodeResponse()`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow, got %+v", out)
	}
}

func TestBackend101ProtoStructLitReturn(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/handler.go",
		Content:  `return &redsysbizumapiproto.ReceivePrizumDebitResponse{BizumPaymentId: rsp.BizumPaymentId}, nil`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out == nil || out.Decision != "block" {
		t.Errorf("want block, got %+v", out)
	}
}

func TestBackend101ProtoStructLitAssignment(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/handler.go",
		Content:  `req := bizumproto.ConfirmCreditRequest{Payment: p}`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out == nil || out.Decision != "block" {
		t.Errorf("want block, got %+v", out)
	}
}

func TestBackend101ProtoStructLitEmptyBraces(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/ping.go",
		Content:  `bizumproto.PingRequest{}.Send(ctx)`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow (empty braces), got %+v", out)
	}
}

func TestBackend101NonProtoStructLitAllowed(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/config.go",
		Content:  `somepkg.Config{Field: v}.Apply(ctx)`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow (non-proto pkg), got %+v", out)
	}
}

func TestBackend101CryptoNotFlagged(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/crypt.go",
		Content:  `crypto.Cipher{Key: k}.Encrypt(data)`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow (crypto != *proto), got %+v", out)
	}
}

func TestBackend101ProtoStructLitEditNewStringOnly(t *testing.T) {
	input := makeToolInput("PreToolUse", "Edit", EditInput{
		FilePath:  "/src/service/payment.go",
		OldString: `rsp, err := bizumproto.ConfirmCreditRequest{Payment: confirmReq}.Send(ctx).DecodeResponse()`,
		NewString: `// removed proto call`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow (bad pattern only in OldString), got %+v", out)
	}
}

func TestBackend101ProtoStructLitNonGoFile(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/notes.py",
		Content:  `bizumproto.ConfirmCreditRequest{Payment: confirmReq}.Send(ctx)`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow (non-.go), got %+v", out)
	}
}

func TestBackend101MapLitSingleLine(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/handler.go",
		Content:  `errParams := map[string]string{"decision_code": decisionCode, "reason_code": reasonCode}`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out == nil || out.Decision != "block" {
		t.Errorf("want block, got %+v", out)
	}
	if out != nil && !strings.Contains(out.Reason, "multi-line") {
		t.Errorf("reason = %q, want mention of multi-line", out.Reason)
	}
}

func TestBackend101MapLitSingleLineSingleEntry(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/handler.go",
		Content:  `m := map[string]int{"a": 1}`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out == nil || out.Decision != "block" {
		t.Errorf("want block, got %+v", out)
	}
}

func TestBackend101MapLitSingleLineSliceValue(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/handler.go",
		Content:  `m := map[string][]string{"a": {"x"}}`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out == nil || out.Decision != "block" {
		t.Errorf("want block, got %+v", out)
	}
}

func TestBackend101MapLitMultiLine(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/handler.go",
		Content: `errParams := map[string]string{
	"decision_code": decisionCode,
	"reason_code":   reasonCode,
}`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow, got %+v", out)
	}
}

func TestBackend101MapLitEmpty(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/handler.go",
		Content:  `m := map[string]int{}`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow (empty braces), got %+v", out)
	}
}

func TestBackend101MapTypeNotLiteral(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/handler.go",
		Content:  `func f(m map[string]int) error { return nil }`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow (map type, no literal), got %+v", out)
	}
}

func TestBackend101MapLitNonGoFile(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/service/notes.py",
		Content:  `m := map[string]string{"a": "b"}`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow (non-.go), got %+v", out)
	}
}

func TestBackend101MapLitEditNewStringOnly(t *testing.T) {
	input := makeToolInput("PreToolUse", "Edit", EditInput{
		FilePath:  "/src/service/handler.go",
		OldString: `m := map[string]string{"a": "b"}`,
		NewString: `// removed map`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow (bad pattern only in OldString), got %+v", out)
	}
}

func TestBackend101TimeParseRFC3339Nano(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/svc/timeline.go",
		Content: `func parseTimestamp(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out == nil || out.Decision != "block" {
		t.Fatalf("want block, got %+v", out)
	}
	if !strings.Contains(out.Reason, "util.ProtoToTime") {
		t.Errorf("reason = %q, want mention of util.ProtoToTime", out.Reason)
	}
}

func TestBackend101TimeParseRFC3339(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/svc/x.go",
		Content:  `t, err := time.Parse(time.RFC3339, s)`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out == nil || out.Decision != "block" {
		t.Errorf("want block, got %+v", out)
	}
}

func TestBackend101TimeParseOtherLayoutAllowed(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/svc/x.go",
		Content:  `t, err := time.Parse("2006-01-02", s)`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow (non-RFC3339 layout), got %+v", out)
	}
}

func TestBackend101TimeParseRFC3339NonGoFile(t *testing.T) {
	input := makeToolInput("PreToolUse", "Write", WriteInput{
		FilePath: "/src/svc/notes.py",
		Content:  `time.Parse(time.RFC3339Nano, s)`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow (non-.go), got %+v", out)
	}
}

func TestBackend101TimeParseRFC3339EditNewStringOnly(t *testing.T) {
	input := makeToolInput("PreToolUse", "Edit", EditInput{
		FilePath:  "/src/svc/x.go",
		OldString: `t, err := time.Parse(time.RFC3339Nano, s)`,
		NewString: `t, err := util.ProtoToTime(s)`,
	})
	out := runHandlerOutput(t, "backend101", input)
	if out != nil {
		t.Errorf("want allow (pattern only in OldString), got %+v", out)
	}
}

func TestCamelToSnake(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"MandateUpdated", "mandate_updated"},
		{"PaymentCreated", "payment_created"},
		{"HTTPHandler", "http_handler"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := camelToSnake(tt.in)
			if got != tt.want {
				t.Errorf("camelToSnake(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
