import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import RruleBuilder from './RruleBuilder.vue'

function mountBuilder(modelValue = '', readonly = false) {
  return mount(RruleBuilder, {
    props: { modelValue, readonly },
  })
}

function getEmittedRrule(wrapper: ReturnType<typeof mount>) {
  const events = wrapper.emitted('update:modelValue') as string[][]
  return events?.[events.length - 1]?.[0] ?? ''
}

describe('RruleBuilder', () => {
  describe('yearly frequency with month/day picker', () => {
    it('shows month and day inputs when yearly is selected', async () => {
      const wrapper = mountBuilder('FREQ=YEARLY')
      await wrapper.vm.$nextTick()

      expect(wrapper.find('.rrule-builder__month').exists()).toBe(true)
      expect(wrapper.find('.rrule-builder__day-input').exists()).toBe(true)
    })

    it('hides month and day inputs for non-yearly frequencies', async () => {
      const wrapper = mountBuilder('FREQ=WEEKLY')
      await wrapper.vm.$nextTick()

      expect(wrapper.find('.rrule-builder__month').exists()).toBe(false)
      expect(wrapper.find('.rrule-builder__day-input').exists()).toBe(false)
    })

    it('emits BYMONTH and BYMONTHDAY when both are set', async () => {
      const wrapper = mountBuilder('FREQ=YEARLY;BYMONTH=3;BYMONTHDAY=15')
      await wrapper.vm.$nextTick()

      const rrule = getEmittedRrule(wrapper)
      expect(rrule).toContain('FREQ=YEARLY')
      expect(rrule).toContain('BYMONTH=3')
      expect(rrule).toContain('BYMONTHDAY=15')
    })

    it('emits plain YEARLY when neither month nor day is set', async () => {
      const wrapper = mountBuilder('FREQ=YEARLY')
      await wrapper.vm.$nextTick()

      const rrule = getEmittedRrule(wrapper)
      expect(rrule).toContain('FREQ=YEARLY')
      expect(rrule).not.toContain('BYMONTH')
      expect(rrule).not.toContain('BYMONTHDAY')
    })

    it('shows human-readable preview for yearly with month/day', async () => {
      const wrapper = mountBuilder('FREQ=YEARLY;BYMONTH=3;BYMONTHDAY=15')
      await wrapper.vm.$nextTick()

      const preview = wrapper.find('.rrule-builder__preview')
      expect(preview.exists()).toBe(true)
      expect(preview.text()).toContain('March')
      expect(preview.text()).toContain('15')
    })
  })

  describe('parsing yearly rules', () => {
    it('parses BYMONTH and BYMONTHDAY from existing rule', async () => {
      const wrapper = mountBuilder('FREQ=YEARLY;BYMONTH=7;BYMONTHDAY=4')
      await wrapper.vm.$nextTick()

      const monthSelect = wrapper.find('.rrule-builder__month')
        .element as HTMLSelectElement
      const dayInput = wrapper.find('.rrule-builder__day-input')
        .element as HTMLInputElement

      expect(monthSelect.value).toBe('7')
      expect(dayInput.value).toBe('4')
    })

    it('parses rule with RRULE: prefix', async () => {
      const wrapper = mountBuilder('RRULE:FREQ=YEARLY;BYMONTH=12;BYMONTHDAY=25')
      await wrapper.vm.$nextTick()

      const monthSelect = wrapper.find('.rrule-builder__month')
        .element as HTMLSelectElement
      const dayInput = wrapper.find('.rrule-builder__day-input')
        .element as HTMLInputElement

      expect(monthSelect.value).toBe('12')
      expect(dayInput.value).toBe('25')
    })

    it('round-trips a yearly birthday rule', async () => {
      const input = 'FREQ=YEARLY;BYMONTH=3;BYMONTHDAY=15'
      const wrapper = mountBuilder(input)
      await wrapper.vm.$nextTick()

      const rrule = getEmittedRrule(wrapper)
      expect(rrule).toContain('FREQ=YEARLY')
      expect(rrule).toContain('BYMONTH=3')
      expect(rrule).toContain('BYMONTHDAY=15')
    })
  })

  describe('day clamping per month', () => {
    it('clamps day when switching to a month with fewer days', async () => {
      const wrapper = mountBuilder('FREQ=YEARLY;BYMONTH=1;BYMONTHDAY=31')
      await wrapper.vm.$nextTick()

      // Verify initial state
      expect(getEmittedRrule(wrapper)).toContain('BYMONTHDAY=31')

      // Switch month to February via change event
      const monthSelect = wrapper.find('.rrule-builder__month')
      await monthSelect.setValue('2')
      await monthSelect.trigger('change')
      await wrapper.vm.$nextTick()

      const rrule = getEmittedRrule(wrapper)
      expect(rrule).toContain('BYMONTH=2')
      expect(rrule).toContain('BYMONTHDAY=28')
    })
  })

  describe('readonly mode', () => {
    it('disables month and day inputs when readonly', async () => {
      const wrapper = mountBuilder('FREQ=YEARLY;BYMONTH=3;BYMONTHDAY=15', true)
      await wrapper.vm.$nextTick()

      expect(
        (wrapper.find('.rrule-builder__month').element as HTMLSelectElement).disabled,
      ).toBe(true)
      expect(
        (wrapper.find('.rrule-builder__day-input').element as HTMLInputElement).disabled,
      ).toBe(true)
    })
  })

  describe('frequency-specific UI', () => {
    it('shows weekday buttons for weekly frequency', async () => {
      const wrapper = mountBuilder('FREQ=WEEKLY')
      await wrapper.vm.$nextTick()

      expect(wrapper.find('.rrule-builder__weekdays').exists()).toBe(true)
      expect(wrapper.find('.rrule-builder__yearly').exists()).toBe(false)
    })

    it('shows yearly picker, not weekdays, for yearly frequency', async () => {
      const wrapper = mountBuilder('FREQ=YEARLY')
      await wrapper.vm.$nextTick()

      expect(wrapper.find('.rrule-builder__yearly').exists()).toBe(true)
      expect(wrapper.find('.rrule-builder__weekdays').exists()).toBe(false)
    })
  })
})
