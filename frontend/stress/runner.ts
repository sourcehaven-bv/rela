// The runner: drives N concurrent users through a scenario, runs a
// latency canary in parallel, captures pprof on threshold breach, and
// produces a structured report.
//
// Design: each user is an async loop that picks a weighted operation,
// runs it, records the result, and continues until the deadline.
// The schema canary is a separate loop that fires at a fixed interval —
// it's intentionally simple so the latency we record is not skewed by
// our own queueing.

import { chromium, firefox, webkit, Browser, BrowserContext, Page } from '@playwright/test'

export type BrowserName = 'chromium' | 'firefox' | 'webkit'
import * as fs from 'node:fs'
import * as path from 'node:path'

import type {
  RunContext,
  Scenario,
  RunReport,
  OpResult,
  OpStats,
  LatencyStats,
} from './types.js'
import { makeApi } from './api.js'
import { makeProjectFs } from './projectFs.js'
import { makeRng } from './rng.js'
import { startServer, ServerHandle } from './serverProcess.js'

export interface RunOptions {
  scenario: Scenario
  durationMs: number
  users: number
  sourceProject: string
  reportDir: string
  seed: number
  browser: BrowserName
}

export async function run(opts: RunOptions): Promise<RunReport> {
  fs.mkdirSync(opts.reportDir, { recursive: true })
  const startedAt = Date.now()

  // Boot the server first so we have pprof + a baseline goroutine count
  // before any browser starts. The pprof URL is empty if pprof is off,
  // but here we always enable it for the stress runner.
  const server = await startServer({
    sourceProject: opts.sourceProject,
    reportDir: opts.reportDir,
    enablePprof: true,
  })
  log(opts.reportDir, `server up at ${server.baseUrl} (project=${server.projectRoot})`)

  let report: RunReport
  try {
    const baselineGoroutines = await countGoroutines(server.pprofUrl)
    log(opts.reportDir, `baseline goroutines: ${baselineGoroutines}`)

    // One browser, N independent BrowserContexts. Contexts share the
    // browser process but have isolated cookies and storage, which is
    // the right model for "N users on the same app".
    const launcher =
      opts.browser === 'firefox' ? firefox : opts.browser === 'webkit' ? webkit : chromium
    log(opts.reportDir, `launching ${opts.browser}`)
    const browser = await launcher.launch({ headless: true })
    try {
      report = await runScenario(opts, server, browser, baselineGoroutines, startedAt)
    } finally {
      await browser.close()
    }
  } finally {
    await server.stop()
  }

  fs.writeFileSync(
    path.join(opts.reportDir, 'report.json'),
    JSON.stringify(report, null, 2),
  )
  return report
}

