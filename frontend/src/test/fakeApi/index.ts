// A verified fake of the data-entry API. See server.ts for what it covers and
// frontend/contract/ for the test that keeps it equal to the real server.

import snapshotJSON from '../../../contract/fixture/snapshot.json'

import type { FakeSnapshot } from './server'

export {
  FakeServer,
  type FakeEvent,
  type FakeStream,
  type FakeRequest,
  type FakeResponse,
  type FakeServerOptions,
  type FakeSnapshot,
} from './server'
export {
  fakeAdapter,
  type FakeAdapter,
  type FakeAdapterOptions,
  type RecordedRequest,
} from './adapter'
export { fakeEventSource } from './eventSource'
export { Gate } from './gate'
export { INERT_ROUTES } from './inert'

/** The schema and config of the contract fixture, as the real server serves them. */
export const fixtureSnapshot = snapshotJSON as unknown as FakeSnapshot
