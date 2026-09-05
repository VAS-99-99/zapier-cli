package cli

import (
	"bytes"
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

const (
	maxRunsPageSize             = 100
	maxReportingPaginationPages = 10000
)

type reportingRunEdge struct {
	ID        string `json:"id"`
	StartTime string `json:"startTime"`
	Status    string `json:"status"`
	Zap       *struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"zap"`
}

type reportingRunsPage struct {
	Edges      []reportingRunEdge
	TotalCount int
}

// parseReportingRunDetail rejects partial GraphQL data rather than presenting
// a missing run or missing steps as a successful empty detail. A null zapRun
// is the one expected absence and is returned as found=false.
func parseReportingRunDetail(raw json.RawMessage) (detail runDetail, found bool, err error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return runDetail{}, false, err
	}
	data, ok := root["data"]
	if !ok || isNullJSON(data) {
		return runDetail{}, false, fmt.Errorf("reporting response has no data")
	}
	var dataObject map[string]json.RawMessage
	if err := json.Unmarshal(data, &dataObject); err != nil {
		return runDetail{}, false, fmt.Errorf("reporting response data is not an object")
	}
	zapRun, ok := dataObject["zapRun"]
	if !ok {
		return runDetail{}, false, fmt.Errorf("reporting response has no zapRun")
	}
	if isNullJSON(zapRun) {
		return runDetail{}, false, nil
	}
	var runObject map[string]json.RawMessage
	if err := json.Unmarshal(zapRun, &runObject); err != nil {
		return runDetail{}, false, fmt.Errorf("reporting response zapRun is not an object")
	}
	if detail.ID, err = requiredReportingString(runObject, "id"); err != nil {
		return runDetail{}, false, err
	}
	if detail.Status, err = requiredReportingString(runObject, "status"); err != nil {
		return runDetail{}, false, err
	}
	if startTime, ok := runObject["startTime"]; ok && !isNullJSON(startTime) {
		if err := json.Unmarshal(startTime, &detail.StartTime); err != nil {
			return runDetail{}, false, fmt.Errorf("reporting response startTime is invalid")
		}
	}
	steps, ok := runObject["steps"]
	if !ok || isNullJSON(steps) {
		return runDetail{}, false, fmt.Errorf("reporting response has no steps")
	}
	var stepValues []json.RawMessage
	if err := json.Unmarshal(steps, &stepValues); err != nil {
		return runDetail{}, false, fmt.Errorf("reporting response steps are invalid: %w", err)
	}
	detail.Steps = make([]runStep, 0, len(stepValues))
	for i, stepValue := range stepValues {
		if isNullJSON(stepValue) {
			return runDetail{}, false, fmt.Errorf("reporting response step %d is null", i)
		}
		var stepObject map[string]json.RawMessage
		if err := json.Unmarshal(stepValue, &stepObject); err != nil {
			return runDetail{}, false, fmt.Errorf("reporting response step %d is invalid", i)
		}
		if _, err := requiredReportingString(stepObject, "title"); err != nil {
			return runDetail{}, false, fmt.Errorf("reporting response step %d: %w", i, err)
		}
		if _, err := requiredReportingString(stepObject, "status"); err != nil {
			return runDetail{}, false, fmt.Errorf("reporting response step %d: %w", i, err)
		}
		var step runStep
		if err := json.Unmarshal(stepValue, &step); err != nil {
			return runDetail{}, false, fmt.Errorf("reporting response step %d is invalid: %w", i, err)
		}
		detail.Steps = append(detail.Steps, step)
	}
	if zap, ok := runObject["zap"]; ok && !isNullJSON(zap) {
		var zapObject struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		}
		if err := json.Unmarshal(zap, &zapObject); err != nil {
			return runDetail{}, false, fmt.Errorf("reporting response zap is invalid")
		}
		detail.ZapID, detail.ZapTitle = zapObject.ID, zapObject.Title
	}
	return detail, true, nil
}

func requiredReportingString(object map[string]json.RawMessage, field string) (string, error) {
	raw, ok := object[field]
	if !ok || isNullJSON(raw) {
		return "", fmt.Errorf("reporting response is missing %s", field)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("reporting response has invalid %s", field)
	}
	return value, nil
}

// parseReportingRunsPage is deliberately strict: GraphQL's nullable response
// branches must not be mistaken for an empty history page.
func parseReportingRunsPage(raw json.RawMessage) (reportingRunsPage, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return reportingRunsPage{}, err
	}
	data, ok := root["data"]
	if !ok || isNullJSON(data) {
		return reportingRunsPage{}, fmt.Errorf("reporting response has no data")
	}
	var dataObject map[string]json.RawMessage
	if err := json.Unmarshal(data, &dataObject); err != nil {
		return reportingRunsPage{}, fmt.Errorf("reporting response data is not an object")
	}
	zapRuns, ok := dataObject["zapRuns"]
	if !ok || isNullJSON(zapRuns) {
		return reportingRunsPage{}, fmt.Errorf("reporting response has no zapRuns")
	}
	var runsObject map[string]json.RawMessage
	if err := json.Unmarshal(zapRuns, &runsObject); err != nil {
		return reportingRunsPage{}, fmt.Errorf("reporting response zapRuns is not an object")
	}
	edges, ok := runsObject["edges"]
	if !ok || isNullJSON(edges) {
		return reportingRunsPage{}, fmt.Errorf("reporting response has no edges")
	}
	total, ok := runsObject["totalCount"]
	if !ok || isNullJSON(total) {
		return reportingRunsPage{}, fmt.Errorf("reporting response has no totalCount")
	}
	var page reportingRunsPage
	if err := json.Unmarshal(edges, &page.Edges); err != nil {
		return reportingRunsPage{}, fmt.Errorf("reporting response edges are invalid: %w", err)
	}
	if err := json.Unmarshal(total, &page.TotalCount); err != nil || page.TotalCount < 0 {
		return reportingRunsPage{}, fmt.Errorf("reporting response totalCount is invalid")
	}
	if page.TotalCount < len(page.Edges) {
		return reportingRunsPage{}, fmt.Errorf("reporting response totalCount %d is smaller than returned page size %d", page.TotalCount, len(page.Edges))
	}
	for i, edge := range page.Edges {
		if strings.TrimSpace(edge.ID) == "" || strings.TrimSpace(edge.Status) == "" {
			return reportingRunsPage{}, fmt.Errorf("reporting response edge %d is missing id or status", i)
		}
	}
	return page, nil
}

func isNullJSON(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
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
	var offset int
	var all bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List recent zap runs, optionally filtered by zap and status",
		Long:  "Lists Zap runs across your account (or one zap with --zap). Read-only: this only reads history, it never replays or cancels a run.",
		Example: strings.Trim(`
  zapier-pp-cli runs list
  zapier-pp-cli runs list --status error
  zapier-pp-cli runs list --zap 101 --status error
  zapier-pp-cli runs list --offset 25
  zapier-pp-cli runs list --all --limit 100
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if limit < 1 || limit > maxRunsPageSize {
				return usageErr(fmt.Errorf("--limit must be between 1 and %d", maxRunsPageSize))
			}
			if offset < 0 {
				return usageErr(fmt.Errorf("--offset must be at least 0"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "list runs")
			}
			accountID, err := currentAccountID(cmd, flags)
			if err != nil {
				return err
			}
			results := make([]runSummary, 0, limit)
			currentOffset := offset
			totalCount := 0
			seenRunIDs := make(map[string]struct{})
			for pageNumber := 0; ; pageNumber++ {
				if pageNumber >= maxReportingPaginationPages {
					return apiErr(fmt.Errorf("parsing run list: history pagination exceeded %d pages; retry with a narrower window", maxReportingPaginationPages))
				}
				vars := map[string]any{"accountId": accountID, "limit": limit, "offset": currentOffset, "sortBy": "-start_time"}
				if zapID != "" {
					vars["zapIds"] = []string{zapID}
				}
				if status != "" {
					vars["status"] = []string{status}
				}
				raw, err := callReportingGraphQL(cmd, flags, graphqlRequest{OperationName: "ZapRuns", Variables: vars, Query: zapRunsQuery})
				if err != nil {
					return err
				}
				page, err := parseReportingRunsPage(raw)
				if err != nil {
					return apiErr(fmt.Errorf("parsing run list: %w", err))
				}
				totalCount = page.TotalCount
				if currentOffset+len(page.Edges) < page.TotalCount && len(page.Edges) == 0 {
					return apiErr(fmt.Errorf("parsing run list: reporting page made no progress at offset %d", currentOffset))
				}
				for _, e := range page.Edges {
					if _, duplicate := seenRunIDs[e.ID]; duplicate {
						return apiErr(fmt.Errorf("parsing run list: history changed while paging (run %s repeated at offset %d); retry", e.ID, currentOffset))
					}
					seenRunIDs[e.ID] = struct{}{}
					r := runSummary{ID: e.ID, StartTime: e.StartTime, Status: e.Status}
					if e.Zap != nil {
						r.ZapID, r.ZapTitle = e.Zap.ID, e.Zap.Title
					}
					results = append(results, r)
				}
				hasMore := currentOffset+len(page.Edges) < page.TotalCount
				if !all || !hasMore {
					break
				}
				currentOffset += len(page.Edges)
			}
			hasMore := offset+len(results) < totalCount
			var nextOffset *int
			if hasMore {
				n := offset + len(results)
				nextOffset = &n
			}
			pagination := map[string]any{"offset": offset, "returned": len(results), "total_count": totalCount, "has_more": hasMore, "next_offset": nextOffset}
			if !all && hasMore {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: results truncated; more run history is available. Re-run with --all to fetch every page.")
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				raw, err := json.Marshal(results)
				if err != nil {
					return err
				}
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, map[string]any{"source": "live", "pagination": pagination})
			}
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", r.ID, r.Status, r.StartTime, r.ZapTitle)
			}
			return nil
		},
	}
	c.Flags().StringVar(&zapID, "zap", "", "zap ID to filter runs to; see zaps list")
	c.Flags().StringVar(&status, "status", "", "filter by run status, e.g. success, error, held")
	c.Flags().IntVar(&limit, "limit", 25, fmt.Sprintf("runs per page (1-%d)", maxRunsPageSize))
	c.Flags().IntVar(&offset, "offset", 0, "number of matching runs to skip")
	c.Flags().BoolVar(&all, "all", false, "fetch every page after --offset")
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
			detail, found, err := parseReportingRunDetail(raw)
			if err != nil {
				return apiErr(fmt.Errorf("parsing run detail: %w", err))
			}
			if !found {
				return notFoundErr(fmt.Errorf("run %s not found", runID))
			}
			if detail.ID != runID {
				return apiErr(fmt.Errorf("parsing run detail: reporting response returned run %s for requested run %s", detail.ID, runID))
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
