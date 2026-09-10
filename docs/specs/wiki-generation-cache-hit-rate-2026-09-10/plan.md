# Wiki 生成缓存命中率优化 Plan

## 架构

1. Chat 层增加内部 Prompt 缓存断点描述，可指定消息序号和原始字符串的 UTF-8 字节偏移。
2. Provider 传输层根据协议能力处理统一断点：Anthropic 使用 content block；百炼显式缓存使用消息级截断，因此把原消息拆成同角色的稳定消息和动态消息。动态尾部不打标，总断点限制为四个。
3. Prompt Cache 策略通过 URL hostname 识别阿里云 Workspace 兼容端点；`generic` 配置按端点能力启用，不硬编码模型名。支持缓存路由键的 Provider 使用同一前缀指纹。
4. Wiki 调用层定位页面生成的 `<page_metadata>`、引用抽取的 `<chunks>`，把稳定前缀结束位置传给 Chat 层；业务层仍保留原始单字符串消息。
5. 页面生成和引用抽取共用前缀 warmup gate；首请求创建缓存后，同前缀请求再并发执行。

## 技术决策

| 决策 | 选择 | 原因 |
|---|---|---|
| 缓存类型 | 服务商显式 Context Cache | 直接减少输入 Token 推理与计费，不引入生成结果陈旧问题 |
| 端点识别 | 解析 URL hostname | 避免用查询串或路径误判服务商 |
| 模型适配 | Provider 能力适配，不维护模型名白名单 | 模型版本会演进，Wiki 缓存边界应可复用于任意模型 |
| Wiki Prompt 分块 | 仅在支持显式缓存的传输层按字节偏移拆分 | 非目标 Provider 收到的请求结构不变；目标端点的稳定/动态消息拼接后与原 Prompt 逐字节相同 |
| TTL | `ephemeral` 默认 5 分钟 | 符合百炼当前协议，且覆盖一次 Wiki 并发生成批次 |
| 输出缓存 | 不采用 | 页面生成包含现有内容和证据合并，输出持久缓存会引入失效与生成语义变化 |

## 修改范围

- `internal/models/chat/chat.go`：内部断点描述。
- `internal/models/chat/prompt_cache.go`：Provider/端点策略、断点注入和上限处理。
- `internal/models/chat/remote_api.go`：把模型和断点传给缓存策略。
- `internal/application/service/wiki_ingest.go`：Wiki Prompt 稳定/动态分块。
- 对应 Chat 与 Wiki 单元测试。
- `docs/evidence/wiki-generation-cache-2026-09-10/`：真实验收数据和结果。
