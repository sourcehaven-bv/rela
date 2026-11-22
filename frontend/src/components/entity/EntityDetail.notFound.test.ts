import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { PiniaColada } from '@pinia/colada'
import EntityDetail from './EntityDetail.vue'
import { ApiError } from '@/api/errors'
import { useSchemaStore } from '@/stores/schema'

// A missing entity, or one the reader may not see, is the uniform 404. The
// page shows the plain not-found state with a way home, not the raw error
// and a Retry that cannot help. Another failure keeps the error and Retry.

const fetchViewMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  fetchView: (...a: unknown[]) => fetchViewMock(...a),
  getCommands: async () => [],
}))
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/entity/account/ACC-99999', name: 'entity' }),
  RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
}))

function failure(status: number, code: string, title: string): ApiError {
  return new ApiError(title, {
    kind: 'http',
    status,
    problem: { type: `https://rela.dev/errors/${code}`, title, status },
    original: null,
  })
}

describe('EntityDetail for an entity that cannot be served', () => {
  let pinia: ReturnType<typeof createPinia>

  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    useSchemaStore().entityTypes.set('account', {
      name: 'account',
      label: 'Account',
      properties: {},
    } as never)
    fetchViewMock.mockReset()
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.restoreAllMocks()
    document.body.innerHTML = ''
  })

  async function mountDetail(err: ApiError) {
    fetchViewMock.mockRejectedValue(err)
    const wrapper = mount(EntityDetail, {
      props: { entityType: 'account', entityId: 'ACC-99999' },
      attachTo: document.body,
      global: { plugins: [pinia, PiniaColada] },
    })
    await flushPromises()
    return wrapper
  }

  it('shows the not-found state with a way home for a 404', async () => {
    const w = await mountDetail(failure(404, 'not_found', 'Entity not found'))

    expect(w.find('[data-testid="not-found-state"]').exists()).toBe(true)
    expect(w.find('[data-testid="not-found-home"]').attributes('href')).toBe('/')
    expect(w.text()).not.toContain('Entity not found')
    expect(w.text()).not.toContain('Retry')
    // An answer, not a fault: nothing is logged.
    expect(console.error).not.toHaveBeenCalled()
  })

  it('keeps the error and Retry for another failure', async () => {
    const w = await mountDetail(failure(500, 'internal', 'Something broke'))

    expect(w.find('[data-testid="not-found-state"]').exists()).toBe(false)
    expect(w.text()).toContain('Something broke')
    expect(w.text()).toContain('Retry')
  })
})
