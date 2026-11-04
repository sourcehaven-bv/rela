// Totals under a table section (TKT-4Q2EIVZ). The values are the entry's own
// properties, which the server has already gated for the reader and in which
// it has merged roll-ups, so nothing is summed here.

import type { EntityType } from '@/types'
import type { SectionTotal } from '@/api/views'
import type { TotalRow } from 'rela-components/components/data/types'
import { formatCellValue, formatDisplayNumber } from '@/utils/format'

/**
 * The lines to show for a section's `totals`. A property the entry does not
 * carry (unset, or hidden from this reader) is left out rather than shown as
 * zero, which would read as a real sum.
 */
export function sectionTotalRows(
  totals: SectionTotal[] | undefined,
  properties: Record<string, unknown> | undefined,
  entityType: EntityType | undefined
): TotalRow[] {
  if (!totals?.length || !properties) return []
  const rows: TotalRow[] = []
  for (const total of totals) {
    const raw = properties[total.property]
    if (raw === undefined || raw === null || raw === '') continue
    const def = entityType?.properties?.[total.property]
    const value = formatDisplayNumber(raw, def) ?? formatCellValue(raw, total.property, entityType)
    rows.push({
      label: total.label || def?.label || total.property,
      value,
      emphasis: total.emphasis || undefined,
    })
  }
  return rows
}
