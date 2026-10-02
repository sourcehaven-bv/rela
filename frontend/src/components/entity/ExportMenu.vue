<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { getTransforms, type TransformInfo } from '@/api/transforms'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIcon from 'rela-components/components/common/RlIcon.vue'
import RlMenu from 'rela-components/components/overlay/RlMenu.vue'
import RlMenuItem from 'rela-components/components/overlay/RlMenuItem.vue'

const props = defineProps<{
  /**
   * Given a transform name, return the export URL to navigate to. The parent
   * builds it (entity vs. list, plus any current list filter/sort params) so
   * the menu stays agnostic about the export target.
   */
  urlFor: (transform: string) => string
}>()

const transforms = ref<TransformInfo[]>([])
let abort: AbortController | null = null

onMounted(async () => {
  abort = new AbortController()
  try {
    transforms.value = await getTransforms(abort.signal)
  } catch (err) {
    // A missing/empty registry simply hides the menu; don't disrupt the page.
    console.error('Failed to load export transforms:', err)
    transforms.value = []
  }
})

onBeforeUnmount(() => abort?.abort())

function exportAs(t: TransformInfo) {
  // Navigate to the hardened forced-download endpoint. Using a real nav (not
  // fetch) lets the browser's download machinery handle Content-Disposition.
  // RlMenu closes itself on a click inside the panel.
  window.location.href = props.urlFor(t.name)
}
</script>

<template>
  <!--
    RlMenu owns what this component used to hand-roll: the open state, the
    click-outside listener, arrow-key navigation, and the panel's position
    (teleported, so a scrolling ancestor cannot clip it).
  -->
  <RlMenu v-if="transforms.length" class="export-menu">
    <template #trigger="{ toggle, attrs }">
      <RlButton variant="secondary" v-bind="attrs" @click="toggle">
        Export
        <template #trailing>
          <RlIcon name="chevron-down" :size="14" aria-hidden="true" />
        </template>
      </RlButton>
    </template>

    <RlMenuItem
      v-for="t in transforms"
      :key="t.name"
      class="export-menu-item"
      @click="exportAs(t)"
    >
      {{ t.name }}
    </RlMenuItem>
  </RlMenu>
</template>

<style scoped>
/*
 * Transform names are short format identifiers (pdf, docx), so they read as
 * labels rather than prose. Everything else the list used to declare -- the
 * panel box, its position, the item hover -- is RlMenu's and RlMenuItem's.
 */
.export-menu-item {
  text-transform: uppercase;
}
</style>
