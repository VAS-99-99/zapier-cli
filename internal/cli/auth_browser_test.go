package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/zapier/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/zapier/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/zapier/internal/config"
)

type recordedAgentBrowserCall struct {
	binary string
	args   []string
}

type fakeAgentBrowser struct {
	mu            sync.Mutex
	calls         []recordedAgentBrowserCall
	openFailures  int
	currentURL    string
	cookies       []agentBrowserCookie
	cookiePayload []byte
}

func (f *fakeAgentBrowser) run(_ context.Context, binary string, args ...string) (agentBrowserCommandResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, recordedAgentBrowserCall{binary: binary, args: append([]string(nil), args...)})
	if containsArg(args, "open") {
		if f.openFailures > 0 {
			f.openFailures--
			return agentBrowserCommandResult{Stdout: []byte(`{"success":false,"error":"Chrome not found. Run ` + "`agent-browser install`" + ` to download Chrome."}`)}, errors.New("exit status 1")
		}
		return jsonAgentBrowserResult(`{}`), nil
	}
	if containsArg(args, "install") {
		return agentBrowserCommandResult{}, nil
	}
	if hasArgSequence(args, "session", "info") {
		return jsonAgentBrowserResult(`{"active":true,"runtime":{"browserLaunched":true,"pageCount":1}}`), nil
	}
	if hasArgSequence(args, "get", "url") {
		return jsonAgentBrowserResult(`{"url":"` + f.currentURL + `"}`), nil
	}
	if containsArg(args, "cookies") {
		if f.cookiePayload != nil {
			return agentBrowserCommandResult{Stdout: append([]byte(nil), f.cookiePayload...)}, nil
		}
		var body bytes.Buffer
		body.WriteString(`{"cookies":[`)
		for index, cookie := range f.cookies {
			if index > 0 {
				body.WriteByte(',')
			}
			body.WriteString(`{"name":"` + cookie.Name + `","value":"` + cookie.Value + `","domain":"` + cookie.Domain + `","path":"` + cookie.Path + `"}`)
		}
		body.WriteString(`]}`)
		return jsonAgentBrowserResult(body.String()), nil
	}
	if containsArg(args, "close") {
		return jsonAgentBrowserResult(`{"closed":true}`), nil
	}
	return agentBrowserCommandResult{}, errors.New("unexpected fake command")
}

func (f *fakeAgentBrowser) snapshotCalls() []recordedAgentBrowserCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]recordedAgentBrowserCall, len(f.calls))
	copy(out, f.calls)
	return out
}

func jsonAgentBrowserResult(data string) agentBrowserCommandResult {
	return agentBrowserCommandResult{Stdout: []byte(`{"success":true,"data":` + data + `}`)}
}

func containsArg(args []string, target string) bool {
	for _, arg := range args {
		if arg == target {
			return true
		}
	}
	return false
}

func hasArgSequence(args []string, sequence ...string) bool {
	for index := 0; index+len(sequence) <= len(args); index++ {
		matched := true
		for offset := range sequence {
			if args[index+offset] != sequence[offset] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func stubAgentBrowserGlobals(t *testing.T, configRoot string, fake *fakeAgentBrowser) {
	t.Helper()
	testenv.Isolate(t, cliutil.ConfigDir, cliutil.DataDir, config.LegacyConfigPath)
	t.Setenv("ZAPIER_SESSION_COOKIE", "")
	previousGOOS := browserRuntimeGOOS
	previousGOARCH := browserRuntimeGOARCH
	previousConfigDir := browserUserConfigDir
	previousCacheDir := browserUserCacheDir
	previousEnsure := ensureAgentBrowserTool
	previousRunner := runAgentBrowserCommand
	previousOpenRunner := runAgentBrowserOpen
	previousSession := newAgentBrowserSession
	previousPoll := browserPollInterval
	previousSessionClient := browserSessionHTTPClient
	browserSessionHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(validBrowserSessionFixture))}, nil
	})}
	browserRuntimeGOOS = "windows"
	browserRuntimeGOARCH = "amd64"
	browserUserConfigDir = func() (string, error) { return configRoot, nil }
	browserUserCacheDir = func() (string, error) { return configRoot, nil }
	ensureAgentBrowserTool = func(_ context.Context, allowInstall bool) (string, bool, error) {
		if !allowInstall {
			return "", false, errors.New("installation disabled")
		}
		return filepath.Join(configRoot, "fake-agent-browser.exe"), true, nil
	}
	runAgentBrowserCommand = fake.run
	runAgentBrowserOpen = func(ctx context.Context, binary string, args ...string) error {
		result, err := fake.run(ctx, binary, args...)
		return agentBrowserOpenResult(ctx, result, err)
	}
	newAgentBrowserSession = func() string { return "zapier-pp-auth-test" }
	browserPollInterval = time.Millisecond
	t.Cleanup(func() {
		browserRuntimeGOOS = previousGOOS
		browserRuntimeGOARCH = previousGOARCH
		browserUserConfigDir = previousConfigDir
		browserUserCacheDir = previousCacheDir
		ensureAgentBrowserTool = previousEnsure
		runAgentBrowserCommand = previousRunner
		runAgentBrowserOpen = previousOpenRunner
		newAgentBrowserSession = previousSession
		browserPollInterval = previousPoll
		browserSessionHTTPClient = previousSessionClient
	})
}

