import { describe, it, expect } from 'vitest'
import { shouldDeferToBrowser, safeInternalHref } from './openIntent'

function click(init: Partial<MouseEventInit> & { defaultPrevented?: boolean } = {}) {
  const { defaultPrevented, ...rest } = init
  const evt = new MouseEvent('click', { cancelable: true, ...rest })
  if (defaultPrevented) evt.preventDefault()
  return evt
}

describe('shouldDeferToBrowser', () => {
  it('is false for a plain primary click — that is ours to route', () => {
    expect(shouldDeferToBrowser(click())).toBe(false)
  })

  it.each([
    ['metaKey', { metaKey: true }],
    ['ctrlKey', { ctrlKey: true }],
    ['shiftKey', { shiftKey: true }],
    // Option-click downloads the target on macOS. We stay consistent with
    // useDocumentClicks, which also defers on altKey.
    ['altKey', { altKey: true }],
  ])('is true for %s so the browser opens a tab/window itself', (_name, init) => {
    expect(shouldDeferToBrowser(click(init))).toBe(true)
  })

  it('is true for middle-click', () => {
    expect(shouldDeferToBrowser(click({ button: 1 }))).toBe(true)
  })

  it('is true for right-click', () => {
    expect(shouldDeferToBrowser(click({ button: 2 }))).toBe(true)
  })

  it('is true when a nested control already handled the event', () => {
    // Not a new-tab intent — the point is that nothing further should happen.
    expect(shouldDeferToBrowser(click({ defaultPrevented: true }))).toBe(true)
  })
})

describe('safeInternalHref', () => {
  it.each(['/entity/ticket/TKT-1', '/list/all?from=x', '/'])(
    'accepts the same-origin path %s',
    (path) => {
      expect(safeInternalHref(path)).toBe(true)
    },
  )

  it('rejects protocol-relative paths, which navigate off-origin', () => {
    expect(safeInternalHref('//evil.com')).toBe(false)
  })

  it.each(['javascript:alert(1)', 'JavaScript:alert(1)', 'data:text/html,x', 'https://evil.com', '', 'entity/x'])(
    'rejects %s',
    (path) => {
      expect(safeInternalHref(path)).toBe(false)
    },
  )
})
