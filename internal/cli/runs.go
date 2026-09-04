package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const zapRunsQuery = `query ZapRuns($accountId: ID!, $zapIds: [ID!], $status: [String!], $limit: Int, $offset: Int, $sortBy: String) {
  zapRuns(accountId: $accountId, zapIds: $zapIds, status: $status, limit: $limit, offset: $offset, sortBy: $sortBy) {
    edges { id startTime status zap { id title } }
    totalCount
  }
}`

const runDetailQuery = `query RunDetail($runId: ID!) {
  zapRun(id: $runId) {
    id status startTime
    zap { id title }
    steps { title app status input output error { title } }
  }
}`

type graphqlRequest struct {
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
	Query         string         `json:"query"`
}

type runSummary struct {
	ID        string `json:"id"`
	StartTime string `json:"start_time"`
	Status    string `json:"status"`
	ZapID     string `json:"zap_id"`
	ZapTitle  string `json:"zap_title"`
}

type runStep struct {
	Title  string         `json:"title"`
	App    string         `json:"app"`
	Status string         `json:"status"`
	Input  map[string]any `json:"input,omitempty"`
	Output map[string]any `json:"output,omitempty"`
	Error  *struct {
		Title string `json:"title"`
	} `json:"error,omitempty"`
}

type runDetail struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	StartTime string    `json:"start_time"`
	ZapID     string    `json:"zap_id"`
	ZapTitle  string    `json:"zap_title"`
	Steps     []runStep `json:"steps"`
}

func newRunsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "Read Zap run history and per-step detail (read-only)",
	}
	cmd.AddCommand(newRunsListCmd(flags))
	cmd.AddCommand(newRunsGetCmd(flags))
	return cmd
}

func validateReportingGraphQL(req graphqlRequest) error {
	switch req.OperationName {
	case "ZapRuns":
		if req.Query == zapRunsQuery {
			return nil
		}
	case "RunDetail":
		if req.Query == runDetailQuery {
			return nil
		}
	}
	return usageErr(fmt.Errorf("reporting GraphQL operation and query are not an allowed read-only pair"))
}

func callReportingGraphQL(cmd *cobra.Command, flags *rootFlags, req graphqlRequest) (json.RawMessage, error) {
	if err := validateReportingGraphQL(req); err != nil {
		return nil, err
	}
	client, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	raw, status, err := client.PostQueryWithParams(cmd.Context(), "/api/reporting/graphql", nil, req)
	if err != nil {
		return nil, apiErr(fmt.Errorf("calling zapier: %w", err))
	}
	if status >= 400 {
		return nil, apiErr(fmt.Errorf("zapier returned HTTP %d: %s", status, string(raw)))
	}
	var gqlErr struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &gqlErr); err == nil && len(gqlErr.Errors) > 0 {
		return nil, apiErr(fmt.Errorf("zapier graphql error: %s", gqlErr.Errors[0].Message))
	}
	return raw, nil
}

func currentAccountID(cmd *cobra.Command, flags *rootFlags) (string, error) {
	client, err := flags.newClient()
	if err != nil {
		return "", err
	}
	raw, err := client.GetNoCache(cmd.Context(), "/api/v4/session", nil)
	if err != nil {
		return "", apiErr(fmt.Errorf("resolving account: %w", err))
	}
	var session struct {
		CurrentAccountID json.RawMessage `json:"current_account_id"`
	}
	if err := json.Unmarshal(raw, &session); err != nil {
		return "", apiErr(fmt.Errorf("could not determine account id from session"))
	}
	var id uint64
	if err := json.Unmarshal(session.CurrentAccountID, &id); err != nil {
		var text string
		if json.Unmarshal(session.CurrentAccountID, &text) != nil {
			return "", apiErr(fmt.Errorf("could not determine account id from session"))
		}
		parsed, parseErr := strconv.ParseUint(strings.TrimSpace(text), 10, 64)
		if parseErr != nil {
			return "", apiErr(fmt.Errorf("could not determine account id from session"))
		}
		id = parsed
	}
	if id == 0 {
		return "", apiErr(fmt.Errorf("could not determine account id from session"))
	}
	return strconv.FormatUint(id, 10), nil
}

