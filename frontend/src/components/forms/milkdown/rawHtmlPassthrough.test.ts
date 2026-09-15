// Raw-HTML passthrough in the Milkdown editor (#1597).
//
// CommonMark lets an entity body contain literal HTML — `<img>`, `<div>`,
// `<script>` — and the `commonmark` preset keeps it as an `html` node rather
// than dropping it, so the bytes DO reach the editor. TKT-3I9DDY's security
// note claimed "no HTML is executed by the editor"; that was correct but
// unverified, which is the whole of the finding.
//
// The claim rests on one property of the preset's `html` node: its `toDOM`
// returns the ProseMirror spec `['span', attrs, node.attrs.value]`, whose
// third element is a CHILD — ProseMirror builds it with `createTextNode`,
// not by assigning `innerHTML`. Markup therefore becomes visible text.
// Nothing in our code declares that, so nothing in our code would fail if a
// future Milkdown release switched the node to a DOM-parsing renderer, or if
// someone added an override that did. Milkdown has shipped exactly this
// vulnerability class twice in adjacent nodes (CVE-2026-57530 link href,
// CVE-2026-57531 emoji innerHTML), so a silent regression here is a realistic
// upgrade hazard, not a hypothetical one.
//
// These tests assert the OBSERVABLE consequence — no element, no handler, no
// navigation-capable href — rather than the DOM-spec shape, so they keep
// holding however the node is implemented.
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import MilkdownEditor from './MilkdownEditor.vue'

const searchEntities = vi.fn()
vi.mock('@/api', () => ({
  searchEntities: (...args: unknown[]) => searchEntities(...args),
  listEntities: vi.fn(),
}))

async function mountEditor(modelValue: string) {
  const wrapper = mount(MilkdownEditor, {
    props: { modelValue },
    attachTo: document.body,
  })
  await flushPromises()
  await flushPromises()
  return wrapper
}

describe('MilkdownEditor raw-HTML passthrough', () => {
  beforeEach(() => {
    searchEntities.mockReset()
    searchEntities.mockResolvedValue({ data: [] })
  })
  afterEach(() => {
    document.body.innerHTML = ''
  })

  // The issue's literal reproduction case. `onerror` fires without any user
  // interaction, so an <img> element existing at all is the failure — we do
  // not need the handler to run to know the sink is unsafe.
  it('does not create an element for an inline <img onerror>', async () => {
    const w = await mountEditor('Body <img src=x onerror="alert(1)"> tail\n')
    const editor = w.find('.ProseMirror')

    expect(editor.find('img').exists()).toBe(false)
    // The markup survives as visible text: this is passthrough, not stripping.
    expect(editor.text()).toContain('onerror')
    w.unmount()
  })

  it('does not create an element for a block-level <script>', async () => {
    // Assembled rather than written literally: a bundler rewriting a literal
    // closing script tag inside a string is a classic way for this kind of
    // test to stop testing what it names.
    const tag = ['<script>window.__pwned = true</', 'script>'].join('')
    const w = await mountEditor(`${tag}\n`)
    await flushPromises()

    expect(w.find('.ProseMirror').find('script').exists()).toBe(false)
    expect((window as unknown as { __pwned?: boolean }).__pwned).toBeUndefined()
    w.unmount()
  })

  // A container tag is the subtler case: it creates no handler by itself, so
  // a DOM-parsing sink would pass the two tests above while still building an
  // attacker-controlled subtree that later markup can exploit.
  it('does not build a subtree for container tags', async () => {
    const w = await mountEditor('<div id="injected"><span>inner</span></div>\n')
    const editor = w.find('.ProseMirror')

    expect(editor.find('#injected').exists()).toBe(false)
    expect(editor.text()).toContain('injected')
    w.unmount()
  })

  // Covers the shape of CVE-2026-57530 on the path the editor actually uses:
  // a javascript: URL must not survive as a clickable href. The preset's
  // sanitizeLinkHref blanks non-allowlisted schemes; assert the outcome so an
  // unpinned downgrade below 7.21.3 fails here.
  it('does not render a javascript: link as a navigable href', async () => {
    const w = await mountEditor('[click](javascript:alert(1))\n')
    const anchors = w.find('.ProseMirror').findAll('a')

    for (const a of anchors) {
      expect(a.attributes('href') ?? '').not.toMatch(/^javascript:/i)
    }
    w.unmount()
  })

  // Passthrough must be lossless in BOTH directions. If a future sanitizer
  // were added at the parse step instead of at the render step, opening and
  // saving an entity would silently rewrite the author's stored body — a
  // data-integrity bug wearing a security fix's clothes. The editor's own
  // write-back guard is what surfaces that, so assert through it.
  it('round-trips raw HTML back to the stored markdown unchanged', async () => {
    const src = 'Body <img src=x onerror="alert(1)"> tail\n'
    const w = await mountEditor(src)

    const guarded = (
      w.vm as unknown as { guardedValue: () => { value: string; verdict: string } }
    ).guardedValue()

    expect(guarded.value).toContain('<img src=x onerror="alert(1)">')
    expect(w.emitted('update:modelValue')).toBeUndefined()
    w.unmount()
  })
})
