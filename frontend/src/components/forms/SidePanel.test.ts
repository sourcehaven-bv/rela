// The side panel's "+ Add <Type>" button must open a create form that is
// actually pre-linked (TKT-R4BMJM, AC8).
//
// It did not. `createNewForSection` pushed `_relation` / `_linkAs` / `_peerId`,
// and `DynamicForm` reads `link_relation` / `link_peer` / `link_as`. A
// repo-wide grep found one write site and zero read sites for the underscore
// spelling, so the button navigated to a create form with no relation context
// and the user had to link by hand — exactly the chore it exists to remove.
//
// The assertion below deliberately checks the names the FORM parses, not the
// names this component happens to emit. A test written the other way round
// passes against the bug.

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import SidePanel from './SidePanel.vue'
import type { SidePanelSection } from '@/types'

const pushMock = vi.fn()
vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRouter: () => ({ push: pushMock }),
    RouterLink: { name: 'RouterLink', template: '<a><slot /></a>', props: ['to'] },
  }
})

const getMock = vi.fn()
vi.mock('@/api/client', () => ({
  api: {
    get: (...args: unknown[]) => getMock(...args),
  },
}))

/** A section with one add target, as the server resolves it. */
function sectionWithAdd(): SidePanelSection {
  return {
    heading: 'Features',
    sectionId: 'features',
    display: 'cards',
    isEmpty: false,
    entities: [],
    addInfo: {
      relation: 'implements',
      linkAs: 'to',
      peerId: 'TKT-001',
      targets: [{ entityType: 'feature', formId: 'create_feature', label: 'Feature' }],
    },
  }
}

async function mountPanel(sections: SidePanelSection[]) {
  getMock.mockResolvedValue(sections)
  const wrapper = mount(SidePanel, {
    props: { formId: 'ticketform', entityId: 'TKT-001' },
    global: {
      stubs: { Badge: true },
    },
  })
  await flushPromises()
  return wrapper
}

describe('SidePanel add button', () => {
  beforeEach(() => {
    pushMock.mockClear()
    getMock.mockReset()
  })

  it('pre-links the new entity using the params DynamicForm reads', async () => {
    const wrapper = await mountPanel([sectionWithAdd()])

    // `.btn-add`, not a text match: the section HEADER button also contains
    // "Features", and matching on text finds the collapse toggle instead.
    const addBtn = wrapper.find('button.btn-add')
    expect(addBtn.exists(), 'the add button should render for a resolved target').toBe(true)
    expect(addBtn.text()).toContain('Feature')

    await addBtn.trigger('click')

    expect(pushMock).toHaveBeenCalledTimes(1)
    const target = pushMock.mock.calls[0][0] as { path: string; query: Record<string, string> }

    expect(target.path).toBe('/form/create_feature')
    // These three names are what DynamicForm's initializeDefaults parses. The
    // bug was that this component emitted a different spelling entirely.
    expect(target.query).toMatchObject({
      link_relation: 'implements',
      link_peer: 'TKT-001',
      link_as: 'to',
    })
  })

  it('emits no underscore-prefixed params', async () => {
    const wrapper = await mountPanel([sectionWithAdd()])
    await wrapper.find('button.btn-add').trigger('click')

    const target = pushMock.mock.calls[0][0] as { query: Record<string, string> }
    const underscored = Object.keys(target.query).filter((k) => k.startsWith('_'))
    expect(underscored, 'the dead underscore spelling must not come back').toEqual([])
  })
})
