package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Use the test executable as a browser helper whose daemon inherits output
// handles. This reproduces the Windows startup hang without a real login.
func TestBrowserProcessFixture(t *testing.T) {
	mode := os.Getenv("ZAPIER_BROWSER_PROCESS_FIXTURE")
	if mode == "" {
		return
	}
	if mode == "child" {
		time.Sleep(3 * time.Second)
		os.Exit(0)
	}
	if mode == "failure" {
		fmt.Print(`{"success":false,"error":"navigation failed"}`)
		os.Exit(0)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestBrowserProcessFixture$")
	child.Env = append(os.Environ(), "ZAPIER_BROWSER_PROCESS_FIXTURE=child")
	child.Stdout, child.Stderr = os.Stdout, os.Stderr
	if err := child.Start(); err != nil {
		os.Exit(2)
	}
	fmt.Print(`{"success":true,"data":{"url":"https://zapier.com/app/login"}}`)
	os.Exit(0)
}

func TestBrowserCommandDoesNotWaitForInheritedDaemonHandles(t *testing.T) {
	t.Setenv("ZAPIER_BROWSER_PROCESS_FIXTURE", "parent")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	result, _ := execAgentBrowserCommand(ctx, os.Args[0], "-test.run=^TestBrowserProcessFixture$")
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("helper held inherited output handles for %s after its parent exited", elapsed)
	}
	if !strings.Contains(string(result.Stdout), `"success":true`) {
		t.Fatal("missing helper response")
	}
}

func TestBrowserOpenRejectsFailureEnvelopeWithZeroExitCode(t *testing.T) {
	t.Setenv("ZAPIER_BROWSER_PROCESS_FIXTURE", "failure")
	if err := execAgentBrowserOpen(context.Background(), os.Args[0], "-test.run=^TestBrowserProcessFixture$"); err == nil {
		t.Fatal("failed navigation was accepted as an opened login window")
	}
}
