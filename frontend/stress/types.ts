// Shared type definitions for the stress runner.
//
// Kept in one file because the harness is small and these types are used
// from every other file. If this grows past ~150 lines, split per concern.

import type { BrowserContext, Page } from '@playwright/test'

/**
 * RunContext is the per-user state passed to every workload operation.
 *
 * `page` is a real Chromium page bound to the test backend (via the
 * existing API routing fixture pattern). `api` is a thin REST helper for
 * sanity checks and seeding. `project` exposes file-system operations on
 * the *server's* isolated /tmp project so a workload can fire the watcher.
 */
export interface RunContext {
  userId: number
  page: Page
  context: BrowserContext
  api: ApiClient
  project: ProjectFs
  rand: Rng
  /** Server's base URL on the loopback. */
  baseUrl: string
  /** pprof base URL. Empty if pprof not enabled for this run. */
  pprofUrl: string
}

/**
 * A workload operation. Operations are picked by weight from a Workload
 * and dispatched concurrently across users.
 *
 * Each op should be self-contained: it picks its own targets, performs
 * one logical action, and reports its own latency via the runner. Errors
 * are caught by the runner and recorded as op failures.
 */
export interface Operation {
  name: string
  weight: number
  /** Skip this op if it cannot run right now (no targets, etc.). */
  guard?: (ctx: RunContext) => boolean | Promise<boolean>
  fn: (ctx: RunContext) => Promise<void>
}

export interface Scenario {
  name: string
  description: string
  /** How many concurrent users (browser contexts) to run. */
  users: number
  /** Workload duration. Parsed from CLI as `30s` / `2m`. */
  defaultDurationMs: number
  /** Operations and their weights. */
  operations: Operation[]
  /** Optional one-time setup before users start. */
  setup?: (ctx: SetupContext) => Promise<void>
  /** Invariant thresholds, scenario-specific. */
  thresholds: Thresholds
}

export interface SetupContext {
  api: ApiClient
  project: ProjectFs
  baseUrl: string
}

export interface Thresholds {
  /** Hard cap for any HTTP 5xx during the run. Default 0. */
  maxServer5xx: number
  /** p99 milliseconds for /api/v1/_schema (the BUG-FMS1 canary). */
  schemaP99Ms: number
  /** p99 milliseconds for entity create/update. */
  mutationP99Ms: number
  /** Maximum allowed browser console errors. */
  maxConsoleErrors: number
  /** Maximum goroutine growth from baseline (only checked if pprof enabled). */
  maxGoroutineGrowth: number
}

/** Minimal REST client. Uses node fetch — no axios coupling. */
export interface ApiClient {
  get<T = unknown>(path: string): Promise<T>
  post<T = unknown>(path: string, body: unknown): Promise<T>
  patch<T = unknown>(path: string, body: unknown): Promise<T>
  delete(path: string): Promise<void>
  /** Latency-tracked variant — used by the runner's canary. */
  timed(method: string, path: string, body?: unknown): Promise<{ status: number; ms: number }>
}

/** Filesystem operations on the server's isolated project. */
export interface ProjectFs {
  /** Server-side absolute path to the project root. */
  root: string
  /** Touch an existing entity file to fire the watcher. Returns the path touched. */
  touchRandomEntity(rand: Rng): string
  /** Append a no-op comment to an existing entity (forces a real Write event). */
  rewriteRandomEntity(rand: Rng): string
}

/** Deterministic seeded RNG. */
export interface Rng {
  /** [0, 1) */
  next(): number
  /** integer in [0, max) */
  int(max: number): number
  /** Pick one element. Throws if empty. */
  pick<T>(items: readonly T[]): T
  /** Weighted pick from operations. */
  pickWeighted(items: readonly { weight: number }[]): number
}

/** A single recorded operation outcome. */
export interface OpResult {
  user: number
  op: string
  startMs: number
  durationMs: number
  ok: boolean
  /** HTTP status if the op caused a recorded HTTP request. */
  httpStatus?: number
  /** Error message on failure. */
  error?: string
}

export interface RunReport {
  scenario: string
  durationMs: number
  users: number
  totalOps: number
  failedOps: number
  opStats: Record<string, OpStats>
  /** Sampled latency for the schema canary, the BUG-FMS1 guard. */
  schemaCanary: LatencyStats
  /** Initial SPA navigation per user (first-paint to first-row-visible). */
  initialNav: { samplesMs: number[]; failures: number }
  /** All breached invariants. Empty if the run passed. */
  breaches: string[]
  /** Server 5xx count seen by the canary or any user-driven request. */
  server5xxCount: number
  consoleErrorCount: number
  /** Browser-quirk errors filtered out of the strict invariant. */
  consoleErrorsBenignCount: number
  baselineGoroutines: number
  finalGoroutines: number
}

export interface OpStats {
  count: number
  failures: number
  p50Ms: number
  p95Ms: number
  p99Ms: number
  maxMs: number
}

export interface LatencyStats {
  count: number
  p50Ms: number
  p95Ms: number
  p99Ms: number
  maxMs: number
  /** Slowest sample's full timing. */
  slowestMs: number
  /** Number of samples that exceeded the threshold. */
  breachCount: number
}
