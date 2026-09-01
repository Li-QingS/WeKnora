package evalcmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Tencent/WeKnora/cli/internal/cmdutil"
	"github.com/Tencent/WeKnora/cli/internal/evalrunner"
	"github.com/Tencent/WeKnora/cli/internal/iostreams"
)

// RunOptions captures `eval run` flag state.
type RunOptions struct {
	Config    string
	TaskID    string
	Wait      bool
	NoWait    bool
	Timeout   time.Duration
	Interval  time.Duration
	ReportDir string
	DryRun    bool
}

type runOutput struct {
	TaskID      string   `json:"task_id"`
	ConfigHash  string   `json:"config_hash,omitempty"`
	Status      string   `json:"status,omitempty"`
	ReportPaths []string `json:"report_paths,omitempty"`
	Reproduce   string   `json:"reproduce,omitempty"`
}

// NewCmdRun builds `weknora eval run`.
func NewCmdRun(f *cmdutil.Factory) *cobra.Command {
	opts := &RunOptions{}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run one evaluation from a YAML config",
		Long: `Run a reproducible evaluation baseline from a YAML config and write
evaluation-result.json plus evaluation-report.md.

Exit codes (WP2 runner contract):
  0    success
  2    regression (reserved for WP3)
  3    config or dataset error
  4    service or model unavailable
  5    run failed or wait timed out
  130  interrupted (Ctrl-C / SIGTERM)`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			fopts, err := cmdutil.CheckFormatFlag(c)
			if err != nil {
				return err
			}
			fopts.ResolveDefault(iostreams.IO.IsStdoutTTY())

			if strings.TrimSpace(opts.Config) == "" && strings.TrimSpace(opts.TaskID) == "" {
				return cmdutil.NewError(cmdutil.CodeEvalConfigError, "--config is required unless --task-id is provided")
			}
			if handled, err := cmdutil.HandleDryRun(c, opts.DryRun, cmdutil.DryRunPlan{
				Action: "eval.run",
				Args: map[string]any{
					"config":     opts.Config,
					"report_dir": opts.ReportDir,
				},
			}); handled {
				return err
			}

			var cfg *evalrunner.RunnerConfig
			if strings.TrimSpace(opts.Config) != "" {
				cfg, err = evalrunner.LoadConfig(opts.Config)
				if err != nil {
					return mapEvalError(err)
				}
			}
			runOpts := buildRunOptions(c, opts, cfg, strings.TrimSpace(opts.TaskID))
			cli, err := f.Client()
			if err != nil {
				return cmdutil.NewError(cmdutil.CodeEvalServiceUnavailable, err.Error())
			}

			result, runErr := evalrunner.Run(c.Context(), cfg, cli, runOpts)
			if runErr != nil {
				return mapEvalError(runErr)
			}

			data := runOutput{TaskID: result.TaskID}
			if result.Report != nil {
				data.ConfigHash = result.Report.ConfigHash
				data.Status = result.Report.StatusLabel
				data.ReportPaths = result.ReportPaths
				data.Reproduce = result.Report.Reproduce
			}
			if fopts.WantsJSON() {
				return fopts.Emit(iostreams.IO.Out, data, nil)
			}
			fmt.Fprintf(iostreams.IO.Out, "run_id: %s\n", result.TaskID)
			if data.ConfigHash != "" {
				fmt.Fprintf(iostreams.IO.Out, "config_hash: %s\n", data.ConfigHash)
			}
			for _, path := range result.ReportPaths {
				fmt.Fprintf(iostreams.IO.Out, "report: %s\n", path)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Config, "config", "", "Path to the evaluation YAML config")
	cmd.Flags().StringVar(&opts.TaskID, "task-id", "", "Poll and report an existing evaluation task id")
	cmd.Flags().BoolVar(&opts.Wait, "wait", true, "Wait for the run to reach a terminal state")
	cmd.Flags().BoolVar(&opts.NoWait, "no-wait", false, "Start the run and print its task id without waiting")
	cmd.Flags().DurationVar(&opts.Timeout, "timeout", 30*time.Minute, "Max wait time")
	cmd.Flags().DurationVar(&opts.Interval, "interval", 2*time.Second, "Poll interval")
	cmd.Flags().StringVar(&opts.ReportDir, "report-dir", "artifacts/evaluation", "Report output directory")
	cmdutil.AddFormatFlag(cmd, "task_id", "config_hash", "status", "report_paths")
	cmdutil.AddDryRunFlag(cmd, &opts.DryRun)
	cmdutil.SetAgentHelp(cmd, cmdutil.AgentHelp{
		UsedFor:       "Run a reproducible evaluation baseline from a YAML config, poll until terminal, and write JSON + Markdown reports",
		RequiredFlags: []string{"--config or --task-id"},
		Examples: []string{
			"weknora eval run --config evaluation/configs/default.yaml",
			"weknora eval run --config eval.yaml --no-wait",
			"weknora eval run --task-id evaluation_1_123_demo",
		},
		Output: "envelope.data is {task_id, config_hash?, status?, report_paths?}",
		Warnings: []string{
			"exit codes follow the WP2 runner contract: 0 success, 2 regression (WP3), 3 config/dataset error, 4 service/model unavailable, 5 run failed/timeout, 130 interrupted",
			"credentials come from the active CLI profile or WEKNORA_HOST + WEKNORA_TOKEN/WEKNORA_API_KEY",
		},
	})
	return cmd
}

func buildRunOptions(
	c *cobra.Command,
	opts *RunOptions,
	cfg *evalrunner.RunnerConfig,
	taskID string,
) evalrunner.RunOptions {
	wait := opts.Wait && !opts.NoWait
	if cfg != nil && !c.Flags().Changed("wait") && !c.Flags().Changed("no-wait") {
		wait = cfg.Wait()
	}
	timeout := opts.Timeout
	if cfg != nil && !c.Flags().Changed("timeout") {
		if t, err := cfg.Timeout(); err == nil {
			timeout = t
		}
	}
	interval := opts.Interval
	if cfg != nil && !c.Flags().Changed("interval") {
		if iv, err := cfg.Interval(); err == nil {
			interval = iv
		}
	}
	reportDir := opts.ReportDir
	if cfg != nil && !c.Flags().Changed("report-dir") {
		reportDir = cfg.Execution.ReportDir
	}
	reproduce := fmt.Sprintf("make eval-baseline CONFIG=%s", opts.Config)
	if taskID != "" {
		reproduce = fmt.Sprintf("weknora eval run --task-id %s", taskID)
	}
	return evalrunner.RunOptions{
		Wait:      wait,
		Timeout:   timeout,
		Interval:  interval,
		ReportDir: reportDir,
		Reproduce: reproduce,
		TaskID:    taskID,
	}
}

func mapEvalError(err error) error {
	switch {
	case errors.Is(err, evalrunner.ErrInvalidConfig):
		return cmdutil.NewError(cmdutil.CodeEvalConfigError, err.Error())
	case errors.Is(err, evalrunner.ErrServiceUnavailable):
		return cmdutil.NewError(cmdutil.CodeEvalServiceUnavailable, err.Error())
	case errors.Is(err, evalrunner.ErrRunFailed):
		return cmdutil.NewError(cmdutil.CodeEvalRunFailed, err.Error())
	default:
		return err
	}
}
