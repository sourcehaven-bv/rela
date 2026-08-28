import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import CalendarEventChip from './CalendarEventChip.vue'
import type { CalendarDay } from '@/utils/calendarGrid'

const d = (day: number): CalendarDay => ({ year: 2026, month: 8, day })

function chip(startDay: number, endDay: number, renderedOn: number, extra = {}) {
  return mount(CalendarEventChip, {
    props: {
      event: {
        id: '0:task:T-1',
        entity: { id: 'T-1', type: 'task', properties: {}, relations: {} },
        entityType: 'task',
        summary: 'Conference',
        startDay: d(startDay),
        endDay: d(endDay),
        timed: false,
        timeLabel: '',
        minutes: null,
        color: 'blue',
        fields: [],
        dateProperty: 'due',
        dateKind: 'date',
        sourceIndex: 0,
        ...extra,
      } as never,
      day: d(renderedOn),
      draggable: false,
    },
  })
}

/**
 * A spanning event is drawn as one chip per day it covers. Without a marker,
 * three consecutive identical chips read as three separate events — a
 * three-day conference becomes indistinguishable from a daily standup.
 */
describe('multi-day span markers', () => {
  it('shows no marker for a single-day event', () => {
    expect(chip(20, 20, 20).find('.calendar-chip-span').exists()).toBe(false)
  })

  it.each([
    [20, 'start', '→', 'Continues on following days'],
    [21, 'middle', '↔', 'Continues before and after this day'],
    [22, 'end', '←', 'Continued from earlier days'],
  ])('marks day %i as the %s of the span', (renderedOn, _segment, marker, description) => {
    const marker_el = chip(20, 22, renderedOn).find('.calendar-chip-span')
    expect(marker_el.exists()).toBe(true)
    expect(marker_el.text()).toBe(marker)
    // The arrow is decorative; screen readers get the sentence.
    expect(marker_el.attributes('aria-label')).toBe(description)
  })

  it('shows a two-day event as start then end, with no middle', () => {
    expect(chip(20, 21, 20).find('.calendar-chip-span').text()).toBe('→')
    expect(chip(20, 21, 21).find('.calendar-chip-span').text()).toBe('←')
  })

  /**
   * The span is clamped to the visible window only when walking days, not on
   * the event itself — so an event that began before the period still reads as
   * a continuation on the first visible day rather than claiming to start there.
   */
  it('marks a continuation when the real start is outside the window', () => {
    // Event runs 10-25 August; the chip is rendered on the 20th.
    expect(chip(10, 25, 20).find('.calendar-chip-span').text()).toBe('↔')
    // On its real last day it closes, even though its start is far behind.
    expect(chip(10, 25, 25).find('.calendar-chip-span').text()).toBe('←')
  })

  it('keeps the marker for a timed multi-day event alongside its time', () => {
    const w = chip(20, 22, 20, { timed: true, timeLabel: '09:00', minutes: 540 })
    expect(w.find('.calendar-chip-time').text()).toBe('09:00')
    expect(w.find('.calendar-chip-span').text()).toBe('→')
  })
})
