package evalcmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Tencent/WeKnora/cli/internal/cmdutil"
	"github.com/Tencent/WeKnora/cli/internal/evalrunner"
	"github.com/Tencent/WeKnora/cli/internal/iostreams"
)

// NewCmdBaseline builds the `weknora eval baseline` parent command.
func NewCmdBaseline() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "baseline",
		Short: "Manage evaluation baselines",
		Long:  "Create and inspect evaluation baseline YAML files.",
	}
	cmd.AddCommand(NewCmdBaselineCreate())
	return cmd
}

// BaselineCreateOptions captures `baseline create` flag state.
type BaselineCreateOptions struct {
	Result         string
	Output         string
	ApprovedBy     string
	ApprovedCommit string
	Note           string
	Force          bool
	DryRun         bool
}

// NewCmdBaselineCreate builds `weknora eval baseline create`.
func NewCmdBaselineCreate() *cobra.Command {
	opts := &BaselineCreateOptions{}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a baseline YAML from a trusted evaluation result",
		Long: `Create a baseline YAML from a trusted evaluation-result.json. Approval
metadata is required so baseline changes stay auditable.

The generated baseline records current metric values but leaves threshold
rules empty; edit the YAML to add min_value / max_absolute_drop /
max_relative_drop before using it as a gate.`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			fopts, err := cmdutil.CheckFormatFlag(c)
			if err != nil {
				return err
			}
			fopts.ResolveDefault(iostreams.IO.IsStdoutTTY())
			if strings.TrimSpace(opts.Result) == "" ||
				strings.TrimSpace(opts.Output) == "" ||
				strings.TrimSpace(opts.ApprovedBy) == "" ||
				strings.TrimSpace(opts.ApprovedCommit) == "" {
				return cmdutil.NewError(cmdutil.CodeEvalConfigError,
					"--result, --output, --approved-by and --approved-commit are required")
			}
			if handled, err := cmdutil.HandleDryRun(c, opts.DryRun, cmdutil.DryRunPlan{
				Action: "eval.baseline.create",
				Args: map[string]any{
					"result":          opts.Result,
					"output":          opts.Output,
					"approved_by":     opts.ApprovedBy,
					"approved_commit": opts.ApprovedCommit,
				},
			}); handled {
				return err
			}

			result, err := evalrunner.LoadResult(opts.Result)
			if err != nil {
				return mapEvalError(err)
			}
			baseline, err := evalrunner.GenerateBaseline(result, evalrunner.BaselineGenOptions{
				ApprovedBy:     opts.ApprovedBy,
				ApprovedCommit: opts.ApprovedCommit,
				Note:           opts.Note,
			})
			if err != nil {
				return mapEvalError(err)
			}
			if err := evalrunner.WriteBaseline(opts.Output, baseline, opts.Force); err != nil {
				return mapEvalError(err)
			}

			data := map[string]any{
				"path":             opts.Output,
				"config_hash":      baseline.ConfigHash,
				"dataset_sha256":   baseline.Dataset.SHA256,
				"approved_by":      baseline.Metadata.ApprovedBy,
				"approved_commit":  baseline.Metadata.ApprovedCommit,
				"thresholds_empty": true,
			}
			if fopts.WantsJSON() {
				return fopts.Emit(iostreams.IO.Out, data, nil)
			}
			fmt.Fprintf(iostreams.IO.Out, "wrote baseline: %s\n", opts.Output)
			fmt.Fprintf(iostreams.IO.Out, "config_hash: %s\n", baseline.ConfigHash)
			fmt.Fprintln(iostreams.IO.Out, "threshold rules are empty; edit the YAML to add min_value / max_absolute_drop / max_relative_drop")
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Result, "result", "", "Path to evaluation-result.json")
	cmd.Flags().StringVar(&opts.Output, "output", "", "Path to write the baseline YAML")
	cmd.Flags().StringVar(&opts.ApprovedBy, "approved-by", "", "Reviewer/author approving the baseline")
	cmd.Flags().StringVar(&opts.ApprovedCommit, "approved-commit", "", "Commit SHA the baseline was approved at")
	cmd.Flags().StringVar(&opts.Note, "note", "", "Optional note explaining the baseline")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "Overwrite an existing baseline file")
	cmdutil.AddFormatFlag(cmd, "path", "config_hash", "dataset_sha256", "approved_by", "approved_commit")
	cmdutil.AddDryRunFlag(cmd, &opts.DryRun)
	cmdutil.SetAgentHelp(cmd, cmdutil.AgentHelp{
		UsedFor:       "Create an auditable baseline YAML from a trusted evaluation result",
		RequiredFlags: []string{"--result", "--output", "--approved-by", "--approved-commit"},
		Examples: []string{
			"weknora eval baseline create --result artifacts/evaluation/evaluation-result.json --output evaluation/baselines/demo.yaml --approved-by lqs --approved-commit 657466d8",
		},
		Output: "envelope.data is {path, config_hash, dataset_sha256, approved_by, approved_commit, thresholds_empty}",
		Warnings: []string{
			"generated thresholds are empty; edit the YAML before using it as a gate",
			"existing files are only overwritten with --force",
		},
	})
	return cmd
}
