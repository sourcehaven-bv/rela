import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import { computed, defineComponent, ref, type Slot } from 'vue'
import { useSchemaStore } from '@/stores/schema'
import { resetDetailPanel, useDetailPanelOutlet } from '@/composables/useDetailPanel'
import InlinePropertyValue from '@/components/forms/InlinePropertyValue.vue'
import EntityPageMenu from '@/components/pages/EntityPageMenu.vue'
import { useEntityPageEditing } from './useEntityPageEditing'
import type { Entity } from '@/types'

const updateEntityMock = vi.fn()
const deleteEntityMock = vi.fn()
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  updateEntity: (...args: unknown[]) => updateEntityMock(...args),
  deleteEntity: (...args: unknown[]) => deleteEntityMock(...args),
}))

const pushMock = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock }),
  RouterLink: { props: ['to'], template: '<a><slot /></a>' },
}))

const confirmMock = vi.fn()
vi.mock('@/composables/useConfirm', () => ({ useConfirm: () => ({ confirm: confirmMock }) }))
vi.mock('@/components/entity/EntityDetailPanel.vue', () => ({
  default: defineComponent({ name: 'EntityDetailPanel', render: () => null }),
}))

// The page header edits the page's own entity: each write is a PATCH naming
// one property, and each control follows the server's affordances.

let pinia: ReturnType<typeof createPinia>
const mounted: VueWrapper[] = []

beforeEach(() => {
  pinia = createPinia()
  setActivePinia(pinia)
  resetDetailPanel()
  updateEntityMock.mockReset().mockResolvedValue({})
  deleteEntityMock.mockReset().mockResolvedValue(undefined)
  confirmMock.mockReset()
  pushMock.mockReset()
  useSchemaStore().entityTypes.set('topic', {
    label: 'Topic',
    primary: 'title',
    properties: {
      title: { type: 'string', required: true },
      gezondheid: { type: 'enum', values: ['op_koers', 'risico'] },
    },
  } as never)
})

afterEach(() => {
  for (const w of mounted.splice(0)) w.unmount()
})

function topic(extra: Partial<Entity> = {}): Entity {
  return {
    id: 'TOP-1',
    type: 'topic',
    _title: 'Website',
    properties: { title: 'Website', gezondheid: 'op_koers' },
    _actions: { update: true, delete: true },
    ...extra,
  } as Entity
}

function setup(entity: Entity) {
  let editing!: ReturnType<typeof useEntityPageEditing>
  const host = mount(
    defineComponent({
      setup() {
        const current = ref(entity)
        editing = useEntityPageEditing({
          entity: computed(() => current.value),
          entityType: computed(() => 'topic'),
          badgeProperty: computed(() => 'gezondheid'),
        })
        return () => null
      },
    }),
    { global: { plugins: [pinia, PiniaColada] } }
  )
  mounted.push(host)
  return editing
}

function render(slot: Slot | undefined) {
  const w = mount(defineComponent({ render: () => slot?.() }), {
    global: { plugins: [pinia, PiniaColada] },
  })
  mounted.push(w)
  return w
}

describe('useEntityPageEditing', () => {
  it('saves a badge pick, and unsets it when cleared', async () => {
    const editing = setup(topic())
    const badge = render(editing.badge.value).getComponent(InlinePropertyValue)
    expect(badge.props('writable')).toBe(true)
    badge.vm.$emit('update', 'risico')
    await flushPromises()
    expect(updateEntityMock).toHaveBeenLastCalledWith('topic', 'TOP-1', {
      properties: { gezondheid: 'risico' },
    })
    badge.vm.$emit('update', undefined)
    await flushPromises()
    expect(updateEntityMock).toHaveBeenLastCalledWith('topic', 'TOP-1', {
      properties_unset: ['gezondheid'],
    })
  })

  it('shows the badge read-only to a reader who may not update, and hides it when empty', () => {
    const readOnly = setup(topic({ _actions: { update: false } }))
    expect(render(readOnly.badge.value).getComponent(InlinePropertyValue).props('writable')).toBe(
      false
    )
    const empty = setup(topic({ _actions: { update: false }, properties: { title: 'Website' } }))
    expect(empty.badge.value).toBeUndefined()
  })

  it('offers Delete only when the server allows it', () => {
    const allowed = render(setup(topic()).menu.value).getComponent(EntityPageMenu)
    expect(allowed.props('canDelete')).toBe(true)
    const denied = render(setup(topic({ _actions: { update: true } })).menu.value).getComponent(
      EntityPageMenu
    )
    expect(denied.props('canDelete')).toBe(false)
  })

  it('deletes after confirmation and leaves the page', async () => {
    const menu = render(setup(topic()).menu.value).getComponent(EntityPageMenu)
    confirmMock.mockResolvedValueOnce(false)
    menu.vm.$emit('delete')
    await flushPromises()
    expect(deleteEntityMock).not.toHaveBeenCalled()

    confirmMock.mockResolvedValueOnce(true)
    menu.vm.$emit('delete')
    await flushPromises()
    expect(deleteEntityMock).toHaveBeenCalledWith('topic', 'TOP-1')
    expect(pushMock).toHaveBeenCalledWith('/')
  })

  it('opens the details in the panel', () => {
    const menu = render(setup(topic()).menu.value).getComponent(EntityPageMenu)
    menu.vm.$emit('details')
    const content = useDetailPanelOutlet().content.value
    expect(content?.mode).toBe('overlay')
    expect(content?.props).toMatchObject({ entityType: 'topic', entityId: 'TOP-1' })
  })
})
