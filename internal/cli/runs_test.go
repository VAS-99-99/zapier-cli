package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunsListHelpUsesStableZapMetavariable(t *testing.T) {
	cmd := newRunsListCmd(&rootFlags{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, "--zap string") || strings.Contains(got, "--zap zaps list") {
		t.Fatalf("unexpected --zap help rendering:\n%s", got)
	}
}

func TestCurrentAccountID_AcceptsNumberAndString(t *testing.T) {
	for _, value := range []string{`77`, `"77"`} {
		t.Run(value, func(t *testing.T) {
			var gqlAccount any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v4/accounts" {
					t.Error("must not infer account context from accounts ordering")
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if r.URL.Path == "/api/v4/session" {
					_, _ = io.WriteString(w, `{"current_account_id":`+value+`}`)
					return
				}
				var req graphqlRequest
				_ = json.NewDecoder(r.Body).Decode(&req)
				gqlAccount = req.Variables["accountId"]
				_, _ = io.WriteString(w, `{"data":{"zapRuns":{"edges":[],"totalCount":0}}}`)
			}))
			t.Cleanup(srv.Close)
			if out, err := runCLI(t, srv.URL, "runs", "list", "--limit", "1"); err != nil {
				t.Fatalf("runs list: %v\n%s", err, out)
			}
			if gqlAccount != "77" {
				t.Fatalf("accountId = %#v", gqlAccount)
			}
		})
	}
}

func TestCurrentAccountID_RejectsMissingEmptyAndZero(t *testing.T) {
	for _, body := range []string{`{}`, `{"current_account_id":""}`, `{"current_account_id":0}`, `{"current_account_id":"0"}`} {
		t.Run(body, func(t *testing.T) {
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				_, _ = io.WriteString(w, body)
			}))
			t.Cleanup(srv.Close)
			if _, err := runCLI(t, srv.URL, "runs", "list", "--limit", "1"); err == nil {
				t.Fatal("invalid account id should fail")
			}
			if requests != 1 {
				t.Fatalf("invalid session should stop before GraphQL; requests=%d", requests)
			}
		})
	}
}

func TestReportingGraphQL_RejectsEveryNonExactPairBeforeDial(t *testing.T) {
	bad := []graphqlRequest{
		{OperationName: "Unknown", Query: zapRunsQuery},
		{OperationName: "ZapRuns", Query: runDetailQuery},
		{OperationName: "RunDetail", Query: runDetailQuery + " "},
		{OperationName: "ZapRuns", Query: zapRunsQuery + runDetailQuery},
		{OperationName: "ZapRuns", Query: "mutation ZapRuns { deleteZap }"},
	}
	for _, req := range bad {
		if _, err := callReportingGraphQL(&cobra.Command{}, nil, req); err == nil {
			t.Errorf("accepted operation=%q query=%q", req.OperationName, req.Query)
		}
	}
}

func TestReportingGraphQL_AllowedQueriesReachServerInVerifyMode(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	var seen []graphqlRequest
	var reportingRequests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `{"current_account_id":77}`)
			return
		}
		reportingRequests = append(reportingRequests, r.Method+" "+r.URL.Path)
		var req graphqlRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		seen = append(seen, req)
		if req.OperationName == "ZapRuns" {
			_, _ = io.WriteString(w, `{"data":{"zapRuns":{"edges":[],"totalCount":0}}}`)
		} else {
			_, _ = io.WriteString(w, `{"data":{"zapRun":{"id":"run-b","status":"success","zap":{"id":"51","title":"Fixture zap"},"steps":[]}}}`)
		}
	}))
	t.Cleanup(srv.Close)
	if _, err := runCLI(t, srv.URL, "runs", "list", "--limit", "1"); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, srv.URL, "runs", "get", "run-b"); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[0].Query != zapRunsQuery || seen[1].Query != runDetailQuery {
		t.Fatalf("allowed operation text did not reach server: %+v", seen)
	}
	if strings.Join(reportingRequests, "|") != "POST /api/reporting/graphql|POST /api/reporting/graphql" {
		t.Fatalf("allowed operations used wrong request method/path: %v", reportingRequests)
	}
}

