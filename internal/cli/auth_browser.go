// Copyright 2026 Vas and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/zapier/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/zapier/internal/config"
	"github.com/spf13/cobra"
)

const (
	agentBrowserVersion               = "0.36.0"
	agentBrowserReleaseBaseURL        = "https://github.com/vercel-labs/agent-browser/releases/download/v" + agentBrowserVersion
	agentBrowserMaxDownloadBytes      = 32 << 20
	agentBrowserMaxCommandOutputBytes = 8 << 20
	agentBrowserMaxCookieHeaderBytes  = 64 << 10
	agentBrowserInstallTimeout        = 5 * time.Minute
	agentBrowserCommandTimeout        = 30 * time.Second
	browserConnectTimeout             = 5 * time.Minute
	browserSessionVerifyTimeout       = 5 * time.Second
	browserSessionVerifyURL           = "https://zapier.com/api/v4/session"
	browserSessionMaxResponseBytes    = 1 << 20
	agentBrowserSessionPrefix         = "zp"
	agentBrowserManagedDirectory      = "browser-tools"
	agentBrowserPersistentProfile     = "browser-profile"
)

var zapierAPIPaths = []string{
	"/api/v4/session",
	"/api/v4/zaps",
	"/api/reporting/graphql",
}

var (
	errAgentBrowserMissing    = errors.New("private browser is not installed")
	errAgentBrowserWindowLost = errors.New("the private sign-in browser was closed or became unavailable; run auth browser again when ready")
	browserRuntimeGOOS        = runtime.GOOS
	browserRuntimeGOARCH      = runtime.GOARCH
	browserUserConfigDir      = os.UserConfigDir
	browserUserCacheDir       = os.UserCacheDir
	browserPollInterval       = 500 * time.Millisecond
	ensureAgentBrowserTool    = ensurePinnedAgentBrowser
	runAgentBrowserCommand    = execAgentBrowserCommand
	runAgentBrowserOpen       = execAgentBrowserOpen
	newAgentBrowserSession    = makeAgentBrowserSessionName
	agentBrowserHTTPClient    = &http.Client{Timeout: 2 * time.Minute}
	browserSessionHTTPClient  = &http.Client{Timeout: browserSessionVerifyTimeout}
)

type agentBrowserRelease struct {
	Filename string
	SHA256   string
}

type agentBrowserCommandResult struct {
	Stdout    []byte
	Stderr    []byte
	Truncated bool
}

type agentBrowserCookie struct {
	Name    string  `json:"name"`
	Value   string  `json:"value"`
	Domain  string  `json:"domain"`
	Path    string  `json:"path"`
	Expires float64 `json:"expires"`
}

type agentBrowserEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
}

