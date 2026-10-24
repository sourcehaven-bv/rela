// The conversion dialog: Review must show a review step even when the
// server sends null for an empty preview list, and errors must be shown.

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const post = vi.fn()
vi.mock('@/api/client', () => ({ api: { post: (...args: unknown[]) => post(...args) } }))

vi.mock('@/composables/modalStack', () => ({ useModalStack: vi.fn() }))

vi.mock('@/stores', () => ({
  useSchemaStore: () => ({
    getEntityType: () => ({ label: 'Opdracht', id_type: 'short', properties: {} }),
  }),
}))

import ConversionModal from './ConversionModal.vue'

const source = { id: 'TOPIC-G8Z2Y', type: 'topic', properties: {}, content: 'body' } as never
const offer = {
  name: 'topic-naar-opdracht',
  label: 'Omzetten naar opdracht',
  to: 'opdracht',
  allowed: true,
} as never

function mountModal() {
  return mount(ConversionModal, {
    props: { source, offer },
    global: { stubs: { Teleport: true } },
  })
}

function reviewButton(wrapper: ReturnType<typeof mountModal>) {
  return wrapper.findAll('button').find((b) => /Review|Convert/.test(b.text()))!
}

describe('ConversionModal', () => {
  beforeEach(() => post.mockReset())

  it('shows the review step when the preview lists are null', async () => {
    post.mockResolvedValueOnce({
      target_type: 'opdracht',
      required: null,
      relations_to_drop: null,
      invalid_relations: null,
      target_faces: null,
      moves: [{ source_face: '', target_face: '' }],
    })
    const wrapper = mountModal()
    await reviewButton(wrapper).trigger('click')
    await flushPromises()

    expect(post).toHaveBeenCalledWith(
      '/_conversions/topic-naar-opdracht/preflight',
      expect.objectContaining({ source_id: 'TOPIC-G8Z2Y' })
    )
    expect(wrapper.find('.rl-modal').exists()).toBe(true)
    expect(wrapper.text()).toContain('Review done')
    expect(reviewButton(wrapper).text()).toBe('Convert')
    expect(wrapper.emitted('close')).toBeUndefined()
  })

  it('converts on the second click', async () => {
    post
      .mockResolvedValueOnce({ target_type: 'opdracht', moves: [] })
      .mockResolvedValueOnce({ target_type: 'opdracht', moves: [] })
      .mockResolvedValueOnce({ entity_id: 'OPDR-1', face: '' })
    const wrapper = mountModal()
    await reviewButton(wrapper).trigger('click')
    await flushPromises()
    await reviewButton(wrapper).trigger('click')
    await flushPromises()
    expect(post).toHaveBeenLastCalledWith('/_conversions/topic-naar-opdracht', expect.anything())
    expect(wrapper.emitted('converted')).toEqual([['OPDR-1', '']])
  })

  it('shows a server error', async () => {
    post.mockRejectedValueOnce(new Error('Conversion definition changed; reload and try again'))
    const wrapper = mountModal()
    await reviewButton(wrapper).trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toContain('Conversion definition changed')
    expect(wrapper.emitted('close')).toBeUndefined()
  })
})