func TestAuthBrowserUsesPrivateAgentBrowserSessionAndNeverPrintsCookies(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	fake := &fakeAgentBrowser{
		currentURL: "https://zapier.com/app/home",
		cookies: []agentBrowserCookie{
			{Name: "session", Value: "top-secret-session", Domain: ".zapier.com", Path: "/"},
			{Name: "csrf", Value: "top-secret-csrf", Domain: "zapier.com", Path: "/api"},
			{Name: "lookalike", Value: "must-not-save", Domain: "evilzapier.com", Path: "/"},
			{Name: "other", Value: "must-not-save-either", Domain: ".example.com", Path: "/"},
		},
	}
	stubAgentBrowserGlobals(t, configRoot, fake)

	flags := &rootFlags{asJSON: true, timeout: time.Second}
	cmd := newAuthBrowserCmd(flags)
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	for _, secret := range []string{"top-secret-session", "top-secret-csrf", "must-not-save"} {
		if strings.Contains(out.String(), secret) || strings.Contains(stderr.String(), secret) {
			t.Fatalf("command output exposed cookie value %q", secret)
		}
	}
	if !strings.Contains(out.String(), `"credential_saved": true`) || !strings.Contains(out.String(), `"verified": false`) || !strings.Contains(out.String(), `"cookies_imported": 2`) {
		t.Fatalf("unexpected command output: %s", out.String())
	}
	if !strings.Contains(out.String(), `"browser_tool": "agent-browser"`) || !strings.Contains(out.String(), agentBrowserVersion) {
		t.Fatalf("browser tool metadata missing: %s", out.String())
	}

	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.ZapierSessionCookie; !strings.Contains(got, "session=top-secret-session") || !strings.Contains(got, "csrf=top-secret-csrf") {
		t.Fatal("saved credential did not include both Zapier cookies")
	}
	if strings.Contains(cfg.ZapierSessionCookie, "must-not-save") {
		t.Fatal("saved credential included a cookie outside the Zapier domain boundary")
	}

	calls := fake.snapshotCalls()
	var openCall, closeCall *recordedAgentBrowserCall
	for index := range calls {
		call := &calls[index]
		if containsArg(call.args, "open") {
			openCall = call
		}
		if containsArg(call.args, "close") {
			closeCall = call
		}
		if containsArg(call.args, "mcp") || containsArg(call.args, "eval") || containsArg(call.args, "click") {
			t.Fatalf("auth flow invoked a generic browser action: %v", call.args)
		}
		if (containsArg(call.args, "open") || containsArg(call.args, "get") || containsArg(call.args, "cookies") || containsArg(call.args, "close")) &&
			!hasArgSequence(call.args, "--namespace", "zapier-pp-auth-test") {
			t.Fatalf("browser command was not isolated in the dedicated namespace: %v", call.args)
		}
	}
	if openCall == nil {
		t.Fatal("auth flow did not open the private browser")
	}
	profilePath := filepath.Join(configRoot, "ZapierCLI", agentBrowserPersistentProfile)
	browserConfigPath := filepath.Join(configRoot, "zapier-pp-cli", agentBrowserManagedDirectory, "zapier-auth-agent-browser.json")
	for _, required := range [][]string{
		{"--config", browserConfigPath},
		{"--namespace", "zapier-pp-auth-test"},
		{"--session", "zapier-pp-auth-test"},
		{"--profile", profilePath},
		{"--headed"},
		{"--no-webmcp"},
		{"open", zapierLoginURL},
		{"--json"},
	} {
		if !hasArgSequence(openCall.args, required...) {
			t.Errorf("open args %v missing %v", openCall.args, required)
		}
	}
	if closeCall == nil || !hasArgSequence(closeCall.args, "--session", "zapier-pp-auth-test", "close") {
		t.Fatalf("auth flow did not close only its named session: %#v", closeCall)
	}
	if containsArg(closeCall.args, "--all") {
		t.Fatalf("auth flow attempted to close all browser sessions: %v", closeCall.args)
	}
	if !hasArgSequence(closeCall.args, "--config", browserConfigPath) {
		t.Fatalf("close command did not use the locked auth config: %v", closeCall.args)
	}
	if !hasArgSequence(closeCall.args, "--namespace", "zapier-pp-auth-test") {
		t.Fatalf("close command did not use the dedicated namespace: %v", closeCall.args)
	}
	configBytes, err := os.ReadFile(browserConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	var browserConfig map[string]any
	if err := json.Unmarshal(configBytes, &browserConfig); err != nil {
		t.Fatalf("auth browser config is invalid JSON: %v", err)
	}
	if browserConfig["headed"] != true || browserConfig["noWebmcp"] != true || browserConfig["profile"] != profilePath {
		t.Fatalf("auth browser config = %#v, want controlled headed/private-profile/no-WebMCP settings", browserConfig)
	}
	if info, err := os.Stat(browserConfigPath); err != nil {
		t.Fatal(err)
	} else if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("auth browser config permissions = %o, want no group/other access", info.Mode().Perm())
	}
	if _, err := os.Stat(profilePath); !os.IsNotExist(err) {
		t.Fatalf("temporary browser profile remained after credential capture: %v", err)
	}
}

