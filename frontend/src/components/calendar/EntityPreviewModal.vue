<script setup lang="ts">
/**
 * Read-first preview of one entity, opened from a calendar chip.
 *
 * The interaction this exists for: a chip is small, so clicking it should
 * answer "what is this?" before offering "change it". Jumping straight into an
 * edit form — which is what the calendar did before — skips the step people
 * actually want and puts a save button in front of someone who was only
 * looking.
 *
 * # Why it wraps EntityDetail rather than rendering its own summary
 *
 * A second, simpler renderer would drift: the entity page shows configured
 * view sections, resolved relations, rendered markdown and per-type overrides,
 * and a hand-rolled preview would show a subset that slowly disagrees with it.
 * EntityDetail already takes just (entityType, entityId) — the route view is a
 * 12-line wrapper around it — so a modal is the same wrapper with an overlay.
 *
 * Note EntityDetail's own keyboard shortcuts stand down while any modal is
 * open (it checks isAnyModalOpen), so its handlers do not fight the calendar's
 * while this is up.
 *
 * EntityDetail is rendered with `hide-actions`, so its own Edit / History /
 * Delete toolbar is suppressed and this wrapper owns the actions instead. A
 * preview is for reading: a destructive Delete one click from a calendar chip
 * is a bigger gesture than the click that opened it, and History belongs on
 * the full page where there is room for it.
 *
 * What remains is the two things a reader actually wants next — edit this, or
 * go see it properly.
 */
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { useModalStack } from '@/composables/modalStack'
import { shouldDeferToBrowser } from '@/utils/openIntent'
import EntityDetail from '@/components/entity/EntityDetail.vue'
import RlModal from 'rela-components/components/overlay/RlModal.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'

const props = defineProps<{
  open: boolean
  entityType: string
  entityId: string
  /** Form opened by Edit; without one, Edit is not offered. */
  editForm?: string
}>()

const emit = defineEmits<{ (e: 'close'): void }>()

// RlModal owns the scrim, the Tab trap, the scroll lock, Escape and the focus
// restore. rela's modal stack is a SEPARATE registry that gates global
// keyboard shortcuts (isAnyModalOpen), so it is still registered here — see
// the note in ScriptErrorDialog.
useModalStack(computed(() => props.open))

function close() {
  emit('close')
}

// Both footer actions are pure navigation, so they render as real links and
// support cmd/ctrl/middle-click. On a modifier click the browser opens a tab
// and the modal deliberately STAYS OPEN — closing it would drop the preview the
// user was reading. A plain click still closes and routes in place.
const fullPageTarget = computed(() => `/entity/${props.entityType}/${props.entityId}`)
const editFormTarget = computed(() =>
  props.editForm ? `/form/${props.editForm}/${props.entityId}` : undefined
)

function onNavigate(event: MouseEvent) {
  if (shouldDeferToBrowser(event)) return
  close()
}

</script>

<template>
  <RlModal
    :open="open"
    title="Entity preview"
    title-hidden
    size="lg"
    panel-class="entity-preview-panel"
    @close="close"
  >
    <!-- Keyed so switching between events remounts rather than showing the
         previous entity's content while the next one loads. -->
    <EntityDetail
      :key="`${entityType}/${entityId}`"
      :entity-type="entityType"
      :entity-id="entityId"
      hide-actions
    />

    <template #actions>
      <div class="entity-preview-actions">
        <RlButton :as="RouterLink" variant="secondary" :to="fullPageTarget" @click="onNavigate">
          Open full page
        </RlButton>
        <RlButton
          v-if="editFormTarget"
          :as="RouterLink"
          variant="primary"
          :to="editFormTarget"
          @click="onNavigate"
        >
          Edit
        </RlButton>
      </div>
    </template>
  </RlModal>
</template>

<style scoped>
/*
 * The preview wraps EntityDetail, which renders the same view sections as the
 * full entity page, so it gets more room than RlModal's `lg` (800px). This is
 * the case `panelClass` exists for: a width the size scale does not cover,
 * rather than general styling.
 *
 * RlModal scrolls its own body and separates the footer, so only the action
 * row's own layout is left.
 */
:global(.entity-preview-panel) {
  max-width: min(860px, 92vw);
  max-height: 88vh;
}

.entity-preview-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-sm);
}
</style>
