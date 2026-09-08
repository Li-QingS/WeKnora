# 课题三最终实测证据 Plan

> 状态：已自主审查（2026-09-08）

## 架构概览

证据包分为三个独立工具。`parser-benchmark` 直接复用 `internal/infrastructure/docparser` 的 Reader 注册与真实适配器，避免绕过项目解析实现；`wiki-cache-report.py` 只读取脱敏后的 CSV 并生成汇总和图；CI workflow 通过显式输入选择正常 fixture 或在临时目录中生成 Recall 退化副本。

## 解析基线

### 输入

- `evaluation/parser_benchmark/fixture.pdf`：固定的合成 PDF，包含标题、普通事实、列表和表格。
- `evaluation/parser_benchmark/gold.json`：必须出现的事实短语及 Markdown 结构检查项。
- 8 个引擎名称固定为应用侧注册表的 8 项，不把 DocReader 内部的 `markitdown`/`opendataloader` 另算成第 9/10 个应用引擎。

### 执行

Go 命令连接 DocReader，并调用 `docparser.NewReader`。外部覆盖配置只从环境变量读取；WeKnora Cloud 凭据也只从环境读取。默认编译可报告 anydoc 未链接；带 `-tags anydoc` 的正式运行加载本地静态库。

### 指标

- `success_rate`：该引擎在固定 batch 上成功文档数/总文档数。
- `content_recall`：Gold 事实短语在规范化输出中的覆盖率。
- `structure_recall`：标题、列表、表格三类 Markdown 结构检查的覆盖率。
- `quality_score = 0.8 × content_recall + 0.2 × structure_recall`。
- 无真实输出时上述质量指标为 `null`，不记 0 分。

输出由命令直接写 JSON、CSV、Markdown、SVG，原始 Markdown 单独保存并写 SHA-256。

## Wiki 缓存对比

SQL 按一次成功的 `wiki_candidate_slug` 作为运行起点，以后一轮起点作为边界聚合模型调用。历史前四轮属于 Prompt 特定调整前基线；最终代码重新连续运行两轮作为 current cold/current warm。原始 CSV只包含 purpose、状态、时间、token、缓存计数、耗时、模型名和代码提交等非敏感字段。

报告脚本按运行和阶段计算：

```text
hit_rate = cache_read_tokens / (cache_read_tokens + cache_miss_tokens)
```

前后表同时报告中位数和逐轮值。SVG 使用同一纵轴比较命中率；若优化未稳定提升，结论按数据如实表述。

## CI 双场景

`workflow_dispatch` 新增 `evidence_scenario`：

- `pass`：比较正式 pass fixture 与 demo baseline，预期退出 0。
- `degraded`：复制 pass fixture到 runner 临时目录，用仓库脚本把 Recall 改为低于门槛的值，再执行同一比较器，预期工作流因退出码 2 失败。
- `auto`：保留 PR、schedule 和现有 secrets 分支的正式行为。

为能在当前无 `gh` 客户端的环境中触发证据运行，workflow 对专用证据分支的 push 读取一个非生产场景文件；完成 fail 后恢复 `pass` 并保留最终绿色提交。GitHub 公共 Actions 页面由 Chromium headless 截图，API/网页只读取公开运行信息。

## 文件组织

```text
cmd/parser-benchmark/main.go
evaluation/parser_benchmark/fixture.pdf
evaluation/parser_benchmark/gold.json
evaluation/scripts/wiki_cache_report.py
evaluation/scripts/set_recall.py
docs/evidence/parser-baseline-2026-09-08/*
docs/evidence/wiki-cache-2026-09-08/*
docs/evidence/ci-gate-2026-09-08/*
.github/workflows/rag-quality-gate.yml
docs/specs/evaluation-evidence/{spec,plan,task,checklist}.md
```

## 技术决策

| 决策 | 选择 | 理由 |
|---|---|---|
| 解析调用层 | 真实 `docparser.Reader` | 覆盖项目路由与适配器，避免独立库测试冒充产品结果 |
| 评分 | Gold 事实 + Markdown 结构规则 | 确定性、零模型成本、易解释 |
| 不可用引擎 | null 分数 + 原因 | 区分环境缺口与质量差 |
| 图形格式 | SVG，CI 页面另存 PNG | SVG 可由标准库生成且适合版本控制；PNG满足截图证据 |
| Wiki 数据源 | `model_call_records` 导出 | Token 与厂商缓存信息来自真实调用台账 |
| 退化注入 | runner 临时副本 | 不污染正式 fixture 和批准基线 |
