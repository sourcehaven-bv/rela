// An in-process fake of the data-entry API, for SPA tests.
//
// It covers the entity-editing slice only: the schema and config snapshot,
// entity get/list/create/update/delete, per-field `_versions` with 412
// conflicts, an entity view's properties and body sections, presence, mentioned-by,
// `entity:changed` events and live subscriptions. `frontend/contract/` runs the same requests
// against a real `rela-server` on the memory store and compares the answers,
// so this file may only change together with a passing contract run.
//
// What it does NOT model, so a test passing against it says nothing about:
// ACL (every `_actions` verb is the answer of a project without acl.yaml),
// automations, validation beyond the required-property warning, relations,
// faces and worlds. A route it does not implement answers 501 with the route
// in the body, so a test cannot pass by accident on an empty answer.

import { inertRoute } from './inert'
import { fieldVersionsOf, goDecodeString, compareUTF8, type FieldVersions } from './tokens'

export interface FakeSnapshot {
  schema: SchemaSnapshot
  config: ConfigSnapshot
}

interface SchemaSnapshot {
  entities: Record<string, EntityTypeSnapshot>
  [key: string]: unknown
}

interface EntityTypeSnapshot {
  plural: string
  primary?: string
  id_type?: string
  id_prefix?: string
  properties: Record<string, { type: string; required?: boolean }>
}

interface ConfigSnapshot {
  views?: Record<string, ViewConfigSnapshot>
  [key: string]: unknown
}

interface ViewConfigSnapshot {
  title?: string
  sections?: {
    heading?: string
    source?: string
    display?: string
    render?: string
    fields?: { property: string; label?: string; render?: string }[]
  }[]
}

export interface FakeRequest {
  method: string
  /** Path below `/api/v1`, without the query string. */
  path: string
  query: Record<string, string>
  body?: unknown
  headers?: Record<string, string>
}

export interface FakeResponse {
  status: number
  data: unknown
}

export interface FakeServerOptions {
  /**
   * The last sequence number already used per entity type, so the next
   * created id continues from it. The real server never reuses a number, not
   * even a deleted one; the contract test uses this to make the fake's ids
   * equal to those of a server that has been running for a while.
   */
  lastSequence?: Record<string, number>
}

export interface FakeEvent {
  event: string
  data: unknown
}

/** One open `/_events` stream. */
export interface FakeStream {
  /** The connection id the stream's `ready` frame carries. */
  readonly conn: string
  /**
   * Replays every subscription of the stream and returns a `live:changed`
   * for each one whose answer moved since its baseline, moving the baseline.
   * The server runs a round per 500 ms window, so writes inside one window
   * share a round, and a value changed and changed back sends nothing. The
   * caller decides when a round runs, which is the fake's window.
   */
  liveRound(): FakeEvent[]
  close(): void
}

interface LiveSubscription {
  path: string
  baseline: string
}

interface StreamState {
  listener: (e: FakeEvent) => void
  subs: Map<string, LiveSubscription>
}

// The server's key allowlist (liveKeyPattern in live.go) and the subscribed
// GETs the fake answers. The server takes more paths; the fake answers 501
// for those.
const LIVE_KEY = /^[A-Za-z0-9_:.-]{1,64}$/
const LIVE_PATH = /^\/api\/v1\/(_views\/[^/?]+\/[^/?]+|[^_/?][^/?]*\/[^/?]+(\/_presence)?)$/
const LIVE_MAX_PER_CONN = 16

interface StoredEntity {
  id: string
  type: string
  properties: Record<string, unknown>
  // The wire leaves the key out when the body is empty, so an entity created
  // without a body and one with "" look the same.
  content: string
}

const ENTITY_ACTIONS = { delete: true, rename: false, update: true }
const COLLECTION_ACTIONS = { create: true }
const DEFAULT_PER_PAGE = 25

export class FakeServer {
  private readonly entities = new Map<string, StoredEntity>()
  private readonly nextSeq = new Map<string, number>()
  private readonly streams = new Map<string, StreamState>()
  private streamSeq = 0
  private readonly byPlural = new Map<string, string>()

  constructor(
    private readonly snapshot: FakeSnapshot,
    opts: FakeServerOptions = {}
  ) {
    for (const [type, def] of Object.entries(snapshot.schema.entities))
      this.byPlural.set(def.plural, type)
    for (const [type, n] of Object.entries(opts.lastSequence ?? {})) this.nextSeq.set(type, n)
  }

