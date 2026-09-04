package cli

import "github.com/spf13/cobra"

func init() {
	whichIndex = []whichEntry{
		{Command: "zaps list", Description: "List and search the Zaps available to the connected account", Group: "Zap inspection", WhyItMatters: "Find a Zap before inspecting its history or diagnosing a failure."},
		{Command: "runs list", Description: "List Zap run history, including failed runs", Group: "Run history", WhyItMatters: "Review recent activity or narrow investigation to failures."},
		{Command: "runs get", Description: "Get the detail for one Zap run", Group: "Run history", WhyItMatters: "Inspect a known run after finding it in run history."},
		{Command: "diagnose", Description: "Diagnose failed steps for a Zap", Group: "Failure diagnosis", WhyItMatters: "Explain which step failed and why without changing the Zap."},
	}
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newZapsCmd(flags))
		addNovelCommandIfAbsent(root, newRunsCmd(flags))
		addNovelCommandIfAbsent(root, newDiagnoseCmd(flags))
	})
}
