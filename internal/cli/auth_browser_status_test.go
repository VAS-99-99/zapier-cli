package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBrowserStatusExplainsFailureWithoutLeakingHelperOutput(t *testing.T) {
	for _, tc := range []struct {
		name, payload, reason string
		failed                bool
	}{
		{"command", "secret-cookie-value", "status command failed", true},
		{"malformed", "secret-cookie-value", "invalid status response", false},
		{"inactive", `{"success":true,"data":{"active":false}}`, "session inactive", false},
		{"missing runtime", `{"success":true,"data":{"active":true,"runtime":null}}`, "browser runtime did not respond", false},
		{"not launched", `{"success":true,"data":{"active":true,"runtime":{"browserLaunched":false}}}`, "browser not launched", false},
		{"no page", `{"success":true,"data":{"active":true,"runtime":{"browserLaunched":true,"pageCount":0}}}`, "no open pages", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := runAgentBrowserCommand
			t.Cleanup(func() { runAgentBrowserCommand = old })
			runAgentBrowserCommand = func(context.Context, string, ...string) (agentBrowserCommandResult, error) {
				result := agentBrowserCommandResult{Stdout: []byte(tc.payload), Stderr: []byte("secret-cookie-value")}
				if tc.failed {
					return result, errors.New("secret-cookie-value")
				}
				return result, nil
			}
			err := requireAgentBrowserWindow(context.Background(), "unused", "unused", "unused")
			lost := tc.name == "inactive" || tc.name == "not launched" || tc.name == "no page"
			if err == nil || errors.Is(err, errAgentBrowserWindowLost) != lost || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("missing safe diagnosis %q: %v", tc.reason, err)
			}
			if strings.Contains(err.Error(), "secret-cookie-value") {
				t.Fatal("leaked helper output")
			}
		})
	}
}

func TestAuthBrowserStatusTimeoutRecoversWithoutReopening(t *testing.T) {
	fake := &fakeAgentBrowser{currentURL: "https://zapier.com/app/home", cookies: []agentBrowserCookie{{Name: "session", Value: "synthetic-only", Domain: ".zapier.com", Path: "/"}}}
	stubAgentBrowserGlobals(t, t.TempDir(), fake)
	probes := 0
	runAgentBrowserCommand = func(ctx context.Context, binary string, args ...string) (agentBrowserCommandResult, error) {
		if hasArgSequence(args, "session", "info") {
			probes++
			if probes < 3 {
				return agentBrowserCommandResult{}, context.DeadlineExceeded
			}
		}
		return fake.run(ctx, binary, args...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	header, _, err := waitForAgentBrowserLogin(ctx, "fake", "config", "session")
	if err != nil || header == "" {
		t.Fatalf("transient status timeout aborted login: %v", err)
	}
	for _, call := range fake.snapshotCalls() {
		if containsArg(call.args, "open") || containsArg(call.args, "close") {
			t.Fatal("status recovery reopened or closed browser")
		}
	}
}
