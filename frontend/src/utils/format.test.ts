import { describe, it, expect } from 'vitest'
import {
  formatValue,
  formatCellValue,
  formatDate,
  formatDatetime,
  utcISOToLocalInput,
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

    it('formats date type with short month name and exact day', () => {
      const result = formatValue('2024-01-15', 'date')
      expect(result).toMatch(/15/)
      expect(result).toMatch(/2024/)
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

    it('formats a datetime in the given display zone', () => {
      // 12:30 UTC in New York (UTC-4 in July) is 08:30.
      expect(formatValue('2026-07-13T12:30:00Z', 'datetime', 'America/New_York')).toContain('8:30')
    })

    it('defaults the datetime zone to the browser when tz omitted', () => {
      // No throw, returns a non-dash formatted string.
      const out = formatValue('2026-07-13T12:30:00Z', 'datetime')
      expect(out).not.toBe('-')
      expect(out).toMatch(/2026/)
    })
  })

  describe('formatDatetime', () => {
    it('renders a zoned instant identically regardless of the interpretation zone arg for absolute values', () => {
      // A value with an explicit Z is absolute; the tz arg only affects display.
      expect(formatDatetime('2026-07-13T12:30:00Z', 'UTC')).toContain('12:30')
    })

    it('interprets a NAIVE value consistently with the edit widget (RR-P9NKU7)', () => {
      // The key property: display (formatDatetime) and edit (utcISOToLocalInput)
      // must resolve a naive value (no Z/offset) to the SAME wall-clock in the
      // zone. Both now parse via TZDate, so they agree — previously formatDatetime
      // used native new Date() and diverged. The exact value isn't the contract;
      // the AGREEMENT between view and edit is.
      const naive = '2026-07-13T12:30:00'
      const tz = 'America/New_York'
      const editWallClock = utcISOToLocalInput(naive, tz) // e.g. 2026-07-13T06:30
      const shownTime = editWallClock.slice(11) // "06:30"
      const [h, m] = shownTime.split(':')
      const hour12 = String(Number(h) % 12 || 12)
      expect(formatDatetime(naive, tz)).toContain(`${hour12}:${m}`)
    })

    it('returns null for an unparseable value', () => {
      expect(formatDatetime('not-a-datetime', 'UTC')).toBeNull()
    })

    it('returns null (does not throw) on an invalid time zone', () => {
      expect(formatDatetime('2026-07-13T12:30:00Z', 'Not/AZone')).toBeNull()
    })
  })

  describe('formatCellValue', () => {
    const mockEntityType: EntityType = {
      label: 'Test',
      description: '',
      properties: {
        created_at: { type: 'date' },
        starts_at: { type: 'datetime' },
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

    it('formats date property with short month name and exact day', () => {
      const result = formatCellValue('2024-01-15', 'created_at', mockEntityType)
      expect(result).toMatch(/15/)
      expect(result).toMatch(/2024/)
    })

    it('returns empty string for invalid date property (matches cell-empty sentinel)', () => {
      expect(formatCellValue('invalid', 'created_at', mockEntityType)).toBe('')
    })

    it('formats a datetime cell in the passed display zone (honors the tz override, RR-K3WEW2)', () => {
      // 12:30 UTC in New York (UTC-4 in July) is 08:30 — the list column must
      // honor the effective zone threaded from EntityList, not the browser zone.
      const out = formatCellValue('2026-07-13T12:30:00Z', 'starts_at', mockEntityType, 'America/New_York')
      expect(out).toContain('8:30')
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

  describe('formatDate', () => {
    it('formats YYYY-MM-DD as e.g. "Jan 15, 2024" in en-US', () => {
      expect(formatDate('2024-01-15', 'en-US')).toBe('Jan 15, 2024')
    })

    it('formats YYYY-MM-DD as e.g. "15 Jan 2024" in en-GB', () => {
      expect(formatDate('2024-01-15', 'en-GB')).toBe('15 Jan 2024')
    })

    it('preserves day-of-month regardless of host timezone', () => {
      // Date-only strings must render in local time so a user in UTC-12
      // does not see the previous day. Constructing via parseDate's
      // component-wise path avoids the UTC-midnight pitfall of
      // `new Date('2024-01-15')`.
      expect(formatDate('2024-01-15', 'en-US')).toContain('15')
      expect(formatDate('2024-12-31', 'en-US')).toContain('31')
    })

    it('returns null for invalid input', () => {
      expect(formatDate('not-a-date')).toBeNull()
      expect(formatDate('')).toBeNull()
      expect(formatDate('2024-13-45')).toBeNull()
    })

    it('uses host locale when none provided', () => {
      const result = formatDate('2024-01-15')
      expect(typeof result).toBe('string')
      expect(result).toMatch(/2024/)
    })
  })
})
