<script setup lang="ts">
/**
 * Switches between the faces of one record, such as Concept and
 * Vastgesteld, as a row of links in the detail header.
 *
 * It takes over from FaceMenu when a record has two or three faces in total
 * (see faceToggleOptions). Unlike the menu it stays in view in a narrow
 * header, and it shows the face on screen as the current one.
 *
 * ## Which world a face opens in
 *
 * Each link goes to the face's ADDRESS, which the server serves literally
 * under any world. When a declared world heads that face (worldForFace), the
 * link also switches to that world, so the related sections and the sidebar
 * world switcher agree with the face on screen. When no world heads it, or
 * the reader may not read that world, the current world is kept, as
 * FaceMenu does: a denied world would serve nothing, while the address
 * alone is served. The face on screen links to the page as it is.
 *
 * Not rendered on embedded read surfaces (`hideActions`): a preview must not
 * navigate away from its host, and the detail panel gets the faces as
 * actions instead.
 */
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import RlSegmentedControl from 'rela-components/components/data/RlSegmentedControl.vue'
import { worldQuery } from '@/composables/useWorld'
import { useSchemaStore } from '@/stores/schema'
import type { FaceToggleOption } from './faceToggleOptions'

const props = defineProps<{
  entityType: string
  current: string
  options: FaceToggleOption[]
}>()

const { t } = useI18n()
const route = useRoute()
const schemaStore = useSchemaStore()

const segments = computed(() =>
  props.options.map((o) => {
    const base = { value: o.face, label: o.label, as: RouterLink }
    if (o.face === props.current) return { ...base, attrs: { to: route.fullPath } }
    const target = schemaStore.worldForFace(props.entityType, o.face)
    const switchWorld = target !== undefined && schemaStore.worlds.get(target)?.readable !== false
    const query = switchWorld
      ? worldQuery(target, route.query, schemaStore.defaultWorld)
      : { ...route.query }
    return { ...base, attrs: { to: { path: `/entity/${props.entityType}/${o.ref}`, query } } }
  }),
)
</script>

<template>
  <RlSegmentedControl
    class="face-toggle"
    data-testid="face-toggle"
    :model-value="current"
    :options="segments"
    :label="t('entity.faceToggle')"
    size="sm"
  />
</template>

<style scoped>
/* The header info is a column that stretches its children; the toggle keeps
   its own width. */
.face-toggle {
  align-self: flex-start;
}
</style>
