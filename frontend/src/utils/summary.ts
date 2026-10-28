/**
 * Summaries: one aggregate line over a set of entities (TKT-SVLQQ6).
 *
 * The kanban column header is the first user. The spec type (`Summary`) is
 * shared so that list footers, group_by headers and dashboard cards can use
 * the same function later. The server validates the spec, so this code
 * trusts its shape: count has no property, sum and avg aggregate an integer,
 * min and max an integer, date or datetime.
 */
import type { PropertyDef, Summary, SummaryFunction } from '@/types'
import { t } from '@/i18n'
import {
  browserTimeZone,
  formatAmount,
  formatDate,
  formatDatetime,
  formatDisplayNumber,
  numericValue,
} from './format'

/** The entity shape a summary reads: properties plus the redaction list. */
export interface SummarizedEntity {
  properties: Record<string, unknown>
  _redacted?: string[]
}

/** Shown when there is no value to report, such as the average of nothing. */
export const NO_VALUE = '–'

const DEFAULT_LABELS = {
  count: 'list.summary.count',
  sum: 'list.summary.sum',
  avg: 'list.summary.avg',
  min: 'list.summary.min',
  max: 'list.summary.max',
} as const satisfies Record<SummaryFunction, string>

/**
 * One summary as display text, such as "Total: € 11.500".
 *
 * An empty set shows 0 for count and sum, and "–" for avg, min and max: an
 * average or extreme of nothing does not exist, but a sum of nothing is 0.
 *
 * A value hidden from the reader by `visible:` is absent, but it is not "not
 * set": `_redacted` names it. The result then misses values the reader may
 * not see, so it is marked "(partial)" rather than stated as the whole.
 */
export function formatSummary(
  spec: Summary,
  entities: readonly SummarizedEntity[],
  propDef: PropertyDef | undefined,
  tz: string = browserTimeZone()
): string {
  const key = DEFAULT_LABELS[spec.function] as (typeof DEFAULT_LABELS)[SummaryFunction] | undefined
  const label = spec.label || (key ? t(key) : spec.function)
  const line = `${label}: ${summaryValue(spec, entities, propDef, tz)}`
  const prop = spec.property
  const partial = prop !== undefined && entities.some((e) => e._redacted?.includes(prop))
  return partial ? t('list.summary.partial', { line }) : line
}

function summaryValue(
  spec: Summary,
  entities: readonly SummarizedEntity[],
  propDef: PropertyDef | undefined,
  tz: string
): string {
  if (spec.function === 'count' || !spec.property) return formatAmount(entities.length)
  const raw = entities.map((e) => e.properties[spec.property as string])

  if (propDef?.type === 'date' || propDef?.type === 'datetime') {
    if (spec.function !== 'min' && spec.function !== 'max') return NO_VALUE
    const pick = extremeDate(raw, spec.function)
    if (pick === undefined) return NO_VALUE
    return (propDef.type === 'date' ? formatDate(pick) : formatDatetime(pick, tz)) ?? pick
  }

  const numbers = raw.map(numericValue).filter((n): n is number => n !== undefined)
  let result: number
  switch (spec.function) {
    case 'sum':
      result = numbers.reduce((a, b) => a + b, 0)
      break
    case 'avg':
      if (numbers.length === 0) return NO_VALUE
      result = numbers.reduce((a, b) => a + b, 0) / numbers.length
      break
    case 'min':
    case 'max':
      if (numbers.length === 0) return NO_VALUE
      result = spec.function === 'min' ? Math.min(...numbers) : Math.max(...numbers)
      break
    default:
      return NO_VALUE
  }
  return formatDisplayNumber(result, propDef) ?? formatAmount(result)
}

// The earliest or latest of the date values, as the stored string so the
// caller formats it the way the property formats everywhere else.
function extremeDate(values: unknown[], fn: 'min' | 'max'): string | undefined {
  let best: { raw: string; ms: number } | undefined
  for (const v of values) {
    if (typeof v !== 'string' || v === '') continue
    const ms = Date.parse(v)
    if (Number.isNaN(ms)) continue
    if (!best || (fn === 'min' ? ms < best.ms : ms > best.ms)) best = { raw: v, ms }
  }
  return best?.raw
}
