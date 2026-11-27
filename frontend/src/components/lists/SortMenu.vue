<script setup lang="ts">
/**
 * Sort control for a list or a board: pick a property, then a direction.
 *
 * The control is a menu, so it is keyboard-operable the way RlMenu is (arrow
 * keys, Home/End, Escape). Choosing a property replaces the whole sort, as a
 * plain header click does; the direction rows change the primary key only. A
 * multi-key sort the reader built with shift-click on column headers is listed
 * key by key in its own section. Choosing a listed key flips its direction, and
 * flipping a descending key removes it, as a shift-click does.
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
}>()

const model = defineModel<SortSpec[]>({ required: true })

const { t } = useI18n()

const active = computed(() => model.value[0])
function labelOf(property: string): string {
  return props.options.find((o) => o.property === property)?.label ?? property
}
const activeLabel = computed(() => (active.value ? labelOf(active.value.property) : ''))
const extraKeys = computed(() => model.value.length - 1)
const multi = computed(() => model.value.length > 1)

function choose(property: string) {
  const spec = active.value
  model.value = [{ property, direction: spec?.property === property ? spec.direction : 'asc' }]
}

function setDirection(direction: SortSpec['direction']) {
  const spec = active.value
  if (spec) model.value = [{ property: spec.property, direction }, ...model.value.slice(1)]
}

/** Flip one key of a multi-key sort: ascending to descending, descending off. */
function cycleKey(index: number) {
  const next = model.value.map((spec) => ({ ...spec }))
  if (next[index].direction === 'asc') next[index].direction = 'desc'
  else next.splice(index, 1)
  model.value = next
}
</script>

<template>
  <RlMenu v-if="options.length" class="sort-menu" align="start">
    <template #trigger="{ toggle, attrs }">
      <RlButton variant="secondary" data-testid="sort-menu-button" v-bind="attrs" @click="toggle">
        <!-- In the icon slot, beside the label: inside the label the block
             icon took a line of its own, above the word. -->
        <template #icon>
          <RlIcon
            :name="
              active?.direction === 'desc' ? 'arrow-down' : active ? 'arrow-up' : 'arrow-up-down'
            "
            :size="14"
            aria-hidden="true"
          />
        </template>
        {{
          multi
            ? t('list.sort.buttonMulti', { property: activeLabel, count: extraKeys })
            : active
              ? t('list.sort.buttonActive', { property: activeLabel })
              : t('list.sort.button')
        }}
      </RlButton>
    </template>

    <RlMenuSection :label="t('list.sort.menuLabel')" />
    <RlMenuItem :current="!active" data-testid="sort-default" @click="model = []">
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
    <template v-if="multi">
      <RlMenuSeparator />
      <RlMenuSection :label="t('list.sort.keysLabel')" />
      <RlMenuItem
        v-for="(spec, index) in model"
        :key="spec.property"
        :icon="spec.direction === 'desc' ? 'arrow-down' : 'arrow-up'"
        :data-sort-key="spec.property"
        @click="cycleKey(index)"
      >
        {{
          t(spec.direction === 'desc' ? 'list.sort.keyDescending' : 'list.sort.keyAscending', {
            property: labelOf(spec.property),
          })
        }}
      </RlMenuItem>
    </template>
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