// pp:data-source local
func newAuthBrowserCmd(flags *rootFlags) *cobra.Command {
	var noInstall bool
	cmd := &cobra.Command{
		Use:   "browser",
		Short: "Connect through a private browser window without copying a token",
		Long: "Open a dedicated Chrome profile, wait for you to sign in to Zapier, " +
			"and save only Zapier-scoped cookies to the CLI's private credential store. " +
			"Cookie values stay inside this process and are never printed.",
		Example:     "  zapier-pp-cli auth browser\n  zapier-pp-cli auth browser --timeout 10m",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"mcp:local-write": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsAnyHarness() {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"connected": false,
						"skipped":   "verification harness",
					}, flags)
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "verification harness: browser connection skipped")
				return nil
			}

			if _, ok := agentBrowserReleaseFor(browserRuntimeGOOS, browserRuntimeGOARCH); !ok {
				return authErr(fmt.Errorf("automatic browser connection is not available on %s/%s", browserRuntimeGOOS, browserRuntimeGOARCH))
			}

			binaryPath, installed, err := ensureAgentBrowserTool(cmd.Context(), !noInstall)
			if err != nil {
				return authErr(fmt.Errorf("preparing the private sign-in browser: %w", err))
			}
			browserCtx, cleanupSockets, err := prepareAgentBrowserSocketContext(cmd.Context())
			if err != nil {
				return configErr(err)
			}
			defer cleanupSockets()
			cmd.SetContext(browserCtx)

			profilePath, err := agentBrowserProfilePath()
			if err != nil {
				return configErr(err)
			}
			if err := removeManagedAgentBrowserProfile(profilePath); err != nil {
				return configErr(fmt.Errorf("clearing a prior private browser profile: %w", err))
			}
			if err := os.MkdirAll(profilePath, 0o700); err != nil {
				return configErr(fmt.Errorf("creating the private browser profile: %w", err))
			}
			defer cleanupAgentBrowserProfile(profilePath, cmd.ErrOrStderr())
			browserConfigPath, err := ensureAgentBrowserAuthConfig(profilePath)
			if err != nil {
				return configErr(err)
			}

			sessionName := newAgentBrowserSession()
			namespaceName := sessionName
			sessionTouched := false
			defer func() {
				if !sessionTouched {
					return
				}
				closeCtx, cancel := context.WithTimeout(context.WithoutCancel(cmd.Context()), 10*time.Second)
				defer cancel()
				result, err := runAgentBrowserCommand(closeCtx, binaryPath, "--config", browserConfigPath, "--namespace", namespaceName, "--session", sessionName, "close", "--json")
				var data json.RawMessage
				if err != nil || result.Truncated || decodeAgentBrowserData(result.Stdout, &data) != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), "warning: could not close the private sign-in browser; close its window manually before running auth browser again")
				}
			}()

			openBrowser := func(ctx context.Context) error {
				sessionTouched = true
				return runAgentBrowserOpen(ctx, binaryPath,
					"--config", browserConfigPath,
					"--namespace", namespaceName,
					"--session", sessionName,
					"--profile", profilePath,
					"open", zapierLoginURL,
					"--headed",
					"--no-webmcp",
					"--json",
				)
			}

			openCtx, cancelOpen := context.WithTimeout(cmd.Context(), agentBrowserCommandTimeout)
			openErr := openBrowser(openCtx)
			cancelOpen()
			browserInstalled := false
			openFailed := openErr != nil
			if errors.Is(openErr, errAgentBrowserMissing) {
				if noInstall {
					return authErr(errors.New("the private Zapier sign-in window could not be opened; rerun without --no-install to install Chrome for Testing if needed"))
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "Installing the private sign-in browser. This happens once.")
				installCtx, cancelInstall := context.WithTimeout(cmd.Context(), agentBrowserInstallTimeout)
				_, installErr := runAgentBrowserCommand(installCtx, binaryPath, "install")
				cancelInstall()
				if installErr != nil {
					return authErr(errors.New("Chrome for Testing installation failed"))
				}
				browserInstalled = true

				openCtx, cancelOpen = context.WithTimeout(cmd.Context(), agentBrowserCommandTimeout)
				openErr = openBrowser(openCtx)
				cancelOpen()
				openFailed = openErr != nil
			}
			if cmd.Context().Err() != nil || errors.Is(openErr, context.Canceled) {
				return authErr(errors.New("Zapier sign-in canceled; run auth browser again"))
			}
			pageCtx, cancelPage := context.WithTimeout(cmd.Context(), agentBrowserCommandTimeout)
			// A navigation wait may time out after the window has opened. Inspect
			// that window before giving up, without interrupting login or SSO.
			retryBlank := openBrowser
			if openFailed {
				retryBlank = nil
			}
			pageErr := ensureAgentBrowserLoginPage(pageCtx, binaryPath, browserConfigPath, sessionName, retryBlank)
			cancelPage()
			if pageErr != nil {
				return authErr(pageErr)
			}

			fmt.Fprintln(cmd.ErrOrStderr(), "Sign in to Zapier in the new browser window. Waiting for sign-in to finish...")
			wait := browserConnectTimeout
			if flags.timeoutExplicit && flags.timeout > 0 {
				wait = flags.timeout
			}
			loginCtx, cancelLogin := context.WithTimeout(cmd.Context(), wait)
			defer cancelLogin()
			cookieHeader, cookieCount, err := waitForAgentBrowserLogin(loginCtx, binaryPath, browserConfigPath, sessionName)
			if err != nil {
				return authErr(err)
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			closeCtx, cancelClose := context.WithTimeout(cmd.Context(), 10*time.Second)
			closeResult, closeErr := runAgentBrowserCommand(closeCtx, binaryPath, "--config", browserConfigPath, "--namespace", namespaceName, "--session", sessionName, "close", "--json")
			cancelClose()
			var closeData json.RawMessage
			if closeErr != nil || closeResult.Truncated || decodeAgentBrowserData(closeResult.Stdout, &closeData) != nil {
				return authErr(errors.New("the private sign-in browser could not be closed safely; close the window and run auth browser again"))
			}
			sessionTouched = false
			if err := removeManagedAgentBrowserProfile(profilePath); err != nil {
				return configErr(fmt.Errorf("clearing the temporary browser profile: %w", err))
			}
			if loginCtx.Err() != nil {
				return authErr(errors.New("Zapier sign-in was canceled or timed out before saving; run auth browser again"))
			}
			cfg.AuthHeaderVal = ""
			if err := cfg.SaveCredential(cookieHeader); err != nil {
				return configErr(fmt.Errorf("saving browser credential: %w", err))
			}

			out := map[string]any{
				"connected":                     true,
				"account_confirmation_required": true,
				"credential_saved":              true,
				"session_validated":             true,
				"verified":                      false,
				"verification":                  "pending account check",
				"browser":                       "Chrome",
				"browser_tool":                  "agent-browser",
				"browser_tool_version":          agentBrowserVersion,
				"browser_tool_installed":        installed,
				"browser_installed":             browserInstalled,
				"cookies_imported":              cookieCount,
				"config_path":                   cfg.Path,
				"next_step":                     "Run only zapier-pp-cli session --agent --no-learn to identify the connected account, then stop for confirmation.",
			}
			if !cfg.AgentcookieManagedByExternalStore() {
				out["credentials_path"] = credentialSavePath(cfg)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Connected to Zapier and saved the validated session (%d scoped cookies imported). Confirm the account next.\n", cookieCount)
			fmt.Fprintf(cmd.OutOrStdout(), "Session cookie saved to %s\n", credentialSavePath(cfg))
			fmt.Fprintln(cmd.OutOrStdout(), "Next: run only 'zapier-pp-cli session --agent --no-learn' to identify the connected account, then stop for confirmation.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&noInstall, "no-install", false, "Do not download the pinned browser tool or Chrome for Testing when missing")
	return cmd
}

func agentBrowserReleaseFor(goos, goarch string) (agentBrowserRelease, bool) {
	assets := map[string]agentBrowserRelease{
		"darwin/amd64":  {Filename: "agent-browser-darwin-x64", SHA256: "45d9ac061a7d72e61eaff905326e2e19365f4dadb12142ea2f2d76d84689c708"},
		"darwin/arm64":  {Filename: "agent-browser-darwin-arm64", SHA256: "b2106ab39db0838e7b1772f7f26f760518de56d09053150c56f9dddf15af997d"},
		"linux/amd64":   {Filename: "agent-browser-linux-x64", SHA256: "56d15181e51e00213f907fcf39707cfc76bfa804ff20f5a9373661c73f96de5e"},
		"windows/amd64": {Filename: "agent-browser-win32-x64.exe", SHA256: "412ff72737a109e93f5304b0ff76c988fb6f1f451d0fc7e010577922bcc20ff3"},
	}
	release, ok := assets[goos+"/"+goarch]
	return release, ok
}

func agentBrowserToolPath() (string, error) {
	base, err := browserUserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locating the user configuration directory: %w", err)
	}
	filename := "agent-browser"
	if browserRuntimeGOOS == "windows" {
		filename += ".exe"
	}
	return filepath.Join(base, "zapier-pp-cli", agentBrowserManagedDirectory, filename), nil
}

func agentBrowserProfilePath() (string, error) {
	if browserRuntimeGOOS == "windows" {
		base, err := browserUserCacheDir()
		if err != nil {
			return "", fmt.Errorf("locating the user cache directory: %w", err)
		}
		return filepath.Join(base, "ZapierCLI", agentBrowserPersistentProfile), nil
	}
	base, err := browserUserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locating the user configuration directory: %w", err)
	}
	return filepath.Join(base, "zapier-pp-cli", agentBrowserPersistentProfile), nil
}

