<script lang="ts">
import type { PropertyDef, TaskItemConversionOffer } from '@/types'
import type { TaskItemTarget } from './milkdown/taskItemConvert'
import type { EntityRefInsert } from './milkdown/insertEntityRef'

/** What this component needs of the editor's exposed API. */
export interface TaskItemEditor {
  applyTaskItemResult(
    token: number,
    text: string,
    mode: 'link' | 'remove',
    ref: EntityRefInsert
  ): boolean
  markTaskItemBusy(target: TaskItemTarget): number | null
  clearTaskItemBusy(token: number): void
}

/**
 * The convert button's label for a set of offers, or empty for no button.
 * Only allowed offers count: a button that can only refuse is noise.
 */
export function taskItemConvertLabel(offers: TaskItemConversionOffer[] | undefined): string {
  const allowed = (offers ?? []).filter((o) => o.allowed)
  if (allowed.length === 0) return ''
  return allowed.length === 1 ? allowed[0].label : 'Convert task item'
}
</script>

<script setup lang="ts">
/**
 * Runs a task-item conversion for the editor (TKT-3IBLIP).
 *
 * The editor emits `taskItemConvert` with the item; the host passes it to
 * `start`. One allowed recipe converts straight away; several open a choice.
 * If the recipe leaves required fields empty, a dialog asks for exactly those
 * fields. The server creates the entity and the relation in one request, then
 * the editor links or removes the item.
 *
 * The item's busy mark lasts the whole conversion, from `start` until the
 * dialog closes or the last request returns. The result is applied through
 * it, so it lands on the item that was converted, wherever edits moved it.
 */
import { computed, reactive, ref } from 'vue'
import { useSchemaStore } from '@/stores'
import { useUIStore } from '@/stores/ui'
import { getErrorMessage } from '@/api'
import {
  convertTaskItem,
  incompleteConversionEntity,
  previewTaskItemConversion,
  type TaskItemConversionResult,
} from '@/api/taskItemConversions'
import DatetimeWidget from '@/widgets/DatetimeWidget.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlButtonGroup from 'rela-components/components/common/RlButtonGroup.vue'
import RlModal from 'rela-components/components/overlay/RlModal.vue'
import RlCheckbox from 'rela-components/components/form/RlCheckbox.vue'
import RlDateField from 'rela-components/components/form/RlDateField.vue'
import RlFieldShell from 'rela-components/components/form/RlFieldShell.vue'
import RlNumberField from 'rela-components/components/form/RlNumberField.vue'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import type { SelectOption } from 'rela-components/components/form/types'

const props = defineProps<{
  offers: TaskItemConversionOffer[] | undefined
  source: { id: string; face?: string }
  editor: () => TaskItemEditor | null
}>()

const schema = useSchemaStore()
const ui = useUIStore()

const allowed = computed(() => (props.offers ?? []).filter((o) => o.allowed))
const step = ref<'closed' | 'choose' | 'fields'>('closed')
const target = ref<TaskItemTarget | null>(null)
const offer = ref<TaskItemConversionOffer | null>(null)
const required = ref<string[]>([])
const values = reactive<Record<string, unknown>>({})
const busy = ref(false)
const error = ref('')
/** The item's busy mark, which also names the item the result goes to. */
let token: number | null = null
/** Set when the user dismissed the conversion; nothing is created after it. */
let cancelled = false

const targetType = computed(() => (offer.value ? schema.getEntityType(offer.value.to) : undefined))

function fieldDef(name: string): PropertyDef | undefined {
  return targetType.value?.properties?.[name]
}

function fieldLabel(name: string): string {
  return fieldDef(name)?.label || name
}

function enumOptions(name: string): SelectOption[] {
  const def = fieldDef(name)
  return (def?.values ?? []).map((v) => ({ value: v, label: def?.labels?.[v] || v }))
}

function isEnumList(name: string): boolean {
  const def = fieldDef(name)
  return def?.type === 'enum' && def.list === true
}

function listValue(name: string): string[] {
  const value = values[name]
  return Array.isArray(value) ? value.map(String) : []
}

function toggleChoice(name: string, choice: string, on: boolean) {
  const rest = listValue(name).filter((v) => v !== choice)
  values[name] = on ? [...rest, choice] : rest
}

