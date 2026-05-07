// CustomView regression tests for issue #647.
//
// The bug: clicking an item in a `display: list` section does nothing
// because the click handler called router.push({ name: 'entity', params:
// { id } }) against a route that requires :type/:id. The fix renders the
// item as a real <a :href> so right-click / cmd-click work, and the
// click handler now resolves a valid path through entityDetailHref.
//
// These tests assert both contracts: rendered href is the right path,
// and a left-click pushes the same path with return_to context.

import { setActivePinia, createPinia } from 'pinia'
import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { createRouter, createMemoryHistory } from 'vue-router'

import CustomView from './CustomView.vue'
import { useSchemaStore } from '@/stores'
import { fetchView } from '@/api'

vi.mock('@/api', async () => {
  const actual = await vi.importActual<typeof import('@/api')>('@/api')
  return { ...actual, fetchView: vi.fn() }
})

// useScopeNavigation makes its own GET; stub it so the mounted component
// doesn't reach for the network and its watch settles cleanly.
vi.mock('@/composables', async () => {
  const actual = await vi.importActual<typeof import('@/composables')>('@/composables')
  return {
    ...actual,
    useScopeNavigation: () => ({
      scopeNav: ref(null),
      loadScopeNav: vi.fn().mockResolvedValue(undefined),
      navigateScope: vi.fn(),
    }),
  }
})

function makeRouter() {
  // Routes match the real router at the destinations we navigate to so
  // router.push doesn't reject with MISSING_REQUIRED_PARAMS.
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div/>' } },
      {
        path: '/view/:id/:entityId',
        name: 'view',
        component: { template: '<div/>' },
      },
      {
        path: '/entity/:type/:id',
        name: 'entity',
        component: { template: '<div/>' },
      },
    ],
  })
}

async function mountWithSection(
  section: Record<string, unknown>,
  opts: { detailView?: string } = {}
) {
  const router = makeRouter()
  // Land on a /view/... route so route.params reflects the CustomView context.
  await router.push('/view/policy_detail/POLICY-001')
  await router.isReady()

  const fetchViewMock = vi.mocked(fetchView)
  fetchViewMock.mockResolvedValue({
    entry: {
      id: 'POLICY-001',
      type: 'policy',
      properties: { title: 'A policy' },
    },
    sections: [section],
  } as unknown as Awaited<ReturnType<typeof fetchView>>)

  const wrapper = mount(CustomView, {
    props: { id: 'policy_detail', entityId: 'POLICY-001' },
    global: { plugins: [router] },
  })

  // Seed the schema store after Pinia is active.
  const schemaStore = useSchemaStore()
  if (opts.detailView) {
    schemaStore.entityViewConfigs.set('procedure', { detail_view: opts.detailView })
  }

  await flushPromises()
  return { wrapper, router }
}

describe('CustomView — list section navigation (issue #647)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders list items as real <a href> elements', async () => {
    const { wrapper } = await mountWithSection({
      sectionId: 'related',
      heading: 'Related',
      display: 'list',
      entities: [{ id: 'PROC-1', type: 'procedure', title: 'First' }],
      isEmpty: false,
    })

    const link = wrapper.find('a.list-link')
    expect(link.exists()).toBe(true)
    // No detail_view configured → /entity/:type/:id floor.
    expect(link.attributes('href')).toBe('/entity/procedure/PROC-1')
  })

  it('uses entity_views detail_view when configured', async () => {
    const { wrapper } = await mountWithSection(
      {
        sectionId: 'related',
        heading: 'Related',
        display: 'list',
        entities: [{ id: 'PROC-2', type: 'procedure', title: 'Second' }],
        isEmpty: false,
      },
      { detailView: 'detail_procedure' }
    )

    const link = wrapper.find('a.list-link')
    expect(link.attributes('href')).toBe('/view/detail_procedure/PROC-2')
  })

  it('clicking a list item routes to the entity with return_to query', async () => {
    const { wrapper, router } = await mountWithSection({
      sectionId: 'related',
      heading: 'Related',
      display: 'list',
      entities: [{ id: 'PROC-3', type: 'procedure', title: 'Third' }],
      isEmpty: false,
    })

    const link = wrapper.find('a.list-link')
    expect(link.exists()).toBe(true)

    await link.trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/entity/procedure/PROC-3')
    expect(router.currentRoute.value.query.return_to).toBe('/view/policy_detail/POLICY-001')
  })
})
