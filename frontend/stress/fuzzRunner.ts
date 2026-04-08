// Property-based fuzzer for the data-entry SPA.
//
// Generates a sequence of UI actions, replays it in one fresh Firefox
// BrowserContext, then runs a liveness oracle: a fresh navigation must
// complete within a bounded time and produce a usable list view, with no
// non-benign browser console errors. fast-check shrinks any failing
// sequence to a minimal counter-example.
//
// This runs intentionally with one user at a time and a fresh context per
// iteration so we sidestep the multi-user runner wedge tracked in BUG-K570.

import { firefox, chromium, Browser, BrowserContext, Page } from '@playwright/test'
import * as fc from 'fast-check'
import * as fs from 'node:fs'
import * as path from 'node:path'

import { startServer, ServerHandle } from './serverProcess.js'

export type BrowserName = 'firefox' | 'chromium'

/**
 * Action vocabulary the fuzzer can generate. Kept deliberately small so
 * shrinking finds minimal counter-examples quickly. Notable exclusions:
 *
 * - No `delete` action: would open a confirm dialog and stick.
 * - No multi-tab actions: doubles the search space; add later if simple
 *   sequences fail to find anything.
 *
 * `create-entity` and `edit-entity` mutate the project state: the
 * project is a /tmp copy so this is safe for the run, but the user
 * should not set `--source-project` to anything non-disposable.
 */
type Action =
  | { kind: 'goto'; list: string }
  | { kind: 'click-row'; index: number }
  | { kind: 'reload' }
  | { kind: 'back' }
  | { kind: 'wait'; ms: number }
  | { kind: 'touch-file' }
  | { kind: 'create-entity'; entityType: EntityTypeName }
  | { kind: 'edit-entity'; rowIndex: number }

type EntityTypeName = 'ticket' | 'bug' | 'feature' | 'idea'

const ENTITY_TYPES: EntityTypeName[] = ['ticket', 'bug', 'feature', 'idea']

// Map entity type → its create form ID and list ID. The seed project's
// data-entry.yaml exposes these as create_${type}.
const CREATE_FORM_BY_TYPE: Record<EntityTypeName, string> = {
  ticket: 'create_ticket',
  bug: 'create_bug',
  feature: 'create_feature',
  idea: 'create_idea',
}

export interface FuzzOptions {
  sourceProject: string
  reportDir: string
  browser: BrowserName
  /** fast-check `numRuns` — sequences to try before giving up. */
  numRuns: number
  /** Max actions per sequence. fast-check picks 1..maxActions per example. */
  maxActions: number
  /** Hard wall for the liveness oracle's final navigation. */
  oracleNavTimeoutMs: number
  /**
   * Per-action latency budget. Every goto/reload/back/click that takes
   * longer than this fails the sequence. This is the "long long time"
   * guard — the user reported rapid-navigation hangs that lasted many
   * seconds but eventually loaded, which the console-error oracle
   * doesn't catch. Default: 3000 ms (generous enough to avoid flakes on
   * GC/first-paint, tight enough to catch user-visible hangs).
   */
  actionLatencyBudgetMs: number
  /**
   * When true, `create-entity` and `edit-entity` actually submit the
   * form and write to the /tmp project copy. When false, they fill the
   * form and navigate away without submitting — exercising the
   * form-load path but leaving the project state unchanged.
   *
   * Default: false. Mutations expose a different class of bugs
   * (state-dependent edge cases in SSE cascades, metamodel
   * automations, slug renames) that build up across iterations and
   * make the fuzzer harder to make deterministic. Enable with
   * `--mutate` when investigating those classes specifically.
   */
  mutate: boolean
  /** RNG seed for reproducibility. */
  seed: number
}

interface FuzzContext {
  page: Page
  context: BrowserContext
  baseUrl: string
  projectRoot: string
  consoleErrors: string[]
  /** Toggled on by the runner so the oracle can read errors at end. */
  recordErrors: boolean
  /** Per-action latency budget in ms. */
  latencyBudgetMs: number
  /**
   * Pushed to by `runAction` every time a measurable UI action
   * (goto/reload/back/click) exceeds the latency budget. Non-empty at
   * sequence end means the liveness oracle has failed.
   */
  slowActions: Array<{ action: string; ms: number }>
  /** Whether create/edit actions should actually submit. */
  mutate: boolean
}

const LISTS = ['all_tickets', 'all_bugs', 'all_features', 'all_ideas']

