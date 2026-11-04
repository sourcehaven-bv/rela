/**
 * Page scoping of the next-action surface.
 *
 * The card is mounted once for the whole app, so the composable must
 * re-resolve per page, keep the suggestion on screen across a navigation, and
 * never let a slow answer for the previous page overwrite the current one.
 */
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { ref, nextTick } from 'vue'
import { useNextAction, __resetNextActionForTest, REFRESH_DEBOUNCE_MS } from './useNextAction'
import { getNextAction, sendNextActionFeedback } from '@/api'
import type { NextActionResponse, NextActionSuggestion } from '@/types'

const mockRoute = ref<{
  name?: string
  params: Record<string, string>
  query: Record<string, unknown>
}>({
  name: 'dashboard',
  params: {},
  query: {},
})

vi.mock('vue-router', () => ({
  useRoute: () => ({
    get name() {
      return mockRoute.value.name
    },
    get params() {
      return mockRoute.value.params
    },
    get query() {
      return mockRoute.value.query
    },
  }),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}))

vi.mock('@/api', async () => {
  const actual = await vi.importActual<typeof import('@/api')>('@/api')
  return { ...actual, getNextAction: vi.fn(), sendNextActionFeedback: vi.fn() }
})

const mockGet = vi.mocked(getNextAction)
const mockFeedback = vi.mocked(sendNextActionFeedback)

function suggestion(over: Partial<NextActionSuggestion> = {}): NextActionSuggestion {
  return { source: 'stale', band: 'stalled', entity_id: 'TASK-1', message: 'Still on it?', ...over }
}

function navigate(name: string, params: Record<string, string> = {}) {
  mockRoute.value = { name, params, query: {} }
}

async function settle() {
  await nextTick()
  await Promise.resolve()
  await nextTick()
}

