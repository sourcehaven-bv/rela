// Scenario: reproduce BUG-FMS1.
//
// Hypothesis: when the file watcher fires onReload (which takes
// App.mu.Lock) while a slow read handler holds App.mu.RLock, Go's
// writer-priority RWMutex blocks every new request until the slow reader
// finishes. With the user's tickets project (≈1k files) and IDE/git
// activity touching files, this stalls page loads for seconds.
//
// To reproduce: one or more browser users navigate the SPA and trigger
// reads (list pages, entity details, search), while a background
// "watcher pressure" operation rapidly touches files on disk to fire the
// watcher. The schema canary measures the cost.

import type { Scenario } from '../types.js'

export const watcherPressureScenario: Scenario = {
  name: 'watcher-pressure',
  description:
    'Reproduce BUG-FMS1 by firing the file watcher while users navigate the SPA',
  users: 4,
  defaultDurationMs: 30_000,
  thresholds: {
    maxServer5xx: 0,
    schemaP99Ms: 200,
    mutationP99Ms: 1_000,
    maxConsoleErrors: 0,
    maxGoroutineGrowth: 50,
  },
  operations: [
    // Heavy reads to keep the App's RLock hot.
    {
      name: 'navigate-list',
      weight: 30,
      async fn(ctx) {
        const lists = ['all_tickets', 'all_bugs', 'all_features', 'all_ideas']
        const target = ctx.rand.pick(lists)
        await ctx.page.goto(`${ctx.baseUrl}/v2/list/${target}`, {
          waitUntil: 'domcontentloaded',
          timeout: 10_000,
        })
      },
    },
    {
      name: 'open-entity',
      weight: 25,
      async fn(ctx) {
        // Navigate to the list first if not already there, then click a
        // random row. We don't care which entity, only that the SPA does
        // a real entity-detail fetch through the SSE-connected page.
        const url = ctx.page.url()
        if (!url.includes('/v2/list/')) {
          await ctx.page.goto(`${ctx.baseUrl}/v2/list/all_tickets`, {
            waitUntil: 'domcontentloaded',
          })
        }
        // Wait briefly for rows. If the list itself is broken we want
        // that to surface as a row-not-found error, not a silent skip.
        const first = ctx.page.locator('.entity-row').first()
        await first.waitFor({ state: 'visible', timeout: 10_000 })
        const rows = await ctx.page.locator('.entity-row').all()
        if (rows.length === 0) return
        const row = rows[ctx.rand.int(rows.length)]!
        // Re-throw timeout so the runner records this op as failed.
        // 10s is generous; if a click takes longer, that's the bug.
        await row.click({ timeout: 10_000 })
        // Wait for navigation to settle so latency counts include the
        // detail-view bootstrap, not just the click event.
        await ctx.page.waitForURL(/\/v2\/(?:entity|form|list)\//, { timeout: 10_000 })
      },
    },
    {
      name: 'fetch-schema',
      weight: 10,
      async fn(ctx) {
        // Direct API hit alongside the canary — provides a second lens
        // on the schema endpoint without going through the browser.
        const r = await ctx.api.timed('GET', '_schema')
        if (r.status >= 500) throw new Error(`schema status ${r.status}`)
      },
    },
    // The actual pressure source — touches files on disk to fire the
    // file watcher. Weighted high enough that the watcher fires multiple
    // times per second across all users.
    {
      name: 'touch-file-on-disk',
      weight: 25,
      async fn(ctx) {
        ctx.project.touchRandomEntity(ctx.rand)
      },
    },
    {
      name: 'rewrite-file-on-disk',
      weight: 10,
      async fn(ctx) {
        // utimes-only touches don't always trigger fsnotify Write events;
        // an actual content rewrite always does. Mix both so we cover
        // platforms that ignore mtime-only updates.
        ctx.project.rewriteRandomEntity(ctx.rand)
      },
    },
  ],
}
