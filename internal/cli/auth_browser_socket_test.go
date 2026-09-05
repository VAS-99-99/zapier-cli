package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestAuthBrowserMacSocketContextReachesCancellationCleanup(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS socket directory")
	}
	fake := &fakeAgentBrowser{}
	stubAgentBrowserGlobals(t, t.TempDir(), fake)
	browserRuntimeGOOS = "darwin"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var socketDirectory string
	closed := false
	runAgentBrowserOpen = func(ctx context.Context, _ string, _ ...string) error {
		socketDirectory, _ = ctx.Value(agentBrowserSocketContextKey{}).(string)
		if socketDirectory == "" {
			t.Fatal("launch lost the private socket context")
		}
		cancel()
		return context.Canceled
	}
	runAgentBrowserCommand = func(ctx context.Context, _ string, args ...string) (agentBrowserCommandResult, error) {
		if !containsArg(args, "close") {
			t.Fatal("canceled login issued a non-close command")
		}
		if ctx.Err() != nil || ctx.Value(agentBrowserSocketContextKey{}) != socketDirectory {
			t.Fatal("cleanup lost socket context or retained cancellation")
		}
		if _, err := os.Stat(socketDirectory); err != nil {
			t.Fatal("socket directory removed before browser close")
		}
		closed = true
		return jsonAgentBrowserResult(`{"closed":true}`), nil
	}
	cmd := newAuthBrowserCmd(&rootFlags{})
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil || !closed {
		t.Fatal("cancellation did not close its own browser")
	}
	if _, err := os.Stat(socketDirectory); !os.IsNotExist(err) {
		t.Fatal("private socket directory survived cleanup")
	}
}

func TestAgentBrowserSessionNameFitsMacSocketPath(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		name := makeAgentBrowserSessionName()
		path := filepath.Join("/Users/example/.agent-browser/namespaces", name, "run", name+".sock")
		if len(path) > 103 {
			t.Fatalf("generated socket path is %d bytes; macOS maximum is 103: %s", len(path), path)
		}
		if len(name) > 19 || seen[name] {
			t.Fatalf("session name is too long or duplicated: %s", name)
		}
		seen[name] = true
	}
}

func TestAgentBrowserSocketContextPlatformIsolation(t *testing.T) {
	previous := browserRuntimeGOOS
	t.Cleanup(func() { browserRuntimeGOOS = previous })
	for _, platform := range []string{"darwin", "windows", "linux"} {
		t.Run(platform, func(t *testing.T) {
			if platform == "darwin" && runtime.GOOS != "darwin" {
				t.Skip("macOS short temporary directory is tested on macOS")
			}
			browserRuntimeGOOS = platform
			ctx, cleanup, err := prepareAgentBrowserSocketContext(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			directory, hasDirectory := ctx.Value(agentBrowserSocketContextKey{}).(string)
			if platform != "darwin" {
				if hasDirectory {
					t.Fatal("changed non-macOS socket handling")
				}
				return
			}
			info, err := os.Stat(directory)
			if err != nil {
				t.Fatal(err)
			}
			if runtime.GOOS != "windows" && info.Mode().Perm() != 0700 {
				t.Fatal("socket directory is not private")
			}
			name := makeAgentBrowserSessionName()
			canonical, err := filepath.EvalSymlinks(directory)
			if err != nil {
				t.Fatal(err)
			}
			if len(filepath.Join(canonical, "namespaces", name, "run", name+".sock")) > 103 {
				t.Fatal("socket exceeds macOS path limit")
			}
			canceled, cancel := context.WithCancel(ctx)
			cancel()
			if context.WithoutCancel(canceled).Value(agentBrowserSocketContextKey{}) != directory {
				t.Fatal("cleanup lost socket location")
			}
			cleanup()
			if _, err := os.Stat(directory); !os.IsNotExist(err) {
				t.Fatal("socket directory not removed")
			}
		})
	}
}

// This optional native test runs only a blank page, never the login command.
// It exercises the real launch/probe/close seam with the pinned local helper.
func TestAgentBrowserNativeMacSocketLifecycle(t *testing.T) {
	if runtime.GOOS != "darwin" || os.Getenv("ZAPIER_TEST_NATIVE_BROWSER") != "1" {
		t.Skip("opt-in macOS blank-browser test")
	}
	helper, _, err := ensurePinnedAgentBrowser(context.Background(), os.Getenv("ZAPIER_TEST_BROWSER_INSTALL") == "1")
	if err != nil {
		t.Fatal(err)
	}
	// Use a synthetic long home and runtime directory; no existing profiles.
	home := filepath.Join(t.TempDir(), strings.Repeat("long-home-", 12))
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(home, "runtime"))
	deadline, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	ctx, cleanup, err := prepareAgentBrowserSocketContext(deadline)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	profile := filepath.Join(home, "profile")
	config := filepath.Join(home, "browser.json")
	if err := os.WriteFile(config, []byte(`{"headed":true,"noWebmcp":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	session := makeAgentBrowserSessionName()
	args := []string{"--config", config, "--namespace", session, "--session", session}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		result, err := execAgentBrowserCommand(closeCtx, helper, append(args, "close", "--json")...)
		if err != nil || !strings.Contains(string(result.Stdout), `"success":true`) {
			t.Errorf("diagnostic browser close failed: %v", err)
		}
	}()
	if err := execAgentBrowserOpen(ctx, helper, append(args, "--profile", profile, "--headed", "open", "about:blank", "--json")...); err != nil {
		t.Fatal(err)
	}
	if err := requireAgentBrowserWindow(ctx, helper, config, session); err != nil {
		t.Fatal(err)
	}
}
