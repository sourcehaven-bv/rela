<script setup lang="ts">
/**
 * DuplicateModal — copy an entity, choosing which relations come with it
 * (TKT-Z8K2FS).
 *
 * Two steps in one dialog. First a checkbox list of the source's relation
 * types; then the real create form, embedded, prefilled from the source. The
 * form is hosted here rather than reached by navigation because the payload
 * cannot survive a URL: `?prop.*` stringifies every value (dropping list
 * properties outright), has no channel for the markdown body at all, and a
 * mean entity in this repo is several KB of markdown before encoding.
 *
 * Three structural rules, each inherited from a mechanism that already exists:
 *
 * 1. `provideInlineCreateDepth()` puts the embedded form at depth 1, so its
 *    relation fields offer link-existing but not inline-create. The depth cap
 *    is STRUCTURAL (`useInlineCreate.ts`): `modalStack` is a Set and cannot say
 *    which dialog is topmost, so a modal opened over this one would have no
 *    defined Escape recipient. This is the opt-in that keeps it unreachable.
 * 2. `useModalStack` registers the dialog, so the detail page's Del/Backspace
 *    delete shortcut cannot fire underneath an open duplicate.
 * 3. `DynamicForm` mounts under `v-if`, never `v-show` — unmounting aborts its
 *    in-flight dry-run rather than leaving it POSTing behind a closed dialog.
 */
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import DynamicForm from '../forms/DynamicForm.vue'
import { useModalStack } from '@/composables/modalStack'
import { provideInlineCreateDepth } from '@/composables/useInlineCreate'
import { useConfirm } from '@/composables/useConfirm'
import { useSchemaStore } from '@/stores'
import { getAllEntityRelations } from '@/api/entities'
import {
  buildDuplicatePrefill,
  relationChoices,
  defaultSelection,
  type DuplicatePrefill,
  type RelationChoice,
} from './duplicatePrefill'
import type { Entity, RelationEntry } from '@/types'

const props = defineProps<{
  /**
   * The entity being duplicated, as the detail page already loaded it.
   *
   * There is deliberately no `show` prop: the host mounts this under `v-if`,
   * so the component's lifetime IS the dialog's. Carrying both a `v-if` and a
   * `show` gate made the close transition unobservable — the unmount beat the
   * watcher, so focus was never returned to the trigger.
   */
  source: Entity
  /** Create form id, from the sidebar `inline_create` map. */
  formId: string
  /**
   * World the create is issued in. Decides which face the copy lands in; a
   * faced type has no default row to fall back to, so dropping it is a refusal
   * rather than a silent default.
   */
  world?: string
}>()

const emit = defineEmits<{
  close: []
  created: [entity: Entity]
}>()

const schemaStore = useSchemaStore()
const { confirm } = useConfirm()

// Registered for the component's whole lifetime, which is exactly how long
// the dialog is on screen.
useModalStack(computed(() => true))
provideInlineCreateDepth()

type Phase = 'choosing' | 'loading' | 'failed' | 'form'

const phase = ref<Phase>('loading')
const relations = ref<Record<string, RelationEntry[]>>({})
const choices = ref<RelationChoice[]>([])
const selected = ref<string[]>([])
const prefill = ref<DuplicatePrefill | null>(null)
const loadError = ref('')

const dialogRef = ref<HTMLElement | null>(null)
const previouslyFocused = ref<HTMLElement | null>(null)
const formRef = ref<{
  isDirty: () => boolean
  isSaving: () => boolean
  submit: () => void
} | null>(null)

const titleId = `duplicate-title-${Math.random().toString(36).slice(2, 10)}`

const typeLabel = computed(
  () => schemaStore.getEntityType(props.source.type)?.label || props.source.type
)

/**
 * Properties that will not carry, for disclosure.
 *
 * A duplicate of an entity with hidden or uncopyable fields is incomplete by
 * construction, and the user is told which ones rather than handed a quietly
 * short copy. `not-configured` is excluded: the operator asked for that one.
 */
const omittedNotices = computed(() =>
  (prefill.value?.omitted ?? []).filter((o) => o.reason !== 'not-configured')
)

