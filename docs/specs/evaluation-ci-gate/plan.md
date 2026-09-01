# CI 质量回归门禁（课题三 WP3）Plan

> 状态：待审批（2026-09-01）
> 上游文档：[spec.md](./spec.md)（已批准）

## 架构概览

WP3 在 WP2 的 Runner 与报告之上增加四块能力：

1. **基线文件**：把一次可信运行的 `config_hash`、数据集哈希、12 项指标基线和阈值保存为 YAML，作为“及格线”。
2. **比较器**：`weknora eval compare` 读取当前 `evaluation-result.json` 与基线，逐指标判定通过/退化，输出 delta，返回退出码 0/2/3。
3. **Runner 集成**：`weknora eval run --baseline <path>` 成功运行后自动比较，退化时以退出码 2 结束，并把对比结果写进报告。
4. **CI 门禁**：`make eval-ci` + `.github/workflows/rag-quality-gate.yml`，有真实服务 secrets 时跑完整评测，没有时跑比较器自测，保证 PR 不会静默跳过。

比较器只依赖结果文件与基线文件，不依赖服务端和数据库，因此可以在任何机器、任何 CI 上复现同一判定。

```
可信运行 evaluation-result.json
        │
        ▼
weknora eval baseline create --approved-by ...
        │
        ▼
evaluation/baselines/demo.yaml（config_hash + dataset.sha256 + 指标阈值）
        │
        ▼
weknora eval run --baseline demo.yaml
        ├─ 成功 → weknora eval compare → 全过 → exit 0
        └─ 成功 → 任一退化          → exit 2，报告含对比小节
```

## 核心数据结构

### Baseline（YAML）

```go
type Baseline struct {
    Version  int             `yaml:"version"`
    ConfigHash string        `yaml:"config_hash"`
    Dataset  BaselineDataset `yaml:"dataset"`
    Metrics  BaselineMetrics `yaml:"metrics"`
    Metadata BaselineMetadata `yaml:"metadata"`
}

type BaselineDataset struct {
    ID     string `yaml:"id"`
    SHA256 string `yaml:"sha256"`
}

type BaselineMetrics struct {
    Retrieval  RetrievalThresholds `yaml:"retrieval_metrics"`
    Generation GenerationThresholds `yaml:"generation_metrics"`
}

type MetricThreshold struct {
    Baseline        float64  `yaml:"baseline"`
    MinValue        *float64 `yaml:"min_value,omitempty"`
    MaxAbsoluteDrop *float64 `yaml:"max_absolute_drop,omitempty"`
    MaxRelativeDrop *float64 `yaml:"max_relative_drop,omitempty"`
}

type RetrievalThresholds struct {
    Precision MetricThreshold `yaml:"precision"`
    Recall    MetricThreshold `yaml:"recall"`
    NDCG3     MetricThreshold `yaml:"ndcg3"`
    NDCG10    MetricThreshold `yaml:"ndcg10"`
    MRR       MetricThreshold `yaml:"mrr"`
    MAP       MetricThreshold `yaml:"map"`
}

type GenerationThresholds struct {
    BLEU1  MetricThreshold `yaml:"bleu1"`
    BLEU2  MetricThreshold `yaml:"bleu2"`
    BLEU4  MetricThreshold `yaml:"bleu4"`
    ROUGE1 MetricThreshold `yaml:"rouge1"`
    ROUGE2 MetricThreshold `yaml:"rouge2"`
    ROUGEL MetricThreshold `yaml:"rougel"`
}

type BaselineMetadata struct {
    ApprovedCommit string `yaml:"approved_commit"`
    ApprovedBy     string `yaml:"approved_by"`
    CreatedAt      string `yaml:"created_at"`
    Note           string `yaml:"note,omitempty"`
}
```

### Comparison（比较结果）

```go
type Comparison struct {
    Pass          bool             `json:"pass"`
    ConfigHash    string           `json:"config_hash"`
    DatasetSHA256 string           `json:"dataset_sha256"`
    Items         []ComparisonItem `json:"items"`
    FailedCount   int              `json:"failed_count"`
}

type ComparisonItem struct {
    Group    string  `json:"group"`    // retrieval_metrics | generation_metrics
    Name     string  `json:"name"`     // recall, ndcg10, bleu1, ...
    Baseline float64 `json:"baseline"`
    Current  float64 `json:"current"`
    Delta    float64 `json:"delta"`    // baseline - current
    Pass     bool    `json:"pass"`
    Reason   string  `json:"reason,omitempty"`
}
```

