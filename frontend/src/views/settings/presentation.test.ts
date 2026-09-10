import assert from 'node:assert/strict'
import test from 'node:test'
import {
  evaluationMetricPresentation,
  formatCount,
  formatUSD,
  modelCallStatusLabel,
  modelPurposeLabel,
  modelTypeLabel,
  shortIdentifier,
  wikiMatchMethodLabel,
  wikiNodeTypeLabel,
} from './presentation'

test('formats usage values for scanning and keeps missing cost explicit', () => {
  assert.equal(formatCount(3079), '3,079')
  assert.equal(formatUSD(0.123456), '$0.1235')
  assert.equal(formatUSD(null), '未配置')
  assert.equal(shortIdentifier('12345678-abcdefgh'), '12345678…')
})

test('translates model usage codes without hiding unknown values', () => {
  assert.equal(modelTypeLabel('KnowledgeQA'), '问答模型')
  assert.equal(modelPurposeLabel('document_summary'), '文档摘要')
  assert.equal(modelPurposeLabel('custom_step'), 'custom step')
  assert.equal(modelCallStatusLabel('success'), '成功')
})

test('explains evaluation metrics and wiki matching terms', () => {
  assert.deepEqual(evaluationMetricPresentation('ndcg10'), {
    label: '前 10 条排序质量',
    help: '前 10 条结果的相关性与排序表现',
  })
  assert.equal(wikiNodeTypeLabel('entity'), '实体')
  assert.equal(wikiMatchMethodLabel('semantic'), '语义匹配')
})
