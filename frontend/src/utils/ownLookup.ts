/** Returns table[key] only for an own key, so `constructor` or `toString` find nothing. */
export function ownLookup<V>(
  table: Readonly<Partial<Record<string, V>>>,
  key: string
): V | undefined {
  return Object.prototype.hasOwnProperty.call(table, key) ? table[key] : undefined
}