function omittedReasonLabel(reason: string): string {
  switch (reason) {
    case 'redacted':
      return 'not visible to you'
    case 'file':
      return 'attached file'
    case 'state-machine':
      return 'starts at its initial value'
    case 'self-loop':
      return 'points at the original; re-link on the copy'
    case 'untyped-peer':
      return 'peer type unavailable'
    default:
      return reason
  }
}

/**
 * Display label for a relation group.
 *
 * An INCOMING group is keyed by the relation's inverse name, which is not a
 * relation type, so the lookup misses and the key itself is shown. That is the
 * right fallback: the inverse name is what the operator wrote in the schema
 * (`inverse.id`), so it is already the word they chose for that direction.
 */
function relationLabel(key: string): string {
  return schemaStore.getRelationType(key)?.label || key
}

const outgoingChoices = computed(() =>
  choices.value.filter((c) => c.direction === 'outgoing')
)
const incomingChoices = computed(() =>
  choices.value.filter((c) => c.direction === 'incoming')
)

async function loadRelations() {
  phase.value = 'loading'
  loadError.value = ''
  try {
    const data = await getAllEntityRelations(props.source.type, props.source.id, props.world)
    relations.value = data ?? {}
    choices.value = relationChoices(relations.value)
    selected.value = defaultSelection(choices.value)
    phase.value = 'choosing'
  } catch (err) {
    // A failed fetch must NOT fall through to the empty state. The server drops
    // every neighbour fail-closed on a store error and still answers 200, so
    // "no relations" and "the read broke" are already hard to tell apart; a
    // thrown error rendered as an empty list would silently duplicate an entity
    // with none of its edges.
    loadError.value = err instanceof Error ? err.message : String(err)
    phase.value = 'failed'
  }
}

function toggle(key: string) {
  const at = selected.value.indexOf(key)
  if (at === -1) selected.value.push(key)
  else selected.value.splice(at, 1)
}

function confirmChoices() {
  prefill.value = buildDuplicatePrefill(
    props.source,
    schemaStore.getEntityType(props.source.type),
    relations.value,
    selected.value,
    schemaStore.duplicateConfigFor(props.source.type)
  )
  phase.value = 'form'
}

// The host mounts this under `v-if`, so the component exists only while the
// dialog is open: mount IS open and unmount IS close. A `watch` on `show` was
// the wrong seam — the v-if tears the component down before the watcher can
// observe the transition, so the close branch never ran and focus was never
// returned to the triggering control.
onMounted(async () => {
  previouslyFocused.value = document.activeElement as HTMLElement | null
  // Focus BEFORE the fetch, not after. Escape is bound to the dialog element
  // (deliberately, so it cannot reach past this dialog), which means it only
  // works once focus is inside — and the loading phase is exactly when a user
  // wants out of a slow or hung read.
  await nextTick()
  dialogRef.value?.focus()
  await loadRelations()
})

onBeforeUnmount(() => {
  previouslyFocused.value?.focus?.()
  previouslyFocused.value = null
})

