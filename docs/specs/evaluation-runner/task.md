# 可复现评测 Runner（课题三 WP2）Tasks

> 状态：待审批（2026-09-01）
> 上游文档：[spec.md](./spec.md)（已批准）、[plan.md](./plan.md)（已批准）

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 修改 | `internal/types/evaluation.go` | EvaluationOptions / EvaluationParamsOverride / EvaluationDataset |
| 修改 | `internal/types/interfaces/evaluation.go` | DatasetService / EvaluationService 接口 |
| 修改 | `internal/application/service/dataset.go` | 多数据集加载、校验、哈希 |
| 新建 | `internal/application/service/dataset_test.go` | 数据集服务单测 |
| 修改 | `internal/application/service/evaluation.go` | 参数合并、数据集预加载、config_hash 修正 |
| 修改 | `internal/application/service/evaluation_persist_test.go` | 适配新接口与假实现 |
| 修改 | `internal/handler/evaluation.go` | 新字段 + 400 错误映射 |
| 新建 | `internal/handler/evaluation_test.go` | Handler 单测 |
| 修改 | `client/evaluation.go` | 对齐真实契约 + ListEvaluationRuns |
| 新建 | `client/evaluation_test.go` | SDK 单测 |
| 修改 | `cli/internal/cmdutil/errors.go` | eval.* 错误码 |
| 修改 | `cli/internal/cmdutil/exit.go` | 退出码映射 |
| 修改 | `cli/internal/cmdutil/exit_test.go` | 新增 eval 退出码用例 |
| 新建 | `cli/internal/evalrunner/config.go` | YAML 配置 |
| 新建 | `cli/internal/evalrunner/config_test.go` | 配置解析单测 |
| 新建 | `cli/internal/evalrunner/report.go` | 报告生成 |
| 新建 | `cli/internal/evalrunner/report_test.go` | 报告单测 |
| 新建 | `cli/internal/evalrunner/runner.go` | 运行编排 |
| 新建 | `cli/internal/evalrunner/runner_test.go` | Runner 单测 |
| 新建 | `cli/cmd/eval/eval.go` | `weknora eval` 父命令 |
| 新建 | `cli/cmd/eval/run.go` | `weknora eval run` |
| 新建 | `cli/cmd/eval/run_test.go` | 命令单测 |
| 修改 | `cli/cmd/root.go` | 注册 eval 命令 |
| 修改 | `cli/cmd/dryrun_coverage_test.go` | 登记 eval run |
| 修改 | `Makefile` | eval-baseline 目标 |
| 新建 | `evaluation/configs/default.yaml` | 默认配置 |
| 新建 | `dataset/build_demo.py` | 演示数据集生成脚本 |
| 新建 | `dataset/demo/*.parquet` | 演示数据集（提交生成结果） |
| 新建 | `docs/specs/evaluation-runner/progress-2026-09-01.md` | 实现与验收记录 |

## 执行顺序

```
T1 → T2 → T3 → T4 → T5 → T6 → T7 → T8 → T9 → T10 → T11 → T12
```

T1-T3 是服务端/SDK 契约改造，T4 是数据与配置，T5-T10 是 Runner 命令，T11 是 Makefile，T12 是端到端验收。

---

## T1: 新增评测选项与数据集类型

**文件：** `internal/types/evaluation.go`、`internal/types/interfaces/evaluation.go`
**依赖：** 无
**步骤：**
1. 在 `internal/types/evaluation.go` 增加 `EvaluationOptions`、`EvaluationParamsOverride`、`SummaryConfigOverride`，字段与 [plan.md](./plan.md) 的“核心数据结构”一致，JSON tag 使用 `json:"..."`。
2. 增加 `EvaluationDataset`，字段：`ID`、`SHA256`、`SampleCount`、`Pairs []*QAPair`。
3. 修改 `internal/types/interfaces/evaluation.go`：
   - `DatasetService.GetDatasetByID` 返回 `(*types.EvaluationDataset, error)`；
   - `EvaluationService.Evaluation` 签名改为 `Evaluation(ctx, opts *types.EvaluationOptions)`。

**验证：**
```bash
go test ./internal/types/...
```
> 说明：本任务完成后全仓编译可能暂时中断，T3 完成后统一回归。

---

## T2: DatasetService 多数据集加载、校验与哈希