func clearManagedAgentBrowserProfile() error {
	path, err := agentBrowserProfilePath()
	if err != nil {
		return err
	}
	return removeManagedAgentBrowserProfile(path)
}

func removeManagedAgentBrowserProfile(path string) error {
	if filepath.Base(filepath.Clean(path)) != agentBrowserPersistentProfile {
		return errors.New("refusing to remove an unexpected browser-profile path")
	}
	var err error
	for attempt := 0; attempt < 10; attempt++ {
		err = os.RemoveAll(path)
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

func cleanupAgentBrowserProfile(path string, warnings io.Writer) {
	if err := removeManagedAgentBrowserProfile(path); err != nil {
		fmt.Fprintln(warnings, "warning: could not remove the temporary sign-in profile; browser session data may remain on this machine; close the private browser and run auth browser again to retry cleanup")
	}
}

func agentBrowserAuthConfigPath() (string, error) {
	base, err := browserUserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locating the user configuration directory: %w", err)
	}
	return filepath.Join(base, "zapier-pp-cli", agentBrowserManagedDirectory, "zapier-auth-agent-browser.json"), nil
}

func ensureAgentBrowserAuthConfig(profilePath string) (string, error) {
	path, err := agentBrowserAuthConfigPath()
	if err != nil {
		return "", err
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("creating the browser-tool directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".zapier-auth-config-*")
	if err != nil {
		return "", fmt.Errorf("creating the browser-tool configuration: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return "", fmt.Errorf("securing the browser-tool configuration: %w", err)
	}
	settings, err := json.Marshal(map[string]any{
		"headed":   true,
		"noWebmcp": true,
		"profile":  profilePath,
	})
	if err != nil {
		return "", fmt.Errorf("encoding the browser-tool configuration: %w", err)
	}
	settings = append(settings, '\n')
	if _, err := temporary.Write(settings); err != nil {
		return "", fmt.Errorf("writing the browser-tool configuration: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return "", fmt.Errorf("syncing the browser-tool configuration: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("closing the browser-tool configuration: %w", err)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("replacing the browser-tool configuration: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return "", fmt.Errorf("installing the browser-tool configuration: %w", err)
	}
	committed = true
	return path, nil
}

func ensurePinnedAgentBrowser(ctx context.Context, allowInstall bool) (string, bool, error) {
	release, ok := agentBrowserReleaseFor(browserRuntimeGOOS, browserRuntimeGOARCH)
	if !ok {
		return "", false, fmt.Errorf("unsupported platform %s/%s", browserRuntimeGOOS, browserRuntimeGOARCH)
	}
	path, err := agentBrowserToolPath()
	if err != nil {
		return "", false, err
	}
	if matchesPinnedSHA256(path, release.SHA256) {
		return path, false, nil
	}
	if !allowInstall {
		return "", false, errors.New("the pinned agent-browser tool is missing; rerun without --no-install")
	}
	if err := installPinnedAgentBrowser(ctx, path, release); err != nil {
		return "", false, err
	}
	return path, true, nil
}

func installPinnedAgentBrowser(ctx context.Context, destination string, release agentBrowserRelease) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, agentBrowserReleaseBaseURL+"/"+release.Filename, nil)
	if err != nil {
		return errors.New("building the agent-browser download request failed")
	}
	response, err := agentBrowserHTTPClient.Do(request)
	if err != nil {
		return errors.New("downloading the pinned agent-browser release failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading the pinned agent-browser release returned HTTP %d", response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, agentBrowserMaxDownloadBytes+1))
	if err != nil {
		return errors.New("reading the pinned agent-browser release failed")
	}
	if len(payload) == 0 || len(payload) > agentBrowserMaxDownloadBytes {
		return errors.New("the pinned agent-browser release had an invalid size")
	}
	sum := sha256.Sum256(payload)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), release.SHA256) {
		return errors.New("the pinned agent-browser release failed SHA-256 verification")
	}

	directory := filepath.Dir(destination)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("creating the browser-tool directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".agent-browser-*")
	if err != nil {
		return fmt.Errorf("creating the browser-tool staging file: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o700); err != nil {
		return fmt.Errorf("securing the browser-tool staging file: %w", err)
	}
	if _, err := temporary.Write(payload); err != nil {
		return fmt.Errorf("writing the browser-tool staging file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("syncing the browser-tool staging file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("closing the browser-tool staging file: %w", err)
	}
	if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("replacing the browser tool: %w", err)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("installing the browser tool: %w", err)
	}
	committed = true
	return nil
}

func matchesPinnedSHA256(path, expected string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, io.LimitReader(file, agentBrowserMaxDownloadBytes+1)); err != nil {
		return false
	}
	info, err := file.Stat()
	if err != nil || info.Size() <= 0 || info.Size() > agentBrowserMaxDownloadBytes {
		return false
	}
	return strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), expected)
}

func makeAgentBrowserSessionName() string {
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err == nil {
		return fmt.Sprintf("%s-%x", agentBrowserSessionPrefix, suffix)
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())))
	return fmt.Sprintf("%s-%x", agentBrowserSessionPrefix, digest[:8])
}

