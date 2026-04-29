import { describe, it, expect } from 'vitest'
import {
  formatValue,
  formatCellValue,
  getCellValue,
  isEnumProperty,
  isEnumPropertyDef,
  asArray,
} from './format'
import type { EntityType } from '@/types'

describe('format', () => {
  describe('formatValue', () => {
    it('returns dash for null', () => {
      expect(formatValue(null)).toBe('-')
    })

    it('returns dash for undefined', () => {
      expect(formatValue(undefined)).toBe('-')
    })

    it('formats date type correctly', () => {
      const result = formatValue('2024-01-15', 'date')
      expect(result).toMatch(/\d+/)
    })

    it('returns dash for invalid date', () => {
      expect(formatValue('not-a-date', 'date')).toBe('-')
    })

    it('formats boolean true as Yes', () => {
      expect(formatValue(true, 'boolean')).toBe('Yes')
    })

    it('formats boolean false as No', () => {
      expect(formatValue(false, 'boolean')).toBe('No')
    })

    it('joins arrays with comma', () => {
      expect(formatValue(['a', 'b', 'c'])).toBe('a, b, c')
    })

    it('returns dash for empty array', () => {
      expect(formatValue([])).toBe('-')
    })

    it('converts numbers to string', () => {
      expect(formatValue(42)).toBe('42')
    })

    it('returns string values as-is', () => {
      expect(formatValue('hello')).toBe('hello')
    })
  })

  describe('formatCellValue', () => {
    const mockEntityType: EntityType = {
      label: 'Test',
      description: '',
      properties: {
        created_at: { type: 'date' },
        is_active: { type: 'boolean' },
        title: { type: 'string' },
        schedule: { type: 'rrule' },
      },
    }

    it('returns empty string for null', () => {
      expect(formatCellValue(null, 'title', mockEntityType)).toBe('')
    })

    it('returns empty string for undefined', () => {
      expect(formatCellValue(undefined, 'title', mockEntityType)).toBe('')
    })

    it('joins arrays with comma', () => {
      expect(formatCellValue(['x', 'y'], 'tags', mockEntityType)).toBe('x, y')
    })

    it('formats date property correctly', () => {
      const result = formatCellValue('2024-01-15', 'created_at', mockEntityType)
      expect(result).toMatch(/\d+/)
    })

    it('returns dash for invalid date property', () => {
      expect(formatCellValue('invalid', 'created_at', mockEntityType)).toBe('-')
    })

    it('formats boolean property as Yes/No', () => {
      expect(formatCellValue(true, 'is_active', mockEntityType)).toBe('Yes')
      expect(formatCellValue(false, 'is_active', mockEntityType)).toBe('No')
    })

    it('converts values to string without entity type', () => {
      expect(formatCellValue(123, 'count', undefined)).toBe('123')
    })

    it('converts values to string without property', () => {
      expect(formatCellValue('text', undefined, mockEntityType)).toBe('text')
    })

    describe('rrule property', () => {
      const cases: Array<[string, string]> = [
        ['bare', 'FREQ=DAILY'],
        ['with RRULE: prefix', 'RRULE:FREQ=DAILY'],
        ['with DTSTART and RRULE', 'DTSTART:20240101T000000Z\nRRULE:FREQ=DAILY'],
        ['malformed', 'NOT_A_RULE'],
      ]

      it.each(cases)('matches formatValue parity: %s', (_label, input) => {
        expect(formatCellValue(input, 'schedule', mockEntityType)).toBe(
          formatValue(input, 'rrule'),
        )
      })

      it('formats rrule wrapped in a single-element array', () => {
        const input = 'FREQ=DAILY'
        expect(formatCellValue([input], 'schedule', mockEntityType)).toBe(
          formatValue(input, 'rrule'),
        )
      })

      it('returns empty string for null', () => {
        expect(formatCellValue(null, 'schedule', mockEntityType)).toBe('')
      })

      it('returns empty string for empty string', () => {
        expect(formatCellValue('', 'schedule', mockEntityType)).toBe('')
      })
    })
  })

  describe('getCellValue', () => {
    const entity = {
      id: 'ENT-001',
      properties: {
        title: 'Test Entity',
        status: 'open',
      },
      relations: {
        assigned_to: ['USER-001', 'USER-002'],
        parent: ['ENT-000'],
      },
    }

    it('returns entity id for id property', () => {
      expect(getCellValue(entity, { property: 'id' })).toBe('ENT-001')
    })

    it('returns property value', () => {
      expect(getCellValue(entity, { property: 'title' })).toBe('Test Entity')
      expect(getCellValue(entity, { property: 'status' })).toBe('open')
    })

    it('returns undefined for missing property', () => {
      expect(getCellValue(entity, { property: 'nonexistent' })).toBeUndefined()
    })

    it('returns relation values as array', () => {
      expect(getCellValue(entity, { relation: 'assigned_to' })).toEqual(['USER-001', 'USER-002'])
    })

    it('returns empty array for missing relation', () => {
      expect(getCellValue(entity, { relation: 'nonexistent' })).toEqual([])
    })

    it('returns empty string for empty column config', () => {
      expect(getCellValue(entity, {})).toBe('')
    })

    it('handles entity without relations', () => {
      const entityNoRelations = { id: 'ENT-002', properties: {} }
      expect(getCellValue(entityNoRelations, { relation: 'parent' })).toBe('')
    })
  })

  describe('isEnumProperty', () => {
    it('returns true for enum type', () => {
      expect(isEnumProperty({ type: 'enum' })).toBe(true)
    })

    it('returns true for property with values', () => {
      expect(isEnumProperty({ values: ['a', 'b', 'c'] })).toBe(true)
    })

    it('returns true for enum type with values', () => {
      expect(isEnumProperty({ type: 'enum', values: ['x', 'y'] })).toBe(true)
    })

    it('returns false for non-enum type without values', () => {
      expect(isEnumProperty({ type: 'string' })).toBe(false)
    })

    it('returns false for empty values array', () => {
      expect(isEnumProperty({ values: [] })).toBe(false)
    })

    it('returns false for empty object', () => {
      expect(isEnumProperty({})).toBe(false)
    })
  })

  describe('isEnumPropertyDef', () => {
    it('returns false for undefined', () => {
      expect(isEnumPropertyDef(undefined)).toBe(false)
    })

    it('returns true for enum type', () => {
      expect(isEnumPropertyDef({ type: 'enum' })).toBe(true)
    })

    it('returns true for property with values', () => {
      expect(isEnumPropertyDef({ type: 'string', values: ['a', 'b'] })).toBe(true)
    })

    it('returns false for non-enum without values', () => {
      expect(isEnumPropertyDef({ type: 'string' })).toBe(false)
    })
  })

  describe('asArray', () => {
    it('returns [] for null', () => {
      expect(asArray(null)).toEqual([])
    })

    it('returns [] for undefined', () => {
      expect(asArray(undefined)).toEqual([])
    })

    it('returns [] for empty string', () => {
      expect(asArray('')).toEqual([])
    })

    it('wraps scalar string in array', () => {
      expect(asArray('bug')).toEqual(['bug'])
    })

    it('coerces number to string', () => {
      expect(asArray(42)).toEqual(['42'])
    })

    it('returns array as-is', () => {
      expect(asArray(['bug', 'ui'])).toEqual(['bug', 'ui'])
    })

    it('filters empty strings from array', () => {
      expect(asArray(['bug', '', 'ui'])).toEqual(['bug', 'ui'])
    })

    it('coerces mixed array items to string', () => {
      expect(asArray(['bug', 42, true])).toEqual(['bug', '42', 'true'])
    })

    it('returns empty array for empty input array', () => {
      expect(asArray([])).toEqual([])
    })
  })
})
