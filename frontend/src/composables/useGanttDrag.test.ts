import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import type { GanttNode } from '@/api'
import type { Entity } from '@/types/entity'
import { parseDay } from '@/utils/ganttLayout'
import { ApiError } from '@/api/errors'
import {
  canDrag,
  nodeAddress,
  shift,
  shiftStored,
  useGanttDrag,
  type GanttDates,
  type GanttDragDeps,
} from './useGanttDrag'

const getEntityMock = vi.fn()
const updateEntityMock = vi.fn()
vi.mock('@/api', async (orig) => ({
  ...(await orig<typeof import('@/api')>()),
  getEntity: (...args: unknown[]) => getEntityMock(...args),
  updateEntity: (...args: unknown[]) => updateEntityMock(...args),
}))

const DATES: GanttDates = { start: 'start', end: 'end', startKind: 'date', endKind: 'date' }

function httpError(status: number, message: string) {
  return new ApiError(message, { kind: 'http', status, original: null })
}
const PX = 10

const node: GanttNode = {
  id: 'STEP-1',
  type: 'step',
  face: 'draft',
  title: 'Build',
  planned: { start: '2026-03-02', end: '2026-03-06' },
}
const day = (s: string) => parseDay(s)!

function stored(over: Partial<Entity> = {}): Entity {
  const entity: Entity = {
    id: node.id,
    type: node.type,
    properties: { start: node.planned!.start, end: node.planned!.end },
    _versions: { properties: { start: 'v-start', end: 'v-end' }, content: 'v-c' },
    ...over,
  }
  return entity
}

function setup(depsOver: Partial<GanttDragDeps> = {}) {
  const deps: GanttDragDeps = {
    datesOf: () => DATES,
    pxPerDay: () => PX,
    refresh: vi.fn().mockResolvedValue(undefined),
    error: vi.fn(),
    warning: vi.fn(),
    ...depsOver,
  }
  let drag!: ReturnType<typeof useGanttDrag>
  const wrapper = mount(
    defineComponent({
      setup() {
        drag = useGanttDrag(deps)
        return () => h('div')
      },
    })
  )
  return { drag, deps, wrapper }
}

function pointer(type: string, clientX: number, el = document.createElement('div')) {
  const e = new MouseEvent(type, { clientX, button: 0 }) as PointerEvent
  Object.defineProperty(e, 'pointerId', { value: 1 })
  Object.defineProperty(e, 'currentTarget', { value: el })
  return e
}

/** Grants the verdict for `node` the way a hover would. */
async function grant(drag: ReturnType<typeof useGanttDrag>, entity = stored()) {
  getEntityMock.mockResolvedValueOnce(entity)
  drag.probe(node, true)
  await vi.waitFor(() => expect(drag.draggable(node)).toBe(true))
}

/** Drags from x=100 by dx and releases. */
function dragBy(drag: ReturnType<typeof useGanttDrag>, mode: 'move' | 'start' | 'end', dx: number) {
  drag.onPointerDown(pointer('pointerdown', 100), node, mode)
  drag.onPointerMove(pointer('pointermove', 100 + dx))
  drag.onPointerUp(pointer('pointerup', 100 + dx))
}

beforeEach(() => {
  getEntityMock.mockReset()
  updateEntityMock.mockReset().mockResolvedValue({})
})
afterEach(() => vi.useRealTimers())

describe('shift', () => {
  it.each([
    ['move', 2, { start: 12, end: 16 }],
    ['start', 2, { start: 12, end: 14 }],
    ['end', -2, { start: 10, end: 12 }],
    ['start', 9, { start: 14, end: 14 }],
    ['end', -9, { start: 10, end: 10 }],
  ] as const)('%s by %i', (mode, delta, want) => {
    expect(shift({ start: 10, end: 14 }, mode, delta)).toEqual(want)
  })
})

