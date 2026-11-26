import { describe, expect, it, vi } from 'vitest'

import type { EntityType } from '@/types'
import { defaultRegistry } from '@/widgets/registry'
import { createDenseCellResolver } from './denseCell'
import { formatAmount } from './format'

const deal = {
  label: 'Deal',
  properties: {
    stage: { type: 'enum', values: ['lead', 'won'] },
    value: { type: 'integer', display: { currency: 'EUR' } },
    note: { type: 'string' },
  },
} as unknown as EntityType

describe('createDenseCellResolver', () => {
  it('hands an enum widget the raw value and a passthrough widget the formatted one', () => {
    const resolve = createDenseCellResolver()
    expect(resolve('won', 'stage', deal, 'UTC')?.modelValue).toBe('won')
    expect(resolve(3500, 'value', deal, 'UTC')?.modelValue).toBe(formatAmount(3500, 'EUR'))
  })

  it('resolves the widget once per type and property', () => {
    const spy = vi.spyOn(defaultRegistry, 'resolveDense')
    const resolve = createDenseCellResolver()
    resolve('lead', 'stage', deal, 'UTC')
    resolve('won', 'stage', deal, 'UTC')
    resolve('x', 'note', deal, 'UTC')
    expect(spy).toHaveBeenCalledTimes(2)
    spy.mockRestore()
  })

  it('leaves empty values, unknown properties and unknown types to plain text', () => {
    const resolve = createDenseCellResolver()
    expect(resolve('', 'stage', deal, 'UTC')).toBeUndefined()
    expect(resolve([], 'stage', deal, 'UTC')).toBeUndefined()
    expect(resolve('x', 'missing', deal, 'UTC')).toBeUndefined()
    expect(resolve('x', 'stage', undefined, 'UTC')).toBeUndefined()
  })
})
