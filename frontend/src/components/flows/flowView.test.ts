import { describe, it, expect } from 'vitest'
import type { FlowRun } from '@/api/flows'
import { runHref, statusLabel, statusTone, subjectHref, timelineEntries } from './flowView'

const t = (key: string, values?: Record<string, unknown>) =>
  values ? `${key} ${JSON.stringify(values)}` : key

function run(over: Partial<FlowRun> = {}): FlowRun {
  return {
    id: 'R1',
    flow: 'onboarding',
    flow_label: 'Onboarding',
    version: 'v1',
    subject: 'PER-1',
    state: 'collect',
    state_label: 'Collect documents',
    status: 'input_required',
    generation: 2,
    active: true,
    _actions: { complete: false, cancel: false, retry: false },
    ...over,
  }
}

describe('flowView', () => {
  it('labels a known status and keeps an unknown one', () => {
    expect(statusLabel('failed', t)).toBe('flows.status.failed')
    expect(statusLabel('later', t)).toBe('later')
    expect(statusTone('completed')).toBe('green')
    expect(statusTone('later')).toBe('grey')
  })

  it('reads the timeline newest first and marks the current state of an active run', () => {
    const entries = timelineEntries(
      run({
        log: [
          { at: '2026-01-01T10:00:00Z', state: 'collect', via: 'start', by: 'ann' },
          {
            at: '2026-01-02T10:00:00Z',
            state: 'escalated',
            from: 'collect',
            via: 'timer',
            timer: 'stalled',
          },
        ],
      }),
      t
    )
    expect(entries.map((e) => e.summary)).toEqual([
      'flows.log.timer {"state":"escalated","timer":"stalled"}',
      'flows.log.start {"state":"collect"}',
    ])
    expect(entries[0].current).toBe(true)
    expect(entries[0].tone).toBe('amber')
    expect(entries[1].actor).toBe('ann')
    expect(entries[1].current).toBe(false)
  })

  it('marks no entry current once the run has finished', () => {
    const entries = timelineEntries(
      run({ active: false, log: [{ at: '2026-01-01T10:00:00Z', state: 'finished', via: 'done' }] }),
      t
    )
    expect(entries[0].current).toBe(false)
  })

  it('links a run and a readable subject', () => {
    expect(runHref('a/b')).toBe('/flows/runs/a%2Fb')
    expect(subjectHref('PER-1', 'person')).toBe('/entity/person/PER-1')
    expect(subjectHref(undefined, 'person')).toBe('')
    expect(subjectHref('PER-1', undefined)).toBe('')
  })
})
