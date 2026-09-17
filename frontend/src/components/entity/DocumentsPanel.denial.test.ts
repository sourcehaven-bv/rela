import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import DocumentsPanel from './DocumentsPanel.vue'
import { ApiError } from '@/api/errors'
import { useSchemaStore } from '@/stores/schema'
import type { DocumentRenderResponse } from '@/types'

// Issue #1603 / CONTROL-8-03, the DocumentsPanel half.
//
// This component and DocumentView carry the same copy-pasted loader, and
// BUG-DJZTRF had to be fixed in both — fixing one leaves the bug live on the
// entity page. The shared `shouldDropHeldContent` classifier removes the
// chance of the two DISAGREEING about what counts as a denial, but the
// decision to act on it is still a separate line in each file, so it needs
// its own test here. Without one, a refactor deleting the branch from this
// component keeps CI green.
//
// Scope is deliberately narrow: the re-render mechanics (in-flight mounting,
// identical-HTML guard, generation fence) are pinned for DocumentView in
// DocumentView.rerender.test.ts and are not re-tested here.

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn().mockResolvedValue(undefined) }),
  useRoute: () => ({ query: {}, path: '/entity/ticket/TKT-1', fullPath: '/entity/ticket/TKT-1' }),
}))

vi.mock('@/utils/markdown', () => ({
  renderMermaidDiagrams: vi.fn().mockResolvedValue(undefined),
  renderPlantUMLDiagrams: vi.fn(),
  // Identity stand-in: these tests assert denial/hold behaviour by comparing
  // the HTML that reaches the DOM, so the table-scroll wrapper would only add
  // noise. Its own behaviour is covered in utils/markdown.test.ts.
  wrapTablesForScroll: (html: string) => html,
}))

const renderDocumentMock = vi.fn<() => Promise<DocumentRenderResponse>>()
vi.mock('@/api/documents', () => ({
  renderDocument: () => renderDocumentMock(),
}))

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

/** A server refusal as the axios interceptor delivers it to a catch site. */
function denial(status: number): ApiError {
  return new ApiError(`Request failed (${status})`, {
    kind: 'http',
    status,
    original: new Error('denied'),
  })
}

async function mountPanel(html: string) {
  const store = useSchemaStore()
  // entity_type must match the panel's prop, or availableDocuments is empty
  // and the component renders nothing at all.
  store.documents = new Map([['ticket-summary', { title: 'Summary', entity_type: 'ticket' }]])
  store.loaded = true

  renderDocumentMock.mockResolvedValue({ html, cached: false, entity_ids: [] })
  const wrapper = mount(DocumentsPanel, { props: { entityType: 'ticket', entityId: 'TKT-1' } })
  await flushPromises()
  return wrapper
}

describe('DocumentsPanel denial handling (#1603)', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    entityChangedHandler = undefined
  })

  it('renders the document once access is granted', async () => {
    const wrapper = await mountPanel('<p>original</p>')

    // Guards the harness itself: every assertion below is about content
    // DISAPPEARING, so a panel that never rendered would pass them all.
    expect(wrapper.find(BODY).exists()).toBe(true)
    expect(wrapper.html()).toContain('original')
  })

  it.each([
    ['401 expired session', 401],
    ['403 refused capability', 403],
    ['404 read gate (uniform not-found)', 404],
  ])('blanks the document when a re-render is denied (%s)', async (_name, status) => {
    const wrapper = await mountPanel('<p>original</p>')

    renderDocumentMock.mockRejectedValue(denial(status))
    entityChangedHandler?.()
    await flushPromises()

    expect(wrapper.html()).not.toContain('original')
    expect(wrapper.find(BODY).exists()).toBe(false)
    expect(wrapper.text()).toContain(EMPTY_STATE)
  })

  it('keeps the document on screen when a re-render fails transiently', async () => {
    const wrapper = await mountPanel('<p>original</p>')

    // BUG-DJZTRF's behaviour, which this fix must not undo. It sits here
    // rather than only in the DocumentView suite because a copy of the branch
    // that blanks on EVERY error would satisfy the denial tests above while
    // reintroducing the flash this component was fixed for.
    renderDocumentMock.mockRejectedValue(new Error('render failed'))
    entityChangedHandler?.()
    await flushPromises()

    expect(wrapper.html()).toContain('original')
    expect(wrapper.text()).not.toContain(EMPTY_STATE)
  })
})
