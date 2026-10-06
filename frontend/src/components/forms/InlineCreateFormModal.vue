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
import { computed, ref } from 'vue'
import DynamicForm from './DynamicForm.vue'
import { useModalStack } from '@/composables/modalStack'
// RlModal owns the scrim, Tab trap, scroll lock, Escape and focus restore.
// `useModalStack` below stays: rela's registry is what suppresses global
// shortcuts, and the library's overlay stack answers a different question.
import RlModal from 'rela-components/components/overlay/RlModal.vue'
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
  /**
   * Entity template variant to preselect, from the section's
   * `create.types.<type>.template` (TKT-R4BMJM). Undefined leaves the form's
   * own default selection alone.
   */
  template?: string
  /**
   * Pre-link context, when this modal was opened from a section's create
   * affordance. Passed as a PROP rather than through the URL because an
   * embedded form deliberately reads an empty query — it mounts over the host's
   * page, and honouring that page's params would pre-fill the new entity from
   * whatever the host happened to be showing.
   *
   * Only `linkAs: 'to'` is applied here (the form carries the edge in its create
   * payload). The reverse direction is the host's job, after the id exists.
   */
  link?: { relation: string; peer: string; linkAs: 'from' | 'to' }
  /**
   * World the create is issued in. Decides which face the new entity lands in
   * (`worlds.<name>.create`); a faced type has no default row to fall back to,
   * so omitting it is a refusal rather than a silent default.
   */
  world?: string
  /**
   * Offer "Create & add another". Only for a host that has no link step: each
   * extra record arrives through `created-another` and the dialog stays open.
   * A relation field leaves it off, since it links exactly one entity.
   */
  addAnother?: boolean
  /**
   * Property values the new entity starts with, such as the group value of
   * the list section whose Add opened this dialog. Marked as user input, so
   * a value the form cannot write is refused by the server rather than
   * dropped without a word (see `embeddedPrefill` on DynamicForm).
   */
  prefill?: {
    properties: Record<string, unknown>
    relations?: Record<string, { id: string; type: string }[]>
  }
}>()

const emit = defineEmits<{
  close: []
  created: [entity: Entity]
  'created-another': [entity: Entity]
}>()

const schemaStore = useSchemaStore()
const { confirm } = useConfirm()

useModalStack(computed(() => props.show))
provideInlineCreateDepth()

// The embedded form's exposed handle (isDirty / isSaving / submit). We ASK the
// form for its state rather than sniffing DOM events, because a relation
// selection, a wizard step and a markdown body edit all emit Vue events that
// never surface as bubbling native input/change.
const formRef = ref<{
  isDirty: () => boolean
  isSaving: () => boolean
  submit: () => void
} | null>(null)

const typeLabel = computed(
  () => schemaStore.getEntityType(props.entityType)?.label || props.entityType
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

// Bound to the dialog panel (RlModal forwards attrs onto it) rather than to
// `document`, so it cannot reach past this dialog — the host form's own
// handlers stay untouched. Cmd/Ctrl+Enter submits: the embedded form
// deliberately does NOT register its document-level listener (two live forms
// would both act on one keypress), so the dialog owns the shortcut its Create
// button advertises. Escape is RlModal's.
function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
    e.preventDefault()
    e.stopPropagation()
    formRef.value?.submit()
  }
}
</script>

<template>
  <RlModal
    :open="show"
    :title="`New ${typeLabel}`"
    size="lg"
    panel-class="inline-create-panel"
    :layer="900"
    @keydown="handleKeydown"
    @close="requestClose"
  >
    <!-- v-if, not v-show: see the component doc. -->
    <DynamicForm
      ref="formRef"
      :form-id="formId"
      embedded
      :embedded-template="template"
      :embedded-link="link"
      :embedded-world="world"
      :embedded-add-another="addAnother"
      :embedded-prefill="prefill"
      @inline-created="handleCreated"
      @inline-created-another="emit('created-another', $event)"
      @inline-cancelled="requestClose"
    />
  </RlModal>
</template>

<style scoped>
/* Wider than RlModal's `lg`: this hosts a full form, not a sentence. Height is
   capped so a long or multi-step form scrolls inside the dialog instead of
   pushing its actions off-screen. The `layer` prop above keeps it below
   ConfirmModal's 1000 — see the component doc. */
:global(.inline-create-panel) {
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