// Benign-error filter. Firefox is noisy about in-flight requests being
// cancelled during navigation, and browser-internal warnings from the
// EventSource reconnect logic. None of these are fixable at the
// application level — they are browser-native warnings for normal
// lifecycle events.
const BENIGN_PATTERNS = [
  /connection to .* was interrupted while the page was loading/i,
  /the operation was aborted/i,
  /networkerror when attempting to fetch/i,
  /load failed/i,
  // Firefox-specific EventSource reconnect warning. The SPA closes
  // its EventSource on navigation and opens a new one; during the
  // window between close and open Firefox prints this warning if the
  // connection attempt fails. The SPA's own onerror handler already
  // reconnects, so this is cosmetic and unfixable at our layer.
  /can.t establish a connection to the server at/i,
  // Vite chunk-load failures during rapid navigation. main.ts and
  // router/index.ts auto-reload the page on these, so the user never
  // sees them — but Playwright still captures the exception that
  // triggered the reload. Swallow them here too.
  /couldn't resolve component/i,
  /failed to fetch dynamically imported module/i,
  /unable to preload css/i,
]

function isBenign(text: string): boolean {
  return BENIGN_PATTERNS.some((p) => p.test(text))
}

export async function runFuzz(opts: FuzzOptions): Promise<{
  passed: boolean
  numRunsExecuted: number
  failingSequence?: Action[]
  failingError?: string
}> {
  fs.mkdirSync(opts.reportDir, { recursive: true })

  const server = await startServer({
    sourceProject: opts.sourceProject,
    reportDir: opts.reportDir,
    enablePprof: true,
  })
  log(opts.reportDir, `server up at ${server.baseUrl} (project=${server.projectRoot})`)

  // Sanity check: verify the seed project actually has clickable rows on
  // the lists we're going to fuzz. Without this, the fuzzer would happily
  // flag every sequence as failing because the oracle can't find a row.
  await seedSanityCheck(server, opts.reportDir)

  // Firefox + Playwright appears to accumulate state across
  // BrowserContext create/destroy cycles. After ~8-10 iterations, the
  // initial `page.goto` for a fresh context can time out even though
  // the context is brand-new. Related to BUG-K570 (multi-user wedge).
  // Workaround: rebuild the browser process every BROWSER_RECYCLE_EVERY
  // iterations. The relaunch adds ~500 ms per cycle which is tolerable
  // given a sequence already takes ~1-3 seconds.
  const BROWSER_RECYCLE_EVERY = 8
  const launcher = opts.browser === 'firefox' ? firefox : chromium
  log(opts.reportDir, `launching ${opts.browser}`)
  // eslint-disable-next-line prefer-const
  let browser = await launcher.launch({ headless: true })

  let runsExecuted = 0
  let failingSequence: Action[] | undefined
  let failingError: string | undefined

  try {
    const result = await fc.check(
      fc.asyncProperty(arbActionSequence(opts.maxActions), async (actions) => {
        runsExecuted++
        if (runsExecuted % 10 === 0 || runsExecuted < 10) {
          log(opts.reportDir, `[fuzz] run ${runsExecuted} actions=${actions.length}`)
        }
        if (runsExecuted > 0 && runsExecuted % BROWSER_RECYCLE_EVERY === 0) {
          log(opts.reportDir, `[fuzz] recycling browser at run ${runsExecuted}`)
          await browser.close().catch(() => undefined)
          browser = await launcher.launch({ headless: true })
        }
        return await runOneSequence(browser, server, actions, opts)
      }),
      {
        numRuns: opts.numRuns,
        seed: opts.seed,
        verbose: 1,
      },
    )

    if (result.failed) {
      failingSequence = result.counterexample?.[0] as Action[] | undefined
      failingError = formatFailureError(result)
      log(opts.reportDir, `[fuzz] FAILED after ${result.numRuns} runs, ${result.numShrinks} shrinks`)
      log(opts.reportDir, `[fuzz] minimal failing sequence:\n${JSON.stringify(failingSequence, null, 2)}`)
      log(opts.reportDir, `[fuzz] error: ${failingError}`)
    } else {
      log(opts.reportDir, `[fuzz] PASS — ${result.numRuns} sequences explored, no failures`)
    }
  } finally {
    await browser.close().catch(() => undefined)
    await server.stop()
  }

  return {
    passed: failingSequence === undefined,
    numRunsExecuted: runsExecuted,
    failingSequence,
    failingError,
  }
}

