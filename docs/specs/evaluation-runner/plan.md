# 可复现评测 Runner（课题三 WP2）Plan

> 状态：待审批（2026-09-01）
> 上游文档：[spec.md](./spec.md)（已批准）

## 架构概览

WP2 由三块组成：

1. **服务端能力补齐**：`dataset_id` 真正生效（多数据集 + 校验 + 确定性 SHA-256）；`POST /evaluation` 接受可选的 `embedding_id` 与 `params` 覆盖；`config_hash` 在创建时即包含数据集哈希与生效参数，保证“同配置同哈希”。
2. **SDK 契约对齐**：更新 `client/` 的评测请求/响应类型，使其匹配 WP1 的真实 API；新增历史运行查询，供 Runner 生成与数据库一致的报告。
3. **Runner 命令行**：在现有 `cli/` 模块内新增 `weknora eval run`，读取 YAML、解析模型、发起任务、轮询、生成 `evaluation-result.json` / `evaluation-report.md`，并返回 WP2 约定的退出码；根目录 `make eval-baseline` 作为一条命令入口。

Runner 不重写评测链路，所有评测逻辑仍由服务端完成。

```
make eval-baseline CONFIG=... 
  → weknora eval run --config ...
    → 读取/校验 YAML
    → 用现有 CLI 凭据（profile 或 WEKNORA_HOST/TOKEN）构造 SDK client
    → model list 解析模型名称 → 组装 EvaluationOptions
    → POST /api/v1/evaluation
    → 轮询 GET /api/v1/evaluation?task_id=
    → GET /api/v1/evaluation/runs 取 config_hash/config_snapshot
    → 写 artifacts/evaluation/evaluation-result.json + evaluation-report.md
    → 按 WP2 退出码退出（0/2/3/4/5/130）
```

## 核心数据结构

### types.EvaluationOptions

替代原来 `Evaluation` 的多个字符串参数，Handler 从请求体映射到该结构：

```go
type EvaluationOptions struct {
    DatasetID        string                   `json:"dataset_id"`
    KnowledgeBaseID  string                   `json:"knowledge_base_id"`
    ChatModelID      string                   `json:"chat_id"`
    RerankModelID    string                   `json:"rerank_id"`
    EmbeddingModelID string                   `json:"embedding_id"`
    Params           *EvaluationParamsOverride `json:"params,omitempty"`
}
```

### types.EvaluationParamsOverride

只包含 Runner 需要覆盖的参数，全部用指针区分“未传”与“零值”：

```go
type EvaluationParamsOverride struct {
    VectorThreshold  *float64                `json:"vector_threshold,omitempty"`
    KeywordThreshold *float64                `json:"keyword_threshold,omitempty"`
    EmbeddingTopK    *int                    `json:"embedding_top_k,omitempty"`
    MaxRounds        *int                    `json:"max_rounds,omitempty"`
    RerankTopK       *int                    `json:"rerank_top_k,omitempty"`
    RerankThreshold  *float64                `json:"rerank_threshold,omitempty"`
    EnableRewrite    *bool                   `json:"enable_rewrite,omitempty"`
    SummaryConfig    *SummaryConfigOverride  `json:"summary_config,omitempty"`
}

type SummaryConfigOverride struct {
    MaxTokens            *int     `json:"max_tokens,omitempty"`
    Temperature          *float64 `json:"temperature,omitempty"`
    TopK                 *int     `json:"top_k,omitempty"`
    TopP                 *float64 `json:"top_p,omitempty"`
    RepeatPenalty        *float64 `json:"repeat_penalty,omitempty"`
    FrequencyPenalty     *float64 `json:"frequency_penalty,omitempty"`
    PresencePenalty      *float64 `json:"presence_penalty,omitempty"`
    Seed                 *int     `json:"seed,omitempty"`
    MaxCompletionTokens  *int     `json:"max_completion_tokens,omitempty"`
}
```

### types.EvaluationDataset

DatasetService 的返回值，替代原来的裸 `[]*types.QAPair`：

```go
type EvaluationDataset struct {
    ID          string
    SHA256      string
    SampleCount int
    Pairs       []*QAPair
}
```

### cli/internal/evalrunner.RunnerConfig

YAML 配置的 Go 表示：

```go
type RunnerConfig struct {
    DatasetID string          `yaml:"dataset_id"`
    Models    RunnerModels    `yaml:"models"`
    Retrieval RunnerRetrieval `yaml:"retrieval"`
    Generation RunnerGeneration `yaml:"generation"`
    Execution RunnerExecution `yaml:"execution"`
}

type RunnerModels struct {
    Chat      string `yaml:"chat"`
    Embedding string `yaml:"embedding"`
    Rerank    string `yaml:"rerank"`
}

type RunnerExecution struct {
    Wait       bool   `yaml:"wait"`
    Timeout    string `yaml:"timeout"`    // 如 30m
    Interval   string `yaml:"interval"`   // 如 2s
    ReportDir  string `yaml:"report_dir"` // 默认 artifacts/evaluation
}
```