**文件：** `internal/application/service/dataset.go`、新建 `dataset_test.go`
**依赖：** T1
**步骤：**
1. 保留现有 `TextInfo / RelsInfo / QaInfo` 与 `loadParquet[T]`。
2. 新增 `var ErrDatasetNotFound = errors.New("dataset not found")`、`var ErrInvalidDataset = errors.New("invalid dataset")`。
3. 新增 `validDatasetID(id string) bool`：仅允许 `^[A-Za-z0-9_-]+$`，空值按 `default`。
4. 新增 `datasetDir(id string) string`：`default` → `dataset/samples`，其他 → `dataset/<id>`。
5. 新增 `loadDatasetDir(dir string) (*types.EvaluationDataset, error)`：
   - 依次加载 `queries/corpus/answers/qrels/qas` 五个 Parquet；
   - 校验引用完整性：qas 的 qid 有查询与答案；qrels 的 qid 有查询、pid 有语料；每个查询至少有答案与 qrel；
   - 调用 `canonicalDatasetHash` 计算确定性 SHA-256；
   - 返回 `EvaluationDataset{ID, SHA256, SampleCount, Pairs}`。
6. 新增 `canonicalDatasetHash(queries, corpus, answers, qrels, qas)`：对每类行按主键排序（`id` 或 `qid,pid` / `qid,aid`），固定文件顺序序列化后 SHA-256。
7. 改造 `GetDatasetByID`：调用 `validDatasetID` + `loadDatasetDir`；不存在的目录返回 `ErrDatasetNotFound`，校验失败返回 `ErrInvalidDataset`。

**验证：**
```bash
go test ./internal/application/service/ -run 'TestDataset'
```
测试覆盖：非法 ID（含 `../`）、缺少文件、坏引用、哈希确定性、`default` 加载现有 `dataset/samples`。

---

## T3: EvaluationService 与 Handler 适配

**文件：** `internal/application/service/evaluation.go`、`internal/application/service/evaluation_persist_test.go`、`internal/handler/evaluation.go`、新建 `internal/handler/evaluation_test.go`
**依赖：** T1、T2
**步骤：**
1. 新增 `var ErrInvalidEvaluationParams = errors.New("invalid evaluation params")`。
2. 改造 `EvaluationService.Evaluation(ctx, opts *types.EvaluationOptions)`：
   - 用 `opts` 取代原来的 5 个字符串参数；
   - 调用 `dataset.GetDatasetByID(ctx, opts.DatasetID)`，得到 `*types.EvaluationDataset`；失败直接返回（不创建运行记录）；
   - `opts.EmbeddingModelID` 非空时用 `modelService.GetModelByID` 校验为 Embedding 模型；否则沿用现有默认选择；
   - 默认 `ChatManage` 构建完成后，用 `applyEvaluationParams(params, opts.Params)` 应用非空覆盖，并校验阈值范围与 TopK 正数；
   - 快照的 `Dataset.SHA256 / SampleCount` 创建时即填入，再计算 `config_hash`；
   - `EvalDataset(ctx, detail, knowledgeBaseID, loadedDataset)` 直接复用已加载数据集，不再重新读盘。
3. 调整 `EvalDataset` 签名并删除内部 `GetDatasetByID` 调用；`SetDatasetHash` 仍用相同值调用（幂等，保持 WP1 行为）。
4. Handler：`EvaluationRequest` 增加 `EmbeddingModelID` 与 `Params`；映射到 `types.EvaluationOptions`；`ErrDatasetNotFound / ErrInvalidDataset / ErrInvalidEvaluationParams` 返回 HTTP 400，其余保持现状。
5. 更新 `evaluation_persist_test.go` 的假 `evalDatasetService` 返回 `*types.EvaluationDataset`，并把所有 `svc.Evaluation(ctx, "dataset-1", "kb-1", "chat-1", "rerank-1")` 改为 `svc.Evaluation(ctx, &types.EvaluationOptions{...})`。

**验证：**
```bash
go test ./internal/application/service/ ./internal/handler/
go build ./...
```
重点断言：数据集失败不再创建 pending 记录；params 覆盖进入快照；同一数据集+配置两次运行 `config_hash` 相同；不含密钥。

---

## T4: 演示数据集与默认配置

**文件：** 新建 `dataset/build_demo.py`、`dataset/demo/*.parquet`、`evaluation/configs/default.yaml`
**依赖：** T2（供本地手工验证加载）
**步骤：**
1. 编写 `dataset/build_demo.py`：确定性生成 15 个 QA 对（中文，围绕 RAG / 向量检索 / 重排 / WeKnora 概念），输出 `queries/corpus/answers/qrels/qas` 五个 Parquet 到 `dataset/demo/`。
2. 运行脚本生成 `dataset/demo/*.parquet` 并提交生成结果（不要依赖脚本运行时环境）。
3. 新建 `evaluation/configs/default.yaml`：
   - `dataset_id: demo`；
   - `models` 三项留空；
   - `retrieval` 与 `generation` 填一组合理默认值；
   - `execution.wait: true`、`timeout: 30m`、`interval: 2s`、`report_dir: artifacts/evaluation`。

