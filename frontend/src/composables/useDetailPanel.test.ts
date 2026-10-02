import { describe, it, expect, beforeEach } from 'vitest'
import { defineComponent, effectScope, h } from 'vue'
import {
  useDetailPanel,
  useDetailPanelOutlet,
  resetDetailPanel,
} from './useDetailPanel'

const Stub = defineComponent({ setup: () => () => h('div') })

describe('useDetailPanel', () => {
  beforeEach(() => {
    resetDetailPanel()
  })

  it('starts closed', () => {
    const outlet = useDetailPanelOutlet()
    expect(outlet.open.value).toBe(false)
    expect(outlet.content.value).toBeNull()
  })

  it('shows what a view puts in it', () => {
    const outlet = useDetailPanelOutlet()
    const panel = useDetailPanel()

    panel.show({ component: Stub, props: { entityId: 'FEAT-1' }, mode: 'inline' })

    expect(outlet.open.value).toBe(true)
    expect(outlet.content.value?.props).toEqual({ entityId: 'FEAT-1' })
    expect(outlet.mode.value).toBe('inline')
  })

  it('reports the mode the view asked for', () => {
    const outlet = useDetailPanelOutlet()
    const panel = useDetailPanel()

    panel.show({ component: Stub, props: {}, mode: 'overlay' })

    expect(outlet.mode.value).toBe('overlay')
  })

  it('falls back to inline when closed, rather than reporting a stale mode', () => {
    const outlet = useDetailPanelOutlet()
    const panel = useDetailPanel()

    panel.show({ component: Stub, props: {}, mode: 'overlay' })
    panel.clear()

    expect(outlet.mode.value).toBe('inline')
  })

  it('clears when the owning view is torn down', () => {
    // The reason this matters: without it, navigating away from a list with
    // an open panel leaves the panel rendered over the next screen, which
    // has no idea it is there.
    const outlet = useDetailPanelOutlet()
    const scope = effectScope()
    scope.run(() => {
      useDetailPanel().show({ component: Stub, props: {}, mode: 'inline' })
    })

    expect(outlet.open.value).toBe(true)

    scope.stop()

    expect(outlet.open.value).toBe(false)
  })

  it("leaves another writer's content alone", () => {
    const outlet = useDetailPanelOutlet()
    const page = useDetailPanel()
    const list = useDetailPanel()

    list.show({ component: Stub, props: { entityId: 'ROW-1' }, mode: 'inline' })
    page.show({ component: Stub, props: { entityId: 'PAGE-1' }, mode: 'overlay' })
    list.clear()

    expect(outlet.content.value?.props).toEqual({ entityId: 'PAGE-1' })

    page.clear()
    expect(outlet.open.value).toBe(false)
  })
})