func TestRuns_ValidateBeforeNetworkAndAddLiveMeta(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	t.Cleanup(srv.Close)
	for _, args := range [][]string{{"runs", "list", "--limit", "0"}, {"--data-source", "local", "runs", "list"}, {"--data-source", "local", "runs", "get", "run-c"}} {
		if _, err := runCLI(t, srv.URL, args...); err == nil {
			t.Fatalf("args %v should fail", args)
		}
	}
	if requests != 0 {
		t.Fatalf("validation dialed server %d times", requests)
	}
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `{"current_account_id":77}`)
		} else {
			var req graphqlRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.OperationName == "RunDetail" {
				_, _ = io.WriteString(w, `{"data":{"zapRun":{"id":"run-d","status":"success","zap":{"id":"61","title":"Fixture zap"},"steps":[]}}}`)
			} else {
				_, _ = io.WriteString(w, `{"data":{"zapRuns":{"edges":[],"totalCount":0}}}`)
			}
		}
	}))
	t.Cleanup(srv2.Close)
	out, err := runCLI(t, srv2.URL, "--agent", "runs", "list", "--limit", "1")
	if err != nil || !strings.Contains(squashJSON(out), `"source":"live"`) {
		t.Fatalf("agent output lacks live metadata: err=%v out=%s", err, out)
	}
	out, err = runCLI(t, srv2.URL, "--agent", "runs", "get", "run-d")
	if err != nil || !strings.Contains(squashJSON(out), `"source":"live"`) {
		t.Fatalf("runs get agent output lacks live metadata: err=%v out=%s", err, out)
	}
}

func TestRunsList_RejectsNullOrMissingReportingData(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"data":null}`,
		`{"data":{}}`,
		`{"data":{"zapRuns":null}}`,
		`{"data":{"zapRuns":{"edges":null,"totalCount":0}}}`,
		`{"data":{"zapRuns":{"edges":[]}}}`,
		`{"data":{"zapRuns":{"edges":[{"id":"run-a"}],"totalCount":1}}}`,
		`{"data":{"zapRuns":{"edges":[{"id":"run-a","status":"error"},{"id":"run-b","status":"error"}],"totalCount":1}}}`,
	} {
		t.Run(body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v4/session" {
					_, _ = io.WriteString(w, `{"current_account_id":77}`)
					return
				}
				_, _ = io.WriteString(w, body)
			}))
			t.Cleanup(srv.Close)
			if _, err := runCLI(t, srv.URL, "runs", "list"); err == nil {
				t.Fatal("malformed reporting response must fail closed")
			}
		})
	}
}

func TestRunsGet_RejectsMalformedRunDetail(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"data":null}`,
		`{"data":{}}`,
		`{"data":{"zapRun":{"id":"run-a","status":"success"}}}`,
		`{"data":{"zapRun":{"id":"run-a","steps":[]}}}`,
		`{"data":{"zapRun":{"id":"run-a","status":"success","steps":null}}}`,
		`{"data":{"zapRun":{"id":"run-a","status":"success","steps":[{"title":"Step"}]}}}`,
	} {
		t.Run(body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, body)
			}))
			t.Cleanup(srv.Close)
			if _, err := runCLI(t, srv.URL, "runs", "get", "run-a"); err == nil {
				t.Fatal("malformed run detail must fail closed")
			}
		})
	}
}

func TestRunsGet_NullRunDetailIsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data":{"zapRun":null}}`)
	}))
	t.Cleanup(srv.Close)
	_, err := runCLI(t, srv.URL, "runs", "get", "run-a")
	if err == nil || ExitCode(err) != 3 {
		t.Fatalf("null run detail must be not found, got %v", err)
	}
}

func TestRunsList_OffsetAllAndAgentPagination(t *testing.T) {
	var offsets []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/session" {
			_, _ = io.WriteString(w, `{"current_account_id":77}`)
			return
		}
		var req graphqlRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		offset := int(req.Variables["offset"].(float64))
		offsets = append(offsets, offset)
		if offset == 2 {
			_, _ = io.WriteString(w, `{"data":{"zapRuns":{"edges":[{"id":"run-c","status":"success"},{"id":"run-d","status":"error"}],"totalCount":5}}}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":{"zapRuns":{"edges":[{"id":"run-e","status":"success"}],"totalCount":5}}}`)
	}))
	t.Cleanup(srv.Close)

	out, err := runCLI(t, srv.URL, "--agent", "runs", "list", "--offset", "2", "--all", "--limit", "2")
	if err != nil {
		t.Fatalf("runs list: %v\n%s", err, out)
	}
	if got := strings.Join([]string{strconv.Itoa(offsets[0]), strconv.Itoa(offsets[1])}, ","); got != "2,4" {
		t.Fatalf("offsets = %s, want 2,4", got)
	}
	for _, want := range []string{`"results":[{`, `"offset":2`, `"returned":3`, `"total_count":5`, `"has_more":false`, `"next_offset":null`} {
		if !strings.Contains(squashJSON(out), want) {
			t.Errorf("output missing %s: %s", want, out)
		}
	}
}

