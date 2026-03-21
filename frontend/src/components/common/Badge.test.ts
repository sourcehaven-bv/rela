import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import Badge from './Badge.vue'
import { useSchemaStore } from '@/stores/schema'

describe('Badge', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

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

  describe('schema-based colors', () => {
    it('uses schema store styles when available', () => {
      const schemaStore = useSchemaStore()
      schemaStore.styles = {
        status: {
          open: 'badge-blue',
          done: 'badge-green',
          pending: 'badge-orange',
        },
      }

      const wrapper = mount(Badge, {
        props: { value: 'open' },
      })

      const style = wrapper.find('.badge').attributes('style')
      // badge-blue uses text color #60a5fa
      expect(style).toContain('color: #60a5fa')
    })

    it('uses property-specific styles when property is provided', () => {
      const schemaStore = useSchemaStore()
      schemaStore.styles = {
        priority: {
          high: 'badge-red',
        },
        status: {
          high: 'badge-orange', // Different color for same value in different property
        },
      }

      const wrapper = mount(Badge, {
        props: { value: 'high', property: 'priority' },
      })

      const style = wrapper.find('.badge').attributes('style')
      // badge-red uses text color #f87171
      expect(style).toContain('color: #f87171')
    })

    it('falls back to searching all properties when property not specified', () => {
      const schemaStore = useSchemaStore()
      schemaStore.styles = {
        status: {
          done: 'badge-green',
        },
      }

      const wrapper = mount(Badge, {
        props: { value: 'done' },
      })

      const style = wrapper.find('.badge').attributes('style')
      // badge-green uses text color #4ade80
      expect(style).toContain('color: #4ade80')
    })
  })

  describe('fallback color', () => {
    it('uses CSS variable fallback for unknown values', () => {
      const wrapper = mount(Badge, {
        props: { value: 'unknown-status' },
      })

      const style = wrapper.find('.badge').attributes('style')
      expect(style).toContain('var(--hover-bg)')
      expect(style).toContain('var(--muted-text)')
    })

    it('uses CSS variable fallback when schema has no matching style', () => {
      const schemaStore = useSchemaStore()
      schemaStore.styles = {
        status: {
          open: 'badge-blue',
        },
      }

      const wrapper = mount(Badge, {
        props: { value: 'closed' },
      })

      const style = wrapper.find('.badge').attributes('style')
      expect(style).toContain('var(--hover-bg)')
    })
  })

  describe('value normalization', () => {
    beforeEach(() => {
      const schemaStore = useSchemaStore()
      schemaStore.styles = {
        status: {
          in_progress: 'badge-orange',
        },
      }
    })

    it('handles uppercase values', () => {
      const schemaStore = useSchemaStore()
      schemaStore.styles = {
        status: {
          open: 'badge-blue',
        },
      }

      const wrapper = mount(Badge, {
        props: { value: 'OPEN' },
      })

      const style = wrapper.find('.badge').attributes('style')
      // badge-blue uses text color #60a5fa
      expect(style).toContain('color: #60a5fa')
    })

    it('handles underscores in value lookup', () => {
      const wrapper = mount(Badge, {
        props: { value: 'in_progress' },
      })

      const style = wrapper.find('.badge').attributes('style')
      // badge-orange uses text color #fb923c
      expect(style).toContain('color: #fb923c')
    })

    it('converts spaces to underscores for lookup', () => {
      const wrapper = mount(Badge, {
        props: { value: 'in progress' },
      })

      const style = wrapper.find('.badge').attributes('style')
      // badge-orange uses text color #fb923c
      expect(style).toContain('color: #fb923c')
    })

    it('handles mixed case with underscores', () => {
      const wrapper = mount(Badge, {
        props: { value: 'In_Progress' },
      })

      const style = wrapper.find('.badge').attributes('style')
      // badge-orange uses text color #fb923c
      expect(style).toContain('color: #fb923c')
    })
  })

  describe('badge class to style mapping', () => {
    // Note: jsdom doesn't support color-mix so we test text colors instead
    it.each([
      ['badge-blue', '#60a5fa'],
      ['badge-purple', '#a78bfa'],
      ['badge-green', '#4ade80'],
      ['badge-red', '#f87171'],
      ['badge-orange', '#fb923c'],
      ['badge-yellow', '#facc15'],
    ])('maps %s to correct text color %s', (badgeClass, textColor) => {
      const schemaStore = useSchemaStore()
      schemaStore.styles = {
        test: {
          value: badgeClass,
        },
      }

      const wrapper = mount(Badge, {
        props: { value: 'value' },
      })

      const style = wrapper.find('.badge').attributes('style')
      expect(style).toContain(`color: ${textColor}`)
    })

    it('maps badge-gray to CSS variables', () => {
      const schemaStore = useSchemaStore()
      schemaStore.styles = {
        test: {
          value: 'badge-gray',
        },
      }

      const wrapper = mount(Badge, {
        props: { value: 'value' },
      })

      const style = wrapper.find('.badge').attributes('style')
      expect(style).toContain('var(--hover-bg)')
      expect(style).toContain('var(--muted-text)')
    })
  })
})
