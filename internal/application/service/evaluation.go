package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	repository "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/buildinfo"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/utils"
	"golang.org/x/sync/errgroup"
)

/*
corpus: pid -> content
queries: qid -> content
answers: aid -> content
qrels: qid -> pid
arels: qid -> aid
*/

const (
	evaluationHeartbeatInterval = 10 * time.Second
	// EvaluationStaleCutoff is the maximum allowed age of the last heartbeat
	// before a pending/running run is considered interrupted at startup.
	EvaluationStaleCutoff = 45 * time.Second
)

// ErrEvaluationTaskNotFound is returned when no run exists for the caller's tenant.
var ErrEvaluationTaskNotFound = errors.New("evaluation task not found")

// ErrInvalidEvaluationParams is returned when request parameter overrides are invalid.
var ErrInvalidEvaluationParams = errors.New("invalid evaluation params")

// EvaluationService handles evaluation tasks for knowledge base and chat models
type EvaluationService struct {
	config               *config.Config                  // Application configuration
	dataset              interfaces.DatasetService       // Service for dataset operations
	knowledgeBaseService interfaces.KnowledgeBaseService // Service for knowledge base operations
	knowledgeService     interfaces.KnowledgeService     // Service for knowledge operations
	sessionService       interfaces.SessionService       // Service for chat sessions
	modelService         interfaces.ModelService         // Service for model operations

	evaluationRunRepository interfaces.EvaluationRunRepository // Persistent storage for evaluation runs
}

func NewEvaluationService(
	config *config.Config,
	dataset interfaces.DatasetService,
	knowledgeBaseService interfaces.KnowledgeBaseService,
	knowledgeService interfaces.KnowledgeService,
	sessionService interfaces.SessionService,
	modelService interfaces.ModelService,
	evaluationRunRepository interfaces.EvaluationRunRepository,
) interfaces.EvaluationService {
	return &EvaluationService{
		config:                  config,
		dataset:                 dataset,
		knowledgeBaseService:    knowledgeBaseService,
		knowledgeService:        knowledgeService,
		sessionService:          sessionService,
		modelService:            modelService,
		evaluationRunRepository: evaluationRunRepository,
	}
}

func (e *EvaluationService) EvaluationResult(ctx context.Context, taskID string) (*types.EvaluationDetail, error) {
	logger.Info(ctx, "Start getting evaluation result")
	logger.Infof(ctx, "Task ID: %s", taskID)

	tenantID := types.MustTenantIDFromContext(ctx)
	run, err := e.evaluationRunRepository.GetByID(ctx, tenantID, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrEvaluationRunNotFound) {
			return nil, ErrEvaluationTaskNotFound
		}
		logger.Errorf(ctx, "Failed to get evaluation task: %v", err)
		return nil, err
	}

	logger.Info(ctx, "Evaluation result retrieved successfully")
	return e.evaluationRunToDetail(run)
}

func (e *EvaluationService) evaluationRunToDetail(run *types.EvaluationRun) (*types.EvaluationDetail, error) {
	var params types.ChatManage
	if len(run.Params) > 0 {
		if err := json.Unmarshal(run.Params, &params); err != nil {
			return nil, fmt.Errorf("evaluation run: decode params: %w", err)
		}
	}

	var metric *types.MetricResult
	if len(run.Metric) > 0 {
		metric = &types.MetricResult{}
		if err := json.Unmarshal(run.Metric, metric); err != nil {
			return nil, fmt.Errorf("evaluation run: decode metric: %w", err)
		}
	}

	return &types.EvaluationDetail{
		Task: &types.EvaluationTask{
			ID:        run.ID,
			TenantID:  run.TenantID,
			DatasetID: run.DatasetID,
			StartTime: run.StartTime,
			Status:    run.Status,
			ErrMsg:    run.ErrMsg,
			Total:     run.Total,
			Finished:  run.Finished,
		},
		Params: &params,
		Metric: metric,
	}, nil
}

