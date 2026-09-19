// The duplicate dialog (TKT-Z8K2FS).
//
// These cover the behaviours that are properties of THIS component rather than
// of the prefill rules (which duplicatePrefill.test.ts owns): the depth cap
// opt-in, the modal-stack registration, the failed-fetch state, and the
// disclosure of properties that could not be carried.

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const getAllEntityRelations = vi.fn()
vi.mock('@/api/entities', () => ({
  getAllEntityRelations: (...args: unknown[]) => getAllEntityRelations(...args),
}))

const provideInlineCreateDepth = vi.fn()
vi.mock('@/composables/useInlineCreate', () => ({
  provideInlineCreateDepth: () => provideInlineCreateDepth(),
  useInlineCreate: () => ({ value: [] }),
  INLINE_CREATE_DEPTH: Symbol('depth'),
}))

const useModalStack = vi.fn()
vi.mock('@/composables/modalStack', () => ({
  useModalStack: (...args: unknown[]) => useModalStack(...args),
  registerModal: vi.fn(),
  unregisterModal: vi.fn(),
  isAnyModalOpen: () => false,
}))

vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: vi.fn().mockResolvedValue(true) }),
}))

vi.mock('@/stores', () => ({
  useSchemaStore: () => ({
    getEntityType: () => ({
      label: 'Ticket',
      properties: { title: { type: 'string' }, shot: { type: 'file' } },
    }),
    getRelationType: (n: string) => ({ label: n === 'blocks' ? 'blocks' : n }),
    duplicateConfigFor: () => undefined,
  }),
}))

import DuplicateModal from './DuplicateModal.vue'

const source = {
  id: 'TKT-001',
  type: 'ticket',
  properties: { title: 'Original', shot: 'a.png' },
  content: 'body',
  _redacted: ['salary'],
} as never

function mountModal() {
  return mount(DuplicateModal, {
    props: { show: true, source, formId: 'create_ticket' },
    global: { stubs: { DynamicForm: true, Teleport: true } },
  })
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
  getAllEntityRelations.mockResolvedValue({
    blocks: [{ id: 'TKT-2', type: 'ticket', direction: 'outgoing' }],
    blockedBy: [{ id: 'TKT-9', type: 'ticket', direction: 'incoming' }],
  })
})

describe('DuplicateModal', () => {
  // AC10c. The cap is structural but opt-in: depth only advances when a host
  // calls this. Without it the embedded form sits at depth 0 and its relation
  // pickers would offer inline-create, opening a modal over this one — which
  // modalStack (a Set) cannot route Escape for.
  it('advances the inline-create depth so it cannot host a nested modal', async () => {
    mountModal()
    await flushPromises()

    expect(provideInlineCreateDepth).toHaveBeenCalled()
  })

  // AC20. The detail page binds Del/Backspace to delete; an unregistered modal
  // would leave that live under an open dialog.
  it('registers with the modal stack', async () => {
    mountModal()
    await flushPromises()

    expect(useModalStack).toHaveBeenCalled()
  })

  it('lists relation groups with counts, outgoing checked and incoming not', async () => {
    const w = mountModal()
    await flushPromises()

    const rows = w.findAll('.duplicate-choice')
    expect(rows).toHaveLength(2)
    const byLabel = Object.fromEntries(
      rows.map((r) => [
        r.find('.duplicate-choice-label').text(),
        (r.find('input').element as HTMLInputElement).checked,
      ])
    )
    expect(byLabel).toEqual({ blocks: true, blockedBy: false })
  })

  // AC17. The server drops every neighbour fail-closed on a store error and
  // still answers 200, so a thrown fetch rendered as the empty state would
  // silently duplicate an entity with none of its edges.
  it('shows an error, not an empty state, when the relation fetch fails', async () => {
    getAllEntityRelations.mockRejectedValue(new Error('boom'))
    const w = mountModal()
    await flushPromises()

    expect(w.find('.duplicate-error').exists()).toBe(true)
    expect(w.text()).toContain('would be missing them')
    // Crucially NOT the "no relations" wording, and no live Confirm.
    expect(w.text()).not.toContain('no relations to carry over')
    expect(w.findAll('.duplicate-choice')).toHaveLength(0)
  })

  // AC4: the empty state is explicit, and distinct from the failure above.
  it('shows an explicit empty state when there are no relations', async () => {
    getAllEntityRelations.mockResolvedValue({})
    const w = mountModal()
    await flushPromises()

    expect(w.text()).toContain('no relations to carry over')
    expect(w.find('.duplicate-error').exists()).toBe(false)
  })

  // AC10a / AC19b. A copy missing fields is acceptable; a copy missing fields
  // silently is not.
  it('names properties that could not be carried', async () => {
    const w = mountModal()
    await flushPromises()
    await w.findAll('.duplicate-actions button')[1].trigger('click')
    await flushPromises()

    const notices = w.find('.duplicate-omitted').text()
    expect(notices).toContain('salary')
    expect(notices).toContain('not visible to you')
    expect(notices).toContain('shot')
    expect(notices).toContain('attached file')
  })
})
