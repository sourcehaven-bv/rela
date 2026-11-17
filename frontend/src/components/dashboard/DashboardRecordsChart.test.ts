import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'

import DashboardRecordsChart from './DashboardRecordsChart.vue'
import type { DashboardRecords } from '@/types'
import type { AggregateRecordsQuery, RecordPoint } from '@/api/records'

const aggregateMock = vi.fn<(type: string, q: AggregateRecordsQuery) => Promise<RecordPoint[]>>()
const getEntityMock = vi.fn<(type: string, id: string) => Promise<{ _title: string }>>()
vi.mock('@/api', async () => {
  const errors = await import('@/api/errors')
  return {
    aggregateRecords: (type: string, q: AggregateRecordsQuery) => aggregateMock(type, q),
    getRecordTypes: () =>
      Promise.resolve([{ key: 'measurement', label: 'Measurement', owner: 'metric' }]),
    getEntity: (type: string, id: string) => getEntityMock(type, id),
    shouldDropHeldContent: errors.shouldDropHeldContent,
  }
})

enableAutoUnmount(afterEach)

const records: DashboardRecords = {
  type: 'measurement',
  field: 'value',
  owners: ['MET-A', 'MET-B'],
  bucket: 'month',
  range: '365d',
}

function pt(owner: string, start: string, latest: string): RecordPoint {
  return { owner, start, count: 1, latest }
}

async function mountChart(r: DashboardRecords = records) {
  const w = mount(DashboardRecordsChart, { props: { records: r, title: 'Coverage' } })
  await flushPromises()
  return w
}

beforeEach(() => {
  aggregateMock.mockReset()
  getEntityMock.mockReset()
  getEntityMock.mockImplementation((_t, id) => Promise.resolve({ _title: `Title ${id}` }))
})