// Evaluation starts a new evaluation task with the given options.
func (e *EvaluationService) Evaluation(ctx context.Context, opts *types.EvaluationOptions) (*types.EvaluationDetail, error) {
	logger.Info(ctx, "Start evaluation")
	if opts == nil {
		return nil, ErrInvalidEvaluationParams
	}
	datasetID := opts.DatasetID
	knowledgeBaseID := opts.KnowledgeBaseID
	chatModelID := opts.ChatModelID
	rerankModelID := opts.RerankModelID
	embeddingModelID := opts.EmbeddingModelID
	logger.Infof(ctx, "Dataset ID: %s, Knowledge Base ID: %s, Chat Model ID: %s, Rerank Model ID: %s, Embedding Model ID: %s",
		datasetID, knowledgeBaseID, chatModelID, rerankModelID, embeddingModelID)

	// Get tenant ID from context for multi-tenancy support
	tenantID := types.MustTenantIDFromContext(ctx)
	logger.Infof(ctx, "Tenant ID: %d", tenantID)

	// Load and validate the dataset before creating the knowledge base or
	// persisting a run, so configuration errors fail fast with HTTP 400.
	if datasetID == "" {
		datasetID = "default"
		logger.Info(ctx, "Using default dataset")
	}
	loadedDataset, err := e.dataset.GetDatasetByID(ctx, datasetID)
	if err != nil {
		logger.Errorf(ctx, "Failed to load dataset: %v", err)
		return nil, err
	}

	// Handle knowledge base creation if not provided
	if knowledgeBaseID == "" {
		logger.Info(ctx, "No knowledge base ID provided, creating new knowledge base")
		var llmModelID string
		if embeddingModelID == "" {
			// 获取默认的嵌入模型和LLM模型
			models, listErr := e.modelService.ListModels(ctx)
			if listErr != nil {
				logger.Errorf(ctx, "Failed to list models: %v", listErr)
				return nil, listErr
			}
			for _, model := range models {
				if model == nil {
					continue
				}
				if model.Type == types.ModelTypeEmbedding {
					embeddingModelID = model.ID
				}
				if model.Type == types.ModelTypeKnowledgeQA {
					llmModelID = model.ID
				}
			}

			if embeddingModelID == "" || llmModelID == "" {
				return nil, fmt.Errorf("no default models found for evaluation")
			}
		} else if err := validateEvaluationEmbeddingModel(ctx, e.modelService, embeddingModelID); err != nil {
			return nil, err
		}

		kb, err := e.knowledgeBaseService.CreateKnowledgeBase(ctx, &types.KnowledgeBase{
			Name:             "evaluation",
			Description:      "evaluation",
			EmbeddingModelID: embeddingModelID,
			SummaryModelID:   llmModelID,
		})
		if err != nil {
			logger.Errorf(ctx, "Failed to create knowledge base: %v", err)
			return nil, err
		}
		embeddingModelID = kb.EmbeddingModelID
		knowledgeBaseID = kb.ID
		logger.Infof(ctx, "Created new knowledge base with ID: %s", knowledgeBaseID)
	} else {
		logger.Infof(ctx, "Using existing knowledge base ID: %s", knowledgeBaseID)
		// Create evaluation-specific knowledge base based on existing one
		kb, err := e.knowledgeBaseService.GetKnowledgeBaseByID(ctx, knowledgeBaseID)
		if err != nil {
			logger.Errorf(ctx, "Failed to get knowledge base: %v", err)
			return nil, err
		}

		if embeddingModelID == "" {
			embeddingModelID = kb.EmbeddingModelID
		} else if err := validateEvaluationEmbeddingModel(ctx, e.modelService, embeddingModelID); err != nil {
			return nil, err
		}

		kb, err = e.knowledgeBaseService.CreateKnowledgeBase(ctx, &types.KnowledgeBase{
			Name:             "evaluation",
			Description:      "evaluation",
			EmbeddingModelID: embeddingModelID,
			SummaryModelID:   kb.SummaryModelID,
		})
		if err != nil {
			logger.Errorf(ctx, "Failed to create knowledge base: %v", err)
			return nil, err
		}
		knowledgeBaseID = kb.ID
		logger.Infof(ctx, "Created new knowledge base with ID: %s based on existing one", knowledgeBaseID)
	}

	if rerankModelID == "" {
		// 获取默认的重排模型
		models, err := e.modelService.ListModels(ctx)
		if err == nil {
			for _, model := range models {
				if model == nil {
					continue
				}
				if model.Type == types.ModelTypeRerank {
					rerankModelID = model.ID
					break
				}
			}
		}
		if rerankModelID == "" {
			logger.Warnf(ctx, "No rerank model found, skipping rerank")
		} else {
			logger.Infof(ctx, "Using default rerank model: %s", rerankModelID)
		}
	}

	if chatModelID == "" {
		// 获取默认的LLM模型
		models, err := e.modelService.ListModels(ctx)
		if err == nil {
			for _, model := range models {
				if model == nil {
					continue
				}
				if model.Type == types.ModelTypeKnowledgeQA {
					chatModelID = model.ID
					break
				}
			}
		}
		if chatModelID == "" {
			return nil, fmt.Errorf("no default chat model found")
		}
		logger.Infof(ctx, "Using default chat model: %s", chatModelID)
	}

	// Create evaluation task with unique ID
	logger.Info(ctx, "Creating evaluation task")
	taskID := utils.GenerateTaskID("evaluation", tenantID, datasetID)
	logger.Infof(ctx, "Generated task ID: %s", taskID)

	// Prepare evaluation detail with all parameters
	params := &types.ChatManage{
		PipelineRequest: types.PipelineRequest{
			VectorThreshold:  e.config.Conversation.VectorThreshold,
			KeywordThreshold: e.config.Conversation.KeywordThreshold,
			EmbeddingTopK:    e.config.Conversation.EmbeddingTopK,
			MaxRounds:        e.config.Conversation.MaxRounds,
			RerankModelID:    rerankModelID,
			RerankTopK:       e.config.Conversation.RerankTopK,
			RerankThreshold:  e.config.Conversation.RerankThreshold,
			ChatModelID:      chatModelID,
			SummaryConfig: types.SummaryConfig{
				MaxTokens:           e.config.Conversation.Summary.MaxTokens,
				RepeatPenalty:       e.config.Conversation.Summary.RepeatPenalty,
				TopK:                e.config.Conversation.Summary.TopK,
				TopP:                e.config.Conversation.Summary.TopP,
				Prompt:              e.config.Conversation.Summary.Prompt,
				ContextTemplate:     e.config.Conversation.Summary.ContextTemplate,
				FrequencyPenalty:    e.config.Conversation.Summary.FrequencyPenalty,
				PresencePenalty:     e.config.Conversation.Summary.PresencePenalty,
				NoMatchPrefix:       e.config.Conversation.Summary.NoMatchPrefix,
				Temperature:         e.config.Conversation.Summary.Temperature,
				Seed:                e.config.Conversation.Summary.Seed,
				MaxCompletionTokens: e.config.Conversation.Summary.MaxCompletionTokens,
			},
			FallbackResponse:    e.config.Conversation.FallbackResponse,
			RewritePromptSystem: e.config.Conversation.RewritePromptSystem,
			RewritePromptUser:   e.config.Conversation.RewritePromptUser,
		},
	}
	if err := applyEvaluationParams(params, opts.Params); err != nil {
		logger.Errorf(ctx, "Failed to apply evaluation params: %v", err)
		return nil, err
	}
	detail := &types.EvaluationDetail{
		Task: &types.EvaluationTask{
			ID:        taskID,
			TenantID:  tenantID,
			DatasetID: datasetID,
			Status:    types.EvaluationStatuePending,
			StartTime: time.Now(),
		},
		Params: params,
	}

	paramsJSON, err := json.Marshal(sanitizeEvaluationParams(params))
	if err != nil {
		return nil, fmt.Errorf("evaluation: encode params: %w", err)
	}
	snapshot := types.EvaluationConfigSnapshot{
		Dataset: types.DatasetSnapshot{
			ID:          datasetID,
			SHA256:      loadedDataset.SHA256,
			SampleCount: loadedDataset.SampleCount,
		},
		Models: e.buildModelSnapshots(ctx, embeddingModelID, chatModelID, rerankModelID),
		Version: types.VersionSignature{
			AppVersion: buildinfo.Version,
			GitCommit:  buildinfo.CommitID,
			GitDirty:   buildinfo.IsGitDirty(),
			GoVersion:  buildinfo.GoVersion,
		},
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("evaluation: encode config snapshot: %w", err)
	}
	configHash, err := e.computeConfigHash(params, snapshot.Dataset, snapshot.Models)
	if err != nil {
		return nil, fmt.Errorf("evaluation: compute config hash: %w", err)
	}

	run := &types.EvaluationRun{
		ID:             taskID,
		TenantID:       tenantID,
		DatasetID:      datasetID,
		Status:         types.EvaluationStatuePending,
		StartTime:      time.Now(),
		Params:         paramsJSON,
		ConfigHash:     configHash,
		ConfigSnapshot: snapshotJSON,
		TemporaryKBID:  knowledgeBaseID,
	}
	logger.Info(ctx, "Persisting evaluation task")
	if err := e.evaluationRunRepository.Create(ctx, run); err != nil {
		logger.Errorf(ctx, "Failed to persist evaluation task: %v", err)
		return nil, err
	}

	// Start evaluation in background goroutine
	logger.Info(ctx, "Starting evaluation in background")
	go func() {
		// Create new context with logger for background task
		newCtx := logger.CloneContext(ctx)
		logger.Infof(newCtx, "Background evaluation started for task ID: %s", taskID)

		// CAS: pending -> running; anything else means the run was already
		// terminal (e.g. marked interrupted by a concurrent restart scan).
		ok, err := e.evaluationRunRepository.TransitionStatus(
			newCtx,
			taskID,
			[]types.EvaluationStatue{types.EvaluationStatuePending},
			types.EvaluationStatueRunning,
			"",
		)
		if err != nil {
			logger.Errorf(newCtx, "Failed to start evaluation task: %v", err)
			return
		}
		if !ok {
			logger.Warnf(newCtx, "Evaluation task no longer pending, skipping execution: %s", taskID)
			return
		}
		detail.Task.Status = types.EvaluationStatueRunning
		logger.Info(newCtx, "Evaluation task status set to running")

		heartbeatCtx, stopHeartbeat := context.WithCancel(newCtx)
		go e.runHeartbeat(heartbeatCtx, taskID)

		// Execute actual evaluation
		if err := e.EvalDataset(newCtx, detail, knowledgeBaseID, loadedDataset); err != nil {
			stopHeartbeat()
			detail.Task.Status = types.EvaluationStatueFailed
			detail.Task.ErrMsg = err.Error()
			if _, transitionErr := e.evaluationRunRepository.TransitionStatus(
				newCtx,
				taskID,
				[]types.EvaluationStatue{types.EvaluationStatueRunning},
				types.EvaluationStatueFailed,
				err.Error(),
			); transitionErr != nil {
				logger.Errorf(newCtx, "Failed to persist evaluation failure: %v", transitionErr)
			}
			logger.Errorf(newCtx, "Evaluation task failed: %v, task ID: %s", err, taskID)
			return
		}
		stopHeartbeat()

		metricJSON, err := json.Marshal(detail.Metric)
		if err != nil {
			detail.Task.Status = types.EvaluationStatueFailed
			detail.Task.ErrMsg = err.Error()
			if _, transitionErr := e.evaluationRunRepository.TransitionStatus(
				newCtx,
				taskID,
				[]types.EvaluationStatue{types.EvaluationStatueRunning},
				types.EvaluationStatueFailed,
				err.Error(),
			); transitionErr != nil {
				logger.Errorf(newCtx, "Failed to persist evaluation failure: %v", transitionErr)
			}
			logger.Errorf(newCtx, "Failed to encode evaluation metric: %v", err)
			return
		}
		if err := e.evaluationRunRepository.UpdateProgress(
			newCtx,
			taskID,
			detail.Task.Finished,
			detail.Task.Total,
			metricJSON,
		); err != nil {
			detail.Task.Status = types.EvaluationStatueFailed
			detail.Task.ErrMsg = err.Error()
			if _, transitionErr := e.evaluationRunRepository.TransitionStatus(
				newCtx,
				taskID,
				[]types.EvaluationStatue{types.EvaluationStatueRunning},
				types.EvaluationStatueFailed,
				err.Error(),
			); transitionErr != nil {
				logger.Errorf(newCtx, "Failed to persist evaluation failure: %v", transitionErr)
			}
			logger.Errorf(newCtx, "Failed to persist final evaluation metric: %v", err)
			return
		}

		// Mark task as completed successfully
		ok, err = e.evaluationRunRepository.TransitionStatus(
			newCtx,
			taskID,
			[]types.EvaluationStatue{types.EvaluationStatueRunning},
			types.EvaluationStatueSuccess,
			"",
		)
		if err != nil {
			logger.Errorf(newCtx, "Failed to persist evaluation success: %v", err)
			return
		}
		if !ok {
			logger.Warnf(newCtx, "Evaluation task was not running, skipping success transition: %s", taskID)
			return
		}
		logger.Infof(newCtx, "Evaluation task completed successfully, task ID: %s", taskID)
	}()

	logger.Infof(ctx, "Evaluation task created successfully, task ID: %s", taskID)
	return detail, nil
}

