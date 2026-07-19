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

  describe('custom-type keyed styles (BUG-28Y0Y2)', () => {
    // The server keys the styles map by custom-type name, not property name.
    function withCustomType() {
      const schemaStore = useSchemaStore()
      schemaStore.entityTypes = new Map([
        ['ticket', { label: 'Ticket', properties: { status: { type: 'ticket-status' } } }],
      ]) as never
      schemaStore.customTypes = new Map([
        ['ticket-status', { values: ['todo', 'doing', 'done'] }],
      ]) as never
      return schemaStore
    }

    it('resolves the color via the custom-type name when property ≠ type', () => {
      const schemaStore = withCustomType()
      schemaStore.styles = { 'ticket-status': { todo: 'badge-yellow' } }

      const wrapper = mount(Badge, {
        props: { value: 'todo', property: 'status' },
      })

      expect(wrapper.find('.badge').classes()).toContain('badge--yellow')
    })

    it('prefers the type-keyed styles over property-keyed ones', () => {
      const schemaStore = withCustomType()
      schemaStore.styles = {
        'ticket-status': { todo: 'badge-yellow' },
        status: { todo: 'badge-red' },
      }

      const wrapper = mount(Badge, {
        props: { value: 'todo', property: 'status' },
      })

      expect(wrapper.find('.badge').classes()).toContain('badge--yellow')
    })

    it('uses the entityType prop to disambiguate same-named properties', () => {
      const schemaStore = useSchemaStore()
      schemaStore.entityTypes = new Map([
        ['bug', { label: 'Bug', properties: { status: { type: 'bug-status' } } }],
        ['ticket', { label: 'Ticket', properties: { status: { type: 'ticket-status' } } }],
      ]) as never
      schemaStore.customTypes = new Map([
        ['bug-status', { values: ['open'] }],
        ['ticket-status', { values: ['open'] }],
      ]) as never
      schemaStore.styles = {
        'bug-status': { open: 'badge-red' },
        'ticket-status': { open: 'badge-blue' },
      }

      const wrapper = mount(Badge, {
        props: { value: 'open', property: 'status', entityType: 'ticket' },
      })

      expect(wrapper.find('.badge').classes()).toContain('badge--blue')
    })

    it('still normalizes the value when resolving through the type key', () => {
      const schemaStore = withCustomType()
      schemaStore.styles = { 'ticket-status': { in_progress: 'badge-orange' } }

      const wrapper = mount(Badge, {
        props: { value: 'In Progress', property: 'status' },
      })

      expect(wrapper.find('.badge').classes()).toContain('badge--orange')
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

  describe('enum labels', () => {
    // Register an entity type whose `status` property is an inline enum with a
    // label for in_progress, so getEnumLabel resolves via the property def.
    function withInlineLabel() {
      const schemaStore = useSchemaStore()
      schemaStore.entityTypes = new Map([
        [
          'ticket',
          {
            label: 'Ticket',
            properties: {
              status: {
                type: 'enum',
                values: ['in_progress', 'done'],
                labels: { in_progress: 'In Progress' },
              },
            },
          },
        ],
      ]) as never
      return schemaStore
    }

    it('renders the label when the metamodel configures one', () => {
      withInlineLabel()
      const wrapper = mount(Badge, {
        props: { value: 'in_progress', property: 'status' },
      })
      expect(wrapper.text()).toBe('In Progress')
    })

    it('suppresses CSS capitalize for a labeled value (author chose the casing)', () => {
      withInlineLabel()
      const wrapper = mount(Badge, {
        props: { value: 'in_progress', property: 'status' },
      })
      expect(wrapper.find('.badge').classes()).toContain('badge--labeled')
    })

    it('keeps color styling keyed on the raw value, not the label', () => {
      const schemaStore = withInlineLabel()
      schemaStore.styles = { status: { in_progress: 'badge-orange' } }
      const wrapper = mount(Badge, {
        props: { value: 'in_progress', property: 'status' },
      })
      // Text is the label; color still resolves from the value key.
      expect(wrapper.text()).toBe('In Progress')
      expect(wrapper.find('.badge').classes()).toContain('badge--orange')
    })

    it('falls back to the raw value (with capitalize) when no label configured', () => {
      withInlineLabel()
      const wrapper = mount(Badge, {
        props: { value: 'done', property: 'status' },
      })
      expect(wrapper.text()).toBe('done')
      // No label → capitalize stays on (badge--labeled absent).
      expect(wrapper.find('.badge').classes()).not.toContain('badge--labeled')
    })

    it('resolves labels from a referenced custom type', () => {
      const schemaStore = useSchemaStore()
      schemaStore.entityTypes = new Map([
        [
          'ticket',
          { label: 'Ticket', properties: { priority: { type: 'priority_t' } } },
        ],
      ]) as never
      schemaStore.customTypes = new Map([
        ['priority_t', { values: ['hi'], labels: { hi: 'High' } }],
      ]) as never
      const wrapper = mount(Badge, {
        props: { value: 'hi', property: 'priority' },
      })
      expect(wrapper.text()).toBe('High')
    })

    it('escapes label text (no HTML injection from a metamodel label)', () => {
      const schemaStore = useSchemaStore()
      schemaStore.entityTypes = new Map([
        [
          'ticket',
          {
            label: 'Ticket',
            properties: {
              status: { type: 'enum', values: ['x'], labels: { x: '<img src=x>' } },
            },
          },
        ],
      ]) as never
      const wrapper = mount(Badge, {
        props: { value: 'x', property: 'status' },
      })
      // Rendered as text, not parsed into an element.
      expect(wrapper.find('img').exists()).toBe(false)
      expect(wrapper.text()).toBe('<img src=x>')
    })
  })
})
