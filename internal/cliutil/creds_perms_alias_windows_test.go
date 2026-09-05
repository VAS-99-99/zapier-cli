//go:build windows

package cliutil

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsCredentialTrusteeAliasRequiresExactSID(t *testing.T) {
	localAdmin, err := resolveWindowsSDDLTrustee("LA")
	if err != nil {
		t.Fatalf("resolve native LA trustee: %v", err)
	}
	const foreign = "S-1-5-21-999-998-997-1000"
	if localAdmin == foreign {
		t.Fatal("fixture SID must differ from native LA SID")
	}
	for _, tc := range []struct {
		name, sddl, me string
		wantErr        bool
	}{
		{"same account", "O:LAD:(A;;FA;;;LA)", localAdmin, false},
		{"foreign ACE", "O:" + foreign + "D:(A;;FA;;;LA)", foreign, true},
		{"foreign owner", "O:LAD:(A;;FA;;;" + foreign + ")", foreign, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := evalCredsSecurityWithSIDResolver(tc.sddl, tc.me, resolveWindowsSDDLTrustee)
			if (err != nil) != tc.wantErr {
				t.Fatalf("native alias evaluation error = %v, want error = %v", err, tc.wantErr)
			}
		})
	}
	me, err := currentUserSID()
	if err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("O:" + me + "D:(A;;FA;;;" + me + ")")
	if err != nil {
		t.Fatal(err)
	}
	if err := evalCredsSecurityWithSIDResolver(sd.String(), me, resolveWindowsSDDLTrustee); err != nil {
		t.Fatalf("native current-user SDDL round trip rejected: %v", err)
	}
}
