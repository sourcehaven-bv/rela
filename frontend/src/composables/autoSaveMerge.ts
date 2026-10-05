// Three-way merge rules for autosave conflicts (TKT-2VDVHF).
//
// When a PATCH precondition fails, autosave holds three versions of each
// conflicting field: the BASE its edit started from, OURS (the edit), and
// THEIRS (what the server stores now). These functions decide what, if
// anything, to write. They never produce conflict markers: a merge that
// cannot be done cleanly is reported, and nothing is written for that field.

import { diff3Merge } from 'node-diff3'
import type { ModernRelationsField, ResourceIdentifier } from '@/types'

// A failed merge still carries `oursInConflicts`: every clean hunk merged,
// and OUR lines in each conflicting region. Showing that keeps the other
// side's non-conflicting changes, so a later save can only overwrite the
// regions that actually conflicted.
export type TextMerge = { ok: true; merged: string } | { ok: false; oursInConflicts: string }

/**
 * Line-based three-way merge of a markdown body. Two edits to the same line
 * conflict; edits to different lines combine. Identical edits on both sides
 * are not a conflict.
 */
export function mergeText(base: string, ours: string, theirs: string): TextMerge {
  if (ours === theirs) return { ok: true, merged: ours }
  if (theirs === base) return { ok: true, merged: ours }
  if (ours === base) return { ok: true, merged: theirs }
  const regions = diff3Merge(ours.split('\n'), base.split('\n'), theirs.split('\n'))
  const lines: string[] = []
  let conflicted = false
  for (const region of regions) {
    if (region.conflict) {
      conflicted = true
      lines.push(...region.conflict.a)
    } else {
      lines.push(...(region.ok ?? []))
    }
  }
  const text = lines.join('\n')
  return conflicted ? { ok: false, oursInConflicts: text } : { ok: true, merged: text }
}

export type PropertyMerge =
  | { kind: 'write' } // theirs still equals base: our edit applies
  | { kind: 'same' } // theirs already holds our value: nothing to write
  | { kind: 'conflict' } // both changed, to different values

/** Decides one conflicting property. `undefined` means unset. */
export function mergeProperty(
  base: unknown,
  ours: unknown,
  theirs: unknown,
  equal: (a: unknown, b: unknown) => boolean
): PropertyMerge {
  if (equal(theirs, ours)) return { kind: 'same' }
  if (equal(theirs, base)) return { kind: 'write' }
  return { kind: 'conflict' }
}

/**
 * Merges the relations body of a PATCH as sets, per outgoing relation type:
 * the result is THEIRS plus the edges OURS added, minus the edges OURS
 * removed, both measured against BASE.
 *
 * `incoming` names body keys that address incoming edges; those are not in
 * the entity's `relations` map, so they pass through unchanged. A delta entry
 * (`add`/`remove`) also passes through: it names only the edges it changes,
 * so it already composes with whatever THEIRS did to the others.
 *
 * `typeOf` resolves the entity type of an edge only THEIRS has. A PATCH entry
 * needs it and the relations map carries ids only. Returns null when a type is
 * unknown, because writing the list without that edge would delete it.
 */
export function mergeRelations(
  base: Record<string, string[]>,
  ours: ModernRelationsField,
  theirs: Record<string, string[]>,
  incoming: (key: string) => boolean,
  typeOf: (id: string) => string | undefined
): ModernRelationsField | null {
  const out: ModernRelationsField = {}
  for (const [key, update] of Object.entries(ours)) {
    if (incoming(key) || !('data' in update)) {
      out[key] = update
      continue
    }
    const baseIds = new Set(base[key] ?? [])
    const ourEntries = new Map(update.data.map((e) => [e.id, e]))
    const removed = new Set([...baseIds].filter((id) => !ourEntries.has(id)))
    const data: ResourceIdentifier[] = []
    const seen = new Set<string>()
    for (const id of theirs[key] ?? []) {
      if (removed.has(id) || seen.has(id)) continue
      seen.add(id)
      const mine = ourEntries.get(id)
      if (mine) {
        data.push(mine)
        continue
      }
      const type = typeOf(id)
      if (!type) return null
      data.push({ type, id })
    }
    for (const [id, entry] of ourEntries) {
      if (seen.has(id) || baseIds.has(id)) continue
      seen.add(id)
      data.push(entry)
    }
    out[key] = { data }
  }
  return out
}
