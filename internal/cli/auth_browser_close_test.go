package cli

import (
	"context"
	"os"
	"testing"
)

func TestCloseAgentBrowserCannotRequestLaunch(t *testing.T) {
	previous := runAgentBrowserCommand
	t.Cleanup(func() { runAgentBrowserCommand = previous })
	var configPath string
	runAgentBrowserCommand = func(ctx context.Context, binary string, args ...string) (agentBrowserCommandResult, error) {
		configPath = commandArgumentValue(args, "--config")
		contents, err := os.ReadFile(configPath)
		if err != nil || string(contents) != "{}\n" {
			t.Fatalf("close must omit all launch settings: %q, %v", contents, err)
		}
		if !hasArgSequence(args, "--namespace", "only-this-session", "--session", "only-this-session", "close", "--json") {
			t.Fatalf("close escaped its session: %v", args)
		}
		if ctx.Err() != nil {
			t.Fatal("cancellation prevented browser cleanup")
		}
		return jsonAgentBrowserResult(`{"closed":true}`), nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := closeAgentBrowser(ctx, "helper", "only-this-session"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatal("temporary close config was not removed")
	}
}

func TestCloseAgentBrowserRequiresConfirmedClosure(t *testing.T) {
	previous := runAgentBrowserCommand
	t.Cleanup(func() { runAgentBrowserCommand = previous })
	for _, payload := range []string{`{}`, `{"closed":false}`} {
		runAgentBrowserCommand = func(context.Context, string, ...string) (agentBrowserCommandResult, error) {
			return jsonAgentBrowserResult(payload), nil
		}
		if err := closeAgentBrowser(context.Background(), "helper", "session"); err == nil {
			t.Fatalf("accepted unconfirmed shutdown: %s", payload)
		}
	}
}
