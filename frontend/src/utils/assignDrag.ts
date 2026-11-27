import type { AssignSource } from '@/api/assign'

/** The drag payload MIME type of list rows that can be assigned. */
const ASSIGN_MIME = 'application/x-rela-assign'

export interface AssignDragPayload {
  entityType: string
  addresses: string[]
}

export function writeAssignDrag(dt: DataTransfer, p: AssignDragPayload): void {
  dt.setData(ASSIGN_MIME, JSON.stringify(p))
  dt.effectAllowed = 'move'
}

export function readAssignDrag(dt: DataTransfer | null): AssignDragPayload | null {
  const raw = dt?.getData(ASSIGN_MIME)
  if (!raw) return null
  try {
    const p = JSON.parse(raw) as AssignDragPayload
    return typeof p.entityType === 'string' && Array.isArray(p.addresses) ? p : null
  } catch {
    return null
  }
}

/** Whether a drag carries assignable rows; the payload is unreadable before drop. */
export function hasAssignDrag(dt: DataTransfer | null): boolean {
  return !!dt && Array.from(dt.types).includes(ASSIGN_MIME)
}

interface AssignLookup {
  listAssignType(id: string): string | undefined
  pageAssignType(id: string): string | undefined
}

/**
 * The block a sidebar link drops onto: `/list/ID` for a list with an assign,
 * `/p/PAGE/ID` for a page with one. Returns the item type it accepts.
 */
export function assignTargetForHref(
  href: string,
  lookup: AssignLookup
): { src: AssignSource; entityType: string } | null {
  const path = href.split(/[?#]/)[0]
  let m = /\/list\/([^/]+)$/.exec(path)
  if (m) {
    const list = decodeURIComponent(m[1])
    const t = lookup.listAssignType(list)
    return t ? { src: { list }, entityType: t } : null
  }
  m = /\/p\/([^/]+)\/([^/]+)$/.exec(path)
  if (m) {
    const page = decodeURIComponent(m[1])
    const t = lookup.pageAssignType(page)
    return t ? { src: { page, target: decodeURIComponent(m[2]) }, entityType: t } : null
  }
  return null
}