func TestAuthBrowserInstallsChromeForTestingOnlyAfterMissingBrowserError(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	fake := &fakeAgentBrowser{
		openFailures: 1,
		currentURL:   "https://zapier.com/app/home",
		cookies:      []agentBrowserCookie{{Name: "session", Value: "secret", Domain: ".zapier.com", Path: "/"}},
	}
	stubAgentBrowserGlobals(t, configRoot, fake)

	cmd := newAuthBrowserCmd(&rootFlags{timeout: time.Second})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var opens, installs int
	for _, call := range fake.snapshotCalls() {
		if containsArg(call.args, "open") {
			opens++
		}
		if containsArg(call.args, "install") {
			installs++
		}
	}
	if opens != 2 || installs != 1 {
		t.Fatalf("open calls = %d, install calls = %d; want 2 and 1", opens, installs)
	}
}

func TestAuthBrowserNoInstallDoesNotDownloadTool(t *testing.T) {
	configRoot := t.TempDir()
	fake := &fakeAgentBrowser{}
	stubAgentBrowserGlobals(t, configRoot, fake)
	allowInstallSeen := true
	ensureAgentBrowserTool = func(_ context.Context, allowInstall bool) (string, bool, error) {
		allowInstallSeen = allowInstall
		return "", false, errors.New("tool missing")
	}

	cmd := newAuthBrowserCmd(&rootFlags{})
	cmd.SetArgs([]string{"--no-install"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "tool missing") {
		t.Fatalf("error = %v, want missing-tool error", err)
	}
	if allowInstallSeen {
		t.Fatal("--no-install allowed the browser tool download")
	}
	if len(fake.snapshotCalls()) != 0 {
		t.Fatal("browser runner was called when the pinned tool was missing")
	}
}

func TestAuthBrowserWaitsForPostLoginZapierURL(t *testing.T) {
	configRoot := t.TempDir()
	fake := &fakeAgentBrowser{currentURL: zapierLoginURL}
	stubAgentBrowserGlobals(t, configRoot, fake)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Millisecond)
	defer cancel()
	_, _, err := waitForAgentBrowserLogin(ctx, "fake", "config", "test")
	if err == nil || !strings.Contains(err.Error(), "timed out waiting for Zapier sign-in") {
		t.Fatalf("error = %v, want sign-in timeout", err)
	}
	for _, call := range fake.snapshotCalls() {
		if containsArg(call.args, "cookies") {
			t.Fatal("read cookies before the browser left the Zapier login page")
		}
	}
}

