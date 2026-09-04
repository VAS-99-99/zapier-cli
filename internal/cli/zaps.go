package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const zapsEndpoint = "/api/v4/zaps"
const zapsPageSize = 100

// zapNode is one step of a zap. Used to line a failing run step up against the
// zap's own step list.
type zapNode struct {
	ID          int64  `json:"id"`
	Title       string `json:"title,omitempty"`
	SelectedAPI string `json:"selected_api,omitempty"`
	Action      string `json:"action,omitempty"`
	TypeOf      string `json:"type_of,omitempty"`
}

// zapSummary is one zap as returned by zapsEndpoint. Field names match the wire
// format so the same struct both decodes the response and prints as JSON.
type zapSummary struct {
	ID         int64     `json:"id"`
	ZapID      string    `json:"zap_id,omitempty"`
	Title      string    `json:"title"`
	State      string    `json:"state"`
	Paused     bool      `json:"paused"`
	LastLiveAt string    `json:"last_live_at,omitempty"`
	Tasks      int       `json:"tasks"`
	Nodes      []zapNode `json:"nodes,omitempty"`
}

// zapCompact is the --compact projection: id, title, state and nothing else.
type zapCompact struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	State string `json:"state"`
}

// zapsPage is the response envelope. zapsEndpoint does not return a bare array.
type zapsPage struct {
	Count    int          `json:"count"`
	Next     string       `json:"next"`
	Previous string       `json:"previous"`
	Results  []zapSummary `json:"results"`
}

// fetchZaps reads until limit matching zaps have been found. A zero limit reads
// all pages. fresh bypasses cache reads for reporting commands.
func fetchZaps(cmd *cobra.Command, flags *rootFlags, limit int, match func(zapSummary) bool, fresh bool) ([]zapSummary, error) {
	client, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	get := client.Get
	if fresh {
		get = client.GetNoCache
	}
	out := make([]zapSummary, 0, zapsPageSize)
	offset := 0
	for limit == 0 || len(out) < limit {
		raw, err := get(cmd.Context(), zapsEndpoint, map[string]string{
			"limit":  strconv.Itoa(zapsPageSize),
			"offset": strconv.Itoa(offset),
		})
		if err != nil {
			return nil, apiErr(fmt.Errorf("listing zaps: %w", err))
		}
		var page zapsPage
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, apiErr(fmt.Errorf("parsing zap list: %w", err))
		}
		if len(page.Results) == 0 {
			break
		}
		for _, zap := range page.Results {
			if match == nil || match(zap) {
				out = append(out, zap)
				if limit > 0 && len(out) == limit {
					return out, nil
				}
			}
		}
		nextOffset := offset + len(page.Results)
		if nextOffset <= offset {
			return nil, apiErr(fmt.Errorf("zap pagination made no progress at offset %d", offset))
		}
		offset = nextOffset
		if page.Count > 0 {
			if offset >= page.Count {
				break
			}
			continue
		}
		if page.Next == "" {
			break
		}
	}
	return out, nil
}

func zapMatcher(needle string) func(zapSummary) bool {
	needle = strings.TrimSpace(needle)
	if id, err := strconv.ParseInt(needle, 10, 64); err == nil {
		return func(z zapSummary) bool { return z.ID == id }
	}
	normalized := strings.ToLower(needle)
	return func(z zapSummary) bool { return strings.Contains(strings.ToLower(z.Title), normalized) }
}

// compactZaps projects zaps down to the --compact fields.
func compactZaps(zaps []zapSummary) []zapCompact {
	out := make([]zapCompact, 0, len(zaps))
	for _, z := range zaps {
		out = append(out, zapCompact{ID: z.ID, Title: z.Title, State: z.State})
	}
	return out
}

func printLiveValue(w io.Writer, value any, flags *rootFlags) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(w, raw, flags, map[string]any{"source": "live"})
}

func newZapsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "zaps",
		Short: "List and find zaps in your account (read-only)",
	}
	cmd.AddCommand(newZapsListCmd(flags))
	return cmd
}

// pp:data-source live
func newZapsListCmd(flags *rootFlags) *cobra.Command {
	var name string
	var limit int
	c := &cobra.Command{
		Use:   "list",
		Short: "List zaps, optionally filtered by name",
		Long:  "Lists zaps in your account. Read-only: never turns a zap on/off, edits, or deletes anything.",
		Example: strings.Trim(`
  zapier-pp-cli zaps list
  zapier-pp-cli zaps list --name "webhook"
  zapier-pp-cli zaps list --limit 5 --compact
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if limit < 1 {
				return usageErr(fmt.Errorf("--limit must be at least 1"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "list zaps")
			}
			var match func(zapSummary) bool
			if strings.TrimSpace(name) != "" {
				match = zapMatcher(name)
			}
			zaps, err := fetchZaps(cmd, flags, limit, match, false)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if flags.compact && flags.selectFields == "" {
					return printLiveValue(cmd.OutOrStdout(), compactZaps(zaps), flags)
				}
				return printLiveValue(cmd.OutOrStdout(), zaps, flags)
			}
			for _, z := range zaps {
				fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\n", z.ID, z.Title, z.State)
			}
			return nil
		},
	}
	c.Flags().StringVar(&name, "name", "", "case-insensitive substring filter on zap title")
	c.Flags().IntVar(&limit, "limit", zapsPageSize, "maximum zaps to return")
	return c
}