func execAgentBrowserCommand(ctx context.Context, binary string, args ...string) (agentBrowserCommandResult, error) {
	return execAgentBrowserWithEnvironment(ctx, privateAgentBrowserEnvironment(), binary, args...)
}

type agentBrowserSocketContextKey struct{}

func prepareAgentBrowserSocketContext(ctx context.Context) (context.Context, func(), error) {
	if browserRuntimeGOOS != "darwin" {
		return ctx, func() {}, nil
	}
	// macOS sun_path permits only 103 bytes. Its normal TMPDIR and a long
	// home directory can exceed this even with compact session names. The
	// random directory is private (0700), not a shared or predictable socket.
	directory, err := os.MkdirTemp("/tmp", "zpp-")
	if err != nil {
		return ctx, nil, errors.New("could not create a private browser socket directory")
	}
	return context.WithValue(ctx, agentBrowserSocketContextKey{}, directory), func() {
		_ = os.RemoveAll(directory)
	}, nil
}

func execAgentBrowserWithEnvironment(ctx context.Context, environment []string, binary string, args ...string) (agentBrowserCommandResult, error) {
	if directory, ok := ctx.Value(agentBrowserSocketContextKey{}).(string); ok {
		environment = append(environment, "AGENT_BROWSER_SOCKET_DIR="+directory)
	}
	var stdout, stderr limitedCommandCapture
	command := exec.CommandContext(ctx, binary, args...) // #nosec G204 -- binary is the hash-verified managed tool and args are fixed by this package.
	// A detached browser daemon can inherit these pipes. Bound the drain after
	// the helper exits, including when a context deadline kills the helper.
	command.WaitDelay = 250 * time.Millisecond
	command.Stdout = &stdout
	command.Stderr = &stderr
	command.Env = environment
	err := command.Run()
	if errors.Is(err, exec.ErrWaitDelay) && ctx.Err() == nil {
		// The helper exited successfully; callers still validate its response.
		err = nil
	}
	return agentBrowserCommandResult{
		Stdout:    stdout.Bytes(),
		Stderr:    stderr.Bytes(),
		Truncated: stdout.truncated || stderr.truncated,
	}, err
}

