import { describe, it, expect } from 'vitest'
import { OPERATOR_MAP, toApiOperator, buildFilterKey } from './filters'

describe('filters', () => {
  describe('OPERATOR_MAP', () => {
    it('maps != to ne', () => {
      expect(OPERATOR_MAP['!=']).toBe('ne')
    })

    it('maps = to eq', () => {
      expect(OPERATOR_MAP['=']).toBe('eq')
    })

    it('maps == to eq', () => {
      expect(OPERATOR_MAP['==']).toBe('eq')
    })

    it('maps > to gt', () => {
      expect(OPERATOR_MAP['>']).toBe('gt')
    })

    it('maps >= to gte', () => {
      expect(OPERATOR_MAP['>=']).toBe('gte')
    })

    it('maps < to lt', () => {
      expect(OPERATOR_MAP['<']).toBe('lt')
    })

    it('maps <= to lte', () => {
      expect(OPERATOR_MAP['<=']).toBe('lte')
    })

    it('maps ~ to contains', () => {
      expect(OPERATOR_MAP['~']).toBe('contains')
    })
  })

  describe('toApiOperator', () => {
    it('converts known operators', () => {
      expect(toApiOperator('!=')).toBe('ne')
      expect(toApiOperator('=')).toBe('eq')
      expect(toApiOperator('==')).toBe('eq')
      expect(toApiOperator('>')).toBe('gt')
      expect(toApiOperator('>=')).toBe('gte')
      expect(toApiOperator('<')).toBe('lt')
      expect(toApiOperator('<=')).toBe('lte')
      expect(toApiOperator('~')).toBe('contains')
    })

    it('defaults to eq for undefined', () => {
      expect(toApiOperator(undefined)).toBe('eq')
    })

    it('defaults to eq for unknown operators', () => {
      expect(toApiOperator('unknown')).toBe('eq')
      expect(toApiOperator('??')).toBe('eq')
    })
  })

  describe('buildFilterKey', () => {
    it('builds filter key with explicit operator', () => {
      expect(buildFilterKey('status', '=')).toBe('filter[status][eq]')
      expect(buildFilterKey('priority', '!=')).toBe('filter[priority][ne]')
      expect(buildFilterKey('count', '>')).toBe('filter[count][gt]')
      expect(buildFilterKey('name', '~')).toBe('filter[name][contains]')
    })

    it('builds filter key with undefined operator (defaults to eq)', () => {
      expect(buildFilterKey('status', undefined)).toBe('filter[status][eq]')
    })

    it('handles various property names', () => {
      expect(buildFilterKey('my_property', '=')).toBe('filter[my_property][eq]')
      expect(buildFilterKey('camelCase', '>=')).toBe('filter[camelCase][gte]')
    })
  })
})
