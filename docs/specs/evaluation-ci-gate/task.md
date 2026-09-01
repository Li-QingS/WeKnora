# CI 质量回归门禁（课题三 WP3）Tasks

> 状态：待审批（2026-09-01）
> 上游文档：[spec.md](./spec.md)（已批准）、[plan.md](./plan.md)（已批准）

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 新建 | `cli/internal/evalrunner/baseline.go` | Baseline YAML 读写与生成 |
| 新建 | `cli/internal/evalrunner/baseline_test.go` | 基线单测 |
| 新建 | `cli/internal/evalrunner/compare.go` | 比较器 |
| 新建 | `cli/internal/evalrunner/compare_test.go` | 比较器单测 |
| 修改 | `cli/internal/evalrunner/report.go` | EvalReport.Comparison + Markdown 对比小节 |
| 修改 | `cli/internal/evalrunner/report_test.go` | 报告对比渲染单测 |
| 修改 | `cli/internal/evalrunner/runner.go` | --baseline 集成 + ErrRegression |
| 修改 | `cli/internal/evalrunner/runner_test.go` | Runner 基线场景单测 |
| 新建 | `cli/cmd/eval/compare.go` | `weknora eval compare` |
| 新建 | `cli/cmd/eval/compare_test.go` | compare 命令单测 |
| 新建 | `cli/cmd/eval/baseline.go` | `weknora eval baseline create` |
| 新建 | `cli/cmd/eval/baseline_test.go` | baseline 命令单测 |
| 修改 | `cli/cmd/eval/eval.go` | 注册 compare / baseline |
| 修改 | `cli/cmd/eval/run.go` | 增加 `--baseline` |
| 修改 | `cli/cmd/eval/run_test.go` | 增加 baseline 集成用例 |
| 修改 | `cli/cmd/dryrun_coverage_test.go` | 登记 eval compare / baseline create |
| 新建 | `evaluation/baselines/demo.yaml` | demo 基线 |
| 新建 | `evaluation/fixtures/evaluation-result-pass.json` | 通过夹具 |
| 新建 | `evaluation/fixtures/evaluation-result-degraded.json` | 退化夹具 |
| 修改 | `Makefile` | eval-ci / eval-baseline-generate |
| 新建 | `.github/workflows/rag-quality-gate.yml` | CI 质量门禁 |
| 新建 | `docs/specs/evaluation-ci-gate/progress-2026-09-01.md` | 实现与验收记录 |

## 执行顺序

```
T1 → T2 → T3 → T4 → T5 → T6 → T7 → T8 → T9
```

T1/T2 是核心库，T3/T4/T5 是命令与 Runner 集成，T6 是数据文件，T7/T8 是入口与 CI，T9 是端到端验收。

---

## T1: Baseline 类型、读取与生成

**文件：** 新建 `cli/internal/evalrunner/baseline.go`、`cli/internal/evalrunner/baseline_test.go`
**依赖：** 无
**步骤：**
1. 按 [plan.md](./plan.md) 的“核心数据结构”定义 `Baseline / BaselineDataset / BaselineMetrics / MetricThreshold / RetrievalThresholds / GenerationThresholds / BaselineMetadata / BaselineGenOptions`，YAML tag 对齐。
2. `LoadBaseline(path string) (*Baseline, error)`：读取并解析 YAML；`version != 1`、`config_hash` 为空、dataset 哈希为空时报错。
3. `GenerateBaseline(result *EvalReport, opts BaselineGenOptions) (*Baseline, error)`：
   - 从 result 取 `config_hash`、dataset 信息、12 项指标当前值作为 `baseline`；
   - 阈值规则留空；
   - 缺少 `approved_by` 或 `approved_commit` 返回错误。
4. `WriteBaseline(path string, baseline *Baseline, force bool) error`：文件已存在且 `force=false` 时拒绝覆盖。
5. 单测：生成、写入、拒绝覆盖、缺失批准信息、坏 YAML。

**验证：**
```bash
cd cli && go test ./internal/evalrunner/ -run Baseline
```

---

## T2: 比较器

