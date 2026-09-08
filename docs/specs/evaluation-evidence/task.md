# 课题三最终实测证据 Tasks

> 状态：已自主审查（2026-09-08）

1. **同步上游。** fetch `upstream/main`，合并并验证祖先关系和 behind=0。
2. **建立固定解析考卷。** 生成合成 PDF 和 Gold，计算并记录 SHA-256。
3. **实现解析 Runner。** 接线 8 个应用侧引擎、可用性判断、超时、原始输出和确定性评分。
4. **实现解析报告。** 生成 JSON/CSV/Markdown/SVG，测试评分、null 和转义行为。
5. **运行解析基线。** 构建 anydoc 后运行；核对每个引擎的状态、输出和数字。
6. **导出 Wiki 历史基线。** 从 PostgreSQL 按运行边界导出原始 CSV，禁止敏感字段。
7. **运行最终 Wiki 冷/热轮。** 最新代码对同一 KB/文档连续触发两次，等待后台任务结束并导出两轮数据。
8. **实现 Wiki 报表。** 从 CSV 生成汇总 JSON、Markdown 和 SVG，核对公式与逐轮数据。
9. **扩展 CI 证据场景。** workflow 增加 pass/degraded 输入和专用分支触发；退化仅操作临时副本。
10. **运行真实 CI。** 推送 pass 场景并等待成功；推送 degraded 场景并等待失败；最后恢复 pass。
11. **保存 CI 证据。** 记录 run ID/URL/commit/conclusion，下载或保存关键日志，Chromium 截取成功/失败页面 PNG。
12. **最终回归与提交。** 运行目标单测、Go 检查、Python脚本自检、`git diff --check`，提交数据与证据。
