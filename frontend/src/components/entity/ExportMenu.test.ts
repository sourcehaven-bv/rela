import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import ExportMenu from './ExportMenu.vue'
import { getTransforms } from '@/api/transforms'

vi.mock('@/api/transforms', () => ({ getTransforms: vi.fn() }))

/**
 * ExportMenu is shared by the entity view, the list view and (since
 * TKT-K7J6FL) the document view. It is generic over a `urlFor` callback, so
 * these tests pin the two behaviours every caller depends on: the menu is
 * invisible when no transforms are registered, and choosing a format navigates
 * to whatever URL the parent built.
 */
describe('ExportMenu', () => {
  beforeEach(() => vi.clearAllMocks())

  it('renders nothing when no transforms are registered', async () => {
    vi.mocked(getTransforms).mockResolvedValue([])

    const wrapper = mount(ExportMenu, { props: { urlFor: (t: string) => `/x?transform=${t}` } })
    await flushPromises()

    expect(wrapper.find('.export-menu').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('Export')
  })

  it('shows the menu and lists each registered transform', async () => {
    vi.mocked(getTransforms).mockResolvedValue([
      { name: 'pdf', produces: 'application/pdf' },
      { name: 'odt', produces: 'application/vnd.oasis.opendocument.text' },
    ])

    const wrapper = mount(ExportMenu, { props: { urlFor: (t: string) => `/x?transform=${t}` } })
    await flushPromises()

    expect(wrapper.find('.export-menu').exists()).toBe(true)
    await wrapper.find('button').trigger('click')

    const items = wrapper.findAll('.export-menu-item')
    expect(items.map((i) => i.text())).toEqual(['pdf', 'odt'])
  })

  it('navigates to the URL the parent builds for the chosen transform', async () => {
    vi.mocked(getTransforms).mockResolvedValue([{ name: 'pdf', produces: 'application/pdf' }])
    // A real navigation is what lets the browser handle Content-Disposition,
    // so the component sets location.href rather than fetching.
    const href = vi.fn()
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: {
        get href() {
          return ''
        },
        set href(v: string) {
          href(v)
        },
      },
    })

    const wrapper = mount(ExportMenu, {
      props: { urlFor: (t: string) => `/api/v1/_documents/sales/_export?transform=${t}` },
    })
    await flushPromises()

    await wrapper.find('button').trigger('click')
    await wrapper.find('.export-menu-item').trigger('click')

    expect(href).toHaveBeenCalledWith('/api/v1/_documents/sales/_export?transform=pdf')
  })

  it('hides the menu when the registry fetch fails, without throwing', async () => {
    vi.mocked(getTransforms).mockRejectedValue(new Error('offline'))

    const wrapper = mount(ExportMenu, { props: { urlFor: (t: string) => `/x?transform=${t}` } })
    await flushPromises()

    expect(wrapper.find('.export-menu').exists()).toBe(false)
  })
})