/** Stores an input's text as the value the property's type expects. */
function setValue(name: string, input: string) {
  const def = fieldDef(name)
  if (def?.list) {
    values[name] = input
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean)
  } else {
    values[name] = input
  }
}

/**
 * Gives the fields an input cannot leave empty their empty value: a box the
 * user did not tick is false, not missing, and a list starts empty.
 */
function seedValues(names: string[]) {
  for (const name of names) {
    if (fieldDef(name)?.type === 'boolean') values[name] = false
    else if (fieldDef(name)?.list) values[name] = []
  }
}

/** Starts a conversion of `item`. */
function start(item: TaskItemTarget) {
  if (busy.value || step.value !== 'closed' || allowed.value.length === 0) return
  release()
  target.value = item
  cancelled = false
  token = props.editor()?.markTaskItemBusy(item) ?? null
  if (allowed.value.length === 1) {
    void choose(allowed.value[0])
    return
  }
  step.value = 'choose'
}

/**
 * Ends the dialog. A request still running ends the conversion when it
 * returns: a preflight then creates nothing, a create still reports.
 */
function close() {
  cancelled = true
  step.value = 'closed'
  error.value = ''
  for (const key of Object.keys(values)) delete values[key]
  if (!busy.value) release()
}

function release() {
  if (token != null) props.editor()?.clearTaskItemBusy(token)
  token = null
}

function request(properties?: Record<string, unknown>) {
  return {
    source_id: props.source.id,
    source_face: props.source.face ?? '',
    text: target.value?.text ?? '',
    ...(properties ? { properties } : {}),
  }
}

async function choose(next: TaskItemConversionOffer) {
  if (busy.value) return
  offer.value = next
  await run(async () => {
    const preview = await previewTaskItemConversion(next.name, request())
    // Dismissed while the preflight ran: the user no longer wants it.
    if (cancelled) return
    required.value = preview.required
    if (preview.required.length > 0) {
      seedValues(preview.required)
      step.value = 'fields'
      return
    }
    await commit()
  })
}

async function submitFields() {
  if (busy.value) return
  await run(() => commit({ ...values }))
}

async function commit(properties?: Record<string, unknown>) {
  const item = target.value
  const chosen = offer.value
  if (!item || !chosen) return
  let result: TaskItemConversionResult
  try {
    result = await convertTaskItem(chosen.name, request(properties))
  } catch (err) {
    const id = incompleteConversionEntity(err)
    if (id == null) throw err
    // The entity exists without its relation. Linking the item to it keeps
    // the item from being converted into a second one.
    close()
    const linked = apply(item, 'link', { id, title: '', entityType: chosen.to })
    const missing = `Created ${id}, but could not link it to this item's source. Link it by hand.`
    ui.error(linked ? missing : `${missing} The task item changed, so it was left as it is.`)
    return
  }
  close()
  const ref: EntityRefInsert = {
    id: result.entity_id,
    title: result.title,
    entityType: result.entity_type,
  }
  const applied = apply(item, result.item, ref)
  const warnings = result.warnings ?? []
  const codes = [...new Set(warnings.map((w) => w.code))].join(', ')
  const created = warnings.length
    ? `Created ${result.entity_id} with ${warnings.length} warning(s): ${codes}`
    : `Created ${result.entity_id}`
  if (!applied) {
    // The entity exists; only the edit to the body could not be made.
    ui.warning(`${created}, but the task item changed, so it was left as it is`)
  } else if (warnings.length) {
    ui.warning(created)
  } else {
    ui.success(created)
  }
}

/** Applies a result to the marked item; false without a mark or an editor. */
function apply(item: TaskItemTarget, mode: 'link' | 'remove', ref: EntityRefInsert): boolean {
  if (token == null) return false
  return props.editor()?.applyTaskItemResult(token, item.text, mode, ref) ?? false
}

/** Runs one request with the dialog disabled. */
async function run(fn: () => Promise<void>) {
  busy.value = true
  error.value = ''
  try {
    await fn()
  } catch (err) {
    const message = getErrorMessage(err, 'Conversion failed')
    if (step.value === 'closed') ui.error(message)
    else error.value = message
  } finally {
    busy.value = false
    // A closed dialog is the end of the conversion; an open one still needs the item.
    if (step.value === 'closed') release()
  }
}

/**
 * The dialog's elements, scrim included, for a host that ends its edit when
 * the user presses outside it. While a request runs the whole page counts:
 * ending the edit would unmount the editor the result goes into. Escape
 * still ends it, as an explicit act; the result is then reported instead.
 */
