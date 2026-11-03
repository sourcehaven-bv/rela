// Per-field version tokens, ported from internal/dataentry/fieldversions.go.
//
// The fake issues the same tokens as the server, byte for byte, so the
// contract test can compare `_versions` and 412 bodies exactly. A token is
// derived from the value, not from a write counter: writing A, then B, then A
// again gives back A's first token. A counter-based fake would disagree with
// the server on exactly the sequence an autosave merge depends on.

import { createHash } from 'node:crypto'

const CONTENT_FIELD = '\x00content'
const RELATIONS_FIELD = '\x00relations'

/** Hashes one field of one entity, as `fieldToken` does in Go. */
export function fieldToken(
  id: string,
  type: string,
  field: string,
  value: unknown,
  present: boolean
): string {
  const h = createHash('sha256')
  h.update(`${id}\x00${type}\x00${field}\x00`, 'utf8')
  if (!present) {
    h.update(Buffer.from([0]))
  } else {
    h.update(Buffer.from([1]))
    h.update(goJSON(value), 'utf8')
  }
  return h.digest('hex').slice(0, 8)
}

/** Hashes a relations map as a set, as `relationsToken` does in Go. */
export function relationsToken(id: string, type: string, rels: Record<string, string[]>): string {
  let s = ''
  for (const t of Object.keys(rels)
    .filter((k) => rels[k]!.length > 0)
    .sort(compareUTF8)) {
    s += t
    for (const to of [...rels[t]!].sort(compareUTF8)) s += `\x00${to}`
    s += '\n'
  }
  return fieldToken(id, type, RELATIONS_FIELD, s, true)
}

export interface FieldVersions {
  properties: Record<string, string>
  content: string
  relations: string
}

/**
 * The tokens for a per-entity response: every property on the wire, plus
 * every declared property that is unset. The fake has no redaction, so no
 * property is ever left out.
 */
export function fieldVersionsOf(
  id: string,
  type: string,
  properties: Record<string, unknown>,
  content: string,
  declared: readonly string[]
): FieldVersions {
  const props: Record<string, string> = {}
  for (const [k, v] of Object.entries(properties)) props[k] = fieldToken(id, type, k, v, true)
  for (const k of declared) {
    if (!(k in props)) props[k] = fieldToken(id, type, k, null, false)
  }
  return {
    properties: props,
    content: fieldToken(id, type, CONTENT_FIELD, content, true),
    relations: relationsToken(id, type, {}),
  }
}

/**
 * Encodes a value as Go's `encoding/json` does: object keys sorted by their
 * UTF-8 bytes, and `<`, `>`, `&`, U+2028 and U+2029 escaped. `JSON.stringify`
 * does neither, so hashing its output would give a different token for any
 * map value or any string with one of those characters.
 */
export function goJSON(value: unknown): string {
  if (value === null || value === undefined) return 'null'
  if (Array.isArray(value)) return `[${value.map(goJSON).join(',')}]`
  if (typeof value === 'object') {
    const obj = value as Record<string, unknown>
    const keys = Object.keys(obj).sort(compareUTF8)
    return `{${keys.map((k) => `${goString(k)}:${goJSON(obj[k])}`).join(',')}}`
  }
  if (typeof value === 'string') return goString(value)
  return JSON.stringify(value)
}

function goString(s: string): string {
  return JSON.stringify(s).replace(
    /[<>&\u2028\u2029]/g,
    (c) => `\\u${c.charCodeAt(0).toString(16).padStart(4, '0')}`
  )
}

/** Orders strings by their UTF-8 bytes, which is how Go's sort.Strings orders them. */
export function compareUTF8(a: string, b: string): number {
  return Buffer.compare(Buffer.from(a, 'utf8'), Buffer.from(b, 'utf8'))
}

/**
 * Replaces lone UTF-16 surrogates with U+FFFD, as Go's JSON decoder does with
 * a `\ud800` escape in a request body. The fake applies it to every string it
 * stores, so it holds the value the server would hold.
 */
export function goDecodeString(s: string): string {
  return s.replace(/[\ud800-\udbff](?![\udc00-\udfff])|(?<![\ud800-\udbff])[\udc00-\udfff]/g, '�')
}
