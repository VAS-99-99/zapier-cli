package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDiagnoseDogfoodFixtureAnnotation(t *testing.T) {
	t.Setenv("ZAPIER_DOGFOOD_ZAP_ID", "")
	if got := diagnoseAnnotations()["pp:happy-args"]; got != "" {
		t.Fatalf("unexpected default dogfood fixture %q", got)
	}

	t.Setenv("ZAPIER_DOGFOOD_ZAP_ID", "123")
	if got := diagnoseAnnotations()["pp:happy-args"]; got != "" {
		t.Fatalf("fixture must be ignored outside dogfood, got %q", got)
	}
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
	if got := diagnoseAnnotations()["pp:happy-args"]; got != "zap-id=123" {
		t.Fatalf("dogfood fixture = %q, want zap-id=123", got)
	}

	for _, invalid := range []string{"0", "-1", "123;--limit=0", "not-an-id"} {
		t.Setenv("ZAPIER_DOGFOOD_ZAP_ID", invalid)
		if got := diagnoseAnnotations()["pp:happy-args"]; got != "" {
			t.Fatalf("invalid fixture %q produced annotation %q", invalid, got)
		}
	}
}

func errorStep(title, app string) runStep {
	errValue := &struct {
		Title string `json:"title"`
	}{Title: "fixture failure"}
	return runStep{Title: title, App: app, Status: "error", Error: errValue}
}

func TestFailuresFromSteps_UsesUniqueNormalizedTitleOnly(t *testing.T) {
	nodes := []zapNode{{Title: "Branch", Action: "branch"}, {Title: "  SEND   RECORD ", Action: "send"}, {Title: "Duplicate", Action: "first"}, {Title: "duplicate", Action: "second"}}
	steps := []runStep{errorStep("Send Record", "Run app"), errorStep("Branch child", "Branch app"), errorStep(" DUPLICATE ", "Duplicate app")}
	got := failuresFromSteps("run-a", "2000-01-02T03:04:05Z", steps, nodes)
	if len(got) != 3 || got[0].NodeAction != "send" {
		t.Fatalf("unique reordered title was not enriched: %+v", got)
	}
	if got[1].NodeAction != "" || got[2].NodeAction != "" {
		t.Fatalf("branch/duplicate mapping must stay empty: %+v", got)
	}
	if got[0].StepTitle != "Send Record" || got[0].App != "Run app" {
		t.Fatalf("run title/app were overwritten: %+v", got[0])
	}
}

func diagnoseFixtureServer(t *testing.T, detailStatus int, detailBody string) (*httptest.Server, *requestLog) {
	log := &requestLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case zapsEndpoint:
			writeZapPage(w, 1, "", zapSummary{ID: 41, Title: "Fixture zap", Nodes: []zapNode{{Title: "Send record", Action: "send"}}})
		case "/api/v4/session":
			_, _ = io.WriteString(w, `{"current_account_id":"77"}`)
		case "/api/reporting/graphql":
			var req graphqlRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.OperationName == "ZapRuns" {
				_, _ = io.WriteString(w, `{"data":{"zapRuns":{"edges":[{"id":"run-a","status":"error"}],"totalCount":1}}}`)
				return
			}
			w.WriteHeader(detailStatus)
			_, _ = io.WriteString(w, detailBody)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, log
}

func TestDiagnose_ReportsFailureWithLiveMeta(t *testing.T) {
	detail := `{"data":{"zapRun":{"id":"run-a","status":"error","startTime":"2000-01-02T03:04:05Z","steps":[{"title":"Send record","app":"Fixture app","status":"error","error":{"title":"fixture failure"}}]}}}`
	srv, log := diagnoseFixtureServer(t, http.StatusOK, detail)
	out, err := runCLI(t, srv.URL, "--agent", "diagnose", "Fixture zap", "--limit", "1")
	if err != nil {
		t.Fatalf("diagnose: %v\n%s", err, out)
	}
	for _, want := range []string{`"node_action":"send"`, `"runs_checked":1`, `"source":"live"`} {
		if !strings.Contains(squashJSON(out), want) {
			t.Errorf("output missing %s: %s", want, out)
		}
	}
	for _, req := range log.all() {
		if !strings.HasPrefix(req, "GET ") && !strings.HasPrefix(req, "POST /api/reporting/graphql") {
			t.Errorf("mutating request: %s", req)
		}
	}
}

func TestDiagnose_AmbiguousOutputHasLiveMeta(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		writeZapPage(w, 2, "", zapSummary{ID: 41, Title: "Fixture alpha"}, zapSummary{ID: 42, Title: "Fixture beta"})
	}))
	t.Cleanup(srv.Close)
	out, err := runCLI(t, srv.URL, "--agent", "diagnose", "Fixture")
	if err != nil || requests != 1 || !strings.Contains(squashJSON(out), `"source":"live"`) || !strings.Contains(out, "matches 2 zaps") {
		t.Fatalf("ambiguous result: err=%v requests=%d out=%s", err, requests, out)
	}
}

func TestDiagnose_FindsNumericIDOnSecondPage(t *testing.T) {
	var offsets []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case zapsEndpoint:
			offset := r.URL.Query().Get("offset")
			offsets = append(offsets, offset)
			if offset == "0" {
				writeZapPage(w, 2, "", zapSummary{ID: 51, Title: "First page"})
			} else {
				writeZapPage(w, 2, "", zapSummary{ID: 52, Title: "Numeric target"})
			}
		case "/api/v4/session":
			_, _ = io.WriteString(w, `{"current_account_id":77}`)
		case "/api/reporting/graphql":
			_, _ = io.WriteString(w, `{"data":{"zapRuns":{"edges":[],"totalCount":0}}}`)
		}
	}))
	t.Cleanup(srv.Close)
	out, err := runCLI(t, srv.URL, "diagnose", "52")
	if err != nil || strings.Join(offsets, ",") != "0,1" || !strings.Contains(squashJSON(out), `"zap_id":"52"`) {
		t.Fatalf("numeric page-2 lookup failed: err=%v offsets=%v out=%s", err, offsets, out)
	}
}

