package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuthSetupExplainsAutomaticBrowserConnection(t *testing.T) {
	cmd := newAuthSetupCmd(&rootFlags{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"https://zapier.com/app/login",
		"zapier-pp-cli auth browser",
		"Vercel Labs agent-browser",
		"Chrome for Testing",
		"never prints or asks you to paste them",
		"zapier-pp-cli session --agent --no-learn",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("setup output missing %q: %s", want, got)
		}
	}
	for _, forbidden := range []string{"api" + " key", "$" + "TOKEN", "Cookie request-header value", "Get-Content -Raw"} {
		if strings.Contains(strings.ToLower(got), strings.ToLower(forbidden)) {
			t.Fatalf("setup output must not contain %q: %s", forbidden, got)
		}
	}
	if strings.Contains(got, "export ZAPIER_SESSION_COOKIE=\"") {
		t.Fatalf("setup must not put the session cookie in shell history: %s", got)
	}
}

func TestAuthLogoutClearsManagedBrowserProfile(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	previousGOOS := browserRuntimeGOOS
	previousCacheDir := browserUserCacheDir
	browserRuntimeGOOS = "windows"
	browserUserCacheDir = func() (string, error) { return configRoot, nil }
	t.Cleanup(func() {
		browserRuntimeGOOS = previousGOOS
		browserUserCacheDir = previousCacheDir
	})

	profilePath := filepath.Join(configRoot, "ZapierCLI", agentBrowserPersistentProfile)
	if err := os.MkdirAll(profilePath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profilePath, "Cookies"), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newAuthLogoutCmd(&rootFlags{})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(profilePath); !os.IsNotExist(err) {
		t.Fatalf("managed browser profile remained after logout: %v", err)
	}
}

func TestManualSetTokenIsAbsentFromCommandsAndAgentDiscovery(t *testing.T) {
	auth := newAuthCmd(&rootFlags{})
	for _, command := range auth.Commands() {
		if command.Name() == "set-token" {
			t.Fatal("manual set-token command is still registered")
		}
	}
	contextBytes, err := json.Marshal(buildAgentContext(RootCmd()))
	if err != nil {
		t.Fatal(err)
	}
	contextText := string(contextBytes)
	for _, forbidden := range []string{"set-token", "ZAPIER_SESSION_COOKIE"} {
		if strings.Contains(contextText, forbidden) {
			t.Fatalf("agent discovery exposed manual credential surface %q", forbidden)
		}
	}
	if !strings.Contains(contextText, `"mode":"browser_session"`) {
		t.Fatal("agent discovery did not advertise browser-session authentication")
	}
}

func TestAuthSetupVerifyModeNeverOpensBrowser(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	previousEnsure := ensureAgentBrowserTool
	previousRunner := runAgentBrowserCommand
	ensureCalled := false
	runCalled := false
	ensureAgentBrowserTool = func(_ context.Context, _ bool) (string, bool, error) {
		ensureCalled = true
		return "", false, nil
	}
	runAgentBrowserCommand = func(_ context.Context, _ string, _ ...string) (agentBrowserCommandResult, error) {
		runCalled = true
		return agentBrowserCommandResult{}, nil
	}
	t.Cleanup(func() {
		ensureAgentBrowserTool = previousEnsure
		runAgentBrowserCommand = previousRunner
	})

	cmd := newAuthSetupCmd(&rootFlags{})
	cmd.SetArgs([]string{"--launch"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if ensureCalled || runCalled {
		t.Fatal("verification mode installed or launched the browser tool")
	}
}

func TestAuthSetupDogfoodModeNeverOpensBrowser(t *testing.T) {
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
	previousEnsure := ensureAgentBrowserTool
	previousRunner := runAgentBrowserCommand
	ensureCalled := false
	runCalled := false
	ensureAgentBrowserTool = func(_ context.Context, _ bool) (string, bool, error) {
		ensureCalled = true
		return "", false, nil
	}
	runAgentBrowserCommand = func(_ context.Context, _ string, _ ...string) (agentBrowserCommandResult, error) {
		runCalled = true
		return agentBrowserCommandResult{}, nil
	}
	t.Cleanup(func() {
		ensureAgentBrowserTool = previousEnsure
		runAgentBrowserCommand = previousRunner
	})

	cmd := newAuthSetupCmd(&rootFlags{})
	cmd.SetArgs([]string{"--launch"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if ensureCalled || runCalled {
		t.Fatal("dogfood mode installed or launched the browser tool")
	}
}
