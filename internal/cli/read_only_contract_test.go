package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestReadOnlyContract_SpecEndpointsAndNovelHTTP(t *testing.T) {
	var walk func(*cobra.Command)
	inspected := 0
	walk = func(cmd *cobra.Command) {
		if cmd.Annotations["pp:endpoint"] != "" || cmd.Annotations["pp:method"] != "" || cmd.Annotations["pp:path"] != "" {
			inspected++
			if cmd.Annotations["pp:method"] != "GET" || cmd.Annotations["mcp:read-only"] != "true" {
				t.Errorf("spec endpoint %s is not GET/read-only: %v", cmd.CommandPath(), cmd.Annotations)
			}
		}
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(RootCmd())
	if inspected == 0 {
		t.Fatal("no spec endpoints were inspected")
	}
	sources := map[string]string{}
	for _, name := range []string{"zaps.go", "diagnose.go", "runs.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		sources[name] = string(data)
		for _, forbidden := range []string{".Put(", ".Patch(", ".Delete(", ".Post("} {
			if strings.Contains(sources[name], forbidden) {
				t.Errorf("%s contains unguarded non-read call %s", name, forbidden)
			}
		}
	}
	if strings.Count(sources["runs.go"], "PostQueryWithParams(") != 1 {
		t.Fatal("reporting GraphQL must be the only guarded non-GET novel path")
	}
	for _, required := range []string{"get := client.Get", "get = client.GetNoCache", "fetchZaps(cmd, flags, 0, zapMatcher(needle), true)", `client.GetNoCache(cmd.Context(), "/api/v4/session", nil)`} {
		if !strings.Contains(sources["zaps.go"]+sources["diagnose.go"]+sources["runs.go"], required) {
			t.Errorf("read method contract missing %q", required)
		}
	}
}