describe('canDrag', () => {
  it.each([
    ['allowed', stored(), true],
    ['update refused', stored({ _actions: { update: false } } as Partial<Entity>), false],
    ['date read-only', stored({ _fields: { end: { writable: false } } } as Partial<Entity>), false],
    ['date redacted', stored({ _redacted: ['start'] }), false],
    ['date not a date', stored({ properties: { start: 'soon', end: '2026-03-06' } }), false],
    ['date empty', stored({ properties: { start: '2026-03-02' } }), false],
  ])('%s', (_name, entity, want) => {
    expect(canDrag(entity, DATES)).toBe(want)
  })
})

describe('nodeAddress', () => {
  it('addresses the face the chart shows', () => {
    expect(nodeAddress(node)).toBe('STEP-1@draft')
    expect(nodeAddress({ ...node, face: undefined })).toBe('STEP-1')
  })
})

describe('useGanttDrag verdict', () => {
  it('reads the entity once per address after the pointer rests', async () => {
    vi.useFakeTimers()
    const { drag } = setup()
    getEntityMock.mockResolvedValue(stored())
    drag.probe(node)
    drag.probe(node)
    expect(getEntityMock).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(150)
    expect(getEntityMock).toHaveBeenCalledTimes(1)
    expect(getEntityMock).toHaveBeenCalledWith('step', 'STEP-1@draft')
    expect(drag.draggable(node)).toBe(true)
    drag.probe(node)
    await vi.advanceTimersByTimeAsync(150)
    expect(getEntityMock).toHaveBeenCalledTimes(1)
  })

  it('skips the read when the pointer leaves first', async () => {
    vi.useFakeTimers()
    const { drag } = setup()
    drag.probe(node)
    drag.unprobe()
    await vi.advanceTimersByTimeAsync(300)
    expect(getEntityMock).not.toHaveBeenCalled()
  })

  it('never reads a node without both own dates, or a source mapping one', () => {
    const { drag } = setup({ datesOf: () => null })
    drag.probe(node, true)
    setup().drag.probe({ ...node, planned: { start: '2026-03-02' } }, true)
    expect(getEntityMock).not.toHaveBeenCalled()
  })

  it('offers no drag after a refusal and does not ask again', async () => {
    const { drag } = setup()
    getEntityMock.mockRejectedValueOnce(httpError(403, 'no'))
    drag.probe(node, true)
    await vi.waitFor(() => expect(getEntityMock).toHaveBeenCalled())
    await flushPromises()
    expect(drag.draggable(node)).toBe(false)
    drag.onPointerDown(pointer('pointerdown', 100), node, 'move')
    drag.onPointerMove(pointer('pointermove', 100 + 3 * PX))
    expect(drag.preview.value).toBeNull()
    drag.probe(node, true)
    expect(getEntityMock).toHaveBeenCalledTimes(1)
  })

  it('asks again after a failed read', async () => {
    const { drag } = setup()
    getEntityMock.mockRejectedValueOnce(httpError(502, 'down'))
    drag.probe(node, true)
    await flushPromises()
    expect(drag.draggable(node)).toBe(false)
    await grant(drag)
    expect(getEntityMock).toHaveBeenCalledTimes(2)
  })

  it('drops a verdict read before a reset', async () => {
    const { drag } = setup()
    let answer!: (e: Entity) => void
    getEntityMock.mockReturnValueOnce(new Promise<Entity>((r) => (answer = r)))
    drag.probe(node, true)
    drag.reset()
    answer(stored())
    await flushPromises()
    expect(drag.draggable(node)).toBe(false)
  })
})