### cli/internal/evalrunner.EvalReport

报告数据模型（JSON 字段固定，序列化顺序由 struct 定义保证）：

```go
type EvalReport struct {
    RunID       string            `json:"run_id"`
    ConfigHash  string            `json:"config_hash"`
    Status      int               `json:"status"`
    StatusLabel string            `json:"status_label"`
    Dataset     DatasetReport     `json:"dataset"`
    Models      []ModelReport     `json:"models"`
    Params      map[string]any    `json:"params"`
    Metric      map[string]any    `json:"metric,omitempty"`
    ErrMsg      string            `json:"err_msg,omitempty"`
    Finished    int               `json:"finished"`
    Total       int               `json:"total"`
    Version     map[string]any    `json:"version,omitempty"`
    Reproduce   string            `json:"reproduce"`
    GeneratedAt string            `json:"generated_at"`
}
```

## 接口变更

### interfaces.DatasetService

```go
type DatasetService interface {
    // GetDatasetByID loads, validates and hashes the named dataset.
    GetDatasetByID(ctx context.Context, datasetID string) (*types.EvaluationDataset, error)
}
```

### interfaces.EvaluationService

```go
type EvaluationService interface {
    Evaluation(ctx context.Context, opts *types.EvaluationOptions) (*types.EvaluationDetail, error)
    EvaluationResult(ctx context.Context, taskID string) (*types.EvaluationDetail, error)
    ListEvaluationRuns(ctx context.Context, status *types.EvaluationStatue, p *types.Pagination) (*types.PageResult, error)
}
```

### client SDK（client/evaluation.go）

```go
type EvaluationTask struct {
    ID        string `json:"id"`
    TenantID  uint64 `json:"tenant_id"`
    DatasetID string `json:"dataset_id"`
    StartTime string `json:"start_time"`
    Status    int    `json:"status"`
    ErrMsg    string `json:"err_msg,omitempty"`
    Total     int    `json:"total,omitempty"`
    Finished  int    `json:"finished,omitempty"`
}

type EvaluationDetail struct {
    Task   EvaluationTask  `json:"task"`
    Params json.RawMessage `json:"params"`
    Metric json.RawMessage `json:"metric,omitempty"`
}

type EvaluationRun struct { // 对齐 /evaluation/runs 列表项
    ID             string          `json:"id"`
    DatasetID      string          `json:"dataset_id"`
    Status         int             `json:"status"`
    ErrMsg         string          `json:"err_msg"`
    Total          int             `json:"total"`
    Finished       int             `json:"finished"`
    Params         json.RawMessage `json:"params"`
    Metric         json.RawMessage `json:"metric,omitempty"`
    ConfigHash     string          `json:"config_hash"`
    ConfigSnapshot json.RawMessage `json:"config_snapshot"`
}
```

SDK 方法：

- `StartEvaluation(ctx, *EvaluationRequest) (*EvaluationDetail, error)`
- `GetEvaluationResult(ctx, taskID) (*EvaluationDetail, error)`
- `ListEvaluationRuns(ctx, page, pageSize) ([]EvaluationRun, int, error)`

## 模块设计

### 服务端：DatasetService 改造

**职责：** 按 `dataset_id` 加载、校验、哈希数据集。
**对外接口：** `GetDatasetByID(ctx, datasetID) (*types.EvaluationDataset, error)`
**依赖：** 本地文件系统 + `parquet-go`。

实现要点：

1. ID 安全校验：仅允许 `[A-Za-z0-9_-]+`；空值按 `default` 处理。
2. 目录解析：`default` → `dataset/samples`，其他 ID → `dataset/<id>`。
3. 加载 5 个 Parquet：`queries/corpus/answers/qrels/qas`，缺任一文件即报数据集错误。
4. 引用完整性校验：
   - 每个 `qas.qid` 有对应查询与答案；
   - 每个 `qrels.qid` 有对应查询；
   - 每个 `qrels.pid` 有对应语料；
   - 每个查询至少有一条答案与一条 qrel（否则认为数据集无效）。
5. 确定性哈希：对每个文件按主键排序（`id` 或 `qid,pid` / `qid,aid`），再按固定文件顺序序列化后做 SHA-256，内容相同则哈希相同。
6. 错误类型：`ErrDatasetNotFound`、`ErrInvalidDataset`，Handler 映射为 HTTP 400。

### 服务端：EvaluationService 参数装配

**职责：** 合并默认参数与请求覆盖，加载数据集并计算最终 `config_hash`。
**依赖：** DatasetService、ModelService、EvaluationRunRepository。

流程调整：

