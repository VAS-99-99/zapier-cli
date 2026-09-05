package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/zapier/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/zapier/internal/config"
)

const validBrowserSessionFixture = `{"is_logged_in":true,"is_temporary":false,"is_masquerade":false,"current_account_id":77,"id":88,"user_id":88}`

func TestAuthBrowserDoesNotSaveCookiesWithoutAuthenticatedSession(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	fake := &fakeAgentBrowser{
		currentURL: "https://zapier.com/app/mfa",
		cookies:    []agentBrowserCookie{{Name: "analytics", Value: "candidate-secret", Domain: ".zapier.com", Path: "/"}},
	}
	stubAgentBrowserGlobals(t, root, fake)
	browserSessionHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"is_logged_in":false}`))}, nil
	})}
	cmd := newAuthBrowserCmd(&rootFlags{asJSON: true, timeoutExplicit: true, timeout: 20 * time.Millisecond})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("cookies on an MFA page were accepted and saved without an authenticated session")
	}
	if strings.Contains(out.String()+err.Error(), "candidate-secret") {
		t.Fatal("candidate credential leaked")
	}
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ZapierSessionCookie != "" {
		t.Fatal("failed login saved a credential")
	}
	if _, err := os.Stat(filepath.Join(root, "ZapierCLI", agentBrowserPersistentProfile)); !os.IsNotExist(err) {
		t.Fatalf("temporary browser profile remained: %v", err)
	}
}

func TestBrowserSessionValidationRejectsIncompleteIdentity(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
	}{
		{"null", `null`},
		{"empty", `{}`},
		{"array", `[]`},
		{"malformed", `{"cookie":"candidate-secret"`},
		{"trailing data", validBrowserSessionFixture + `{}`},
		{"logged out", strings.Replace(validBrowserSessionFixture, `"is_logged_in":true`, `"is_logged_in":false`, 1)},
		{"login null", strings.Replace(validBrowserSessionFixture, `"is_logged_in":true`, `"is_logged_in":null`, 1)},
		{"login string", strings.Replace(validBrowserSessionFixture, `"is_logged_in":true`, `"is_logged_in":"true"`, 1)},
		{"temporary", strings.Replace(validBrowserSessionFixture, `"is_temporary":false`, `"is_temporary":true`, 1)},
		{"temporary missing", strings.Replace(validBrowserSessionFixture, `"is_temporary":false,`, ``, 1)},
		{"masquerade", strings.Replace(validBrowserSessionFixture, `"is_masquerade":false`, `"is_masquerade":true`, 1)},
		{"masquerade null", strings.Replace(validBrowserSessionFixture, `"is_masquerade":false`, `"is_masquerade":null`, 1)},
		{"account missing", strings.Replace(validBrowserSessionFixture, `"current_account_id":77,`, ``, 1)},
		{"account null", strings.Replace(validBrowserSessionFixture, `"current_account_id":77`, `"current_account_id":null`, 1)},
		{"account zero", strings.Replace(validBrowserSessionFixture, `"current_account_id":77`, `"current_account_id":0`, 1)},
		{"account negative", strings.Replace(validBrowserSessionFixture, `"current_account_id":77`, `"current_account_id":-1`, 1)},
		{"account fractional", strings.Replace(validBrowserSessionFixture, `"current_account_id":77`, `"current_account_id":1.5`, 1)},
		{"account string zero", strings.Replace(validBrowserSessionFixture, `"current_account_id":77`, `"current_account_id":"0"`, 1)},
		{"identity missing", strings.Replace(validBrowserSessionFixture, `,"id":88,"user_id":88`, ``, 1)},
		{"identity zero", strings.ReplaceAll(validBrowserSessionFixture, `:88`, `:0`)},
		{"oversized", validBrowserSessionFixture + strings.Repeat(" ", browserSessionMaxResponseBytes)},
	} {
		t.Run(test.name, func(t *testing.T) {
			stubAgentBrowserGlobals(t, t.TempDir(), &fakeAgentBrowser{})
			browserSessionHTTPClient = browserFixtureClient(t, http.StatusOK, test.body)
			err := verifyAgentBrowserSession(context.Background(), "session=candidate-secret")
			if err == nil {
				t.Fatal("incomplete session was accepted")
			}
			if strings.Contains(err.Error(), "candidate-secret") {
				t.Fatal("invalid response exposed a credential")
			}
		})
	}
}

func browserFixtureClient(t *testing.T, status int, body string) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.String() != "https://zapier.com/api/v4/session" || request.Body != nil {
			t.Fatal("session check escaped the fixed GET endpoint")
		}
		deadline, ok := request.Context().Deadline()
		if !ok || time.Until(deadline) > 5*time.Second {
			t.Fatal("session check has no bounded request context")
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
}

func TestBrowserSessionValidationUsesFixedEndpointAndAcceptsCompleteIdentity(t *testing.T) {
	for _, body := range []string{
		validBrowserSessionFixture,
		strings.Replace(validBrowserSessionFixture, `"current_account_id":77`, `"current_account_id":"77"`, 1),
		strings.Replace(validBrowserSessionFixture, `,"id":88`, ``, 1),
		strings.Replace(validBrowserSessionFixture, `,"user_id":88`, ``, 1),
	} {
		t.Run(body, func(t *testing.T) {
			stubAgentBrowserGlobals(t, t.TempDir(), &fakeAgentBrowser{})
			t.Setenv("ZAPIER_BASE_URL", "https://foreign.example")
			browserSessionHTTPClient = browserFixtureClient(t, http.StatusOK, body)
			if err := verifyAgentBrowserSession(context.Background(), "session=candidate-secret"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBrowserSessionValidationRejectsRedirectsAndExpiredSessions(t *testing.T) {
	for _, status := range []int{http.StatusMovedPermanently, http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect, http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			stubAgentBrowserGlobals(t, t.TempDir(), &fakeAgentBrowser{})
			calls := 0
			browserSessionHTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if request.URL.String() != browserSessionVerifyURL {
					t.Fatal("followed a redirect with candidate credentials")
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Location": {"https://foreign.example/candidate-secret"}}, Body: io.NopCloser(strings.NewReader(validBrowserSessionFixture))}, nil
			})}
			err := verifyAgentBrowserSession(context.Background(), "session=candidate-secret")
			if err == nil || calls != 1 || strings.Contains(err.Error(), "candidate-secret") {
				t.Fatalf("session check did not safely reject HTTP %d, calls=%d", status, calls)
			}
		})
	}
}

func TestAuthBrowserFailedReconnectPreservesCredentialAndCleansUp(t *testing.T) {
	for _, failure := range []string{"expired", "malformed", "cancel", "timeout", "close"} {
		t.Run(failure, func(t *testing.T) {
			root := t.TempDir()
			fake := &fakeAgentBrowser{currentURL: "https://zapier.com/app/home", cookies: []agentBrowserCookie{{Name: "session", Value: "candidate-secret", Domain: ".zapier.com", Path: "/"}}}
			stubAgentBrowserGlobals(t, root, fake)
			cfg, err := config.Load("")
			if err != nil {
				t.Fatal(err)
			}
			if err := cfg.SaveCredential("session=existing-secret"); err != nil {
				t.Fatal(err)
			}
			credentialPath, err := cliutil.CredentialsFilePath()
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(credentialPath)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			browserSessionHTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if failure == "cancel" {
					cancel()
				}
				if failure == "cancel" || failure == "timeout" {
					<-request.Context().Done()
					return nil, errors.New("transport includes candidate-secret")
				}
				status, body := http.StatusOK, validBrowserSessionFixture
				if failure == "expired" {
					status = http.StatusUnauthorized
				}
				if failure == "malformed" {
					body = `{"secret":"candidate-secret"`
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			if failure == "close" {
				runAgentBrowserCommand = func(ctx context.Context, binary string, args ...string) (agentBrowserCommandResult, error) {
					result, err := fake.run(ctx, binary, args...)
					if containsArg(args, "close") {
						return agentBrowserCommandResult{Stdout: []byte(`{"success":false}`)}, nil
					}
					return result, err
				}
			}
			cmd := newAuthBrowserCmd(&rootFlags{asJSON: true, timeoutExplicit: true, timeout: 25 * time.Millisecond})
			cmd.SetContext(ctx)
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			if err := cmd.Execute(); err == nil {
				t.Fatal("failed reconnect reported success")
			} else if strings.Contains(output.String()+err.Error(), "secret") {
				t.Fatal("failed reconnect exposed credentials")
			}
			after, err := os.ReadFile(credentialPath)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("failed reconnect changed the existing credential")
			}
			if _, err := os.Stat(filepath.Join(root, "ZapierCLI", agentBrowserPersistentProfile)); !os.IsNotExist(err) {
				t.Fatal("failed reconnect left a browser profile")
			}
			closed := false
			for _, call := range fake.snapshotCalls() {
				closed = closed || containsArg(call.args, "close")
			}
			if !closed {
				t.Fatal("failed reconnect did not attempt browser cleanup")
			}
		})
	}
}

func TestAuthBrowserWaitsForMFAThenValidatesBeforeSaveAndClose(t *testing.T) {
	root := t.TempDir()
	fake := &fakeAgentBrowser{currentURL: "https://zapier.com/app/mfa", cookies: []agentBrowserCookie{{Name: "session", Value: "candidate-secret", Domain: ".zapier.com", Path: "/"}}}
	stubAgentBrowserGlobals(t, root, fake)
	calls := 0
	browserSessionHTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.Header.Get("Cookie") != "session=candidate-secret" {
			t.Fatal("candidate cookies not passed to session check")
		}
		cfg, err := config.Load("")
		if err != nil || cfg.ZapierSessionCookie != "" {
			t.Fatal("credential saved before validation")
		}
		for _, call := range fake.snapshotCalls() {
			if containsArg(call.args, "close") {
				t.Fatal("browser closed before validation")
			}
		}
		body := validBrowserSessionFixture
		if calls == 1 {
			body = `{"is_logged_in":false}`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	cmd := newAuthBrowserCmd(&rootFlags{asJSON: true, timeoutExplicit: true, timeout: time.Second})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || !strings.Contains(output.String(), `"session_validated": true`) || !strings.Contains(output.String(), "session --agent --no-learn") {
		t.Fatal("login did not wait for session validation and preserve the account checkpoint")
	}
}

func TestAuthBrowserDoesNotVerifyMissingOrExpiredCookies(t *testing.T) {
	for _, payload := range []string{`{"cookies":[]}`, `{"cookies":[{"name":"session","value":"candidate-secret","domain":".zapier.com","path":"/","expires":1}]}`} {
		t.Run(payload, func(t *testing.T) {
			fake := &fakeAgentBrowser{currentURL: "https://zapier.com/app/home", cookiePayload: jsonAgentBrowserResult(payload).Stdout}
			stubAgentBrowserGlobals(t, t.TempDir(), fake)
			browserSessionHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("tried to verify missing or expired cookies")
				return nil, errors.New("unexpected request")
			})}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			if header, count, err := waitForAgentBrowserLogin(ctx, "fake", "config", "session"); err == nil || header != "" || count != 0 {
				t.Fatal("missing or expired cookies were accepted")
			}
		})
	}
}

type failingBrowserSessionBody struct{}

func (failingBrowserSessionBody) Read([]byte) (int, error) { return 0, errors.New("candidate-secret") }
func (failingBrowserSessionBody) Close() error             { return nil }

func TestBrowserSessionValidationDoesNotExposeBodyReadErrors(t *testing.T) {
	stubAgentBrowserGlobals(t, t.TempDir(), &fakeAgentBrowser{})
	browserSessionHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: failingBrowserSessionBody{}}, nil
	})}
	err := verifyAgentBrowserSession(context.Background(), "session=candidate-secret")
	if err == nil || strings.Contains(err.Error(), "candidate-secret") {
		t.Fatal("body read failure was not safely rejected")
	}
}