async function requestClose() {
  if (formRef.value?.isSaving()) return
  if (phase.value === 'form' && formRef.value?.isDirty()) {
    const ok = await confirm({
      title: 'Discard copy?',
      message: 'This copy has not been created yet. Your input will be lost.',
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

// Bound to the dialog rather than `document`, so the host page's handlers stay
// untouched and these cannot reach past this dialog.
function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.stopPropagation()
    void requestClose()
    return
  }
  if (e.key === 'Enter' && (e.metaKey || e.ctrlKey) && phase.value === 'form') {
    e.preventDefault()
    e.stopPropagation()
    formRef.value?.submit()
  }
}
</script>

<template>
  <Teleport to="body">
    <div class="modal-overlay" @click.self="requestClose">
      <div
        ref="dialogRef"
        class="modal duplicate-modal"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleId"
        tabindex="-1"
        @keydown="handleKeydown"
      >
        <header class="duplicate-header">
          <h2 :id="titleId">Duplicate {{ typeLabel }}</h2>
          <button type="button" class="close-btn" aria-label="Close" @click="requestClose">
            &times;
          </button>
        </header>

        <div class="duplicate-body">
          <template v-if="phase === 'loading'">
            <p class="duplicate-status">Loading relations…</p>
            <div class="duplicate-actions">
              <button type="button" class="btn" @click="requestClose">Cancel</button>
            </div>
          </template>

          <div v-else-if="phase === 'failed'" class="duplicate-status duplicate-error">
            <p>Could not load this entity's relations, so a copy would be missing them.</p>
            <p class="duplicate-error-detail">{{ loadError }}</p>
            <div class="duplicate-actions">
              <button type="button" class="btn" @click="requestClose">Cancel</button>
              <button type="button" class="btn btn-primary" @click="loadRelations">Try again</button>
            </div>
          </div>

          <template v-else-if="phase === 'choosing'">
            <p v-if="choices.length === 0" class="duplicate-status">
              This {{ typeLabel.toLowerCase() }} has no relations to carry over.
            </p>

            <template v-else>
              <p class="duplicate-intro">Choose which relations the copy should keep.</p>

              <fieldset v-if="outgoingChoices.length" class="duplicate-group">
                <legend>Outgoing</legend>
                <label v-for="c in outgoingChoices" :key="c.key" class="duplicate-choice">
                  <input
                    type="checkbox"
                    :checked="selected.includes(c.key)"
                    @change="toggle(c.key)"
                  />
                  <span class="duplicate-choice-label">{{ relationLabel(c.key) }}</span>
                  <span class="duplicate-count">{{ c.count }}</span>
                </label>
              </fieldset>

              <fieldset v-if="incomingChoices.length" class="duplicate-group">
                <legend>Incoming</legend>
                <label v-for="c in incomingChoices" :key="c.key" class="duplicate-choice">
                  <input
                    type="checkbox"
                    :checked="selected.includes(c.key)"
                    @change="toggle(c.key)"
                  />
                  <span class="duplicate-choice-label">{{ relationLabel(c.key) }}</span>
                  <span class="duplicate-count">{{ c.count }}</span>
                </label>
              </fieldset>
            </template>

            <div class="duplicate-actions">
              <button type="button" class="btn" @click="requestClose">Cancel</button>
              <button type="button" class="btn btn-primary" @click="confirmChoices">
                Continue
              </button>
            </div>
          </template>

          <template v-else>
            <ul v-if="omittedNotices.length" class="duplicate-omitted">
              <li v-for="o in omittedNotices" :key="o.property">
                <strong>{{ o.property }}</strong> was not copied ({{
                  omittedReasonLabel(o.reason)
                }})
              </li>
            </ul>

            <!-- v-if, not v-show: see the component doc. -->
            <DynamicForm
              v-if="prefill"
              ref="formRef"
              :form-id="formId"
              embedded
              :embedded-world="world"
              :embedded-prefill="prefill"
              @inline-created="handleCreated"
              @inline-cancelled="requestClose"
            />
          </template>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
/* Below ConfirmModal's overlay (1000 in App.vue): both Teleport to body, so at
   equal z-index the later-mounted one wins on DOM order, which would hide the
   discard-confirm behind this dialog. */
.modal-overlay {
  z-index: 900;
}

.duplicate-modal {
  width: min(760px, 92vw);
  max-width: none;
  max-height: 88vh;
  display: flex;
  flex-direction: column;
  padding: 0;
}

.duplicate-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
  padding: var(--space-lg) var(--space-lg) 0;
}

.duplicate-body {
  padding: var(--space-lg);
  overflow-y: auto;
}

.duplicate-intro {
  margin: 0 0 var(--space-md);
  color: var(--text-secondary);
}

.duplicate-status {
  color: var(--text-secondary);
  margin: 0;
}

.duplicate-error-detail {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

.duplicate-group {
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: var(--space-md);
  margin: 0 0 var(--space-md);
}

.duplicate-group legend {
  padding: 0 var(--space-xs);
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

.duplicate-choice {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  padding: var(--space-xs) 0;
  cursor: pointer;
}

.duplicate-choice-label {
  flex: 1;
}

.duplicate-count {
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
}

.duplicate-omitted {
  margin: 0 0 var(--space-md);
  padding-left: var(--space-lg);
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
}

.duplicate-actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-sm);
  margin-top: var(--space-lg);
}
</style>
