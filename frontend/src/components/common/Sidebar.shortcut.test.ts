import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { createRouter, createWebHistory, type Router } from 'vue-router'
import Sidebar from './Sidebar.vue'

/**
 * Regression coverage for BUG-DNP5E7.
 *
 * The Sidebar registers its `/` handler on `document` in onMounted, so it is
 * live on every route. It used to guard with an inline
 * `['INPUT','TEXTAREA'].includes(e.target.tagName)` check, which cannot see a
 * `contenteditable` host — so pressing `/` inside the Milkdown editor navigated
 * to /search, throwing the user out of the document mid-sentence and shadowing
 * Milkdown's own slash-command menu.
 *
 * These tests mount the real component rather than re-testing the guard helper,
 * because the defect was never in the helper: it was a handler that did not
 * call it. Only mounting can see that.
 *
 * Teardown note: unmounting in `afterEach` rather than at the end of each test
 * body is load-bearing, not tidiness. Vitest throws on a failed `expect`, so a
 * trailing `wrapper.unmount()` is unreachable exactly when a test fails — and
 * the Sidebar's listener is bound to `document`, which `innerHTML = ''` does
 * not clear. One genuine failure would then leak a live handler into every
 * later test in this file, inverting the unmount test below and sending the
 * next reader after a teardown bug that does not exist.
 */
describe('Sidebar / search shortcut', () => {
  let router: Router
  let wrapper: VueWrapper | null = null

  beforeEach(async () => {
    router = createRouter({
      history: createWebHistory(),
      routes: [
        { path: '/', name: 'home', component: { template: '<div/>' } },
        { path: '/search', name: 'search', component: { template: '<div/>' } },
      ],
    })
    await router.push('/')
    await router.isReady()
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
    document.body.innerHTML = ''
    vi.restoreAllMocks()
  })

  async function mountSidebar() {
    wrapper = mount(Sidebar, { global: { plugins: [router] } })
    await router.isReady()
    return wrapper
  }

  function pressSlash() {
    document.dispatchEvent(new KeyboardEvent('keydown', { key: '/', bubbles: true }))
  }

  /** Appends el, focuses it, and returns it. */
  function focus<T extends HTMLElement>(el: T): T {
    document.body.appendChild(el)
    el.focus()
    return el
  }

  function milkdownEditor(): HTMLElement {
    const el = document.createElement('div')
    el.setAttribute('contenteditable', 'true')
    el.className = 'ProseMirror milkdown-prose'
    return el
  }

  it('navigates to /search when focus is not in a text surface', async () => {
    await mountSidebar()
    const push = vi.spyOn(router, 'push')

    pressSlash()

    expect(push).toHaveBeenCalledWith('/search')
  })

  it('does not navigate while the Milkdown editor has focus', async () => {
    await mountSidebar()
    focus(milkdownEditor())

    const push = vi.spyOn(router, 'push')
    pressSlash()

    expect(push).not.toHaveBeenCalled()
  })

  it.each([
    ['input', () => document.createElement('input')],
    ['textarea', () => document.createElement('textarea')],
  ])('does not navigate while a <%s> has focus', async (_name, make) => {
    await mountSidebar()
    focus(make())

    const push = vi.spyOn(router, 'push')
    pressSlash()

    expect(push).not.toHaveBeenCalled()
  })

  it('still defers to a list view that owns its own search box (TKT-603FQ)', async () => {
    await mountSidebar()

    const list = document.createElement('div')
    list.className = 'entity-list'
    const box = document.createElement('div')
    box.className = 'search-box'
    list.appendChild(box)
    document.body.appendChild(list)

    const push = vi.spyOn(router, 'push')
    pressSlash()

    expect(push).not.toHaveBeenCalled()
  })

  it('unregisters its handler on unmount', async () => {
    const w = await mountSidebar()
    w.unmount()
    wrapper = null // already unmounted; keep afterEach from double-unmounting

    const push = vi.spyOn(router, 'push')
    pressSlash()

    expect(push).not.toHaveBeenCalled()
  })

  it('registers exactly one handler per mount', async () => {
    // Pins the teardown contract the afterEach above exists to uphold. If a
    // listener ever leaks between tests, `/` fires more than once per press
    // and this fails with a count — a far clearer signal than the unmount
    // test mysteriously inverting.
    await mountSidebar()
    const push = vi.spyOn(router, 'push')

    pressSlash()

    expect(push).toHaveBeenCalledTimes(1)
  })
})
