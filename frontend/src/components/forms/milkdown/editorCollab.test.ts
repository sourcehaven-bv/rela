import { describe, expect, it } from 'vitest'
import { PEER_NAME_MAX, peerCursor } from './editorCollab'

describe('peerCursor', () => {
  it('shows the color and name a peer sent', () => {
    const caret = peerCursor({ name: 'alice', color: '#12abEF' })
    expect(caret.style.getPropertyValue('--collab-color')).toBe('#12abEF')
    expect(caret.textContent).toContain('alice')
  })

  it('ignores a color that is not #rrggbb and cuts a long name', () => {
    const caret = peerCursor({ name: 'x'.repeat(500), color: 'red; background: url(x)' })
    expect(caret.style.getPropertyValue('--collab-color')).toBe('')
    expect(caret.querySelector('div')!.textContent).toHaveLength(PEER_NAME_MAX)
  })

  it('ignores fields that are not strings', () => {
    const caret = peerCursor({ name: { a: 1 }, color: 7 })
    expect(caret.style.getPropertyValue('--collab-color')).toBe('')
    expect(caret.querySelector('div')!.textContent).toBe('')
  })
})
