/**
 * Frontend filter state types.
 *
 * The shape is intentionally `Record<property, FilterValue>` rather than a flat
 * map of strings so that operators round-trip through the URL. A filter without
 * an operator means equality (the default).
 */

export interface FilterValue {
  /** The string value to compare against. */
  value: string
  /**
   * UI operator symbol (=, !=, <, <=, >, >=, ~, in). Omitted means "=".
   * Stored as the UI symbol so it can be displayed by FilterBar widgets;
   * convert to API form via toApiOperator at request time.
   */
  op?: string
}

/** Property name → filter value/op pair. */
export type FilterState = Record<string, FilterValue>
