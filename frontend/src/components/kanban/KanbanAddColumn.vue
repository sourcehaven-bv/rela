<script setup lang="ts">
/**
 * "Add column" at the end of a kanban board (TKT-1X5G3BP).
 *
 * A column is a value of the board's enum property, so a new one is a
 * configuration write. It goes through the Configure API, the only
 * server-side config writer: fetch the snapshot, append the value
 * (`addEnumColumn`), save it with the snapshot's version. The server checks
 * `config:edit` and its allowlist again; the board only hides the control
 * from someone who cannot configure.
 *
 * A save makes the server broadcast `config-changed`, and every open tab
 * reloads on it. This tab reloads itself once the save has answered, so it
 * marks the save in flight first, as the Configure space does.
 */
import { nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import { getErrorMessage } from '@/api'
import { getConfigure, saveConfigure } from '@/api/configure'
import { addEnumColumn, type AddColumnTarget } from '@/configure/addEnumColumn'
import { machineName } from '@/configure/names'
import { beginConfigSave, endConfigSave, setConfigIncomplete } from '@/configure/saveGate'

const props = defineProps<{ target: AddColumnTarget }>()

const { t } = useI18n()
const open = ref(false)
const text = ref('')
const busy = ref(false)
const error = ref('')
const field = ref<InstanceType<typeof RlTextField> | null>(null)
const button = ref<InstanceType<typeof RlButton> | null>(null)

function focusIn(component: { $el?: unknown } | null, selector: string) {
  const el = component?.$el
  if (!(el instanceof HTMLElement)) return
  ;(el.matches(selector) ? el : el.querySelector<HTMLElement>(selector))?.focus()
}

async function start() {
  open.value = true
  await nextTick()
  focusIn(field.value, 'input')
}

// Focus goes back to the button, so a keyboard user is not left on the page.
async function cancel() {
  if (busy.value) return
  open.value = false
  text.value = ''
  error.value = ''
  await nextTick()
  focusIn(button.value, 'button')
}

async function onEnter(event: KeyboardEvent) {
  if (event.isComposing || busy.value) return
  // A name with no letter or digit is refused before anything is fetched.
  if (!machineName(text.value)) {
    error.value = t('kanban.addColumnNoName')
    return
  }
  busy.value = true
  error.value = ''
  let reloading = false
  try {
    const snapshot = await getConfigure()
    const edit = addEnumColumn(snapshot, props.target, text.value)
    if (!('draft' in edit)) {
      error.value =
        edit.reason === 'no_name'
          ? t('kanban.addColumnNoName')
          : edit.reason === 'exists'
            ? t('kanban.addColumnExists', { name: edit.name })
            : t('kanban.addColumnUnsupported')
      return
    }
    beginConfigSave()
    const result = await saveConfigure(edit.draft)
    if (!result.saved) {
      const problems = (result.problems ?? []).map((p) => p.message).join(' ')
      error.value = problems || t('kanban.addColumnFailed')
      return
    }
    if (result.incomplete) setConfigIncomplete(true)
    reloading = true
    window.location.reload()
  } catch (err) {
    error.value = getErrorMessage(err, t('kanban.addColumnFailed'))
  } finally {
    if (!reloading) {
      endConfigSave()
      busy.value = false
    }
  }
}
</script>

<template>
  <div class="kanban-add-column" data-testid="kanban-add-column">
    <RlButton v-if="!open" ref="button" variant="ghost" size="sm" @click="start">
      {{ t('kanban.addColumn') }}
    </RlButton>
    <template v-else>
      <RlTextField
        ref="field"
        v-model="text"
        size="sm"
        :label="t('kanban.addColumnLabel')"
        label-hidden
        :placeholder="t('kanban.addColumnPlaceholder')"
        :readonly="busy"
        :error="error || undefined"
        :hint="busy ? t('kanban.addColumnSaving') : undefined"
        autocomplete="off"
        @keydown.enter.prevent="onEnter"
        @keydown.esc.stop="cancel"
      />
      <RlButton variant="ghost" size="sm" :disabled="busy" @click="cancel">
        {{ t('kanban.addColumnCancel') }}
      </RlButton>
    </template>
  </div>
</template>

<style scoped>
.kanban-add-column {
  flex: none;
  align-self: flex-start;
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-1);
  width: 16rem;
}
</style>