  /**
   * Opens a stream, as `GET /_events` does. The listener receives an
   * `entity:changed` per successful write; live frames come from the
   * stream's `liveRound`.
   */
  connect(listener: (e: FakeEvent) => void): FakeStream {
    const conn = `fake-${++this.streamSeq}`
    const state: StreamState = { listener, subs: new Map() }
    this.streams.set(conn, state)
    return {
      conn,
      liveRound: () => {
        const frames: FakeEvent[] = []
        for (const [k, sub] of state.subs) {
          const now = this.replay(sub.path)
          if (now === sub.baseline) continue
          sub.baseline = now
          frames.push({ event: 'live:changed', data: { key: k } })
        }
        return frames
      },
      close: () => this.streams.delete(conn),
    }
  }

  /** Reads an entity as stored, for assertions. Undefined when absent. */
  stored(type: string, id: string): Readonly<StoredEntity> | undefined {
    const e = this.entities.get(key(type, id))
    return e && structuredClone(e)
  }

  handle(req: FakeRequest): FakeResponse {
    const method = req.method.toUpperCase()
    const parts = req.path.split('/').filter(Boolean).map(decodeURIComponent)
    // Only a list reads query parameters; it checks its own. Everywhere else
    // a parameter (`?world=`, `?include=`) asks for something not modelled,
    // and answering as if it were absent would pass a test by accident.
    const param = Object.keys(req.query)[0]
    const isList = parts.length === 1 && method === 'GET'
    if (param !== undefined && !isList && !inertRoute(method, req.path)) {
      return notImplemented(method, `${req.path}?${param}`)
    }

    if (method === 'GET' && req.path === '/_schema') return ok(this.snapshot.schema)
    if (method === 'GET' && req.path === '/_config') return ok(this.snapshot.config)
    if (method === 'GET' && parts[0] === '_views' && parts.length === 3)
      return this.view(parts[1]!, parts[2]!)
    if (
      method === 'PUT' &&
      parts[0] === '_events' &&
      parts.length === 3 &&
      parts[2] === 'subscriptions'
    )
      return this.putSubscriptions(parts[1]!, req)

    const inert = inertRoute(method, req.path)
    if (inert) return inert

    if (parts.length >= 1 && !parts[0]!.startsWith('_')) {
      const type = this.byPlural.get(parts[0]!)
      if (type === undefined) {
        return problem(404, 'unknown_type', 'Unknown entity type', `/api/v1/${parts[0]}`)
      }
      if (parts.length === 1 && method === 'GET') return this.list(type, req.query)
      if (parts.length === 1 && method === 'POST') return this.create(type, req)
      if (parts.length === 2 && method === 'GET') return this.get(type, parts[1]!)
      if (parts.length === 2 && method === 'PATCH') return this.patch(type, parts[1]!, req)
      if (parts.length === 2 && method === 'DELETE') return this.remove(type, parts[1]!)
      if (parts.length === 3 && parts[2] === '_presence' && method === 'GET')
        return this.presence(type, parts[1]!)
      if (parts.length === 3 && parts[2] === 'mentioned-by' && method === 'GET')
        return this.mentionedBy(type, parts[1]!)
    }
    return notImplemented(method, req.path)
  }

  private def(type: string): EntityTypeSnapshot {
    return this.snapshot.schema.entities[type]!
  }

  private list(type: string, query: Record<string, string>): FakeResponse {
    for (const k of Object.keys(query)) {
      if (k !== 'page' && k !== 'per_page')
        return notImplemented('GET', `/${this.def(type).plural}?${k}`)
    }
    const page = Number(query.page ?? 1)
    const perPage = Number(query.per_page ?? DEFAULT_PER_PAGE)
    const rows = [...this.entities.values()]
      .filter((e) => e.type === type)
      .sort((a, b) => compareUTF8(a.id, b.id))
    const slice = rows.slice((page - 1) * perPage, page * perPage)
    return ok({
      data: slice.map((e) => ({
        ...this.header(e),
        properties: structuredClone(e.properties),
        _actions: ENTITY_ACTIONS,
      })),
      meta: { total: rows.length, page, per_page: perPage, has_more: page * perPage < rows.length },
      _actions: COLLECTION_ACTIONS,
    })
  }

