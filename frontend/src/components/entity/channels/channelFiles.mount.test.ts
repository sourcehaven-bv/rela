import { describe, expect, it } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import RlChannelMessage from 'rela-components/components/channel/RlChannelMessage.vue'
import RlChannelComposer from 'rela-components/components/channel/RlChannelComposer.vue'
import type {
  ChannelMessageTypeDef,
  ChannelMessageView,
  ChannelPostEvent,
} from 'rela-components/components/channel/types'

// The library's file chips and the composer's file field, as the app wires
// them: chips draw an <img> only for a preview URL the app supplies, and the
// composer hands picked files back as `files`, never as a prop value.

const note: ChannelMessageTypeDef = {
  name: 'note',
  label: 'Note',
  persist: 'log',
  reply: true,
  actions: [],
  properties: [
    { name: 'body', label: 'Body', type: 'markdown' },
    { name: 'attach', label: 'Attach', type: 'file', max: 2 },
  ],
}

describe('RlChannelMessage file chips', () => {
  const message: ChannelMessageView = {
    seq: 5,
    rev: 5,
    type: 'note',
    props: { body: 'hi' },
    files: [
      { id: 'a', name: 'shot.png', size: '2 kB', previewUrl: 'blob:shot' },
      { id: 'b', name: 'page.html', size: '1 kB' },
    ],
  }

  it('shows a thumbnail only for a file with a preview URL, and a chip for every file', () => {
    const w = mount(RlChannelMessage, { props: { message, messageType: note } })
    const imgs = w.findAll('.rl-channel-message__files img')
    expect(imgs).toHaveLength(1)
    expect(imgs[0]!.attributes('src')).toBe('blob:shot')
    expect(w.find('.rl-channel-message__files').text()).toContain('page.html')
    expect(w.findAll('.rl-channel-message__files li')).toHaveLength(2)
    expect(w.html()).not.toContain('<iframe')
    expect(w.html()).not.toContain('<object')
  })

  it('emits download with the message seq and the file id', async () => {
    const w = mount(RlChannelMessage, { props: { message, messageType: note } })
    await w.findAll('.rl-channel-message__file-button')[1]!.trigger('click')
    expect(w.emitted('download')).toEqual([[5, 'b']])
  })
})

describe('RlChannelComposer file field', () => {
  it('posts picked files in `files`, never in values or rendered', async () => {
    const w = mount(RlChannelComposer, {
      props: { messageTypes: [note], postable: ['note'] },
      attachTo: document.body,
    })
    const input = w.find('input[type="file"]')
    expect(input.attributes('multiple')).toBeDefined()
    const f = new File(['x'], 'a.txt', { type: 'text/plain' })
    Object.defineProperty(input.element, 'files', { value: [f], configurable: true })
    await input.trigger('change')
    expect(w.text()).toContain('a.txt')
    await w.find('.rl-channel-composer__send').trigger('click')
    await flushPromises()
    const ev = w.emitted('post')?.[0]?.[0] as ChannelPostEvent | undefined
    expect(ev?.files).toEqual({ attach: [f] })
    expect(ev?.values).not.toHaveProperty('attach')
    expect(ev?.rendered).not.toContain('attach')
    w.unmount()
  })
})
