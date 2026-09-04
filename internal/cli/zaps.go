package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type zapSummary struct {
	ID         int64    `json:"id"`
	Title      string   `json:"title"`
	Apps       []string `json:"apps"`
	UpdatedAt  string   `json:"updated_at"`
	LastRunAt  string   `json:"last_run_at,omitempty"`
}

type searchZapsResult struct {
	Result struct {
		Data struct {
			Count   int          `json:"count"`
			Results []zapDetail `json:"results"`
		} `json:"data"`
	} `json:"result"`
}

type zapDetail struct {
	ID        int64    `json:"id"`
	Title     string   `json:"title"`
	Apps      []string `json:"apps"`
	UpdatedAt string   `json:"updatedAt"`
	LastRunAt string   `json:"lastRunAt"`
}

func newZapsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "zaps",
		Short: "List and find zaps in your account (read-only)",
	}
	cmd.AddCommand(newZapsListCmd(flags))
	return cmd
}

func newZapsListCmd(flags *rootFlags) *cobra.Command {
	var find string
	c := &cobra.Command{
		Use:   "list",
		Short: "List zaps, optionally filtered by name",
		Long:  "Lists zaps in your account. Read-only: never turns a zap on/off, edits, or deletes anything.",
		Example: strings.Trim(`
  zapier-pp-cli zaps list
  zapier-pp-cli zaps list --find "webhook"
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "list zaps")
			}
			client, err := flags.newClient()
			if err != nil {
				return err
			}
			input := map[string]any{"limit": 100, "offset": 0, "order": "-updated_at"}
			inputJSON, err := json.Marshal(input)
			if err != nil {
				return apiErr(err)
			}
			raw, err := client.Get(cmd.Context(), "/api/asset-management-bff/trpc/zap.searchZaps", map[string]string{
				"input": string(inputJSON),
			})
			if err != nil {
				return apiErr(fmt.Errorf("listing zaps: %w", err))
			}
			var parsed searchZapsResult
			if err := json.Unmarshal(raw, &parsed); err != nil {
				return apiErr(fmt.Errorf("parsing zap list: %w", err))
			}
			results := make([]zapSummary, 0, len(parsed.Result.Data.Results))
			for _, z := range parsed.Result.Data.Results {
				if find != "" && !strings.Contains(strings.ToLower(z.Title), strings.ToLower(find)) {
					continue
				}
				results = append(results, zapSummary{
					ID: z.ID, Title: z.Title, Apps: z.Apps,
					UpdatedAt: z.UpdatedAt, LastRunAt: z.LastRunAt,
				})
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			for _, z := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\n", z.ID, z.Title, strings.Join(z.Apps, ","))
			}
			return nil
		},
	}
	c.Flags().StringVar(&find, "find", "", "case-insensitive substring filter on zap title")
	return c
}