describe('useGanttDrag gesture', () => {
  it('moves both dates by whole days in one PATCH with preconditions', async () => {
    const { drag, deps } = setup()
    await grant(drag)
    getEntityMock.mockResolvedValueOnce(stored())
    dragBy(drag, 'move', 2 * PX + 3)
    await vi.waitFor(() => expect(deps.refresh).toHaveBeenCalledWith('step'))
    expect(updateEntityMock).toHaveBeenCalledTimes(1)
    expect(updateEntityMock).toHaveBeenCalledWith('step', 'STEP-1@draft', {
      properties: { start: '2026-03-04', end: '2026-03-08' },
      preconditions: { properties: { start: 'v-start', end: 'v-end' } },
    })
    expect(drag.preview.value).toBeNull()
  })

  it('keeps the preview until the reload shows the result', async () => {
    let finish!: () => void
    const { drag } = setup({ refresh: () => new Promise<void>((r) => (finish = r)) })
    await grant(drag)
    getEntityMock.mockResolvedValueOnce(stored())
    dragBy(drag, 'end', 3 * PX)
    await vi.waitFor(() => expect(updateEntityMock).toHaveBeenCalled())
    await vi.waitFor(() => expect(finish).toBeTypeOf('function'))
    expect(drag.preview.value).toMatchObject({
      id: node.id,
      start: day('2026-03-02'),
      end: day('2026-03-09'),
    })
    finish()
    await vi.waitFor(() => expect(drag.preview.value).toBeNull())
  })

  it('writes only the edge that moved', async () => {
    const { drag, deps } = setup()
    await grant(drag)
    getEntityMock.mockResolvedValueOnce(stored())
    dragBy(drag, 'start', -PX)
    await vi.waitFor(() => expect(deps.refresh).toHaveBeenCalled())
    expect(updateEntityMock.mock.calls[0][2]).toEqual({
      properties: { start: '2026-03-01' },
      preconditions: { properties: { start: 'v-start' } },
    })
  })

  it('shifts a datetime in its own offset, keeping its time', async () => {
    const { drag, deps } = setup({
      datesOf: () => ({ ...DATES, startKind: 'datetime', endKind: 'datetime' }),
    })
    // Near midnight in a non-UTC offset: shifting in another frame would
    // land on a different day than the chart draws.
    const raw = stored({
      properties: { start: '2026-03-02T00:30:00+01:00', end: '2026-03-06T23:30:00-05:00' },
    })
    await grant(drag, raw)
    getEntityMock.mockResolvedValueOnce(raw)
    dragBy(drag, 'move', PX)
    await vi.waitFor(() => expect(deps.refresh).toHaveBeenCalled())
    expect(updateEntityMock.mock.calls[0][2].properties).toEqual({
      start: '2026-03-03T00:30:00+01:00',
      end: '2026-03-07T23:30:00-05:00',
    })
  })

  it('treats a press below the threshold as a click', async () => {
    const { drag } = setup()
    await grant(drag)
    dragBy(drag, 'move', 3)
    const click = new MouseEvent('click', { cancelable: true })
    drag.onClickCapture(click)
    expect(click.defaultPrevented).toBe(false)
    expect(drag.preview.value).toBeNull()
    expect(updateEntityMock).not.toHaveBeenCalled()
  })

  it('swallows the click a drag ends with', async () => {
    const { drag } = setup()
    await grant(drag)
    getEntityMock.mockResolvedValueOnce(stored())
    dragBy(drag, 'move', PX)
    const click = new MouseEvent('click', { cancelable: true })
    drag.onClickCapture(click)
    expect(click.defaultPrevented).toBe(true)
  })

  it('sends nothing for a drag that lands where it started', async () => {
    const { drag } = setup()
    await grant(drag)
    dragBy(drag, 'move', 4)
    expect(updateEntityMock).not.toHaveBeenCalled()
    expect(drag.preview.value).toBeNull()
  })

  it('abandons the gesture on Escape and on cancel', async () => {
    const { drag } = setup()
    await grant(drag)
    drag.onPointerDown(pointer('pointerdown', 100), node, 'move')
    drag.onPointerMove(pointer('pointermove', 100 + 3 * PX))
    expect(drag.preview.value).not.toBeNull()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(drag.preview.value).toBeNull()
    // The button is still down: releasing it neither writes nor drills.
    drag.onPointerUp(pointer('pointerup', 100 + 3 * PX))
    const click = new MouseEvent('click', { cancelable: true })
    drag.onClickCapture(click)
    expect(click.defaultPrevented).toBe(true)

    drag.onPointerDown(pointer('pointerdown', 100), node, 'move')
    drag.onPointerMove(pointer('pointermove', 100 + 3 * PX))
    drag.cancel()
    expect(drag.preview.value).toBeNull()
    expect(updateEntityMock).not.toHaveBeenCalled()
  })

  it.each([
    ['403', httpError(403, 'You may not edit this')],
    ['412', httpError(412, 'Changed by someone else')],
  ])('reports a refused write (%s) and reloads the server dates', async (_n, err) => {
    const { drag, deps } = setup()
    await grant(drag)
    getEntityMock.mockResolvedValueOnce(stored())
    updateEntityMock.mockRejectedValueOnce(err)
    dragBy(drag, 'move', PX)
    await vi.waitFor(() => expect(deps.refresh).toHaveBeenCalled())
    expect(deps.error).toHaveBeenCalledWith(expect.stringContaining('Could not reschedule Build'))
    expect(drag.preview.value).toBeNull()
  })

  it('refuses at commit when the fresh read no longer allows the write', async () => {
    const { drag, deps } = setup()
    await grant(drag)
    getEntityMock.mockResolvedValueOnce(stored({ _actions: { update: false } } as Partial<Entity>))
    dragBy(drag, 'move', PX)
    await vi.waitFor(() => expect(deps.refresh).toHaveBeenCalled())
    expect(updateEntityMock).not.toHaveBeenCalled()
    expect(deps.error).toHaveBeenCalledWith('You can no longer change the dates of Build.')
  })

  it('shows warnings from a successful write', async () => {
    const { drag, deps } = setup()
    await grant(drag)
    getEntityMock.mockResolvedValueOnce(stored())
    updateEntityMock.mockResolvedValueOnce({
      warnings: [{ code: 'x', path: 'end', detail: 'ends after its plan' }],
    })
    dragBy(drag, 'move', PX)
    await vi.waitFor(() =>
      expect(deps.warning).toHaveBeenCalledWith(expect.stringContaining('ends after its plan'))
    )
  })
})