// pp:data-source live
func newRunsListCmd(flags *rootFlags) *cobra.Command {
	var zapID string
	var status string
	var limit int
	c := &cobra.Command{
		Use:   "list",
		Short: "List recent zap runs, optionally filtered by zap and status",
		Long:  "Lists Zap runs across your account (or one zap with --zap). Read-only: this only reads history, it never replays or cancels a run.",
		Example: strings.Trim(`
  zapier-pp-cli runs list
  zapier-pp-cli runs list --status error
  zapier-pp-cli runs list --zap 101 --status error
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
				return writeDryRun(cmd.OutOrStdout(), flags, "list runs")
			}
			accountID, err := currentAccountID(cmd, flags)
			if err != nil {
				return err
			}
			vars := map[string]any{
				"accountId": accountID,
				"limit":     limit,
				"offset":    0,
				"sortBy":    "-start_time",
			}
			if zapID != "" {
				vars["zapIds"] = []string{zapID}
			}
			if status != "" {
				vars["status"] = []string{status}
			}
			raw, err := callReportingGraphQL(cmd, flags, graphqlRequest{
				OperationName: "ZapRuns", Variables: vars, Query: zapRunsQuery,
			})
			if err != nil {
				return err
			}
			var parsed struct {
				Data struct {
					ZapRuns struct {
						Edges []struct {
							ID        string `json:"id"`
							StartTime string `json:"startTime"`
							Status    string `json:"status"`
							Zap       struct {
								ID    string `json:"id"`
								Title string `json:"title"`
							} `json:"zap"`
						} `json:"edges"`
						TotalCount int `json:"totalCount"`
					} `json:"zapRuns"`
				} `json:"data"`
			}
			if err := json.Unmarshal(raw, &parsed); err != nil {
				return apiErr(fmt.Errorf("parsing run list: %w", err))
			}
			results := make([]runSummary, 0, len(parsed.Data.ZapRuns.Edges))
			for _, e := range parsed.Data.ZapRuns.Edges {
				results = append(results, runSummary{
					ID: e.ID, StartTime: e.StartTime, Status: e.Status,
					ZapID: e.Zap.ID, ZapTitle: e.Zap.Title,
				})
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printLiveValue(cmd.OutOrStdout(), results, flags)
			}
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", r.ID, r.Status, r.StartTime, r.ZapTitle)
			}
			return nil
		},
	}
	c.Flags().StringVar(&zapID, "zap", "", "zap ID to filter runs to; see zaps list")
	c.Flags().StringVar(&status, "status", "", "filter by run status, e.g. success, error, held")
	c.Flags().IntVar(&limit, "limit", 25, "maximum runs to return")
	return c
}

// pp:data-source live
func newRunsGetCmd(flags *rootFlags) *cobra.Command {
	c := &cobra.Command{
		Use:   "get <run-id>",
		Short: "Show full detail for one run, including every step's data and error",
		Long:  "Shows a single run's steps with input/output data and the failing step's error message, if any. Read-only: never replays the run.",
		Example: strings.Trim(`
  zapier-pp-cli runs get <run-id>
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "get run detail")
			}
			runID := args[0]
			raw, err := callReportingGraphQL(cmd, flags, graphqlRequest{
				OperationName: "RunDetail",
				Variables:     map[string]any{"runId": runID},
				Query:         runDetailQuery,
			})
			if err != nil {
				return err
			}
			var parsed struct {
				Data struct {
					ZapRun *struct {
						ID        string `json:"id"`
						Status    string `json:"status"`
						StartTime string `json:"startTime"`
						Zap       struct {
							ID    string `json:"id"`
							Title string `json:"title"`
						} `json:"zap"`
						Steps []runStep `json:"steps"`
					} `json:"zapRun"`
				} `json:"data"`
			}
			if err := json.Unmarshal(raw, &parsed); err != nil {
				return apiErr(fmt.Errorf("parsing run detail: %w", err))
			}
			if parsed.Data.ZapRun == nil {
				return notFoundErr(fmt.Errorf("run %s not found", runID))
			}
			z := parsed.Data.ZapRun
			detail := runDetail{
				ID: z.ID, Status: z.Status, StartTime: z.StartTime,
				ZapID: z.Zap.ID, ZapTitle: z.Zap.Title, Steps: z.Steps,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printLiveValue(cmd.OutOrStdout(), detail, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Run %s (%s) — %s\n", detail.ID, detail.Status, detail.ZapTitle)
			for i, s := range detail.Steps {
				errMsg := ""
				if s.Error != nil {
					errMsg = " ERROR: " + s.Error.Title
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %d. [%s] %s%s\n", i+1, s.Status, s.Title, errMsg)
			}
			return nil
		},
	}
	return c
}