async function runScenario(
  opts: RunOptions,
  server: ServerHandle,
  browser: Browser,
  baselineGoroutines: number,
  startedAt: number,
): Promise<RunReport> {
  const { scenario } = opts
  const api = makeApi(server.baseUrl)
  const projectFs = makeProjectFs(server.projectRoot)

  // One-time scenario setup (e.g. seed entities).
  if (scenario.setup) {
    log(opts.reportDir, `running setup for scenario ${scenario.name}`)
    await scenario.setup({ api, project: projectFs, baseUrl: server.baseUrl })
  }

  // Build per-user contexts.
  const contexts: BrowserContext[] = []
  const pages: Page[] = []
  let consoleErrors = 0
  // Firefox aborts in-flight requests during navigation and surfaces them
  // as console errors. These are not bugs in rela — they are a Firefox
  // quirk vs Chromium and would otherwise drown out genuine errors.
  // We count them separately so we still see the rate.
  let consoleErrorsBenign = 0
  const benignPatterns = [
    /connection to .* was interrupted while the page was loading/i,
    /the operation was aborted/i,
    /networkerror when attempting to fetch/i,
    /load failed/i,
  ]
  function classify(text: string): 'benign' | 'real' {
    for (const p of benignPatterns) if (p.test(text)) return 'benign'
    return 'real'
  }
  for (let i = 0; i < opts.users; i++) {
    const ctx = await browser.newContext()
    const page = await ctx.newPage()
    page.on('console', (msg) => {
      if (msg.type() !== 'error') return
      const text = msg.text()
      if (classify(text) === 'benign') {
        consoleErrorsBenign++
        return
      }
      consoleErrors++
      log(opts.reportDir, `[user ${i}] console error: ${text}`)
    })
    page.on('pageerror', (err) => {
      if (classify(err.message) === 'benign') {
        consoleErrorsBenign++
        return
      }
      consoleErrors++
      log(opts.reportDir, `[user ${i}] page error: ${err.message}`)
    })
    contexts.push(ctx)
    pages.push(page)
  }

  const initialNavMs: number[] = []
  let initialNavFailures = 0
  // Pre-navigate every user to the SPA so the SSE EventSource is open
  // before the workload begins. Use `domcontentloaded` rather than
  // `networkidle` because the SSE connection at /api/v1/_events is
  // intentionally long-lived — `networkidle` would never fire.
  // We then wait for an entity row to appear as a soft signal that the
  // SPA's bootstrap chain (schema → config → list) actually completed.
  for (let i = 0; i < pages.length; i++) {
    const target = `${server.baseUrl}/v2/list/all_tickets`
    const navStart = performance.now()
    try {
      await pages[i]!.goto(target, { waitUntil: 'domcontentloaded', timeout: 15_000 })
      // Soft wait for at least one row to render. This is the real
      // "page is usable" signal. Failure here means the SPA bootstrap
      // chain stalled — exactly the BUG-FMS1 symptom.
      await pages[i]!
        .locator('.entity-row')
        .first()
        .waitFor({ state: 'visible', timeout: 15_000 })
      const elapsed = performance.now() - navStart
      initialNavMs.push(elapsed)
      log(opts.reportDir, `[user ${i}] initial nav OK in ${elapsed.toFixed(0)}ms`)
    } catch (err) {
      initialNavFailures++
      const elapsed = performance.now() - navStart
      log(
        opts.reportDir,
        `[user ${i}] initial nav FAILED after ${elapsed.toFixed(0)}ms: ${(err as Error).message}`,
      )
      // This is itself a strong BUG-FMS1 signal. Capture pprof now.
      if (server.pprofUrl) {
        log(opts.reportDir, '  capturing pprof on initial nav failure')
        await capturePprof(server.pprofUrl, opts.reportDir)
      }
    }
  }
  log(
    opts.reportDir,
    `${pages.length} users navigated to SPA (failures=${initialNavFailures})`,
  )

  // Schema canary: independent loop that fires every 200ms regardless
  // of what the workload is doing. This is what guards BUG-FMS1.
  const canaryResults: number[] = []
  let server5xxCount = 0
  const canaryStop = { done: false }
  const canaryLoop = (async () => {
    while (!canaryStop.done) {
      const t = await api.timed('GET', '_schema').catch((err) => ({
        status: 0,
        ms: -1,
        error: (err as Error).message,
      }))
      if ('error' in t) {
        log(opts.reportDir, `canary fetch failed: ${t.error}`)
      } else {
        if (t.status >= 500) server5xxCount++
        canaryResults.push(t.ms)
      }
      await sleep(200)
    }
  })()

  // User loops: each user picks ops by weight until the deadline.
  const deadline = Date.now() + opts.durationMs
  const opResults: OpResult[] = []
  // Per-op slow-call threshold. When exceeded we capture pprof
  // immediately so we have evidence even if the overall run passes.
  // Capped to avoid flooding the report dir on a long soak.
  let pprofCapturesRemaining = 10
  const slowOpThresholdMs = 1000

  // Periodic progress + canary checkpoint so a long soak shows life and
  // we can spot the moment things turn bad. Every 30s.
  const progressTimer = setInterval(() => {
    const elapsed = Math.floor((Date.now() - (deadline - opts.durationMs)) / 1000)
    const remaining = Math.max(0, Math.floor((deadline - Date.now()) / 1000))
    const sortedCanary = [...canaryResults].sort((a, b) => a - b)
    const cp50 = percentile(sortedCanary, 50)
    const cp99 = percentile(sortedCanary, 99)
    const failed = opResults.filter((r) => !r.ok).length
    log(
      opts.reportDir,
      `[progress t+${elapsed}s, ${remaining}s left] ops=${opResults.length} ` +
        `failed=${failed} canary{n=${sortedCanary.length} ` +
        `p50=${cp50.toFixed(1)}ms p99=${cp99.toFixed(1)}ms ` +
        `max=${(sortedCanary[sortedCanary.length - 1] ?? 0).toFixed(1)}ms ` +
        `5xx=${server5xxCount}}`,
    )
  }, 30_000)
  const userLoops = pages.map((page, userIdx) => async () => {
    const ctx: RunContext = {
      userId: userIdx,
      page,
      context: contexts[userIdx]!,
      api,
      project: projectFs,
      rand: makeRng(opts.seed + userIdx),
      baseUrl: server.baseUrl,
      pprofUrl: server.pprofUrl,
    }
    while (Date.now() < deadline) {
      // Filter operations whose guard fails right now (e.g. delete with
      // no entities to delete). Re-evaluated each iteration because
      // entity counts change as the workload runs.
      const eligible: typeof scenario.operations = []
      for (const op of scenario.operations) {
        if (!op.guard) {
          eligible.push(op)
          continue
        }
        try {
          if (await op.guard(ctx)) eligible.push(op)
        } catch {
          /* skip */
        }
      }
      if (eligible.length === 0) {
        await sleep(50)
        continue
      }
      const op = eligible[ctx.rand.pickWeighted(eligible)]!
      const opStart = performance.now()
      let ok = true
      let error: string | undefined
      try {
        await op.fn(ctx)
      } catch (e) {
        ok = false
        error = (e as Error).message
        log(opts.reportDir, `[user ${userIdx}] op ${op.name} failed: ${error}`)
      }
      const durationMs = performance.now() - opStart
      opResults.push({
        user: userIdx,
        op: op.name,
        startMs: opStart,
        durationMs,
        ok,
        error,
      })
      if (durationMs > slowOpThresholdMs && pprofCapturesRemaining > 0 && server.pprofUrl) {
        pprofCapturesRemaining--
        log(
          opts.reportDir,
          `[user ${userIdx}] SLOW op ${op.name} (${durationMs.toFixed(0)}ms) — capturing pprof`,
        )
        // Use a unique filename so we don't clobber prior captures.
        const tag = `slow-${op.name}-u${userIdx}-${Date.now()}`
        await capturePprofTagged(server.pprofUrl, opts.reportDir, tag)
      }
    }
  })

  await Promise.all(userLoops.map((fn) => fn()))
  canaryStop.done = true
  await canaryLoop
  clearInterval(progressTimer)

  const finalGoroutines = await countGoroutines(server.pprofUrl)

  // Cleanup browser contexts.
  for (const ctx of contexts) await ctx.close().catch(() => undefined)

  const schemaStats = computeLatency(canaryResults, scenario.thresholds.schemaP99Ms)
  const opStats = aggregateOps(opResults)

  const breaches: string[] = []
  if (initialNavFailures > 0) {
    breaches.push(`initial nav failures: ${initialNavFailures}/${pages.length}`)
  }
  if (initialNavMs.length > 0) {
    const navStats = computeLatency(initialNavMs, scenario.thresholds.schemaP99Ms * 10)
    if (navStats.maxMs > 5000) {
      breaches.push(
        `initial nav max ${navStats.maxMs.toFixed(0)}ms > 5000ms`,
      )
    }
  }
  if (server5xxCount > scenario.thresholds.maxServer5xx) {
    breaches.push(`server 5xx count ${server5xxCount} > ${scenario.thresholds.maxServer5xx}`)
  }
  if (schemaStats.p99Ms > scenario.thresholds.schemaP99Ms) {
    breaches.push(
      `schema canary p99 ${schemaStats.p99Ms.toFixed(1)}ms > ${scenario.thresholds.schemaP99Ms}ms`,
    )
  }
  if (consoleErrors > scenario.thresholds.maxConsoleErrors) {
    breaches.push(
      `console errors ${consoleErrors} > ${scenario.thresholds.maxConsoleErrors}`,
    )
  }
  for (const op of ['create-entity', 'edit-entity']) {
    const s = opStats[op]
    if (s && s.p99Ms > scenario.thresholds.mutationP99Ms) {
      breaches.push(
        `${op} p99 ${s.p99Ms.toFixed(1)}ms > ${scenario.thresholds.mutationP99Ms}ms`,
      )
    }
  }
  const goroutineGrowth = finalGoroutines - baselineGoroutines
  if (goroutineGrowth > scenario.thresholds.maxGoroutineGrowth) {
    breaches.push(
      `goroutines grew by ${goroutineGrowth} > ${scenario.thresholds.maxGoroutineGrowth}`,
    )
  }

  // On any breach, dump goroutines and a heap profile for post-mortem.
  if (breaches.length > 0 && server.pprofUrl) {
    log(opts.reportDir, `BREACHED — capturing pprof state`)
    await capturePprof(server.pprofUrl, opts.reportDir)
  }

  return {
    scenario: scenario.name,
    durationMs: Date.now() - startedAt,
    users: opts.users,
    totalOps: opResults.length,
    failedOps: opResults.filter((r) => !r.ok).length,
    opStats,
    schemaCanary: schemaStats,
    initialNav: { samplesMs: initialNavMs, failures: initialNavFailures },
    breaches,
    server5xxCount,
    consoleErrorCount: consoleErrors,
    consoleErrorsBenignCount: consoleErrorsBenign,
    baselineGoroutines,
    finalGoroutines,
  }
}