func TestRunsList_AllFailsClosedOnRepeatedPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/session" {
			_, _ = io.WriteString(w, `{"current_account_id":77}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":{"zapRuns":{"edges":[{"id":"run-a","status":"error"}],"totalCount":2}}}`)
	}))
	t.Cleanup(srv.Close)
	if _, err := runCLI(t, srv.URL, "runs", "list", "--all", "--limit", "1"); err == nil || !strings.Contains(err.Error(), "repeated") {
		t.Fatalf("repeated page must fail closed, got %v", err)
	}
}

func TestRunsList_AllFailsClosedOnOverlappingShiftedPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/session" {
			_, _ = io.WriteString(w, `{"current_account_id":77}`)
			return
		}
		var req graphqlRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Variables["offset"].(float64) == 0 {
			_, _ = io.WriteString(w, `{"data":{"zapRuns":{"edges":[{"id":"run-a","status":"error"},{"id":"run-b","status":"error"}],"totalCount":4}}}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":{"zapRuns":{"edges":[{"id":"run-b","status":"success"},{"id":"run-c","status":"error"}],"totalCount":4}}}`)
	}))
	t.Cleanup(srv.Close)
	if _, err := runCLI(t, srv.URL, "runs", "list", "--all", "--limit", "2"); err == nil || !strings.Contains(err.Error(), "history changed") {
		t.Fatalf("overlapping shifted page must fail closed, got %v", err)
	}
}

// --- Run-monitor dependency patch regressions -------------------------------
//
// These cases pin the raw-preservation contract required by the downstream
// zapier-run-monitor collector: nullable step status, unknown fields, empty
// objects, integers above 2^53, and precise decimals must survive the
// machine-readable output byte-for-byte in value.

// everlastStepsFixture returns the documented 26-step Everlast shape: 10
// success, 3 filtered, 13 null-status (unexecuted branch steps).
func everlastStepsFixture() []map[string]any {
	steps := make([]map[string]any, 0, 26)
	for i := 0; i < 26; i++ {
		step := map[string]any{
			"title": fmt.Sprintf("Step %d", i+1),
			"app":   "FixtureApp",
		}
		switch {
		case i < 10:
			step["status"] = "success"
			step["input"] = map[string]any{}
			step["output"] = map[string]any{"outcome": "success"}
		case i < 13:
			step["status"] = "filtered"
			step["input"] = map[string]any{"filter_criteria": []any{}}
			step["output"] = map[string]any{}
		default:
			step["status"] = nil
			step["input"] = nil
			step["output"] = nil
		}
		step["error"] = nil
		steps = append(steps, step)
	}
	return steps
}

func detailResponse(t *testing.T, steps string) string {
	t.Helper()
	return `{"data":{"zapRun":{"id":"run-fixture","status":"success","startTime":"2026-09-01T12:00:00Z","zap":{"id":"61","title":"Fixture zap"},"steps":[` + steps + `]}}}`
}

func TestRunsGet_NullStepStatusSurvives(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/session" {
			_, _ = io.WriteString(w, `{"current_account_id":77}`)
			return
		}
		_, _ = io.WriteString(w, detailResponse(t, `{"title":"Branch","app":"Paths","status":null,"input":null,"output":null,"error":null}`))
	}))
	t.Cleanup(srv.Close)
	out, err := runCLI(t, srv.URL, "--agent", "--json", "runs", "get", "run-fixture")
	if err != nil {
		t.Fatalf("null step status must be accepted: %v\n%s", err, out)
	}
	if !strings.Contains(squashJSON(out), `"status":null`) {
		t.Fatalf("null status must survive output verbatim: %s", out)
	}
}

func TestRunsGet_MalformedNonNullStatusStillFails(t *testing.T) {
	for _, status := range []string{`5`, `"  "`, `true`, `{"code":"x"}`} {
		body := detailResponse(t, `{"title":"S","app":"A","status":`+status+`,"input":null,"output":null,"error":null}`)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, body)
		}))
		t.Cleanup(srv.Close)
		if _, err := runCLI(t, srv.URL, "--agent", "--json", "runs", "get", "run-fixture"); err == nil {
			t.Fatalf("non-null malformed status %s must fail closed", status)
		}
	}
}

