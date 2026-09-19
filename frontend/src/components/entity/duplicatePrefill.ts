import type { DuplicateConfig } from '@/types/config'
import type { Entity, RelationEntry } from '@/types/entity'
import type { EntityType } from '@/types/schema'

/**
 * The prefill payload a Duplicate hands to the embedded create form.
 *
 * Shaped like a template on purpose: `applyTemplate` already writes typed
 * properties, content and relations into the form and re-baselines its
 * unsaved-changes guard, so a duplicate candidate is a template computed from
 * an entity rather than a second apply path.
 */
export interface DuplicatePrefill {
  properties: Record<string, unknown>
  content: string
  /**
   * Peers to pre-link, keyed exactly as `GET /{plural}/{id}/relations`
   * returned them — canonical names for outgoing edges, inverse names for
   * incoming ones. The create body resolves both, so these are passed through
   * untranslated.
   */
  relations: Record<string, string[]>
  /**
   * Properties the source withheld or that cannot be carried, for disclosure
   * to the user. A duplicate that quietly drops fields is the one unacceptable
   * outcome, so the caller surfaces these rather than producing a short copy
   * in silence.
   */
  omitted: OmittedProperty[]
}

export interface OmittedProperty {
  property: string
  reason: 'redacted' | 'file' | 'state-machine' | 'not-configured'
}

/**
 * Builds the prefill for a duplicate of `source`.
 *
 * Four categories of property do not carry, and each is reported rather than
 * dropped silently:
 *
 *   - `redacted`: field-level `visible:` ACL withheld the value on read, so the
 *     client never had it. (This covers `visible:` only — git-crypt locked
 *     content and relation-meta redaction are different mechanisms.)
 *   - `file`: attachments are stored under the SOURCE entity's id, so copying
 *     the filename would point the duplicate at bytes that are not its own.
 *   - `state-machine`: a create is an ENTRY, not a transition. The server pins
 *     these to the machine's entry value, so carrying the source's value would
 *     be silently replaced rather than honoured.
 *   - `not-configured`: an operator `duplicate.properties` allowlist excluded it.
 *     Reported at a lower stakes level than the rest — this one is intended.
 */
export function buildDuplicatePrefill(
  source: Entity,
  typeInfo: EntityType | undefined,
  relations: Record<string, RelationEntry[]>,
  selectedRelationKeys: string[],
  config: DuplicateConfig | undefined
): DuplicatePrefill {
  const properties: Record<string, unknown> = {}
  const omitted: OmittedProperty[] = []

  // Only state-machine-typed fields appear in `_transitions`; a plain enum has
  // no key there, so this does not over-exclude ordinary enums.
  const machineProps = new Set(Object.keys(source._transitions ?? {}))

  for (const [name, value] of Object.entries(source.properties ?? {})) {
    if (machineProps.has(name)) {
      omitted.push({ property: name, reason: 'state-machine' })
      continue
    }
    if (typeInfo?.properties?.[name]?.type === 'file') {
      omitted.push({ property: name, reason: 'file' })
      continue
    }
    if (config && !carriesProperty(config, name)) {
      omitted.push({ property: name, reason: 'not-configured' })
      continue
    }
    properties[name] = value
  }

  // `_redacted` names what the server withheld, which is NOT inferable from a
  // missing key: an unset property is absent too (BUG-MLT9DE).
  for (const name of source._redacted ?? []) {
    omitted.push({ property: name, reason: 'redacted' })
  }

  const selected = new Set(selectedRelationKeys)
  const outRelations: Record<string, string[]> = {}
  for (const [key, edges] of Object.entries(relations)) {
    if (!selected.has(key)) continue
    const peers = edges.map((e) => e.id)
    if (peers.length > 0) outRelations[key] = peers
  }

  return {
    properties,
    content: source.content ?? '',
    relations: outRelations,
    omitted,
  }
}

/** Mirrors DuplicateConfig.CarriesProperty on the Go side. */
function carriesProperty(config: DuplicateConfig, name: string): boolean {
  if (!config.properties || config.properties.length === 0) return true
  return config.properties.includes(name)
}

/** One selectable row in the relation picker. */
export interface RelationChoice {
  /** Wire key — canonical for outgoing, inverse for incoming. */
  key: string
  direction: 'outgoing' | 'incoming'
  count: number
}

/**
 * Turns the relations response into picker rows.
 *
 * A group with no visible edges is omitted entirely rather than shown as a
 * zero row: the caller cannot distinguish "type has no edges" from "every peer
 * is hidden", and offering an empty checkbox would imply the former.
 */
export function relationChoices(
  relations: Record<string, RelationEntry[]>
): RelationChoice[] {
  const out: RelationChoice[] = []
  for (const [key, edges] of Object.entries(relations)) {
    if (!edges || edges.length === 0) continue
    // Every edge in a group shares a direction; the first is representative.
    const direction = edges[0].direction === 'incoming' ? 'incoming' : 'outgoing'
    out.push({ key, direction, count: edges.length })
  }
  return out.sort(
    (a, b) => a.direction.localeCompare(b.direction) || a.key.localeCompare(b.key)
  )
}

/**
 * Default selection: outgoing types checked, incoming unchecked.
 *
 * An incoming edge means "something else points at me", which is usually not
 * part of what the user is duplicating — but it is offered, because sometimes
 * it is.
 */
export function defaultSelection(choices: RelationChoice[]): string[] {
  return choices.filter((c) => c.direction === 'outgoing').map((c) => c.key)
}
