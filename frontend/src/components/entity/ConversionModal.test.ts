// The conversion dialog: Review must show a review step even when the
// server sends null for an empty preview list, and errors must be shown.

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const post = vi.fn()
vi.mock('@/api/client', () => ({ api: { post: (...args: unknown[]) => post(...args) } }))

vi.mock('@/composables/modalStack', () => ({ useModalStack: vi.fn() }))

let targetProperties: Record<string, unknown> = {}
let forms = new Map<string, unknown>()
vi.mock('@/stores', () => ({
  useSchemaStore: () => ({
    getEntityType: () => ({ label: 'Opdracht', id_type: 'short', properties: targetProperties }),
    get forms() {
      return forms
    },
  }),
}))

// Milkdown needs a real layout engine; the dialog's contract with it is the
// v-model, which the stub keeps.
const MarkdownEditorStub = {
  name: 'MarkdownEditor',
  props: ['modelValue', 'refResolver', 'mentionSelf'],
  emits: ['update:modelValue'],
  template: '<div class="md-stub">{{ modelValue }}</div>',
}

import ConversionModal from './ConversionModal.vue'

const source = { id: 'TOPIC-G8Z2Y', type: 'topic', properties: {}, content: 'body' } as never
const offer = {
  name: 'topic-naar-opdracht',
  label: 'Omzetten naar opdracht',
  to: 'opdracht',
  allowed: true,
} as never

function mountModal(offerOverride: Record<string, unknown> = {}) {
  return mount(ConversionModal, {
    props: { source, offer: { ...(offer as object), ...offerOverride } as never },
    global: { stubs: { Teleport: true, MarkdownEditor: MarkdownEditorStub } },
  })
}

function reviewButton(wrapper: ReturnType<typeof mountModal>) {
  return wrapper.findAll('button').find((b) => /Review|Convert/.test(b.text()))!
}

describe('ConversionModal', () => {
  beforeEach(() => {
    post.mockReset()
    targetProperties = {}
    forms = new Map()
  })

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

  it('renders target fields through the widget registry with form config', () => {
    targetProperties = {
      priority: { type: 'enum', values: ['low', 'high'], labels: { high: 'Hoog' }, required: true },
      due: { type: 'date' },
      title: { type: 'string', required: true },
    }
    forms = new Map([
      ['opdracht-new', { entity: 'opdracht', fields: [{ property: 'priority', label: 'Prio' }] }],
    ])
    const wrapper = mountModal({ mapped_fields: ['title'] })
    const required = wrapper.find('.convert-fields')
    expect(required.text()).toContain('Prio')
    expect(required.find('select').exists()).toBe(true)
    expect(required.text()).toContain('Hoog')
    expect(wrapper.text()).not.toContain('title')
    const more = wrapper.find('details.convert-more')
    expect(more.find('input[type="date"]').exists()).toBe(true)
  })

  it('edits the body in the markdown editor and sends it', async () => {
    post.mockResolvedValueOnce({ target_type: 'opdracht', moves: [] })
    const wrapper = mountModal()
    const editor = wrapper.findComponent({ name: 'MarkdownEditor' })
    expect(editor.props('modelValue')).toBe('body')
    editor.vm.$emit('update:modelValue', 'new body')
    await reviewButton(wrapper).trigger('click')
    await flushPromises()
    expect(post).toHaveBeenCalledWith(
      '/_conversions/topic-naar-opdracht/preflight',
      expect.objectContaining({ content: 'new body' })
    )
  })

  it('hides the body editor when the mapping supplies the content', () => {
    const wrapper = mountModal({ content_mapped: true })
    expect(wrapper.findComponent({ name: 'MarkdownEditor' }).exists()).toBe(false)
  })

  it('shows a missing required field next to its input', async () => {
    targetProperties = { owner: { type: 'string', required: true } }
    post.mockResolvedValueOnce({ target_type: 'opdracht', required: ['owner'], moves: [] })
    const wrapper = mountModal()
    await reviewButton(wrapper).trigger('click')
    await flushPromises()
    expect(wrapper.find('.convert-fields').text()).toContain('This field is required')
  })

  it('opens the optional fields on an error there and clears it on edit', async () => {
    targetProperties = { notes: { type: 'string' } }
    post.mockResolvedValueOnce({ target_type: 'opdracht', required: ['notes'], moves: [] })
    const wrapper = mountModal()
    const more = () => wrapper.find('details.convert-more')
    expect(more().attributes('open')).toBeUndefined()
    await reviewButton(wrapper).trigger('click')
    await flushPromises()
    expect(more().attributes('open')).toBeDefined()
    expect(more().text()).toContain('This field is required')
    await more().find('input').setValue('a note')
    expect(more().text()).not.toContain('This field is required')
  })

  it('names the file properties that are set after the conversion', () => {
    targetProperties = { scan: { type: 'file' } }
    const wrapper = mountModal()
    expect(wrapper.find('.convert-files').text()).toContain('scan')
    expect(wrapper.find('details.convert-more').exists()).toBe(false)
  })
})