**文件：** 新建 `cli/internal/evalrunner/compare.go`、`cli/internal/evalrunner/compare_test.go`
**依赖：** T1
**步骤：**
1. 定义 `Comparison / ComparisonItem` 与 `ErrBaselineMismatch`。
2. `CompareResult(result *EvalReport, baseline *Baseline) (*Comparison, error)`：
   - 校验 `config_hash` 与 `dataset.sha256`，不匹配返回 `ErrBaselineMismatch`；
   - 从 result 的 retrieval/generation metric 取当前值，缺失返回配置错误；
   - 按 `min_value`、`max_absolute_drop`、`max_relative_drop` 判定；
   - 填充 delta、reason、failed_count。
3. 单测：全过、绝对下限失败、绝对下降失败、相对下降失败、哈希不匹配、指标缺失。

**验证：**
```bash
cd cli && go test ./internal/evalrunner/ -run Compare
```

---

## T3: weknora eval compare 命令

**文件：** 新建 `cli/cmd/eval/compare.go`、`cli/cmd/eval/compare_test.go`，修改 `cli/cmd/eval/eval.go`、`cli/cmd/dryrun_coverage_test.go`
**依赖：** T2
**步骤：**
1. 新建 `weknora eval compare`：
   - `--result`（必填）、`--baseline`（必填）；
   - `cmdutil.AddFormatFlag`、`cmdutil.SetAgentHelp`；
   - 输出 `Comparison` envelope；text 模式输出表格。
2. 退出码：全过 0；任一退化 `CodeEvalRegression`（2）；文件缺失/基线不匹配/指标缺失 `CodeEvalConfigError`（3）。
3. 在 `eval.go` 注册；在 `dryRunExpectation` 登记 `"eval compare": false`。
4. 单测：通过输出、退化输出、缺失参数、不匹配返回 3。

**验证：**
```bash
cd cli && go test ./cmd/eval/ -run Compare
```

---

## T4: weknora eval baseline create 命令

**文件：** 新建 `cli/cmd/eval/baseline.go`、`cli/cmd/eval/baseline_test.go`，修改 `cli/cmd/eval/eval.go`、`cli/cmd/dryrun_coverage_test.go`
**依赖：** T1
**步骤：**
1. 新建 `weknora eval baseline create`：
   - `--result`、`--output` 必填；
   - `--approved-by`、`--approved-commit` 必填；
   - `--note` 可选、`--force` 可选、`--dry-run` 必填；
   - 生成后提示“阈值规则留空，请编辑 YAML 补充”。
2. 在 `eval.go` 注册；在 `dryRunExpectation` 登记 `"eval baseline create": true`。
3. 单测：生成成功、缺批准信息、拒绝覆盖、`--force` 覆盖、dry-run 不写文件。

**验证：**
```bash
cd cli && go test ./cmd/eval/ -run Baseline
```

---

## T5: Runner 集成与报告对比小节

**文件：** 修改 `cli/internal/evalrunner/report.go`、`report_test.go`、`runner.go`、`runner_test.go`、`cli/cmd/eval/run.go`、`run_test.go`
**依赖：** T2、T3
**步骤：**
1. `EvalReport` 增加 `Comparison *Comparison `json:"comparison,omitempty"``；Markdown 在存在时渲染“Baseline comparison”表。
2. `RunOptions` 增加 `Baseline string`；`RunResult` 增加 `Comparison *Comparison`。
3. `finishRun`：`BuildReport` 后若 `opts.Baseline != ""`，加载基线并比较，`report.Comparison = comparison`，再 `WriteReports`；比较失败返回 `ErrRegression`。
4. 新增 `ErrRegression`；`mapEvalError` 增加 `ErrRegression → CodeEvalRegression`。
5. `weknora eval run` 增加 `--baseline` 并传入 `RunOptions`。
6. 单测：`Run` 带基线通过返回 0；带基线退化返回 `ErrRegression` 且报告包含 comparison；无 baseline 报告不含 comparison。

**验证：**
```bash
cd cli && go test ./internal/evalrunner/ ./cmd/eval/
```

---

## T6: demo 基线与夹具

