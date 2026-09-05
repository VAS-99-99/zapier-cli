package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/zapier/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/zapier/internal/cliutil/testenv"
)

func TestSaveCredentialRollsBackWhenConfigWriteFails(t *testing.T) {
	for _, existing := range []bool{true, false} {
		name := "new credential"
		if existing {
			name = "existing credential"
		}
		t.Run(name, func(t *testing.T) {
			testenv.Isolate(t, cliutil.ConfigDir, cliutil.DataDir, LegacyConfigPath)
			t.Setenv("ZAPIER_SESSION_COOKIE", "")
			path, err := cliutil.CredentialsFilePath()
			if err != nil {
				t.Fatal(err)
			}
			var before []byte
			if existing {
				if err := cliutil.SaveCredentials(&cliutil.Credentials{ZapierSessionCookie: "session=old-secret", RefreshToken: "old-refresh-secret"}); err != nil {
					t.Fatal(err)
				}
				before, err = os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
			}
			cfg, err := Load("")
			if err != nil {
				t.Fatal(err)
			}
			// A regular file as parent forces config-write failure on every OS,
			// while the canonical credential directory remains writable.
			blocker := filepath.Join(t.TempDir(), "blocked")
			if err := os.WriteFile(blocker, []byte("fixture"), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg.Path = filepath.Join(blocker, "config.json")
			if err := cfg.SaveCredential("session=new-secret"); err == nil {
				t.Fatal("blocked config write reported success")
			} else if strings.Contains(err.Error(), "secret") {
				t.Fatal("save error exposed a credential")
			}
			after, err := os.ReadFile(path)
			if existing {
				if err != nil || !bytes.Equal(before, after) {
					t.Fatal("failed config write replaced the existing credential")
				}
			} else if !os.IsNotExist(err) {
				t.Fatal("failed config write left a new credential")
			}
		})
	}
}
