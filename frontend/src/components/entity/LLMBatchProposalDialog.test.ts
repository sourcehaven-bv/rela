// The review dialog of a batch `llm:` run (TKT-I2SRNWR): every change starts
// unticked, it writes only what the user ticks, per entity, and skips a
// property that changed since the proposal.

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const getEntity = vi.fn()
const updateEntity = vi.fn()
vi.mock('@/api/entities', () => ({
  getEntity: (...a: unknown[]) => getEntity(...a),
  updateEntity: (...a: unknown[]) => updateEntity(...a),
}))

vi.mock('@/composables/modalStack', () => ({
  useModalStack: vi.fn(),
  registerModal: vi.fn(),
  unregisterModal: vi.fn(),
  isAnyModalOpen: () => false,
}))

const success = vi.fn()
vi.mock('@/stores', () => ({
  useSchemaStore: () => ({
    getEntityType: () => ({ properties: { priority: { type: 'enum', label: 'Priority' } } }),
  }),
  useUIStore: () => ({ success }),
}))

import LLMBatchProposalDialog from './LLMBatchProposalDialog.vue'
import type { ActionBatch } from '@/api/actions'

const batch: ActionBatch = {
  proposals: [
    {
      entity: 'TKT-1',
      title: 'Login',
      set: [{ property: 'priority', current: 'low', proposed: 'high' }],
      relate: [],
    },
    {
      entity: 'TKT-2',
      title: 'Logout',
      set: [{ property: 'priority', current: 'low', proposed: 'medium' }],
      relate: [],
    },
    { entity: 'TKT-3', set: [], relate: [] },
  ],
  failed: [{ entity: 'TKT-4', reason: 'the AI reply has no answer for this record' }],
  skipped: 2,
}

function mountDialog() {
  return mount(LLMBatchProposalDialog, {
    props: { batch, entityType: 'ticket', label: 'Triage' },
    global: { stubs: { Teleport: true } },
  })
}

function applyButton(w: ReturnType<typeof mountDialog>) {
  const btn = w.findAll('button').find((b) => b.text() === 'Apply selected')
  if (!btn) throw new Error('no Apply button')
  return btn
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
  updateEntity.mockResolvedValue({})
})

describe('LLMBatchProposalDialog', () => {
  it('starts with every change unticked and Apply disabled', () => {
    const w = mountDialog()
    const boxes = w.findAll('input[type="checkbox"]')
    // One row box and one change box per entity with a change.
    expect(boxes).toHaveLength(4)
    for (const b of boxes) expect((b.element as HTMLInputElement).checked).toBe(false)
    expect(applyButton(w).attributes('disabled')).toBeDefined()
  })

  it('shows the unchanged, skipped and failed entities', () => {
    const text = mountDialog().text()
    expect(text).toContain('1 entity has no proposed change.')
    expect(text).toContain('2 entities were not sent')
    expect(text).toContain('TKT-4: the AI reply has no answer for this record')
  })

  it('applies only the ticked entity', async () => {
    getEntity.mockResolvedValue({ properties: { priority: 'low' } })
    const w = mountDialog()
    // The row box of TKT-2 ticks its one change.
    await w.findAll('input[type="checkbox"]')[2].setValue(true)
    await applyButton(w).trigger('click')
    await flushPromises()
    expect(getEntity).toHaveBeenCalledTimes(1)
    expect(updateEntity).toHaveBeenCalledWith('ticket', 'TKT-2', {
      properties: { priority: 'medium' },
    })
    expect(updateEntity).toHaveBeenCalledTimes(1)
    expect(w.emitted('applied')).toHaveLength(1)
    expect(w.emitted('close')).toHaveLength(1)
  })

  it('skips a property that changed since the proposal', async () => {
    getEntity.mockResolvedValue({ properties: { priority: 'urgent' } })
    const w = mountDialog()
    await w.findAll('input[type="checkbox"]')[1].setValue(true)
    await applyButton(w).trigger('click')
    await flushPromises()
    expect(updateEntity).not.toHaveBeenCalled()
    expect(w.text()).toContain('Login (TKT-1): Priority changed since the proposal was made')
    expect(w.emitted('close')).toBeUndefined()
  })
})
