import type { StatusColor } from 'rela-components/types'
import type { TimelineEntry } from 'rela-components/components/feedback/timeline'
import type { FlowEvent, FlowRun } from '@/api/flows'
import { browserTimeZone, formatDatetime } from '@/utils/format'
import { entityDetailHref } from '@/utils/entityRoute'

/** Translates a key with named values; vue-i18n's `t` fits it. */
export type Translate = (key: string, values?: Record<string, unknown>) => string

/**
 * The i18n key of each run status. A literal record, so the guard that finds
 * unused keys reads every one of them.
 */
const STATUS_KEYS: Record<string, string> = {
  working: 'flows.status.working',
  input_required: 'flows.status.input_required',
  completed: 'flows.status.completed',
  cancelled: 'flows.status.cancelled',
  failed: 'flows.status.failed',
}

const STATUS_TONES: Record<string, StatusColor> = {
  working: 'blue',
  input_required: 'amber',
  completed: 'green',
  cancelled: 'grey',
  failed: 'red',
}

/** The status in the interface language; an unknown status as it came. */
export function statusLabel(status: string, t: Translate): string {
  const key = STATUS_KEYS[status]
  return key ? t(key) : status
}

export function statusTone(status: string): StatusColor {
  return STATUS_TONES[status] ?? 'grey'
}

/** An RFC 3339 instant in the browser's zone and the interface language. */
export function formatInstant(value: string | undefined): string {
  if (!value) return ''
  return formatDatetime(value, browserTimeZone()) ?? value
}

/** What one timeline entry says, without the actor. */
function eventSummary(e: FlowEvent, t: Translate): string {
  const summary = moveSummary(e, t)
  return e.branch ? t('flows.log.inBranch', { summary, branch: e.branch }) : summary
}

function moveSummary(e: FlowEvent, t: Translate): string {
  switch (e.via) {
    case 'start':
      return t('flows.log.start', { state: e.label || e.state })
    case 'done':
      return t('flows.log.done', { state: e.label || e.state })
    case 'change':
      return t('flows.log.change', { state: e.label || e.state })
    case 'timer':
      return t('flows.log.timer', { state: e.label || e.state, timer: e.timer ?? '' })
    case 'join':
      return t('flows.log.join', { state: e.label || e.state })
    default:
      return t('flows.log.moved', { state: e.label || e.state })
  }
}

/**
 * The run's timeline, newest first. The server sends it oldest first; the
 * newest entry is the state the run is in. While the run is in parallel
 * branches, the newest entry of each branch still under way is.
 */
export function timelineEntries(run: FlowRun, t: Translate): TimelineEntry[] {
  const log = run.log ?? []
  const current = currentEntries(run)
  return log
    .map((e, i) => ({
      id: `${i}`,
      summary: eventSummary(e, t),
      actor: e.by || undefined,
      timestamp: formatInstant(e.at),
      datetime: e.at,
      tone: (e.via === 'timer' ? 'amber' : 'grey') as StatusColor,
      current: current.has(i),
    }))
    .reverse()
}

/** The indexes of the log entries for the states the run is in now. */
function currentEntries(run: FlowRun): Set<number> {
  const log = run.log ?? []
  if (!run.active || log.length === 0) return new Set()
  if (!run.parallel?.length) return new Set([log.length - 1])
  const live = new Set(run.parallel.filter((p) => !p.arrived).map((p) => p.branch))
  const out = new Set<number>()
  for (let i = log.length - 1; i >= 0 && live.size > 0; i--) {
    const b = log[i].branch
    if (b && live.delete(b)) out.add(i)
  }
  return out
}

/** The route of one run's page. */
export function runHref(id: string): string {
  return `/flows/runs/${encodeURIComponent(id)}`
}

/**
 * The route of a run's subject, when the caller may read it. The run names
 * the subject's address only; its type is the flow's subject type.
 */
export function subjectHref(subject: string | undefined, type: string | undefined): string {
  if (!subject || !type) return ''
  return entityDetailHref({ type, id: subject })
}
