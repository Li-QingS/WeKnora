# 课题三最终实测证据 Checklist

> 状态：已完成（2026-09-08）

- [x] 腾讯最新 main 已 fetch 并合并；当前分支包含 `upstream/main`。
- [x] 解析 fixture 与 Gold 已版本化且 SHA-256 入报告。
- [x] 8 个应用侧解析引擎均有一行结果，无 mock 冒充真实结果。
- [x] 当前可用引擎完成真实解析，原始 Markdown 可复查。
- [x] 解析 JSON、CSV、Markdown 表格、SVG 图均生成且数字一致。
- [x] Wiki 历史“优化前”原始记录已导出。
- [x] 最新代码同一文档冷、热两轮均成功。
- [x] Wiki CSV、JSON、Markdown 表格、SVG 图均生成且数字一致。
- [x] 真实 GitHub Actions pass run 完成并保存 run URL、API 元数据和 PNG。
- [x] 人为降低 Recall 的真实 GitHub Actions fail run 完成，GitHub annotation 含 delta/threshold，并保存 run URL、API 元数据和 PNG。
- [x] 分支最终恢复正常 pass 场景，没有故意退化数据。
- [x] 敏感信息扫描无命中。
- [x] 相关测试、构建和格式检查通过。
