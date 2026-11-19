import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

// BUG-5MJ0AJZ. The app shell's frame clips its content but must not be a
// scroll container. With `overflow: hidden` it was one: bringing a run card
// into view (`?run=`, scrollIntoView) scrolled the frame as well as the
// page, which shifted the layout up and hid the top bar on mobile.
// `overflow: clip` clips the same way and cannot be scrolled at all.
//
// A grep test over the source, like focusRing.test.ts: the test runner
// has no layout engine, so a mounted shell cannot show the frame scrolling.

const SHELL = resolve(
  __dirname,
  '../../packages/rela-components/src/components/layout/RlAppShell.vue'
)

function rule(css: string, selector: string): string {
  const start = css.indexOf(`${selector} {`)
  expect(start, `${selector} rule`).toBeGreaterThanOrEqual(0)
  return css.slice(start, css.indexOf('}', start))
}

describe('app shell frame', () => {
  it('clips without being a scroll container', () => {
    const frame = rule(readFileSync(SHELL, 'utf8'), '.rl-app-shell__frame')
    expect(frame).toMatch(/overflow:\s*clip;/)
    expect(frame).not.toMatch(/overflow(-y)?:\s*(hidden|auto|scroll)/)
  })

  it('stops a run card below the sticky top bar of the entity page', () => {
    const card = readFileSync(resolve(__dirname, '../components/flows/FlowRunCard.vue'), 'utf8')
    expect(rule(card, '.flow-run-card')).toMatch(
      /scroll-margin-top:\s*var\(--rl-inline-edit-sticky-top/
    )
  })
})
