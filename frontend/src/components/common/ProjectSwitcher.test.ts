import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ProjectSwitcher from './ProjectSwitcher.vue'

const two = [
  { id: 'aaa', name: 'Alpha', root: '/a', href: '/p/aaa/', active: true },
  { id: 'bbb', name: 'Beta', root: '/b', href: '/p/bbb/', active: false },
]

function mockFetch(body: unknown, ok = true) {
  return vi.fn().mockResolvedValue({
    ok,
    json: () => Promise.resolve(body),
  } as Response)
}

describe('ProjectSwitcher', () => {
  beforeEach(() => {
    // The switcher only queries inside the desktop shell, which is the only
    // thing that injects the Wails runtime.
    vi.stubGlobal('wails', {})
    vi.stubGlobal('fetch', mockFetch(two))
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('shows the active project when several are open', async () => {
    const w = mount(ProjectSwitcher)
    await flushPromises()
    expect(w.text()).toContain('Alpha')
  })

  it('lists the others once opened', async () => {
    const w = mount(ProjectSwitcher)
    await flushPromises()
    await w.find('.switcher-btn').trigger('click')
    expect(w.text()).toContain('Beta')
    expect(w.text()).toContain('/b')
  })

  // One project is not a choice; rendering a switcher would be noise.
  it('stays hidden with a single project', async () => {
    vi.stubGlobal('fetch', mockFetch([two[0]]))
    const w = mount(ProjectSwitcher)
    await flushPromises()
    expect(w.find('.switcher-btn').exists()).toBe(false)
  })

  // On rela-server the endpoint does not exist. The switcher must stay out of
  // the way rather than surface an error.
  it('stays hidden when the endpoint is absent', async () => {
    vi.stubGlobal('fetch', mockFetch(null, false))
    const w = mount(ProjectSwitcher)
    await flushPromises()
    expect(w.find('.switcher-btn').exists()).toBe(false)
  })

  // On rela-server there is no Wails runtime, so the switcher must not even
  // ask — a request per page load is one `networkidle` never needed to wait on.
  it('does not fetch outside the desktop shell', async () => {
    vi.unstubAllGlobals()
    const spy = mockFetch(two)
    vi.stubGlobal('fetch', spy)
    const w = mount(ProjectSwitcher)
    await flushPromises()
    expect(spy).not.toHaveBeenCalled()
    expect(w.find('.switcher-btn').exists()).toBe(false)
  })

  it('stays hidden when the fetch rejects', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')))
    const w = mount(ProjectSwitcher)
    await flushPromises()
    expect(w.find('.switcher-btn').exists()).toBe(false)
  })

  // Each project is a different history base, so the router of this page
  // cannot address another project's routes — it has to be a real navigation.
  it('navigates to the chosen project', async () => {
    const href = { value: '' }
    Object.defineProperty(window, 'location', {
      value: {
        get href() {
          return href.value
        },
        set href(v: string) {
          href.value = v
        },
      },
      writable: true,
    })

    const w = mount(ProjectSwitcher)
    await flushPromises()
    await w.find('.switcher-btn').trigger('click')
    const items = w.findAll('.switcher-item')
    await items[1].trigger('click') // Beta

    expect(href.value).toBe('/p/bbb/')
  })
})
