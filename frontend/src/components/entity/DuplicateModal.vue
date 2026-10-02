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
import { computed, onMounted, ref } from 'vue'
import DynamicForm from '../forms/DynamicForm.vue'
import { useModalStack } from '@/composables/modalStack'
import { provideInlineCreateDepth } from '@/composables/useInlineCreate'
import { useConfirm } from '@/composables/useConfirm'
import { useSchemaStore } from '@/stores'
import { getAllEntityRelations } from '@/api/entities'
import { entityRef, refFace } from '@/utils/entityRef'
import {
  buildDuplicatePrefill,
  relationChoices,
  defaultSelection,
  type DuplicatePrefill,
  type RelationChoice,
} from './duplicatePrefill'
import type { Entity, RelationEntry } from '@/types'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlButtonGroup from 'rela-components/components/common/RlButtonGroup.vue'
import RlModal from 'rela-components/components/overlay/RlModal.vue'
import RlCheckbox from 'rela-components/components/form/RlCheckbox.vue'
import RlText from 'rela-components/components/common/RlText.vue'

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
   * World the create is issued in, for a source on the bare face. A source on
   * a named face creates the copy on that face instead (see `copyFace`).
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

const formRef = ref<{
  isDirty: () => boolean
  isSaving: () => boolean
  submit: () => void
} | null>(null)


// The address of the source row: its face, not the bare id, which a world
// would re-resolve to a different face (BUG-FYEEVX).
const sourceRef = computed(() => entityRef(props.source))

// A copy of a face is a new entity on that same face: duplicating a draft
// makes a draft. '' when the world decides instead: for the bare face, and
// for a face the world served only as a stand-in for its own (a fallback, or
// a later chain entry). Copying a stand-in onto its face would skip the
// world's step, such as publishing a copy from an editorial world.
const copyFace = computed(() => {
  const w = props.source._world
  const standIn =
    w?.via === 'fallback-default' || (w?.via === 'chain' && (w.chain_position ?? 0) > 0)
  return standIn ? '' : refFace(sourceRef.value)
})

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

const outgoingChoices = computed(() => choices.value.filter((c) => c.direction === 'outgoing'))
const incomingChoices = computed(() => choices.value.filter((c) => c.direction === 'incoming'))

async function loadRelations() {
  phase.value = 'loading'
  loadError.value = ''
  try {
    // By address, with no world: the relations sub-resource refuses `?world=`
    // (422), and the address already names the face whose content-scoped
    // edges are on screen.
    const data = await getAllEntityRelations(props.source.type, sourceRef.value)
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
// dialog is open: mount IS open. A `watch` on `show` was the wrong seam — the
// v-if tears the component down before the watcher can observe the transition.
onMounted(loadRelations)

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
/*
 * Only the submit accelerator is ours. Escape, the focus trap, the scroll
 * lock and the overlay stack are RlModal's — it registers with the shared
 * stack, which is what stops this dialog and a confirm raised from inside it
 * both reacting to one Escape press.
 */
function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && (e.metaKey || e.ctrlKey) && phase.value === 'form') {
    e.preventDefault()
    e.stopPropagation()
    formRef.value?.submit()
  }
}
</script>

<template>
  <!--
    RlModal owns the scrim, panel, header, close button, focus trap, Escape
    and the scrolling body. `layer` puts this under a discard-confirm raised
    from inside it; both teleport to body, so equal z-index would hide the
    confirm behind this dialog.
  -->
  <RlModal
    :open="true"
    :title="`Duplicate ${typeLabel}`"
    size="lg"
    :layer="900"
    panel-class="duplicate-modal"
    @close="requestClose"
    @keydown="handleKeydown"
  >
    <template v-if="phase === 'loading'">
      <RlText as="p" tone="muted">Loading relations…</RlText>
    </template>

    <template v-else-if="phase === 'failed'">
      <RlText as="p" tone="muted">
        Could not load this entity's relations, so a copy would be missing them.
      </RlText>
      <RlText as="p" size="sm" tone="muted">{{ loadError }}</RlText>
    </template>

    <template v-else-if="phase === 'choosing'">
      <RlText v-if="choices.length === 0" as="p" tone="muted">
        This {{ typeLabel.toLowerCase() }} has no relations to carry over.
      </RlText>

      <template v-else>
        <RlText as="p" tone="muted">Choose which relations the copy should keep.</RlText>

        <fieldset v-if="outgoingChoices.length" class="duplicate-group">
          <legend>Outgoing</legend>
          <RlCheckbox
            v-for="c in outgoingChoices"
            :key="c.key"
            :model-value="selected.includes(c.key)"
            :label="`${relationLabel(c.key)} (${c.count})`"
            @update:model-value="toggle(c.key)"
          />
        </fieldset>

        <fieldset v-if="incomingChoices.length" class="duplicate-group">
          <legend>Incoming</legend>
          <RlCheckbox
            v-for="c in incomingChoices"
            :key="c.key"
            :model-value="selected.includes(c.key)"
            :label="`${relationLabel(c.key)} (${c.count})`"
            @update:model-value="toggle(c.key)"
          />
        </fieldset>
      </template>
    </template>

    <template v-else>
      <ul v-if="omittedNotices.length" class="duplicate-omitted">
        <li v-for="o in omittedNotices" :key="o.property">
          <strong>{{ o.property }}</strong> was not copied ({{ omittedReasonLabel(o.reason) }})
        </li>
      </ul>

      <!-- v-if, not v-show: see the component doc. -->
      <DynamicForm
        v-if="prefill"
        ref="formRef"
        :form-id="formId"
        embedded
        :embedded-world="copyFace ? undefined : world"
        :embedded-face="copyFace || undefined"
        :embedded-prefill="prefill"
        @inline-created="handleCreated"
        @inline-cancelled="requestClose"
      />
    </template>

    <!-- The form phase carries DynamicForm's own buttons. -->
    <template v-if="phase !== 'form'" #actions>
      <RlButtonGroup>
        <RlButton variant="secondary" @click="requestClose">Cancel</RlButton>
        <RlButton v-if="phase === 'failed'" variant="primary" @click="loadRelations">
          Try again
        </RlButton>
        <RlButton v-else-if="phase === 'choosing'" variant="primary" @click="confirmChoices">
          Continue
        </RlButton>
      </RlButtonGroup>
    </template>
  </RlModal>
</template>

<style scoped>
/* Only the relation groupings are rela's; the panel, header, body scroll and
   footer are RlModal's. */
.duplicate-group {
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-md);
  padding: var(--rl-space-3);
  margin: 0 0 var(--rl-space-3);
}

.duplicate-group legend {
  padding: 0 var(--rl-space-1);
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text-muted);
}

.duplicate-omitted {
  margin: 0 0 var(--rl-space-3);
  padding-left: var(--rl-space-5);
  color: var(--rl-color-text-muted);
  font-size: var(--rl-font-size-sm);
}
</style>