function panels(): Element[] {
  if (busy.value) return [document.documentElement]
  return Array.from(document.querySelectorAll('.task-item-dialog')).flatMap((panel) =>
    panel.parentElement ? [panel.parentElement] : []
  )
}

defineExpose({ start, panels })
</script>

<template>
  <RlModal
    v-if="step !== 'closed'"
    class="task-item-dialog"
    :open="true"
    :title="step === 'fields' && offer ? offer.label : 'Convert task item'"
    @close="close"
  >
    <p class="task-item-text">{{ target?.text }}</p>
    <div v-if="step === 'choose'" class="task-item-choices">
      <RlButton
        v-for="choice in allowed"
        :key="choice.name"
        variant="secondary"
        :loading="busy && offer?.name === choice.name"
        pending-label="Converting…"
        :disabled="busy && offer?.name !== choice.name"
        @click="choose(choice)"
        >{{ choice.label }}</RlButton
      >
    </div>
    <form v-else id="task-item-fields" class="task-item-fields" @submit.prevent="submitFields">
      <template v-for="name in required" :key="name">
        <fieldset v-if="isEnumList(name)" class="task-item-choice-list">
          <legend>{{ fieldLabel(name) }}</legend>
          <RlCheckbox
            v-for="option in enumOptions(name)"
            :key="option.value"
            :model-value="listValue(name).includes(option.value)"
            :label="option.label"
            @update:model-value="toggleChoice(name, option.value, $event)"
          />
        </fieldset>
        <RlSelect
          v-else-if="fieldDef(name)?.type === 'enum'"
          :model-value="String(values[name] ?? '')"
          :label="fieldLabel(name)"
          :options="enumOptions(name)"
          placeholder="Select…"
          required
          @update:model-value="values[name] = $event"
        />
        <RlCheckbox
          v-else-if="fieldDef(name)?.type === 'boolean'"
          :model-value="values[name] === true"
          :label="fieldLabel(name)"
          @update:model-value="values[name] = $event"
        />
        <RlNumberField
          v-else-if="fieldDef(name)?.type === 'integer'"
          :model-value="typeof values[name] === 'number' ? values[name] : undefined"
          :label="fieldLabel(name)"
          :step="1"
          required
          @update:model-value="values[name] = $event"
        />
        <RlDateField
          v-else-if="fieldDef(name)?.type === 'date'"
          :model-value="String(values[name] ?? '')"
          :label="fieldLabel(name)"
          required
          @update:model-value="values[name] = $event"
        />
        <RlFieldShell
          v-else-if="fieldDef(name)?.type === 'datetime'"
          v-slot="control"
          :label="fieldLabel(name)"
          required
        >
          <DatetimeWidget
            :id="control.id"
            mode="edit"
            :model-value="values[name] ?? ''"
            :property-def="fieldDef(name)"
            :property-name="name"
            :described-by="control.describedBy"
            @update:model-value="values[name] = $event"
          />
        </RlFieldShell>
        <RlTextField
          v-else
          :model-value="
            Array.isArray(values[name]) ? values[name].join(', ') : String(values[name] ?? '')
          "
          :label="fieldLabel(name)"
          :hint="fieldDef(name)?.list ? 'Separate values with commas.' : undefined"
          required
          autocomplete="off"
          @update:model-value="setValue(name, $event)"
        />
      </template>
    </form>
    <p v-if="error" role="alert">{{ error }}</p>
    <template #actions>
      <RlButtonGroup>
        <RlButton variant="secondary" :disabled="busy" @click="close">Cancel</RlButton>
        <RlButton
          v-if="step === 'fields'"
          variant="primary"
          type="submit"
          form="task-item-fields"
          :loading="busy"
          pending-label="Creating…"
          >Create</RlButton
        >
      </RlButtonGroup>
    </template>
  </RlModal>
</template>

<style scoped>
.task-item-text {
  margin: 0 0 var(--space-sm);
  color: var(--rl-color-text-muted);
}
.task-item-choices {
  display: grid;
  gap: var(--space-xs);
}
.task-item-fields {
  display: grid;
  gap: var(--space-sm);
}
.task-item-choice-list {
  display: grid;
  gap: var(--space-xs);
  margin: 0;
  padding: 0;
  border: 0;
}
</style>
