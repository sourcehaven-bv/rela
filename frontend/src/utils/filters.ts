/**
 * Filter utilities for converting UI operators to API operators
 */

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
 * Convert a UI operator to its API equivalent
 */
export function toApiOperator(operator: string | undefined): string {
  return OPERATOR_MAP[operator || '='] || 'eq'
}

/**
 * Build a filter query parameter key
 */
export function buildFilterKey(property: string, operator: string | undefined): string {
  const apiOp = toApiOperator(operator)
  return `filter[${property}][${apiOp}]`
}
