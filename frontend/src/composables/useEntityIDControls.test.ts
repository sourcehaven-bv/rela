import { describe, it, expect } from 'vitest'
import { nextTick, ref } from 'vue'
import { useEntityIDControls } from './useEntityIDControls'
import type { EntityType } from '@/types'

function et(partial: Partial<EntityType>): EntityType {
  return {
    label: 'Test',
    properties: {},
    ...partial,
  }
}

describe('useEntityIDControls', () => {
  describe('showManualIDInput', () => {
    it('is true for manual type in create mode', () => {
      const type = ref<EntityType | undefined>(et({ id_type: 'manual' }))
      const c = useEntityIDControls(type, 'create')
      expect(c.showManualIDInput.value).toBe(true)
    })

    it('is false for manual type in edit mode', () => {
      const type = ref<EntityType | undefined>(et({ id_type: 'manual' }))
      const c = useEntityIDControls(type, 'edit')
      expect(c.showManualIDInput.value).toBe(false)
    })

    it('is false for short type', () => {
      const type = ref<EntityType | undefined>(et({ id_type: 'short', id_prefix: 'TKT-' }))
      const c = useEntityIDControls(type, 'create')
      expect(c.showManualIDInput.value).toBe(false)
    })
  })

  describe('showPrefixPicker', () => {
    it('is false for single-prefix types', () => {
      const type = ref<EntityType | undefined>(et({ id_type: 'short', id_prefix: 'TKT-' }))
      const c = useEntityIDControls(type, 'create')
      expect(c.showPrefixPicker.value).toBe(false)
    })

    it('is true for multi-prefix non-manual types in create mode', () => {
      const type = ref<EntityType | undefined>(
        et({ id_type: 'short', id_prefixes: ['DEC-', 'ADR-'] })
      )
      const c = useEntityIDControls(type, 'create')
      expect(c.showPrefixPicker.value).toBe(true)
      expect(c.prefixOptions.value).toEqual(['DEC-', 'ADR-'])
    })

    it('is false for multi-prefix types in edit mode', () => {
      const type = ref<EntityType | undefined>(
        et({ id_type: 'short', id_prefixes: ['DEC-', 'ADR-'] })
      )
      const c = useEntityIDControls(type, 'edit')
      expect(c.showPrefixPicker.value).toBe(false)
    })

    it('is false for manual types even when id_prefixes has multiple entries', () => {
      const type = ref<EntityType | undefined>(
        et({ id_type: 'manual', id_prefixes: ['A-', 'B-'] })
      )
      const c = useEntityIDControls(type, 'create')
      expect(c.showPrefixPicker.value).toBe(false)
      expect(c.showManualIDInput.value).toBe(true)
    })

    it('is false when prefixOptions is empty', () => {
      const type = ref<EntityType | undefined>(et({ id_type: 'short' }))
      const c = useEntityIDControls(type, 'create')
      expect(c.showPrefixPicker.value).toBe(false)
    })
  })

  describe('prefixOptions', () => {
    it('falls back to id_prefix when id_prefixes is absent', () => {
      const type = ref<EntityType | undefined>(et({ id_prefix: 'TKT-' }))
      const c = useEntityIDControls(type, 'create')
      expect(c.prefixOptions.value).toEqual(['TKT-'])
    })

    it('returns empty when entity type has no prefixes', () => {
      const type = ref<EntityType | undefined>(et({}))
      const c = useEntityIDControls(type, 'create')
      expect(c.prefixOptions.value).toEqual([])
    })
  })

  describe('reset', () => {
    it('clears manual id and selects first prefix', () => {
      const type = ref<EntityType | undefined>(
        et({ id_type: 'short', id_prefixes: ['DEC-', 'ADR-'] })
      )
      const c = useEntityIDControls(type, 'create')
      c.manualId.value = 'stale'
      c.selectedPrefix.value = 'stale-'
      c.reset()
      expect(c.manualId.value).toBe('')
      expect(c.selectedPrefix.value).toBe('DEC-')
    })

    it('leaves selectedPrefix empty when no prefixes', () => {
      const type = ref<EntityType | undefined>(et({ id_type: 'manual' }))
      const c = useEntityIDControls(type, 'create')
      c.reset()
      expect(c.selectedPrefix.value).toBe('')
    })
  })

  describe('reactive mode', () => {
    it('responds to mode changes (create → edit hides picker)', async () => {
      const type = ref<EntityType | undefined>(
        et({ id_type: 'short', id_prefixes: ['DEC-', 'ADR-'] })
      )
      const mode = ref<'create' | 'edit'>('create')
      const c = useEntityIDControls(type, mode)
      expect(c.showPrefixPicker.value).toBe(true)

      mode.value = 'edit'
      expect(c.showPrefixPicker.value).toBe(false)
      expect(c.showManualIDInput.value).toBe(false)
      expect(c.buildPayloadFields()).toEqual({})
    })

    it('responds to mode changes (create → edit hides manual input)', async () => {
      const type = ref<EntityType | undefined>(et({ id_type: 'manual' }))
      const mode = ref<'create' | 'edit'>('create')
      const c = useEntityIDControls(type, mode)
      c.manualId.value = 'foo'
      expect(c.showManualIDInput.value).toBe(true)

      mode.value = 'edit'
      expect(c.showManualIDInput.value).toBe(false)
      expect(c.buildPayloadFields()).toEqual({})
    })
  })

  describe('buildPayloadFields', () => {
    it('includes manual id when manual-ID input is shown and filled', () => {
      const type = ref<EntityType | undefined>(et({ id_type: 'manual' }))
      const c = useEntityIDControls(type, 'create')
      c.manualId.value = 'custom-1'
      expect(c.buildPayloadFields()).toEqual({ id: 'custom-1' })
    })

    it('includes prefix when picker is shown and selected', () => {
      const type = ref<EntityType | undefined>(
        et({ id_type: 'short', id_prefixes: ['DEC-', 'ADR-'] })
      )
      const c = useEntityIDControls(type, 'create')
      c.selectedPrefix.value = 'ADR-'
      expect(c.buildPayloadFields()).toEqual({ prefix: 'ADR-' })
    })

    it('returns empty object for single-prefix create form', () => {
      const type = ref<EntityType | undefined>(et({ id_type: 'short', id_prefix: 'TKT-' }))
      const c = useEntityIDControls(type, 'create')
      expect(c.buildPayloadFields()).toEqual({})
    })

    it('does not include prefix for manual types even if selectedPrefix happens to be set', () => {
      const type = ref<EntityType | undefined>(et({ id_type: 'manual' }))
      const c = useEntityIDControls(type, 'create')
      c.selectedPrefix.value = 'X-'
      c.manualId.value = 'foo'
      expect(c.buildPayloadFields()).toEqual({ id: 'foo' })
    })
  })

  // Code-review #9: form components are reused across navigation; the
  // composable must self-clear stale state when the entity type changes.
  describe('auto-reset on entityType change', () => {
    it('clears manualId when switching from manual to short type', async () => {
      const type = ref<EntityType | undefined>(et({ id_type: 'manual' }))
      const c = useEntityIDControls(type, 'create')
      c.manualId.value = 'leftover'

      type.value = et({ id_type: 'short', id_prefix: 'TKT-' })
      await nextTick()
      expect(c.manualId.value).toBe('')
    })

    it('updates selectedPrefix to first of new prefix list', async () => {
      const type = ref<EntityType | undefined>(et({ id_type: 'short', id_prefixes: ['A-', 'B-'] }))
      const c = useEntityIDControls(type, 'create')
      c.selectedPrefix.value = 'B-'

      type.value = et({ id_type: 'short', id_prefixes: ['DEC-', 'ADR-'] })
      await nextTick()
      expect(c.selectedPrefix.value).toBe('DEC-')
    })

    it('does not reset when the same entity type object is reassigned', async () => {
      const same = et({ id_type: 'manual' })
      const type = ref<EntityType | undefined>(same)
      const c = useEntityIDControls(type, 'create')
      c.manualId.value = 'preserved'

      type.value = same
      await nextTick()
      expect(c.manualId.value).toBe('preserved')
    })
  })
})