describe('useNextAction page scoping', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    __resetNextActionForTest()
    navigate('dashboard')
    mockGet.mockResolvedValue({ suggestion: suggestion() })
    mockFeedback.mockResolvedValue(undefined)
  })

  it('sends the page the route names', async () => {
    navigate('entity', { type: 'task', id: 'TASK-9' })
    await useNextAction().loadOnce()
    expect(mockGet).toHaveBeenCalledWith(
      undefined,
      { placement: { kind: 'entity', view: 'task', entity: 'TASK-9' }, current: undefined },
      expect.any(AbortSignal)
    )
  })

  // Mutation: drop `current` on a navigation. The previous page reported the
  // impression, so the server would hide that suggestion under its cooldown
  // and the card would change on every click.
  it('re-resolves on navigation and asks to keep the suggestion on screen', async () => {
    const na = useNextAction()
    await na.loadOnce()
    navigate('list', { id: 'open' })
    await settle()
    expect(mockGet).toHaveBeenCalledTimes(2)
    expect(mockGet).toHaveBeenLastCalledWith(
      undefined,
      {
        placement: { kind: 'list', view: 'open' },
        current: { source: 'stale', entity_id: 'TASK-1', variant: undefined },
      },
      expect.any(AbortSignal)
    )
  })

  it('does not re-resolve for the same page', async () => {
    const na = useNextAction()
    await na.loadOnce()
    await na.loadOnce()
    navigate('dashboard')
    await settle()
    expect(mockGet).toHaveBeenCalledTimes(1)
  })

  // After feedback the reader has answered the suggestion; asking the server
  // to keep it would be wrong.
  it('does not ask to keep the suggestion after feedback', async () => {
    const na = useNextAction()
    await na.loadOnce()
    await na.respond('dismiss')
    expect(mockGet).toHaveBeenLastCalledWith(
      undefined,
      expect.objectContaining({ current: undefined }),
      expect.any(AbortSignal)
    )
  })

  // Mutation: drop the sequence guard. The list page's slow answer then
  // lands after the entity page's and shows a suggestion for the wrong page.
  it('ignores a slow answer for a page the reader has left', async () => {
    let resolveFirst!: (r: NextActionResponse) => void
    mockGet.mockImplementationOnce(() => new Promise((r) => (resolveFirst = r)))
    mockGet.mockResolvedValueOnce({
      suggestion: suggestion({ source: 'here', entity_id: 'TASK-2' }),
    })

    const na = useNextAction()
    const first = na.loadOnce()
    const firstSignal = mockGet.mock.calls[0][2] as AbortSignal
    navigate('entity', { type: 'task', id: 'TASK-2' })
    await settle()
    expect(firstSignal.aborted).toBe(true)

    resolveFirst({ suggestion: suggestion({ source: 'stale-list' }) })
    await first
    expect(na.suggestion.value?.source).toBe('here')
  })

  // The server names the type whose changes matter (`context_type`); the
  // client refreshes on that type alone, debounced.
  describe('entity changes', () => {
    beforeEach(() => vi.useFakeTimers())
    afterEach(() => vi.useRealTimers())

    async function onTaskPage() {
      navigate('entity', { type: 'task', id: 'TASK-1' })
      mockGet.mockResolvedValue({ suggestion: suggestion(), context_type: 'task' })
      const na = useNextAction()
      await na.loadOnce()
      return na
    }

    it('re-resolves once after a burst of changes of the context type', async () => {
      const na = await onTaskPage()
      na.refreshForChange('task')
      na.refreshForChange('task')
      na.refreshForChange('task')
      expect(mockGet).toHaveBeenCalledTimes(1)
      await vi.advanceTimersByTimeAsync(REFRESH_DEBOUNCE_MS)
      expect(mockGet).toHaveBeenCalledTimes(2)
    })

    it('ignores a change to another type', async () => {
      const na = await onTaskPage()
      na.refreshForChange('project')
      await vi.advanceTimersByTimeAsync(REFRESH_DEBOUNCE_MS)
      expect(mockGet).toHaveBeenCalledTimes(1)
    })

    // Mutation: refresh whenever the page has an entity. Most projects have
    // no context source, and every open entity page would then resolve on
    // every change of its type.
    it('ignores changes when the server names no context type', async () => {
      navigate('entity', { type: 'task', id: 'TASK-1' })
      const na = useNextAction()
      await na.loadOnce()
      na.refreshForChange('task')
      await vi.advanceTimersByTimeAsync(REFRESH_DEBOUNCE_MS)
      expect(mockGet).toHaveBeenCalledTimes(1)
    })

    // Mutation: drop forgetAnswered from acknowledge. The refresh then sends
    // the acknowledged suggestion as current and the server keeps it.
    it('does not ask to keep a suggestion the reader acknowledged', async () => {
      const na = await onTaskPage()
      na.acknowledge()
      na.refreshForChange('task')
      await vi.advanceTimersByTimeAsync(REFRESH_DEBOUNCE_MS)
      expect(mockGet).toHaveBeenLastCalledWith(
        undefined,
        expect.objectContaining({ current: undefined }),
        expect.any(AbortSignal)
      )
    })
  })

  // Mutation: send only the suggestion on screen. Returning to the list then
  // sends the kanban's suggestion, which the list cannot show, and the list's
  // own suggestion is in cooldown, so the list moves on every round trip.
  it('asks each page to keep the suggestion it last showed', async () => {
    navigate('list', { id: 'open' })
    mockGet.mockResolvedValueOnce({ suggestion: suggestion({ source: 'on-list', entity_id: 'X' }) })
    const na = useNextAction()
    await na.loadOnce()
    mockGet.mockResolvedValueOnce({
      suggestion: suggestion({ source: 'on-board', entity_id: 'Y' }),
    })
    navigate('kanban', { id: 'board' })
    await settle()
    navigate('list', { id: 'open' })
    await settle()
    expect(mockGet).toHaveBeenLastCalledWith(
      undefined,
      expect.objectContaining({
        current: { source: 'on-list', entity_id: 'X', variant: undefined },
      }),
      expect.any(AbortSignal)
    )
  })

  // Mutation: keep loadedKey after a failure. The page then never retries
  // until the reader leaves it.
  it('tries again on the same page after a failed load', async () => {
    const err = vi.spyOn(console, 'error').mockImplementation(() => {})
    mockGet.mockRejectedValueOnce(new Error('offline'))
    const na = useNextAction()
    await na.loadOnce()
    await na.loadOnce()
    expect(mockGet).toHaveBeenCalledTimes(2)
    expect(na.suggestion.value?.source).toBe('stale')
    err.mockRestore()
  })
})
