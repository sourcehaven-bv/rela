import { describe, it, expect } from 'vitest'
import type { LocationQuery } from 'vue-router'
import {
  OPERATOR_MAP,
  toApiOperator,
  buildFilterKey,
  fromApiOperator,
  parseFilterQueryParams,
  buildQueryWithFilters,
  stringifyFilterQuery,
  filterStateToApiParams,
} from './filters'
import type { FilterState } from '@/types/filters'

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

  describe('fromApiOperator', () => {
    it('converts known API operators back to UI symbols', () => {
      expect(fromApiOperator('ne')).toBe('!=')
      expect(fromApiOperator('eq')).toBe('=')
      expect(fromApiOperator('gt')).toBe('>')
      expect(fromApiOperator('gte')).toBe('>=')
      expect(fromApiOperator('lt')).toBe('<')
      expect(fromApiOperator('lte')).toBe('<=')
      expect(fromApiOperator('contains')).toBe('~')
    })

    it('passes through in unchanged', () => {
      expect(fromApiOperator('in')).toBe('in')
    })

    it('defaults to = for undefined', () => {
      expect(fromApiOperator(undefined)).toBe('=')
    })

    it('defaults to = for unknown operators', () => {
      expect(fromApiOperator('bogus')).toBe('=')
    })
  })

  describe('parseFilterQueryParams', () => {
    it('returns empty object for empty query', () => {
      expect(parseFilterQueryParams({})).toEqual({})
    })

    it('parses simple filter[prop]=value form', () => {
      expect(parseFilterQueryParams({ 'filter[status]': 'open' })).toEqual({
        status: { value: 'open' },
      })
    })

    it('omits operator when api op is eq', () => {
      expect(parseFilterQueryParams({ 'filter[status][eq]': 'open' })).toEqual({
        status: { value: 'open' },
      })
    })

    it('translates api operator to UI symbol', () => {
      expect(parseFilterQueryParams({ 'filter[due_date][lte]': '$today' })).toEqual({
        due_date: { value: '$today', op: '<=' },
      })
    })

    it('joins multi-value array form with commas', () => {
      expect(parseFilterQueryParams({ 'filter[tags][in][]': ['a', 'b'] })).toEqual({
        tags: { value: 'a,b', op: 'in' },
      })
    })

    it('skips null and empty values', () => {
      const query: LocationQuery = {
        'filter[a]': null,
        'filter[b]': '',
        'filter[c]': 'keep',
      }
      expect(parseFilterQueryParams(query)).toEqual({ c: { value: 'keep' } })
    })

    it('skips array values that are entirely empty/null', () => {
      const query: LocationQuery = {
        'filter[tags][in][]': [null, ''],
      }
      expect(parseFilterQueryParams(query)).toEqual({})
    })

    it('last-write-wins for repeated single-value keys', () => {
      const query: LocationQuery = {
        'filter[status]': ['first', 'second'],
      }
      expect(parseFilterQueryParams(query)).toEqual({ status: { value: 'second' } })
    })

    it('rejects non-identifier property names (prototype pollution guard)', () => {
      // __proto__, brackets, whitespace, and non-identifier starts are all
      // dropped rather than being passed through to a plain-object index.
      const query: LocationQuery = {
        'filter[__proto__]': 'x',
        'filter[a b]': 'x',
        'filter[1abc]': 'x',
        'filter[a-b]': 'x',
        'filter[valid_name]': 'keep',
      }
      expect(parseFilterQueryParams(query)).toEqual({
        valid_name: { value: 'keep' },
      })
    })

    it('ignores non-filter query params', () => {
      const query: LocationQuery = {
        page: '2',
        from: 'list',
        'filter[status]': 'open',
      }
      expect(parseFilterQueryParams(query)).toEqual({ status: { value: 'open' } })
    })
  })

  describe('buildQueryWithFilters', () => {
    it('preserves non-filter params', () => {
      expect(buildQueryWithFilters({ page: '2' }, { status: { value: 'open' } })).toEqual({
        page: '2',
        'filter[status]': 'open',
      })
    })

    it('drops existing filter params and keeps non-filter params', () => {
      expect(buildQueryWithFilters({ 'filter[old]': 'x', from: 'list' }, {})).toEqual({
        from: 'list',
      })
    })

    it('omits operator suffix when op is default', () => {
      expect(buildQueryWithFilters({}, { status: { value: 'open', op: '=' } })).toEqual({
        'filter[status]': 'open',
      })
    })

    it('writes operator suffix when op is non-default', () => {
      expect(buildQueryWithFilters({}, { due_date: { value: '$today', op: '<=' } })).toEqual({
        'filter[due_date][lte]': '$today',
      })
    })

    it('skips entries with empty value', () => {
      expect(buildQueryWithFilters({}, { status: { value: '' } })).toEqual({})
    })

    it('round-trips parse(build({}, X)) === X', () => {
      const states: FilterState[] = [
        {},
        { status: { value: 'open' } },
        { due_date: { value: '$today', op: '<=' } },
        { name: { value: 'foo', op: '~' } },
        { priority: { value: 'high', op: '!=' } },
      ]
      for (const state of states) {
        expect(parseFilterQueryParams(buildQueryWithFilters({}, state))).toEqual(state)
      }
    })
  })

  describe('filterStateToApiParams', () => {
    it('returns empty for empty state', () => {
      expect(filterStateToApiParams({})).toEqual({})
    })

    it('omits operator suffix for default =', () => {
      expect(filterStateToApiParams({ status: { value: 'open' } })).toEqual({
        'filter[status]': 'open',
      })
    })

    it('omits operator suffix for explicit =', () => {
      expect(filterStateToApiParams({ status: { value: 'open', op: '=' } })).toEqual({
        'filter[status]': 'open',
      })
    })

    it('writes operator suffix for non-default ops', () => {
      expect(
        filterStateToApiParams({ due_date: { value: '$today', op: '<=' } }),
      ).toEqual({ 'filter[due_date][lte]': '$today' })
    })

    it('skips entries with empty value', () => {
      expect(filterStateToApiParams({ status: { value: '' } })).toEqual({})
    })
  })

  describe('stringifyFilterQuery', () => {
    it('is order-independent', () => {
      const a: LocationQuery = { 'filter[a]': '1', 'filter[b]': '2' }
      const b: LocationQuery = { 'filter[b]': '2', 'filter[a]': '1' }
      expect(stringifyFilterQuery(a)).toBe(stringifyFilterQuery(b))
    })

    it('preserves array values separately from single values', () => {
      const arrayForm: LocationQuery = { 'filter[tags][in][]': ['a', 'b'] }
      const singleForm: LocationQuery = { 'filter[tags][in][]': 'a,b' }
      expect(stringifyFilterQuery(arrayForm)).not.toBe(stringifyFilterQuery(singleForm))
    })

    it('handles null values', () => {
      const q1: LocationQuery = { 'filter[a]': null }
      const q2: LocationQuery = { 'filter[a]': '' }
      // Null and empty string must be distinguishable to avoid false echoes.
      expect(stringifyFilterQuery(q1)).not.toBe(stringifyFilterQuery(q2))
    })

    it('returns a canonical form for empty query', () => {
      expect(stringifyFilterQuery({})).toBe('[]')
    })

    it('does NOT collide when a value contains = or & (regression)', () => {
      // This was the RR-XO1V bug: naive `key=value&…` stringification
      // produced the same signature for these two DIFFERENT queries, so the
      // watcher would skip an external nav that looked like a self-echo.
      const twoKeys: LocationQuery = { 'filter[a]': 'x', 'filter[b]': 'y' }
      const oneKeyWithAmp: LocationQuery = { 'filter[a]': 'x&filter[b]=y' }
      expect(stringifyFilterQuery(twoKeys)).not.toBe(stringifyFilterQuery(oneKeyWithAmp))
    })

    it('does NOT collide on = in value', () => {
      const plain: LocationQuery = { 'filter[a]': 'x' }
      const withEq: LocationQuery = { 'filter[a]': 'x=y' }
      expect(stringifyFilterQuery(plain)).not.toBe(stringifyFilterQuery(withEq))
    })
  })
})
