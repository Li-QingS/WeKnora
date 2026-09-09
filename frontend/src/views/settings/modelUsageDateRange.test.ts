import assert from 'node:assert/strict'
import test from 'node:test'
import { modelUsageDateBounds } from './modelUsageDateRange'

test('keeps a same-day model usage range as calendar dates', () => {
  assert.deepEqual(modelUsageDateBounds(['2026-09-08', '2026-09-08']), {
    from: '2026-09-08',
    to: '2026-09-08',
  })
})

test('omits missing date-picker bounds and rejects invalid dates', () => {
  assert.deepEqual(modelUsageDateBounds([]), {})
  assert.throws(() => modelUsageDateBounds(['2026/09/08', '']), /开始日期格式无效/)
  assert.throws(() => modelUsageDateBounds(['2026-02-30', '']), /开始日期格式无效/)
  assert.throws(() => modelUsageDateBounds(['2026-09-09', '2026-09-08']), /开始日期不能晚于结束日期/)
})
