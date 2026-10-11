<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import RlDrawer from 'rela-components/components/overlay/RlDrawer.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlButtonGroup from 'rela-components/components/common/RlButtonGroup.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlTextarea from 'rela-components/components/form/RlTextarea.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { entityTypeNames } from '@/configure/models'
import { machineName } from '@/configure/names'
import { ensureMap, newMap, set } from '@/configure/tree'
import { configureRoute } from '@/configure/routes'

/**
 * Adds an entity type to the draft. It starts with one property, Title, so
 * its records have something to be called by; the rest is added on its page.
 */
const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()

const draft = useConfigDraftStore()
const router = useRouter()

const label = ref('')
const prefix = ref('')
const description = ref('')

watch(
  () => props.open,
  (open) => {
    if (!open) return
    label.value = ''
    prefix.value = ''
    description.value = ''
  }
)

const name = computed(() => machineName(label.value, '-'))
const nameError = computed(() => {
  if (!label.value.trim()) return undefined
  if (!/^[a-z]/.test(name.value)) return 'Start the name with a letter.'
  if (entityTypeNames(draft.currentSchema).includes(name.value))
    return 'An entity type with this name exists.'
  return undefined
})
const suggestedPrefix = computed(
  () =>
    `${name.value
      .replace(/[^a-z]/g, '')
      .slice(0, 3)
      .toUpperCase()}-`
)
const ready = computed(() => label.value.trim() !== '' && !nameError.value)

function add() {
  if (!ready.value) return
  const id = name.value
  draft.edit('schema', (tree) => {
    const entities = ensureMap(tree, 'entities')
    set(
      entities,
      id,
      newMap({
        label: label.value.trim(),
        description: description.value.trim() || undefined,
        id_type: 'short',
        id_prefix: prefix.value.trim() || suggestedPrefix.value,
        properties: newMap({ title: newMap({ type: 'string', required: true }) }),
      })
    )
  })
  emit('close')
  void router.push(configureRoute.entityType(id))
}
</script>

<template>
  <RlDrawer title="New entity type" size="md" :open="open" @close="emit('close')">
    <form class="drawer-form" @submit.prevent="add">
      <RlTextField
        v-model="label"
        label="Name"
        required
        :error="nameError"
        hint="One record of this type, such as Ticket or Decision."
      />
      <RlTextField
        v-model="prefix"
        label="ID prefix"
        :placeholder="label ? suggestedPrefix : 'TKT-'"
        hint="IDs are this prefix and six random characters."
      />
      <RlTextarea v-model="description" label="Description" :rows="2" />
    </form>
    <template #actions>
      <RlButtonGroup>
        <RlButton variant="secondary" @click="emit('close')">Cancel</RlButton>
        <RlButton
          variant="primary"
          :disabled="!ready"
          data-testid="config-add-entity-type"
          @click="add"
        >
          Add to draft
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
</style>
