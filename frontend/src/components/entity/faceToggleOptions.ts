import type { Face } from '@/types'

/** One face in the record face toggle. */
export interface FaceToggleOption {
  /** The stored coordinate. */
  face: string
  label: string
  /** The face's address (`POL-1@published`). */
  ref: string
}

/**
 * The faces the record toggle offers, in declaration order, or null when the
 * toggle does not apply and the face menu is used instead.
 *
 * `faces` is the entry's `_faces`, which leaves out the face on screen, so
 * the total is one more. The toggle applies to two or three faces in total;
 * more read better as a menu. Every face must have a stored name: the bare
 * face of a faceless type has no operator word to show (TKT-5SZG2L).
 */
export function faceToggleOptions(
  bareId: string,
  current: string,
  faces: Face[] | undefined,
  declared: Record<string, { label?: string }> | undefined,
): FaceToggleOption[] | null {
  if (!current || !faces || faces.length < 1 || faces.length > 2) return null
  if (faces.some((f) => !f.face)) return null
  const labelOf = (face: string) => declared?.[face]?.label || face
  const all: FaceToggleOption[] = [
    { face: current, label: labelOf(current), ref: `${bareId}@${current}` },
    ...faces.map((f) => ({ face: f.face, label: f.label || labelOf(f.face), ref: f.ref || `${bareId}@${f.face}` })),
  ]
  // Declaration order, so the segments stay put as the reader switches.
  const order = Object.keys(declared ?? {})
  const rank = (face: string) => {
    const i = order.indexOf(face)
    return i < 0 ? order.length : i
  }
  return all.sort((a, b) => rank(a.face) - rank(b.face))
}
