package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/zapier/internal/cliutil"
	"github.com/spf13/cobra"
)

type diagnoseFailure struct {
	RunID      string `json:"run_id"`
	StartTime  string `json:"start_time"`
	Step       int    `json:"step"`
	StepTitle  string `json:"step_title"`
	App        string `json:"app"`
	NodeAction string `json:"node_action,omitempty"`
	Error      string `json:"error"`
}

type diagnoseResult struct {
	ZapID    string            `json:"zap_id"`
	ZapTitle string            `json:"zap_title"`
	Checked  int               `json:"runs_checked"`
	Failures []diagnoseFailure `json:"failures"`
	Note     string            `json:"note,omitempty"`
}

func normalizedStepTitle(title string) string {
	return strings.ToLower(strings.Join(strings.Fields(title), " "))
}

// failuresFromSteps enriches only a unique normalized title match. Run step
// order is not assumed to match Zap node order because branches can reorder it.
func failuresFromSteps(runID, startTime string, steps []runStep, nodes []zapNode) []diagnoseFailure {
	byTitle := make(map[string][]zapNode, len(nodes))
	for _, node := range nodes {
		key := normalizedStepTitle(node.Title)
		if key != "" {
			byTitle[key] = append(byTitle[key], node)
		}
	}
	failures := make([]diagnoseFailure, 0, len(steps))
	for i, s := range steps {
		errMsg := ""
		if s.Error != nil {
			errMsg = s.Error.Title
		}
		if !strings.EqualFold(s.Status, "error") && errMsg == "" {
			continue
		}
		f := diagnoseFailure{
			RunID: runID, StartTime: startTime, Step: i + 1,
			StepTitle: s.Title, App: s.App, Error: errMsg,
		}
		if matches := byTitle[normalizedStepTitle(s.Title)]; len(matches) == 1 {
			f.NodeAction = matches[0].Action
		}
		failures = append(failures, f)
	}
	return failures
}

// failedRunIDs lists the zap's recent error runs through the reporting GraphQL
// endpoint. Read-only: the POST carries a query, never a mutation.
func failedRunIDs(cmd *cobra.Command, flags *rootFlags, zapID string, limit int, includeHistorical bool) ([]string, error) {
	accountID, err := currentAccountID(cmd, flags)
	if err != nil {
		return nil, err
	}
	pageSize := limit
	if includeHistorical {
		pageSize = 100
	}
	ids := make([]string, 0, limit)
	seenRunIDs := make(map[string]struct{})
	for offset, pageNumber := 0, 0; ; pageNumber++ {
		if pageNumber >= maxReportingPaginationPages {
			return nil, apiErr(fmt.Errorf("parsing error runs: history pagination exceeded %d pages; retry with a narrower window", maxReportingPaginationPages))
		}
		variables := map[string]any{
			"accountId": accountID,
			"status":    []string{"error"},
			"limit":     pageSize,
			"offset":    offset,
			"sortBy":    "-start_time",
		}
		if !includeHistorical {
			variables["zapIds"] = []string{zapID}
		}
		raw, err := callReportingGraphQL(cmd, flags, graphqlRequest{
			OperationName: "ZapRuns", Variables: variables, Query: zapRunsQuery,
		})
		if err != nil {
			return nil, err
		}
		page, err := parseReportingRunsPage(raw)
		if err != nil {
			return nil, apiErr(fmt.Errorf("parsing error runs: %w", err))
		}
		edges := page.Edges
		if offset+len(edges) < page.TotalCount && len(edges) == 0 {
			return nil, apiErr(fmt.Errorf("parsing error runs: reporting page made no progress at offset %d", offset))
		}
		for _, edge := range edges {
			if _, duplicate := seenRunIDs[edge.ID]; duplicate {
				return nil, apiErr(fmt.Errorf("parsing error runs: history changed while paging (run %s repeated at offset %d); retry", edge.ID, offset))
			}
			seenRunIDs[edge.ID] = struct{}{}
			if !includeHistorical || (edge.Zap != nil && edge.Zap.ID == zapID) {
				ids = append(ids, edge.ID)
				if len(ids) == limit {
					return ids, nil
				}
			}
		}
		if len(edges) == 0 || offset+len(edges) >= page.TotalCount {
			return ids, nil
		}
		offset += len(edges)
	}
}

// runFailures opens one run and returns its broken steps.
func runFailures(cmd *cobra.Command, flags *rootFlags, runID string, nodes []zapNode) ([]diagnoseFailure, error) {
	raw, err := callReportingGraphQL(cmd, flags, graphqlRequest{
		OperationName: "RunDetail",
		Variables:     map[string]any{"runId": runID},
		Query:         runDetailQuery,
	})
	if err != nil {
		return nil, apiErr(fmt.Errorf("reading run %s detail: %w", runID, err))
	}
	detail, found, err := parseReportingRunDetail(raw)
	if err != nil {
		return nil, apiErr(fmt.Errorf("parsing run %s detail: %w", runID, err))
	}
	if !found {
		return nil, notFoundErr(fmt.Errorf("run %s not found", runID))
	}
	if detail.ID != runID {
		return nil, apiErr(fmt.Errorf("parsing run %s detail: reporting response returned run %s", runID, detail.ID))
	}
	failures := failuresFromSteps(runID, detail.StartTime, detail.Steps, nodes)
	if len(failures) == 0 {
		return nil, apiErr(fmt.Errorf("run %s detail contains no errored step", runID))
	}
	return failures, nil
}

