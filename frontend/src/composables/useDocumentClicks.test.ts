import { describe, it, expect, vi } from 'vitest'
import { createDocumentClickHandler } from './useDocumentClicks'

// Build a little DOM tree rooted at `containerHtml`, return both the
// target element and a click event fired from `clickSelector`. The
// tree is wrapped in a `.document-body` container so the handler's
// chrome-filter recognises it as rendered-doc content.
function clickInDOM(containerHtml: string, clickSelector: string) {
  document.body.innerHTML = `<div class="document-body">${containerHtml}</div>`
  const target = document.querySelector(clickSelector) as HTMLElement
  if (!target) throw new Error(`no element matched ${clickSelector}`)
  const evt = new MouseEvent('click', { bubbles: true, cancelable: true })
  Object.defineProperty(evt, 'target', { value: target, writable: false })
  return { evt, target }
}

describe('createDocumentClickHandler', () => {
  it('intercepts internal links and routes through vue-router', () => {
    const router = { push: vi.fn() }
    const handler = createDocumentClickHandler(router as never)
    const { evt } = clickInDOM(
      '<div><a href="/list/all">click me</a></div>',
      'a',
    )

    handler(evt)

    expect(evt.defaultPrevented).toBe(true)
    expect(router.push).toHaveBeenCalledWith('/list/all')
  })

  it('leaves external links alone', () => {
    const router = { push: vi.fn() }
    const handler = createDocumentClickHandler(router as never)
    const { evt } = clickInDOM(
      '<a href="https://example.com">out</a>',
      'a',
    )

    handler(evt)

    expect(evt.defaultPrevented).toBe(false)
    expect(router.push).not.toHaveBeenCalled()
  })

  it('appends the anchor\'s id to return_to as a #fragment', () => {
    // The server emits id="edit-<entity>-<n>" on every form link so the
    // scroll-back anchor is stable across title edits.
    const router = { push: vi.fn() }
    const handler = createDocumentClickHandler(router as never)
    const { evt } = clickInDOM(
      '<h2 id="biz"><a id="edit-prs-bf-7hn6-0" href="/form/bf/PRS-BF-7HN6?return_to=%2Fdoc">edit</a></h2>',
      'a',
    )

    handler(evt)

    expect(router.push).toHaveBeenCalledWith(
      '/form/bf/PRS-BF-7HN6?return_to=%2Fdoc%23edit-prs-bf-7hn6-0',
    )
  })

  it('leaves return_to alone when the anchor has no id', () => {
    // Non-form links (e.g. /entity/..., /list/...) never have return_to
    // injected by the server in the first place, so this path rarely
    // fires in production — but if a return_to is present and the
    // clicked anchor has no id, we pass through without a fragment.
    const router = { push: vi.fn() }
    const handler = createDocumentClickHandler(router as never)
    const { evt } = clickInDOM(
      '<div><a href="/form/foo?return_to=%2Fentity%2Fticket%2FX">edit</a></div>',
      'a',
    )

    handler(evt)

    expect(router.push).toHaveBeenCalledWith('/form/foo?return_to=%2Fentity%2Fticket%2FX')
  })

  it('leaves return_to alone when it already has a hash', () => {
    const router = { push: vi.fn() }
    const handler = createDocumentClickHandler(router as never)
    // Author-supplied return_to already has #explicit-anchor; don't clobber.
    const { evt } = clickInDOM(
      '<a id="edit-x-0" href="/form/foo?return_to=%2Fx%23explicit">edit</a>',
      'a',
    )

    handler(evt)

    expect(router.push).toHaveBeenCalledWith('/form/foo?return_to=%2Fx%23explicit')
  })

  it.each([
    ['meta', { metaKey: true }],
    ['ctrl', { ctrlKey: true }],
    ['shift', { shiftKey: true }],
    ['alt', { altKey: true }],
    ['middle-click', { button: 1 }],
    ['right-click', { button: 2 }],
  ])('leaves %s clicks to the browser', (_name, init) => {
    const router = { push: vi.fn() }
    const handler = createDocumentClickHandler(router as never)
    document.body.innerHTML = '<div class="document-body"><a href="/list/x">x</a></div>'
    const target = document.querySelector('a') as HTMLElement
    const evt = new MouseEvent('click', {
      bubbles: true,
      cancelable: true,
      button: 0,
      ...init,
    })
    Object.defineProperty(evt, 'target', { value: target, writable: false })

    handler(evt)

    expect(evt.defaultPrevented).toBe(false)
    expect(router.push).not.toHaveBeenCalled()
  })

  it('leaves target="_blank" links to the browser', () => {
    const router = { push: vi.fn() }
    const handler = createDocumentClickHandler(router as never)
    const { evt } = clickInDOM(
      '<a href="/list/x" target="_blank">x</a>',
      'a',
    )

    handler(evt)

    expect(evt.defaultPrevented).toBe(false)
    expect(router.push).not.toHaveBeenCalled()
  })

  it('leaves download links to the browser', () => {
    const router = { push: vi.fn() }
    const handler = createDocumentClickHandler(router as never)
    const { evt } = clickInDOM(
      '<a href="/export.csv" download>dl</a>',
      'a',
    )

    handler(evt)

    expect(evt.defaultPrevented).toBe(false)
    expect(router.push).not.toHaveBeenCalled()
  })

  it('handles links without return_to by routing verbatim', () => {
    const router = { push: vi.fn() }
    const handler = createDocumentClickHandler(router as never)
    const { evt } = clickInDOM(
      '<h2 id="biz"><a href="/list/tasks">list</a></h2>',
      'a',
    )

    handler(evt)

    expect(router.push).toHaveBeenCalledWith('/list/tasks')
  })
})
