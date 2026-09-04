package cli

import (
	"strings"
	"testing"
)

func TestZapierWhichIndexResolvesNaturalLanguage(t *testing.T) {
	for query, want := range map[string]string{
		"list zaps":      "zaps list",
		"failed runs":    "runs list",
		"diagnose a zap": "diagnose",
	} {
		matches := rankWhich(whichIndex, query, 1)
		if len(matches) != 1 || matches[0].Entry.Command != want {
			t.Fatalf("which %q = %+v, want %q", query, matches, want)
		}
	}
}

func TestZapierWhichIndexCommandsResolveExactly(t *testing.T) {
	root := newRootCmd(&rootFlags{})
	for _, entry := range whichIndex {
		found, remaining, err := root.Find(strings.Fields(entry.Command))
		if err != nil || len(remaining) != 0 || found == nil || found.CommandPath() != root.Name()+" "+entry.Command {
			t.Fatalf("command %q did not resolve exactly: found=%v remaining=%v err=%v", entry.Command, found, remaining, err)
		}
	}
}