  private create(type: string, req: FakeRequest): FakeResponse {
    const body = (req.body ?? {}) as {
      id?: string
      properties?: Record<string, unknown>
      content?: string
    }
    if (body.id !== undefined) return notImplemented('POST', `${req.path} with an explicit id`)
    const def = this.def(type)
    const properties = this.withDefaults(
      type,
      decodeValue(body.properties ?? {}) as Record<string, unknown>
    )
    const refused = refuseNUL(properties, `/api/v1/${def.plural}`)
    // A refused create does not use up a sequence number.
    if (refused) return refused
    const seq = (this.nextSeq.get(type) ?? 0) + 1
    this.nextSeq.set(type, seq)
    const id = `${def.id_prefix}-${String(seq).padStart(3, '0')}`
    const stored: StoredEntity = {
      id,
      type,
      properties,
      content: goDecodeString(body.content ?? ''),
    }
    this.entities.set(key(type, id), stored)
    this.emit(type)
    const warnings = this.warnings(stored)
    return {
      status: 201,
      data: { ...this.wire(stored, false), ...(warnings.length ? { warnings } : {}) },
    }
  }

  private get(type: string, id: string): FakeResponse {
    const e = this.entities.get(key(type, id))
    if (!e) return entityNotFound(this.def(type).plural, id)
    return ok({ ...this.wire(e, true), _world: { name: 'default', face: '', via: 'unscoped' } })
  }

  // Nobody else is ever viewing: the fake has one client.
  private presence(type: string, id: string): FakeResponse {
    if (!this.entities.has(key(type, id))) {
      return entityNotFound(this.def(type).plural, `${id}/_presence`)
    }
    return ok({ viewers: [], more: 0 })
  }

  // No body ever mentions another entity: the contract test writes no
  // entity references into a body, and the server finds mentions by parsing
  // them.
  private mentionedBy(type: string, id: string): FakeResponse {
    if (!this.entities.has(key(type, id))) {
      return entityNotFound(this.def(type).plural, `${id}/mentioned-by`)
    }
    return ok({ items: [] })
  }

  private patch(type: string, id: string, req: FakeRequest): FakeResponse {
    const e = this.entities.get(key(type, id))
    const path = `/api/v1/${this.def(type).plural}/${id}`
    if (!e) return entityNotFound(this.def(type).plural, id)
    if (req.headers?.['If-Match'] !== undefined)
      return notImplemented('PATCH', `${req.path} with If-Match`)
    const body = (req.body ?? {}) as {
      properties?: Record<string, unknown>
      properties_unset?: string[]
      content?: string
      relations?: unknown
      preconditions?: { properties?: Record<string, string>; content?: string; relations?: string }
    }
    if (body.relations !== undefined) return notImplemented('PATCH', `${req.path} with relations`)

    const pre = body.preconditions
    if (pre) {
      const writes = new Set([
        ...Object.keys(body.properties ?? {}),
        ...(body.properties_unset ?? []),
      ])
      for (const p of Object.keys(pre.properties ?? {})) {
        if (!writes.has(p)) {
          return problem(
            400,
            'invalid_precondition',
            'Precondition names a field this request does not write',
            path,
            `/preconditions/properties/${p}`
          )
        }
      }
      if (pre.content !== undefined && body.content === undefined) {
        return problem(
          400,
          'invalid_precondition',
          'Precondition names a field this request does not write',
          path,
          '/preconditions/content'
        )
      }
      if (pre.relations !== undefined)
        return notImplemented('PATCH', `${req.path} with a relations precondition`)

      const current = this.versions(e)
      const conflicts: {
        properties?: Record<string, { expected: string; actual: string }>
        content?: unknown
      } = {}
      for (const [p, expected] of Object.entries(pre.properties ?? {})) {
        const actual = current.properties[p]!
        if (actual !== expected) (conflicts.properties ??= {})[p] = { expected, actual }
      }
      if (pre.content !== undefined && pre.content !== current.content) {
        conflicts.content = { expected: pre.content, actual: current.content }
      }
      if (conflicts.properties || conflicts.content) {
        return {
          status: 412,
          data: {
            type: 'https://rela.dev/errors/precondition_failed',
            title: 'Entity has been modified',
            status: 412,
            detail: 'field changed since it was read',
            instance: path,
            conflicts,
            versions: current,
          },
        }
      }
    }

    const merged = { ...e.properties }
    for (const [k, v] of Object.entries(body.properties ?? {})) merged[k] = decodeValue(v)
    for (const k of body.properties_unset ?? []) delete merged[k]
    const refused = refuseNUL(merged, path)
    if (refused) return refused
    e.properties = merged
    if (body.content !== undefined) e.content = goDecodeString(body.content)
    this.emit(type)
    const warnings = this.warnings(e)
    return ok({ ...this.wire(e, true), ...(warnings.length ? { warnings } : {}) })
  }

