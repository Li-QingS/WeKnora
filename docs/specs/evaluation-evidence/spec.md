# 课题三最终实测证据 Spec

> 状态：已自主审查（用户已授权无需逐阶段审批）
> 日期：2026-09-08

## 背景

课题三已有可复现 RAG 评测、质量门禁、模型调用台账、Embedding 缓存和 Wiki Prompt 缓存优化，但还缺三组适合汇报且可复查的最终证据：8 个解析引擎在同一批输入上的横向基线、Wiki 缓存优化前后数据图，以及真实 GitHub Actions 的通过与 Recall 退化失败记录。

## 目标

- 用固定输入和确定性规则比较课题材料所指的 8 个应用侧解析引擎。
- 从真实 `model_call_records` 和新运行中固化 Wiki Prompt 缓存前后数据。
- 在 GitHub Actions 中真实产生一次通过和一次人为降低 Recall 的失败，并保存截图与日志。
- 所有结果可由仓库中的命令重新生成，不写入密钥或私人文档正文。

## 功能需求

- **F1：解析引擎基线。** 对 `builtin`、`simple`、`anydoc`、`weknoracloud`、`mineru`、`mineru_cloud`、`paddleocr_vl`、`paddleocr_vl_cloud` 使用同一份版本化 PDF fixture。逐引擎记录可用性、成功状态、耗时、内容事实 Recall、Markdown 结构 Recall 和综合分。
- **F2：诚实处理不可用引擎。** 缺少服务、构建标签或凭据时记录 `unavailable` 和原因，质量分显示 N/A，不用 mock 输出冒充真实结果。
- **F3：解析报告。** 生成 JSON、CSV、Markdown 表格和 SVG 图；记录输入 SHA-256、代码提交和评分公式，并保留成功引擎的原始 Markdown 输出。
- **F4：Wiki 缓存对比。** 固化实际调用的逐轮、逐 purpose 数据，至少展示 `wiki_page_modify`、`wiki_chunk_citation` 和全部 Wiki 调用的 cache-read、cache-miss、命中率、调用次数与耗时。
- **F5：前后口径。** “优化前”使用 Prompt 特定调整前的相同知识库/文档运行，“优化后”使用最终代码的连续冷、热运行；报告同时给出逐轮数据和聚合值，不隐藏模型缓存波动。
- **F6：Wiki 图表。** 生成原始 CSV、汇总 JSON、Markdown 表格和 SVG 对比图，写明知识库、文档、模型、提交和查询时间边界。
- **F7：真实 CI 双场景。** 同一 GitHub Actions workflow 支持显式 `pass` 与 `degraded` 证据场景。`degraded` 场景只在 runner 工作区复制结果并降低 Recall，不修改已批准基线或正式 fixture。
- **F8：CI 证据。** 保存两次 GitHub Actions 的 run URL、run ID、提交、结论、关键比较输出和 PNG 截图；失败必须显示 Recall 的 baseline/current/delta/threshold 以及非零退出。
- **F9：恢复绿色状态。** 失败证据运行完成后，开发分支最终提交必须恢复正常门禁配置，不留下故意退化的数据。

## 非功能需求

- **N1：可复现。** fixture、Gold、评分和图表生成均版本化；报告记录 SHA-256 和 commit。
- **N2：确定性。** 解析评分和 Recall 注入不调用生成模型；同一输入输出得到相同分数。
- **N3：安全。** 报告、日志、截图和提交中不出现 API Key、Token、密码或模型 Prompt 正文。
- **N4：可解释。** 不把环境可用性和解析忠实度混成一个数字；聚合公式、N/A 和失败原因清楚展示。
- **N5：低侵入。** 证据工具不修改线上业务表结构，不把 Wiki 质量评测接入门禁。

## 不做的事

- 不代购或伪造 MinerU、PaddleOCR-VL、WeKnora Cloud 凭据。
- 不用 LLM 裁判解析质量。
- 不把人为退化结果提交为正式基线。
- 不宣称单一小 fixture 能代表所有语言、扫描件和复杂版面。

## 验收标准

- **AC1：** 腾讯 `upstream/main` 是当前分支祖先，ahead/behind 的 behind 为 0。
- **AC2：** 一条命令产生 8 行解析结果及 JSON、CSV、Markdown、SVG；每行有真实状态或明确不可用原因。
- **AC3：** 至少所有当前环境可运行的解析引擎完成真实解析并保留原始输出，fixture 哈希与报告一致。
- **AC4：** Wiki 前后原始数据、汇总表和 SVG 存在，数字能由 SQL 导出记录重新计算。
- **AC5：** 最新代码至少完成连续两次同文档 Wiki 生成，能区分冷、热运行。
- **AC6：** GitHub Actions 有一个 `success` run 和一个由 Recall 降低触发的 `failure` run。
- **AC7：** 两个 CI run 均有 URL、日志文本和 PNG 截图，失败截图可读出 Recall 退化。
- **AC8：** 最终分支无故意退化内容，相关单测、格式检查和报告一致性检查通过。
