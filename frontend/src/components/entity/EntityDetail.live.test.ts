import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityDetail from './EntityDetail.vue'
import { useSchemaStore } from '@/stores/schema'
import type { Entity } from '@/types'
import type { ViewResponse } from '@/api'

// The live-view refresh (TKT-HJ5Z9Y) replaces the view in the background. It
// must not land over another entity, and must not replace an optimistic
// checkbox toggle that autosave still holds.

const fetchViewMock = vi.fn()
const updateEntityMock = vi.fn()

vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  fetchView: (...a: unknown[]) => fetchViewMock(...a),
  getCommands: vi.fn().mockResolvedValue([]),
}))
vi.mock('@/api/entities', async (orig) => ({
  ...(await orig<typeof import('@/api/entities')>()),
  updateEntity: (...a: unknown[]) => updateEntityMock(...a),
}))

// Captures the live subscription's onChange, which the stream would call.
let liveChanged: () => void = () => {}
vi.mock('@/composables/useLiveSubscription', async (orig) => ({
  ...(await orig<typeof import('@/composables/useLiveSubscription')>()),
  useLiveSubscription: (_label: string, _path: unknown, onChange: () => void) => {
    liveChanged = onChange
  },
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/entity/task/TASK-1', name: 'entity' }),
  RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
}))

function view(id: string, title: string, content = ''): ViewResponse {
  const entry: Entity = {
    id,
    type: 'task',
    _title: title,
    properties: { title },
    content,
    _actions: { update: true },
  }
  return {
    entry,
    sections: content
      ? [{ heading: '', sectionId: 'content', display: 'content', hasContent: true, content, isEmpty: false, isGrouped: false }]
      : [],
  } as ViewResponse
}

function deferred<T>() {
  let resolve!: (v: T) => void
  const promise = new Promise<T>((r) => (resolve = r))
  return { promise, resolve }
}

describe('EntityDetail live refresh', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    useSchemaStore().entityTypes.set('task', {
      name: 'task',
      label: 'Task',
      properties: { title: { type: 'string', values: null } },
    } as never)
    fetchViewMock.mockReset()
    updateEntityMock.mockReset()
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  async function mountDetail(resp: ViewResponse) {
    fetchViewMock.mockResolvedValue(resp)
    const w = mount(EntityDetail, {
      props: { entityType: 'task', entityId: resp.entry!.id },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()
    return w
  }

  it('drops a refresh that resolves after navigating to another entity', async () => {
    const w = await mountDetail(view('TASK-1', 'First'))
    const late = deferred<ViewResponse>()
    fetchViewMock.mockReturnValueOnce(late.promise)
    liveChanged()

    fetchViewMock.mockResolvedValue(view('TASK-2', 'Second'))
    await w.setProps({ entityId: 'TASK-2' } as never)
    await flushPromises()
    expect(w.text()).toContain('Second')

    late.resolve(view('TASK-1', 'First again'))
    await flushPromises()
    expect(w.text()).toContain('Second')
    expect(w.text()).not.toContain('First again')
  })

  it('waits for autosave before replacing an optimistic checkbox toggle', async () => {
    const w = await mountDetail(view('TASK-1', 'First', '- [ ] a task'))
    const save = deferred<Entity>()
    updateEntityMock.mockReturnValue(save.promise)
    await w.find('input[type="checkbox"][data-cb-idx]').trigger('click')

    fetchViewMock.mockClear()
    liveChanged()
    await flushPromises()
    expect(fetchViewMock).not.toHaveBeenCalled()
    expect(w.find<HTMLInputElement>('input[type="checkbox"][data-cb-idx]').element.checked).toBe(true)

    // Once the save settles, the deferred refresh runs.
    fetchViewMock.mockResolvedValue(view('TASK-1', 'First', '- [x] a task'))
    await vi.waitFor(() => expect(updateEntityMock).toHaveBeenCalled())
    save.resolve(view('TASK-1', 'First', '- [x] a task').entry!)
    await vi.waitFor(() => expect(fetchViewMock).toHaveBeenCalledTimes(1))
  })
})
