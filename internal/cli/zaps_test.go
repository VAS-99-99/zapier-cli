package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/zapier/internal/cliutil/testenv"
)

type requestLog struct {
	mu   sync.Mutex
	seen []string
}

func (l *requestLog) add(r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seen = append(l.seen, r.Method+" "+r.URL.RequestURI())
}

func (l *requestLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.seen...)
}

func runCLI(t *testing.T, baseURL string, args ...string) (string, error) {
	t.Helper()
	home := testenv.Isolate(t)
	t.Setenv("ZAPIER_BASE_URL", baseURL)
	t.Setenv("ZAPIER_SESSION_COOKIE", "fixture-token")
	t.Setenv("ZAPIER_CONFIG", filepath.Join(home, "config.json"))
	t.Setenv("ZAPIER_NO_LEARN", "1")
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"--no-learn", "--json"}, args...))
	err := cmd.Execute()
	return out.String(), err
}

func squashJSON(s string) string {
	return strings.NewReplacer(" ", "", "\n", "", "\t", "").Replace(s)
}

func writeZapPage(w http.ResponseWriter, count int, next string, zaps ...zapSummary) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(zapsPage{Count: count, Next: next, Results: zaps})
}

func TestZapsList_PaginatesUntilMatchLimit(t *testing.T) {
	log := &requestLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		if r.Method != http.MethodGet || r.URL.Path != zapsEndpoint {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		switch offset {
		case 0:
			writeZapPage(w, 4, "", zapSummary{ID: 11, Title: "Other one"}, zapSummary{ID: 12, Title: "Other two"})
		case 2:
			writeZapPage(w, 4, "", zapSummary{ID: 21, Title: "Target alpha"}, zapSummary{ID: 22, Title: "Other three"})
		default:
			writeZapPage(w, 4, "")
		}
	}))
	t.Cleanup(srv.Close)
	out, err := runCLI(t, srv.URL, "zaps", "list", "--name", "target", "--limit", "1")
	if err != nil {
		t.Fatalf("zaps list: %v\n%s", err, out)
	}
	flat := squashJSON(out)
	if !strings.Contains(flat, `"id":21`) || strings.Contains(flat, `"id":11`) {
		t.Fatalf("limit must count matches, got %s", out)
	}
	want := []string{"GET /api/v4/zaps?limit=100&offset=0", "GET /api/v4/zaps?limit=100&offset=2"}
	if got := log.all(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("pagination requests = %v, want %v", got, want)
	}
}

func TestZapsList_StopsOnEmptyPage(t *testing.T) {
	log := &requestLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		writeZapPage(w, 50, "still-more")
	}))
	t.Cleanup(srv.Close)
	out, err := runCLI(t, srv.URL, "zaps", "list", "--name", "absent", "--limit", "2")
	if err != nil || len(log.all()) != 1 || strings.TrimSpace(out) != "[]" {
		t.Fatalf("empty page did not stop cleanly: err=%v requests=%v out=%s", err, log.all(), out)
	}
}

func TestZapsList_StopsWhenCountIsSatisfied(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		writeZapPage(w, 1, "contradictory-next", zapSummary{ID: 23, Title: "Only zap"})
	}))
	t.Cleanup(srv.Close)
	out, err := runCLI(t, srv.URL, "zaps", "list", "--limit", "5")
	if err != nil || requests != 1 {
		t.Fatalf("satisfied count should stop after one request: err=%v requests=%d out=%s", err, requests, out)
	}
}

func TestZapsList_ValidatesBeforeNetworkAndAddsLiveMeta(t *testing.T) {
	log := &requestLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.add(r)
		writeZapPage(w, 1, "", zapSummary{ID: 31, Title: "Fixture zap", State: "on"})
	}))
	t.Cleanup(srv.Close)
	for _, args := range [][]string{{"--limit", "0"}, {"--data-source", "local"}} {
		if _, err := runCLI(t, srv.URL, append([]string{"zaps", "list"}, args...)...); err == nil {
			t.Fatalf("args %v should fail", args)
		}
	}
	if got := log.all(); len(got) != 0 {
		t.Fatalf("validation dialed server: %v", got)
	}
	out, err := runCLI(t, srv.URL, "--agent", "zaps", "list", "--limit", "1")
	if err != nil || !strings.Contains(squashJSON(out), `"source":"live"`) {
		t.Fatalf("agent output lacks live metadata: err=%v out=%s", err, out)
	}
}

func TestZapsList_SelectWinsOverAgentCompact(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeZapPage(w, 1, "", zapSummary{
			ID:    32,
			Title: "Fixture zap",
			Nodes: []zapNode{{ID: 7, Title: "Fixture step"}},
		})
	}))
	t.Cleanup(srv.Close)

	out, err := runCLI(t, srv.URL, "--agent", "--select", "nodes", "zaps", "list", "--limit", "1")
	if err != nil {
		t.Fatalf("zaps list --select nodes: %v\n%s", err, out)
	}
	flat := squashJSON(out)
	if !strings.Contains(flat, `"nodes":[{"id":7`) {
		t.Fatalf("--select must win over --agent compact projection, got %s", out)
	}
}