func TestAuthBrowserRetriesBlankPageBeforeAskingForLogin(t *testing.T) {
	fake := &fakeAgentBrowser{currentURL: "about:blank"}
	stubAgentBrowserGlobals(t, t.TempDir(), fake)
	opens := 0
	err := ensureAgentBrowserLoginPage(context.Background(), "fake", "config", "test", func(context.Context) error {
		opens++
		fake.currentURL = zapierLoginURL
		return nil
	})
	if err != nil || opens != 1 {
		t.Fatalf("retry: opens=%d, err=%v", opens, err)
	}
}

func TestAuthBrowserNeverAsksForLoginOnBlankPage(t *testing.T) {
	fake := &fakeAgentBrowser{currentURL: "about:blank"}
	stubAgentBrowserGlobals(t, t.TempDir(), fake)
	cmd := newAuthBrowserCmd(&rootFlags{})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "login page did not load") {
		t.Fatalf("expected navigation failure, got %v", err)
	}
	if strings.Contains(output.String(), "Waiting for sign-in") {
		t.Fatal("asked user to sign in on a blank page")
	}
	for _, call := range fake.snapshotCalls() {
		if containsArg(call.args, "cookies") {
			t.Fatal("read cookies before login page loaded")
		}
	}
}

func TestAuthBrowserRejectsUnsupportedPlatformBeforeInstall(t *testing.T) {
	previousGOOS := browserRuntimeGOOS
	previousGOARCH := browserRuntimeGOARCH
	previousEnsure := ensureAgentBrowserTool
	browserRuntimeGOOS = "freebsd"
	browserRuntimeGOARCH = "amd64"
	ensureCalled := false
	ensureAgentBrowserTool = func(context.Context, bool) (string, bool, error) {
		ensureCalled = true
		return "", false, nil
	}
	t.Cleanup(func() {
		browserRuntimeGOOS = previousGOOS
		browserRuntimeGOARCH = previousGOARCH
		ensureAgentBrowserTool = previousEnsure
	})

	cmd := newAuthBrowserCmd(&rootFlags{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "freebsd/amd64") {
		t.Fatalf("error = %v, want supported-platform guidance", err)
	}
	if ensureCalled {
		t.Fatal("unsupported platform attempted browser-tool installation")
	}
}

func TestAuthBrowserDeclaresLocalCredentialWrite(t *testing.T) {
	cmd := newAuthBrowserCmd(&rootFlags{})
	if got := cmd.Annotations["mcp:local-write"]; got != "true" {
		t.Fatalf("mcp:local-write annotation = %q, want true", got)
	}
}

func TestAuthBrowserVerifyModeDoesNotInstallOrLaunch(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	previousEnsure := ensureAgentBrowserTool
	previousRunner := runAgentBrowserCommand
	previousOpenRunner := runAgentBrowserOpen
	ensureCalled := false
	runCalled := false
	ensureAgentBrowserTool = func(context.Context, bool) (string, bool, error) {
		ensureCalled = true
		return "", false, nil
	}
	runAgentBrowserCommand = func(context.Context, string, ...string) (agentBrowserCommandResult, error) {
		runCalled = true
		return agentBrowserCommandResult{}, nil
	}
	runAgentBrowserOpen = func(context.Context, string, ...string) error {
		runCalled = true
		return nil
	}
	t.Cleanup(func() {
		ensureAgentBrowserTool = previousEnsure
		runAgentBrowserCommand = previousRunner
		runAgentBrowserOpen = previousOpenRunner
	})

	cmd := newAuthBrowserCmd(&rootFlags{asJSON: true})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if ensureCalled || runCalled {
		t.Fatal("verification harness installed or launched a browser")
	}
	if !strings.Contains(out.String(), `"skipped": "verification harness"`) {
		t.Fatalf("unexpected verification output: %s", out.String())
	}
}

func TestAgentBrowserReleasePinsOfficialAssets(t *testing.T) {
	tests := []struct {
		goos, goarch, filename, sha string
	}{
		{"windows", "amd64", "agent-browser-win32-x64.exe", "412ff72737a109e93f5304b0ff76c988fb6f1f451d0fc7e010577922bcc20ff3"},
		{"darwin", "arm64", "agent-browser-darwin-arm64", "b2106ab39db0838e7b1772f7f26f760518de56d09053150c56f9dddf15af997d"},
		{"linux", "amd64", "agent-browser-linux-x64", "56d15181e51e00213f907fcf39707cfc76bfa804ff20f5a9373661c73f96de5e"},
	}
	for _, test := range tests {
		release, ok := agentBrowserReleaseFor(test.goos, test.goarch)
		if !ok || release.Filename != test.filename || release.SHA256 != test.sha {
			t.Errorf("release for %s/%s = %#v, %v", test.goos, test.goarch, release, ok)
		}
	}
	if _, ok := agentBrowserReleaseFor("windows", "arm64"); ok {
		t.Fatal("unsupported Windows ARM64 release was accepted")
	}
	if _, ok := agentBrowserReleaseFor("linux", "arm64"); ok {
		t.Fatal("Linux ARM64 cannot install managed Chrome and must not be advertised")
	}
}

func TestInstallPinnedAgentBrowserVerifiesSHAAndWritesPrivateExecutable(t *testing.T) {
	payload := []byte("fake pinned agent-browser binary")
	sum := sha256.Sum256(payload)
	release := agentBrowserRelease{Filename: "agent-browser-test", SHA256: hex.EncodeToString(sum[:])}
	previousClient := agentBrowserHTTPClient
	agentBrowserHTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != agentBrowserReleaseBaseURL+"/"+release.Filename {
			t.Fatalf("download URL = %s", request.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(payload)),
			Header:     make(http.Header),
		}, nil
	})}
	t.Cleanup(func() { agentBrowserHTTPClient = previousClient })

	destination := filepath.Join(t.TempDir(), "tools", "agent-browser")
	if err := installPinnedAgentBrowser(context.Background(), destination, release); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("installed browser tool bytes did not match the verified release")
	}
	if info, err := os.Stat(destination); err != nil {
		t.Fatal(err)
	} else if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("browser tool permissions = %o, want no group/other access", info.Mode().Perm())
	}
}

