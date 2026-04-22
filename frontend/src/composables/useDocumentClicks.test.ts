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

  it('appends nearest-ancestor id to return_to as a #fragment', () => {
    const router = { push: vi.fn() }
    const handler = createDocumentClickHandler(router as never)
    // Link is inside <h2 id="biz"> — we expect the handler to rewrite
    // return_to from /entity to /entity#biz.
    const { evt } = clickInDOM(
      '<h2 id="biz"><a href="/form/foo?return_to=%2Fentity%2Fticket%2FX">edit</a></h2>',
      'a',
    )

    handler(evt)

    expect(router.push).toHaveBeenCalledWith('/form/foo?return_to=%2Fentity%2Fticket%2FX%23biz')
  })

  it('does not touch return_to when no element in the doc has an id', () => {
    const router = { push: vi.fn() }
    const handler = createDocumentClickHandler(router as never)
    const { evt } = clickInDOM(
      '<div><a href="/form/foo?return_to=%2Fentity%2Fticket%2FX">edit</a></div>',
      'a',
    )

    handler(evt)

    expect(router.push).toHaveBeenCalledWith('/form/foo?return_to=%2Fentity%2Fticket%2FX')
  })

  it('finds preceding heading id when link is in a following sibling', () => {
    // Table-row case: link is inside <tr>, not inside the <h2>. The
    // closest-ancestor walk misses the heading, but the preceding-id
    // walk catches it.
    const router = { push: vi.fn() }
    const handler = createDocumentClickHandler(router as never)
    const { evt } = clickInDOM(
      `<h2 id="biz">Businessfuncties</h2>
       <table><tr><td><a href="/form/bf/BF-001?return_to=%2Fentity%2Fapp%2FA">edit</a></td></tr></table>`,
      'a',
    )

    handler(evt)

    expect(router.push).toHaveBeenCalledWith(
      '/form/bf/BF-001?return_to=%2Fentity%2Fapp%2FA%23biz',
    )
  })

  it('ignores ids on elements outside the rendered document body', () => {
    // The app shell has id="app"; the handler must not treat that as
    // an anchor inside the document. With no in-doc id, return_to is
    // left unchanged.
    document.body.innerHTML = `
      <div id="app">
        <div class="document-body">
          <p><a href="/form/foo?return_to=%2Fentity%2Fx">edit</a></p>
        </div>
      </div>`
    const target = document.querySelector('a') as HTMLElement
    const evt = new MouseEvent('click', { bubbles: true, cancelable: true })
    Object.defineProperty(evt, 'target', { value: target, writable: false })

    const router = { push: vi.fn() }
    const handler = createDocumentClickHandler(router as never)
    handler(evt)

    expect(router.push).toHaveBeenCalledWith('/form/foo?return_to=%2Fentity%2Fx')
  })

  it('leaves return_to alone when it already has a hash', () => {
    const router = { push: vi.fn() }
    const handler = createDocumentClickHandler(router as never)
    // Author-supplied return_to already has #explicit-anchor; don't clobber.
    const { evt } = clickInDOM(
      '<h2 id="biz"><a href="/form/foo?return_to=%2Fx%23explicit">edit</a></h2>',
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
