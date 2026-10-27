import { describe, expect, it } from 'vitest'

import { OPTION_COLORS, tagColor } from './names'

describe('tagColor', () => {
  it('draws each picker colour as its own tag', () => {
    expect(OPTION_COLORS.map((c) => tagColor(c))).toEqual([
      'grey',
      'blue',
      'teal',
      'green',
      'yellow',
      'amber',
      'red',
      'purple',
    ])
  })

  it('draws a shade as its hue and anything else grey', () => {
    expect(tagColor('red/700')).toBe('red')
    expect(tagColor('teal/50')).toBe('teal')
    expect(tagColor('risk-critical')).toBe('grey')
    expect(tagColor('cyan')).toBe('grey')
    expect(tagColor(undefined)).toBe('grey')
  })
})