// EvalDataset performs the actual evaluation of a loaded dataset.
func (e *EvaluationService) EvalDataset(
	ctx context.Context,
	detail *types.EvaluationDetail,
	knowledgeBaseID string,
	dataset *types.EvaluationDataset,
) error {
	logger.Info(ctx, "Start evaluating dataset")
	logger.Infof(ctx, "Task ID: %s, Dataset ID: %s", detail.Task.ID, detail.Task.DatasetID)

	logger.Infof(ctx, "Dataset retrieved successfully with %d QA pairs", len(dataset.Pairs))

	detail.Task.Total = len(dataset.Pairs)
	if err := e.evaluationRunRepository.SetDatasetHash(
		ctx,
		detail.Task.ID,
		dataset.SHA256,
		len(dataset.Pairs),
	); err != nil {
		logger.Errorf(ctx, "Failed to persist dataset hash: %v", err)
		return err
	}
	logger.Infof(ctx, "Updated task total to %d QA pairs", len(dataset.Pairs))

	// Extract and organize passages from dataset
	passages := getPassageList(dataset.Pairs)
	logger.Infof(ctx, "Creating knowledge from %d passages", len(passages))

	// Create knowledge base from passages (sync: wait for indexing to complete before querying)
	knowledge, err := e.knowledgeService.CreateKnowledgeFromPassageSync(ctx, knowledgeBaseID, passages, "")
	if err != nil {
		logger.Errorf(ctx, "Failed to create knowledge from passages: %v", err)
		return err
	}
	logger.Infof(ctx, "Knowledge created and indexed successfully, ID: %s", knowledge.ID)

	// Setup cleanup of temporary resources
	defer func() {
		logger.Infof(ctx, "Cleaning up resources - deleting knowledge: %s", knowledge.ID)
		if err := e.knowledgeService.DeleteKnowledge(ctx, knowledge.ID); err != nil {
			logger.Errorf(ctx, "Failed to delete knowledge: %v, knowledge ID: %s", err, knowledge.ID)
		}

		logger.Infof(ctx, "Cleaning up resources - deleting knowledge base: %s", knowledgeBaseID)
		if err := e.knowledgeBaseService.DeleteKnowledgeBase(ctx, knowledgeBaseID); err != nil {
			logger.Errorf(
				ctx,
				"Failed to delete knowledge base: %v, knowledge base ID: %s",
				err, knowledgeBaseID,
			)
		}
	}()

	// Initialize parallel evaluation metrics
	var finished int
	var mu sync.Mutex
	var g errgroup.Group
	metricHook := NewHookMetric(len(dataset.Pairs))

	// Set worker limit based on available CPUs
	g.SetLimit(max(runtime.GOMAXPROCS(0)-1, 1))
	logger.Infof(ctx, "Starting evaluation with %d parallel workers", max(runtime.GOMAXPROCS(0)-1, 1))

	// Process each QA pair in parallel
	for i, qaPair := range dataset.Pairs {
		qaPair := qaPair
		i := i
		g.Go(func() error {
			logger.Infof(ctx, "Processing QA pair %d, question: %s", i, qaPair.Question)

			// Prepare chat management parameters for this QA pair
			chatManage := detail.Params.Clone()
			chatManage.Query = qaPair.Question
			chatManage.RewriteQuery = qaPair.Question
			// Set knowledge base ID and search targets for this evaluation
			chatManage.KnowledgeBaseIDs = []string{knowledgeBaseID}
			chatManage.SearchTargets = types.SearchTargets{
				&types.SearchTarget{
					Type:            types.SearchTargetTypeKnowledgeBase,
					KnowledgeBaseID: knowledgeBaseID,
				},
			}

			// Execute knowledge QA pipeline
			logger.Infof(ctx, "Running knowledge QA for question: %s", qaPair.Question)
			err = e.sessionService.KnowledgeQAByEvent(ctx, chatManage, types.Pipline["rag"])
			if err != nil {
				logger.Errorf(ctx, "Failed to process question %d: %v", i, err)
				return err
			}

			// Record evaluation metrics
			logger.Infof(ctx, "Recording metrics for QA pair %d", i)
			metricHook.recordInit(i)
			metricHook.recordQaPair(i, qaPair)
			metricHook.recordSearchResult(i, chatManage.SearchResult)
			metricHook.recordRerankResult(i, chatManage.RerankResult)
			metricHook.recordChatResponse(i, chatManage.ChatResponse)
			metricHook.recordFinish(i)

			// Update progress metrics
			mu.Lock()
			finished += 1
			metricResult := metricHook.MetricResult()
			mu.Unlock()
			metricJSON, err := json.Marshal(metricResult)
			if err != nil {
				logger.Errorf(ctx, "Failed to encode evaluation metric: %v", err)
				return err
			}
			detail.Task.Finished = finished
			detail.Metric = metricResult
			if err := e.evaluationRunRepository.UpdateProgress(
				ctx,
				detail.Task.ID,
				finished,
				len(dataset.Pairs),
				metricJSON,
			); err != nil {
				logger.Errorf(ctx, "Failed to persist evaluation progress: %v", err)
				return err
			}
			logger.Infof(ctx, "Updated task progress: %d/%d completed", finished, len(dataset.Pairs))
			return nil
		})
	}

	// Wait for all parallel evaluations to complete
	logger.Info(ctx, "Waiting for all evaluation tasks to complete")
	if err := g.Wait(); err != nil {
		logger.Errorf(ctx, "Evaluation error: %v", err)
		return err
	}

	metricResult := metricHook.MetricResult()
	metricJSON, err := json.Marshal(metricResult)
	if err != nil {
		return err
	}
	detail.Task.Finished = finished
	detail.Metric = metricResult
	if err := e.evaluationRunRepository.UpdateProgress(
		ctx,
		detail.Task.ID,
		finished,
		len(dataset.Pairs),
		metricJSON,
	); err != nil {
		logger.Errorf(ctx, "Failed to persist final evaluation progress: %v", err)
		return err
	}

	logger.Infof(ctx, "Dataset evaluation completed successfully, task ID: %s", detail.Task.ID)
	return nil
}

