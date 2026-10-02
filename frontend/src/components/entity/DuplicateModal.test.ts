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

function mountModal(props: Record<string, unknown> = {}) {
  return mount(DuplicateModal, {
    props: { source, formId: 'create_ticket', ...props },
    global: { stubs: { DynamicForm: true, Teleport: true } },
  })
}

// Clicks Continue so the embedded form mounts.
async function continueToForm(w: ReturnType<typeof mountModal>) {
  const proceed = w.findAll('button').find((b) => b.text() === 'Continue')
  if (!proceed) throw new Error('no Continue button')
  await proceed.trigger('click')
  await flushPromises()
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

  // The dialog's lifetime IS the dialog: the host mounts it under v-if, so
  // focus must return to the trigger on UNMOUNT. A `show`-prop watch could
  // never observe that transition, because the unmount beats it.
  it('returns focus to the element that opened it', async () => {
    const trigger = document.createElement('button')
    document.body.appendChild(trigger)
    trigger.focus()

    const w = mountModal()
    await flushPromises()
    w.unmount()

    expect(document.activeElement).toBe(trigger)
    trigger.remove()
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

    // RlCheckbox renders the label text and the native input together, so the
    // pair is read off the rendered control rather than off rela classes.
    const rows = w.findAll('.rl-checkbox')
    expect(rows).toHaveLength(2)
    const byLabel = Object.fromEntries(
      rows.map((r) => [
        (r.element.closest('label') ?? r.element).textContent?.trim().split(' (')[0],
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

    expect(w.text()).toContain('would be missing them')
    expect(w.text()).toContain('boom')
    // Crucially NOT the "no relations" wording, and no live Confirm.
    expect(w.text()).not.toContain('no relations to carry over')
    expect(w.findAll('.rl-checkbox')).toHaveLength(0)
  })

  // AC4: the empty state is explicit, and distinct from the failure above.
  it('shows an explicit empty state when there are no relations', async () => {
    getAllEntityRelations.mockResolvedValue({})
    const w = mountModal()
    await flushPromises()

    expect(w.text()).toContain('no relations to carry over')
    expect(w.text()).not.toContain('would be missing them')
  })

  // AC10a / AC19b. A copy missing fields is acceptable; a copy missing fields
  // silently is not.
  it('names properties that could not be carried', async () => {
    const w = mountModal()
    await flushPromises()
    const proceed = w.findAll('button').find((b) => b.text() === 'Continue')
    await proceed!.trigger('click')
    await flushPromises()

    const notices = w.find('.duplicate-omitted').text()
    expect(notices).toContain('salary')
    expect(notices).toContain('not visible to you')
    expect(notices).toContain('shot')
    expect(notices).toContain('attached file')
  })

  // BUG-FYEEVX. The relations sub-resource refuses `?world=` (422), and a
  // content-scoped edge belongs to one face, so the read goes to the source's
  // ADDRESS. A copy of a face lands on that face, whatever the world.
  describe('a source on a named face', () => {
    const draft = {
      id: 'POL-1',
      type: 'policy',
      _self: '/api/v1/policies/POL-1@draft',
      properties: { title: 'Draft' },
    } as never

    it('reads the relations of the face on screen, with no world', async () => {
      mountModal({ source: draft, world: 'published' })
      await flushPromises()
      expect(getAllEntityRelations).toHaveBeenCalledWith('policy', 'POL-1@draft')
    })

    it('creates the copy on that face rather than in the world', async () => {
      const w = mountModal({ source: draft, world: 'published' })
      await flushPromises()
      await continueToForm(w)
      const form = w.findComponent({ name: 'DynamicForm' })
      expect(form.props('embeddedFace')).toBe('draft')
      expect(form.props('embeddedWorld')).toBeUndefined()
    })

    it.each([
      ['a fallback', { name: 'editorial', face: 'published', via: 'fallback-default' }],
      [
        'a later chain entry',
        { name: 'editorial', face: 'published', via: 'chain', chain_position: 1 },
      ],
    ])('leaves the copy to the world when the face stands in by %s', async (_label, served) => {
      const standIn = {
        id: 'POL-1',
        type: 'policy',
        _self: '/api/v1/policies/POL-1@published',
        _world: served,
        properties: { title: 'Published' },
      } as never
      const w = mountModal({ source: standIn, world: 'editorial' })
      await flushPromises()
      // The relations on screen are still those of the served face.
      expect(getAllEntityRelations).toHaveBeenCalledWith('policy', 'POL-1@published')
      await continueToForm(w)
      const form = w.findComponent({ name: 'DynamicForm' })
      expect(form.props('embeddedWorld')).toBe('editorial')
      expect(form.props('embeddedFace')).toBeUndefined()
    })

    it('keeps the world for a source on the bare face', async () => {
      const w = mountModal({ world: 'editorial' })
      await flushPromises()
      expect(getAllEntityRelations).toHaveBeenCalledWith('ticket', 'TKT-001')
      await continueToForm(w)
      const form = w.findComponent({ name: 'DynamicForm' })
      expect(form.props('embeddedWorld')).toBe('editorial')
      expect(form.props('embeddedFace')).toBeUndefined()
    })
  })
})
