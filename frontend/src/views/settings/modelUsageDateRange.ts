const DATE_ONLY_PATTERN = /^\d{4}-\d{2}-\d{2}$/

export interface ModelUsageDateBounds {
  from?: string
  to?: string
}

function requireCalendarDate(value: string, label: string): string {
  if (!DATE_ONLY_PATTERN.test(value)) throw new Error(`${label}格式无效`)
  const [year, month, day] = value.split('-').map(Number)
  const parsed = new Date(Date.UTC(year, month - 1, day))
  if (
    parsed.getUTCFullYear() !== year
    || parsed.getUTCMonth() + 1 !== month
    || parsed.getUTCDate() !== day
  ) {
    throw new Error(`${label}格式无效`)
  }
  return value
}

/**
 * Keep date-picker values as calendar dates. The API expands the end date to
 * the next midnight so a same-day range covers that whole day without a
 * browser/server timezone shift.
 */
export function modelUsageDateBounds(range: readonly string[]): ModelUsageDateBounds {
  const [from, to] = range
  const bounds: ModelUsageDateBounds = {}
  if (from) bounds.from = requireCalendarDate(from.trim(), '开始日期')
  if (to) bounds.to = requireCalendarDate(to.trim(), '结束日期')
  if (bounds.from && bounds.to && bounds.from > bounds.to) {
    throw new Error('开始日期不能晚于结束日期')
  }
  return bounds
}
