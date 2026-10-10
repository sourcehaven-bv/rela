<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import RlDrawer from 'rela-components/components/overlay/RlDrawer.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlButtonGroup from 'rela-components/components/common/RlButtonGroup.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlTextarea from 'rela-components/components/form/RlTextarea.vue'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import RlCheckbox from 'rela-components/components/form/RlCheckbox.vue'
import RlCallout from 'rela-components/components/feedback/RlCallout.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import LockedNote from './LockedNote.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import {
  choiceListNames,
  defaultText,
  entityTypeLabel,
  propertiesOf,
  typedDefault,
  type PropertyModel,
} from '@/configure/models'
import { BUILTIN_TYPES, machineName } from '@/configure/names'
import {
  ensureMap,
  getIn,
  isMap,
  newMap,
  remove,
  set,
  setOrRemove,
  str,
  get,
} from '@/configure/tree'

/**
 * Adds or edits one property of an entity type.
 *
 * A property's name is its key, so changing the name is a rename: the draft
 * records it, records keep their values under the new name, and the forms,
 * lists and boards that show it follow along.
 */
const props = defineProps<{
  entityType: string
  /** The property to edit, or null to add one. Undefined closes the drawer. */
  property: string | null | undefined
}>()
const emit = defineEmits<{ close: [] }>()

const draft = useConfigDraftStore()

const name = ref('')
const type = ref('string')
const required = ref(false)
const list = ref(false)
const unique = ref(false)
const description = ref('')
const defaultValue = ref('')
/** The default as loaded; undefined when it has no text form (a list). */
const loadedDefault = ref<string | undefined>('')

const open = computed(() => props.property !== undefined)
const isNew = computed(() => props.property === null)
const current = computed<PropertyModel | undefined>(() =>
  props.property
    ? propertiesOf(draft.currentSchema, props.entityType).find((p) => p.name === props.property)
    : undefined
)
const locked = computed(() => current.value?.managed === true)

watch(
  () => [props.property, open.value] as const,
  () => {
    const p = current.value
    name.value = p ? p.name : ''
    type.value = p?.type ?? 'string'
    required.value = p?.required ?? false
    list.value = p?.list ?? false
    unique.value = p?.unique ?? false
    description.value = p?.description ?? ''
    loadedDefault.value = p
      ? defaultText(
          get(
            getIn(draft.currentSchema, ['entities', props.entityType, 'properties', p.name]),
            'default'
          )
        )
      : ''
    defaultValue.value = loadedDefault.value ?? ''
  },
  { immediate: true }
)

const key = computed(() => {
  // An unchanged name keeps the key exactly, whatever its spelling.
  if (current.value && name.value === current.value.name) return current.value.name
  return machineName(name.value, '_')
})

const nameError = computed(() => {
  if (!name.value.trim()) return undefined
  if (!/^[a-z]/.test(key.value)) return 'Start the name with a letter.'
  const taken = propertiesOf(draft.currentSchema, props.entityType).some(
    (p) => p.name === key.value && p.name !== current.value?.name
  )
  return taken ? 'This entity type already has a property with this name.' : undefined
})

const typeOptions = computed(() => {
  const options = [
    ...BUILTIN_TYPES,
    ...choiceListNames(draft.currentSchema).map((n) => ({ value: n, label: `Choice list: ${n}` })),
  ]
  if (!options.some((o) => o.value === type.value))
    options.push({ value: type.value, label: type.value })
  return options
})

const renaming = computed(() => !!current.value && key.value !== current.value.name)
const becameRequired = computed(() => required.value && !current.value?.required)
const typeChanged = computed(() => !!current.value && type.value !== current.value.type)
const ready = computed(() => name.value.trim() !== '' && !nameError.value && !locked.value)
const typeLabel = computed(() => entityTypeLabel(draft.currentSchema, props.entityType))

