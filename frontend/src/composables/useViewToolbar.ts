/**
 * The search box and Filters menu of a view that is not a list or a board:
 * a calendar or a gantt. Both show several entity types at once, so the
 * controls are resolved against the properties of all of them.
 *
 * The state lives in the URL, as on a list: `?q=` for the search and
 * `?filter[...]=` for the filters (see useUrlFilterSync). The view reads
 * `filters` and `searchQuery` into its own read, and binds the two refs to the
 * search box and the Filters menu.
 *
 * `/` focuses the search box and `f` opens the Filters menu, as on a list.
 */
import { computed, ref } from 'vue'
import { useUrlFilterSync } from '@/composables/useUrlFilterSync'
import { useListKeyboard } from '@/composables/useListKeyboard'
import { useSchemaStore } from '@/stores/schema'
import { filterStateToApiParams } from '@/utils/filters'
import type { EntityType, FilterControl, FilterState } from '@/types'

/**
 * One entity type that holds the properties of every given type, for the
 * Filters menu to resolve a control against. A property several types declare
 * takes its definition from the first. Nothing is read from it but
 * `properties`, so the rest is the first type's.
 */
export function mergedEntityType(types: Array<EntityType | undefined>): EntityType | undefined {
  const known = types.filter((t): t is EntityType => !!t)
  if (!known.length) return undefined
  const properties: EntityType['properties'] = {}
  for (const type of [...known].reverse()) Object.assign(properties, type.properties)
  return { ...known[0], properties }
}

/**
 * The filter controls a view that spans several types can offer: property
 * controls only. A relation control is resolved by the list read of one entity
 * type, and a calendar or a gantt has no such type, so it is left out.
 */
export function propertyControls(controls: FilterControl[] | undefined): FilterControl[] {
  return (controls ?? []).filter((c) => !!c.property && !c.relation)
}

/**
 * The API params for the filters that apply to one entity type: those naming
 * a property the type declares. A filter on a property another source has
 * does not narrow this one, and is not sent, because a list read refuses a
 * property its type lacks.
 */
export function filterParamsForType(
  filters: FilterState,
  type: Pick<EntityType, 'properties'> | undefined,
): Record<string, string> {
  const own: FilterState = {}
  for (const [property, value] of Object.entries(filters)) {
    if (type?.properties[property]) own[property] = value
  }
  return filterStateToApiParams(own)
}

export function useViewToolbar(options: {
  /** The entity types the view shows, by name. */
  entityTypes: () => string[]
  /** The view's own `filter_controls`. */
  controls: () => FilterControl[] | undefined
}) {
  const schemaStore = useSchemaStore()
  const { filters, q: searchQuery, writeToQuery } = useUrlFilterSync({
    // A calendar or a gantt has no static `filters:` to protect.
    staticFilterProperties: () => new Set(),
  })

  const searchBoxRef = ref<{ focus: () => void } | null>(null)
  const filterBarRef = ref<{ open: () => unknown } | null>(null)

  const entityType = computed(() =>
    mergedEntityType(options.entityTypes().map((name) => schemaStore.getEntityType(name))),
  )
  const filterConfig = computed(() => ({ filter_controls: propertyControls(options.controls()) }))

  function handleSearchUpdate(value: string) {
    writeToQuery(filters.value, value)
  }
  function handleFilter(next: FilterState) {
    writeToQuery(next)
  }

  useListKeyboard({
    itemCount: ref(0),
    onFocusSearch: () => searchBoxRef.value?.focus(),
    onOpenFilter: () => filterBarRef.value?.open(),
  })

  // Function refs for the template: a string ref to a destructured binding
  // reads as unused to the type checker.
  function bindSearchBox(el: unknown) {
    searchBoxRef.value = el as { focus: () => void } | null
  }
  function bindFilterBar(el: unknown) {
    filterBarRef.value = el as { open: () => unknown } | null
  }

  return {
    filters,
    searchQuery,
    bindSearchBox,
    bindFilterBar,
    entityType,
    filterConfig,
    handleSearchUpdate,
    handleFilter,
  }
}
