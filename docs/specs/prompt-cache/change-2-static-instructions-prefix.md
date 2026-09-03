# Change 2：wiki_page_modify 固定指令整体前置

## 动机

Change 1 只增加了短 `<task_scope>`，验证后无提升。原因是新增前缀过短，
不足以改变 cache_read 规模。

## 改动

`WikiPageModifyUserPrompt` 重新排布：

- 共享 Source Context 之后、`<page_metadata>` 之前：
  - SUMMARY 输出规则；
  - 保留已有有效内容的规则；
  - Wiki 链接处理规则；
  - 页面结构维护规则；
  - 语言规则。
- `<page_metadata>` 之后只保留真正页面相关的内容和条件更新规则：
  - 新增信息合并与冲突检查；
  - 删除文档处理；
  - 空页兜底。

不再把 `PageTitle / PageSlug` 内联进固定规则文本，改为引用
`<page_metadata>`，使规则段可以在页面之间稳定复用。

## 测试

`TestWikiPageModifyUserPrompt_StableInstructionsPrecedePageMetadata`：

- 同一 Source Context 渲染 Alpha / Beta；
- `<page_metadata>` 前逐字节一致；
- 固定规则确实位于稳定前缀内。

## 待验证

重启后端后，用同一批文档跑两次 Wiki 生成，对比：

- `wiki_page_modify.cache_read`
- `wiki_page_modify` 命中率

目标：cache_read 显著大于 Change 1 的 512-2304 级别。
