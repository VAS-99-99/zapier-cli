package cli

import (
	"context"
	"errors"
	"os"
	"time"
)

// closeAgentBrowser deliberately omits launch settings. The pinned helper sends
// a launch command before close when its config contains headed/profile options.
// A lost browser must never be reopened by cleanup.
func closeAgentBrowser(ctx context.Context, binary, session string) error {
	config, err := os.CreateTemp("", "zpp-close-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(config.Name())
	if _, err := config.WriteString("{}\n"); err != nil {
		_ = config.Close()
		return err
	}
	if err := config.Close(); err != nil {
		return err
	}
	// CDP allows 30 seconds for Browser.close, then up to 5 seconds to reap
	// Chrome. Do not kill the client while the daemon is still shutting down.
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 45*time.Second)
	defer cancel()
	result, err := runAgentBrowserCommand(closeCtx, binary, "--config", config.Name(), "--namespace", session, "--session", session, "close", "--json")
	if err != nil {
		return err
	}
	var data struct {
		Closed bool `json:"closed"`
	}
	if result.Truncated || decodeAgentBrowserData(result.Stdout, &data) != nil || !data.Closed {
		return errors.New("browser close did not return a successful response")
	}
	return nil
}
