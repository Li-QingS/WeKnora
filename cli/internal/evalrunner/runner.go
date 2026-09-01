package evalrunner

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Tencent/WeKnora/cli/internal/cmdutil"
	sdk "github.com/Tencent/WeKnora/client"
)

// Runner error sentinels are mapped to the WP2 exit-code contract by the
// cobra command layer.
var (
	ErrServiceUnavailable = errors.New("eval service unavailable")
	ErrRunFailed          = errors.New("eval run failed")
)

// EvalClient is the narrow SDK surface the runner needs. *sdk.Client
// satisfies it via duck typing.
type EvalClient interface {
	ListModels(ctx context.Context) ([]sdk.Model, error)
	StartEvaluation(ctx context.Context, request *sdk.EvaluationRequest) (*sdk.EvaluationDetail, error)
	GetEvaluationResult(ctx context.Context, taskID string) (*sdk.EvaluationDetail, error)
	ListEvaluationRuns(ctx context.Context, page, pageSize int) ([]sdk.EvaluationRun, int, error)
}

// RunOptions controls polling and report output for one invocation.
type RunOptions struct {
	Wait      bool
	Timeout   time.Duration
	Interval  time.Duration
	ReportDir string
	Reproduce string
	TaskID    string
}

// RunResult carries everything a successful (or reported-failure) invocation
// produced.
type RunResult struct {
	TaskID      string
	Detail      *sdk.EvaluationDetail
	Run         *sdk.EvaluationRun
	Report      *EvalReport
	ReportPaths []string
}

// Run resolves models, starts the evaluation, polls it, and writes reports.
// Failure after the run was created still writes a report and returns a
// non-nil error so the caller can classify the exit code.
func Run(ctx context.Context, cfg *RunnerConfig, cli EvalClient, opts RunOptions) (*RunResult, error) {
	if cfg == nil && opts.TaskID == "" {
		return nil, fmt.Errorf("%w: nil config", ErrInvalidConfig)
	}
	if cli == nil {
		return nil, fmt.Errorf("%w: nil client", ErrServiceUnavailable)
	}

	if opts.TaskID != "" {
		return finishRun(ctx, cli, &RunResult{TaskID: opts.TaskID}, opts)
	}

	chatID, err := resolveModelRef(ctx, cli, cfg.Models.Chat, "KnowledgeQA")
	if err != nil {
		return nil, err
	}
	embeddingID, err := resolveModelRef(ctx, cli, cfg.Models.Embedding, "Embedding")
	if err != nil {
		return nil, err
	}
	rerankID, err := resolveModelRef(ctx, cli, cfg.Models.Rerank, "Rerank")
	if err != nil {
		return nil, err
	}

	request := &sdk.EvaluationRequest{
		DatasetID:        cfg.DatasetID,
		ChatModelID:      chatID,
		EmbeddingModelID: embeddingID,
		RerankModelID:    rerankID,
		Params:           buildParams(cfg),
	}
	detail, err := cli.StartEvaluation(ctx, request)
	if err != nil {
		var apiErr *sdk.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusBadRequest {
			return nil, fmt.Errorf("%w: start evaluation: %v", ErrInvalidConfig, err)
		}
		return nil, fmt.Errorf("%w: start evaluation: %v", ErrServiceUnavailable, err)
	}

	result := &RunResult{TaskID: detail.Task.ID, Detail: detail}
	if !opts.Wait {
		return result, nil
	}

	return finishRun(ctx, cli, result, opts)
}

func finishRun(
	ctx context.Context,
	cli EvalClient,
	result *RunResult,
	opts RunOptions,
) (*RunResult, error) {
	terminalDetail, err := waitForTerminal(ctx, cli, result.TaskID, opts)
	if err != nil {
		return nil, err
	}
	result.Detail = terminalDetail

	run, listErr := findRun(ctx, cli, result.TaskID)
	if listErr != nil {
		return result, listErr
	}
	result.Run = run

	report, buildErr := BuildReport(terminalDetail, run, opts.Reproduce)
	if buildErr != nil {
		return result, fmt.Errorf("%w: build report: %v", ErrRunFailed, buildErr)
	}
	result.Report = report
	paths, writeErr := WriteReports(opts.ReportDir, report)
	if writeErr != nil {
		return result, fmt.Errorf("%w: write report: %v", ErrRunFailed, writeErr)
	}
	result.ReportPaths = paths

	if terminalDetail.Task.Status != 2 {
		return result, fmt.Errorf("%w: run %s ended with status %d: %s",
			ErrRunFailed, terminalDetail.Task.ID, terminalDetail.Task.Status, terminalDetail.Task.ErrMsg)
	}
	return result, nil
}

