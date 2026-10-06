import type { EntityType } from '@/types/schema'

/**
 * The ID prefixes a type declares. The API sends the same list in both fields;
 * `id_prefixes` is read first because it carries every prefix.
 */
function prefixesOf(def: EntityType): string[] {
  if (def.id_prefixes?.length) return def.id_prefixes
  return def.id_prefix ? [def.id_prefix] : []
}

/**
 * Whether `id` starts with `prefix` (case-insensitive).
 *
 * A prefix may be configured with or without its trailing dash (`MOD-` or
 * `MOD`); generated ids always carry the dash. A dashless prefix must end at a
 * non-letter, so `TC` does not claim `TCX-1`.
 */
function idHasPrefix(id: string, prefix: string): boolean {
  if (prefix === '' || id.length < prefix.length) return false
  if (id.slice(0, prefix.length).toUpperCase() !== prefix.toUpperCase()) return false
  if (/[-_]$/.test(prefix)) return true
  const next = id.charAt(prefix.length)
  return next === '' || !/[A-Za-z]/.test(next)
}

/**
 * The entity type an id belongs to, from its ID prefix, or undefined.
 *
 * Reads both `id_prefix` and `id_prefixes`, in either spelling. When several
 * prefixes match, the longest wins, so `SPEC-` beats `S-`. A prefix-less id
 * (manual ids such as `backend`) has no type here.
 */
export function entityTypeForId(
  id: string,
  types: Iterable<[string, EntityType]>
): string | undefined {
  let best: { type: string; length: number } | undefined
  for (const [name, def] of types) {
    for (const prefix of prefixesOf(def)) {
      if (!idHasPrefix(id, prefix)) continue
      if (!best || prefix.length > best.length) best = { type: name, length: prefix.length }
    }
  }
  return best?.type
}
