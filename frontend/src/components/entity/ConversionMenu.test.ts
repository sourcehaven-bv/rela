import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ConversionMenu from './ConversionMenu.vue'
import type { ConversionOffer } from '@/types'

function offer(name: string, allowed = true): ConversionOffer {
  const o: ConversionOffer = { name, label: `To ${name}`, to: name, allowed }
  return o
}

describe('ConversionMenu', () => {
  it('renders nothing without allowed offers', () => {
    const w = mount(ConversionMenu, { props: { offers: [offer('a', false)] } })
    expect(w.find('button').exists()).toBe(false)
  })

  it('renders one offer as a button with its label', async () => {
    const only = offer('a')
    const w = mount(ConversionMenu, { props: { offers: [only, offer('b', false)] } })
    expect(w.find('button').text()).toBe('To a')
    await w.find('button').trigger('click')
    expect(w.emitted('choose')).toEqual([[only]])
  })

  it('renders several offers as one Convert menu', async () => {
    const w = mount(ConversionMenu, {
      props: { offers: [offer('a'), offer('b'), offer('c')] },
      attachTo: document.body,
    })
    const buttons = w.findAll('button')
    expect(buttons).toHaveLength(1)
    expect(buttons[0].text()).toBe('Convert')
    w.unmount()
  })
})
