# 8 个解析引擎横向基线（2026-09-08）

- Commit：`2390c6748ea9`
- Fixture：`evaluation/parser_benchmark/fixture.pdf`
- SHA-256：`db9a3e681fb48d49a1d3c7cc286f84c2f5476b86ddc88f5a70a0e5d80a5b1123`
- 评分：`quality_score = 0.8 * content_recall + 0.2 * structure_recall; unavailable/unsupported/failed scores are null`

| 引擎 | 状态 | 耗时 ms | 内容 Recall | 结构 Recall | 综合分 | 说明 |
|---|---:|---:|---:|---:|---:|---|
| `builtin` | success | 1872 | 100.00% | 20.00% | 84.00% |  |
| `simple` | unsupported | 0 | N/A | N/A | N/A | engine does not declare PDF support |
| `anydoc` | success | 5 | 100.00% | 100.00% | 100.00% |  |
| `weknoracloud` | unavailable | 0 | N/A | N/A | N/A | WeKnora Cloud credentials not configured. Go to Settings → WeKnora Cloud to set up. |
| `mineru` | unavailable | 0 | N/A | N/A | N/A | MinerU service not configured |
| `mineru_cloud` | unavailable | 0 | N/A | N/A | N/A | MinerU API Key not configured |
| `paddleocr_vl` | unavailable | 0 | N/A | N/A | N/A | PaddleOCR-VL service not configured |
| `paddleocr_vl_cloud` | unavailable | 0 | N/A | N/A | N/A | PaddleOCR-VL Cloud Token not configured |

## 口径说明

质量分只针对真实成功输出计算。`unavailable` 表示当前环境缺服务、凭据或构建能力；`unsupported` 表示引擎登记的能力不包含 PDF。二者均显示 N/A，不能解释为解析质量 0 分。该小型合成 PDF 用于建立可复查横向基线，不代表扫描件、中文复杂版面或超长文档的完整表现。