async function seedSanityCheck(server: ServerHandle, reportDir: string): Promise<void> {
  for (const list of LISTS) {
    const r = await fetch(`${server.baseUrl}/api/v1/${listToEntityType(list)}?per_page=1`, {
      headers: { Referer: `${server.baseUrl}/` },
    })
    if (!r.ok) {
      log(reportDir, `[fuzz] seed sanity: GET ${list} → ${r.status} (continuing)`)
      continue
    }
    const body = (await r.json()) as { data?: unknown[] }
    if (!body.data || body.data.length === 0) {
      throw new Error(
        `[fuzz] seed sanity check failed: list ${list} has no entities. ` +
          `The fuzzer needs at least one row to click on. ` +
          `Seed the source project before running the fuzzer.`,
      )
    }
  }
  log(reportDir, '[fuzz] seed sanity OK — all fuzzed lists have content')
}

function listToEntityType(list: string): string {
  // The /api/v1 routes use plural entity-type names, which happen to be
  // the same as the list IDs minus the "all_" prefix here. If new lists
  // get added with different entity types, extend this mapping.
  return list.replace(/^all_/, '')
}

function arbActionSequence(maxActions: number): fc.Arbitrary<Action[]> {
  const list = fc.constantFrom(...LISTS)
  // Index is 0..14 — entity-row count varies by list, but every list in
  // the seed project paginates at >=15. Out-of-range indices are handled
  // by the runner as no-ops, not failures.
  const rowIndex = fc.integer({ min: 0, max: 14 })
  // Wait amounts cover "right after" (10ms), "short delay" (100ms), and
  // "long enough for SSE / debounced rerender" (500-1500ms).
  const waitMs = fc.constantFrom(10, 50, 100, 250, 500, 1000, 1500)
  const entityType = fc.constantFrom(...ENTITY_TYPES)

  // Navigation and lifecycle actions are given higher weight than mutating
  // actions so most sequences stay in the "navigation churn" regime that
  // matched the user's reported symptom. Mutating actions are included
  // at ~10% weight each so they still surface regularly.
  //
  // fc.oneof takes arbitraries of the same union type; we wrap each with
  // the appropriate tag before combining.
  const action: fc.Arbitrary<Action> = fc.oneof(
    { arbitrary: list.map((l) => ({ kind: 'goto', list: l }) as Action), weight: 3 },
    { arbitrary: rowIndex.map((i) => ({ kind: 'click-row', index: i }) as Action), weight: 3 },
    { arbitrary: fc.constant({ kind: 'reload' } as Action), weight: 2 },
    { arbitrary: fc.constant({ kind: 'back' } as Action), weight: 2 },
    { arbitrary: waitMs.map((ms) => ({ kind: 'wait', ms }) as Action), weight: 1 },
    { arbitrary: fc.constant({ kind: 'touch-file' } as Action), weight: 1 },
    {
      arbitrary: entityType.map((t) => ({ kind: 'create-entity', entityType: t }) as Action),
      weight: 1,
    },
    {
      arbitrary: rowIndex.map((i) => ({ kind: 'edit-entity', rowIndex: i }) as Action),
      weight: 1,
    },
  )
  return fc.array(action, { minLength: 1, maxLength: maxActions })
}

