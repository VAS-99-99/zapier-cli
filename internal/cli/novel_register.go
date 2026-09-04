package cli

import "github.com/spf13/cobra"

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newZapsCmd(flags))
		addNovelCommandIfAbsent(root, newRunsCmd(flags))
		addNovelCommandIfAbsent(root, newDiagnoseCmd(flags))
	})
}