function aggregateOps(results: OpResult[]): Record<string, OpStats> {
  const byOp = new Map<string, number[]>()
  const failuresByOp = new Map<string, number>()
  for (const r of results) {
    if (!byOp.has(r.op)) byOp.set(r.op, [])
    byOp.get(r.op)!.push(r.durationMs)
    if (!r.ok) failuresByOp.set(r.op, (failuresByOp.get(r.op) ?? 0) + 1)
  }
  const out: Record<string, OpStats> = {}
  for (const [op, samples] of byOp) {
    samples.sort((a, b) => a - b)
    out[op] = {
      count: samples.length,
      failures: failuresByOp.get(op) ?? 0,
      p50Ms: percentile(samples, 50),
      p95Ms: percentile(samples, 95),
      p99Ms: percentile(samples, 99),
      maxMs: samples[samples.length - 1] ?? 0,
    }
  }
  return out
}

function computeLatency(samples: number[], threshold: number): LatencyStats {
  const sorted = [...samples].sort((a, b) => a - b)
  let breachCount = 0
  for (const s of samples) if (s > threshold) breachCount++
  return {
    count: sorted.length,
    p50Ms: percentile(sorted, 50),
    p95Ms: percentile(sorted, 95),
    p99Ms: percentile(sorted, 99),
    maxMs: sorted[sorted.length - 1] ?? 0,
    slowestMs: sorted[sorted.length - 1] ?? 0,
    breachCount,
  }
}