func TestInstallPinnedAgentBrowserRejectsHashMismatchWithoutReplacingFile(t *testing.T) {
	previousClient := agentBrowserHTTPClient
	agentBrowserHTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("tampered")),
			Header:     make(http.Header),
		}, nil
	})}
	t.Cleanup(func() { agentBrowserHTTPClient = previousClient })

	destination := filepath.Join(t.TempDir(), "agent-browser")
	if err := os.WriteFile(destination, []byte("existing"), 0o700); err != nil {
		t.Fatal(err)
	}
	err := installPinnedAgentBrowser(context.Background(), destination, agentBrowserRelease{Filename: "test", SHA256: strings.Repeat("0", 64)})
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("error = %v, want SHA-256 rejection", err)
	}
	got, readErr := os.ReadFile(destination)
	if readErr != nil || string(got) != "existing" {
		t.Fatalf("existing tool changed after rejected download: %q, %v", got, readErr)
	}
}

func TestZapierCookieHeaderFiltersDomainsExpiredAndInvalidValues(t *testing.T) {
	future := float64(time.Now().Add(time.Hour).Unix())
	past := float64(time.Now().Add(-time.Hour).Unix())
	header, count := zapierCookieHeader([]agentBrowserCookie{
		{Name: "root", Value: "one", Domain: ".zapier.com", Path: "/", Expires: future},
		{Name: "reporting", Value: "two", Domain: "zapier.com", Path: "/api/reporting"},
		{Name: "subdomain", Value: "not-for-root", Domain: "app.zapier.com", Path: "/"},
		{Name: "wrong-path", Value: "not-for-api", Domain: "zapier.com", Path: "/app"},
		{Name: "evil", Value: "three", Domain: "evilzapier.com", Path: "/"},
		{Name: "expired", Value: "four", Domain: "zapier.com", Path: "/", Expires: past},
		{Name: "empty", Value: "", Domain: "zapier.com", Path: "/"},
		{Name: "bad name", Value: "five", Domain: "zapier.com", Path: "/"},
	})
	if count != 2 || header != "reporting=two; root=one" {
		t.Fatalf("cookie header = %q, count = %d", header, count)
	}
}