// Validate navigation as well as process exit. A helper may return a failed
// JSON response with exit code zero; that must never become a login wait.
func execAgentBrowserOpen(ctx context.Context, binary string, args ...string) error {
	controlled := []string{
		"AGENT_BROWSER_HEADED=true",
		"AGENT_BROWSER_NO_WEBMCP=true",
	}
	if profile := commandArgumentValue(args, "--profile"); profile != "" {
		controlled = append(controlled, "AGENT_BROWSER_PROFILE="+profile)
	}
	if session := commandArgumentValue(args, "--session"); session != "" {
		controlled = append(controlled, "AGENT_BROWSER_SESSION="+session)
	}
	if namespace := commandArgumentValue(args, "--namespace"); namespace != "" {
		controlled = append(controlled, "AGENT_BROWSER_NAMESPACE="+namespace)
	}
	result, err := execAgentBrowserWithEnvironment(ctx, privateAgentBrowserEnvironment(controlled...), binary, args...)
	return agentBrowserOpenResult(ctx, result, err)
}

func agentBrowserOpenResult(ctx context.Context, result agentBrowserCommandResult, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// Classify only the pinned helper's explicit missing-browser response.
	// Never echo its raw output, which may contain URLs or session values.
	var failure struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if !result.Truncated && json.Unmarshal(result.Stdout, &failure) == nil && !failure.Success &&
		strings.HasPrefix(failure.Error, "Chrome not found.") &&
		strings.Contains(failure.Error, "Run `agent-browser install` to download Chrome") {
		return errAgentBrowserMissing
	}
	if err != nil {
		return err
	}
	if result.Truncated {
		return errors.New("browser navigation response exceeded the output limit")
	}
	var data json.RawMessage
	return decodeAgentBrowserData(result.Stdout, &data)
}

