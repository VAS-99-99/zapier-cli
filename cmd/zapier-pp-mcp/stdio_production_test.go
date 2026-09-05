package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Run the real entry point in a child, without credentials or tool execution.
func TestMCPProductionStdioFixture(t *testing.T) {
	if os.Getenv("ZAPIER_MCP_STDIO_FIXTURE") != "1" {
		return
	}
	flag.CommandLine = flag.NewFlagSet("mcp-fixture", flag.ExitOnError)
	os.Args = []string{os.Args[0], "--transport", "stdio"}
	main()
	os.Exit(0)
}

func TestMCPProductionStdioDiscovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestMCPProductionStdioFixture$")
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(strings.ToUpper(name), "ZAPIER_") && !strings.HasPrefix(name, "PP_MCP_") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "ZAPIER_MCP_STDIO_FIXTURE=1", "ZAPIER_NO_LEARN=true", "ZAPIER_HOME="+t.TempDir())
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	encoder, decoder := json.NewEncoder(stdin), json.NewDecoder(stdout)
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{
		"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "release-fixture", "version": "1"},
	}}); err != nil {
		t.Fatal(err)
	}
	var reply struct {
		ID     int             `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := decoder.Decode(&reply); err != nil || reply.ID != 1 || len(reply.Error) != 0 {
		t.Fatalf("MCP initialization failed: decode=%v id=%d error=%s", err, reply.ID, reply.Error)
	}
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&reply); err != nil || reply.ID != 2 || len(reply.Error) != 0 {
		t.Fatalf("MCP tool discovery failed: decode=%v id=%d error=%s", err, reply.ID, reply.Error)
	}
	var result struct {
		Tools []struct {
			Name        string `json:"name"`
			Annotations struct {
				ReadOnly bool `json:"readOnlyHint"`
			} `json:"annotations"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(reply.Result, &result); err != nil {
		t.Fatal(err)
	}
	required := map[string]bool{"session_check": false, "zaps_list": false, "runs_list": false, "runs_get": false, "diagnose": false}
	for _, tool := range result.Tools {
		if _, ok := required[tool.Name]; ok {
			if !tool.Annotations.ReadOnly {
				t.Errorf("inspection tool %s is not marked read-only", tool.Name)
			}
			required[tool.Name] = true
		}
	}
	for name, found := range required {
		if !found {
			t.Errorf("MCP discovery omitted %s", name)
		}
	}
	_ = stdin.Close()
}
