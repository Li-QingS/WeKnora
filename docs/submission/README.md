# 课题三最终成果运行及阅读说明

完整的功能展示、逐项验收步骤和缓存实测对照见 [`WeKnora_课题三_功能展示与验收手册_李青松.docx`](./WeKnora_课题三_功能展示与验收手册_李青松.docx)。同内容的可检索版本为 [`课题三_功能展示与验收手册.md`](./课题三_功能展示与验收手册.md)。

## 成果信息

- 课题：腾讯犀牛鸟 WeKnora 开源训练营课题三「质量评测基线与成本可观测」
- 最终 Tag：`rhino-2026-final-3`
- 完整 Commit：`501f957e584f99f1d231466fa27fa0c0ccd25980`
- 代码链接：<https://github.com/Li-QingS/WeKnora/tree/rhino-2026-final-3>
- 开发分支：`feat/evaluation-persistence`
- 上游基线：腾讯 `upstream/main` 提交 `b60351f86128d22bfa998b13989530917d32ef2b` 已合并。

## 成果内容

本成果完成了可复现 RAG 评测、评测结果持久化、质量回归门禁、模型调用台账和费用估算、Embedding 持久缓存、Wiki Prompt Cache 优化及 Web 端评测与用量页面。Wiki 评测作为扩展实验能力，衡量实体/概念覆盖和有向连接结构，不加入 CI 门禁。

主要入口：

1. Web 页面：登录后打开“设置 → 评测中心”，可运行 RAG 问答评测或 Wiki 图谱评测。
2. Web 页面：打开“设置 → 模型用量”，可按模型和自然日区间查看调用、Token、失败、缓存复用与费用。
3. CLI：`make eval-baseline CONFIG=./evaluation/configs/enterprise.yaml` 运行可复现评测并生成 JSON、Markdown 报告。
4. CI：`.github/workflows/rag-quality-gate.yml` 运行质量门禁；退化指标会使比较器以退出码 2 失败。

## 启动

环境准备、模型配置和完整部署方式见仓库根目录 `README.md` 与 `docs/开发指南.md`。开发模式可执行：

```bash
cp .env.example .env
make dev-start
```

另开两个终端：

```bash
make dev-app
```

```bash
make dev-frontend
```

默认前端地址为 <http://localhost:5173>，后端健康检查为：

```bash
curl -fsS http://localhost:8080/health
```

## 运行评测

先在 Web 页面配置可用的 Chat、Embedding 模型。CLI 使用已登录的活动配置，或者通过环境变量提供服务地址和令牌：

```bash
export WEKNORA_HOST=http://localhost:8080
export WEKNORA_TOKEN='<登录后取得的访问令牌>'
make eval-baseline CONFIG=./evaluation/configs/enterprise.yaml
```

报告输出到：

- `artifacts/evaluation/evaluation-result.json`
- `artifacts/evaluation/evaluation-report.md`

使用固定夹具验证门禁的成功和失败分支：

```bash
make -C cli build
./cli/bin/weknora eval compare \
  --result evaluation/fixtures/evaluation-result-pass.json \
  --baseline evaluation/baselines/demo.yaml

./cli/bin/weknora eval compare \
  --result evaluation/fixtures/evaluation-result-degraded.json \
  --baseline evaluation/baselines/demo.yaml
```

第二条命令预期列出退化指标并返回退出码 2。

## 测试

```bash
go test ./...
go vet ./...
go build ./...

(cd client && go test ./... && go vet ./...)
(cd cli && go test ./... && go vet ./...)

(cd frontend && npm install && npm test && npm run type-check && npm run build)
```

最终页面修改验证结果：前端全量 785 项测试通过，类型检查和生产构建通过。合并上游后的全仓 Go 测试、静态检查、构建以及 PostgreSQL/SQLite 迁移验证均已通过。

## 实测结果和证据

- Embedding 固定格式漂移场景：命中率 `33.33% → 66.67%`，Provider 输入 `270 → 135`，减少 50%。
- 32 个相同并发冷请求：Provider 调用 `32 → 1`，减少 96.875%。
- Wiki 页面生成缓存读取率：最初无专项优化基线 `11.29% →` 最终热运行 `21.49%`，增加 10.20 个百分点。
- Wiki 引用抽取缓存读取率：最初无专项优化基线 `10.44% →` 最终热运行 `15.34%`，增加 4.90 个百分点。
- 全部 Wiki 调用缓存读取率：最初无专项优化基线 `14.01% →` 最终热运行 `24.97%`，增加 10.96 个百分点。初始值不为零是模型服务商已有隐式缓存；9 月 8 日数据仅用于拆分第二阶段增量。
- RAG CI 已保存真实成功、Recall 人为退化失败和恢复成功三组 GitHub Actions 证据。
- 8 个解析引擎已建立统一可复查基线；本地成功运行 builtin 与 anydoc，其余引擎因当前环境缺少服务或凭据标为 N/A，不按零分处理。

详细证据：

- `docs/evidence/embedding-cache-hit-rate-2026-09-10/`
- `docs/evidence/wiki-generation-cache-2026-09-10/`
- `docs/evidence/ci-gate-2026-09-08/`
- `docs/evidence/parser-baseline-2026-09-08/`
- `docs/specs/upstream-sync-review-2026-09-10/review.md`

## 已知问题

1. Wiki 评测是用户主动运行的实验功能，生成 Wiki 会产生真实 Chat 模型调用；评分使用名称匹配、Embedding 和确定性图比较，不使用生成模型裁判。
2. Embedding 缓存默认关闭，当前采用 SQL 持久缓存和进程内并发合并；尚未提供面向长期大规模生产的租户容量配额与完整 TTL 治理。
3. 模型费用依赖管理员预先配置单价，未配置时页面显示“未配置”；价格按调用时快照保存，不追溯修改历史记录。
4. Wiki Prompt Cache 的实际收益取决于模型服务商是否支持缓存及其缓存周期，代码没有绑定特定模型名称。
5. 使用真实模型运行 CI 需要在 GitHub 配置 `WEKNORA_EVAL_HOST`、`WEKNORA_EVAL_TOKEN` 等 Secrets，并为目标分支启用保护规则。
6. Vite 构建仍有上游已有的大体积 chunk 警告，不影响构建和本次功能。
