package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