function apply() {
  if (!ready.value) return
  const old = current.value?.name
  if (old && key.value !== old) draft.renameProperty(props.entityType, old, key.value)
  draft.edit('schema', (tree) => {
    const entity = getIn(tree, ['entities', props.entityType])
    if (!isMap(entity)) return
    const properties = ensureMap(entity, 'properties')
    const existing = get(properties, key.value)
    const def = isMap(existing) ? existing : newMap()
    if (!isMap(existing)) set(properties, key.value, def)
    // A property without a type is text; leave it that way rather than spell it out.
    if (str(get(def, 'type')) !== undefined || type.value !== 'string') set(def, 'type', type.value)
    setOrRemove(def, 'required', required.value || undefined)
    setOrRemove(def, 'list', list.value || undefined)
    setOrRemove(def, 'unique', unique.value || undefined)
    setOrRemove(def, 'description', description.value.trim() || undefined)
    // Leave an unedited default as it is, so a typed or list value keeps its form.
    if (
      loadedDefault.value !== undefined &&
      (defaultValue.value !== loadedDefault.value || typeChanged.value)
    )
      setOrRemove(def, 'default', typedDefault(defaultValue.value, type.value))
  })
  emit('close')
}

function removeProperty() {
  const old = current.value?.name
  if (!old) return
  draft.edit('schema', (tree) => {
    const properties = getIn(tree, ['entities', props.entityType, 'properties'])
    if (isMap(properties)) remove(properties, old)
  })
  draft.forgetProperty(props.entityType, old)
  emit('close')
}
</script>

<template>
  <RlDrawer
    :title="isNew ? 'New property' : `Property: ${current ? current.name : ''}`"
    size="md"
    :open="open"
    @close="emit('close')"
  >
    <form class="drawer-form" data-testid="config-property-drawer" @submit.prevent="apply">
      <LockedNote v-if="locked">
        The server sets this property, by computing it or by scanning an uploaded file. Change it in
        the project's files.
      </LockedNote>
      <RlTextField
        v-model="name"
        label="Name"
        required
        :disabled="locked"
        :error="nameError"
        hint="What people see on forms and in lists."
      />
      <RlCallout v-if="renaming" tone="info" title="Renamed">
        Records keep their values under the new name. Forms, lists and boards that show it follow.
      </RlCallout>
      <RlSelect v-model="type" label="Type" :options="typeOptions" :disabled="locked" />
      <RlCallout v-if="typeChanged" tone="warning" title="Type changed">
        Saving checks whether every {{ typeLabel }} has a value that fits the new type.
      </RlCallout>
      <RlCheckbox
        v-model="required"
        label="Required"
        :disabled="locked"
        :hint="`A ${typeLabel} cannot be saved without it.`"
      />
      <RlCallout v-if="becameRequired && !isNew" tone="info" title="Existing records">
        Records without a value stay valid until someone edits them.
      </RlCallout>
      <RlCheckbox v-model="list" label="Allow several values" :disabled="locked" />
      <RlCheckbox
        v-model="unique"
        label="Unique"
        hint="No two records may share a value."
        :disabled="locked"
      />
      <RlTextField
        v-model="defaultValue"
        label="Default"
        :disabled="locked || loadedDefault === undefined"
        :hint="
          loadedDefault === undefined
            ? 'This default is a list; edit it in schema.yaml.'
            : 'Filled in on new records.'
        "
      />
      <RlTextarea
        v-model="description"
        label="Help text"
        :rows="2"
        :disabled="locked"
        hint="Shown under the field on forms."
      />
      <div v-if="current && !locked" class="drawer-form__remove">
        <RlButton variant="ghost" tone="danger" icon="delete" size="sm" @click="removeProperty">
          Remove property
        </RlButton>
        <RlText size="sm" tone="muted">Values records hold for it are no longer shown.</RlText>
      </div>
    </form>
    <template #actions>
      <RlButtonGroup>
        <RlButton variant="secondary" @click="emit('close')">Cancel</RlButton>
        <RlButton
          variant="primary"
          :disabled="!ready"
          data-testid="config-apply-property"
          @click="apply"
        >
          {{ isNew ? 'Add to draft' : 'Apply' }}
        </RlButton>
      </RlButtonGroup>
    </template>
  </RlDrawer>
</template>

<style scoped>
.drawer-form {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-4);
}

.drawer-form__remove {
  display: flex;
  align-items: center;
  gap: var(--rl-space-3);
  padding-top: var(--rl-space-4);
  border-top: 1px solid var(--rl-color-border);
}
</style>
