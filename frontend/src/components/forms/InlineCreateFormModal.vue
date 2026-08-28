<script setup lang="ts">
/**
 * InlineCreateFormModal — hosts a real create form in a dialog so a relation
 * field can spawn a new target entity without navigating (TKT-OMUD56).
 *
 * The form is `DynamicForm` itself, in `embedded` mode: the same fields,
 * widgets, templates, validation, wizard steps and dry-run affordances a
 * top-level create form gets. It deliberately is NOT a parallel renderer —
 * the modal this replaced hand-rolled its own widget dispatch off the raw
 * metamodel and drifted (it had no validation, no templates, and dropped
 * intentional `false` booleans).
 *
 * Two structural rules live here:
 *
 * 1. `DynamicForm` is mounted under `v-if`, never `v-show`. Unmounting is what
 *    aborts the nested form's in-flight dry-run and marks it gone (RR-2PZB);
 *    a hidden-but-alive form would keep POSTing behind a closed dialog.
 * 2. The nested form is one level deeper (`provideInlineCreateDepth`), which
 *    is what stops a relation field inside it offering inline create in turn.
 *    Modal-in-modal is unreachable rather than merely discouraged, because
 *    `modalStack` is a Set and cannot say which dialog is topmost.
 */
import { computed, nextTick, ref, watch } from 'vue'
import DynamicForm from './DynamicForm.vue'
import { useModalStack } from '@/composables/modalStack'
import { provideInlineCreateDepth } from '@/composables/useInlineCreate'
import { useConfirm } from '@/composables/useConfirm'
import { useSchemaStore } from '@/stores'
import type { Entity } from '@/types'

const props = defineProps<{
  show: boolean
  /** Form id to render — resolved server-side, never user-supplied. */
  formId: string
  /** Entity type being created; used for the dialog title. */
  entityType: string
}>()

const emit = defineEmits<{
  close: []
  created: [entity: Entity]
}>()

const schemaStore = useSchemaStore()
const { confirm } = useConfirm()

useModalStack(computed(() => props.show))
provideInlineCreateDepth()

const dialogRef = ref<HTMLElement | null>(null)
const previouslyFocused = ref<HTMLElement | null>(null)
// The embedded form's exposed handle (isDirty / isSaving / submit). We ASK the
// form for its state rather than sniffing DOM events, because a relation
// selection, a wizard step and a markdown body edit all emit Vue events that
// never surface as bubbling native input/change.
const formRef = ref<{
  isDirty: () => boolean
  isSaving: () => boolean
  submit: () => void
} | null>(null)

// Teleport puts this at document.body, so a static id would collide if two
// instances ever mounted at once.
const titleId = `inline-create-title-${Math.random().toString(36).slice(2, 10)}`

const typeLabel = computed(
  () => schemaStore.getEntityType(props.entityType)?.label || props.entityType
)

watch(
  () => props.show,
  async (isOpen, wasOpen) => {
    if (isOpen && !wasOpen) {
      previouslyFocused.value = document.activeElement as HTMLElement | null
      // Focus the dialog itself; the form's first field is the user's next
      // Tab stop. Focusing a specific field would fight the wizard, whose
      // first step varies.
      await nextTick()
      dialogRef.value?.focus()
    } else if (!isOpen && wasOpen) {
      previouslyFocused.value?.focus?.()
      previouslyFocused.value = null
    }
  }
)

async function requestClose() {
  // Don't tear the form out from under an in-flight create.
  if (formRef.value?.isSaving()) return
  if (formRef.value?.isDirty()) {
    const ok = await confirm({
      title: 'Discard new ' + typeLabel.value.toLowerCase() + '?',
      message: 'This entity has not been created yet. Your input will be lost.',
      confirmLabel: 'Discard',
      danger: true,
    })
    if (!ok) return
  }
  emit('close')
}

function handleCreated(entity: Entity) {
  emit('created', entity)
}

// Bound to the dialog element rather than `document` so these cannot reach past
// this dialog — the host form's own handlers stay untouched.
//
// Cmd/Ctrl+Enter submits: the embedded form deliberately does NOT register its
// document-level listener (two live forms would both act on one keypress), so
// the dialog owns the shortcut its Create button advertises.
function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.stopPropagation()
    void requestClose()
    return
  }
  if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
    e.preventDefault()
    e.stopPropagation()
    formRef.value?.submit()
  }
}
</script>

<template>
  <Teleport to="body">
    <div v-if="show" class="modal-overlay" @click.self="requestClose">
      <div
        ref="dialogRef"
        class="modal inline-create-modal"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleId"
        tabindex="-1"
        @keydown="handleKeydown"
      >
        <header class="inline-create-header">
          <h2 :id="titleId">New {{ typeLabel }}</h2>
          <button type="button" class="close-btn" aria-label="Close" @click="requestClose">
            &times;
          </button>
        </header>

        <div class="inline-create-body">
          <!-- v-if, not v-show: see the component doc. -->
          <DynamicForm
            ref="formRef"
            :form-id="formId"
            embedded
            @inline-created="handleCreated"
            @inline-cancelled="requestClose"
          />
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
/* Below ConfirmModal's overlay (z-index 1000 in App.vue). Both Teleport to
   body, so at equal z-index the later-mounted one wins on DOM order — which
   would put the discard-confirm UNDERNEATH this dialog, leaving the user
   staring at a frozen modal awaiting an invisible answer. */
.modal-overlay {
  z-index: 900;
}

/* Wider than the shared .modal default: this hosts a full form, not a
   sentence. Height is capped so a long or multi-step form scrolls inside the
   dialog instead of pushing its actions off-screen. */
.inline-create-modal {
  width: min(760px, 92vw);
  max-width: none;
  max-height: 88vh;
  display: flex;
  flex-direction: column;
  padding: 0;
}

.inline-create-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  padding: var(--space-lg) var(--space-lg) 0;
}

.inline-create-header h2 {
  margin: 0;
  font-size: var(--font-size-lg);
}

.close-btn {
  background: none;
  border: none;
  font-size: var(--font-size-xl);
  line-height: 1;
  cursor: pointer;
  color: var(--text-secondary);
  padding: 0 var(--space-xs);
}

.close-btn:hover {
  color: var(--text-primary);
}

.inline-create-body {
  overflow-y: auto;
  padding: var(--space-lg);
}
</style>