async function runOneSequence(
  browser: Browser,
  server: ServerHandle,
  actions: Action[],
  opts: FuzzOptions,
): Promise<boolean> {
  const ctx = await browser.newContext()
  const page = await ctx.newPage()
  const consoleErrors: string[] = []
  page.on('console', async (msg) => {
    if (msg.type() !== 'error') return
    const text = msg.text()
    if (isBenign(text)) return
    const loc = msg.location()
    const argDetails: string[] = []
    for (const arg of msg.args()) {
      try {
        // Try to extract richer info: name, message, stack if it's an Error.
        const v = await arg.evaluate((o) => {
          if (o instanceof Error) {
            return {
              __error__: true,
              name: o.name,
              message: o.message,
              stack: o.stack,
            }
          }
          // Best-effort serialise other values.
          try {
            return JSON.parse(JSON.stringify(o))
          } catch {
            return String(o)
          }
        })
        argDetails.push(JSON.stringify(v))
      } catch (e) {
        argDetails.push(`<eval-failed: ${(e as Error).message}>`)
      }
    }
    consoleErrors.push(
      `CONSOLE.ERROR text="${text}" at ${loc.url}:${loc.lineNumber}:${loc.columnNumber} args=${argDetails.join(' | ')}`,
    )
  })
  page.on('pageerror', (err) => {
    if (isBenign(err.message)) return
    consoleErrors.push(`[pageerror] name=${err.name} msg=${err.message}\n${err.stack ?? '(no stack)'}`)
  })

  const slowActions: Array<{ action: string; ms: number }> = []
  const fctx: FuzzContext = {
    page,
    context: ctx,
    baseUrl: server.baseUrl,
    projectRoot: server.projectRoot,
    consoleErrors,
    recordErrors: true,
    latencyBudgetMs: opts.actionLatencyBudgetMs,
    slowActions,
    mutate: opts.mutate,
  }

  let oraclePassed = true
  let oracleError: string | undefined
  try {
    // Initial navigation: every sequence starts from a known-good state.
    await page.goto(`${server.baseUrl}/v2/list/all_tickets`, {
      waitUntil: 'domcontentloaded',
      timeout: 10_000,
    })
    await page
      .locator('.entity-row')
      .first()
      .waitFor({ state: 'visible', timeout: 10_000 })

    // Replay the generated action sequence. After each action we yield
    // briefly so any in-flight navigation/render has a chance to settle
    // before the next action triggers another. Without this we get a
    // lot of "Navigation to X is interrupted by another navigation"
    // false-positives that are harness artefacts, not bugs.
    for (const action of actions) {
      await runAction(fctx, action)
      await sleep(50)
    }

    // Settle pause before the oracle. Any pending navigation from the
    // last action must complete (or hit its 5 s per-action timeout)
    // before the oracle's fresh goto starts. Otherwise the oracle is
    // measuring the wrong thing.
    await sleep(200)
    try {
      await ctx.pages()[0]?.waitForLoadState('domcontentloaded', { timeout: 5_000 })
    } catch {
      /* settle is best-effort */
    }

    // Liveness oracle 1: strict per-action latency budget. If any
    // action in the replayed sequence exceeded the budget, the
    // sequence fails. This is the "long long time hang" guard that
    // catches the user's original reported symptom — a click or reload
    // that eventually succeeds but took multiple seconds of user time.
    if (slowActions.length > 0) {
      oraclePassed = false
      const summary = slowActions
        .map((s) => `${s.action}:${s.ms.toFixed(0)}ms`)
        .join(', ')
      oracleError = `${slowActions.length} slow action(s) over ${opts.actionLatencyBudgetMs}ms budget: ${summary}`
    }

    // Liveness oracle 2: a fresh navigation must still complete within
    // budget AND the resulting page must have at least one entity row
    // visible. This catches the "sequence left the SPA in a bad state"
    // case, distinct from "one action was slow".
    if (oraclePassed) {
      const oracleStart = performance.now()
      await page.goto(`${server.baseUrl}/v2/list/all_tickets`, {
        waitUntil: 'domcontentloaded',
        timeout: opts.oracleNavTimeoutMs,
      })
      await page
        .locator('.entity-row')
        .first()
        .waitFor({ state: 'visible', timeout: opts.oracleNavTimeoutMs })
      const oracleMs = performance.now() - oracleStart

      if (oracleMs > opts.oracleNavTimeoutMs) {
        oraclePassed = false
        oracleError = `oracle nav took ${oracleMs.toFixed(0)}ms`
      }
    }

    // Error oracle: any non-benign console error during the sequence is
    // a failure. The minimal failing sequence is then the smallest set
    // of actions that produces an error.
    if (oraclePassed && consoleErrors.length > 0) {
      oraclePassed = false
      oracleError = `${consoleErrors.length} console error(s): ${consoleErrors[0]}`
    }
  } catch (err) {
    oraclePassed = false
    oracleError = (err as Error).message
  } finally {
    if (!oraclePassed) {
      // Capture post-mortem state before the context is destroyed.
      // fast-check re-runs shrunk sequences so the failing one may
      // appear many times; only capture once per run to avoid flooding
      // the report dir.
      try {
        const captureDir = path.join(opts.reportDir, 'captures')
        fs.mkdirSync(captureDir, { recursive: true })
        const tag = `fail-${Date.now()}-${Math.floor(Math.random() * 1000)}`
        const url = page.url()
        const html = await page.content().catch(() => '<content-unavailable>')
        fs.writeFileSync(
          path.join(captureDir, `${tag}.info.txt`),
          `url=${url}\nerror=${oracleError}\nactions=${JSON.stringify(actions, null, 2)}\n`,
        )
        fs.writeFileSync(path.join(captureDir, `${tag}.html`), html.slice(0, 50_000))
        await page
          .screenshot({ path: path.join(captureDir, `${tag}.png`), fullPage: true })
          .catch(() => undefined)
      } catch {
        /* capture is best-effort */
      }
    }
    await ctx.close().catch(() => undefined)
  }

  if (!oraclePassed) {
    log(
      opts.reportDir,
      `[fuzz] oracle FAILED for sequence (${actions.length} actions): ${oracleError}`,
    )
  }
  return oraclePassed
}