1. `Evaluation(ctx, opts)` 先调用 `dataset.GetDatasetByID`，让坏数据集在创建运行前失败（Runner 才能得到退出码 3）。
2. 构建默认 `ChatManage`（现有逻辑），再应用 `opts.Params` 的非空覆盖；覆盖值做合法性校验（阈值 0-1、TopK > 0 等）。
3. 若 `opts.EmbeddingModelID` 非空，通过 `modelService.GetModelByID` 校验类型为 Embedding 并用于建临时知识库；否则沿用现有默认选择逻辑。
4. 快照中的 `dataset.sha256 / sample_count` 在创建时即填入，`config_hash` 包含生效参数 + 数据集哈希 + 模型快照；版本签名照旧。
5. 创建运行记录后，后台 goroutine 不再重新读盘，直接使用已加载的 `EvaluationDataset`。

### 服务端：Handler 与错误映射

`EvaluationRequest` 增加：

```go
type EvaluationRequest struct {
    DatasetID       string `json:"dataset_id"`
    KnowledgeBaseID string `json:"knowledge_base_id"`
    ChatModelID     string `json:"chat_id"`
    RerankModelID   string `json:"rerank_id"`
    EmbeddingModelID string `json:"embedding_id"`
    Params          *types.EvaluationParamsOverride `json:"params,omitempty"`
}
```

Handler 把请求映射为 `types.EvaluationOptions` 后调用服务；`ErrDatasetNotFound / ErrInvalidDataset / 参数校验错误` 映射为 HTTP 400，其余错误维持现有行为。

### SDK：client/evaluation.go

**职责：** 让 CLI 与真实服务端契约一致。

- 更新请求/响应类型与真实 JSON 字段对齐；
- `StartEvaluation` / `GetEvaluationResult` / `ListEvaluationRuns` 三个方法；
- 用 `httptest` 单测覆盖请求体与响应解析。

### CLI：eval 命令与 Runner

**命令树：**

```text
weknora eval run --config <path> [--wait=false] [--timeout 30m]
                 [--interval 2s] [--report-dir artifacts/evaluation]
                 [--dry-run] [--format json|text]
```

**模块：**

- `cli/cmd/eval/`：cobra 命令与参数校验。
- `cli/internal/evalrunner/config.go`：YAML 解析与本地校验。
- `cli/internal/evalrunner/runner.go`：模型解析、发起、轮询、退出码映射。
- `cli/internal/evalrunner/report.go`：JSON/Markdown 报告生成与原子写盘。

**Runner 流程：**

1. 解析并校验 YAML；失败 → `eval.config_error`（退出码 3）。
2. 通过 `cmdutil.Factory` 构造 SDK client（沿用 profile / env 凭据）。
3. `ListModels` 解析配置中的模型名称；解析失败 → 配置错误（3）。
4. `StartEvaluation` 发起任务；HTTP 错误/鉴权失败 → `eval.service_unavailable`（4）。
5. `--wait=false` 时打印任务 ID 并退出 0。
6. 等待模式轮询 `GetEvaluationResult`：
   - success → 继续；
   - failed / interrupted → 生成报告后 `eval.run_failed`（5）；
   - 超时 → 生成部分报告后 `eval.run_failed`（5）。
7. 成功路径调用 `ListEvaluationRuns` 找到同 `run_id` 记录，取 `config_hash` 与 `config_snapshot` 写入报告。
8. 写两份报告后退出 0。

**退出码映射：** 在 `cmdutil` 中新增：

```go
CodeEvalRegression        ErrorCode = "eval.regression"         // 2，WP3 使用
CodeEvalConfigError       ErrorCode = "eval.config_error"       // 3
CodeEvalServiceUnavailable ErrorCode = "eval.service_unavailable" // 4
CodeEvalRunFailed         ErrorCode = "eval.run_failed"         // 5
```

`ExitCode` 增加对应分支；`eval run` 的 Long/AgentHelp 明确写明这套独立退出码，避免与通用 CLI 矩阵混淆。

### Makefile 与样例文件

根目录 `Makefile` 增加：

```make
CONFIG ?= ./evaluation/configs/default.yaml
eval-baseline:
	$(MAKE) -C cli build
	./cli/bin/weknora eval run --config $(CONFIG) --wait --report-dir artifacts/evaluation
```

样例：

```text
evaluation/configs/default.yaml   # 默认配置：dataset_id=demo，模型留空走服务端默认
dataset/demo/*.parquet            # 10-30 个 QA 对的自定义数据集
dataset/build_demo.py             # 从源码生成 demo 数据集的 Python 脚本（一次性生成后提交 parquet）
```

## 模块交互