func TestRunsGet_AbsentStatusKeyStillFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, detailResponse(t, `{"title":"S","app":"A","input":null,"output":null,"error":null}`))
	}))
	t.Cleanup(srv.Close)
	if _, err := runCLI(t, srv.URL, "--agent", "--json", "runs", "get", "run-fixture"); err == nil {
		t.Fatal("absent status key is structural malformation and must fail closed")
	}
}

func TestRunsGet_PreservesUnknownFieldsEmptyObjectsBigNumbers(t *testing.T) {
	steps := `{"title":"Code","app":"Code","status":"success",` +
		`"input":{"empty_obj":{},"nested":{"k":null}},` +
		`"output":{"big":123456789012345678901234567890,"empty_arr":[],"decimal":"0.1000000000000000055511151231257827","future_field":{"a":[1,2,null,"x"]}},` +
		`"error":null,"unknown_top_level":{"kept":true}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/session" {
			_, _ = io.WriteString(w, `{"current_account_id":77}`)
			return
		}
		_, _ = io.WriteString(w, detailResponse(t, steps))
	}))
	t.Cleanup(srv.Close)
	out, err := runCLI(t, srv.URL, "--agent", "--json", "runs", "get", "run-fixture")
	if err != nil {
		t.Fatalf("rich step must be accepted: %v\n%s", err, out)
	}
	for _, want := range []string{
		`123456789012345678901234567890`,       // integer above 2^53 unrounded
		`0.1000000000000000055511151231257827`, // precise decimal string intact
		`"empty_obj":{}`,
		`"empty_arr":[]`,
		`"unknown_top_level":{"kept":true}`,
		`"future_field":{"a":[1,2,null,"x"]}`,
	} {
		if !strings.Contains(squashJSON(out), want) {
			t.Errorf("run-level output missing preserved value %s: %s", want, out)
		}
	}
}

func TestRunsGet_NormalizedFieldsWinAliasCollisions(t *testing.T) {
	// The API would never send these, but a future field rename or alias
	// must not let an unknown key clobber an authoritative normalized value.
	body := `{"data":{"zapRun":{"id":"run-fixture","status":"success",` +
		`"startTime":"2026-09-01T10:00:00Z",` +
		`"start_time":"wrong-alias",` +
		`"zap_id":"wrong-alias",` +
		`"zap":{"id":"real-zap","title":"Fixture zap"},` +
		`"steps":[{"title":"S","app":"A","status":"success","input":null,"output":null,"error":null}]}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/session" {
			_, _ = io.WriteString(w, `{"current_account_id":77}`)
			return
		}
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	out, err := runCLI(t, srv.URL, "--agent", "--json", "runs", "get", "run-fixture")
	if err != nil {
		t.Fatalf("run with alias keys must be accepted: %v\n%s", err, out)
	}
	var envelope struct {
		Results struct {
			StartTime string `json:"start_time"`
			ZapID     string `json:"zap_id"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(squashJSON(out)), &envelope); err != nil {
		t.Fatalf("agent output is not the documented envelope: %v", err)
	}
	if envelope.Results.StartTime != "2026-09-01T10:00:00Z" {
		t.Errorf("normalized start_time must win alias collision, got %q", envelope.Results.StartTime)
	}
	if envelope.Results.ZapID != "real-zap" {
		t.Errorf("normalized zap_id must win alias collision, got %q", envelope.Results.ZapID)
	}
}

func TestRunsGet_PreservesUnknownFieldsNestedInZap(t *testing.T) {
	body := `{"data":{"zapRun":{"id":"run-fixture","status":"success",` +
		`"startTime":"2026-09-01T10:00:00Z",` +
		`"zap":{"id":"real-zap","title":"Fixture zap","future_metadata":{"kept":true},"big":123456789012345678901234567890},` +
		`"steps":[{"title":"S","app":"A","status":"success","input":null,"output":null,"error":null}]}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/session" {
			_, _ = io.WriteString(w, `{"current_account_id":77}`)
			return
		}
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	out, err := runCLI(t, srv.URL, "--agent", "--json", "runs", "get", "run-fixture")
	if err != nil {
		t.Fatalf("run with nested zap fields must be accepted: %v\n%s", err, out)
	}
	for _, want := range []string{
		// squashJSON removes spaces, so expectations avoid them.
		`"future_metadata":{"kept":true}`,
		`"big":123456789012345678901234567890`,
		`"zap_id":"real-zap"`,
		`"zap_title":"Fixturezap"`,
	} {
		if !strings.Contains(squashJSON(out), want) {
			t.Errorf("zap output missing preserved value %s: %s", want, out)
		}
	}
}

func TestRunsGet_PreservesUnknownRunLevelFieldsAndNumbers(t *testing.T) {
	// zapRun is emitted verbatim here so unknown run-level keys ride along
	// with the known ones, exactly as the API returns them.
	body := `{"data":{"zapRun":{"id":"run-fixture","status":"success",` +
		`"startTime":"2026-09-01T12:00:00Z",` +
		`"zap":{"id":"61","title":"Fixture zap"},` +
		`"steps":[{"title":"S","app":"A","status":"success","input":null,"output":null,"error":null}],` +
		`"big_id":123456789012345678901234567890,` +
		`"decimal":0.12345678901234567890123456789,` +
		`"unknown_run":{"keep":true},` +
		`"future_list":[1,null,"x"]}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/session" {
			_, _ = io.WriteString(w, `{"current_account_id":77}`)
			return
		}
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	out, err := runCLI(t, srv.URL, "--agent", "--json", "runs", "get", "run-fixture")
	if err != nil {
		t.Fatalf("run with unknown fields must be accepted: %v\n%s", err, out)
	}
	for _, want := range []string{
		`123456789012345678901234567890`,    // run-level integer above 2^53 unrounded
		`0.12345678901234567890123456789`,   // run-level precise decimal unrounded
		`"unknown_run":{"keep":true}`,       // unknown run-level object kept
		`"future_list":[1,null,"x"]`,        // unknown run-level array kept
		`"id":"run-fixture"`,                // known normalized fields still emitted
		`"zap_id":"61"`,                     // normalized snake_case naming intact
		`"start_time":"2026-09-01T12:00:00Z"`,
		`"steps":[{"title":"S","app":"A","status":"success","input":null,"output":null,"error":null}]`,
	} {
		if !strings.Contains(squashJSON(out), want) {
			t.Errorf("run-level output missing preserved value %s: %s", want, out)
		}
	}
}

func TestRunsGet_Everlast26StepsRetained(t *testing.T) {
	steps := everlastStepsFixture()
	raw, err := json.Marshal(steps)
	if err != nil {
		t.Fatal(err)
	}
	// detailResponse expects the steps content without the enclosing array.
	stepsContent := strings.TrimSuffix(strings.TrimPrefix(string(raw), "["), "]")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/session" {
			_, _ = io.WriteString(w, `{"current_account_id":77}`)
			return
		}
		_, _ = io.WriteString(w, detailResponse(t, stepsContent))
	}))
	t.Cleanup(srv.Close)
	out, err := runCLI(t, srv.URL, "--agent", "--json", "runs", "get", "run-fixture")
	if err != nil {
		t.Fatalf("26-step run must be accepted: %v\n%s", err, out)
	}
	var envelope struct {
		Results struct {
			Steps []struct {
				Title      string `json:"title"`
				Status     string `json:"status"`
				StatusNull bool   `json:"-"`
			} `json:"steps"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(squashJSON(out)), &envelope); err != nil {
		t.Fatalf("agent output is not the documented envelope: %v", err)
	}
	if len(envelope.Results.Steps) != 26 {
		t.Fatalf("want 26 steps, got %d", len(envelope.Results.Steps))
	}
	nullCount := strings.Count(squashJSON(out), `"status":null`)
	if nullCount < 13 {
		t.Fatalf("13 null statuses must survive, saw %d occurrences of \"status\":null", nullCount)
	}
	filtered := 0
	for _, s := range envelope.Results.Steps {
		if s.Status == "filtered" {
			filtered++
		}
	}
	if filtered != 3 {
		t.Fatalf("want 3 filtered steps, got %d", filtered)
	}
}

func TestRunsList_PreservesEdgeValues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/session" {
			_, _ = io.WriteString(w, `{"current_account_id":77}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":{"zapRuns":{"edges":[{"id":"run-é","status":"held","startTime":"2026-09-01T12:00:00Z","zap":{"id":"61","title":"Zap ünïcode"}}],"totalCount":1}}}`)
	}))
	t.Cleanup(srv.Close)
	out, err := runCLI(t, srv.URL, "--agent", "--json", "runs", "list", "--limit", "1")
	if err != nil {
		t.Fatalf("list: %v\n%s", err, out)
	}
	for _, want := range []string{`run-é`, `Zap ünïcode`, `held`} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}