**验证：**
```bash
python3 dataset/build_demo.py
python3 - <<'PY'
import pandas as pd
for f in ["queries","corpus","answers","qrels","qas"]:
    print(f, len(pd.read_parquet(f"dataset/demo/{f}.parquet")))
PY
```
期望：queries 15 条，其余文件数量与引用一致。

---

## T5: Client SDK 评测契约对齐

**文件：** `client/evaluation.go`、新建 `client/evaluation_test.go`
**依赖：** 无（与服务端 T3 并行语义一致）
**步骤：**
1. 重写 `client/evaluation.go` 类型：`EvaluationTask`、`EvaluationDetail`、`EvaluationRun`、`EvaluationRequest` 与真实 JSON 字段对齐（见 plan）。
2. `StartEvaluation(ctx, *EvaluationRequest)` 返回 `*EvaluationDetail`。
3. `GetEvaluationResult(ctx, taskID)` 返回 `*EvaluationDetail`。
4. 新增 `ListEvaluationRuns(ctx, page, pageSize) ([]EvaluationRun, int, error)`，路径 `GET /api/v1/evaluation/runs`。
5. 新建 `evaluation_test.go`：用 `httptest.Server` 覆盖三个方法的请求路径、查询参数与响应解析，以及非 2xx 返回 `*APIError`。

**验证：**
```bash
cd client && go test ./...
```

---

## T6: eval 错误码与退出码映射

**文件：** `cli/internal/cmdutil/errors.go`、`cli/internal/cmdutil/exit.go`、`cli/internal/cmdutil/exit_test.go`
**依赖：** 无
**步骤：**
1. 在 `errors.go` 增加：
   - `CodeEvalRegression = "eval.regression"`；
   - `CodeEvalConfigError = "eval.config_error"`；
   - `CodeEvalServiceUnavailable = "eval.service_unavailable"`；
   - `CodeEvalRunFailed = "eval.run_failed"`。
2. 在 `ExitCode` 增加分支：regression→2、config_error→3、service_unavailable→4、run_failed→5。
3. 在 `exit_test.go` 的用例表中加入四个用例。

**验证：**
```bash
cd cli && go test ./internal/cmdutil/ -run 'TestExitCode'
```

---

## T7: evalrunner 配置解析

**文件：** 新建 `cli/internal/evalrunner/config.go`、`cli/internal/evalrunner/config_test.go`
**依赖：** 无
**步骤：**
1. 定义 `RunnerConfig` 及子结构（见 plan）。
2. `LoadConfig(path string) (*RunnerConfig, error)`：读取 YAML，支持字段缺省；`timeout`/`interval` 用 `time.ParseDuration` 解析。
3. `Validate()`：
   - `dataset_id` 非空且匹配 `^[A-Za-z0-9_-]+$`；
   - `execution.report_dir` 缺省为 `artifacts/evaluation`；
   - 阈值在 0-1、TopK > 0；
   - `timeout > 0`、`interval > 0`。
4. 测试：合法配置、缺省值、非法数据集 ID、非法阈值、坏 YAML。

**验证：**
```bash
cd cli && go test ./internal/evalrunner/ -run Config
```

---

## T8: evalrunner 报告生成

**文件：** 新建 `cli/internal/evalrunner/report.go`、`cli/internal/evalrunner/report_test.go`
**依赖：** T7（复用 RunnerConfig）
**步骤：**
1. 定义 `EvalReport` 与 `DatasetReport / ModelReport`（见 plan），JSON tag 固定。
2. `BuildReport(...)`：从运行详情、运行记录、配置、SDK 版本信息组装报告。
3. `WriteReports(dir string, report *EvalReport) ([]string, error)`：先写临时文件再 rename，保证原子性；生成 `evaluation-result.json` 与 `evaluation-report.md`。
4. Markdown 至少包含：run_id、config_hash、状态、指标表、数据集哈希、复现命令。
5. 测试：写临时目录后断言两个文件存在、JSON 可解析、字段齐全、Markdown 含关键字段。

**验证：**
```bash
cd cli && go test ./internal/evalrunner/ -run Report
```

---

## T9: evalrunner 运行编排

**文件：** 新建 `cli/internal/evalrunner/runner.go`、`cli/internal/evalrunner/runner_test.go`
**依赖：** T6、T7、T8、T5（SDK 类型）
**步骤：**
1. 定义窄接口（由 `*sdk.Client` 鸭子类型满足）：
   ```go
   type EvalClient interface {
       ListModels(ctx context.Context) ([]sdk.Model, error)
       StartEvaluation(ctx context.Context, req *sdk.EvaluationRequest) (*sdk.EvaluationDetail, error)
       GetEvaluationResult(ctx context.Context, taskID string) (*sdk.EvaluationDetail, error)
       ListEvaluationRuns(ctx context.Context, page, pageSize int) ([]sdk.EvaluationRun, int, error)
   }
   ```