func privateAgentBrowserEnvironment(controlled ...string) []string {
	environment := make([]string, 0, len(os.Environ())+len(controlled)+1)
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(strings.ToUpper(name), "AGENT_BROWSER_") {
			continue
		}
		environment = append(environment, entry)
	}
	environment = append(environment, "NO_COLOR=1")
	return append(environment, controlled...)
}

func commandArgumentValue(args []string, name string) string {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == name {
			return args[index+1]
		}
	}
	return ""
}

type limitedCommandCapture struct {
	buffer    bytes.Buffer
	truncated bool
}

func (c *limitedCommandCapture) Write(data []byte) (int, error) {
	written := len(data)
	remaining := agentBrowserMaxCommandOutputBytes - c.buffer.Len()
	if remaining <= 0 {
		c.truncated = true
		return written, nil
	}
	if len(data) > remaining {
		_, _ = c.buffer.Write(data[:remaining])
		c.truncated = true
		return written, nil
	}
	_, _ = c.buffer.Write(data)
	return written, nil
}

func (c *limitedCommandCapture) Bytes() []byte {
	return append([]byte(nil), c.buffer.Bytes()...)
}

func ensureAgentBrowserLoginPage(ctx context.Context, binaryPath, configPath, sessionName string, open func(context.Context) error) error {
	for attempt := 0; attempt < 2; attempt++ {
		currentURL, err := agentBrowserCurrentURL(ctx, binaryPath, configPath, sessionName)
		if errors.Is(err, errAgentBrowserWindowLost) {
			return err
		}
		if err != nil && ctx.Err() == nil {
			// Navigation can briefly destroy the page's execution context.
			// Retry the read, never the navigation, while the window is alive.
			continue
		}
		if err == nil {
			parsed, parseErr := url.Parse(currentURL)
			// HTTPS redirects can be an identity provider or an MFA challenge.
			// Cookie capture still requires a Zapier URL and a validated session.
			if parseErr == nil && parsed.Scheme == "https" && parsed.Hostname() != "" {
				return nil
			}
		}
		if attempt == 0 && err == nil && currentURL == "about:blank" && open != nil && ctx.Err() == nil {
			if err := open(ctx); err != nil {
				break
			}
		} else {
			break
		}
	}
	return errors.New("the browser opened but the Zapier login page did not load; check the internet connection and run auth browser again")
}

func waitForAgentBrowserLogin(ctx context.Context, binaryPath, configPath, sessionName string) (string, int, error) {
	delay := browserPollInterval
	lastFailure := "finish signing in, including any verification challenge"
	for {
		if ctx.Err() != nil {
			return "", 0, browserLoginWaitError(ctx, lastFailure)
		}
		currentURL, err := agentBrowserCurrentURL(ctx, binaryPath, configPath, sessionName)
		if errors.Is(err, errAgentBrowserWindowLost) && ctx.Err() == nil {
			return "", 0, err
		}
		if err == nil && isSignedInZapierURL(currentURL) {
			cookies, cookieErr := agentBrowserCookies(ctx, binaryPath, configPath, sessionName)
			if errors.Is(cookieErr, errAgentBrowserWindowLost) && ctx.Err() == nil {
				return "", 0, cookieErr
			}
			header, count := zapierCookieHeader(cookies)
			if cookieErr == nil && count > 0 {
				if err := verifyAgentBrowserSession(ctx, header); err == nil {
					return header, count, nil
				} else {
					lastFailure = err.Error()
				}
			}
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", 0, browserLoginWaitError(ctx, lastFailure)
		case <-timer.C:
		}
		if delay < 2*time.Second {
			delay *= 2
			if delay > 2*time.Second {
				delay = 2 * time.Second
			}
		}
	}
}

func browserLoginWaitError(ctx context.Context, reason string) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return fmt.Errorf("Zapier sign-in canceled; %s; run auth browser again", reason)
	}
	return fmt.Errorf("timed out waiting for Zapier sign-in; %s; run auth browser again", reason)
}