// writeZapMatches reports an ambiguous name rather than guessing which zap the
// caller meant.
func writeZapMatches(cmd *cobra.Command, flags *rootFlags, needle string, matches []zapSummary) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printLiveValue(cmd.OutOrStdout(), map[string]any{
			"note":    fmt.Sprintf("%q matches %d zaps; re-run with one id", needle, len(matches)),
			"matches": compactZaps(matches),
		}, flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%q matches %d zaps. Re-run with one id:\n", needle, len(matches))
	for _, z := range matches {
		fmt.Fprintf(cmd.OutOrStdout(), "  %d\t%s\t%s\n", z.ID, z.Title, z.State)
	}
	return nil
}

func writeDiagnoseHuman(cmd *cobra.Command, result diagnoseResult) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s (zap %s)\n", result.ZapTitle, result.ZapID)
	if result.Note != "" {
		fmt.Fprintln(out, result.Note)
		return nil
	}
	for _, f := range result.Failures {
		fmt.Fprintf(out, "  run %s @ %s: step %d %q failed in %s\n",
			f.RunID, f.StartTime, f.Step, f.StepTitle, f.App)
		if f.NodeAction != "" {
			fmt.Fprintf(out, "    action: %s\n", f.NodeAction)
		}
		if f.Error != "" {
			fmt.Fprintf(out, "    error: %s\n", f.Error)
		}
	}
	return nil
}

// pp:client-call
// runDiagnose resolves the zap, pulls its failed runs, and reports the broken
// step in each. Read-only end to end.
func runDiagnose(cmd *cobra.Command, flags *rootFlags, needle string, limit int) error {
	zaps, err := fetchZaps(cmd, flags, 0, zapMatcher(needle), true)
	if err != nil {
		return err
	}
	var historicalID int64
	if parsed, parseErr := strconv.ParseInt(strings.TrimSpace(needle), 10, 64); parseErr == nil && parsed > 0 {
		historicalID = parsed
	}
	historical := false
	switch {
	case len(zaps) == 0 && historicalID > 0:
		// A run can outlive its Zap. Numeric IDs remain useful for historical
		// diagnosis even when the Zap no longer appears in current inventory;
		// run detail still supplies the authoritative step title, app, and error.
		zaps = []zapSummary{{ID: historicalID, Title: "Historical Zap " + needle}}
		historical = true
	case len(zaps) == 0:
		return notFoundErr(fmt.Errorf("no zap matching %q found in your account", needle))
	case len(zaps) > 1:
		return writeZapMatches(cmd, flags, needle, zaps)
	}
	match := zaps[0]
	zapID := strconv.FormatInt(match.ID, 10)

	runIDs, err := failedRunIDs(cmd, flags, zapID, limit, historical)
	if err != nil {
		return err
	}
	if historical && len(runIDs) == 0 {
		return notFoundErr(fmt.Errorf("no failed run history found for zap id %s", zapID))
	}
	result := diagnoseResult{
		ZapID: zapID, ZapTitle: match.Title,
		Failures: []diagnoseFailure{},
	}
	if len(runIDs) == 0 {
		result.Note = "no failed runs found in the checked window"
	}
	for _, runID := range runIDs {
		failures, err := runFailures(cmd, flags, runID, match.Nodes)
		if err != nil {
			return err
		}
		result.Checked++
		result.Failures = append(result.Failures, failures...)
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printLiveValue(cmd.OutOrStdout(), result, flags)
	}
	return writeDiagnoseHuman(cmd, result)
}

func diagnoseAnnotations() map[string]string {
	annotations := map[string]string{"mcp:read-only": "true"}
	// The generic live-dogfood runner cannot discover a positional fixture for
	// a top-level command. Accept a fresh, positive Zap ID only during an
	// explicitly configured verification run; the value is never persisted in
	// source, help output, or normal command behavior.
	if raw := strings.TrimSpace(os.Getenv("ZAPIER_DOGFOOD_ZAP_ID")); cliutil.IsDogfoodEnv() && raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			annotations["pp:happy-args"] = "zap-id=" + raw
		}
	}
	return annotations
}

// pp:data-source live
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
  zapier-pp-cli diagnose <zap-id> --limit 10
`, "\n"),
		Annotations: diagnoseAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if limit < 1 {
				return usageErr(fmt.Errorf("--limit must be at least 1"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "diagnose zap")
			}
			return runDiagnose(cmd, flags, args[0], limit)
		},
	}
	c.Flags().IntVar(&limit, "limit", 10, "maximum failed runs to inspect")
	return c
}
