import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { createRouter, createWebHistory, type Router } from 'vue-router'
import SearchView from './SearchView.vue'

/**
 * Regression coverage for the second half of BUG-DNP5E7.
 *
 * SearchView's `f` (open filter menu) shortcut carried the same weak
 * `tagName === 'INPUT' || tagName === 'TEXTAREA'` guard the Sidebar did, so it
 * fired while focus sat in a contenteditable surface. Lower impact than the
 * Sidebar's `/` — this handler only runs on the search page — but the same
 * defect, fixed the same way.
 */

// Stub the filter menu so we can observe `open()` without its own data loading.
const openSpy = vi.fn()
const FilterMenuStub = defineComponent({
  name: 'AdHocFilterMenu',
  setup(_, { expose }) {
    expose({ open: openSpy, close: vi.fn() })
    return () => null
  },
})

describe('SearchView f filter shortcut', () => {
  let router: Router
  let wrapper: VueWrapper | null = null

  beforeEach(async () => {
    openSpy.mockClear()
    router = createRouter({
      history: createWebHistory(),
      routes: [
        { path: '/', name: 'home', component: { template: '<div/>' } },
        { path: '/search', name: 'search', component: { template: '<div/>' } },
      ],
    })
    await router.push('/search')
    await router.isReady()
  })

  // Unmount here, not at the end of each test body: a failed `expect` throws,
  // so a trailing unmount is unreachable exactly when a test fails, and the
  // leaked document-level listener would corrupt every later test in the file.
  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
    document.body.innerHTML = ''
    vi.restoreAllMocks()
  })

  async function mountSearch() {
    wrapper = mount(SearchView, {
      global: {
        plugins: [router],
        stubs: { AdHocFilterMenu: FilterMenuStub },
      },
    })
    await router.isReady()
    return wrapper
  }

  function pressF() {
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'f', bubbles: true }))
  }

  it('opens the filter menu when focus is not in a text surface', async () => {
    await mountSearch()

    pressF()

    // Exactly once, which also pins the teardown contract: a listener leaked
    // from an earlier test would show up here as a count > 1.
    expect(openSpy).toHaveBeenCalledTimes(1)
  })

  it('does not open the filter menu while a contenteditable host has focus', async () => {
    await mountSearch()

    const editor = document.createElement('div')
    editor.setAttribute('contenteditable', 'true')
    editor.className = 'ProseMirror'
    document.body.appendChild(editor)
    editor.focus()

    pressF()

    expect(openSpy).not.toHaveBeenCalled()
  })

  it('does not open the filter menu while an input has focus', async () => {
    await mountSearch()

    const input = document.createElement('input')
    document.body.appendChild(input)
    input.focus()

    pressF()

    expect(openSpy).not.toHaveBeenCalled()
  })
})
