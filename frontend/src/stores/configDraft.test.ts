import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { draftStorageKey, useConfigDraftStore } from './configDraft'
import { snapshot } from '@/configure/__tests__/fixture'
import { get, getIn, keys, set, isMap, mapAt, newMap } from '@/configure/tree'

function fresh() {
  setActivePinia(createPinia())
  const store = useConfigDraftStore()
  store.load(snapshot())
  return store
}

function renameFeature(store: ReturnType<typeof useConfigDraftStore>, label: string) {
  store.edit('schema', (tree) => {
    const feature = getIn(tree, ['entities', 'feature'])
    if (isMap(feature)) set(feature, 'label', label)
  })
}

describe('useConfigDraftStore', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => vi.restoreAllMocks())

  it('counts changes and drops a draft edited back to the base', () => {
    const store = fresh()
    expect(store.hasDraft).toBe(false)
    expect(store.count).toBe(0)

    renameFeature(store, 'Epic')
    expect(store.hasDraft).toBe(true)
    expect(store.count).toBe(1)
    expect(store.changes[0].title).toBe('Epic')

    renameFeature(store, 'Feature')
    expect(store.schema).toBeNull()
    expect(store.hasDraft).toBe(false)
    expect(store.count).toBe(0)
  })

  it('never edits the base it was loaded with', () => {
    const store = fresh()
    renameFeature(store, 'Epic')
    expect(get(getIn(store.base?.schema, ['entities', 'feature']), 'label')).toBe('Feature')
    expect(get(getIn(store.currentSchema, ['entities', 'feature']), 'label')).toBe('Epic')
  })

  it('keeps the draft across a reload of the page', () => {
    const store = fresh()
    renameFeature(store, 'Epic')
    store.setMigrationTitle('Call features epics')
    expect(localStorage.getItem(draftStorageKey())).not.toBeNull()

    const again = fresh()
    expect(again.count).toBe(1)
    expect(again.migrationTitle).toBe('Call features epics')
    expect(again.stale).toBe(false)
  })

  it('marks a kept draft stale when the configuration moved on', () => {
    const store = fresh()
    renameFeature(store, 'Epic')

    setActivePinia(createPinia())
    const later = useConfigDraftStore()
    later.load(snapshot('v2'))
    expect(later.stale).toBe(true)
    expect(later.body().base_version).toBe('v1')
  })

  it('discards everything at once', () => {
    const store = fresh()
    renameFeature(store, 'Epic')
    store.renameProperty('ticket', 'effort', 'estimate')
    store.setValueMapping({ entity_type: 'ticket', property: 'status', from: 'doing', to: 'open' })
    store.discardAll()

    expect(store.hasDraft).toBe(false)
    expect(store.count).toBe(0)
    expect(store.renames).toEqual([])
    expect(store.valueMappings).toEqual([])
    expect(localStorage.getItem(draftStorageKey())).toBeNull()
  })

  it('records one rename for a chain, and none for a rename back', () => {
    const store = fresh()
    store.renameProperty('ticket', 'effort', 'estimate')
    store.renameProperty('ticket', 'estimate', 'size')
    expect(store.renames).toEqual([{ entity_type: 'ticket', from: 'effort', to: 'size' }])
    expect(keys(getIn(store.currentSchema, ['entities', 'ticket', 'properties']))).toContain('size')

    store.renameProperty('ticket', 'size', 'effort')
    expect(store.renames).toEqual([])
    expect(store.hasDraft).toBe(false)
  })

  it('renames a property on the screens that show it', () => {
    const store = fresh()
    store.renameProperty('ticket', 'effort', 'estimate')
    const fields = getIn(store.currentDataEntry, ['forms', 'new_ticket', 'fields'])
    expect(get(getIn(fields, [2]), 'property')).toBe('estimate')
    expect(
      get(getIn(store.currentDataEntry, ['kanbans', 'board', 'card', 'fields', 0]), 'property')
    ).toBe('estimate')
  })

  it('sends only what changed', () => {
    const store = fresh()
    renameFeature(store, 'Epic')
    const body = store.body()
    expect(body.base_version).toBe('v1')
    expect(body.schema).toBeDefined()
    expect(body.data_entry).toBeUndefined()
    expect(body.renames).toBeUndefined()
  })

  it('edits by path, creating mappings where needed', () => {
    const store = fresh()
    store.setAt('screens', ['styles', 'priority'], 'high', 'red')
    expect(get(getIn(store.currentDataEntry, ['styles', 'priority']), 'high')).toBe('red')
    store.setAt('screens', ['styles', 'priority'], 'high', undefined)
    expect(getIn(store.currentDataEntry, ['styles', 'priority'])).toBeUndefined()
    expect(store.dataEntry).toBeNull()

    store.moveAt('screens', ['forms', 'new_ticket', 'fields'], 2, 0)
    expect(
      get(getIn(store.currentDataEntry, ['forms', 'new_ticket', 'fields', 0]), 'property')
    ).toBe('effort')
  })

  it('edits settings of list items, keeping their origin index', () => {
    const store = fresh()
    store.setAt('schema', ['validations', 0], 'severity', 'warning')
    const rule = mapAt(store.currentSchema, ['validations', 0])
    expect(get(rule, 'severity')).toBe('warning')
    expect(rule?.$i).toBe(0)

    store.moveAt('screens', ['forms', 'new_ticket', 'fields'], 2, 0)
    store.setAt('screens', ['forms', 'new_ticket', 'fields', 0], 'label', 'Estimate')
    const field = mapAt(store.currentDataEntry, ['forms', 'new_ticket', 'fields', 0])
    expect(get(field, 'property')).toBe('effort')
    expect(get(field, 'label')).toBe('Estimate')
    expect(field?.$i).toBe(2)

    store.setAt('schema', ['automations', 0, 'on'], 'becomes', 'open')
    expect(get(mapAt(store.currentSchema, ['automations', 0, 'on']), 'becomes')).toBe('open')

    store.appendAt('schema', ['automations', 0], 'do', newMap({ set: 'status', value: 'x' }))
    expect(getIn(store.currentSchema, ['automations', 0, 'do', 1])).toBeDefined()
    store.removeAt('schema', ['automations', 0, 'do'], 0)
    const actions = getIn(store.currentSchema, ['automations', 0, 'do'])
    expect(get(getIn(actions, [0]), 'set')).toBe('status')
    expect(mapAt(store.currentSchema, ['automations', 0])?.$i).toBe(0)

    store.setAt('screens', ['forms', 'new_ticket', 'fields', 9], 'label', 'Ghost')
    expect(getIn(store.currentDataEntry, ['forms', 'new_ticket', 'fields', 9])).toBeUndefined()
  })

  it('stores an explicit false where the configuration had the setting', () => {
    const store = fresh()
    store.setAt('schema', ['entities', 'ticket', 'properties', 'title'], 'required', false)
    expect(
      get(getIn(store.currentSchema, ['entities', 'ticket', 'properties', 'title']), 'required')
    ).toBe(false)

    const other = fresh()
    other.setAt('screens', ['forms', 'new_ticket', 'fields', 0], 'hidden', true)
    other.setAt('screens', ['forms', 'new_ticket', 'fields', 0], 'hidden', false)
    expect(other.dataEntry).toBeNull()
  })

  it('starts a fresh version when a kept draft turns out to be the configuration', () => {
    const store = fresh()
    renameFeature(store, 'Epic')

    setActivePinia(createPinia())
    const later = useConfigDraftStore()
    const next = snapshot('v2')
    const feature = getIn(next.schema, ['entities', 'feature'])
    if (isMap(feature)) set(feature, 'label', 'Epic')
    later.load(next)
    expect(later.hasDraft).toBe(false)
    expect(later.draftVersion).toBe('v2')

    later.setAt('schema', ['entities', 'ticket'], 'label', 'Task')
    expect(later.stale).toBe(false)
    expect(later.body().base_version).toBe('v2')
  })

  it('shows a draft another tab wrote', () => {
    const tab = fresh()
    setActivePinia(createPinia())
    const other = useConfigDraftStore()
    other.load(snapshot())
    renameFeature(other, 'Epic')

    window.dispatchEvent(
      new StorageEvent('storage', {
        key: draftStorageKey(),
        newValue: localStorage.getItem(draftStorageKey()),
      })
    )
    expect(tab.count).toBe(1)
    expect(get(getIn(tab.currentSchema, ['entities', 'feature']), 'label')).toBe('Epic')

    other.discardAll()
    window.dispatchEvent(new StorageEvent('storage', { key: draftStorageKey(), newValue: null }))
    expect(tab.hasDraft).toBe(false)
  })

  it('keeps the newer draft in memory when storing it failed', () => {
    const store = fresh()
    renameFeature(store, 'Epic')
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('quota')
    })
    renameFeature(store, 'Initiative')
    store.load(snapshot())
    expect(get(getIn(store.currentSchema, ['entities', 'feature']), 'label')).toBe('Initiative')
  })

  it('cuts the migration title to 120 bytes without splitting a character', () => {
    const store = fresh()
    renameFeature(store, 'Epic')
    store.setMigrationTitle('é'.repeat(130))
    const title = store.body().migration_title ?? ''
    expect(new TextEncoder().encode(title).length).toBeLessThanOrEqual(120)
    expect(title).toBe('é'.repeat(60))

    store.setMigrationTitle('a' + '😀'.repeat(60))
    expect(store.body().migration_title).toBe('a' + '😀'.repeat(29))
  })
})