```
weknora eval run
  ├─ evalrunner.Config.Load/Validate
  ├─ cmdutil.Factory.Client()          // profile/env 凭据
  ├─ ListModels → 模型名称解析
  ├─ POST /api/v1/evaluation           // 服务端同步校验数据集并创建 pending run
  │     ├─ DatasetService.GetDatasetByID → 校验 + 哈希
  │     ├─ 默认参数 + overrides → ChatManage
  │     ├─ config_hash = sha256(params + dataset + models)
  │     └─ repository.Create(pending)
  ├─ [wait] 轮询 GET /evaluation?task_id=... → success/failed/interrupted
  ├─ GET /evaluation/runs → 同 run_id 的 config_hash/config_snapshot
  └─ report.go → evaluation-result.json + evaluation-report.md
```

## 文件组织

```text
docs/specs/evaluation-runner/
├── spec.md      — 已批准
├── plan.md      — 本文件
├── task.md      — 待生成
└── checklist.md — 待生成

internal/types/evaluation.go                 — EvaluationOptions / EvaluationParamsOverride / EvaluationDataset
internal/types/interfaces/evaluation.go      — DatasetService / EvaluationService 接口
internal/application/service/dataset.go      — 多数据集加载、校验、哈希
internal/application/service/dataset_test.go — 新增
internal/application/service/evaluation.go   — 参数合并、数据集预加载、config_hash 修正
internal/application/service/evaluation_persist_test.go — 适配接口
internal/handler/evaluation.go               — 新字段 + 400 错误映射
internal/handler/evaluation_test.go          — 新增/适配

client/evaluation.go                         — 对齐真实契约 + ListEvaluationRuns
client/evaluation_test.go                    — 新增

cli/cmd/eval/eval.go                         — weknora eval 父命令
cli/cmd/eval/run.go                          — weknora eval run
cli/cmd/eval/run_test.go                     — 新增
cli/cmd/root.go                              — 注册 eval
cli/cmd/dryrun_coverage_test.go              — 登记 eval run
cli/internal/cmdutil/errors.go               — eval.* 错误码
cli/internal/cmdutil/exit.go                 — 退出码映射
cli/internal/evalrunner/config.go            — YAML 配置
cli/internal/evalrunner/runner.go            — 运行编排
cli/internal/evalrunner/report.go            — 报告生成
cli/internal/evalrunner/runner_test.go       — 新增

evaluation/configs/default.yaml              — 默认配置
dataset/demo/*.parquet                       — 演示数据集（提交生成结果）
dataset/build_demo.py                        — 演示数据集生成脚本
Makefile                                     — eval-baseline 目标
```

## 技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| Runner 位置 | 现有 `cli/` 模块新增 `weknora eval run` + 根 Makefile 包装 | 复用 profile/env 鉴权、SDK、输出与错误框架；`make eval-baseline` 保持“一条命令” |
| 退出码 | 新增 `eval.*` 错误码映射到 WP2 的 0/2/3/4/5/130 | 满足 CI 语义；帮助文档明确独立约定，避免与通用 CLI 矩阵混淆 |
| 参数覆盖 | 请求新增可选 `params`（指针字段），服务端合并默认值 | 不破坏现有请求；Runner 与服务端共享同一套生效配置口径 |
| 数据集加载时机 | `Evaluation()` 同步加载并校验，后台复用已加载数据 | 坏数据集在创建运行前失败，Runner 才能可靠返回退出码 3 |
| 数据集哈希 | 5 个 Parquet 按主键排序后固定顺序序列化做 SHA-256 | 内容哈希稳定，不受 Parquet 写入元数据影响 |
| config_hash | 创建时即包含数据集哈希与生效参数 | 修正 WP1 中“dataset hash 后补但 config_hash 不变”的口径，保证同配置同哈希 |
| 报告一致来源 | Runner 通过 `GET /evaluation/runs` 读取同 run_id 的 `config_hash/config_snapshot` | 报告与数据库共享同一事实来源，无需直连数据库 |
| 数据集生成 | 提交 `dataset/demo/*.parquet` + Python 生成脚本 | 开箱可用且可重建，不依赖 LLM |

## Spec 覆盖自检

| Spec 需求 | Plan 归属 |
|-----------|-----------|
| F1 一条命令 / 非等待 | `make eval-baseline` + `weknora eval run --wait=false` |
| F2 YAML 配置 | `evalrunner.RunnerConfig` + 本地校验 |
| F3 按 ID 加载 / 路径安全 | DatasetService 改造 + ID 正则 |
| F4 校验与确定性哈希 | DatasetService 加载/校验/排序哈希 |
| F5 模型与参数覆盖 | `EvaluationOptions` + 参数合并 |
| F6 轮询 | Runner 等待循环 |
| F7 报告 | `EvalReport` + `report.go` |
| F8 退出码 | `eval.*` 错误码映射 |
| F9 演示数据集 | `dataset/demo` |
| N1-N7 | 服务端兼容、确定性、脱敏、错误分类、无新表、跨部署、测试 |