// Validate the user-owned login in memory. This deliberately bypasses the
// configurable API client, cache, output and learning paths. Never include
// transport errors or response bytes in an error: either can contain secrets.
func verifyAgentBrowserSession(ctx context.Context, cookieHeader string) error {
	verifyCtx, cancel := context.WithTimeout(ctx, browserSessionVerifyTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(verifyCtx, http.MethodGet, browserSessionVerifyURL, nil)
	if err != nil {
		return errors.New("could not prepare the Zapier session check")
	}
	request.Header.Set("Cookie", cookieHeader)
	request.Header.Set("Accept", "application/json")
	client := *browserSessionHTTPClient
	client.Timeout = browserSessionVerifyTimeout
	client.Jar = nil
	// Even same-host redirects could lead outside the one permitted endpoint.
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(request)
	if err != nil {
		return errors.New("could not verify the Zapier session; check the internet connection and finish signing in")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return errors.New("the Zapier session is expired or incomplete; finish signing in")
	}
	if response.StatusCode != http.StatusOK {
		return errors.New("Zapier did not confirm the session; finish signing in and retry")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, browserSessionMaxResponseBytes+1))
	if err != nil || len(body) > browserSessionMaxResponseBytes {
		return errors.New("could not read the Zapier session check; retry signing in")
	}
	var session struct {
		LoggedIn       *bool           `json:"is_logged_in"`
		Temporary      *bool           `json:"is_temporary"`
		Masquerade     *bool           `json:"is_masquerade"`
		CurrentAccount json.RawMessage `json:"current_account_id"`
		ID             json.RawMessage `json:"id"`
		UserID         json.RawMessage `json:"user_id"`
	}
	if json.Unmarshal(body, &session) != nil {
		return errors.New("Zapier returned an invalid session check; retry signing in")
	}
	if session.LoggedIn == nil || !*session.LoggedIn || session.Temporary == nil || *session.Temporary || session.Masquerade == nil || *session.Masquerade ||
		!browserSessionPositiveID(session.CurrentAccount) || (!browserSessionPositiveID(session.ID) && !browserSessionPositiveID(session.UserID)) {
		return errors.New("Zapier has not confirmed a complete, non-temporary account session; finish signing in")
	}
	if verifyCtx.Err() != nil {
		return errors.New("Zapier session verification was canceled or timed out; retry signing in")
	}
	return nil
}

func browserSessionPositiveID(raw json.RawMessage) bool {
	value := strings.TrimSpace(string(raw))
	if strings.HasPrefix(value, `"`) {
		if json.Unmarshal(raw, &value) != nil {
			return false
		}
	}
	id, err := strconv.ParseInt(value, 10, 64)
	return err == nil && id > 0
}

// session info bypasses daemon startup and is a skip-launch action in the
// pinned helper. In contrast, get url and cookies can relaunch a closed browser.
// The helper has no atomic no-relaunch read, so a close between this probe and
// the next read can still race. Stop on any lost session instead of polling it.
func requireAgentBrowserWindow(ctx context.Context, binaryPath, configPath, sessionName string) error {
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result, err := runAgentBrowserCommand(probeCtx, binaryPath, "--config", configPath, "--namespace", sessionName, "--session", sessionName, "session", "info", "--json")
	var data struct {
		Active  bool `json:"active"`
		Runtime struct {
			BrowserLaunched bool `json:"browserLaunched"`
			PageCount       int  `json:"pageCount"`
		} `json:"runtime"`
	}
	if err != nil || result.Truncated || decodeAgentBrowserData(result.Stdout, &data) != nil ||
		!data.Active || !data.Runtime.BrowserLaunched || data.Runtime.PageCount < 1 {
		return errAgentBrowserWindowLost
	}
	return nil
}

func agentBrowserCurrentURL(ctx context.Context, binaryPath, configPath, sessionName string) (string, error) {
	if err := requireAgentBrowserWindow(ctx, binaryPath, configPath, sessionName); err != nil {
		return "", err
	}
	result, err := runAgentBrowserCommand(ctx, binaryPath, "--config", configPath, "--namespace", sessionName, "--session", sessionName, "get", "url", "--json")
	if err != nil || result.Truncated {
		return "", errors.New("browser status unavailable")
	}
	var data struct {
		URL string `json:"url"`
	}
	if err := decodeAgentBrowserData(result.Stdout, &data); err != nil {
		return "", err
	}
	if strings.TrimSpace(data.URL) == "" {
		return "", errors.New("browser did not return its current URL")
	}
	return data.URL, nil
}