**文件：** 新建 `evaluation/baselines/demo.yaml`、`evaluation/fixtures/evaluation-result-pass.json`、`evaluation/fixtures/evaluation-result-degraded.json`
**依赖：** T5（可先手工构造）
**步骤：**
1. 以当前真实 demo 运行结果为基础生成 pass 夹具，去掉时间相关字段波动并固定 `generated_at`。
2. 复制 pass 夹具生成 degraded 夹具：保持 `run_id/config_hash/dataset` 不变，把 `retrieval_metrics` 的 recall、ndcg10、precision 等降到明显低于基线。
3. 编写 `evaluation/baselines/demo.yaml`：
   - `config_hash` 与 dataset 哈希取自 pass 夹具；
   - retrieval 指标设置 `min_value`/`max_absolute_drop`；
   - generation 指标设置 `max_relative_drop`；
   - metadata 填 `approved_commit`、`approved_by`、`created_at`、`note`。

**验证：**
```bash
./cli/bin/weknora eval compare --result evaluation/fixtures/evaluation-result-pass.json --baseline evaluation/baselines/demo.yaml
echo $?   # 0
./cli/bin/weknora eval compare --result evaluation/fixtures/evaluation-result-degraded.json --baseline evaluation/baselines/demo.yaml
echo $?   # 2
```

---

## T7: Makefile 目标

**文件：** `Makefile`
**依赖：** T5、T6
**步骤：**
1. 增加 `BASELINE ?= ./evaluation/baselines/demo.yaml`。
2. 增加：
   ```make
   eval-ci:
   	$(MAKE) -C cli build
   	./cli/bin/weknora eval run --config $(CONFIG) --wait \
   		--report-dir artifacts/evaluation --baseline $(BASELINE)
   ```
3. 增加 `eval-baseline-generate`（调用 `weknora eval baseline create`，传 `APPROVED_BY`/`APPROVED_COMMIT`）。
4. 加入 `.PHONY`。

**验证：**
```bash
make -n eval-ci CONFIG=./evaluation/configs/default.yaml BASELINE=./evaluation/baselines/demo.yaml
make -n eval-baseline-generate APPROVED_BY=lqs APPROVED_COMMIT=657466d8
```

---

## T8: GitHub Actions workflow

**文件：** 新建 `.github/workflows/rag-quality-gate.yml`
**依赖：** T7
**步骤：**
1. 触发：PR paths + `workflow_dispatch`，路径包含 `internal/**`、`config/**`、`dataset/**`、`migrations/**`、`evaluation/**`、`cli/cmd/eval/**`、`cli/internal/evalrunner/**`、`Makefile`、workflow 自身。
2. 步骤：
   - checkout + setup-go（1.26，缓存 `cli/go.sum`）；
   - `make -C cli build`；
   - 检查 `WEKNORA_EVAL_HOST` / `WEKNORA_EVAL_TOKEN` secrets；
   - 有 secrets：`make eval-ci`，env 注入两个变量；
   - 无 secrets：跑比较器自测（pass 期望 0，degraded 期望 2）；
   - `if: always()` 上传 `artifacts/evaluation/**`。
3. 参照现有 `cli-e2e.yml` 的 secrets guard 风格。

**验证：**
```bash
git diff --check
```
本地无法触发 GitHub Actions，workflow 通过 YAML 检查与自测步骤保证可读。

---

## T9: 端到端验收与进展记录

**文件：** 新建 `docs/specs/evaluation-ci-gate/progress-2026-09-01.md`
**依赖：** T1-T8
**步骤：**
1. 构建 CLI，运行比较器自测：pass 退出码 0，degraded 退出码 2。
2. 启动 Lite 服务，真实运行 `make eval-ci CONFIG=./evaluation/configs/default.yaml BASELINE=./evaluation/baselines/demo.yaml`，期望退出码 0，报告含“Baseline comparison”小节。
3. 人为验证退化路径：用 degraded 夹具或临时降低阈值跑 `weknora eval run --baseline`，期望退出码 2。
4. 记录命令、退出码、报告内容与数据库核对结果到 progress 文档。
5. 全量回归：`go build ./...`、`go test ./internal/...`、client 测试、CLI 测试 + vet。

**验证：**
```bash
make eval-ci CONFIG=./evaluation/configs/default.yaml BASELINE=./evaluation/baselines/demo.yaml
echo $?
grep -n "Baseline comparison" artifacts/evaluation/evaluation-report.md
```

---

## 最终回归

```bash
go build ./...
go test ./internal/...
cd client && go test ./...
cd cli && go test ./... && go vet ./...
```

并在 `progress-2026-09-01.md` 记录结果。
