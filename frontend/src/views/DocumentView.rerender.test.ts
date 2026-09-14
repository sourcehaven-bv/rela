import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import DocumentView from './DocumentView.vue'
import { useSchemaStore } from '@/stores/schema'
import type { DocumentRenderResponse } from '@/types'

// BUG-DJZTRF. The document view re-renders on every `entity:changed` SSE
// frame, and that feed is type-scoped (TKT-POT9GQ) — any entity write of any
// type reaches every connected client. The re-render therefore fires
// constantly while the viewed document has not changed, so it must be
// invisible: no unmount of the rendered body, no scroll reset.
//
// These assertions deliberately observe the render DURING the in-flight
// fetch and the DOM node's identity across it. The settled state was
// identical before and after the fix — `docContent` ends up holding the same
// HTML either way — which is exactly why the original defect survived the
// suite. Anything asserted after a bare `flushPromises` passes against the
// buggy code. See AM-document-rerender-preserves-content.

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ query: {}, path: '/document/report', fullPath: '/document/report' }),
}))

// Diagram rendering touches layout and is irrelevant to these assertions.
vi.mock('@/utils/markdown', () => ({
  renderMermaidDiagrams: vi.fn().mockResolvedValue(undefined),
  renderPlantUMLDiagrams: vi.fn(),
}))

const renderDocumentMock = vi.fn<() => Promise<DocumentRenderResponse>>()
vi.mock('@/api/documents', () => ({
  renderDocument: () => renderDocumentMock(),
}))

// Captures the handler DocumentView registers for `entity:changed` so a test
// can fire the SSE event directly, without standing up an EventSource.
let entityChangedHandler: (() => void) | undefined
vi.mock('@/composables/useEvents', () => ({
  useEvents: () => ({
    on: (event: string, handler: () => void) => {
      if (event === 'entity:changed') entityChangedHandler = handler
    },
    off: () => {},
  }),
}))

const BODY = '.document-body'
const EMPTY_STATE = 'No document content available'

function response(html: string): DocumentRenderResponse {
  return { html, cached: false, entity_ids: [] }
}

/** A promise plus the trigger that settles it, so a test can hold a render open. */
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

async function mountView(html: string) {
  const store = useSchemaStore()
  store.documents = new Map([['report', { title: 'Report' }]])
  store.loaded = true

  renderDocumentMock.mockResolvedValue(response(html))
  const wrapper = mount(DocumentView, { props: { name: 'report', entityId: 'TKT-1' } })
  await flushPromises()
  return wrapper
}

describe('DocumentView re-render (BUG-DJZTRF)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    entityChangedHandler = undefined
  })

  it('keeps the rendered document mounted while an SSE re-render is in flight', async () => {
    const wrapper = await mountView('<p>original</p>')
    expect(wrapper.find(BODY).exists()).toBe(true)

    // Hold the re-render open so the in-flight state can be observed. This is
    // the window the bug lived in: docContent was blanked before the await.
    const pending = deferred<DocumentRenderResponse>()
    renderDocumentMock.mockReturnValue(pending.promise)

    entityChangedHandler?.()
    await flushPromises()

    expect(wrapper.find(BODY).exists()).toBe(true)
    expect(wrapper.text()).not.toContain(EMPTY_STATE)
    expect(wrapper.html()).toContain('original')

    pending.resolve(response('<p>original</p>'))
    await flushPromises()

    expect(wrapper.find(BODY).exists()).toBe(true)
  })

  it('does not touch the DOM when the re-render returns identical HTML', async () => {
    const wrapper = await mountView('<p>original</p>')

    // Node identity is the assertion that matters: v-html replaces the whole
    // subtree on any assignment, even an identical one, and that replacement
    // is what resets scroll position.
    const before = wrapper.find(BODY).element

    renderDocumentMock.mockResolvedValue(response('<p>original</p>'))
    entityChangedHandler?.()
    await flushPromises()

    expect(wrapper.find(BODY).element).toBe(before)
  })

  it('swaps content in place when the re-render returns changed HTML', async () => {
    const wrapper = await mountView('<p>original</p>')

    renderDocumentMock.mockResolvedValue(response('<p>updated</p>'))
    entityChangedHandler?.()
    await flushPromises()

    // A real change must still land — the guard is an equality check, not a
    // freeze. The body stays mounted throughout, so scroll is preserved.
    expect(wrapper.html()).toContain('updated')
    expect(wrapper.find(BODY).exists()).toBe(true)
  })

  it('keeps the current document on screen when a re-render fails', async () => {
    const wrapper = await mountView('<p>original</p>')

    renderDocumentMock.mockRejectedValue(new Error('render failed'))
    entityChangedHandler?.()
    await flushPromises()

    // The error surfaces as a toast; blanking the view on a transient failure
    // would lose the user's place and show the empty state instead.
    expect(wrapper.html()).toContain('original')
    expect(wrapper.text()).not.toContain(EMPTY_STATE)
  })

  it('blanks the view when switching to a different document', async () => {
    const wrapper = await mountView('<p>original</p>')

    // A cold load MUST blank: keeping the old body would render one
    // document's content under another's title until the fetch lands.
    const pending = deferred<DocumentRenderResponse>()
    renderDocumentMock.mockReturnValue(pending.promise)

    await wrapper.setProps({ name: 'other' })
    await flushPromises()

    expect(wrapper.html()).not.toContain('original')

    pending.resolve(response('<p>other doc</p>'))
    await flushPromises()

    expect(wrapper.html()).toContain('other doc')
  })
})
