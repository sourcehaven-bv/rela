import { describe, expect, it } from 'vitest'
import { toBusyEntries } from './busy'

describe('toBusyEntries', () => {
  it('keeps the asked type and maps labels', () => {
    const since = '2026-10-30T09:00:00Z'
    expect(
      toBusyEntries(
        [
          { type: 'run', type_label: 'Agent run', state: 'planning', label: 'Planning', since },
          { type: 'job', state: 'working', since },
          { type: 'run', state: 'review', since },
        ],
        'run',
      ),
    ).toEqual([
      { typeLabel: 'Agent run', label: 'Planning', since },
      { typeLabel: 'run', label: 'review', since },
    ])
  })

  it('gives [] for an absent or empty list', () => {
    expect(toBusyEntries(undefined, 'run')).toEqual([])
    expect(toBusyEntries([], 'run')).toEqual([])
  })
})
