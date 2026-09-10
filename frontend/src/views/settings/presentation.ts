const integerFormatter = new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 0 })

export function formatCount(value: number | null | undefined): string {
  return integerFormatter.format(value || 0)
}

export function formatDateTime(value: string | null | undefined): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
  }).format(date)
}

export function formatUSD(value: number | null | undefined, digits = 4): string {
  if (value == null) return '未配置'
  return `$${value.toLocaleString('en-US', { minimumFractionDigits: digits, maximumFractionDigits: digits })}`
}

export function shortIdentifier(value: string | null | undefined, length = 8): string {
  if (!value) return '—'
  return value.length <= length ? value : `${value.slice(0, length)}…`
}

export function modelTypeLabel(value: string | null | undefined): string {
  const labels: Record<string, string> = {
    KnowledgeQA: '问答模型',
    Embedding: '向量模型',
    Rerank: '重排模型',
    VLLM: '多模态模型',
    ASR: '语音识别',
  }
  return value ? labels[value] || value : '未标记'
}

export function modelPurposeLabel(value: string | null | undefined): string {
  const labels: Record<string, string> = {
    agent_round: '智能体推理',
    document_summary: '文档摘要',
    question_generation: '问题生成',
    document_auto_tag: '文档自动标签',
    query_rewrite: '查询改写',
    entity_extraction: '实体抽取',
    data_analysis_plan: '数据分析规划',
    'follow_up.suggestions': '追问建议',
    wiki_outline: 'Wiki 大纲生成',
    wiki_page: 'Wiki 页面生成',
    wiki_page_create: 'Wiki 页面生成',
    wiki_page_modify: 'Wiki 页面修改',
    wiki_citation: 'Wiki 引用抽取',
  }
  return value ? labels[value] || value.replaceAll('_', ' ') : '未标记'
}

export function modelCallStatusLabel(value: string | null | undefined): string {
  const normalized = value?.toLowerCase()
  if (normalized === 'success' || normalized === 'succeeded' || normalized === 'completed') return '成功'
  if (normalized === 'failed' || normalized === 'error') return '失败'
  if (normalized === 'running') return '运行中'
  return value || '未知'
}

export interface MetricPresentation {
  label: string
  help: string
}

const metricPresentations: Record<string, MetricPresentation> = {
  precision: { label: '检索准确率', help: '检索结果中相关文档所占比例' },
  recall: { label: '检索召回率', help: '标准相关文档被检索到的比例' },
  ndcg3: { label: '前 3 条排序质量', help: '相关文档越靠前，得分越高' },
  ndcg10: { label: '前 10 条排序质量', help: '前 10 条结果的相关性与排序表现' },
  mrr: { label: '首个正确结果排名', help: '正确结果首次出现得越靠前，得分越高' },
  map: { label: '平均检索准确度', help: '综合衡量每个相关结果的排序位置' },
  bleu1: { label: '答案词语一致度', help: '生成答案与标准答案的单词重合程度' },
  bleu2: { label: '答案短语一致度', help: '生成答案与标准答案的二元短语重合程度' },
  bleu4: { label: '答案表达一致度', help: '生成答案与标准答案的四元短语重合程度' },
  rouge1: { label: '答案内容覆盖度', help: '标准答案中的关键词被覆盖的程度' },
  rouge2: { label: '答案短语覆盖度', help: '标准答案中的二元短语被覆盖的程度' },
  rougel: { label: '答案结构相似度', help: '基于最长公共子序列衡量答案结构' },
}

export function evaluationMetricPresentation(name: string): MetricPresentation {
  const key = name.toLowerCase().replaceAll('-', '').replaceAll('_', '')
  return metricPresentations[key] || { label: name, help: '分数越高表示表现越好' }
}

export function evaluationCostLabel(name: string): string {
  return ({
    model_calls: '模型调用', prompt_tokens: '输入 Token', completion_tokens: '输出 Token',
    total_tokens: '总 Token', cache_read_tokens: '缓存读取 Token',
    cache_write_tokens: '缓存写入 Token', estimated_cost_usd: '估算费用',
  } as Record<string, string>)[name] || name
}

export function evaluationLatencyLabel(name: string): string {
  return ({
    duration_ms: '总耗时', avg_ms_per_sample: '每个样本平均耗时',
    model_calls: '模型调用', avg_ms_per_model_call: '每次模型调用平均耗时',
  } as Record<string, string>)[name] || name
}

export function wikiNodeTypeLabel(value: string): string {
  return ({ entity: '实体', concept: '概念' } as Record<string, string>)[value] || value
}

export function wikiMatchMethodLabel(value: string): string {
  return ({ exact: '名称匹配', semantic: '语义匹配', unmatched: '未覆盖' } as Record<string, string>)[value] || value
}
