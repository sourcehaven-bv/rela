<script setup lang="ts">
/**
 * The schema conversions offered for an entity. One allowed offer is a
 * button with its own label; several are one "Convert" menu, so the header
 * does not grow a button per conversion. Only allowed offers are listed; a
 * denied conversion is absent, as in CopyMenu.
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ConversionOffer } from '@/types'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlIcon from 'rela-components/components/common/RlIcon.vue'
import RlMenu from 'rela-components/components/overlay/RlMenu.vue'
import RlMenuItem from 'rela-components/components/overlay/RlMenuItem.vue'

const props = defineProps<{ offers?: ConversionOffer[] }>()
const emit = defineEmits<{ choose: [offer: ConversionOffer] }>()
const { t } = useI18n()

const allowed = computed(() => (props.offers ?? []).filter((o) => o.allowed))
const single = computed(() => (allowed.value.length === 1 ? allowed.value[0] : null))

function labelOf(o: ConversionOffer): string {
  return o.label || o.name
}
</script>

<template>
  <RlButton
    v-if="single"
    variant="secondary"
    class="conversion-single"
    @click="emit('choose', single)"
  >
    {{ labelOf(single) }}
  </RlButton>
  <RlMenu v-else-if="allowed.length > 1" class="conversion-menu">
    <template #trigger="{ toggle, attrs }">
      <RlButton variant="secondary" v-bind="attrs" @click="toggle">
        {{ t('entity.convertMenu') }}
        <template #trailing>
          <RlIcon name="chevron-down" :size="14" aria-hidden="true" />
        </template>
      </RlButton>
    </template>
    <RlMenuItem v-for="o in allowed" :key="o.name" @click="emit('choose', o)">
      {{ labelOf(o) }}
    </RlMenuItem>
  </RlMenu>
</template>