2. `Run(ctx, cfg *RunnerConfig, cli EvalClient, opts RunOptions) (*RunResult, error)`：
   - 模型名称解析（复用 `cmdutil.ResolveModelRef` 语义，失败映射为 `eval.config_error`）；
   - `StartEvaluation`，HTTP/鉴权错误映射为 `eval.service_unavailable`；
   - 非等待模式返回任务 ID；
   - 轮询直到 success/failed/interrupted 或超时；终端失败/超时返回 `eval.run_failed`；
   - 成功时 `ListEvaluationRuns` 找到同 run_id 记录，取 `config_hash/config_snapshot`。
3. 错误分类辅助函数：`classifyEvalError(err)` 把通用错误映射为四个 `eval.*` 错误码。
4. 测试：用内存 fake 客户端覆盖成功、failed、interrupted、超时、模型解析失败、服务错误、非等待模式。

**验证：**
```bash
cd cli && go test ./internal/evalrunner/ -run Runner
```

---

## T10: weknora eval run 命令

**文件：** 新建 `cli/cmd/eval/eval.go`、`cli/cmd/eval/run.go`、`cli/cmd/eval/run_test.go`，修改 `cli/cmd/root.go`、`cli/cmd/dryrun_coverage_test.go`
**依赖：** T6、T7、T8、T9
**步骤：**
1. 新建父命令 `weknora eval`，子命令 `run`。
2. `run` 注册：
   - `--config`（必填）、`--wait`（默认 true）、`--timeout`（默认 30m）、`--interval`（默认 2s）、`--report-dir`（默认 artifacts/evaluation）、`--dry-run`；
   - `cmdutil.AddFormatFlag(cmd)`、`cmdutil.SetAgentHelp(...)`；
   - Long 中写明 WP2 退出码约定（0/2/3/4/5/130）。
3. `--dry-run` 走 `cmdutil.HandleDryRun`，不调用 SDK、不写报告。
4. `RunE`：`evalrunner.LoadConfig` → `f.Client()` → `evalrunner.Run` → `evalrunner.WriteReports` → 输出 envelope（JSON）或文本（text）。
5. 在 `root.go` 注册 `evalcmd.NewCmd(f)`；在 `dryRunExpectation` 登记 `"eval run": true`。
6. 测试：`--config` 缺失、坏配置退出码、dry-run 不写盘、成功输出、失败退出码。

**验证：**
```bash
cd cli && go test ./cmd/...
go build ./...
```

---

## T11: 根目录 eval-baseline 目标

**文件：** `Makefile`
**依赖：** T10
**步骤：**
1. 增加变量 `CONFIG ?= ./evaluation/configs/default.yaml`。
2. 增加目标：
   ```make
   eval-baseline:
   	$(MAKE) -C cli build
   	./cli/bin/weknora eval run --config $(CONFIG) --wait --report-dir artifacts/evaluation
   ```
3. 加入 `.PHONY`。

**验证：**
```bash
make -n eval-baseline CONFIG=./evaluation/configs/default.yaml
```
输出应显示 `weknora eval run --config ... --wait --report-dir artifacts/evaluation`。

---

## T12: 端到端验收与进展记录

**文件：** 新建 `docs/specs/evaluation-runner/progress-2026-09-01.md`
**依赖：** T1-T11
**步骤：**
1. 构建并启动 Lite 服务（沿用 `make build-lite SKIP_FRONTEND=1` + 本地 `data/e2e.db`），配置好 `WEKNORA_HOST / WEKNORA_TOKEN`（或 CLI profile）。
2. 运行 `make eval-baseline CONFIG=./evaluation/configs/default.yaml`：
   - 退出码 0；
   - 生成两个报告；
   - `evaluation-result.json` 的 run_id 在服务端可查到，config_hash 一致。
3. 验证错误路径：
   - 不存在的配置/非法 dataset_id → 退出码 3；
   - 服务停止 → 退出码 4；
   - 失败/超时任务 → 退出码 5（可用合成失败运行或注入超时验证）。
4. 记录命令、输出摘要、数据库核对结果到 progress 文档。

**验证：**
```bash
make eval-baseline CONFIG=./evaluation/configs/default.yaml
echo $?
cat artifacts/evaluation/evaluation-result.json
```

---

## 最终回归

全部任务完成后运行：

```bash
go build ./...
go test ./internal/...
cd client && go test ./...
cd cli && go test ./... && go vet ./...
```

并在 `progress-2026-09-01.md` 记录最终回归结果。
