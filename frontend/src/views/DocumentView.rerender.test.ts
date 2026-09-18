import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import DocumentView from './DocumentView.vue'
import { ApiError } from '@/api/errors'
import { useSchemaStore } from '@/stores/schema'
import { renderMermaidDiagrams } from '@/utils/markdown'
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

// Diagram rendering touches layout. Mocked both to keep it out of the way and
// so a test can assert it did NOT run on an unchanged re-render.
vi.mock('@/utils/markdown', () => ({
  renderMermaidDiagrams: vi.fn().mockResolvedValue(undefined),
  renderPlantUMLDiagrams: vi.fn(),
  // Identity stand-in: these tests assert re-render/mount behaviour by
  // comparing the HTML that reaches the DOM, so the table-scroll wrapper would
  // only add noise. Its own behaviour is covered in utils/markdown.test.ts.
  wrapTablesForScroll: (html: string) => html,
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

/** A server refusal as the axios interceptor delivers it to a catch site. */
function denial(status: number): ApiError {
  return new ApiError(`Request failed (${status})`, {
    kind: 'http',
    status,
    original: new Error('denied'),
  })
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

/** Switches the mounted view to a different document (a cold load). */
async function switchDocument(wrapper: Awaited<ReturnType<typeof mountView>>, name: string) {
  await wrapper.setProps({ name } as InstanceType<typeof DocumentView>['$props'])
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

    // Probe a CHILD node, not the container. v-html replaces the container's
    // *contents*, so the container element itself survives either way —
    // asserting on it passes even with the equality guard removed, which is
    // the false-confidence trap this test exists to avoid. The child node is
    // what actually gets destroyed and rebuilt, and rebuilding it is what
    // resets scroll and re-runs the diagram watcher.
    const before = wrapper.find(BODY).element.firstChild
    expect(before).not.toBeNull()

    renderDocumentMock.mockResolvedValue(response('<p>original</p>'))
    entityChangedHandler?.()
    await flushPromises()

    expect(wrapper.find(BODY).element.firstChild).toBe(before)
  })

  it('does not re-run diagram rendering when the HTML is unchanged', async () => {
    const wrapper = await mountView('<p>original</p>')
    vi.mocked(renderMermaidDiagrams).mockClear()

    renderDocumentMock.mockResolvedValue(response('<p>original</p>'))
    entityChangedHandler?.()
    await flushPromises()

    // The sanitizedContent watcher re-runs mermaid/PlantUML on every repaint.
    // An unchanged re-render must not trigger it — diagram re-layout is a
    // second, independent source of scroll movement.
    expect(renderMermaidDiagrams).not.toHaveBeenCalled()
    expect(wrapper.html()).toContain('original')
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

  // Issue #1603 / CONTROL-8-03. The clause above keeps content across a failed
  // re-render, which is right for a transient one and wrong for a denial: an
  // access revocation mid-view left the previous render on screen until the
  // user navigated away by hand. These assert the narrow carve-out — the
  // preceding test pins that the general case is unchanged.
  it.each([
    ['401 expired session', 401],
    ['403 refused capability', 403],
    ['404 read gate (uniform not-found)', 404],
  ])('blanks the document when a re-render is denied (%s)', async (_name, status) => {
    const wrapper = await mountView('<p>original</p>')

    renderDocumentMock.mockRejectedValue(denial(status))
    entityChangedHandler?.()
    await flushPromises()

    expect(wrapper.html()).not.toContain('original')
    expect(wrapper.find(BODY).exists()).toBe(false)
    expect(wrapper.text()).toContain(EMPTY_STATE)
  })

  it('does not blank on a denial from a superseded render', async () => {
    const wrapper = await mountView('<p>original</p>')

    const stale = deferred<DocumentRenderResponse>()
    renderDocumentMock.mockReturnValue(stale.promise)
    entityChangedHandler?.()
    await flushPromises()

    const fresh = deferred<DocumentRenderResponse>()
    renderDocumentMock.mockReturnValue(fresh.promise)
    await switchDocument(wrapper, 'other')
    fresh.resolve(response('<p>other doc</p>'))
    await flushPromises()

    // The abandoned render's denial is about a document no longer displayed.
    // Blanking here would erase a body the newest render was allowed to paint.
    stale.reject(denial(404))
    await flushPromises()

    expect(wrapper.html()).toContain('other doc')
  })

  it('ignores a superseded render that resolves after a document switch', async () => {
    const wrapper = await mountView('<p>original</p>')

    // A warm SSE re-render of the CURRENT document, held open.
    const stale = deferred<DocumentRenderResponse>()
    renderDocumentMock.mockReturnValue(stale.promise)
    entityChangedHandler?.()
    await flushPromises()

    // The user switches documents; that render resolves first.
    const fresh = deferred<DocumentRenderResponse>()
    renderDocumentMock.mockReturnValue(fresh.promise)
    await switchDocument(wrapper, 'other')
    fresh.resolve(response('<p>other doc</p>'))
    await flushPromises()
    expect(wrapper.html()).toContain('other doc')

    // Now the superseded render lands. It must not paint — doing so would
    // show the previous document's body under the new document's title.
    stale.resolve(response('<p>original</p>'))
    await flushPromises()

    expect(wrapper.html()).toContain('other doc')
    expect(wrapper.html()).not.toContain('original')
  })

  it('keeps loading true when a superseded render settles first', async () => {
    const wrapper = await mountView('<p>original</p>')

    const stale = deferred<DocumentRenderResponse>()
    renderDocumentMock.mockReturnValue(stale.promise)
    entityChangedHandler?.()
    await flushPromises()

    const fresh = deferred<DocumentRenderResponse>()
    renderDocumentMock.mockReturnValue(fresh.promise)
    await switchDocument(wrapper, 'other')

    // The superseded render settles while the newest is still in flight. It
    // must not clear `loading`, or Refresh re-enables mid-render.
    stale.resolve(response('<p>original</p>'))
    await flushPromises()

    // PendingButton keeps both labels in the DOM and marks the button
    // aria-disabled while pending (native `disabled` would drop focus).
    const refresh = wrapper.find('.header-right .btn-secondary')
    expect(refresh.attributes('aria-disabled')).toBe('true')

    fresh.resolve(response('<p>other doc</p>'))
    await flushPromises()
  })

  it('does not surface an error from a superseded render', async () => {
    const wrapper = await mountView('<p>original</p>')

    const stale = deferred<DocumentRenderResponse>()
    renderDocumentMock.mockReturnValue(stale.promise)
    entityChangedHandler?.()
    await flushPromises()

    const fresh = deferred<DocumentRenderResponse>()
    renderDocumentMock.mockReturnValue(fresh.promise)
    await switchDocument(wrapper, 'other')
    fresh.resolve(response('<p>other doc</p>'))
    await flushPromises()

    // The abandoned render fails. The user is waiting on a different render
    // that succeeded, so a toast here would be about work they replaced.
    stale.reject(new Error('render failed'))
    await flushPromises()

    expect(wrapper.html()).toContain('other doc')
  })

  it('blanks the view when switching to a different document', async () => {
    const wrapper = await mountView('<p>original</p>')

    // A cold load MUST blank: keeping the old body would render one
    // document's content under another's title until the fetch lands.
    const pending = deferred<DocumentRenderResponse>()
    renderDocumentMock.mockReturnValue(pending.promise)

    await switchDocument(wrapper, 'other')
    await flushPromises()

    expect(wrapper.html()).not.toContain('original')

    pending.resolve(response('<p>other doc</p>'))
    await flushPromises()

    expect(wrapper.html()).toContain('other doc')
  })
})
