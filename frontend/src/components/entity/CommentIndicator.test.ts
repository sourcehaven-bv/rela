import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import CommentIndicator from './CommentIndicator.vue'
import { addComment, updateComment, deleteComment, type Comment } from '@/api/comments'

const mockConfirm = vi.fn<() => Promise<boolean>>()
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: mockConfirm }),
}))

vi.mock('@/api/comments', () => ({
  addComment: vi.fn(),
  updateComment: vi.fn(),
  deleteComment: vi.fn(),
}))

const mockAdd = vi.mocked(addComment)
const mockUpdate = vi.mocked(updateComment)
const mockDelete = vi.mocked(deleteComment)

const ENTITY_TYPE = 'ticket'
const ENTITY_ID = 'TKT-001'
const ANCHOR = { kind: 'property', ref: 'priority' } as const

function comment(over: Partial<Comment> = {}): Comment {
  return {
    id: 'c1',
    author: 'alice@example.com',
    created_at: '2026-09-01T10:00:00Z',
    anchor: { ...ANCHOR },
    body: 'looks wrong',
    resolved: false,
    editable: true,
    deletable: true,
    ...over,
  }
}

function mountIndicator(comments: Comment[] = [], flip = false) {
  return mount(CommentIndicator, {
    props: { entityType: ENTITY_TYPE, entityId: ENTITY_ID, anchor: { ...ANCHOR }, comments, flip },
    attachTo: document.body,
  })
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
  mockConfirm.mockResolvedValue(true)
})

describe('CommentIndicator', () => {
  it('shows a count for a commented field', () => {
    const w = mountIndicator([comment(), comment({ id: 'c2' })])

    expect(w.find('.ci-count').text()).toBe('2')
    expect(w.find('.ci-btn').classes()).toContain('ci-btn--has')
  })

  it('marks a field whose comments are all resolved as done, not open', () => {
    const w = mountIndicator([comment({ resolved: true })])

    expect(w.find('.ci-btn').classes()).toContain('ci-btn--done')
    expect(w.find('.ci-btn').classes()).not.toContain('ci-btn--has')
  })

  it('stays present but empty-styled when the field has no comments', () => {
    const w = mountIndicator([])

    expect(w.find('.ci-btn').exists()).toBe(true)
    expect(w.find('.ci-btn').classes()).toContain('ci-btn--empty')
    expect(w.find('.ci-count').exists()).toBe(false)
  })

  it('flags a detached anchor on the indicator itself', () => {
    const w = mountIndicator([comment({ detached: true })])

    expect(w.find('.ci-btn').classes()).toContain('ci-btn--detached')
  })

  it('opens and closes the popover', async () => {
    const w = mountIndicator([comment()])
    expect(w.find('.ci-pop').exists()).toBe(false)

    await w.find('.ci-btn').trigger('click')
    expect(w.find('.ci-pop').exists()).toBe(true)
    expect(w.find('.ci-body').text()).toBe('looks wrong')

    await w.find('.ci-x').trigger('click')
    expect(w.find('.ci-pop').exists()).toBe(false)
  })

  it('closes on Escape', async () => {
    const w = mountIndicator([comment()])
    await w.find('.ci-btn').trigger('click')
    expect(w.find('.ci-pop').exists()).toBe(true)

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await w.vm.$nextTick()

    expect(w.find('.ci-pop').exists()).toBe(false)
  })

  it('posts against its own anchor and emits changed', async () => {
    const w = mountIndicator([])
    mockAdd.mockResolvedValue(comment({ id: 'new' }))

    await w.find('.ci-btn').trigger('click')
    await w.find('.ci-composer .ci-input').setValue('a new remark')
    await w.find('.ci-composer').trigger('submit')
    await vi.waitFor(() => expect(mockAdd).toHaveBeenCalled())

    expect(mockAdd).toHaveBeenCalledWith(ENTITY_TYPE, ENTITY_ID, {
      anchor: { ...ANCHOR },
      body: 'a new remark',
    })
    expect(w.emitted('changed')).toHaveLength(1)
  })

  it('resolves without echoing the body back', async () => {
    const existing = comment()
    const w = mountIndicator([existing])
    mockUpdate.mockResolvedValue()

    await w.find('.ci-btn').trigger('click')
    await w
      .findAll('.ci-mini')
      .find((b) => b.text() === 'Resolve')
      ?.trigger('click')
    await vi.waitFor(() => expect(mockUpdate).toHaveBeenCalled())

    expect(mockUpdate).toHaveBeenCalledWith(ENTITY_TYPE, ENTITY_ID, existing.id, { resolved: true })
    expect(w.emitted('changed')).toHaveLength(1)
  })

  it('deletes only after the confirmation is accepted', async () => {
    const existing = comment()
    const w = mountIndicator([existing])
    mockDelete.mockResolvedValue()
    mockConfirm.mockResolvedValue(false)

    await w.find('.ci-btn').trigger('click')
    await w
      .findAll('.ci-mini')
      .find((b) => b.text() === 'Delete')
      ?.trigger('click')
    await vi.waitFor(() => expect(mockConfirm).toHaveBeenCalled())

    expect(mockDelete).not.toHaveBeenCalled()
    expect(w.emitted('changed')).toBeUndefined()
  })

  it('hides controls the server did not grant', async () => {
    const w = mountIndicator([comment({ editable: false, deletable: false })])

    await w.find('.ci-btn').trigger('click')

    expect(w.findAll('.ci-cmt .ci-mini')).toHaveLength(0)
  })

  it('opens right-aligned when told to flip', async () => {
    const w = mountIndicator([comment()], true)
    await w.find('.ci-btn').trigger('click')

    expect(w.find('.ci-pop').classes()).toContain('ci-pop--flip')
  })
})
