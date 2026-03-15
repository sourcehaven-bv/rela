import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import Badge from './Badge.vue'

describe('Badge', () => {
  describe('rendering', () => {
    it('renders the value text', () => {
      const wrapper = mount(Badge, {
        props: { value: 'open' },
      })

      expect(wrapper.text()).toBe('open')
    })

    it('renders with badge class', () => {
      const wrapper = mount(Badge, {
        props: { value: 'draft' },
      })

      expect(wrapper.find('.badge').exists()).toBe(true)
    })
  })

  describe('status colors', () => {
    it.each([
      ['open', '#3b82f6'],
      ['in-progress', '#f59e0b'],
      ['done', '#10b981'],
      ['closed', '#6b7280'],
      ['draft', '#94a3b8'],
      ['pending', '#f59e0b'],
      ['approved', '#10b981'],
      ['rejected', '#ef4444'],
      ['blocked', '#ef4444'],
      ['ready', '#3b82f6'],
      ['accepted', '#10b981'],
      ['deprecated', '#6b7280'],
    ])('applies correct background color for status "%s"', (status, expectedColor) => {
      const wrapper = mount(Badge, {
        props: { value: status },
      })

      const style = wrapper.find('.badge').attributes('style')
      expect(style).toContain(`background-color: ${expectedColor}`)
    })
  })

  describe('priority colors', () => {
    it.each([
      ['low', '#94a3b8'],
      ['medium', '#3b82f6'],
      ['high', '#f59e0b'],
      ['critical', '#ef4444'],
      ['urgent', '#ef4444'],
    ])('applies correct background color for priority "%s"', (priority, expectedColor) => {
      const wrapper = mount(Badge, {
        props: { value: priority },
      })

      const style = wrapper.find('.badge').attributes('style')
      expect(style).toContain(`background-color: ${expectedColor}`)
    })
  })

  describe('boolean colors', () => {
    it.each([
      ['yes', '#10b981'],
      ['no', '#ef4444'],
      ['true', '#10b981'],
      ['false', '#ef4444'],
    ])('applies correct background color for boolean "%s"', (bool, expectedColor) => {
      const wrapper = mount(Badge, {
        props: { value: bool },
      })

      const style = wrapper.find('.badge').attributes('style')
      expect(style).toContain(`background-color: ${expectedColor}`)
    })
  })

  describe('fallback color', () => {
    it('uses fallback gray for unknown values', () => {
      const wrapper = mount(Badge, {
        props: { value: 'unknown-status' },
      })

      const style = wrapper.find('.badge').attributes('style')
      expect(style).toContain('background-color: #6b7280')
    })
  })

  describe('text color contrast', () => {
    it('uses white text on dark backgrounds', () => {
      // Blue background (#3b82f6) - dark enough for white text
      const wrapper = mount(Badge, {
        props: { value: 'open' },
      })

      const style = wrapper.find('.badge').attributes('style')
      expect(style).toContain('color: #ffffff')
    })

    it('uses dark text on light backgrounds', () => {
      // Light gray (#94a3b8) - light enough for dark text
      const wrapper = mount(Badge, {
        props: { value: 'draft' },
      })

      const style = wrapper.find('.badge').attributes('style')
      expect(style).toContain('color: #1e293b')
    })
  })

  describe('value normalization', () => {
    it('handles uppercase values', () => {
      const wrapper = mount(Badge, {
        props: { value: 'OPEN' },
      })

      const style = wrapper.find('.badge').attributes('style')
      expect(style).toContain('background-color: #3b82f6')
    })

    it('handles underscores as hyphens', () => {
      const wrapper = mount(Badge, {
        props: { value: 'in_progress' },
      })

      const style = wrapper.find('.badge').attributes('style')
      expect(style).toContain('background-color: #f59e0b')
    })

    it('handles spaces as hyphens', () => {
      const wrapper = mount(Badge, {
        props: { value: 'in progress' },
      })

      const style = wrapper.find('.badge').attributes('style')
      expect(style).toContain('background-color: #f59e0b')
    })

    it('handles mixed case with underscores', () => {
      const wrapper = mount(Badge, {
        props: { value: 'In_Progress' },
      })

      const style = wrapper.find('.badge').attributes('style')
      expect(style).toContain('background-color: #f59e0b')
    })
  })
})
