package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type diagnoseFailure struct {
	RunID     string `json:"run_id"`
	StartTime string `json:"start_time"`
	StepTitle string `json:"step_title"`
	App       string `json:"app"`
	Error     string `json:"error"`
}

type diagnoseResult struct {
	ZapID    string            `json:"zap_id"`
	ZapTitle string            `json:"zap_title"`
	Checked  int               `json:"runs_checked"`
	Failures []diagnoseFailure `json:"failures"`
	Note     string            `json:"note,omitempty"`
}

func newDiagnoseCmd(flags *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{
		Use:   "diagnose <zap-name-or-id>",
		Short: "Find a zap by name, pull its recent failed runs, and show exactly which step broke",
		Long: "One-shot troubleshooting: finds the zap matching the given name or id, lists its recent " +
			"error runs, and opens each one to report the failing step and its error message. " +
			"Read-only end to end: it only reads zap and run data, never edits, replays, or toggles anything.",
		Example: strings.Trim(`
  zapier-pp-cli diagnose webhook
  zapier-pp-cli diagnose 378928393 --limit 10
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "diagnose zap")
			}
			needle := args[0]

			client, err := flags.newClient()
			if err != nil {
				return err
			}

			// Step 1: find the zap by id or name substring.
			input := map[string]any{"limit": 100, "offset": 0, "order": "-updated_at"}
			inputJSON, _ := json.Marshal(input)
			rawZaps, err := client.Get(cmd.Context(), "/api/asset-management-bff/trpc/zap.searchZaps", map[string]string{
				"input": string(inputJSON),
			})
			if err != nil {
				return apiErr(fmt.Errorf("listing zaps: %w", err))
			}
			var searchResp searchZapsResult
			if err := json.Unmarshal(rawZaps, &searchResp); err != nil {
				return apiErr(fmt.Errorf("parsing zap list: %w", err))
			}
			var match *zapDetail
			for i, z := range searchResp.Result.Data.Results {
				if fmt.Sprintf("%d", z.ID) == needle || strings.Contains(strings.ToLower(z.Title), strings.ToLower(needle)) {
					match = &searchResp.Result.Data.Results[i]
					break
				}
			}
			if match == nil {
				return notFoundErr(fmt.Errorf("no zap matching %q found in your account", needle))
			}

			// Step 2: list its recent error runs.
			accountID, err := currentAccountID(cmd, flags)
			if err != nil {
				return err
			}
			zapIDStr := fmt.Sprintf("%d", match.ID)
			rawRuns, err := callReportingGraphQL(cmd, flags, graphqlRequest{
				OperationName: "ZapRuns",
				Variables: map[string]any{
					"accountId": accountID,
					"zapIds":    []string{zapIDStr},
					"status":    []string{"error"},
					"limit":     limit,
					"offset":    0,
					"sortBy":    "-start_time",
				},
				Query: zapRunsQuery,
			})
			if err != nil {
				return err
			}
			var runsParsed struct {
				Data struct {
					ZapRuns struct {
						Edges []struct {
							ID string `json:"id"`
						} `json:"edges"`
					} `json:"zapRuns"`
				} `json:"data"`
			}
			if err := json.Unmarshal(rawRuns, &runsParsed); err != nil {
				return apiErr(fmt.Errorf("parsing error runs: %w", err))
			}

			result := diagnoseResult{ZapID: zapIDStr, ZapTitle: match.Title}
			if len(runsParsed.Data.ZapRuns.Edges) == 0 {
				result.Note = "no failed runs found in the checked window"
			}

			// Step 3: open each error run and find the failing step.
			for _, edge := range runsParsed.Data.ZapRuns.Edges {
				result.Checked++
				rawDetail, err := callReportingGraphQL(cmd, flags, graphqlRequest{
					OperationName: "RunDetail",
					Variables:     map[string]any{"runId": edge.ID},
					Query:         runDetailQuery,
				})
				if err != nil {
					continue // skip a run we can't read; don't fail the whole diagnosis
				}
				var detailParsed struct {
					Data struct {
						ZapRun *struct {
							StartTime string    `json:"startTime"`
							Steps     []runStep `json:"steps"`
						} `json:"zapRun"`
					} `json:"data"`
				}
				if err := json.Unmarshal(rawDetail, &detailParsed); err != nil || detailParsed.Data.ZapRun == nil {
					continue
				}
				for _, s := range detailParsed.Data.ZapRun.Steps {
					if s.Status == "error" || (s.Error != nil && s.Error.Title != "") {
						msg := ""
						if s.Error != nil {
							msg = s.Error.Title
						}
						result.Failures = append(result.Failures, diagnoseFailure{
							RunID: edge.ID, StartTime: detailParsed.Data.ZapRun.StartTime,
							StepTitle: s.Title, App: s.App, Error: msg,
						})
					}
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (zap %s)\n", result.ZapTitle, result.ZapID)
			if result.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), result.Note)
				return nil
			}
			for _, f := range result.Failures {
				fmt.Fprintf(cmd.OutOrStdout(), "  run %s @ %s: %q failed in %s\n", f.RunID, f.StartTime, f.StepTitle, f.App)
				if f.Error != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "    error: %s\n", f.Error)
				}
			}
			return nil
		},
	}
	c.Flags().IntVar(&limit, "limit", 10, "maximum failed runs to inspect")
	return c
}
