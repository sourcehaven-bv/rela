/*
 * Runs axe-core's colour-contrast rule over every story in both themes.
 *
 * Not part of `npm run check`, because it needs a running Storybook:
 *
 *     npm run storybook
 *     node scripts/axe-themes.mjs
 *
 * The theme class is applied before the page settles and the run waits for
 * the network to go idle, because axe measures the colours actually painted:
 * scanning too early reports the light palette on a dark page as hundreds of
 * false failures.
 */

import { chromium } from 'playwright'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'

const axeSource = readFileSync(createRequire(import.meta.url).resolve('axe-core'), 'utf8')
const base = process.env.STORYBOOK_URL ?? 'http://localhost:6006'

const index = await (await fetch(`${base}/index.json`)).json()
const ids = Object.values(index.entries)
  .filter((e) => e.type === 'story')
  .map((e) => e.id)

const browser = await chromium.launch()
let failures = 0

for (const [theme, width] of [['dark', 1440], ['dark', 390], ['light', 1440]]) {
  const context = await browser.newContext({ viewport: { width, height: 900 } })
  const page = await context.newPage()
  let violations = 0

  for (const id of ids) {
    await page.goto(`${base}/iframe.html?id=${id}&viewMode=story`, { waitUntil: 'networkidle' })
    await page.evaluate((t) => {
      const root = document.documentElement
      root.classList.remove('dark', 'light')
      root.classList.add(t)
    }, theme)
    await page.waitForTimeout(150)
    await page.addScriptTag({ content: axeSource })

    const found = await page.evaluate(async () => {
      const result = await window.axe.run(document, { runOnly: ['color-contrast'] })
      return result.violations.flatMap((v) => v.nodes.map((n) => n.any?.[0]?.message ?? ''))
    })

    if (found.length > 0) {
      violations += found.length
      console.error(`${theme} @ ${width}px  ${id}`)
      for (const message of found) console.error(`    ${message}`)
    }
  }

  console.log(`${theme} @ ${width}px: ${ids.length} stories, ${violations} contrast violations`)
  failures += violations
  await context.close()
}

await browser.close()
process.exit(failures > 0 ? 1 : 0)
