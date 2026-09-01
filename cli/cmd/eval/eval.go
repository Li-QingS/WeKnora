// Package evalcmd holds the `weknora eval` command tree: the reproducible
// baseline runner used by WP2.
package evalcmd

import (
	"github.com/spf13/cobra"

	"github.com/Tencent/WeKnora/cli/internal/cmdutil"
)

// NewCmd builds the `weknora eval` parent command.
func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Run reproducible evaluations and baselines",
		Long: `Run evaluation baselines driven by a YAML config. The runner reads the
config, starts the evaluation through the server API, polls until it is
terminal, and writes evaluation-result.json and evaluation-report.md.`,
	}
	cmd.AddCommand(NewCmdRun(f))
	cmd.AddCommand(NewCmdCompare())
	cmd.AddCommand(NewCmdBaseline())
	return cmd
}
