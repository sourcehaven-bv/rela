import type { EntityWorld } from '@/types'

/**
 * Whether the world served this row as a stand-in rather than its first
 * choice: a `fallback-default`, or a `chain` entry after the first.
 *
 * `via` alone cannot answer it, because `chain` covers both the world's prime
 * and a later stand-in; `chain_position` tells them apart. The stand-in badge,
 * the relation picker and the copy dialog all ask this one question, so they
 * ask it here.
 *
 * Nil: accepts undefined (an unscoped read carries no `_world`), answering false.
 */
export function isStandIn(w: EntityWorld | undefined): boolean {
  if (!w) return false
  if (w.via === 'fallback-default') return true
  return w.via === 'chain' && (w.chain_position ?? 0) > 0
}
