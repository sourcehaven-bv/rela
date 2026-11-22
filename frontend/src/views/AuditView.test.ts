import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'

import AuditView from './AuditView.vue'
import { ApiError } from '@/api/errors'
import type { AuditPage } from '@/api/audit'

const getAuditLogMock = vi.fn<(...args: unknown[]) => Promise<AuditPage>>()
vi.mock('@/api/audit', async (orig) => ({
  ...(await orig<typeof import('@/api/audit')>()),
  getAuditLog: (...args: unknown[]) => getAuditLogMock(...args),
}))

function mountView() {
  return mount(AuditView, {
    global: { stubs: { PageLayout: { template: '<div><slot name="topbar" /><slot /></div>' } } },
  })
}

enableAutoUnmount(afterEach)

describe('AuditView', () => {
  beforeEach(() => {
    getAuditLogMock.mockReset()
  })

  it('shows the permission message on a 403', async () => {
    getAuditLogMock.mockRejectedValue(
      new ApiError('Forbidden', { kind: 'http', status: 403, original: null })
    )
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.find('[data-testid="audit-forbidden"]').text()).toContain(
      'You need the audit:read permission'
    )
    expect(wrapper.find('table').exists()).toBe(false)
  })

  it('renders records and pages with next_before', async () => {
    getAuditLogMock
      .mockResolvedValueOnce({
        records: [
          {
            seq: 7,
            record: {
              time: '2026-01-02T10:00:00Z',
              op: 'update',
              subject: { kind: 'entity', type: 'ticket', id: 'TKT-1' },
              principal: { user: 'alice', tool: 'web' },
              summary: 'status changed',
            },
          },
        ],
        next_before: 7,
      })
      .mockResolvedValueOnce({
        records: [
          {
            seq: 6,
            record: {
              time: '2026-01-01T10:00:00Z',
              op: 'relation-create',
              subject: {
                kind: 'relation',
                relation_type: 'affects',
                from_id: 'TKT-1',
                to_id: 'CON-1',
              },
              principal: { user: 'bob', tool: 'cli' },
            },
          },
        ],
      })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.findAll('[data-testid="audit-row"]')).toHaveLength(1)
    expect(wrapper.text()).toContain('ticket TKT-1')

    await wrapper.find('.audit-more button').trigger('click')
    await flushPromises()
    expect(getAuditLogMock).toHaveBeenLastCalledWith({ before: 7 }, expect.any(AbortSignal))
    const rows = wrapper.findAll('[data-testid="audit-row"]')
    expect(rows).toHaveLength(2)
    expect(rows[1].text()).toContain('TKT-1 affects → CON-1')
    expect(wrapper.find('.audit-more').exists()).toBe(false)
  })
})
