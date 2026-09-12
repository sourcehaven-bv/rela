import { describe, it, expect } from 'vitest'
import { commandsCtx } from '@milkdown/kit/core'
import { unavailableCommandIds } from './commandAvailability'
import type { EditorCommand } from './editorCommands'
import type { Ctx } from '@milkdown/kit/ctx'
import type { EditorState } from '@milkdown/kit/prose/state'

/**
 * A ctx stub whose command manager answers from a fixed map. The real dry run
 * is exercised against a live editor in MilkdownEditor.test.ts; this covers
 * the selection logic around it, which is where the reasoning lives.
 */
function ctxWith(answers: Record<string, boolean>): Ctx {
  const manager = {
    get: (name: string) => {
      if (!(name in answers)) return undefined
      return () => () => answers[name]
    },
  }
  const get = (slice: unknown) => {
    if (slice !== commandsCtx) throw new Error('unexpected slice')
    return manager
  }
  // Object.assign rather than an object-literal assertion, which the
  // consistent-type-assertions rule flags. The stub implements only `get`,
  // which is all the module under test reaches for.
  return Object.assign(Object.create(null) as Ctx, { get })
}

const state = Object.create(null) as EditorState

const cmd = (over: Partial<EditorCommand>): EditorCommand => ({
  id: 'x',
  label: 'X',
  command: 'Forward',
  probe: { kind: 'none' },
  keywords: [],
  ...over,
})

describe('unavailableCommandIds', () => {
  it('reports an inapplicable command', () => {
    const out = unavailableCommandIds(
      ctxWith({ Forward: false }),
      state,
      [cmd({})],
      new Set(),
      true
    )
    expect([...out]).toEqual(['x'])
  })

  it('reports nothing when the command applies', () => {
    const out = unavailableCommandIds(ctxWith({ Forward: true }), state, [cmd({})], new Set(), true)
    expect([...out]).toEqual([])
  })

  // The important one: an active command is judged by the inverse, because
  // that is what pressing it runs. Probing forward would grey out exactly the
  // button that removes the formatting.
  it('judges an active command by its inverse, not its forward direction', () => {
    const c = cmd({ toggleTo: 'Back' })
    const ctx = ctxWith({ Forward: false, Back: true })
    expect([...unavailableCommandIds(ctx, state, [c], new Set(['x']), true)]).toEqual([])
  })

  it('disables an active command whose inverse cannot apply either', () => {
    const c = cmd({ toggleTo: 'Back' })
    const ctx = ctxWith({ Forward: true, Back: false })
    expect([...unavailableCommandIds(ctx, state, [c], new Set(['x']), true)]).toEqual(['x'])
  })

  it("takes the caller's answer for the lift inverse, which is not a slice", () => {
    const c = cmd({ toggleTo: 'lift' })
    const ctx = ctxWith({ Forward: false })
    expect([...unavailableCommandIds(ctx, state, [c], new Set(['x']), true)]).toEqual([])
    expect([...unavailableCommandIds(ctx, state, [c], new Set(['x']), false)]).toEqual(['x'])
  })

  it('disables an unregistered command rather than letting it throw on press', () => {
    const out = unavailableCommandIds(
      ctxWith({}),
      state,
      [cmd({ command: 'Missing' })],
      new Set(),
      true
    )
    expect([...out]).toEqual(['x'])
  })

  it('passes the payload through, so heading levels are probed independently', () => {
    const seen: unknown[] = []
    const ctx = {
      get: () => ({
        get: () => (payload: unknown) => {
          seen.push(payload)
          return () => true
        },
      }),
    } as unknown as Ctx
    unavailableCommandIds(ctx, state, [cmd({ payload: 2 })], new Set(), true)
    expect(seen).toEqual([2])
  })
})
