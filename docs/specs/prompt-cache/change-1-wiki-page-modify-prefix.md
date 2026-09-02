# Change 1：wiki_page_modify 增加稳定 user 前缀

## 改动内容

`WikiPageModifyUserPrompt` 在共享 Source Context 之后、`<page_metadata>` 之前
增加一段固定不随页面变化的 `<task_scope>`：

- 明确本次任务是更新 `<page_metadata>` 中声明的页面；
- 提醒共享 Source Context 只用于校准，不作为事实证据；
- 再次固定输出格式：先输出 SUMMARY 行，不带其他 preamble。

该内容不含 `PageTitle / PageSlug` 等页面级变量，因此同一份文档批量更新多个页面时，
该段会成为稳定前缀的一部分，有望提高 qwen 供应商前缀缓存的 cache_read。

## 修改文件

- `internal/agent/prompts_wiki.go`
- `internal/agent/prompts_wiki_test.go`

## 测试

新增 `TestWikiPageModifyUserPrompt_StableTaskScopePrecedesPageMetadata`：

- 同一 Source Context 渲染 Alpha / Beta 两个页面；
- 断言 `<page_metadata>` 前逐字节一致；
- 断言 `<task_scope>` 与 “Output the SUMMARY line first” 位于缓存前缀内。

## 待验证

使用同一批文档重新触发两次 Wiki 生成，对比：

`docs/specs/prompt-cache/baseline-2026-09-03.md`

重点看 `wiki_page_modify` 的 `cache_read` 是否变大、命中率是否超过 14%。