describe('DashboardRecordsChart', () => {
  it('asks for every owner kept apart, over the range', async () => {
    aggregateMock.mockResolvedValue([])
    await mountChart()
    const [type, q] = aggregateMock.mock.calls[0]
    expect(type).toBe('measurement')
    expect(q).toMatchObject({
      owners: ['MET-A', 'MET-B'],
      field: 'value',
      bucket: 'month',
      perOwner: true,
    })
    expect(q.from).toMatch(/^\d{4}-\d{2}-\d{2}T/)
  })

  it('draws a line, a legend entry and a table column per owner', async () => {
    aggregateMock.mockResolvedValue([
      pt('MET-A', '2026-01-01T00:00:00Z', '0.5'),
      pt('MET-A', '2026-02-01T00:00:00Z', '0.75'),
      pt('MET-B', '2026-02-01T00:00:00Z', '0.25'),
    ])
    const w = await mountChart()
    expect(w.findAll('[data-testid="records-line"]')).toHaveLength(2)
    expect(w.get('[data-testid="records-legend"]').text()).toContain('Title MET-A')
    expect(w.get('[data-testid="records-legend"]').text()).toContain('Title MET-B')
    const head = w.findAll('[data-testid="records-table"] thead th').map((th) => th.text())
    expect(head.slice(1)).toEqual(['Title MET-A', 'Title MET-B'])
    expect(w.findAll('[data-testid="records-table"] tbody tr')).toHaveLength(2)
  })

  // The server folds only readable owners: an owner the reader cannot read
  // is absent from the points. The card names it nowhere and does not look
  // up its title.
  it('shows nothing for an owner the reader cannot read', async () => {
    aggregateMock.mockResolvedValue([
      pt('MET-B', '2026-01-01T00:00:00Z', '1'),
      pt('MET-B', '2026-02-01T00:00:00Z', '2'),
    ])
    const w = await mountChart()
    const lines = w.findAll('[data-testid="records-line"]')
    expect(lines).toHaveLength(1)
    // MET-B keeps its own colour slot, the second, though it is drawn alone.
    expect(lines[0].classes()).toContain('records-chart__s2')
    expect(w.find('[data-testid="records-legend"]').exists()).toBe(false)
    expect(w.text()).not.toContain('MET-A')
    expect(getEntityMock.mock.calls.map((c) => c[1])).toEqual(['MET-B'])
  })

  it('shows no data when its single owner is hidden (a 404)', async () => {
    const { ApiError } = await import('@/api/errors')
    aggregateMock.mockRejectedValue(
      new ApiError('not found', { kind: 'http', status: 404, original: null })
    )
    const w = await mountChart({ ...records, owners: ['MET-A'] })
    expect(w.find('[data-testid="records-empty"]').exists()).toBe(true)
    expect(w.find('[data-testid="records-svg"]').exists()).toBe(false)
    expect(w.find('[data-testid="card-failed"]').exists()).toBe(false)
    expect(getEntityMock).not.toHaveBeenCalled()
  })

  it('drops a drawn line when a refetch is refused', async () => {
    aggregateMock.mockResolvedValueOnce([pt('MET-A', '2026-01-01T00:00:00Z', '1')])
    const w = await mountChart()
    expect(w.findAll('[data-testid="records-line"]')).toHaveLength(1)
    const { ApiError } = await import('@/api/errors')
    aggregateMock.mockRejectedValueOnce(
      new ApiError('forbidden', { kind: 'http', status: 403, original: null })
    )
    await w.setProps({ records: { ...records, bucket: 'week' } })
    await flushPromises()
    expect(w.find('[data-testid="records-line"]').exists()).toBe(false)
    expect(w.find('[data-testid="records-empty"]').exists()).toBe(true)
  })

  it('keeps a drawn line when a refetch fails for another reason', async () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    aggregateMock.mockResolvedValueOnce([pt('MET-A', '2026-01-01T00:00:00Z', '1')])
    const w = await mountChart()
    aggregateMock.mockRejectedValueOnce(new Error('network'))
    await w.setProps({ records: { ...records, bucket: 'week' } })
    await flushPromises()
    expect(w.findAll('[data-testid="records-line"]')).toHaveLength(1)
    expect(w.find('[data-testid="card-failed"]').exists()).toBe(false)
    spy.mockRestore()
  })

  it('breaks a line where a bucket has no value', async () => {
    aggregateMock.mockResolvedValue([
      pt('MET-A', '2026-01-01T00:00:00Z', '1'),
      pt('MET-A', '2026-02-01T00:00:00Z', '2'),
      pt('MET-A', '2026-04-01T00:00:00Z', '3'),
    ])
    const w = await mountChart()
    const d = w.get('.records-chart__line').attributes('d') ?? ''
    expect(d.match(/M/g)).toHaveLength(2)
  })

  it('says the card failed on any other error', async () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    aggregateMock.mockRejectedValue(new Error('network'))
    const w = await mountChart()
    expect(w.find('[data-testid="card-failed"]').exists()).toBe(true)
    spy.mockRestore()
  })

  it('reads every line at a bucket from the keyboard', async () => {
    aggregateMock.mockResolvedValue([
      pt('MET-A', '2026-01-01T00:00:00Z', '0.5'),
      pt('MET-A', '2026-02-01T00:00:00Z', '0.75'),
      pt('MET-B', '2026-02-01T00:00:00Z', '0.25'),
    ])
    const w = await mountChart()
    const svg = w.get('[data-testid="records-svg"]')
    await svg.trigger('focus')
    const tip = () => w.get('[data-testid="records-tooltip"]').text()
    expect(tip()).toContain('0.75')
    expect(tip()).toContain('0.25')
    await svg.trigger('keydown', { key: 'ArrowLeft' })
    expect(tip()).toContain('0.5')
    expect(tip()).not.toContain('Title MET-B')
    expect(w.get('[data-testid="records-live"]').text()).toContain('Title MET-A: 0.5')
    await svg.trigger('keydown', { key: 'Escape' })
    expect(w.find('[data-testid="records-tooltip"]').exists()).toBe(false)
  })
})
