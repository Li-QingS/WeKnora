# WP2 可复现 Runner 范围记录

> 状态：待细化（2026-08-30）
> 上游：课题三完整实施计划第 3.4、3.8 节
> 依赖：WP1 评测持久化（已完成）

## 目标

在 WP1 已可用的持久化评测 API 之上，提供一个“干净环境一条命令得到可复现结果”的 Runner：

```bash
make eval-baseline CONFIG=./evaluation/configs/default.yaml
```

Runner 不重写评测链路，只负责：读取配置、发起评测、等待结果、生成报告、返回可靠退出码。

## 范围

### 1. Runner 入口

- 提供 `make eval-baseline` 或 `weknora-eval run` 形式的命令入口
- 支持 `--config`、`--wait`、`--report-dir`
- 支持在非等待模式返回任务 ID，由外部继续轮询

### 2. YAML 配置

- 新建 `evaluation/configs/`
- 配置文件至少包含：
  - dataset_id
  - chat / embedding / rerank 模型
  - 检索参数：TopK、阈值
  - 生成参数：temperature、max_tokens
  - 执行参数：并发、超时
  - 报告输出目录

### 3. 数据集支持

- 支持按 `dataset_id` 加载不同数据集
- 校验 5 个 Parquet 文件的 schema
- 计算数据集 SHA-256
- 准备一个自定义小数据集，建议 10-30 个 QA 对

### 4. 运行编排

- 读取配置 → 校验 → 调用 `POST /evaluation`
- 轮询 `GET /evaluation?task_id=...`
- 获取最终指标后生成报告
- 退出码约定：
  - 0：成功
  - 2：指标退化
  - 3：配置或数据集错误
  - 4：服务或模型不可用
  - 5：运行失败或超时

### 5. 报告输出

- `evaluation-result.json`：完整配置快照、版本、指标、样本/错误摘要、哈希
- `evaluation-report.md`：人可读摘要、复现命令
- 可选：JUnit XML
- 数据库记录与文件使用同一 `run_id / config_hash`

### 6. 可复现签名

- 配置 hash、数据集 hash、模型信息
- 代码版本、git commit、dirty 标记
- WP1 已落库，Runner 负责在报告与配置文件中暴露

## 明确不做

- 阈值比较器与 CI workflow：属于 WP3
- 模型调用台账与费用估算：属于 WP4
- Embedding 缓存：属于 WP5
- Prompt 缓存实验：属于 WP6
- 页面与演示材料：属于 WP7

## 验收标准

```text
一条命令：
make eval-baseline CONFIG=./evaluation/configs/default.yaml

产出：
artifacts/evaluation/evaluation-result.json
artifacts/evaluation/evaluation-report.md

数据库中存在同一 run_id 与 config_hash。

退出码可按约定区分成功、配置错误、服务不可用、运行失败。
```

## 文件组织建议

```text
evaluation/
├── configs/
│   └── default.yaml
├── baselines/            # WP3 使用
├── datasets/             # 自定义数据集
└── artifacts/
    └── evaluation/
        ├── evaluation-result.json
        └── evaluation-report.md
```

## 下一步

1. 按数据集任务准备自定义 Parquet 数据集
2. 为 Runner 配置结构、数据集加载与命令入口生成详细任务
3. 完成后进入 WP3：基线比较器与 CI 门禁