async function runAction(ctx: FuzzContext, action: Action): Promise<void> {
  // Per-action hard timeout. Distinct from the latency budget: the
  // budget is "this was too slow to be user-acceptable" (failure),
  // while the timeout is "this will never complete" (also failure, but
  // the Playwright call itself throws). We set timeout > budget so a
  // budget-violating-but-completing action surfaces as a recorded slow
  // action rather than an exception.
  const PER_ACTION_TIMEOUT = Math.max(ctx.latencyBudgetMs * 2, 10_000)
  const actionStart = performance.now()
  let label: string = action.kind
  // Compound actions (create-entity, edit-entity, wait, touch-file)
  // are multi-step flows that aren't meaningfully measured against a
  // single-number latency budget, and non-browser actions like
  // touch-file shouldn't count at all. Set this flag for them so the
  // measurement block at the bottom skips them, even if they throw.
  let skipLatency = false
  try {
    switch (action.kind) {
      case 'goto':
        label = `goto:${action.list}`
        await ctx.page.goto(`${ctx.baseUrl}/v2/list/${action.list}`, {
          waitUntil: 'domcontentloaded',
          timeout: PER_ACTION_TIMEOUT,
        })
        break
      case 'click-row': {
        label = `click-row:${action.index}`
        const rows = await ctx.page.locator('.entity-row').all()
        if (rows.length === 0) return
        const idx = Math.min(action.index, rows.length - 1)
        await rows[idx]!
          .click({ timeout: PER_ACTION_TIMEOUT, trial: false })
          .catch(() => undefined)
        break
      }
      case 'reload':
        label = 'reload'
        await ctx.page.reload({
          waitUntil: 'domcontentloaded',
          timeout: PER_ACTION_TIMEOUT,
        })
        break
      case 'back':
        label = 'back'
        await ctx.page.goBack({ timeout: PER_ACTION_TIMEOUT }).catch(() => undefined)
        break
      case 'wait':
        skipLatency = true
        await sleep(action.ms)
        break
      case 'touch-file': {
        skipLatency = true
        const ents = listEntityFiles(ctx.projectRoot)
        if (ents.length === 0) break
        const f = ents[Math.floor(Math.random() * ents.length)]!
        const now = new Date()
        try {
          fs.utimesSync(f, now, now)
        } catch {
          /* ignore */
        }
        break
      }
      case 'create-entity':
        skipLatency = true
        label = `create:${action.entityType}`
        await createEntityViaUI(ctx, action.entityType, PER_ACTION_TIMEOUT)
        break
      case 'edit-entity':
        skipLatency = true
        label = `edit-entity:${action.rowIndex}`
        await editEntityViaUI(ctx, action.rowIndex, PER_ACTION_TIMEOUT)
        break
    }
  } catch {
    // Per-action errors are not the oracle — the latency, liveness and
    // console-error oracles are. Suppress the exception so the sequence
    // continues. We still reach the measurement block below, which
    // respects skipLatency.
  }
  if (skipLatency) return
  // Only atomic navigation actions reach here: goto, click-row,
  // reload, back. The budget catches "a single navigation hung for
  // seconds" which is the user's reported symptom.
  const elapsedMs = performance.now() - actionStart
  if (elapsedMs > ctx.latencyBudgetMs) {
    ctx.slowActions.push({ action: label, ms: elapsedMs })
  }
}

/**
 * Navigate to the create form for the given entity type, fill the
 * title, and either submit (if `ctx.mutate`) or navigate away.
 *
 * Mutation safety: when mutate=true, the submit writes to the /tmp
 * project copy which is cleaned up when the run ends. Never point
 * --source-project at a non-disposable directory when --mutate is on.
 */
