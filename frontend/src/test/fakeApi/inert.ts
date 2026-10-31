// Routes the editing surfaces call but these tests do not exercise.
//
// Each entry answers with the empty value the real server gives for the
// contract fixture (frontend/contract/fixture/project), which has no
// commands, comments, presence, transforms or actions. The contract test
// replays every entry against the real server, so an entry cannot claim an
// answer the server does not give. A route that is in neither this table nor
// the FakeServer answers 501.

import type { FakeResponse } from './server'

interface InertRoute {
  method: string
  /** Matches the path below /api/v1, without the query string. */
  path: RegExp
  /** A concrete path for the contract test to request. */
  example: string
  data: unknown
}

export const INERT_ROUTES: readonly InertRoute[] = [
  { method: 'GET', path: /^\/_dashboard$/, example: '/_dashboard', data: { cards: [] } },
  { method: 'GET', path: /^\/_transforms$/, example: '/_transforms', data: [] },
  {
    method: 'GET',
    path: /^\/_commands$/,
    example: '/_commands?page_type=entity&entity_type=note',
    data: [],
  },
  {
    method: 'GET',
    path: /^\/_schema\/channels$/,
    example: '/_schema/channels',
    data: { channel_types: [] },
  },
]

export function inertRoute(method: string, path: string): FakeResponse | undefined {
  const hit = INERT_ROUTES.find((r) => r.method === method && r.path.test(path))
  return hit && { status: 200, data: structuredClone(hit.data) }
}
