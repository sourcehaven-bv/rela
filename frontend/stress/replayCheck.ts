// One-off replay tool: take a hard-coded action sequence and run it N
// times against a fresh BrowserContext, then report how many runs
// reproduced the failure. Used to verify that a fuzzer-found bug is
// actually deterministic before filing it as such.
//
// Run via tsx directly, not through the cli.ts entrypoint:
//   npx tsx stress/replayCheck.ts firefox 10

import { firefox, chromium, BrowserContext } from '@playwright/test'

import { startServer } from './serverProcess.js'

type Action =
  | { kind: 'goto'; list: string }
  | { kind: 'click-row'; index: number }
  | { kind: 'reload' }
  | { kind: 'back' }

const SEQUENCE: Action[] = [
  { kind: 'click-row', index: 0 },
  { kind: 'reload' },
  { kind: 'back' },
]

const BENIGN = [
  /connection to .* was interrupted while the page was loading/i,
  /the operation was aborted/i,
  /networkerror when attempting to fetch/i,
  /load failed/i,
]

async function main(): Promise<void> {
  const browserName = (process.argv[2] ?? 'firefox') as 'firefox' | 'chromium'
  const runs = parseInt(process.argv[3] ?? '10', 10)

  const reportDir = `/tmp/rela-replay-${browserName}-${Date.now()}`
  const server = await startServer({
    sourceProject: 'tickets',
    reportDir,
    enablePprof: false,
  })
  console.log(`server up at ${server.baseUrl}`)
  const launcher = browserName === 'firefox' ? firefox : chromium
  const browser = await launcher.launch({ headless: true })

  let failed = 0
  let totalErrors = 0
  const errorTexts = new Map<string, number>()
  try {
    for (let i = 0; i < runs; i++) {
      const ctx = await browser.newContext()
      const page = await ctx.newPage()
      const errs: string[] = []
      page.on('console', async (msg) => {
        if (msg.type() !== 'error') return
        const t = msg.text()
        if (BENIGN.some((p) => p.test(t))) return
        const loc = msg.location()
        const argDetails: string[] = []
        for (const arg of msg.args()) {
          try {
            const v = await arg.evaluate((o) => {
              if (o instanceof Error) {
                return {
                  __error__: true,
                  name: o.name,
                  message: o.message,
                  stack: o.stack,
                }
              }
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
        errs.push(
          `CONSOLE text="${t}" at ${loc.url}:${loc.lineNumber}:${loc.columnNumber} args=${argDetails.join(' | ')}`,
        )
      })
      page.on('pageerror', (err) => {
        if (BENIGN.some((p) => p.test(err.message))) return
        errs.push(`[pageerror] name=${err.name} msg=${err.message}\n${err.stack ?? '(no stack)'}`)
      })

      try {
        // Initial nav: same as the fuzzer's setup.
        await page.goto(`${server.baseUrl}/v2/list/all_tickets`, {
          waitUntil: 'domcontentloaded',
          timeout: 10_000,
        })
        await page
          .locator('.entity-row')
          .first()
          .waitFor({ state: 'visible', timeout: 10_000 })

        for (const a of SEQUENCE) {
          await runAction(ctx, page, server.baseUrl, a)
          await new Promise((r) => setTimeout(r, 50))
        }
        await new Promise((r) => setTimeout(r, 200))
      } catch (e) {
        errs.push(`[exception] ${(e as Error).message}`)
      }

      const failedThisRun = errs.length > 0
      if (failedThisRun) failed++
      totalErrors += errs.length
      for (const e of errs) {
        errorTexts.set(e, (errorTexts.get(e) ?? 0) + 1)
      }
      console.log(
        `[run ${i + 1}/${runs}] ${failedThisRun ? 'FAIL' : 'pass'} (${errs.length} errors)`,
      )

      await ctx.close().catch(() => undefined)
    }
  } finally {
    await browser.close().catch(() => undefined)
    await server.stop()
  }

  console.log('')
  console.log(`=== REPLAY SUMMARY ===`)
  console.log(`browser: ${browserName}`)
  console.log(`runs:    ${runs}`)
  console.log(`failed:  ${failed}/${runs}  (${((failed / runs) * 100).toFixed(0)}%)`)
  console.log(`errors:  ${totalErrors} total`)
  if (errorTexts.size > 0) {
    console.log('error breakdown:')
    for (const [text, n] of [...errorTexts.entries()].sort((a, b) => b[1] - a[1])) {
      console.log(`  ${n.toString().padStart(3)}× ${text}`)
    }
    console.log('\nfull error text of first few failures:')
    let shown = 0
    for (const text of errorTexts.keys()) {
      console.log(`---\n${text}`)
      if (++shown >= 5) break
    }
  }
  process.exit(failed > 0 ? 1 : 0)
}

async function runAction(
  ctx: BrowserContext,
  page: import('@playwright/test').Page,
  baseUrl: string,
  action: Action,
): Promise<void> {
  try {
    switch (action.kind) {
      case 'goto':
        await page.goto(`${baseUrl}/v2/list/${action.list}`, {
          waitUntil: 'domcontentloaded',
          timeout: 5_000,
        })
        break
      case 'click-row': {
        const rows = await page.locator('.entity-row').all()
        if (rows.length === 0) return
        const idx = Math.min(action.index, rows.length - 1)
        await rows[idx]!
          .click({ timeout: 5_000 })
          .catch(() => undefined)
        break
      }
      case 'reload':
        await page.reload({ waitUntil: 'domcontentloaded', timeout: 5_000 })
        break
      case 'back':
        await page.goBack({ timeout: 5_000 }).catch(() => undefined)
        break
    }
  } catch {
    /* per-action errors are not fatal — the oracle is errs.length */
  }
}

main().catch((err) => {
  console.error(err)
  process.exit(2)
})