func TestDiagnose_NumericIDCanInspectDeletedZapHistory(t *testing.T) {
	log := &requestLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case zapsEndpoint:
			writeZapPage(w, 0, "")
		case "/api/v4/session":
			_, _ = io.WriteString(w, `{"current_account_id":"77"}`)
		case "/api/reporting/graphql":
			var req graphqlRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.OperationName == "ZapRuns" {
				_, _ = io.WriteString(w, `{"data":{"zapRuns":{"edges":[{"id":"historical-run","status":"error","zap":{"id":"99"}}],"totalCount":1}}}`)
				return
			}
			_, _ = io.WriteString(w, `{"data":{"zapRun":{"id":"historical-run","status":"error","startTime":"2000-01-02T03:04:05Z","steps":[{"title":"Historical step","app":"Fixture app","status":"error","error":{"title":"fixture failure"}}]}}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	out, err := runCLI(t, srv.URL, "diagnose", "99", "--limit", "1")
	if err != nil {
		t.Fatalf("diagnose deleted Zap history: %v\n%s", err, out)
	}
	flat := squashJSON(out)
	for _, want := range []string{`"zap_id":"99"`, `"runs_checked":1`, `"step_title":"Historicalstep"`, `"error":"fixturefailure"`} {
		if !strings.Contains(strings.ToLower(flat), strings.ToLower(want)) {
			t.Errorf("output missing %s: %s", want, out)
		}
	}
	for _, request := range log.all() {
		if !strings.HasPrefix(request, "GET ") && !strings.HasPrefix(request, "POST /api/reporting/graphql") {
			t.Errorf("mutating request: %s", request)
		}
	}
}

func TestDiagnose_UnknownHistoricalIDFailsClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case zapsEndpoint:
			writeZapPage(w, 0, "")
		case "/api/v4/session":
			_, _ = io.WriteString(w, `{"current_account_id":"77"}`)
		case "/api/reporting/graphql":
			_, _ = io.WriteString(w, `{"data":{"zapRuns":{"edges":[],"totalCount":0}}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	_, err := runCLI(t, srv.URL, "diagnose", "999", "--limit", "1")
	if err == nil || ExitCode(err) != 3 || !strings.Contains(err.Error(), "no failed run history") {
		t.Fatalf("unknown historical Zap ID must fail closed with not-found, got %v", err)
	}
}

func TestDiagnose_FailsClosedOnBadDetail(t *testing.T) {
	tests := []struct {
		name string
		code int
		body string
	}{
		{"http", http.StatusBadRequest, `{"error":"fixture"}`},
		{"graphql", http.StatusOK, `{"errors":[{"message":"fixture"}]}`},
		{"null", http.StatusOK, `{"data":{"zapRun":null}}`},
		{"no errored step", http.StatusOK, `{"data":{"zapRun":{"id":"run-a","status":"success","steps":[{"title":"Done","status":"success"}]}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := diagnoseFixtureServer(t, tt.code, tt.body)
			_, err := runCLI(t, srv.URL, "diagnose", "41", "--limit", "1")
			if err == nil || !strings.Contains(err.Error(), "run-a") {
				t.Fatalf("wanted typed fail-closed error containing run id, got %v", err)
			}
			if code := ExitCode(err); code != 3 && code != 5 {
				t.Fatalf("exit code = %d, want API/not-found", code)
			}
		})
	}
}

func TestDiagnose_FailsClosedOnMalformedReportingData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case zapsEndpoint:
			writeZapPage(w, 1, "", zapSummary{ID: 41, Title: "Fixture zap"})
		case "/api/v4/session":
			_, _ = io.WriteString(w, `{"current_account_id":77}`)
		case "/api/reporting/graphql":
			_, _ = io.WriteString(w, `{"data":{"zapRuns":null}}`)
		}
	}))
	t.Cleanup(srv.Close)
	if _, err := runCLI(t, srv.URL, "diagnose", "41", "--limit", "1"); err == nil || !strings.Contains(err.Error(), "zapRuns") {
		t.Fatalf("malformed reporting history must fail closed, got %v", err)
	}
}

func TestDiagnose_FailsClosedOnMalformedRunDetail(t *testing.T) {
	srv, _ := diagnoseFixtureServer(t, http.StatusOK, `{"data":{"zapRun":{"id":"run-a","status":"error","steps":[{"title":"Send record"}]}}}`)
	if _, err := runCLI(t, srv.URL, "diagnose", "41", "--limit", "1"); err == nil || !strings.Contains(err.Error(), "run-a") {
		t.Fatalf("malformed run detail must fail closed, got %v", err)
	}
}

func TestDiagnose_ValidatesLimitBeforeNetwork(t *testing.T) {
	srv, log := diagnoseFixtureServer(t, http.StatusOK, `{}`)
	for _, args := range [][]string{{"diagnose", "41", "--limit", "0"}, {"--data-source", "local", "diagnose", "41"}} {
		if _, err := runCLI(t, srv.URL, args...); err == nil {
			t.Fatalf("args %v should fail", args)
		}
	}
	if len(log.all()) != 0 {
		t.Fatalf("limit validation dialed server: %v", log.all())
	}
}
