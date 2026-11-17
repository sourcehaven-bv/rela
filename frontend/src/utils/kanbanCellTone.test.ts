import { describe, expect, it } from 'vitest'

import { cellTone } from './kanbanCellTone'
import { styleFill, styleTokens } from './styleColors'

const labels = { success: 'Good', warning: 'Warning', error: 'Critical', info: 'Info', new: 'New' }

describe('styleTokens', () => {
  it('names a token pair for every class form', () => {
    expect(styleTokens('badge-orange')).toEqual({
      background: '--rl-tag-amber-bg',
      foreground: '--rl-tag-amber-fg',
    })
    expect(styleTokens('kit-red-700')).toEqual({
      background: '--rl-kit-red-700',
      foreground: '--rl-kit-red-700-fg',
    })
    expect(styleTokens('custom-risk')).toEqual({
      background: '--rl-custom-risk',
      foreground: '--rl-custom-risk-fg',
    })
  })

  it('refuses a class outside the grammar', () => {
    for (const cls of [undefined, '', 'kit-red-0', 'custom-a)b', 'constructor']) {
      expect(styleTokens(cls)).toBeUndefined()
    }
  })

  it('keeps styleFill on the same tokens', () => {
    expect(styleFill('badge-red')).toEqual({
      background: 'var(--rl-tag-red-bg)',
      color: 'var(--rl-tag-red-fg)',
    })
  })
})

describe('cellTone', () => {
  it('labels a tone with its translated name', () => {
    expect(cellTone({ color: 'error', class: 'badge-red' }, labels)).toEqual({
      background: '--rl-tag-red-bg',
      foreground: '--rl-tag-red-fg',
      label: 'Critical',
    })
  })

  it('prefers the configured label', () => {
    expect(
      cellTone({ color: 'warning', label: 'Medium', class: 'badge-orange' }, labels)?.label
    ).toBe('Medium')
  })

  it('draws nothing without a colour, a paintable class or a label', () => {
    expect(cellTone(undefined, labels)).toBeUndefined()
    expect(cellTone({ color: 'error' }, labels)).toBeUndefined()
    expect(cellTone({ color: 'error', class: 'nope' }, labels)).toBeUndefined()
    // A non-tone colour without a label is refused at load; here it is not
    // drawn rather than drawn as colour alone.
    expect(cellTone({ color: 'red/700', class: 'kit-red-700' }, labels)).toBeUndefined()
  })
})
