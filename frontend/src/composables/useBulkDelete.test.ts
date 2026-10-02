import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useBulkDelete, UNDO_TOAST_MS, type DeleteTarget } from './useBulkDelete'
import { isTextEntryFocused } from '@/utils/dom'
import { useToasts } from 'rela-components/components/feedback/useToasts'

const deleteEntityMock = vi.fn()
const restoreEntityMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  deleteEntity: (...args: unknown[]) => deleteEntityMock(...args),
  restoreEntity: (...args: unknown[]) => restoreEntityMock(...args),
}))

function targets(...ids: string[]): DeleteTarget[] {
  return ids.map((id) => ({ id, type: 'policy', ref: id }))
}

function setup(noun = { singular: 'policy', plural: 'policies' }) {
  const onStart = vi.fn()
  const onSettled = vi.fn()
  const refresh = vi.fn().mockResolvedValue(undefined)
  const api = useBulkDelete({ noun: () => noun, onStart, onSettled, refresh })
  return { ...api, onStart, onSettled, refresh }
}

const toasts = () => useToasts().toasts.value

describe('useBulkDelete', () => {
  beforeEach(() => {
    deleteEntityMock.mockReset().mockResolvedValue(undefined)
    restoreEntityMock.mockReset().mockResolvedValue(undefined)
  })

  it('deletes each target by its address and reports the outcome', async () => {
    const { deleteMany, onStart, onSettled, refresh } = setup()
    await deleteMany([{ id: 'POL-1', type: 'policy', ref: 'POL-1@published' }, ...targets('POL-2')])

    expect(onStart).toHaveBeenCalledWith(['POL-1', 'POL-2'])
    expect(deleteEntityMock).toHaveBeenCalledWith('policy', 'POL-1@published')
    expect(deleteEntityMock).toHaveBeenCalledWith('policy', 'POL-2')
    expect(refresh).toHaveBeenCalledOnce()
    expect(onSettled).toHaveBeenCalledWith({ deleted: ['POL-1', 'POL-2'], failed: [] })
    expect(toasts()).toHaveLength(1)
    expect(toasts()[0]).toMatchObject({
      title: 'Deleted 2 policies',
      tone: 'success',
      duration: UNDO_TOAST_MS,
    })
  })

  it('uses the singular for one row and adds s when no plural is declared', async () => {
    const one = setup()
    await one.deleteMany(targets('POL-1'))
    expect(toasts()[0].title).toBe('Deleted 1 policy')

    useToasts().clear()
    const bare = setup({ singular: 'risk', plural: '' })
    await bare.deleteMany(targets('R-1', 'R-2'))
    expect(toasts()[0].title).toBe('Deleted 2 risks')
  })

  it('shows only an error when every delete fails, with no Undo', async () => {
    deleteEntityMock.mockRejectedValue(new Error('forbidden'))
    const { deleteMany, onSettled } = setup()
    await deleteMany(targets('POL-1', 'POL-2'))

    expect(onSettled).toHaveBeenCalledWith({ deleted: [], failed: ['POL-1', 'POL-2'] })
    expect(toasts()).toHaveLength(1)
    expect(toasts()[0]).toMatchObject({ title: 'Could not delete POL-1, POL-2: forbidden', tone: 'danger' })
    expect(toasts()[0].action).toBeUndefined()
  })

  it('names the first five failures and counts the rest', async () => {
    deleteEntityMock.mockRejectedValue(new Error('boom'))
    const { deleteMany } = setup()
    await deleteMany(targets('A', 'B', 'C', 'D', 'E', 'F', 'G'))
    expect(toasts()[0].title).toBe('Could not delete A, B, C, D, E and 2 more: boom')
  })

  it('Undo restores only the rows that were deleted', async () => {
    deleteEntityMock.mockImplementation((_t: string, id: string) =>
      id === 'POL-2' ? Promise.reject(new Error('locked')) : Promise.resolve()
    )
    const { deleteMany, refresh } = setup()
    await deleteMany(targets('POL-1', 'POL-2'))
    const undo = toasts().find((t) => t.action)!.action!

    undo.onAction()
    await vi.waitFor(() => expect(refresh).toHaveBeenCalledTimes(2))

    expect(restoreEntityMock).toHaveBeenCalledTimes(1)
    expect(restoreEntityMock).toHaveBeenCalledWith('policy', 'POL-1')
    expect(toasts().some((t) => t.title === 'Restored 1 policy')).toBe(true)
  })

  it('reports a restore failure that is not a 404 in the server\'s words', async () => {
    restoreEntityMock.mockRejectedValue(new Error('server down'))
    const { restoreMany } = setup()
    await restoreMany(targets('POL-1'))
    expect(toasts()[0]).toMatchObject({ title: 'Could not restore POL-1: server down', tone: 'danger' })
  })

  it('ignores a second delete while one is running', async () => {
    let resolve!: () => void
    deleteEntityMock.mockReturnValue(new Promise<void>((r) => (resolve = r)))
    const { deleteMany, deleting } = setup()
    const first = deleteMany(targets('POL-1'))
    expect(deleting.value).toBe(true)
    await deleteMany(targets('POL-2'))
    resolve()
    await first
    expect(deleteEntityMock).toHaveBeenCalledTimes(1)
    expect(deleting.value).toBe(false)
  })
})

describe('isTextEntryFocused', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  function focus(el: HTMLElement) {
    document.body.appendChild(el)
    el.focus()
  }

  function input(type: string) {
    const el = document.createElement('input')
    el.type = type
    return el
  }

  it.each(['text', 'search', 'email', 'number'])('is true for an input of type %s', (type) => {
    focus(input(type))
    expect(isTextEntryFocused()).toBe(true)
  })

  it.each(['checkbox', 'radio', 'button'])('is false for an input of type %s', (type) => {
    focus(input(type))
    expect(isTextEntryFocused()).toBe(false)
  })

  it('is true for a textarea', () => {
    focus(document.createElement('textarea'))
    expect(isTextEntryFocused()).toBe(true)
  })

  it('is false with nothing focused', () => {
    expect(isTextEntryFocused()).toBe(false)
  })
})
