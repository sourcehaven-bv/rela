/**
 * World scoping of the next-action surface.
 *
 * `?world=` on the GET supplies the DISPLAY world: the server checks it
 * against each source's `visible_worlds:` allow list. Without it that list
 * cannot work in the UI at all, because the server would only ever see the
 * default world.
 *
 * The world never reaches the candidate query — that is `source_world:`,
 * operator config, and nothing here sends or offers it. The feedback POST
 * carries no world either: it is a write, and the endpoint refuses one.
 */
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { ref, nextTick } from 'vue'
import { useNextAction, __resetNextActionForTest } from './useNextAction'
import { getNextAction, sendNextActionFeedback } from '@/api'
import type { NextActionSuggestion } from '@/types'

const mockRouteQuery = ref<Record<string, unknown>>({})

vi.mock('vue-router', () => ({
  useRoute: () => ({
    get query() {
      return mockRouteQuery.value
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

describe('useNextAction world scoping', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    __resetNextActionForTest()
    mockRouteQuery.value = {}
    mockGet.mockResolvedValue({ suggestion: suggestion() })
    mockFeedback.mockResolvedValue(undefined)
  })

  // Mutation: drop the world argument from the GET. The server then only ever
  // sees the default world, and `visible_worlds:` can never match anything
  // else — the allow list becomes inert in the UI.
  it('sends the browsed world as the display world', async () => {
    mockRouteQuery.value = { world: 'editorial' }
    const na = useNextAction()

    await na.loadOnce()

    expect(mockGet).toHaveBeenCalledWith('editorial')
  })

  // undefined rather than '' or 'default', so no empty ?world= is emitted —
  // matching worldParam's contract on every other surface.
  it('sends no world in the default world', async () => {
    const na = useNextAction()

    await na.loadOnce()

    expect(mockGet).toHaveBeenCalledWith(undefined)
  })

  it('re-issues the request when the world changes', async () => {
    mockRouteQuery.value = { world: 'editorial' }
    const na = useNextAction()
    await na.loadOnce()
    expect(mockGet).toHaveBeenCalledTimes(1)

    mockGet.mockResolvedValue({ suggestion: suggestion({ source: 'published-only' }) })
    mockRouteQuery.value = { world: 'published' }
    await nextTick()
    await vi.waitFor(() => expect(mockGet).toHaveBeenCalledTimes(2))

    expect(mockGet).toHaveBeenLastCalledWith('published')
    // ...and the new world's answer replaces the old one, rather than the
    // previous world's suggestion being left on screen.
    expect(na.suggestion.value?.source).toBe('published-only')
  })

  // The latch is per world, not per session: two surfaces render the same
  // suggestion, so a repeat mount in the SAME world must not resolve again
  // (that would double the impression and could show two suggestions at once).
  it('still resolves only once per world across consumers', async () => {
    mockRouteQuery.value = { world: 'editorial' }
    const a = useNextAction()
    const b = useNextAction()

    await Promise.all([a.loadOnce(), b.loadOnce()])
    await a.loadOnce()

    expect(mockGet).toHaveBeenCalledTimes(1)
  })

  describe('feedback', () => {
    // A write, not a read. `?world=` on the POST is refused (world_read_only),
    // and the decision it records is not world-scoped anyway.
    it.each(['dismiss', 'snooze', 'shown'] as const)(
      'sends no world on a %s POST',
      async (kind) => {
        mockRouteQuery.value = { world: 'editorial' }
        const na = useNextAction()
        await na.loadOnce()

        if (kind === 'shown') await na.markShown()
        else await na.respond(kind)

        expect(mockFeedback).toHaveBeenCalled()
        for (const [body] of mockFeedback.mock.calls) {
          expect(body).not.toHaveProperty('world')
          expect(Object.keys(body)).not.toContain('world')
        }
      }
    )

    // Feedback is followed by a re-resolve, which must stay in the world the
    // user is standing in rather than falling back to the default.
    it('re-resolves in the same world after feedback', async () => {
      mockRouteQuery.value = { world: 'editorial' }
      const na = useNextAction()
      await na.loadOnce()

      await na.respond('dismiss')

      expect(mockGet).toHaveBeenCalledTimes(2)
      expect(mockGet).toHaveBeenLastCalledWith('editorial')
    })
  })
})
