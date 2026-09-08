import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import CopyMenu from './CopyMenu.vue'
import type { CopyOffer } from '@/types'

// RULING 9's two affordance shapes, and the rule that a denied copy is ABSENT
// rather than disabled.
//
// Ruling 10 note — most assertions here are ABSENCES ("no button renders"),
// which is exactly the class that passes trivially when nothing mounted at
// all. So every absence test also asserts a matching PRESENCE from the same
// mount: an offer that should render does render. A test that only checked
// `find('button').exists() === false` would pass against a component that
// throws on setup.

function offer(over: Partial<CopyOffer> = {}): CopyOffer {
  return {
    name: 'promote-policy',
    label: 'Publish this policy',
    targetFace: 'policy@published',
    allowed: true,
    ...over,
  }
}

describe('CopyMenu', () => {
  it('renders ONE allowed offer as a single button carrying its label', () => {
    const w = mount(CopyMenu, { props: { offers: [offer()] } })
    const btn = w.get('button')
    expect(btn.text()).toBe('Publish this policy')
    // Single-offer form is a plain button, not a disclosure control.
    expect(btn.attributes('aria-haspopup')).toBeUndefined()
  })

  it('falls back to the definition NAME when no label is configured', () => {
    const w = mount(CopyMenu, {
      props: { offers: [offer({ name: 'promote-control', label: '' })] },
    })
    expect(w.get('button').text()).toBe('promote-control')
  })

  it('renders SEVERAL allowed offers as one menu, not sibling buttons', async () => {
    const offers = [
      offer({ name: 'translate-nl', label: 'Vertaal naar Nederlands' }),
      offer({ name: 'translate-fr', label: 'Traduire en français' }),
    ]
    const w = mount(CopyMenu, { props: { offers } })

    // Closed: exactly one control, the disclosure.
    expect(w.findAll('button')).toHaveLength(1)
    const toggle = w.get('button')
    expect(toggle.attributes('aria-haspopup')).toBe('menu')

    await toggle.trigger('click')
    const items = w.findAll('[role="menuitem"]')
    expect(items.map((i) => i.text())).toEqual([
      'Vertaal naar Nederlands',
      'Traduire en français',
    ])
  })

  it('hides a DENIED offer entirely — absent, not disabled', () => {
    const denied = offer({ allowed: false, reason: 'requires permission "x"' })
    const w = mount(CopyMenu, { props: { offers: [denied] } })

    // The absence...
    expect(w.findAll('button')).toHaveLength(0)
    // ...and the proof the absence is meaningful: the SAME component with the
    // SAME offer flipped to allowed renders a button. Without this the
    // assertion above would pass against a component that renders nothing ever.
    const allowedMount = mount(CopyMenu, { props: { offers: [offer()] } })
    expect(allowedMount.findAll('button')).toHaveLength(1)
  })

  it('renders only the ALLOWED subset when offers are mixed', () => {
    const w = mount(CopyMenu, {
      props: {
        offers: [
          offer({ name: 'translate-nl', label: 'Dutch' }),
          offer({ name: 'translate-fr', label: 'French', allowed: false }),
        ],
      },
    })
    // One allowed offer out of two => the single-button form, showing the
    // allowed one. The denied label must not appear anywhere.
    const btn = w.get('button')
    expect(btn.text()).toBe('Dutch')
    expect(w.text()).not.toContain('French')
  })

  it('renders nothing for [] and for absent offers, which are different claims', () => {
    expect(mount(CopyMenu, { props: { offers: [] } }).findAll('button')).toHaveLength(0)
    expect(mount(CopyMenu, { props: {} }).findAll('button')).toHaveLength(0)
    // Presence control, as above.
    expect(mount(CopyMenu, { props: { offers: [offer()] } }).findAll('button')).toHaveLength(1)
  })

  it('emits the whole offer on invoke, so the parent need not re-look it up', async () => {
    const o = offer()
    const w = mount(CopyMenu, { props: { offers: [o] } })
    await w.get('button').trigger('click')
    expect(w.emitted('invoke')).toEqual([[o]])
  })

  it('disables the control while an invoke is in flight', async () => {
    const w = mount(CopyMenu, { props: { offers: [offer()], busy: true } })
    expect(w.get('button').attributes('disabled')).toBeDefined()
    // Proof the attribute tracks `busy` rather than always being set.
    await w.setProps({ busy: false })
    expect(w.get('button').attributes('disabled')).toBeUndefined()
  })

  it('closes the menu after a choice', async () => {
    const offers = [
      offer({ name: 'a', label: 'A' }),
      offer({ name: 'b', label: 'B' }),
    ]
    const w = mount(CopyMenu, { props: { offers } })
    await w.get('button').trigger('click')
    expect(w.findAll('[role="menuitem"]')).toHaveLength(2)
    await w.get('[role="menuitem"]').trigger('click')
    expect(w.findAll('[role="menuitem"]')).toHaveLength(0)
  })
})
