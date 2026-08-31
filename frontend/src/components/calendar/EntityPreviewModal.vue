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
import { computed, nextTick, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useModalStack } from '@/composables/modalStack'
import { shouldDeferToBrowser } from '@/utils/openIntent'
import EntityDetail from '@/components/entity/EntityDetail.vue'

const props = defineProps<{
  open: boolean
  entityType: string
  entityId: string
  /** Form opened by Edit; without one, Edit is not offered. */
  editForm?: string
}>()

const emit = defineEmits<{ (e: 'close'): void }>()

const dialog = ref<HTMLElement | null>(null)
const previouslyFocused = ref<HTMLElement | null>(null)

useModalStack(computed(() => props.open))

watch(
  () => props.open,
  async (isOpen) => {
    if (isOpen) {
      previouslyFocused.value = document.activeElement as HTMLElement | null
      await nextTick()
      dialog.value?.focus()
    } else {
      // Return focus where it was, so dismissing the modal does not dump the
      // user at the top of the page.
      previouslyFocused.value?.focus?.()
    }
  }
)

function close() {
  emit('close')
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.stopPropagation()
    close()
  }
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
  <Teleport to="body">
    <div v-if="open" class="modal-overlay" @click.self="close">
      <div
        ref="dialog"
        class="modal entity-preview"
        role="dialog"
        aria-modal="true"
        aria-label="Entity preview"
        tabindex="-1"
        @keydown="onKeydown"
      >
        <header class="entity-preview-header">
          <button type="button" class="entity-preview-close" aria-label="Close" @click="close">
            ✕
          </button>
        </header>

        <div class="entity-preview-body">
          <!-- Keyed so switching between events remounts rather than showing
               the previous entity's content while the next one loads. -->
          <EntityDetail
            :key="`${entityType}/${entityId}`"
            :entity-type="entityType"
            :entity-id="entityId"
            hide-actions
          />
        </div>

        <footer class="modal-actions">
          <RouterLink class="btn" :to="fullPageTarget" @click="onNavigate">
            Open full page
          </RouterLink>
          <RouterLink
            v-if="editFormTarget"
            class="btn btn-primary"
            :to="editFormTarget"
            @click="onNavigate"
          >
            Edit
          </RouterLink>
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.entity-preview {
  display: flex;
  flex-direction: column;
  width: min(860px, 92vw);
  max-height: 88vh;
  padding: 0;
}

.entity-preview-header {
  display: flex;
  justify-content: flex-end;
  padding: var(--space-sm) var(--space-sm) 0;
}

.entity-preview-close {
  border: none;
  background: none;
  color: var(--muted-text);
  font-size: var(--font-size-lg);
  line-height: 1;
  cursor: pointer;
}

.entity-preview-close:hover {
  color: var(--text-color);
}

/* The detail component owns its own layout; this only bounds and scrolls it
   so a long entity does not push the action buttons off-screen. */
.entity-preview-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 0 var(--space-lg) var(--space-md);
}

.modal-actions {
  padding: var(--space-md) var(--space-lg);
  border-top: 1px solid var(--border-color);
}
</style>
