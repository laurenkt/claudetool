package ghutil

import "testing"

func TestParsePRViewNoPR(t *testing.T) {
	// gh prints nothing to stdout (and errors) when there is no PR; the empty
	// body must read as "no PR", not a zero-value PR we'd then watch.
	for _, out := range []string{"", "   ", "\n"} {
		if _, ok := parsePRView([]byte(out)); ok {
			t.Errorf("parsePRView(%q) ok = true, want false", out)
		}
	}
}

func TestParsePRViewPending(t *testing.T) {
	out := `{
		"number": 42,
		"state": "OPEN",
		"headRefOid": "abc1234deadbeef",
		"statusCheckRollup": [
			{"__typename": "CheckRun", "name": "build", "status": "IN_PROGRESS", "detailsUrl": "https://ci/build"},
			{"__typename": "CheckRun", "name": "lint", "status": "COMPLETED", "conclusion": "SUCCESS", "detailsUrl": "https://ci/lint"},
			{"__typename": "StatusContext", "context": "legacy/jenkins", "state": "PENDING", "targetUrl": "https://ci/jenkins"}
		]
	}`
	pr, ok := parsePRView([]byte(out))
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if pr.Number != 42 || pr.HeadSHA != "abc1234deadbeef" {
		t.Errorf("got number=%d sha=%q", pr.Number, pr.HeadSHA)
	}
	if pr.Rollup.Total != 3 || pr.Rollup.Pending != 2 || pr.Rollup.Failed != 0 {
		t.Errorf("rollup = %+v, want total=3 pending=2 failed=0", pr.Rollup)
	}
	if len(pr.Rollup.Failing) != 0 {
		t.Errorf("Failing = %+v, want empty", pr.Rollup.Failing)
	}
}

func TestParsePRViewFailure(t *testing.T) {
	out := `{
		"number": 7,
		"state": "OPEN",
		"headRefOid": "f00ba7",
		"statusCheckRollup": [
			{"__typename": "CheckRun", "name": "unit-tests", "status": "COMPLETED", "conclusion": "FAILURE", "detailsUrl": "https://ci/unit"},
			{"__typename": "CheckRun", "name": "build", "status": "COMPLETED", "conclusion": "SUCCESS", "detailsUrl": "https://ci/build"},
			{"__typename": "StatusContext", "context": "legacy/deploy", "state": "ERROR", "targetUrl": "https://ci/deploy"},
			{"__typename": "CheckRun", "name": "flaky", "status": "COMPLETED", "conclusion": "SKIPPED"}
		]
	}`
	pr, ok := parsePRView([]byte(out))
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if pr.Rollup.Total != 4 || pr.Rollup.Failed != 2 || pr.Rollup.Pending != 0 {
		t.Errorf("rollup = %+v, want total=4 failed=2 pending=0", pr.Rollup)
	}
	want := map[string]string{
		"unit-tests":    "https://ci/unit",
		"legacy/deploy": "https://ci/deploy",
	}
	if len(pr.Rollup.Failing) != len(want) {
		t.Fatalf("Failing = %+v, want %d entries", pr.Rollup.Failing, len(want))
	}
	for _, c := range pr.Rollup.Failing {
		url, ok := want[c.Name]
		if !ok {
			t.Errorf("unexpected failing check %q", c.Name)
			continue
		}
		if c.URL != url {
			t.Errorf("check %q URL = %q, want %q", c.Name, c.URL, url)
		}
	}
}

func TestParsePRViewNoChecks(t *testing.T) {
	out := `{"number": 1, "state": "OPEN", "headRefOid": "deadbee", "statusCheckRollup": []}`
	pr, ok := parsePRView([]byte(out))
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if pr.Rollup.Total != 0 {
		t.Errorf("Total = %d, want 0", pr.Rollup.Total)
	}
}

func TestParsePRViewMalformed(t *testing.T) {
	if _, ok := parsePRView([]byte("{not json")); ok {
		t.Error("ok = true on malformed JSON, want false")
	}
}