describe('useGanttDrag keyboard', () => {
  const key = (k: string, shiftKey = false) =>
    new KeyboardEvent('keydown', { key: k, shiftKey, cancelable: true })

  it('previews with arrows and writes once on Enter', async () => {
    const { drag, deps } = setup()
    await grant(drag)
    drag.onHandleKey(key('ArrowRight'), node, 'end')
    // The view hands back the node as drawn, with the preview applied.
    const drawn = (end: string) => ({ ...node, planned: { start: '2026-03-02', end } })
    drag.onHandleKey(key('ArrowRight', true), drawn('2026-03-07'), 'end')
    expect(updateEntityMock).not.toHaveBeenCalled()
    expect(drag.preview.value?.end).toBe(day('2026-03-14'))
    getEntityMock.mockResolvedValueOnce(stored())
    drag.onHandleKey(key('Enter'), drawn('2026-03-14'), 'end')
    await vi.waitFor(() => expect(deps.refresh).toHaveBeenCalled())
    expect(updateEntityMock).toHaveBeenCalledTimes(1)
    expect(updateEntityMock.mock.calls[0][2].properties).toEqual({ end: '2026-03-14' })
  })

  it('drops the preview on Escape and on blur', async () => {
    const { drag } = setup()
    await grant(drag)
    drag.onHandleKey(key('ArrowLeft'), node, 'move')
    drag.onHandleKey(key('Escape'), node, 'move')
    expect(drag.preview.value).toBeNull()
    drag.onHandleKey(key('ArrowLeft'), node, 'move')
    drag.onHandleBlur(node, false)
    expect(drag.preview.value).toBeNull()
  })
})

describe('shiftStored', () => {
  it.each([
    ['2026-03-02', 2, 'date', '2026-03-04'],
    ['2026-03-31', 1, 'date', '2026-04-01'],
    // An unquoted YAML date arrives as a datetime string; a date stays a date.
    ['2026-03-02T00:00:00Z', 1, 'date', '2026-03-03'],
    ['2026-03-28T00:30:00+01:00', 2, 'datetime', '2026-03-30T00:30:00+01:00'],
    ['soon', 1, 'date', null],
  ] as const)('%s by %i (%s)', (value, delta, kind, want) => {
    expect(shiftStored(value, delta, kind)).toBe(want)
  })
})

