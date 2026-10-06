/**
 * Editing an entity page's own entity from the page header: the `badge:`
 * property as a picker, and a menu with Details, Open and Delete. The title
 * is edited in Details, not in place, so a click on it changes nothing.
 *
 * Each write names only the property it changes (a PATCH), so a property the
 * reader cannot see is never carried along and cannot be lost. Writability
 * is the server's: `_actions.update` for the entity, `_fields` for each
 * property. A reader who may not write gets the same header, read-only.
 */
import { computed, h, type ComputedRef, type Slot } from 'vue'
import { useRouter } from 'vue-router'
import { useQueryCache } from '@pinia/colada'
import { deleteEntity, updateEntity } from '@/api/entities'
import { entityKeys } from '@/queries/entities'
import { useConfirm } from '@/composables/useConfirm'
import { useDetailPanel } from '@/composables/useDetailPanel'
import { useUIStore } from '@/stores'
import { useSchemaStore } from '@/stores/schema'
import { useSpaceStore } from '@/stores/space'
import { isFieldWritable, isPropertyRedacted } from '@/utils/affordances'
import { entityDisplayTitle } from '@/utils/entityDisplay'
import { entityRef, refFace } from '@/utils/entityRef'
import { defaultRegistry } from '@/widgets/registry'
import EntityDetailPanel from '@/components/entity/EntityDetailPanel.vue'
import EntityPageMenu from '@/components/pages/EntityPageMenu.vue'
import InlinePropertyValue from '@/components/forms/InlinePropertyValue.vue'
import type { Entity } from '@/types'

export function useEntityPageEditing(options: {
  entity: ComputedRef<Entity | undefined>
  entityType: ComputedRef<string | undefined>
  /** The page's `badge:` property, if it has one. */
  badgeProperty: ComputedRef<string | undefined>
}) {
  const router = useRouter()
  const queryCache = useQueryCache()
  const schemaStore = useSchemaStore()
  const spaceStore = useSpaceStore()
  const uiStore = useUIStore()
  const panel = useDetailPanel()
  const { confirm } = useConfirm()

  const typeDef = computed(() =>
    options.entityType.value ? schemaStore.getEntityType(options.entityType.value) : undefined
  )
  const canUpdate = computed(() => options.entity.value?._actions?.update !== false)
  const canDelete = computed(() => options.entity.value?._actions?.delete === true)

  /** Whether the reader may change this one property of the entity. */
  function writable(property: string): boolean {
    const entity = options.entity.value
    if (!entity || !canUpdate.value) return false
    // A redacted property has no `_fields` verdict, which would read as writable.
    if (isPropertyRedacted(property, entity._redacted)) return false
    return isFieldWritable(entity._fields?.[property])
  }

  async function save(property: string, value: unknown) {
    const entity = options.entity.value
    const type = options.entityType.value
    if (!entity || !type) return
    const key = entityKeys.detail(type, entity.id)
    // Shown at once; the refetch below replaces it with what the server holds.
    queryCache.setQueryData(key, {
      ...entity,
      properties: { ...entity.properties, [property]: value },
    })
    const empty = value === undefined || value === null || value === ''
    try {
      await updateEntity(
        type,
        entity.id,
        empty ? { properties_unset: [property] } : { properties: { [property]: value } }
      )
    } catch (err) {
      uiStore.error(err instanceof Error ? err.message : 'Failed to save')
    }
    void queryCache.invalidateQueries({ key })
    void queryCache.invalidateQueries({ key: entityKeys.list(type) })
  }

  /*
   * The badge, drawn by the widget the registry picks for it. A writable one
   * is a picker, as the same property is on the detail page; an empty one
   * then still shows, so a value can be set.
   */
  const badge = computed((): Slot | undefined => {
    const property = options.badgeProperty.value
    const entity = options.entity.value
    const type = options.entityType.value
    if (!property || !entity || !type) return undefined
    const value = entity.properties[property]
    const canWrite = writable(property)
    if (!canWrite && (value === undefined || value === null || value === '')) return undefined
    const propertyDef = typeDef.value?.properties[property]
    const widget = defaultRegistry.resolve(undefined, propertyDef)
    return () => [
      h(InlinePropertyValue, {
        property,
        label: property,
        widget,
        value,
        writable: canWrite,
        propertyDef,
        entityType: type,
        entityId: entity.id,
        onUpdate: (next: unknown) => void save(property, next),
      }),
    ]
  })

  function fullPage(entity: Entity): string {
    return `/entity/${entity.type}/${encodeURIComponent(entity.id)}`
  }

  function openDetails() {
    const entity = options.entity.value
    if (!entity) return
    panel.show({
      component: EntityDetailPanel,
      props: {
        entityType: entity.type,
        entityId: entity.id,
        onClose: () => panel.clear(),
        onExpand: () => void router.push(fullPage(entity)),
      },
      // Over the tab rather than beside it: a board needs its full width.
      mode: 'overlay',
    })
  }

  async function requestDelete() {
    const entity = options.entity.value
    const type = options.entityType.value
    if (!entity || !type) return
    // Addressed to the row on screen: on a faced type that is one face, and
    // the server refuses a bare id there. The last face takes the entity.
    const ref = entityRef(entity)
    const face = refFace(ref)
    const ok = await confirm({
      title: `Delete ${typeDef.value?.label ?? type}?`,
      message: face
        ? `Are you sure you want to delete the ${schemaStore.faceLabel(type, face) || face} face of '${entityDisplayTitle(entity)}'? ` +
          'Its other faces are kept. This action cannot be undone.'
        : `Are you sure you want to delete '${entityDisplayTitle(entity)}'? This action cannot be undone.`,
      confirmLabel: 'Delete',
      danger: true,
    })
    if (!ok) return
    try {
      await deleteEntity(type, ref)
    } catch (err) {
      uiStore.error(err instanceof Error ? err.message : 'Failed to delete')
      return
    }
    panel.clear()
    void queryCache.invalidateQueries({ key: entityKeys.list(type) })
    uiStore.success('Deleted')
    // The page was this entity; the space's home is the nearest place left.
    void router.push(spaceStore.href('/'))
  }

  const menu = computed((): Slot | undefined => {
    const entity = options.entity.value
    if (!entity) return undefined
    return () => [
      h(EntityPageMenu, {
        typeLabel: typeDef.value?.label ?? entity.type,
        canDelete: canDelete.value,
        onDetails: openDetails,
        onOpen: () => void router.push(fullPage(entity)),
        onDelete: () => void requestDelete(),
      }),
    ]
  })

  return { badge, menu }
}
