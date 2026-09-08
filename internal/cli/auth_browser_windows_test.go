package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// The unknown feature produces a harmless daemon warning on a blank page.
// Old Windows helpers panic writing that warning to their abandoned stderr pipe.
func TestAgentBrowserWindowsWarningKeepsBrowserAlive(t *testing.T) {
	if runtime.GOOS != "windows" || os.Getenv("ZAPIER_TEST_NATIVE_BROWSER") != "1" {
		t.Skip("opt-in Windows blank-browser warning regression")
	}
	helper := os.Getenv("ZAPIER_TEST_BROWSER_BINARY")
	if helper == "" {
		var err error
		helper, _, err = ensurePinnedAgentBrowser(context.Background(), os.Getenv("ZAPIER_TEST_BROWSER_INSTALL") == "1")
		if err != nil {
			t.Fatal(err)
		}
	}
	root := t.TempDir()
	config := filepath.Join(root, "browser.json")
	if err := os.WriteFile(config, []byte(`{"headed":true,"noWebmcp":true,"enable":["zpp-regression-warning"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	session := makeAgentBrowserSessionName()
	args := []string{"--config", config, "--namespace", session, "--session", session}
	ctx, cancel := context.WithTimeout(context.Background(), agentBrowserLaunchTimeout)
	defer cancel()
	defer func() {
		if err := closeAgentBrowser(context.Background(), helper, session); err != nil {
			t.Errorf("browser close failed: %v", err)
		}
	}()
	page := "about:blank"
	if os.Getenv("ZAPIER_TEST_PUBLIC_LOGIN") == "1" {
		// A new disposable profile; no authentication or cookie reads.
		page = zapierLoginURL
	}
	if err := execAgentBrowserOpen(ctx, helper, append(args, "--profile", filepath.Join(root, "profile"), "--headed", "open", page, "--json")...); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := requireAgentBrowserWindow(ctx, helper, config, session); err != nil {
			t.Fatal(err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