function percentile(sortedAscending: number[], p: number): number {
  if (sortedAscending.length === 0) return 0
  const idx = Math.min(
    sortedAscending.length - 1,
    Math.floor((p / 100) * sortedAscending.length),
  )
  return sortedAscending[idx]!
}

async function countGoroutines(pprofUrl: string): Promise<number> {
  if (!pprofUrl) return 0
  try {
    const r = await fetch(`${pprofUrl}/debug/pprof/goroutine?debug=1`)
    const text = await r.text()
    const match = text.match(/goroutine profile: total (\d+)/)
    return match ? parseInt(match[1]!, 10) : 0
  } catch {
    return 0
  }
}

async function capturePprof(pprofUrl: string, dir: string): Promise<void> {
  return capturePprofTagged(pprofUrl, dir, 'breach')
}

async function capturePprofTagged(
  pprofUrl: string,
  dir: string,
  tag: string,
): Promise<void> {
  const profiles = [
    { name: `${tag}.goroutine.txt`, path: '/debug/pprof/goroutine?debug=2' },
    { name: `${tag}.goroutine-summary.txt`, path: '/debug/pprof/goroutine?debug=1' },
    { name: `${tag}.heap.txt`, path: '/debug/pprof/heap?debug=1' },
    { name: `${tag}.mutex.txt`, path: '/debug/pprof/mutex?debug=1' },
    { name: `${tag}.block.txt`, path: '/debug/pprof/block?debug=1' },
  ]
  for (const p of profiles) {
    try {
      const r = await fetch(`${pprofUrl}${p.path}`)
      fs.writeFileSync(path.join(dir, p.name), await r.text())
    } catch (err) {
      fs.writeFileSync(
        path.join(dir, `${p.name}.error`),
        `failed to fetch ${p.path}: ${(err as Error).message}\n`,
      )
    }
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms))
}

function log(dir: string, msg: string): void {
  const line = `[${new Date().toISOString()}] ${msg}\n`
  process.stderr.write(line)
  try {
    fs.appendFileSync(path.join(dir, 'runner.log'), line)
  } catch {
    /* directory may not exist yet on first call */
  }
}