  private remove(type: string, id: string): FakeResponse {
    if (!this.entities.delete(key(type, id))) return entityNotFound(this.def(type).plural, id)
    this.emit(type)
    return { status: 204, data: '' }
  }

  private view(type: string, id: string): FakeResponse {
    const def = this.snapshot.schema.entities[type]
    if (!def)
      return problem(
        404,
        'entity_type_not_found',
        'Entity type not found',
        `/api/v1/_views/${type}/${id}`
      )
    const e = this.entities.get(key(type, id))
    // The server answers 422, not 404, for a missing entity here: with no
    // ACL the read gate admits any id, and the view engine then fails to load
    // the entry. GET /{plural}/{id} answers 404 for the same id.
    if (!e) {
      return problem(
        422,
        'view_execution_failed',
        'View execution failed',
        `/api/v1/_views/${type}/${id}`,
        `entry entity not found: ${id}`
      )
    }
    const config = this.snapshot.config.views?.[type]
    if (!config) return notImplemented('GET', `/_views/${type}/${id} without a configured view`)
    const sections = []
    for (const s of config.sections ?? []) {
      if (s.display === 'content' && (s.source === undefined || s.source === 'entry')) {
        // The entry's body. The wire leaves `content` out when the body is
        // empty, and then `hasContent` is false.
        sections.push({
          heading: s.heading,
          sectionId: (s.heading ?? '').toLowerCase(),
          display: 'content',
          isEmpty: false,
          isGrouped: false,
          ...(e.content !== '' ? { content: e.content } : {}),
          hasContent: e.content !== '',
        })
        continue
      }
      if (s.display !== 'properties' || (s.source !== undefined && s.source !== 'entry')) {
        return notImplemented('GET', `/_views/${type}/${id} with a ${s.display} section`)
      }
      const fields = (s.fields ?? []).map((f) => {
        const value = e.properties[f.property]
        return {
          property: f.property,
          label: f.label ?? f.property,
          ...(isEmptyValue(value) ? {} : { values: [value] }),
          propType: def.properties[f.property]?.type,
          ...((f.render ?? s.render) ? { render: f.render ?? s.render } : {}),
        }
      })
      sections.push({
        heading: s.heading,
        sectionId: (s.heading ?? '').toLowerCase(),
        display: 'properties',
        // The server sets isEmpty only for collection sections
        // (sections.go); an entry section is never empty.
        isEmpty: false,
        fields,
        isGrouped: false,
        hasContent: false,
      })
    }
    // A view's entry has no relations token (api-reference.md, "Per-field
    // preconditions").
    const entry = this.wire(e, true)
    const { relations: _relations, ...versions } = entry._versions!
    return ok({ entry: { ...entry, _versions: versions }, sections })
  }

  private withDefaults(type: string, props: Record<string, unknown>): Record<string, unknown> {
    const out = { ...props }
    const types = (this.snapshot.schema.types ?? {}) as Record<string, { default?: unknown }>
    for (const [name, p] of Object.entries(this.def(type).properties)) {
      const dflt = types[p.type]?.default
      if (!(name in out) && dflt !== undefined) out[name] = dflt
    }
    return out
  }

  // The one soft validation the fake copies: a required property that is
  // absent or empty (metamodel.IsEmptyValue). Sorted by path, as the server
  // sorts its warnings.
  private warnings(e: StoredEntity): { code: string; path: string; detail: string }[] {
    return Object.entries(this.def(e.type).properties)
      .filter(([name, p]) => p.required && isEmptyValue(e.properties[name]))
      .map(([name]) => ({
        code: 'required_property_unset',
        path: `/properties/${name}`,
        detail: 'This field is required',
      }))
      .sort((a, b) => compareUTF8(a.path, b.path))
  }

  private versions(e: StoredEntity): FieldVersions {
    return fieldVersionsOf(
      e.id,
      e.type,
      e.properties,
      e.content,
      Object.keys(this.def(e.type).properties)
    )
  }

  private header(e: StoredEntity) {
    const primary = this.def(e.type).primary
    const title = primary ? e.properties[primary] : undefined
    return {
      _self: `/api/v1/${this.def(e.type).plural}/${e.id}`,
      id: e.id,
      type: e.type,
      _title: typeof title === 'string' && title !== '' ? title : e.id,
    }
  }

