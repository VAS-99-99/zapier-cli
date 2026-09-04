package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRemoteWebhookDeliveryIsDisabled(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	t.Cleanup(srv.Close)

	if _, err := ParseDeliverSink("webhook:" + srv.URL); err == nil {
		t.Fatal("webhook delivery must be rejected during parsing")
	}
	if err := Deliver(DeliverSink{Scheme: "webhook", Target: srv.URL}, []byte("fixture"), false); err == nil {
		t.Fatal("direct webhook delivery must fail closed")
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("disabled webhook delivery made %d request(s)", got)
	}
}

func TestFeedbackNeverSendsRemotely(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ZAPIER_FEEDBACK_ENDPOINT", srv.URL)
	t.Setenv("ZAPIER_FEEDBACK_AUTO_SEND", "true")

	out, err := runCLI(t, srv.URL, "feedback", "fixture feedback")
	if err != nil {
		t.Fatalf("local feedback: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"remote": "disabled"`) {
		t.Fatalf("feedback output did not report remote-disabled state: %s", out)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("feedback environment triggered %d remote request(s)", got)
	}

	if _, err := runCLI(t, srv.URL, "feedback", "fixture feedback", "--send"); err == nil {
		t.Fatal("removed --send flag must be rejected")
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("rejected --send triggered %d remote request(s)", got)
	}
}

func TestRootHelpAdvertisesLocalDeliveryOnly(t *testing.T) {
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Contains(got, "webhook:<url>") || !strings.Contains(got, "Inspect Zapier resources through a remote read-only API surface") {
		t.Fatalf("root help violates remote read-only product language:\n%s", got)
	}
}
