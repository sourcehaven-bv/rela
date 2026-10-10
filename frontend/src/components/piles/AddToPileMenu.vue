<script setup lang="ts">
/**
 * "Add to pile": the user's piles, then a new pile made from the same items
 * (TKT-K3RJLH). Used by the list's bulk bar, the search results and the
 * entity page.
 *
 * Takes ADDRESSES (`ID` or `ID@face`), never bare ids: a pile holds the face
 * the user was looking at, and a bare id of a faced type is refused as
 * ambiguous. Callers pass each row's `entityRef`.
 *
 * Renders nothing when piles are not available to this principal.
 */
import { ref } from 'vue'
import { usePiles } from '@/composables/usePiles'
import type { Pile, PileSummary } from '@/api/piles'
import { pileIcon } from './pileIcon'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIcon from 'rela-components/components/common/RlIcon.vue'
import RlMenu from 'rela-components/components/overlay/RlMenu.vue'
import RlMenuItem from 'rela-components/components/overlay/RlMenuItem.vue'
import RlMenuSeparator from 'rela-components/components/overlay/RlMenuSeparator.vue'
import NewPileDialog from './NewPileDialog.vue'

const props = withDefaults(
  defineProps<{
    /** The addresses to add, read when a pile is chosen. */
    addresses: () => string[]
    label?: string
    /** The last entry's label: "New pile from selection…", "New pile…". */
    newLabel?: string
    variant?: 'primary' | 'secondary'
    size?: 'sm' | 'md'
    placement?: 'top' | 'bottom'
  }>(),
  {
    label: 'Add to pile',
    newLabel: 'New pile from selection…',
    variant: 'secondary',
    size: 'sm',
    placement: 'bottom',
  }
)

const emit = defineEmits<{
  /** Items went onto a pile, new or existing. */
  added: [pile: PileSummary | Pile]
}>()

const piles = usePiles()

/** The addresses captured when "New pile…" was chosen, so the dialog shows their count. */
const newPileItems = ref<string[] | null>(null)

async function addTo(pile: PileSummary) {
  const added = await piles.addItems(pile, props.addresses())
  if (added !== null) emit('added', pile)
}

function startNew() {
  newPileItems.value = props.addresses()
}
</script>

<template>
  <template v-if="piles.available.value">
    <RlMenu :placement="placement" align="start" data-testid="add-to-pile">
      <template #trigger="{ toggle, attrs }">
        <RlButton :variant="variant" :size="size" icon="layers" v-bind="attrs" @click="toggle">
          {{ label }}
          <template #trailing>
            <RlIcon
              :name="placement === 'top' ? 'chevron-up' : 'chevron-down'"
              :size="14"
              aria-hidden="true"
            />
          </template>
        </RlButton>
      </template>
      <RlMenuItem
        v-for="p in piles.piles.value"
        :key="p.id"
        :icon="pileIcon(p.icon)"
        data-testid="add-to-pile-item"
        @click="addTo(p)"
      >
        {{ p.name }}
      </RlMenuItem>
      <RlMenuSeparator v-if="piles.piles.value.length" />
      <RlMenuItem icon="plus" data-testid="add-to-new-pile" @click="startNew">{{
        newLabel
      }}</RlMenuItem>
    </RlMenu>
    <NewPileDialog
      v-if="newPileItems"
      :items="newPileItems"
      @saved="emit('added', $event)"
      @close="newPileItems = null"
    />
  </template>
</template>