### EvalReport 扩展

`EvalReport` 增加可选字段：

```go
type EvalReport struct {
    // ...现有字段不变...
    Comparison *Comparison `json:"comparison,omitempty"`
}
```

仅当运行时指定 `--baseline` 才填充，普通运行的报告结构保持不变。

## 模块设计

### evalrunner.Baseline

**职责：** 基线 YAML 的读写与校验。
**文件：** `cli/internal/evalrunner/baseline.go`
**对外接口：**

```go
func LoadBaseline(path string) (*Baseline, error)
func GenerateBaseline(result *EvalReport, opts BaselineGenOptions) (*Baseline, error)
func WriteBaseline(path string, baseline *Baseline, force bool) error
```

`BaselineGenOptions` 包含 `ApprovedBy`、`ApprovedCommit`、`Note`。生成时：

- `version=1`；
- `config_hash` / dataset 来自结果文件；
- 每个指标的 `baseline` 等于当前值，阈值规则留空，由人工填写或后续命令补充；
- 元数据必填 `approved_by` 与 `approved_commit`，缺失返回配置错误；
- 输出路径已存在且未传 `force` 时拒绝覆盖。

### evalrunner.Compare

**职责：** 当前结果与基线比较。
**文件：** `cli/internal/evalrunner/compare.go`
**对外接口：**

```go
func CompareResult(result *EvalReport, baseline *Baseline) (*Comparison, error)
```

比较规则：

1. 校验 `result.ConfigHash == baseline.ConfigHash` 且 `result.Dataset.SHA256 == baseline.Dataset.SHA256`；不匹配返回 `ErrBaselineMismatch`（映射退出码 3）。
2. 从 `result.Metric["retrieval_metrics"]` / `["generation_metrics"]` 取当前值；基线声明但结果缺失时返回配置错误。
3. 对每个规则逐项判定：
   - `min_value != nil && current < *min_value` → fail；
   - `max_absolute_drop != nil && baseline-current > *max_absolute_drop` → fail；
   - `max_relative_drop != nil && baseline != 0 && (baseline-current)/baseline > *max_relative_drop` → fail。
4. 任何规则未设置时该指标只记录 baseline/current，不参与失败判定。

### evalrunner.Runner 集成

`RunOptions` 增加 `Baseline string`；`RunResult` 增加 `Comparison *Comparison`。

成功路径流程：

1. `BuildReport`；
2. 若 `opts.Baseline != ""`，`LoadBaseline` + `CompareResult`；
3. `report.Comparison = comparison`；
4. `WriteReports`（Markdown 自动渲染对比小节）；
5. 若 comparison 不通过，返回 `fmt.Errorf("%w: ...", ErrRegression)`（映射退出码 2），否则返回成功。

新增 sentinel：

```go
var ErrRegression = errors.New("eval metrics regressed")
```

### CLI：weknora eval compare

**文件：** `cli/cmd/eval/compare.go`

```text
weknora eval compare --result <evaluation-result.json> --baseline <demo.yaml>
```

输出 `Comparison` 的 JSON envelope 或 text 摘要；退出码：

- 全过 → 0；
- 任一退化 → `eval.regression`（2）；
- 文件缺失、基线不匹配、结果缺指标 → `eval.config_error`（3）。

### CLI：weknora eval baseline create

**文件：** `cli/cmd/eval/baseline.go`

```text
weknora eval baseline create \
  --result artifacts/evaluation/evaluation-result.json \
  --output evaluation/baselines/demo.yaml \
  --approved-by <name> \
  --approved-commit <sha> \
  --note "Initial demo smoke baseline" \
  [--force]
```

生成后提示“阈值规则留空，请编辑 YAML 补充 min_value / max_absolute_drop / max_relative_drop”。

### CLI：weknora eval run 增加 --baseline

在 `run.go` 增加 `--baseline`，传入 `RunOptions.Baseline`；`mapEvalError` 增加 `ErrRegression → CodeEvalRegression`。

### Makefile

```make
CONFIG   ?= ./evaluation/configs/default.yaml
BASELINE ?= ./evaluation/baselines/demo.yaml

eval-ci:
	$(MAKE) -C cli build
	./cli/bin/weknora eval run --config $(CONFIG) --wait \
		--report-dir artifacts/evaluation --baseline $(BASELINE)

eval-baseline-generate:
	./cli/bin/weknora eval baseline create \
		--result artifacts/evaluation/evaluation-result.json \
		--output $(BASELINE) \
		--approved-by $(APPROVED_BY) \
		--approved-commit $(APPROVED_COMMIT)
```

