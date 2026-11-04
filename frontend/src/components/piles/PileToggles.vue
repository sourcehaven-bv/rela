<script setup lang="ts">
/**
 * The `piles.toggles` buttons of an entity page: each puts the entity on a
 * declared pile (favorites) or takes it off again. The button is pressed
 * while the entity is on the pile, read from the pile's `addresses`, so it
 * costs no request of its own.
 *
 * It also tells the server the entity was opened, which marks it read on
 * every auto-read pile (notifications). That is sent only when the entity is
 * unread on one, so most page views send nothing.
 *
 * Renders nothing when piles are not available to this principal.
 */
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { usePiles } from '@/composables/usePiles'
import { useSchemaStore } from '@/stores'
import { pileIcon } from './pileIcon'
import RlIconButton from 'rela-components/components/common/RlIconButton.vue'

const props = defineProps<{
  /** The served address (`ID@face`) of the entity on screen. */
  address: string
  entityType: string
}>()

const { t } = useI18n()
const schemaStore = useSchemaStore()
const piles = usePiles()

const toggles = computed(() => {
  if (!piles.available.value) return []
  return (schemaStore.pilesConfig?.toggles ?? []).flatMap((tg) => {
    const types = tg.entity_types ?? []
    if (types.length && !types.includes(props.entityType)) return []
    const pile = piles.declared.value.find((p) => p.key === tg.pile)
    if (!pile) return []
    const on = piles.holds(tg.pile, props.address)
    const icon = tg.icon || pile.icon
    return [
      {
        key: tg.pile,
        on,
        icon: pileIcon(on ? tg.icon_active || icon : icon),
        label: on
          ? t('piles.toggle.remove', { name: tg.label || pile.name })
          : t('piles.toggle.add', { name: tg.label || pile.name }),
      },
    ]
  })
})

// Whether the entity is unread is read off the list, so this also fires when
// a notification about the open entity arrives while it is on screen.
const unreadHere = computed(() =>
  piles.declared.value.some(
    (p) => p.auto_read && (p.unread_addresses ?? []).includes(props.address)
  )
)
watch(
  [() => props.address, unreadHere],
  ([address, unread]) => {
    if (unread) void piles.markOpened(address)
  },
  { immediate: true }
)
</script>

<template>
  <div v-if="toggles.length" class="pile-toggles">
    <RlIconButton
      v-for="tg in toggles"
      :key="tg.key"
      :icon="tg.icon"
      :label="tg.label"
      :pressed="tg.on"
      :active="tg.on"
      data-testid="pile-toggle"
      @click="piles.toggle(tg.key, address)"
    />
  </div>
</template>

<style scoped>
/* A row of icon buttons at their own width, not stretched by the header. */
.pile-toggles {
  display: flex;
  gap: var(--space-xs);
  align-self: flex-start;
}
</style>
