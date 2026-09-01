package evalcmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/Tencent/WeKnora/cli/internal/cmdutil"
	"github.com/Tencent/WeKnora/cli/internal/evalrunner"
	"github.com/Tencent/WeKnora/cli/internal/iostreams"
)

// CompareOptions captures `eval compare` flag state.
type CompareOptions struct {
	Result   string
	Baseline string
}

// NewCmdCompare builds `weknora eval compare`.
func NewCmdCompare() *cobra.Command {
	opts := &CompareOptions{}
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare an evaluation result against a baseline",
		Long: `Compare an evaluation-result.json against a baseline YAML. The result and
baseline must share the same config_hash and dataset.sha256.

Exit codes:
  0    all metrics pass
  2    one or more metrics regressed
  3    baseline mismatch, missing result, or invalid baseline`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			fopts, err := cmdutil.CheckFormatFlag(c)
			if err != nil {
				return err
			}
			fopts.ResolveDefault(iostreams.IO.IsStdoutTTY())
			if strings.TrimSpace(opts.Result) == "" || strings.TrimSpace(opts.Baseline) == "" {
				return cmdutil.NewError(cmdutil.CodeEvalConfigError, "--result and --baseline are required")
			}

			result, err := evalrunner.LoadResult(opts.Result)
			if err != nil {
				return mapEvalError(err)
			}
			baseline, err := evalrunner.LoadBaseline(opts.Baseline)
			if err != nil {
				return mapEvalError(err)
			}
			comparison, err := evalrunner.CompareResult(result, baseline)
			if err != nil {
				return mapEvalError(err)
			}

			if err := emitComparison(fopts, comparison); err != nil {
				return err
			}
			if !comparison.Pass {
				return cmdutil.NewError(
					cmdutil.CodeEvalRegression,
					fmt.Sprintf("%d metric(s) regressed", comparison.FailedCount),
				).WithSilent()
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Result, "result", "", "Path to evaluation-result.json")
	cmd.Flags().StringVar(&opts.Baseline, "baseline", "", "Path to baseline YAML")
	cmdutil.AddFormatFlag(cmd, "pass", "config_hash", "dataset_sha256", "failed_count", "items")
	cmdutil.SetAgentHelp(cmd, cmdutil.AgentHelp{
		UsedFor:       "Compare an evaluation result against a committed baseline and report per-metric pass/fail with delta",
		RequiredFlags: []string{"--result", "--baseline"},
		Examples: []string{
			"weknora eval compare --result artifacts/evaluation/evaluation-result.json --baseline evaluation/baselines/demo.yaml",
		},
		Output: "envelope.data is {pass, config_hash, dataset_sha256, failed_count, items:[{group,name,baseline,current,delta,pass,reason}]}",
		Warnings: []string{
			"exit code 2 means metrics regressed; the comparison envelope is still written to stdout",
			"config_hash / dataset.sha256 mismatches are rejected with exit code 3",
		},
	})
	return cmd
}

func emitComparison(fopts *cmdutil.FormatOptions, comparison *evalrunner.Comparison) error {
	if fopts.WantsJSON() {
		return fopts.Emit(iostreams.IO.Out, comparison, nil)
	}
	tw := tabwriter.NewWriter(iostreams.IO.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "GROUP\tMETRIC\tBASELINE\tCURRENT\tDELTA\tPASS")
	for _, item := range comparison.Items {
		pass := "PASS"
		if !item.Pass {
			pass = "FAIL"
		}
		fmt.Fprintf(tw, "%s\t%s\t%.4f\t%.4f\t%.4f\t%s\n",
			item.Group, item.Name, item.Baseline, item.Current, item.Delta, pass)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if comparison.FailedCount > 0 {
		fmt.Fprintf(iostreams.IO.Out, "\n%d metric(s) regressed\n", comparison.FailedCount)
	}
	return nil
}
