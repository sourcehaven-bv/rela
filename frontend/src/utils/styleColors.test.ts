import { describe, expect, it } from 'vitest'

import { styleStatusColor, styleSwatch } from './styleColors'

describe('styleSwatch', () => {
  it('gives orange and yellow different dots', () => {
    expect(styleSwatch('badge-orange')?.status).toBe('amber')
    expect(styleSwatch('badge-yellow')?.status).toBe('--rl-color-status-yellow')
  })

  it('paints a kit shade and an operator colour with palette tokens', () => {
    expect(styleSwatch('kit-teal-50')).toEqual({
      badgeStyle: { background: 'var(--rl-kit-teal-50)', color: 'var(--rl-kit-teal-50-fg)' },
      status: '--rl-kit-teal-50',
    })
    expect(styleSwatch('custom-ok')?.status).toBe('--rl-custom-ok')
  })

  it('refuses a class outside the grammar', () => {
    for (const cls of ['kit-red-0', 'kit-purple-700', 'custom-', 'custom-a)b', 'x', '', 'constructor', 'toString']) {
      expect(styleSwatch(cls)).toBeUndefined()
    }
  })

  it('maps a value through the style map', () => {
    expect(styleStatusColor({ hoog: 'kit-red-900' }, 'Hoog')).toBe('--rl-kit-red-900')
  })
})
