// relaBridge — host-side RPC dispatcher for custom "apps".
//
// A custom app runs in a sandboxed iframe with NO network access of its own
// (CSP default-src 'none'; and origin-"null" would be rejected by the server's
// same-origin check anyway). It talks to the host page over a MessageChannel.
// This dispatcher is the host end: it maps a CLOSED allow-list of method names
// to concrete existing api-client calls. There is deliberately NO arbitrary
// path/URL passthrough — an app can only invoke the operations listed here, and
// every call runs under the logged-in user's session, so an app can do nothing
// the user couldn't already do.
//
// To add a capability, add a method to BRIDGE_METHODS. Never add a generic
// "fetch this URL" method — that would defeat the closed-surface guarantee.

import {
  listEntities,
  getEntity,
  createEntity,
  updateEntity,
  deleteEntity,
  searchEntities,
  analyze,
  getTemplates,
  getEntityPosition,
  createRelation,
  updateRelationProperties,
  deleteRelation,
  type ScopeDescriptor,
  type EntityPatch,
} from '@/api/entities'
import { getSchema, getConfig } from '@/api/schema'
import { runAction } from '@/api/actions'
import type { CreateEntity, ListParams } from '@/types'

/** A single RPC request from an app, sent over the MessageChannel port. */
export interface BridgeRequest {
  id: number
  method: string
  params?: unknown
}

/** The reply the host posts back for each request. */
export interface BridgeResponse {
  id: number
  ok: boolean
  result?: unknown
  error?: { code: string; message: string }
}

// Each handler receives the raw params object the app passed and returns a
// promise. Param shapes are validated minimally here; the backend re-validates
// and re-authorizes everything.
type Handler = (params: Record<string, unknown>) => Promise<unknown>

function str(params: Record<string, unknown>, key: string): string {
  const v = params[key]
  if (typeof v !== 'string' || v === '') {
    throw new BridgeError('invalid_params', `"${key}" must be a non-empty string`)
  }
  return v
}

function optStr(params: Record<string, unknown>, key: string): string | undefined {
  const v = params[key]
  if (v === undefined || v === null) return undefined
  if (typeof v !== 'string') throw new BridgeError('invalid_params', `"${key}" must be a string`)
  return v
}

export class BridgeError extends Error {
  constructor(
    public code: string,
    message: string,
  ) {
    super(message)
    this.name = 'BridgeError'
  }
}

// The closed allow-list. Keys are the only method names an app may call.
export const BRIDGE_METHODS: Record<string, Handler> = {
  // --- reads ---
  schema: () => getSchema(),
  config: () => getConfig(),
  list: (p) => listEntities(str(p, 'type'), p.params as ListParams | undefined),
  get: (p) =>
    getEntity(str(p, 'type'), str(p, 'id'), p.params as { include?: string; fields?: string } | undefined),
  search: (p) => searchEntities(str(p, 'query'), optStr(p, 'type')),
  analyze: () => analyze(),
  templates: (p) => getTemplates(str(p, 'type')),
  position: (p) => getEntityPosition(str(p, 'id'), p.scope as ScopeDescriptor),

  // --- entity writes (re-authorized + audited by the backend) ---
  create: (p) => createEntity(str(p, 'type'), p.entity as CreateEntity),
  update: (p) => updateEntity(str(p, 'type'), str(p, 'id'), p.patch as EntityPatch, optStr(p, 'etag')),
  delete: (p) => deleteEntity(str(p, 'type'), str(p, 'id')),

  // --- relation writes ---
  relationCreate: (p) =>
    createRelation(
      str(p, 'type'),
      str(p, 'id'),
      str(p, 'relation'),
      str(p, 'targetId'),
      p.meta as Record<string, unknown> | undefined,
      optStr(p, 'direction'),
    ),
  relationUpdate: (p) =>
    updateRelationProperties(
      str(p, 'type'),
      str(p, 'id'),
      str(p, 'relation'),
      str(p, 'targetId'),
      (p.meta as Record<string, unknown>) ?? {},
      optStr(p, 'direction'),
    ),
  relationDelete: (p) =>
    deleteRelation(str(p, 'type'), str(p, 'id'), str(p, 'relation'), str(p, 'targetId'), optStr(p, 'direction')),

  // --- registered server-side Lua actions ---
  action: (p) => runAction(str(p, 'actionId'), optStr(p, 'entityId'), optStr(p, 'entityType')),
}

/**
 * Dispatch a single bridge request to its handler. Unknown methods are rejected
 * with a structured error and NO network call — this is the closed-allow-list
 * guarantee. Param/handler errors are normalized to a {code, message} pair so
 * the app never sees raw internals.
 */
export async function dispatchBridgeRequest(req: BridgeRequest): Promise<BridgeResponse> {
  const handler = BRIDGE_METHODS[req.method]
  if (!handler) {
    return {
      id: req.id,
      ok: false,
      error: { code: 'unknown_method', message: `unknown bridge method "${req.method}"` },
    }
  }
  const params = (req.params && typeof req.params === 'object' ? req.params : {}) as Record<string, unknown>
  try {
    const result = await handler(params)
    return { id: req.id, ok: true, result: result ?? null }
  } catch (err) {
    if (err instanceof BridgeError) {
      return { id: req.id, ok: false, error: { code: err.code, message: err.message } }
    }
    const message = err instanceof Error ? err.message : 'request failed'
    return { id: req.id, ok: false, error: { code: 'request_failed', message } }
  }
}
