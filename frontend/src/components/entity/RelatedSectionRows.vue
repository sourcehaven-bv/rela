<script setup lang="ts">
/**
 * The rows of a `display: related` section (TKT-QO14GB): one line per entity,
 * its title and its fields as trailing text.
 *
 * It suits records shown as part of the entry, such as owned subtasks. A row
 * owned by something links to its owner's page, anchored at the row; on the
 * owner's own page that is a jump to the row. Any other row links to its own
 * page.
 *
 * Each row's id is the entity id, so `#<id>` scrolls to it, unless `anchor`
 * gives the anchor to another row on the page.
 */
import { computed } from 'vue'
import { RouterLink, type RouteLocationRaw } from 'vue-router'
import RlRelatedRow from 'rela-components/components/data/RlRelatedRow.vue'
import type { RelatedItem, RelatedMeta } from 'rela-components/components/data/types'
import type { ViewEntity } from '@/api'
import { useSchemaStore } from '@/stores/schema'
import { useUIStore } from '@/stores/ui'
import { ownedEntityHref } from '@/utils/entityRoute'
import { formatCellValue } from '@/utils/format'
import { isDenseEmpty } from '@/widgets/viewRouting'

const props = defineProps<{
  entities: ViewEntity[]
  /** The page's `?world=`, carried onto each link. */
  world?: string
  /** The anchor id for a row, or undefined when another row holds it. */
  anchor?: (id: string) => string | undefined
}>()

const schemaStore = useSchemaStore()
const uiStore = useUIStore()

function target(ent: ViewEntity): RouteLocationRaw | undefined {
  const href = ownedEntityHref(ent)
  if (!href) return undefined
  const [path, hash] = href.split('#')
  return {
    path,
    hash: hash ? `#${hash}` : undefined,
    query: props.world ? { world: props.world } : {},
  }
}

// Field values as text, the way a table cell shows them. An empty value or one
// the principal cannot read is left out rather than drawn as a blank column.
function meta(ent: ViewEntity): RelatedMeta[] {
  const typeDef = schemaStore.entityTypes.get(ent.type)
  const out: RelatedMeta[] = []
  for (const [idx, field] of (ent.fields ?? []).entries()) {
    if (field.inaccessible) continue
    const raw =
      field.property && ent._props && field.property in ent._props
        ? ent._props[field.property]
        : (field.values ?? [])
    if (isDenseEmpty(raw)) continue
    const value = formatCellValue(raw, field.property, typeDef, uiStore.effectiveTimezone)
    if (!value) continue
    out.push({ id: field.property ?? String(idx), value, label: field.label })
  }
  return out
}

const items = computed<RelatedItem[]>(() =>
  props.entities.map((ent) => {
    const to = target(ent)
    return {
      id: ent.id,
      title: ent.title || ent.id,
      meta: meta(ent),
      ...(to ? { as: RouterLink, attrs: { to } } : {}),
    }
  })
)
</script>

<template>
  <div class="related-rows">
    <RlRelatedRow
      v-for="item in items"
      :id="anchor ? anchor(item.id) : item.id"
      :key="item.id"
      :item="item"
    />
  </div>
</template>

<style scoped>
.related-rows {
  border-top: 1px solid var(--rl-color-border);
}
</style>
