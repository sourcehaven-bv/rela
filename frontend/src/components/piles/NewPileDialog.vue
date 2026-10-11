<script setup lang="ts">
/**
 * Creates a pile, or renames and re-icons one (TKT-K3RJLH).
 *
 * The host mounts this under `v-if`, so the component's lifetime is the
 * dialog's, as with DuplicateModal. Creating can carry addresses: the rows
 * selected in a list, or an entity page's own row, go on the new pile in the
 * same request.
 *
 * The icon choices are the server's (`GET /_piles` → `icons`), because the
 * server validates the icon against its own allowlist.
 */
import { computed, ref } from 'vue'
import { usePiles } from '@/composables/usePiles'
import { PILE_NAME_MAX, type Pile, type PileSummary } from '@/api/piles'
import { isIconName } from 'rela-components/components/common/icons'
import { pileIconLabel } from './pileIcon'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlModal from 'rela-components/components/overlay/RlModal.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import RlTextField from 'rela-components/components/form/RlTextField.vue'
import RlSegmentedControl, {
  type SegmentOption,
} from 'rela-components/components/data/RlSegmentedControl.vue'

const props = defineProps<{
  /** The pile to rename. Absent: the dialog creates a new pile. */
  pile?: PileSummary
  /** Addresses to put on a new pile. Ignored when editing. */
  items?: string[]
}>()

const emit = defineEmits<{
  close: []
  saved: [pile: Pile]
}>()

const piles = usePiles()

const DEFAULT_ICON = 'layers'

const name = ref(props.pile?.name ?? '')
const icon = ref(props.pile?.icon ?? DEFAULT_ICON)
const saving = ref(false)
/** Set once the user has tried to submit, so an empty name is not an error on open. */
const attempted = ref(false)

const editing = computed(() => !!props.pile)
const itemCount = computed(() => (editing.value ? 0 : (props.items?.length ?? 0)))

const trimmed = computed(() => name.value.trim())
/** Characters, not UTF-16 units: the server counts runes. */
const nameLength = computed(() => [...trimmed.value].length)

const nameError = computed(() => {
  if (nameLength.value > PILE_NAME_MAX) return `Use at most ${PILE_NAME_MAX} characters`
  if (attempted.value && nameLength.value === 0) return 'Give the pile a name'
  return undefined
})
const valid = computed(() => nameLength.value > 0 && nameLength.value <= PILE_NAME_MAX)

/*
 * Only names the icon registry knows are offered, so a server icon this
 * build cannot draw is left out rather than shown blank. The current icon
 * stays, so an edit never silently changes it.
 */
const iconOptions = computed<SegmentOption[]>(() => {
  const names = piles.icons.value.length ? piles.icons.value : [DEFAULT_ICON]
  const list = names.includes(icon.value) ? names : [icon.value, ...names]
  return list
    .filter(isIconName)
    .map((value) => ({ value, label: pileIconLabel(value), icon: value }))
})

async function submit() {
  attempted.value = true
  if (!valid.value || saving.value) return
  saving.value = true
  try {
    const result = props.pile
      ? await piles.update(props.pile, { name: trimmed.value, icon: icon.value })
      : await piles.create({ name: trimmed.value, icon: icon.value, items: props.items ?? [] })
    // A failure has been toasted; the dialog stays open so the name can change.
    if (result) {
      emit('saved', result)
      emit('close')
    }
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <RlModal
    :open="true"
    :title="editing ? 'Rename pile' : 'New pile'"
    size="sm"
    :persistent="saving"
    @close="emit('close')"
  >
    <form id="new-pile-form" class="new-pile" novalidate @submit.prevent="submit">
      <RlText v-if="itemCount > 0" as="p" tone="subtle" size="sm" data-testid="new-pile-items">
        The {{ itemCount }} selected item{{ itemCount === 1 ? '' : 's' }} go on the new pile.
      </RlText>
      <RlTextField
        v-model="name"
        label="Name"
        placeholder="e.g. Friday review"
        required
        autocomplete="off"
        :error="nameError"
        data-testid="pile-name"
      />
      <div v-if="iconOptions.length > 1" class="new-pile__icon">
        <RlText size="sm" weight="medium">Icon</RlText>
        <RlSegmentedControl
          v-model="icon"
          :options="iconOptions"
          label="Icon"
          icon-only
          size="sm"
        />
      </div>
    </form>
    <template #actions>
      <RlButton :disabled="saving" @click="emit('close')">Cancel</RlButton>
      <RlButton
        variant="primary"
        type="submit"
        form="new-pile-form"
        :loading="saving"
        :pending-label="editing ? 'Saving…' : 'Creating…'"
        :disabled="!valid"
        data-testid="pile-submit"
      >
        {{ editing ? 'Save' : 'Create pile' }}
      </RlButton>
    </template>
  </RlModal>
</template>

<style scoped>
.new-pile {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-4);
}

.new-pile__icon {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--rl-space-3);
}
</style>
