package cli

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestAuthSetupExplainsZapierSessionCookie(t *testing.T) {
	cmd := newAuthSetupCmd(&rootFlags{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"https://zapier.com/app/login",
		"Cookie request-header value",
		"ZAPIER_SESSION_COOKIE",
		"printf '%s'",
		"Get-Content -Raw",
		"Remove-Item",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("setup output missing %q: %s", want, got)
		}
	}
	for _, forbidden := range []string{"api" + " key", "$" + "TOKEN"} {
		if strings.Contains(strings.ToLower(got), strings.ToLower(forbidden)) {
			t.Fatalf("setup output must not contain %q: %s", forbidden, got)
		}
	}
	if strings.Contains(got, "export ZAPIER_SESSION_COOKIE=\"") {
		t.Fatalf("setup must not put the session cookie in shell history: %s", got)
	}
}

func TestAuthSetupVerifyModeNeverOpensBrowser(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	previous := launchBrowser
	t.Cleanup(func() { launchBrowser = previous })
	launchBrowser = func(string) error { return errors.New("browser must not open") }

	cmd := newAuthSetupCmd(&rootFlags{})
	cmd.SetArgs([]string{"--launch"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("PRINTING_PRESS_VERIFY") != "1" {
		t.Fatal("test did not retain verify mode")
	}
}

func TestAuthSetupDogfoodModeNeverOpensBrowser(t *testing.T) {
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
	previous := launchBrowser
	t.Cleanup(func() { launchBrowser = previous })
	launchBrowser = func(string) error { return errors.New("browser must not open") }

	cmd := newAuthSetupCmd(&rootFlags{})
	cmd.SetArgs([]string{"--launch"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}
