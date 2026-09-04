import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import CommentsPanel from './CommentsPanel.vue'
import { useSchemaStore } from '@/stores/schema'
import {
  listComments,
  addComment,
  updateComment,
  deleteComment,
  type Comment,
} from '@/api/comments'
import type { EntityType } from '@/types'

// The confirm dialog is a singleton composable backed by a modal mounted in
// App.vue; mocking it keeps the delete tests about the delete path rather than
// about dialog plumbing.
const mockConfirm = vi.fn<() => Promise<boolean>>()
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({ confirm: mockConfirm }),
}))

vi.mock('@/api/comments', () => ({
  listComments: vi.fn(),
  addComment: vi.fn(),
  updateComment: vi.fn(),
  deleteComment: vi.fn(),
}))

const mockList = vi.mocked(listComments)
const mockAdd = vi.mocked(addComment)
const mockUpdate = vi.mocked(updateComment)
const mockDelete = vi.mocked(deleteComment)

const ENTITY_TYPE = 'ticket'
const ENTITY_ID = 'TKT-001'

function comment(over: Partial<Comment> = {}): Comment {
  return {
    id: 'c1',
    author: 'alice@example.com',
    created_at: '2026-09-01T10:00:00Z',
    anchor: { kind: 'property', ref: 'status' },
    body: 'looks wrong',
    resolved: false,
    editable: true,
    deletable: true,
    ...over,
  }
}

/** Registers the entity type, defaulting to commentable. */
function seedType(over: Partial<EntityType> = {}) {
  const schemaStore = useSchemaStore()
  schemaStore.entityTypes = new Map([
    [
      ENTITY_TYPE,
      {
        label: 'Ticket',
        properties: { title: { type: 'string' }, status: { type: 'string' } },
        commentable: true,
        ...over,
      },
    ],
  ]) as never
}

async function mountPanel(sectionIds: string[] = []) {
  const w = mount(CommentsPanel, {
    props: { entityType: ENTITY_TYPE, entityId: ENTITY_ID, sectionIds },
  })
  await vi.waitFor(() => expect(w.vm).toBeTruthy())
  await w.vm.$nextTick()
  await w.vm.$nextTick()
  return w
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
  mockList.mockResolvedValue([])
  mockConfirm.mockResolvedValue(true)
})

describe('CommentsPanel', () => {
  it('renders nothing for a type the schema does not mark commentable', async () => {
    seedType({ commentable: false })
    const w = await mountPanel()

    expect(w.find('.comments-panel').exists()).toBe(false)
    // The panel must not probe the API for a type it will not render for.
    expect(mockList).not.toHaveBeenCalled()
  })

  it("lists a commentable type's thread", async () => {
    seedType()
    const existing = comment({ body: 'the status is stale' })
    mockList.mockResolvedValue([existing])

    const w = await mountPanel()

    expect(mockList).toHaveBeenCalledWith(ENTITY_TYPE, ENTITY_ID)
    expect(w.find('.comment-body').text()).toBe(existing.body)
    expect(w.find('.comment-author').text()).toBe(existing.author)
    expect(w.find('.comment-anchor').text()).toBe(existing.anchor.ref)
  })

  it('offers both property and section anchors', async () => {
    seedType()
    const w = await mountPanel(['history'])

    const values = w.findAll('.anchor-select option').map((o) => o.attributes('value'))
    expect(values).toContain('property:status')
    expect(values).toContain('section:history')
  })

  it('posts a comment with the selected anchor and clears the box', async () => {
    seedType()
    const created = comment({ id: 'c2', body: 'needs a second look' })
    mockAdd.mockResolvedValue(created)

    const w = await mountPanel()
    await w.find('.anchor-select').setValue('property:title')
    await w.find('.comment-input').setValue(created.body)
    await w.find('.comment-form').trigger('submit')
    await vi.waitFor(() => expect(mockAdd).toHaveBeenCalled())
    await w.vm.$nextTick()

    expect(mockAdd).toHaveBeenCalledWith(ENTITY_TYPE, ENTITY_ID, {
      anchor: { kind: 'property', ref: 'title' },
      body: created.body,
    })
    expect(w.find('.comment-body').text()).toBe(created.body)
    expect((w.find('.comment-input').element as HTMLTextAreaElement).value).toBe('')
  })

  it('resolves a comment without echoing its body back', async () => {
    seedType()
    const existing = comment()
    mockList.mockResolvedValue([existing])
    mockUpdate.mockResolvedValue()

    const w = await mountPanel()
    const resolveBtn = w.findAll('button').find((b) => b.text() === 'Resolve')
    await resolveBtn?.trigger('click')
    await vi.waitFor(() => expect(mockUpdate).toHaveBeenCalled())
    await w.vm.$nextTick()

    expect(mockUpdate).toHaveBeenCalledWith(ENTITY_TYPE, ENTITY_ID, existing.id, { resolved: true })
    expect(w.find('.comment').classes()).toContain('comment-resolved')
  })

  it('deletes a comment once confirmed', async () => {
    seedType()
    const existing = comment()
    mockList.mockResolvedValue([existing])
    mockDelete.mockResolvedValue()

    const w = await mountPanel()
    const deleteBtn = w.findAll('button').find((b) => b.text() === 'Delete')
    await deleteBtn?.trigger('click')
    await vi.waitFor(() => expect(mockDelete).toHaveBeenCalled())
    await w.vm.$nextTick()

    expect(mockDelete).toHaveBeenCalledWith(ENTITY_TYPE, ENTITY_ID, existing.id)
    expect(w.find('.comment').exists()).toBe(false)
  })

  it('keeps a comment when the delete confirmation is declined', async () => {
    seedType()
    mockList.mockResolvedValue([comment()])
    mockConfirm.mockResolvedValue(false)

    const w = await mountPanel()
    await w
      .findAll('button')
      .find((b) => b.text() === 'Delete')
      ?.trigger('click')
    await vi.waitFor(() => expect(mockConfirm).toHaveBeenCalled())
    await w.vm.$nextTick()

    expect(mockDelete).not.toHaveBeenCalled()
    expect(w.find('.comment').exists()).toBe(true)
  })

  it('flags a detached anchor without hiding the comment', async () => {
    seedType()
    const orphan = comment({ detached: true, anchor: { kind: 'property', ref: 'removed_field' } })
    mockList.mockResolvedValue([orphan])

    const w = await mountPanel()

    expect(w.find('.comment-detached').exists()).toBe(true)
    expect(w.find('.comment-body').text()).toBe(orphan.body)
  })

  it('hides controls the server did not grant', async () => {
    seedType()
    mockList.mockResolvedValue([comment({ editable: false, deletable: false })])

    const w = await mountPanel()

    const labels = w.findAll('.comment-actions button').map((b) => b.text())
    expect(labels).toEqual([])
  })

  it('reports a failed load rather than rendering an empty thread', async () => {
    seedType()
    mockList.mockRejectedValue(new Error('nope'))

    const w = await mountPanel()

    expect(w.find('.comments-error').exists()).toBe(true)
    expect(w.text()).not.toContain('No comments yet')
  })
})
