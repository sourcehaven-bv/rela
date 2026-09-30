import { describe, it, expect, beforeEach } from 'vitest'
import { effectScope, h } from 'vue'
import { usePageFrame, usePageHeader, usePageHeaderOutlet, resetPageHeader } from './usePageHeader'

describe('usePageHeader', () => {
  beforeEach(() => {
    resetPageHeader()
  })

  it('starts empty', () => {
    const outlet = usePageHeaderOutlet()
    expect(outlet.present.value).toBe(false)
    expect(outlet.content.value).toBeNull()
  })

  it('shows what a view puts in it', () => {
    const outlet = usePageHeaderOutlet()
    const header = usePageHeader()

    header.show({ title: 'Features' })

    expect(outlet.present.value).toBe(true)
    expect(outlet.content.value?.title).toBe('Features')
  })

  it('carries the view’s tools and actions as slots', () => {
    // Slots rather than props so the view's own template refs (the search
    // box, the filter menu) still resolve from the view that owns them.
    const outlet = usePageHeaderOutlet()
    const header = usePageHeader()

    header.show({
      title: 'Features',
      tools: () => [h('input')],
      actions: () => [h('button')],
    })

    expect(outlet.content.value?.tools).toBeTypeOf('function')
    expect(outlet.content.value?.actions).toBeTypeOf('function')
  })

  it('replaces the previous view’s header rather than stacking', () => {
    const outlet = usePageHeaderOutlet()
    const header = usePageHeader()

    header.show({ title: 'Features' })
    header.show({ title: 'Ideas' })

    expect(outlet.content.value?.title).toBe('Ideas')
  })

  it('clears when the owning view is torn down', () => {
    // Why this matters: a screen that sets no header must not inherit the
    // previous screen's title. Without the scope clear, navigating from a
    // list to settings would leave "Features" above the settings page.
    const outlet = usePageHeaderOutlet()
    const scope = effectScope()
    scope.run(() => {
      usePageHeader().show({ title: 'Features' })
    })

    expect(outlet.present.value).toBe(true)

    scope.stop()

    expect(outlet.present.value).toBe(false)
  })

  it('keeps a page’s title and tabs beside the tab view’s own header', () => {
    // The page and the view in its active tab fill the header together: the
    // page names itself and its tabs, the view brings its tools.
    const outlet = usePageHeaderOutlet()
    const header = usePageHeader()
    const frame = usePageFrame()

    frame.show({ title: 'Tickets', tabs: [{ id: 'table', label: 'Table' }], active: 'table', select: () => {} })
    header.show({ title: 'All tickets', tools: () => [h('input')] })

    expect(outlet.title.value).toBe('Tickets')
    expect(outlet.frame.value?.active).toBe('table')
    expect(outlet.content.value?.tools).toBeTypeOf('function')

    header.clear()
    expect(outlet.present.value).toBe(true)
    frame.clear()
    expect(outlet.present.value).toBe(false)
  })
})
