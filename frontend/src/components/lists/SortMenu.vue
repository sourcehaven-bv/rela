<script setup lang="ts">
/**
 * Sort control for a list or a board: pick a property, then a direction.
 *
 * The control is a menu, so it is keyboard-operable the way RlMenu is (arrow
 * keys, Home/End, Escape). It edits only the primary key. A multi-key sort the
 * reader built with shift-click on column headers shows its first key here, and
 * choosing a property replaces the whole sort, as a plain header click does.
 *
 * It holds no state of its own: the owner keeps the `SortSpec[]` and turns it
 * into the `sort` query param.
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIcon from 'rela-components/components/common/RlIcon.vue'
import RlMenu from 'rela-components/components/overlay/RlMenu.vue'
import RlMenuItem from 'rela-components/components/overlay/RlMenuItem.vue'
import RlMenuSection from 'rela-components/components/overlay/RlMenuSection.vue'
import RlMenuSeparator from 'rela-components/components/overlay/RlMenuSeparator.vue'
import type { SortSpec } from '@/types'

export interface SortOption {
  property: string
  label: string
}

const props = defineProps<{
  options: SortOption[]
  modelValue: SortSpec[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: SortSpec[]]
}>()

const { t } = useI18n()

const active = computed(() => props.modelValue[0])
const activeLabel = computed(() => {
  const spec = active.value
  if (!spec) return ''
  return props.options.find((o) => o.property === spec.property)?.label ?? spec.property
})

function choose(property: string) {
  const spec = active.value
  emit('update:modelValue', [
    { property, direction: spec?.property === property ? spec.direction : 'asc' },
  ])
}

function setDirection(direction: SortSpec['direction']) {
  const spec = active.value
  if (spec) emit('update:modelValue', [{ property: spec.property, direction }])
}
</script>

<template>
  <RlMenu v-if="options.length" class="sort-menu" align="start">
    <template #trigger="{ toggle, attrs }">
      <RlButton variant="secondary" data-testid="sort-menu-button" v-bind="attrs" @click="toggle">
        <RlIcon
          :name="
            active?.direction === 'desc' ? 'arrow-down' : active ? 'arrow-up' : 'arrow-up-down'
          "
          :size="14"
          aria-hidden="true"
        />
        {{
          active ? t('list.sort.buttonActive', { property: activeLabel }) : t('list.sort.button')
        }}
      </RlButton>
    </template>

    <RlMenuSection :label="t('list.sort.menuLabel')" />
    <RlMenuItem
      :current="!active"
      data-testid="sort-default"
      @click="emit('update:modelValue', [])"
    >
      {{ t('list.sort.default') }}
    </RlMenuItem>
    <RlMenuItem
      v-for="option in options"
      :key="option.property"
      :current="active?.property === option.property"
      :data-sort-property="option.property"
      @click="choose(option.property)"
    >
      {{ option.label }}
    </RlMenuItem>
    <RlMenuSeparator />
    <RlMenuItem
      icon="arrow-up"
      :disabled="!active"
      :current="active?.direction === 'asc'"
      data-testid="sort-asc"
      @click="setDirection('asc')"
    >
      {{ t('list.sort.ascending') }}
    </RlMenuItem>
    <RlMenuItem
      icon="arrow-down"
      :disabled="!active"
      :current="active?.direction === 'desc'"
      data-testid="sort-desc"
      @click="setDirection('desc')"
    >
      {{ t('list.sort.descending') }}
    </RlMenuItem>
  </RlMenu>
</template>

<style scoped>
.sort-menu {
  align-self: stretch;
}
</style>