func TestZapierAPICookieScopeMatchesBrowserHostAndPathRules(t *testing.T) {
	for _, domain := range []string{"zapier.com", ".zapier.com", ".ZAPIER.COM."} {
		if !isZapierRequestCookieDomain(domain) {
			t.Errorf("isZapierRequestCookieDomain(%q) = false, want true", domain)
		}
	}
	for _, domain := range []string{"app.zapier.com", ".app.zapier.com", "evilzapier.com"} {
		if isZapierRequestCookieDomain(domain) {
			t.Errorf("isZapierRequestCookieDomain(%q) = true, want false", domain)
		}
	}
	for _, path := range []string{"", "/", "/api", "/api/", "/api/v4", "/api/v4/session", "/api/reporting"} {
		if !cookiePathMatchesZapierAPI(path) {
			t.Errorf("cookiePathMatchesZapierAPI(%q) = false, want true", path)
		}
	}
	for _, path := range []string{"app", "/app", "/api/v40", "/api/reporting-evil"} {
		if cookiePathMatchesZapierAPI(path) {
			t.Errorf("cookiePathMatchesZapierAPI(%q) = true, want false", path)
		}
	}
}

func TestZapierCookieDomainRequiresExactBoundary(t *testing.T) {
	for _, domain := range []string{"zapier.com", ".zapier.com", "app.zapier.com", ".API.ZAPIER.COM."} {
		if !isZapierCookieDomain(domain) {
			t.Errorf("isZapierCookieDomain(%q) = false, want true", domain)
		}
	}
	for _, domain := range []string{"evilzapier.com", "zapier.com.evil.test", "example.com", ""} {
		if isZapierCookieDomain(domain) {
			t.Errorf("isZapierCookieDomain(%q) = true, want false", domain)
		}
	}
}

func TestSignedInZapierURLRequiresHTTPSAndRejectsLoginRoutes(t *testing.T) {
	for _, candidate := range []string{"https://zapier.com/app/home", "https://app.zapier.com/dashboard"} {
		if !isSignedInZapierURL(candidate) {
			t.Errorf("isSignedInZapierURL(%q) = false, want true", candidate)
		}
	}
	for _, candidate := range []string{
		zapierLoginURL,
		"https://zapier.com/sign-up",
		"http://zapier.com/app/home",
		"https://evilzapier.com/app/home",
		"https://zapier.com/",
	} {
		if isSignedInZapierURL(candidate) {
			t.Errorf("isSignedInZapierURL(%q) = true, want false", candidate)
		}
	}
}

func TestPrivateAgentBrowserEnvironmentDropsCallerOverrides(t *testing.T) {
	t.Setenv("AGENT_BROWSER_PROVIDER", "remote")
	t.Setenv("agent_browser_extensions", "untrusted-extension")
	t.Setenv("AGENT_BROWSER_EXECUTABLE_PATH", "/untrusted/browser")
	t.Setenv("AGENT_BROWSER_NAMESPACE", "shared-default")
	t.Setenv("ZAPIER_TEST_PRESERVED", "yes")

	environment := privateAgentBrowserEnvironment(
		"AGENT_BROWSER_PROFILE=/private/profile",
		"AGENT_BROWSER_HEADED=true",
		"AGENT_BROWSER_NO_WEBMCP=true",
		"AGENT_BROWSER_NAMESPACE=private-namespace",
	)
	joined := strings.Join(environment, "\n")
	for _, blocked := range []string{"AGENT_BROWSER_PROVIDER=", "agent_browser_extensions=", "AGENT_BROWSER_EXECUTABLE_PATH=", "AGENT_BROWSER_NAMESPACE=shared-default"} {
		if strings.Contains(joined, blocked) {
			t.Fatalf("private browser environment retained %q", blocked)
		}
	}
	if !strings.Contains(joined, "ZAPIER_TEST_PRESERVED=yes") {
		t.Fatal("private browser environment removed an unrelated setting")
	}
	if !strings.Contains(joined, "NO_COLOR=1") {
		t.Fatal("private browser environment did not force no-color output")
	}
	for _, controlled := range []string{
		"AGENT_BROWSER_PROFILE=/private/profile",
		"AGENT_BROWSER_HEADED=true",
		"AGENT_BROWSER_NO_WEBMCP=true",
		"AGENT_BROWSER_NAMESPACE=private-namespace",
	} {
		if strings.Count(joined, controlled) != 1 {
			t.Fatalf("controlled browser setting %q was not installed exactly once", controlled)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