func agentBrowserCookies(ctx context.Context, binaryPath, configPath, sessionName string) ([]agentBrowserCookie, error) {
	if err := requireAgentBrowserWindow(ctx, binaryPath, configPath, sessionName); err != nil {
		return nil, err
	}
	result, err := runAgentBrowserCommand(ctx, binaryPath, "--config", configPath, "--namespace", sessionName, "--session", sessionName, "cookies", "--json")
	if err != nil || result.Truncated {
		return nil, errors.New("browser cookies unavailable")
	}
	var data struct {
		Cookies []agentBrowserCookie `json:"cookies"`
	}
	if err := decodeAgentBrowserData(result.Stdout, &data); err != nil {
		return nil, err
	}
	return data.Cookies, nil
}

func decodeAgentBrowserData(payload []byte, destination any) error {
	var envelope agentBrowserEnvelope
	if err := json.Unmarshal(bytes.TrimSpace(payload), &envelope); err != nil || !envelope.Success || len(envelope.Data) == 0 {
		return errors.New("browser tool returned an invalid response")
	}
	if err := json.Unmarshal(envelope.Data, destination); err != nil {
		return errors.New("browser tool returned invalid data")
	}
	return nil
}

func isSignedInZapierURL(rawURL string) bool {
	parsed, err := urlParseHTTPS(rawURL)
	if err != nil || !isZapierCookieDomain(parsed.Hostname()) {
		return false
	}
	path := strings.ToLower(strings.TrimSpace(parsed.Path))
	for _, blocked := range []string{"/login", "/sign-in", "/signin", "/sign-up", "/signup", "/oauth"} {
		if strings.Contains(path, blocked) {
			return false
		}
	}
	return path != "" && path != "/"
}

func urlParseHTTPS(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return nil, errors.New("invalid HTTPS URL")
	}
	return parsed, nil
}

func zapierCookieHeader(cookies []agentBrowserCookie) (string, int) {
	now := float64(time.Now().Unix())
	filtered := make([]agentBrowserCookie, 0, len(cookies))
	for _, cookie := range cookies {
		if !isZapierRequestCookieDomain(cookie.Domain) || !cookiePathMatchesZapierAPI(cookie.Path) || strings.TrimSpace(cookie.Name) == "" || strings.TrimSpace(cookie.Value) == "" {
			continue
		}
		if cookie.Expires > 0 && cookie.Expires <= now {
			continue
		}
		candidate := &http.Cookie{Name: cookie.Name, Value: cookie.Value}
		if candidate.Valid() != nil || candidate.String() == "" {
			continue
		}
		filtered = append(filtered, cookie)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if len(filtered[i].Path) != len(filtered[j].Path) {
			return len(filtered[i].Path) > len(filtered[j].Path)
		}
		if filtered[i].Name != filtered[j].Name {
			return filtered[i].Name < filtered[j].Name
		}
		return filtered[i].Domain < filtered[j].Domain
	})
	pairs := make([]string, 0, len(filtered))
	totalBytes := 0
	for _, cookie := range filtered {
		pair := (&http.Cookie{Name: cookie.Name, Value: cookie.Value}).String()
		added := len(pair)
		if len(pairs) > 0 {
			added += 2
		}
		if totalBytes+added > agentBrowserMaxCookieHeaderBytes {
			return "", 0
		}
		pairs = append(pairs, pair)
		totalBytes += added
	}
	return strings.Join(pairs, "; "), len(pairs)
}

func isZapierCookieDomain(domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	domain = strings.TrimPrefix(domain, ".")
	domain = strings.TrimSuffix(domain, ".")
	return domain == "zapier.com" || strings.HasSuffix(domain, ".zapier.com")
}

func isZapierRequestCookieDomain(domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	domain = strings.TrimPrefix(domain, ".")
	domain = strings.TrimSuffix(domain, ".")
	return domain == "zapier.com"
}

func cookiePathMatchesZapierAPI(cookiePath string) bool {
	cookiePath = strings.TrimSpace(cookiePath)
	if cookiePath == "" {
		cookiePath = "/"
	}
	if !strings.HasPrefix(cookiePath, "/") {
		return false
	}
	for _, requestPath := range zapierAPIPaths {
		if requestPath == cookiePath {
			return true
		}
		if strings.HasPrefix(requestPath, cookiePath) && (strings.HasSuffix(cookiePath, "/") || requestPath[len(cookiePath)] == '/') {
			return true
		}
	}
	return false
}