### GitHub Actions

**文件：** `.github/workflows/rag-quality-gate.yml`

```yaml
on:
  pull_request:
    paths:
      - 'internal/**'
      - 'config/**'
      - 'dataset/**'
      - 'evaluation/**'
      - 'cli/cmd/eval/**'
      - 'cli/internal/evalrunner/**'
      - '.github/workflows/rag-quality-gate.yml'
  workflow_dispatch:
```

Job 步骤：

1. checkout + setup-go；
2. 检查 `WEKNORA_EVAL_HOST` / `WEKNORA_EVAL_TOKEN` secrets；
3. 有 secrets：执行 `make eval-ci`，env 注入两个变量；失败时上传 `artifacts/evaluation/**`；
4. 无 secrets：执行比较器自测：
   - `weknora eval compare` 对 `evaluation/fixtures/evaluation-result-pass.json` 期望退出码 0；
   - 对 `evaluation/fixtures/evaluation-result-degraded.json` 期望退出码 2。

## 演示与夹具

为了让“同一张考卷”的退化可复现，仓库内提交两类夹具：

- `evaluation/fixtures/evaluation-result-pass.json`：与基线同 `config_hash` 的正常结果；
- `evaluation/fixtures/evaluation-result-degraded.json`：与基线同 `config_hash`、同数据集哈希，但 Recall 明显下降的结果，模拟“代码改动导致检索退化”。

不提交“换数据集”的退化演示，因为换数据集会导致 `config_hash`/哈希不匹配，属于需要重建基线的情况，而不是门禁退化。

## 文件组织

```text
docs/specs/evaluation-ci-gate/
├── spec.md
├── plan.md
├── task.md
└── checklist.md

cli/internal/evalrunner/
├── baseline.go            — 基线读写与生成
├── baseline_test.go
├── compare.go             — 比较器
├── compare_test.go
├── report.go              — EvalReport.Comparison + Markdown 对比小节
├── runner.go              — Baseline 选项 + ErrRegression
└── runner_test.go

cli/cmd/eval/
├── compare.go             — weknora eval compare
├── compare_test.go
├── baseline.go            — weknora eval baseline create
├── baseline_test.go
├── run.go                 — --baseline
└── run_test.go

evaluation/
├── baselines/demo.yaml
├── configs/default.yaml
└── fixtures/
    ├── evaluation-result-pass.json
    └── evaluation-result-degraded.json

Makefile
.github/workflows/rag-quality-gate.yml
```

## 技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 比较器位置 | `cli/internal/evalrunner`，纯本地文件计算 | 不依赖服务端/数据库，任何 CI 都能复现同一判定 |
| 考卷标识 | `config_hash + dataset.sha256` 双字段校验 | 双保险，避免只靠一个哈希漏判；不匹配拒绝比较 |
| 阈值规则 | min_value / max_absolute_drop / max_relative_drop | 覆盖最低线、绝对下降、相对下降三种常见门禁；成本/耗时规则留给 WP4 |
| 退化演示 | 提交同 config_hash 的退化结果 JSON | 换数据集会触发哈希不匹配，模拟不了“同考卷退化”；真实代码退化由 CI 在 PR 中体现 |
| 公共 CI | secrets 提供真实评测服务；无 secrets 跑比较器自测 | 不引入 mock 模型服务，降低 WP3 复杂度；比较器逻辑仍被 CI 验证 |
| 基线更新 | 显式命令 + `--approved-by`/`--approved-commit` + `--force` | 基线变更可审计，避免悄悄放宽 |
| 报告扩展 | `EvalReport.Comparison` 可选字段 | 不破坏现有 JSON 结构，普通运行不产生该字段 |

## Spec 覆盖自检

| Spec 需求 | Plan 归属 |
|-----------|-----------|
| F1 基线文件 | `Baseline` YAML + `evaluation/baselines/demo.yaml` |
| F2 比较器 | `CompareResult` + `weknora eval compare` |
| F3 Runner 集成 | `RunOptions.Baseline` + `ErrRegression` |
| F4 基线生成/更新 | `baseline create` + `--force` + 批准信息 |
| F5 退化演示 | `evaluation/fixtures/evaluation-result-degraded.json` |
| F6 GitHub Actions | `rag-quality-gate.yml` + secrets 分支 |
| F7 报告联动 | `EvalReport.Comparison` + Markdown 对比小节 |
| N1-N6 | 纯文件比较、退出码复用、无敏感信息、无迁移、测试、跨部署 |
