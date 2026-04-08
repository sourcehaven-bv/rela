/**
 * Filter utilities for converting UI operators to API operators and for
 * round-tripping FilterState through URL query strings.
 */

import type { LocationQuery } from 'vue-router'
import type { FilterState, FilterValue } from '@/types/filters'

/**
 * Map UI operator symbols to API operator names
 */
export const OPERATOR_MAP: Record<string, string> = {
  '!=': 'ne',
  '=': 'eq',
  '==': 'eq',
  '>': 'gt',
  '>=': 'gte',
  '<': 'lt',
  '<=': 'lte',
  '~': 'contains',
}

/**
 * Inverse of OPERATOR_MAP for API → UI translation. Built once at module load.
 *
 * The explicit `eq: '='` override after the spread is load-bearing. Without
 * it, `Object.fromEntries` on `OPERATOR_MAP` entries last-write-wins on
 * duplicate API keys: `OPERATOR_MAP` maps BOTH `'='` and `'=='` to `'eq'`,
 * and iteration order (insertion order for plain objects) means `eq → '=='`
 * survives as the last write. That would make `fromApiOperator('eq')` return
 * `'=='`, which is valid UI but surprising. The override pins `eq → '='` so
 * the canonical form is the shorter symbol.
 *
 * `'in'` is added explicitly because it's its own UI symbol with no entry
 * in `OPERATOR_MAP`.
 */
export const API_TO_UI_OPERATOR: Record<string, string> = {
  ...Object.fromEntries(Object.entries(OPERATOR_MAP).map(([ui, api]) => [api, ui])),
  eq: '=',
  in: 'in',
}

/**
 * Convert a UI operator to its API equivalent
 */
export function toApiOperator(operator: string | undefined): string {
  return OPERATOR_MAP[operator || '='] || 'eq'
}

/**
 * Convert an API operator back to its UI symbol. Unknown / missing operators
 * default to '=' (the implicit equality form).
 */
export function fromApiOperator(operator: string | undefined): string {
  if (!operator) return '='
  return API_TO_UI_OPERATOR[operator] || '='
}

/**
 * Build a filter query parameter key
 */
export function buildFilterKey(property: string, operator: string | undefined): string {
  const apiOp = toApiOperator(operator)
  return `filter[${property}][${apiOp}]`
}

// --- URL ↔ FilterState helpers ---

/**
 * Match a query key like `filter[prop]`, `filter[prop][op]`, or `filter[prop][op][]`.
 * Captures: 1=property, 2=operator (optional), 3=array suffix (optional).
 */
const FILTER_KEY_RE = /^filter\[([^\]]+)\](?:\[([^\]]+)\])?(\[\])?$/

/**
 * Allowlist for property names. Accepts identifier-style names only:
 * alphanumeric + underscore, leading letter or underscore. This is narrower
 * than what the metamodel technically permits but deliberately so — anything
 * outside this set from a URL query is almost certainly a bug or an
 * injection attempt (e.g., `__proto__`, names with brackets, Unicode
 * confusables). Rejecting here short-circuits prototype pollution and makes
 * the parser's behavior predictable across engines.
 */
const PROPERTY_NAME_RE = /^[a-zA-Z_][a-zA-Z0-9_]*$/

/**
 * Parse a Vue Router LocationQuery into a FilterState. Skips null/empty values.
 *
 * Handles three URL formats:
 *   - filter[prop]=value             → {prop: {value}}
 *   - filter[prop][op]=value         → {prop: {value, op}}
 *   - filter[prop][op][]=a&[op][]=b  → {prop: {value: 'a,b', op}}  (multi-value)
 *
 * For repeated single-value keys (last-write-wins), later values overwrite earlier.
 * For multi-value (array suffix `[]`), all values are joined with commas.
 */
export function parseFilterQueryParams(query: LocationQuery): FilterState {
  const result: FilterState = {}

  for (const [key, rawValue] of Object.entries(query)) {
    const match = FILTER_KEY_RE.exec(key)
    if (!match) continue

    const property = match[1]
    if (!PROPERTY_NAME_RE.test(property)) {
      // Reject non-identifier property names. See PROPERTY_NAME_RE above.
      continue
    }
    const apiOp = match[2]
    const isArray = match[3] === '[]'

    // Collect non-null/non-empty values
    const values: string[] = []
    if (Array.isArray(rawValue)) {
      for (const v of rawValue) {
        if (v != null && v !== '') values.push(v)
      }
    } else if (rawValue != null && rawValue !== '') {
      values.push(rawValue)
    }
    if (values.length === 0) continue

    const value = isArray ? values.join(',') : values[values.length - 1]
    const filterValue: FilterValue = { value }

    // 'eq' is the implicit default — omit it from the state to keep things clean
    if (apiOp && apiOp !== 'eq') {
      filterValue.op = fromApiOperator(apiOp)
    }

    result[property] = filterValue
  }

  return result
}

/**
 * Build a new query object that merges the given FilterState into the existing
 * query while:
 *   - Removing all existing `filter[*]` entries (so cleared filters disappear)
 *   - Preserving all non-filter params (e.g. `from`, `sort`, `page`, `scope`)
 *   - Writing each FilterState entry in the most concise form (no operator
 *     suffix when default)
 */
export function buildQueryWithFilters(
  currentQuery: LocationQuery,
  filters: FilterState,
): LocationQuery {
  const result: LocationQuery = {}

  // Keep non-filter params
  for (const [key, value] of Object.entries(currentQuery)) {
    if (FILTER_KEY_RE.test(key)) continue
    result[key] = value
  }

  // Add new filter params
  for (const [property, fv] of Object.entries(filters)) {
    if (!fv || fv.value === '') continue
    if (!fv.op || fv.op === '=') {
      result[`filter[${property}]`] = fv.value
    } else {
      const apiOp = toApiOperator(fv.op)
      result[`filter[${property}][${apiOp}]`] = fv.value
    }
  }

  return result
}

/**
 * Serialize a FilterState into flat bracket-format API params suitable for
 * passing to `entitiesStore.fetchList`. This is the single source of truth
 * for how user-selected filters hit the backend — every call site (EntityList,
 * useScopeNavigation) should funnel through this helper so the wire format
 * stays consistent and a change only needs to be made in one place.
 *
 * Uses `filter[prop]` (no operator suffix) for the default `=` form and
 * `filter[prop][api_op]` otherwise, mirroring buildQueryWithFilters.
 */
export function filterStateToApiParams(
  filters: FilterState,
): Record<string, string> {
  const params: Record<string, string> = {}
  for (const [property, fv] of Object.entries(filters)) {
    if (!fv || fv.value === '') continue
    const key = (!fv.op || fv.op === '=')
      ? `filter[${property}]`
      : buildFilterKey(property, fv.op)
    params[key] = fv.value
  }
  return params
}

/**
 * Produce a deterministic, comparable string representation of a query so the
 * URL-sync composable can detect "did the URL change because of us" without
 * false positives from key ordering — AND without false negatives from values
 * that contain `&` or `=` (which would collide if we used a naive
 * `key=value&…` form since Vue Router stores decoded values).
 *
 * We serialize via JSON.stringify on a sorted entries array. This is
 * unambiguous: any character inside a string is JSON-escaped, and the array
 * form preserves multi-value (array) keys explicitly.
 */
export function stringifyFilterQuery(query: LocationQuery): string {
  const entries: Array<[string, string | (string | null)[] | null]> = []
  for (const [key, value] of Object.entries(query)) {
    entries.push([key, Array.isArray(value) ? [...value] : value])
  }
  entries.sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
  return JSON.stringify(entries)
}
