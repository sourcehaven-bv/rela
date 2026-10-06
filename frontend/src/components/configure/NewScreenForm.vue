<script setup lang="ts">
import { computed, ref } from 'vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlSelect from 'rela-components/components/form/RlSelect.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import { useConfigDraftStore } from '@/stores/configDraft'
import { entityTypeLabel, entityTypeNames } from '@/configure/models'
import { machineName } from '@/configure/names'
import { keys, get } from '@/configure/tree'

/** Asks for the title and entity type of a new form, list or board. */
const props = defineProps<{ section: 'forms' | 'lists' | 'kanbans'; what: string }>()
const emit = defineEmits<{ add: [id: string, title: string, entityType: string] }>()

const draft = useConfigDraftStore()
const title = ref('')
const entityType = ref(entityTypeNames(draft.currentSchema)[0] ?? '')

const typeOptions = computed(() =>
  entityTypeNames(draft.currentSchema).map((t) => ({
    value: t,
    label: entityTypeLabel(draft.currentSchema, t),
  }))
)
const id = computed(() => machineName(title.value, '_'))
const error = computed(() =>
  title.value.trim() && keys(get(draft.currentDataEntry, props.section)).includes(id.value)
    ? `A ${props.what} with this name exists.`
    : undefined
)

function submit() {
  if (!title.value.trim() || error.value || !entityType.value) return
  emit('add', id.value, title.value.trim(), entityType.value)
}
</script>

<template>
  <form class="new-screen" @submit.prevent="submit">
    <RlTextField v-model="title" :label="`Title of the new ${what}`" :error="error" />
    <RlSelect v-model="entityType" label="Entity type" :options="typeOptions" />
    <RlButton variant="primary" type="submit" :disabled="!title.trim() || !!error || !entityType">
      Add to draft
    </RlButton>
  </form>
</template>

<style scoped>
.new-screen {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  align-items: end;
  gap: var(--rl-space-3);
  max-width: 760px;
}
</style>
