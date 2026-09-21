package hook

import (
	"strings"
	"testing"
)

func newRPCWrapperReview(t *testing.T, verdict string) (asyncReview, *bool) {
	t.Helper()
	called := new(bool)
	a := asyncReview{
		name:         "rpc-wrapper",
		fileSuffixes: []string{".go"},
		skipSuffix:   "_test.go",
		tier:         MediumBalanced,
		precheck:     looksLikeRPCWrapper,
		rubric:       rpcWrapperRubric,
		summary:      "Async review flagged a possibly pointless RPC wrapper:",
		review: func(prompt, model string) (string, error) {
			*called = true
			return verdict, nil
		},
	}
	return a, called
}

const thinWrapperSrc = `func readBizumNotification(ctx context.Context, externalID string) (*bizumpaymentproto.BizumNotification, error) {
	rsp, err := bizumpaymentproto.ReadBizumNotificationRequest{
		Id: externalID,
	}.Send(ctx).DecodeResponse()
	if err != nil {
		return nil, terrors.Augment(err, "reading bizum notification", nil)
	}
	return rsp.Notification, nil
}`

func TestRPCWrapperRegistered(t *testing.T) {
	if _, ok := registry["rpc-wrapper"]; !ok {
		t.Fatal("rpc-wrapper not registered")
	}
}

func TestRPCWrapperSkipsTestFiles(t *testing.T) {
	a, called := newRPCWrapperReview(t, "REVISE\n- pointless wrapper")
	out, err := invoke(t, a, "Write", WriteInput{
		FilePath: "/src/x_test.go",
		Content:  thinWrapperSrc,
	})
	if out != nil || err != nil {
		t.Fatalf("want nil/nil for _test.go, got out=%+v err=%v", out, err)
	}
	if *called {
		t.Error("reviewer should not run on _test.go files")
	}
}

func TestRPCWrapperSkipsNonGo(t *testing.T) {
	a, called := newRPCWrapperReview(t, "REVISE\n- x")
	out, err := invoke(t, a, "Write", WriteInput{
		FilePath: "/src/x.py",
		Content:  thinWrapperSrc,
	})
	if out != nil || err != nil {
		t.Fatalf("want nil/nil for non-.go, got out=%+v err=%v", out, err)
	}
	if *called {
		t.Error("reviewer should not run on non-Go files")
	}
}

func TestRPCWrapperPrecheckSkipsNonRPC(t *testing.T) {
	a, called := newRPCWrapperReview(t, "REVISE\n- x")
	out, err := invoke(t, a, "Edit", EditInput{
		FilePath:  "/src/x.go",
		NewString: "func add(a, b int) int {\n\treturn a + b\n}",
	})
	if out != nil || err != nil {
		t.Fatalf("want nil/nil, got out=%+v err=%v", out, err)
	}
	if *called {
		t.Error("reviewer should not run when the change has no RPC call")
	}
}

func TestRPCWrapperPrecheckSkipsBareCallSite(t *testing.T) {
	// An inline RPC call with no surrounding func declaration is a call site,
	// not a wrapper.
	a, called := newRPCWrapperReview(t, "PASS")
	out, err := invoke(t, a, "Edit", EditInput{
		FilePath:  "/src/x.go",
		NewString: "\trsp, err := proto.FooRequest{}.Send(ctx).DecodeResponse()",
	})
	if out != nil || err != nil {
		t.Fatalf("want nil/nil, got out=%+v err=%v", out, err)
	}
	if *called {
		t.Error("reviewer should not run on a bare call site (no func)")
	}
}

func TestRPCWrapperPass(t *testing.T) {
	a, called := newRPCWrapperReview(t, "PASS")
	out, err := invoke(t, a, "Write", WriteInput{
		FilePath: "/src/x.go",
		Content:  thinWrapperSrc,
	})
	if !*called {
		t.Fatal("reviewer should run on a func with an RPC call")
	}
	if out != nil || err != nil {
		t.Fatalf("want nil/nil on PASS, got out=%+v err=%v", out, err)
	}
}

func TestRPCWrapperReviseReturnsError(t *testing.T) {
	a, _ := newRPCWrapperReview(t, "REVISE\n- pure pass-through; inline the RPC at the call site")
	out, err := invoke(t, a, "Write", WriteInput{
		FilePath: "/src/x.go",
		Content:  thinWrapperSrc,
	})
	if out != nil {
		t.Fatalf("want nil output on REVISE, got %+v", out)
	}
	if err == nil || !strings.Contains(err.Error(), "inline the RPC") {
		t.Fatalf("want error carrying the feedback, got %v", err)
	}
	if !strings.Contains(err.Error(), "RPC wrapper") {
		t.Errorf("error should carry the summary header, got %q", err.Error())
	}
}
