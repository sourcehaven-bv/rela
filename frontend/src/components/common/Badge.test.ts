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
    it('uses schema store styles when available (explicit property)', () => {
      const schemaStore = useSchemaStore()
      schemaStore.styles = {
        status: {
          open: 'badge-blue',
          done: 'badge-green',
          pending: 'badge-orange',
        },
      }

      const wrapper = mount(Badge, {
        props: { value: 'open', property: 'status' },
      })

      expect(wrapper.find('.badge').classes()).toContain('badge--blue')
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

      expect(wrapper.find('.badge').classes()).toContain('badge--red')
    })

    it('returns the gray fallback when property is absent (RR-UD2D removed the cross-property scan)', () => {
      const schemaStore = useSchemaStore()
      schemaStore.styles = {
        status: {
          done: 'badge-green',
        },
      }

      const wrapper = mount(Badge, {
        props: { value: 'done' },
      })

      // Pre-refactor this scanned every property looking for a match.
      // Removed (RR-UD2D) because the scan was non-deterministic when
      // the same value was styled under multiple properties.
      expect(wrapper.find('.badge').classes()).toContain('badge--gray')
    })
  })

  describe('fallback color', () => {
    it('uses gray class for unknown values', () => {
      const wrapper = mount(Badge, {
        props: { value: 'unknown-status' },
      })

      expect(wrapper.find('.badge').classes()).toContain('badge--gray')
    })

    it('uses gray class when schema has no matching style', () => {
      const schemaStore = useSchemaStore()
      schemaStore.styles = {
        status: {
          open: 'badge-blue',
        },
      }

      const wrapper = mount(Badge, {
        props: { value: 'closed' },
      })

      expect(wrapper.find('.badge').classes()).toContain('badge--gray')
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
        props: { value: 'OPEN', property: 'status' },
      })

      expect(wrapper.find('.badge').classes()).toContain('badge--blue')
    })

    it('handles underscores in value lookup', () => {
      const wrapper = mount(Badge, {
        props: { value: 'in_progress', property: 'status' },
      })

      expect(wrapper.find('.badge').classes()).toContain('badge--orange')
    })

    it('converts spaces to underscores for lookup', () => {
      const wrapper = mount(Badge, {
        props: { value: 'in progress', property: 'status' },
      })

      expect(wrapper.find('.badge').classes()).toContain('badge--orange')
    })

    it('handles mixed case with underscores', () => {
      const wrapper = mount(Badge, {
        props: { value: 'In_Progress', property: 'status' },
      })

      expect(wrapper.find('.badge').classes()).toContain('badge--orange')
    })
  })

  describe('badge class to style mapping', () => {
    it.each([
      ['badge-blue', 'badge--blue'],
      ['badge-purple', 'badge--purple'],
      ['badge-green', 'badge--green'],
      ['badge-red', 'badge--red'],
      ['badge-orange', 'badge--orange'],
      ['badge-yellow', 'badge--yellow'],
    ])('maps %s to CSS class %s', (badgeClass, cssClass) => {
      const schemaStore = useSchemaStore()
      schemaStore.styles = {
        test: {
          value: badgeClass,
        },
      }

      const wrapper = mount(Badge, {
        props: { value: 'value', property: 'test' },
      })

      expect(wrapper.find('.badge').classes()).toContain(cssClass)
    })

    it('maps badge-gray to badge--gray CSS class', () => {
      const schemaStore = useSchemaStore()
      schemaStore.styles = {
        test: {
          value: 'badge-gray',
        },
      }

      const wrapper = mount(Badge, {
        props: { value: 'value', property: 'test' },
      })

      expect(wrapper.find('.badge').classes()).toContain('badge--gray')
    })
  })
})