// ListEvaluationRuns returns tenant-scoped historical runs ordered by
// creation time descending.
func (e *EvaluationService) ListEvaluationRuns(
	ctx context.Context,
	status *types.EvaluationStatue,
	p *types.Pagination,
) (*types.PageResult, error) {
	if p == nil {
		p = &types.Pagination{}
	}
	tenantID := types.MustTenantIDFromContext(ctx)
	runs, total, err := e.evaluationRunRepository.List(ctx, tenantID, status, p)
	if err != nil {
		return nil, err
	}
	return types.NewPageResult(total, p, runs), nil
}

func (e *EvaluationService) runHeartbeat(ctx context.Context, taskID string) {
	ticker := time.NewTicker(evaluationHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.evaluationRunRepository.UpdateHeartbeat(ctx, taskID, time.Now()); err != nil {
				logger.Errorf(ctx, "Failed to update evaluation heartbeat: %v", err)
			}
		}
	}
}

func (e *EvaluationService) buildModelSnapshots(
	ctx context.Context,
	embeddingID string,
	chatID string,
	rerankID string,
) []types.ModelSnapshot {
	type modelRef struct {
		id  string
		typ string
	}
	refs := []modelRef{
		{embeddingID, string(types.ModelTypeEmbedding)},
		{chatID, string(types.ModelTypeKnowledgeQA)},
		{rerankID, string(types.ModelTypeRerank)},
	}
	snapshots := make([]types.ModelSnapshot, 0, len(refs))
	for _, ref := range refs {
		if ref.id == "" {
			continue
		}
		snapshot := types.ModelSnapshot{ID: ref.id, Type: ref.typ}
		model, err := e.modelService.GetModelByID(ctx, ref.id)
		if err != nil {
			logger.Warnf(ctx, "Failed to load model metadata for snapshot, model: %s: %v", ref.id, err)
		} else if model != nil {
			snapshot.Name = model.Name
			snapshot.Provider = model.Parameters.Provider
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots
}

type configHashPayload struct {
	Params  *types.ChatManage     `json:"params"`
	Dataset types.DatasetSnapshot `json:"dataset"`
	Models  []types.ModelSnapshot `json:"models"`
}

func (e *EvaluationService) computeConfigHash(
	params *types.ChatManage,
	dataset types.DatasetSnapshot,
	models []types.ModelSnapshot,
) (string, error) {
	if models == nil {
		models = []types.ModelSnapshot{}
	}
	payload := configHashPayload{
		Params:  params,
		Dataset: dataset,
		Models:  models,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func validateEvaluationEmbeddingModel(ctx context.Context, modelService interfaces.ModelService, id string) error {
	model, err := modelService.GetModelByID(ctx, id)
	if err != nil {
		return fmt.Errorf("resolve embedding model %q: %w", id, err)
	}
	if model == nil || model.Type != types.ModelTypeEmbedding {
		return fmt.Errorf("model %q is not an embedding model", id)
	}
	return nil
}

func applyEvaluationParams(params *types.ChatManage, overrides *types.EvaluationParamsOverride) error {
	if overrides == nil {
		return nil
	}
	if overrides.VectorThreshold != nil {
		if *overrides.VectorThreshold < 0 || *overrides.VectorThreshold > 1 {
			return fmt.Errorf("%w: vector_threshold must be in [0,1]", ErrInvalidEvaluationParams)
		}
		params.VectorThreshold = *overrides.VectorThreshold
	}
	if overrides.KeywordThreshold != nil {
		if *overrides.KeywordThreshold < 0 || *overrides.KeywordThreshold > 1 {
			return fmt.Errorf("%w: keyword_threshold must be in [0,1]", ErrInvalidEvaluationParams)
		}
		params.KeywordThreshold = *overrides.KeywordThreshold
	}
	if overrides.EmbeddingTopK != nil {
		if *overrides.EmbeddingTopK <= 0 {
			return fmt.Errorf("%w: embedding_top_k must be positive", ErrInvalidEvaluationParams)
		}
		params.EmbeddingTopK = *overrides.EmbeddingTopK
	}
	if overrides.MaxRounds != nil {
		if *overrides.MaxRounds < 0 {
			return fmt.Errorf("%w: max_rounds must be non-negative", ErrInvalidEvaluationParams)
		}
		params.MaxRounds = *overrides.MaxRounds
	}
	if overrides.RerankTopK != nil {
		if *overrides.RerankTopK <= 0 {
			return fmt.Errorf("%w: rerank_top_k must be positive", ErrInvalidEvaluationParams)
		}
		params.RerankTopK = *overrides.RerankTopK
	}
	if overrides.RerankThreshold != nil {
		if *overrides.RerankThreshold < 0 || *overrides.RerankThreshold > 1 {
			return fmt.Errorf("%w: rerank_threshold must be in [0,1]", ErrInvalidEvaluationParams)
		}
		params.RerankThreshold = *overrides.RerankThreshold
	}
	if overrides.EnableRewrite != nil {
		params.EnableRewrite = *overrides.EnableRewrite
	}
	if overrides.SummaryConfig == nil {
		return nil
	}
	sc := &params.SummaryConfig
	if overrides.SummaryConfig.MaxTokens != nil {
		if *overrides.SummaryConfig.MaxTokens < 0 {
			return fmt.Errorf("%w: summary max_tokens must be non-negative", ErrInvalidEvaluationParams)
		}
		sc.MaxTokens = *overrides.SummaryConfig.MaxTokens
	}
	if overrides.SummaryConfig.Temperature != nil {
		if *overrides.SummaryConfig.Temperature < 0 || *overrides.SummaryConfig.Temperature > 2 {
			return fmt.Errorf("%w: temperature must be in [0,2]", ErrInvalidEvaluationParams)
		}
		sc.Temperature = *overrides.SummaryConfig.Temperature
	}
	if overrides.SummaryConfig.TopK != nil {
		if *overrides.SummaryConfig.TopK < 0 {
			return fmt.Errorf("%w: summary top_k must be non-negative", ErrInvalidEvaluationParams)
		}
		sc.TopK = *overrides.SummaryConfig.TopK
	}
	if overrides.SummaryConfig.TopP != nil {
		if *overrides.SummaryConfig.TopP < 0 || *overrides.SummaryConfig.TopP > 1 {
			return fmt.Errorf("%w: top_p must be in [0,1]", ErrInvalidEvaluationParams)
		}
		sc.TopP = *overrides.SummaryConfig.TopP
	}
	if overrides.SummaryConfig.RepeatPenalty != nil {
		if *overrides.SummaryConfig.RepeatPenalty < 0 {
			return fmt.Errorf("%w: repeat_penalty must be non-negative", ErrInvalidEvaluationParams)
		}
		sc.RepeatPenalty = *overrides.SummaryConfig.RepeatPenalty
	}
	if overrides.SummaryConfig.FrequencyPenalty != nil {
		if *overrides.SummaryConfig.FrequencyPenalty < -2 || *overrides.SummaryConfig.FrequencyPenalty > 2 {
			return fmt.Errorf("%w: frequency_penalty must be in [-2,2]", ErrInvalidEvaluationParams)
		}
		sc.FrequencyPenalty = *overrides.SummaryConfig.FrequencyPenalty
	}
	if overrides.SummaryConfig.PresencePenalty != nil {
		if *overrides.SummaryConfig.PresencePenalty < -2 || *overrides.SummaryConfig.PresencePenalty > 2 {
			return fmt.Errorf("%w: presence_penalty must be in [-2,2]", ErrInvalidEvaluationParams)
		}
		sc.PresencePenalty = *overrides.SummaryConfig.PresencePenalty
	}
	if overrides.SummaryConfig.Seed != nil {
		sc.Seed = *overrides.SummaryConfig.Seed
	}
	if overrides.SummaryConfig.MaxCompletionTokens != nil {
		if *overrides.SummaryConfig.MaxCompletionTokens <= 0 {
			return fmt.Errorf("%w: max_completion_tokens must be positive", ErrInvalidEvaluationParams)
		}
		sc.MaxCompletionTokens = *overrides.SummaryConfig.MaxCompletionTokens
	}
	return nil
}

func sanitizeEvaluationParams(params *types.ChatManage) *types.ChatManage {
	if params == nil {
		return nil
	}
	cloned := params.Clone()
	cloned.SummaryConfig.Prompt = ""
	cloned.SummaryConfig.ContextTemplate = ""
	cloned.SummaryConfig.NoMatchPrefix = ""
	cloned.RewritePromptSystem = ""
	cloned.RewritePromptUser = ""
	cloned.FallbackPrompt = ""
	return cloned
}

// getPassageList extracts and organizes passages from QA pairs
// Returns a slice of passages indexed by their passage IDs
func getPassageList(dataset []*types.QAPair) []string {
	pIDMap := make(map[int]string)
	maxPID := 0
	for _, qaPair := range dataset {
		for i := 0; i < len(qaPair.PIDs); i++ {
			pIDMap[qaPair.PIDs[i]] = qaPair.Passages[i]
			maxPID = max(maxPID, qaPair.PIDs[i])
		}
	}
	passages := make([]string, maxPID+1)
	for i := 0; i <= maxPID; i++ {
		if _, ok := pIDMap[i]; ok {
			passages[i] = pIDMap[i]
		}
	}
	return passages
}
