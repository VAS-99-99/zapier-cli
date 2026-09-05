package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestAuthBrowserNavigationFailureDoesNotInstallOrReopen(t *testing.T) {
	for _, failure := range []error{context.DeadlineExceeded, context.Canceled, errors.New("navigation failed")} {
		t.Run(failure.Error(), func(t *testing.T) {
			fake := &fakeAgentBrowser{currentURL: "https://zapier.com/app/home", cookies: []agentBrowserCookie{{Name: "session", Value: "synthetic-only", Domain: ".zapier.com", Path: "/"}}}
			stubAgentBrowserGlobals(t, t.TempDir(), fake)
			opens := 0
			runAgentBrowserOpen = func(context.Context, string, ...string) error { opens++; return failure }
			cmd := newAuthBrowserCmd(&rootFlags{})
			cmd.SetArgs([]string{})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			_ = cmd.Execute()
			installs := 0
			for _, call := range fake.snapshotCalls() {
				if containsArg(call.args, "install") {
					installs++
				}
			}
			if installs != 0 || opens != 1 {
				t.Fatalf("installs=%d opens=%d; want 0 installs and exactly 1 open", installs, opens)
			}
		})
	}
}

func TestAuthBrowserSSORedirectDoesNotRestartNavigation(t *testing.T) {
	fake := &fakeAgentBrowser{currentURL: "https://accounts.google.com/v3/signin/identifier"}
	stubAgentBrowserGlobals(t, t.TempDir(), fake)
	opens := 0
	err := ensureAgentBrowserLoginPage(context.Background(), "fake", "config", "session", func(context.Context) error { opens++; return nil })
	if err != nil || opens != 0 {
		t.Fatalf("SSO interrupted: opens=%d err=%v", opens, err)
	}
}

func TestAuthBrowserSlowNavigationStillSavesValidatedSession(t *testing.T) {
	fake := &fakeAgentBrowser{currentURL: "https://zapier.com/app/home", cookies: []agentBrowserCookie{{Name: "session", Value: "synthetic-only", Domain: ".zapier.com", Path: "/"}}}
	stubAgentBrowserGlobals(t, t.TempDir(), fake)
	runAgentBrowserOpen = func(context.Context, string, ...string) error { return context.DeadlineExceeded }
	cmd := newAuthBrowserCmd(&rootFlags{asJSON: true})
	cmd.SetArgs([]string{})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"connected": true`, `"session_validated": true`, `"account_confirmation_required": true`, `"verified": false`} {
		if !strings.Contains(output.String(), field) {
			t.Fatalf("missing %s in connection result", field)
		}
	}
}

func TestAuthBrowserLostWindowNeverPollsOrNavigates(t *testing.T) {
	for _, payload := range []string{
		`{"active":false,"runtime":null}`,
		`{"active":true,"runtime":{"browserLaunched":false,"pageCount":0}}`,
		`{"active":true,"runtime":{"browserLaunched":true,"pageCount":0}}`,
		`{}`,
	} {
		t.Run(payload, func(t *testing.T) {
			fake := &fakeAgentBrowser{currentURL: "https://zapier.com/app/home"}
			stubAgentBrowserGlobals(t, t.TempDir(), fake)
			runAgentBrowserCommand = func(ctx context.Context, binary string, args ...string) (agentBrowserCommandResult, error) {
				if !hasArgSequence(args, "session", "info") {
					t.Fatalf("lost window triggered browser command: %v", args)
				}
				return jsonAgentBrowserResult(payload), nil
			}
			_, _, err := waitForAgentBrowserLogin(context.Background(), "fake", "config", "session")
			if err == nil || !strings.Contains(err.Error(), "closed or became unavailable") {
				t.Fatalf("expected closed window error, got %v", err)
			}
			opens := 0
			if err := ensureAgentBrowserLoginPage(context.Background(), "fake", "config", "session", func(context.Context) error { opens++; return nil }); err == nil || opens != 0 {
				t.Fatalf("lost window navigation: opens=%d err=%v", opens, err)
			}
			if _, err := agentBrowserCookies(context.Background(), "fake", "config", "session"); err == nil {
				t.Fatal("read cookies without a browser")
			}
		})
	}
}

func TestAuthBrowserClassifiesOnlyMissingChrome(t *testing.T) {
	for _, tc := range []struct {
		output  string
		missing bool
	}{
		{`{"success":false,"error":"Chrome not found. Checked:\n  - agent-browser cache: private-path\nRun ` + "`agent-browser install`" + ` to download Chrome, or use --executable-path."}`, true},
		{`{"success":false,"error":"navigation timed out"}`, false},
		{`{"success":false,"error":"net::ERR_CONNECTION_RESET"}`, false},
		{`{"success":false,"error":"Chrome not found elsewhere in page text"}`, false},
		{`not-json`, false},
	} {
		err := agentBrowserOpenResult(context.Background(), agentBrowserCommandResult{Stdout: []byte(tc.output)}, errors.New("exit status 1"))
		if errors.Is(err, errAgentBrowserMissing) != tc.missing {
			t.Fatalf("wrong missing-browser classification for %s", tc.output)
		}
		if strings.Contains(err.Error(), "private-path") {
			t.Fatal("helper response leaked")
		}
	}
}

func TestAuthBrowserWindowClosedBeforeCookieReadStops(t *testing.T) {
	fake := &fakeAgentBrowser{currentURL: "https://zapier.com/app/home"}
	stubAgentBrowserGlobals(t, t.TempDir(), fake)
	probes := 0
	runAgentBrowserCommand = func(ctx context.Context, binary string, args ...string) (agentBrowserCommandResult, error) {
		if hasArgSequence(args, "session", "info") {
			probes++
			if probes > 1 {
				return jsonAgentBrowserResult(`{"active":false}`), nil
			}
		}
		if containsArg(args, "cookies") || containsArg(args, "open") {
			t.Fatalf("closed window triggered %v", args)
		}
		return fake.run(ctx, binary, args...)
	}
	_, _, err := waitForAgentBrowserLogin(context.Background(), "fake", "config", "session")
	if err == nil || probes != 2 {
		t.Fatalf("expected stop before cookie read; probes=%d err=%v", probes, err)
	}
}

func TestAuthBrowserTransientNavigationReadsDoNotInterruptLogin(t *testing.T) {
	for _, action := range []string{"url", "cookies"} {
		t.Run(action, func(t *testing.T) {
			fake := &fakeAgentBrowser{currentURL: "https://zapier.com/app/home", cookies: []agentBrowserCookie{{Name: "session", Value: "synthetic-only", Domain: ".zapier.com", Path: "/"}}}
			stubAgentBrowserGlobals(t, t.TempDir(), fake)
			failures := 1
			runAgentBrowserCommand = func(ctx context.Context, binary string, args ...string) (agentBrowserCommandResult, error) {
				if containsArg(args, "open") || containsArg(args, "install") {
					t.Fatalf("read error triggered %v", args)
				}
				if containsArg(args, action) && failures > 0 {
					failures--
					return agentBrowserCommandResult{}, errors.New("execution context destroyed")
				}
				return fake.run(ctx, binary, args...)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if _, count, err := waitForAgentBrowserLogin(ctx, "fake", "config", "session"); err != nil || count != 1 || failures != 0 {
				t.Fatalf("transient %s read interrupted login: count=%d err=%v", action, count, err)
			}
			if action == "url" {
				failures = 1
				if err := ensureAgentBrowserLoginPage(ctx, "fake", "config", "session", nil); err != nil {
					t.Fatalf("initial read interrupted login: %v", err)
				}
			}
		})
	}
}