async function createEntityViaUI(
  ctx: FuzzContext,
  entityType: EntityTypeName,
  timeoutMs: number,
): Promise<void> {
  const formId = CREATE_FORM_BY_TYPE[entityType]
  await ctx.page.goto(`${ctx.baseUrl}/v2/form/${formId}`, {
    waitUntil: 'domcontentloaded',
    timeout: timeoutMs,
  })
  const titleInput = ctx.page.locator('#field-title')
  try {
    await titleInput.waitFor({ state: 'visible', timeout: timeoutMs })
  } catch {
    return
  }
  const title = `fuzz-${entityType}-${Date.now()}-${Math.floor(Math.random() * 10_000)}`
  await titleInput.fill(title).catch(() => undefined)
  if (ctx.mutate) {
    const submit = ctx.page.locator('button[type="submit"]').first()
    await submit.click({ timeout: timeoutMs }).catch(() => undefined)
    await ctx.page
      .waitForURL(/\/v2\/(entity|list|form)\//, { timeout: timeoutMs })
      .catch(() => undefined)
  } else {
    // Navigate away without submitting — stateless mode.
    await ctx.page.goBack({ timeout: timeoutMs }).catch(() => undefined)
  }
}

/**
 * Navigate to the entity-detail page of the rowIndex-th row of the
 * currently-open list, open the edit form, and either submit a new
 * title (if `ctx.mutate`) or navigate away via back.
 */
async function editEntityViaUI(
  ctx: FuzzContext,
  rowIndex: number,
  timeoutMs: number,
): Promise<void> {
  if (!ctx.page.url().includes('/v2/list/')) {
    await ctx.page.goto(`${ctx.baseUrl}/v2/list/all_tickets`, {
      waitUntil: 'domcontentloaded',
      timeout: timeoutMs,
    })
  }
  const rows = await ctx.page.locator('.entity-row').all()
  if (rows.length === 0) return
  const idx = Math.min(rowIndex, rows.length - 1)
  await rows[idx]!.click({ timeout: timeoutMs }).catch(() => undefined)
  const editBtn = ctx.page.locator('button:has-text("Edit")').first()
  try {
    await editBtn.waitFor({ state: 'visible', timeout: timeoutMs })
    await editBtn.click({ timeout: timeoutMs })
  } catch {
    return
  }
  const titleInput = ctx.page.locator('#field-title')
  try {
    await titleInput.waitFor({ state: 'visible', timeout: timeoutMs })
  } catch {
    return
  }
  if (ctx.mutate) {
    const newTitle = `fuzz-edit-${Date.now()}-${Math.floor(Math.random() * 10_000)}`
    await titleInput.fill(newTitle).catch(() => undefined)
    const submit = ctx.page.locator('button[type="submit"]').first()
    await submit.click({ timeout: timeoutMs }).catch(() => undefined)
    await ctx.page
      .waitForURL(/\/v2\/(entity|list|form)\//, { timeout: timeoutMs })
      .catch(() => undefined)
  } else {
    // Navigate back without submitting — exercises the form-load path
    // without mutating state.
    await ctx.page.goBack({ timeout: timeoutMs }).catch(() => undefined)
  }
}

function listEntityFiles(root: string): string[] {
  // Cached on first call by storing on the function — avoids re-walking
  // the (potentially large) project tree for every action.
  type WithCache = typeof listEntityFiles & { cache?: Map<string, string[]> }
  const fn = listEntityFiles as WithCache
  if (!fn.cache) fn.cache = new Map()
  const cached = fn.cache.get(root)
  if (cached) return cached
  const entityRoot = path.join(root, 'entities')
  const out: string[] = []
  walk(entityRoot, (p) => {
    if (p.endsWith('.md')) out.push(p)
  })
  fn.cache.set(root, out)
  return out
}

function walk(dir: string, visit: (file: string) => void): void {
  let entries: fs.Dirent[]
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true })
  } catch {
    return
  }
  for (const entry of entries) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) walk(full, visit)
    else visit(full)
  }
}

function formatFailureError(result: fc.RunDetails<[Action[]]>): string {
  if ('error' in result && result.error) return String(result.error)
  return 'unknown failure'
}

function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms))
}

function log(dir: string, msg: string): void {
  const line = `[${new Date().toISOString()}] ${msg}\n`
  process.stderr.write(line)
  try {
    fs.appendFileSync(path.join(dir, 'fuzz.log'), line)
  } catch {
    /* ignore */
  }
}
