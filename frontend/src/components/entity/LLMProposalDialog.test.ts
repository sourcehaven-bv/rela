// The proposal dialog of an `llm:` action (TKT-4USH1HO): it writes only what
// the user checks, through the ordinary write APIs, and skips a property that
// changed since the proposal.

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const getEntity = vi.fn()
const updateEntity = vi.fn()
const createRelation = vi.fn()
vi.mock('@/api/entities', () => ({
  getEntity: (...a: unknown[]) => getEntity(...a),
  updateEntity: (...a: unknown[]) => updateEntity(...a),
  createRelation: (...a: unknown[]) => createRelation(...a),
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
    getEntityType: () => ({
      properties: {
        priority: { type: 'enum', label: 'Priority' },
        summary: { type: 'string', label: 'Summary' },
        contact: { type: 'email', label: 'Contact' },
        points: { type: 'integer', label: 'Points' },
      },
    }),
    getRelationType: () => ({ label: 'Affects' }),
  }),
  useUIStore: () => ({ success }),
}))

import LLMProposalDialog from './LLMProposalDialog.vue'
import type { ActionProposal } from '@/api/actions'

const proposal: ActionProposal = {
  entity: 'TKT-1',
  set: [{ property: 'priority', current: 'low', proposed: 'high' }],
  relate: [
    { relation: 'affects', to: 'CON-1', to_type: 'concept', to_title: 'Sessions' },
    { relation: 'affects', to: 'CON-2', to_type: 'concept', to_title: 'Tokens' },
  ],
}

function mountDialog() {
  return mount(LLMProposalDialog, {
    props: { proposal, entityType: 'ticket', label: 'Triage' },
    global: { stubs: { Teleport: true } },
  })
}

async function clickApply(w: ReturnType<typeof mountDialog>) {
  const btn = w.findAll('button').find((b) => b.text() === 'Apply selected')
  if (!btn) throw new Error('no Apply button')
  await btn.trigger('click')
  await flushPromises()
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
  updateEntity.mockResolvedValue({})
  createRelation.mockResolvedValue(undefined)
})

describe('LLMProposalDialog', () => {
  it('applies only the checked changes', async () => {
    getEntity.mockResolvedValue({ properties: { priority: 'low' } })
    const w = mountDialog()
    // Uncheck the second relation.
    const boxes = w.findAll('input[type="checkbox"]')
    expect(boxes).toHaveLength(3)
    await boxes[2].setValue(false)
    await clickApply(w)

    expect(updateEntity).toHaveBeenCalledWith('ticket', 'TKT-1', {
      properties: { priority: 'high' },
    })
    expect(createRelation).toHaveBeenCalledTimes(1)
    expect(createRelation).toHaveBeenCalledWith('ticket', 'TKT-1', 'affects', 'CON-1')
    expect(success).toHaveBeenCalled()
    expect(w.emitted('close')).toBeTruthy()
  })

  it('skips a property that changed and reports it', async () => {
    getEntity.mockResolvedValue({ properties: { priority: 'medium' } })
    const w = mountDialog()
    await clickApply(w)

    expect(updateEntity).not.toHaveBeenCalled()
    expect(createRelation).toHaveBeenCalledTimes(2)
    expect(w.text()).toContain('Priority changed since the proposal')
    expect(w.emitted('close')).toBeFalsy()
  })

  it('reports a failed relation and keeps the dialog open', async () => {
    getEntity.mockResolvedValue({ properties: { priority: 'low' } })
    createRelation.mockRejectedValueOnce(new Error('nope'))
    const w = mountDialog()
    await clickApply(w)

    expect(w.text()).toContain('Could not add Affects to Sessions')
    expect(w.emitted('applied')).toBeTruthy()
    expect(w.emitted('close')).toBeFalsy()
  })

  it('starts a free-text value unticked', async () => {
    getEntity.mockResolvedValue({ properties: { priority: 'low', summary: 'old' } })
    const w = mount(LLMProposalDialog, {
      props: {
        proposal: {
          ...proposal,
          set: [
            ...proposal.set,
            { property: 'summary', current: 'old', proposed: 'injected text' },
          ],
        },
        entityType: 'ticket',
        label: 'Triage',
      },
      global: { stubs: { Teleport: true } },
    })
    const boxes = w.findAll('input[type="checkbox"]')
    expect(boxes.map((b) => (b.element as HTMLInputElement).checked)).toEqual([
      true,
      false,
      true,
      true,
    ])
    await clickApply(w)
    expect(updateEntity).toHaveBeenCalledWith('ticket', 'TKT-1', {
      properties: { priority: 'high' },
    })
  })

  it('starts a pattern-only custom type unticked and a number ticked', async () => {
    const w = mount(LLMProposalDialog, {
      props: {
        proposal: {
          ...proposal,
          set: [
            { property: 'contact', current: null, proposed: 'a@example.test' },
            { property: 'points', current: 1, proposed: 3 },
          ],
          relate: [],
        },
        entityType: 'ticket',
        label: 'Triage',
      },
      global: { stubs: { Teleport: true } },
    })
    const boxes = w.findAll('input[type="checkbox"]')
    expect(boxes.map((b) => (b.element as HTMLInputElement).checked)).toEqual([false, true])
  })

  it('applying again retries only what did not go through', async () => {
    getEntity.mockResolvedValue({ properties: { priority: 'low' } })
    createRelation.mockRejectedValueOnce(new Error('nope'))
    const w = mountDialog()
    await clickApply(w)
    // The saved property and the created relation are locked and unchecked.
    const boxes = w.findAll('input[type="checkbox"]')
    expect(boxes.map((b) => (b.element as HTMLInputElement).disabled)).toEqual([true, false, true])

    vi.clearAllMocks()
    getEntity.mockResolvedValue({ properties: { priority: 'high' } })
    await clickApply(w)

    expect(updateEntity).not.toHaveBeenCalled()
    expect(createRelation).toHaveBeenCalledTimes(1)
    expect(createRelation).toHaveBeenCalledWith('ticket', 'TKT-1', 'affects', 'CON-1')
    expect(w.emitted('close')).toBeTruthy()
  })
})
