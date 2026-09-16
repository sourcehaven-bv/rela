// @vitest-environment jsdom
//
// Line-ending parity between marked and goldmark (#1594).
//
// `flattenToLine` (internal/dataentry/webhook_routes.go) replaces only \n and
// \r in an interpolated webhook value, leaving \v, \f, U+0085, U+2028 and
// U+2029 alone. Its doc comment is explicit that this is safe only because of
// what the PARSER treats as a line ending, not because of anything the
// function does — so the guarantee is a property of the renderers, and it was
// verified against just one of the two.
//
// goldmark has Go tests on the write path. marked renders the same document on
// the read path here, and a divergence on any one of these characters would
// reopen the forged-sibling-heading vector that rela#1578 closed: a webhook
// producer's value would start a new line, `## Injected` would parse as a
// heading, and every later delivery aimed at the real section would land above
// it. Same class of bug, different renderer.
//
// Each case is the actual attack string, not a character-class probe, and the
// assertion is that no heading element appears — the consequence the threat
// model cares about. \n and \r are included as positive controls: they MUST
// forge a heading here, which is precisely why flattenToLine removes them, and
// they prove these assertions can fail.
import { describe, it, expect } from 'vitest'
import { renderMarkdown } from './markdown'

// Built from code points rather than written literally: the exotic ones are
// invisible in an editor, and a stray copy-paste normalising one of them into
// a plain space would silently turn the test into a tautology.
const VT = String.fromCharCode(0x0b)
const FF = String.fromCharCode(0x0c)
const NEL = String.fromCharCode(0x85)
const LS = String.fromCharCode(0x2028)
const PS = String.fromCharCode(0x2029)
const LF = String.fromCharCode(0x0a)
const CR = String.fromCharCode(0x0d)

// The shape a webhook delivery actually produces: an operator's list-item
// template with one producer-controlled value interpolated into it.
function delivery(separator: string): string {
  return `- alert${separator}## Injected`
}

const HEADING = /<h[1-6][\s>]/i

describe('marked line-ending parity with goldmark', () => {
  // The survivors flattenToLine deliberately leaves in place.
  describe.each([
    ['VT (U+000B)', VT],
    ['FF (U+000C)', FF],
    ['NEL (U+0085)', NEL],
    ['LS (U+2028)', LS],
    ['PS (U+2029)', PS],
  ])('%s does not terminate a line', (_name, ch) => {
    it('renders no heading', () => {
      expect(renderMarkdown(delivery(ch))).not.toMatch(HEADING)
    })

    // Flattening is lossy by design but must not be silently destructive:
    // dropping the text would lose an alert the producer will not resend.
    it('keeps the value text', () => {
      expect(renderMarkdown(delivery(ch))).toContain('Injected')
    })
  })

  // Positive controls. If these ever stop forging a heading, the cases above
  // are no longer evidence of anything.
  describe.each([
    ['LF (U+000A)', LF],
    ['CR (U+000D)', CR],
  ])('%s does terminate a line', (_name, ch) => {
    it('renders a heading, which is why flattenToLine removes it', () => {
      expect(renderMarkdown(delivery(ch))).toMatch(HEADING)
    })
  })

  // End-to-end statement of the fix: the same hostile value, once put through
  // the server's flattening rule, is inert in marked.
  it('renders no heading for a value flattened the way flattenToLine flattens it', () => {
    const hostile = `first${LF}${LF}## Injected${LF}${LF}body`
    const flattened = hostile.replace(/[\n\r]/g, ' ').replace(/\0/g, '')

    const html = renderMarkdown(`- ${flattened}`)
    expect(html).not.toMatch(HEADING)
    expect(html).toContain('Injected')
  })
})