  // The per-entity wire shape. A create response carries no `_versions`: the
  // server issues tokens on reads and on PATCH, not on create.
  private wire(e: StoredEntity, withVersions: boolean) {
    return {
      ...this.header(e),
      properties: structuredClone(e.properties),
      ...(e.content !== '' ? { content: e.content } : {}),
      _actions: ENTITY_ACTIONS,
      _fields: {},
      _relations: {},
      ...(withVersions ? { _versions: this.versions(e) } : {}),
      _redacted: [],
      _attachments: {},
      _copies: [],
      _conversions: [],
      _faces: [],
    }
  }

  // Replaces a stream's subscriptions. A key kept with the same path keeps
  // its baseline; a new one takes the answer it gets now, and sends nothing
  // for it.
  private putSubscriptions(conn: string, req: FakeRequest): FakeResponse {
    const stream = this.streams.get(conn)
    if (!stream)
      return problem(404, 'stream_not_found', 'No such event stream', `/api/v1${req.path}`)
    const subs =
      (req.body as { subscriptions?: { key: string; path: string }[] } | undefined)
        ?.subscriptions ?? []
    if (subs.length > LIVE_MAX_PER_CONN)
      return notImplemented('PUT', `${req.path} with ${subs.length} subscriptions`)
    const next = new Map<string, LiveSubscription>()
    for (const sub of subs) {
      if (!LIVE_KEY.test(sub.key) || next.has(sub.key) || !LIVE_PATH.test(sub.path)) {
        return notImplemented('PUT', `${req.path} for ${sub.key} ${sub.path}`)
      }
      const old = stream.subs.get(sub.key)
      next.set(
        sub.key,
        old?.path === sub.path ? old : { path: sub.path, baseline: this.replay(sub.path) }
      )
    }
    stream.subs = next
    return { status: 204, data: '' }
  }

  // A subscribed GET's answer, in the form a round compares.
  private replay(path: string): string {
    const res = this.handle({ method: 'GET', path: path.slice('/api/v1'.length), query: {} })
    return JSON.stringify([res.status, res.data])
  }

  private emit(type: string) {
    for (const s of this.streams.values()) s.listener({ event: 'entity:changed', data: { type } })
  }
}

function key(type: string, id: string): string {
  return `${type}\x00${id}`
}

// Applies Go's decoding of a JSON request body to a property value: lone
// surrogates become U+FFFD, recursively.
function decodeValue(v: unknown): unknown {
  if (typeof v === 'string') return goDecodeString(v)
  if (Array.isArray(v)) return v.map(decodeValue)
  if (v !== null && typeof v === 'object') {
    return Object.fromEntries(
      Object.entries(v).map(([k, x]) => [goDecodeString(k), decodeValue(x)])
    )
  }
  return v
}

// The store refuses property text with a NUL (storeutil.ValidateProperties,
// BUG-X7ICNM), after the precondition check. A body may hold one. With NUL in
// several properties the server names whichever its map iteration meets
// first, so the contract test generates it in one property only.
function refuseNUL(props: Record<string, unknown>, instance: string): FakeResponse | undefined {
  const hasNUL = (v: unknown): boolean =>
    typeof v === 'string' ? v.includes('\0') : Array.isArray(v) ? v.some(hasNUL) : false
  for (const [k, v] of Object.entries(props)) {
    const what = k.includes('\0')
      ? `property key ${JSON.stringify(k)}`
      : hasNUL(v)
        ? `property ${JSON.stringify(k)}`
        : ''
    if (what) {
      return problem(
        422,
        'validation_failed',
        'Validation failed',
        instance,
        `write entity: store: ${what}: contains NUL`
      )
    }
  }
  return undefined
}

function isEmptyValue(v: unknown): boolean {
  return v === undefined || v === null || v === '' || (Array.isArray(v) && v.length === 0)
}

function ok(data: unknown): FakeResponse {
  return { status: 200, data: structuredClone(data) }
}

function problem(
  status: number,
  code: string,
  title: string,
  instance: string,
  detail?: string
): FakeResponse {
  return {
    status,
    data: {
      type: `https://rela.dev/errors/${code}`,
      title,
      status,
      ...(detail !== undefined ? { detail } : {}),
      instance,
    },
  }
}

function entityNotFound(plural: string, id: string): FakeResponse {
  return problem(404, 'not_found', 'Entity not found', `/api/v1/${plural}/${id}`)
}

function notImplemented(method: string, route: string): FakeResponse {
  return {
    status: 501,
    data: {
      type: 'fake-api/not-implemented',
      title: `The fake API does not implement ${method} ${route}`,
      status: 501,
    },
  }
}