func resolveModelRef(ctx context.Context, cli EvalClient, ref, wantType string) (string, error) {
	if ref == "" {
		return "", nil
	}
	id, err := cmdutil.ResolveModelRef(ctx, cli, ref, wantType)
	if err != nil {
		return "", fmt.Errorf("%w: resolve %s model %q: %v", ErrInvalidConfig, wantType, ref, err)
	}
	return id, nil
}

func waitForTerminal(
	ctx context.Context,
	cli EvalClient,
	taskID string,
	opts RunOptions,
) (*sdk.EvaluationDetail, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	interval := opts.Interval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		detail, err := cli.GetEvaluationResult(ctx, taskID)
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil, context.Canceled
			}
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, fmt.Errorf("%w: wait timed out after %s", ErrRunFailed, timeout)
			}
			return nil, fmt.Errorf("%w: poll evaluation: %v", ErrServiceUnavailable, err)
		}
		switch detail.Task.Status {
		case 2, 3, 4:
			return detail, nil
		}

		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil, context.Canceled
			}
			return nil, fmt.Errorf("%w: wait timed out after %s", ErrRunFailed, timeout)
		case <-ticker.C:
		}
	}
}

func findRun(ctx context.Context, cli EvalClient, taskID string) (*sdk.EvaluationRun, error) {
	runs, _, err := cli.ListEvaluationRuns(ctx, 1, 100)
	if err != nil {
		return nil, fmt.Errorf("%w: list evaluation runs: %v", ErrServiceUnavailable, err)
	}
	for i := range runs {
		if runs[i].ID == taskID {
			return &runs[i], nil
		}
	}
	return nil, fmt.Errorf("%w: history record for %s not found", ErrRunFailed, taskID)
}

func buildParams(cfg *RunnerConfig) map[string]any {
	params := map[string]any{}
	if cfg.Retrieval.VectorThreshold != nil {
		params["vector_threshold"] = *cfg.Retrieval.VectorThreshold
	}
	if cfg.Retrieval.KeywordThreshold != nil {
		params["keyword_threshold"] = *cfg.Retrieval.KeywordThreshold
	}
	if cfg.Retrieval.EmbeddingTopK != nil {
		params["embedding_top_k"] = *cfg.Retrieval.EmbeddingTopK
	}
	if cfg.Retrieval.RerankTopK != nil {
		params["rerank_top_k"] = *cfg.Retrieval.RerankTopK
	}
	if cfg.Retrieval.RerankThreshold != nil {
		params["rerank_threshold"] = *cfg.Retrieval.RerankThreshold
	}
	if summary := buildSummaryParams(cfg.Generation); len(summary) > 0 {
		params["summary_config"] = summary
	}
	if len(params) == 0 {
		return nil
	}
	return params
}

func buildSummaryParams(g RunnerGeneration) map[string]any {
	summary := map[string]any{}
	if g.MaxTokens != nil {
		summary["max_tokens"] = *g.MaxTokens
	}
	if g.RepeatPenalty != nil {
		summary["repeat_penalty"] = *g.RepeatPenalty
	}
	if g.TopK != nil {
		summary["top_k"] = *g.TopK
	}
	if g.TopP != nil {
		summary["top_p"] = *g.TopP
	}
	if g.Temperature != nil {
		summary["temperature"] = *g.Temperature
	}
	if g.Seed != nil {
		summary["seed"] = *g.Seed
	}
	if g.MaxCompletionTokens != nil {
		summary["max_completion_tokens"] = *g.MaxCompletionTokens
	}
	return summary
}
