/**
 * Building an `EntityRefResolver` from a server-supplied mentions map.
 *
 * Two surfaces render entity references: the read view, which rewrites code
 * spans into titled links, and the editor, which shows them as link nodes.
 * Both must show the same title for the same ID, so both build their resolver
 * here rather than each unpacking the wire shape their own way.
 *
 * The map is the ONLY source of titles. It is computed server-side per
 * principal through the read gate, so an entity the caller may not read has no
 * entry and its reference stays a bare ID. Deriving a title from anywhere else
 * (a store cache, a list response, a relation label) would route around that
 * gate.
 */
import type { EntityRefResolver } from './markdown'
import type { Mention } from '@/types'

/**
 * Returns a resolver over `mentions`, or undefined when there is no map.
 *
 * Undefined rather than a resolver that always returns null, because the two
 * mean different things to a caller: no map at all is "this response carried
 * no mention data", and a resolver returning null for an ID is "the server
 * considered that ID and declined to resolve it".
 *
 * Nil: accepts undefined and null for `mentions`, returning undefined.
 */
export function makeRefResolver(
  mentions: Record<string, Mention> | undefined | null
): EntityRefResolver | undefined {
  if (!mentions) return undefined
  return (id) => {
    const m = mentions[id]
    if (!m) return null
    return {
      type: m.type,
      title: m.title,
      inaccessible: m.inaccessible,
      inaccessibleReason: m.inaccessible_reason,
    }
  }
}
