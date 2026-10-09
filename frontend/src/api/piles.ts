import { api } from './client'
import { apiUrl } from './base'
import { ApiError } from './errors'

/**
 * Piles (TKT-K3RJLH): a per-user collection of entity addresses, gathered from
 * lists, search results and entity pages. A pile belongs to the request
 * principal; another user's pile answers exactly like one that does not exist.
 *
 * Every read honours `?world=`: the counts and items are the READABLE ones in
 * that world, so the panel, the sidebar count, the scope and the export agree.
 */

/** A pile as `GET /_piles` lists it: no items, a count of readable ones. */
export interface PileSummary {
  /** `PIL-XXXXXXXX`, minted by the server. */
  id: string
  name: string
  /** One of the names in `PileList.icons`. */
  icon: string
  /** Readable items only, in the request's world. */
  count: number
  created: string
  updated: string
}

/** One entity on a pile, as the owner may read it. */
export interface PileItem {
  /** Bare entity id. */
  id: string
  /** '' for the implicit face. */
  face: string
  /** `ID` or `ID@face`: what links, removal and actions address. */
  address: string
  type: string
  title: string
}

export interface Pile extends PileSummary {
  /** Newest first, readable only. */
  items: PileItem[]
}

/** `GET /_piles`: the user's piles, oldest first, and the icons a pile may use. */
export interface PileList {
  piles: PileSummary[]
  icons: string[]
}

export interface CreatePileInput {
  name: string
  icon?: string
  /** Addresses to put on the new pile. */
  items?: string[]
}

export interface UpdatePileInput {
  name?: string
  icon?: string
}

/** The most addresses one request may carry; the server refuses more with a 400. */
export const PILE_REQUEST_MAX_ITEMS = 500

/** The longest pile name the server accepts, in characters after trimming. */
export const PILE_NAME_MAX = 80

function withWorld(path: string, world?: string): string {
  return world ? `${path}?${new URLSearchParams({ world }).toString()}` : path
}

function pilePath(id: string): string {
  return `/_piles/${encodeURIComponent(id)}`
}

export async function listPiles(world?: string, signal?: AbortSignal): Promise<PileList> {
  return api.get<PileList>('/_piles', world ? { world } : undefined, signal)
}

export async function getPile(id: string, world?: string, signal?: AbortSignal): Promise<Pile> {
  return api.get<Pile>(pilePath(id), world ? { world } : undefined, signal)
}

export async function createPile(input: CreatePileInput, world?: string): Promise<Pile> {
  return api.post<Pile>(withWorld('/_piles', world), input)
}

export async function updatePile(
  id: string,
  input: UpdatePileInput,
  world?: string
): Promise<Pile> {
  return api.patch<Pile>(withWorld(pilePath(id), world), input)
}

export async function deletePile(id: string): Promise<void> {
  await api.delete(pilePath(id))
}

/** Adds addresses to a pile. Already-present items are not counted in `added`. */
export async function addPileItems(
  id: string,
  items: string[],
  world?: string
): Promise<{ added: number }> {
  return api.post<{ added: number }>(withWorld(`${pilePath(id)}/items`, world), { items })
}

/**
 * Removes addresses from a pile. The server answers 204 whatever was stored,
 * so the response says nothing about which addresses were on it.
 */
export async function removePileItems(id: string, items: string[], world?: string): Promise<void> {
  await api.post<void>(withWorld(`${pilePath(id)}/items/_remove`, world), { items })
}

/** The download URL for a pile exported through a registered transform. */
export function pileExportUrl(id: string, transform: string, world?: string): string {
  const q = new URLSearchParams({ transform })
  if (world) q.set('world', world)
  return apiUrl(`/api/v1${pilePath(id)}/_export?${q.toString()}`)
}

/**
 * The machine-readable code of a failed piles request (`pile_name_taken`,
 * `item_not_found`, …), read off the problem document's `type` URL.
 *
 * Nil: never returned; '' when the error carries no code.
 */
export function pileErrorCode(err: unknown): string {
  if (!(err instanceof ApiError)) return ''
  const type = err.problem?.type ?? ''
  const slash = type.lastIndexOf('/')
  if (slash >= 0) return type.slice(slash + 1)
  const raw = (err.problem as { error?: unknown } | undefined)?.error
  return typeof raw === 'string' ? raw : type
}