describe('useGanttDrag review regressions', () => {
  const key = (k: string) => new KeyboardEvent('keydown', { key: k, cancelable: true })
  /** The node as the view draws it while a preview is held. */
  const drawn = (start: string, end: string): GanttNode => ({ ...node, planned: { start, end } })

  it('continues a pointer drag from a held keyboard preview', async () => {
    const { drag, deps } = setup()
    await grant(drag)
    drag.onHandleKey(key('ArrowRight'), node, 'move')
    drag.onHandleKey(key('ArrowRight'), drawn('2026-03-03', '2026-03-07'), 'move')
    const shown = drawn('2026-03-04', '2026-03-08')
    getEntityMock.mockResolvedValueOnce(stored())
    drag.onPointerDown(pointer('pointerdown', 100), shown, 'move')
    drag.onPointerMove(pointer('pointermove', 100 + PX))
    drag.onPointerUp(pointer('pointerup', 100 + PX))
    await vi.waitFor(() => expect(deps.refresh).toHaveBeenCalled())
    expect(updateEntityMock.mock.calls[0][2].properties).toEqual({
      start: '2026-03-05',
      end: '2026-03-09',
    })
  })

  it('refuses a write when a date changed since the chart loaded', async () => {
    const { drag, deps } = setup()
    await grant(drag)
    getEntityMock.mockResolvedValueOnce(
      stored({ properties: { start: '2026-04-01', end: '2026-04-05' } })
    )
    dragBy(drag, 'move', 2 * PX)
    await vi.waitFor(() => expect(deps.refresh).toHaveBeenCalled())
    expect(updateEntityMock).not.toHaveBeenCalled()
    expect(deps.error).toHaveBeenCalledWith(expect.stringContaining('changed by someone else'))
  })

  it('keeps the handles through a successful write, and drops them on a 403', async () => {
    const { drag, deps } = setup()
    await grant(drag)
    getEntityMock.mockResolvedValueOnce(stored())
    dragBy(drag, 'move', PX)
    await vi.waitFor(() => expect(deps.refresh).toHaveBeenCalledTimes(1))
    await flushPromises()
    expect(drag.draggable(node)).toBe(true)

    getEntityMock.mockResolvedValueOnce(stored())
    updateEntityMock.mockRejectedValueOnce(httpError(403, 'no'))
    dragBy(drag, 'move', PX)
    await vi.waitFor(() => expect(deps.refresh).toHaveBeenCalledTimes(2))
    await flushPromises()
    expect(drag.draggable(node)).toBe(false)
  })

  it('saves start and end in one write when focus moves between them', async () => {
    const { drag, deps } = setup()
    await grant(drag)
    drag.onHandleKey(key('ArrowLeft'), node, 'start')
    drag.onHandleBlur(node, true)
    drag.onHandleKey(key('ArrowRight'), drawn('2026-03-01', '2026-03-06'), 'end')
    getEntityMock.mockResolvedValueOnce(stored())
    drag.onHandleKey(key('Enter'), drawn('2026-03-01', '2026-03-07'), 'end')
    await vi.waitFor(() => expect(deps.refresh).toHaveBeenCalled())
    expect(updateEntityMock).toHaveBeenCalledTimes(1)
    expect(updateEntityMock.mock.calls[0][2].properties).toEqual({
      start: '2026-03-01',
      end: '2026-03-07',
    })
  })

  it('cancel drops a held keyboard preview', async () => {
    const { drag } = setup()
    await grant(drag)
    drag.onHandleKey(key('ArrowRight'), node, 'move')
    drag.cancel()
    expect(drag.preview.value).toBeNull()
  })

  it('neither reloads nor reports after the view is gone', async () => {
    const { drag, deps, wrapper } = setup()
    await grant(drag)
    let answer!: (e: Entity) => void
    getEntityMock.mockReturnValueOnce(new Promise<Entity>((r) => (answer = r)))
    dragBy(drag, 'move', PX)
    wrapper.unmount()
    answer(stored())
    await flushPromises()
    expect(updateEntityMock).not.toHaveBeenCalled()
    expect(deps.refresh).not.toHaveBeenCalled()
    expect(deps.error).not.toHaveBeenCalled()
  })
})
