<script setup lang="ts">
import { computed, toRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { useModalStack } from '@/composables/modalStack'
// RlModal brings Escape, the focus trap, focus restore and the scroll lock,
// none of which this dialog had — it even listed "Esc  Close modal" in its own
// table without implementing it.
import RlModal from 'rela-components/components/overlay/RlModal.vue'
import RlKbd from 'rela-components/components/data/RlKbd.vue'

const props = defineProps<{
  open: boolean
}>()

const { t } = useI18n()

// rela's separate registry, which is what stops the global shortcut handler
// acting while this is up. Notably absent before: pressing "?" over the
// shortcut list re-triggered the very handler that opened it.
useModalStack(toRef(props, 'open'))

const emit = defineEmits<{
  close: []
}>()

/**
 * Which separator a shortcut string uses.
 *
 * Each entry below uses exactly one, and RlKbd takes one at a time — it splits
 * on the separator and reads it out, so `G then D` is announced as three
 * tokens rather than as the unpronounceable string "GthenD". Defaulting to
 * `+` is safe: a single key has nothing to split on.
 */
function separatorFor(keys: string): '+' | 'then' | 'or' {
  if (keys.includes(' then ')) return 'then'
  if (keys.includes(' or ')) return 'or'
  return '+'
}

const isMac = computed(() => /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent))
const mod = computed(() => (isMac.value ? '\u2318' : 'Ctrl'))

const shortcuts = computed(() => [
  {
    group: t('shortcuts.group.global'),
    items: [
      { keys: '?', description: t('shortcuts.showShortcuts') },
      { keys: '/', description: t('shortcuts.focusSearch') },
      { keys: `${mod.value} + K`, description: t('shortcuts.quickJump') },
      { keys: 'Esc', description: t('shortcuts.closeModal') },
    ],
  },
  {
    group: t('shortcuts.group.navigation'),
    items: [
      { keys: 'G then D', description: t('shortcuts.goDashboard') },
      { keys: 'G then G', description: t('shortcuts.goGraph') },
      { keys: 'G then S', description: t('shortcuts.goSearch') },
      { keys: 'G then A', description: t('shortcuts.goAnalyze') },
    ],
  },
  {
    group: t('shortcuts.group.listView'),
    items: [
      { keys: 'J or \u2193', description: t('shortcuts.moveDown') },
      { keys: 'K or \u2191', description: t('shortcuts.moveUp') },
      { keys: 'Enter or O', description: t('shortcuts.openSelected') },
      { keys: 'E', description: t('shortcuts.editSelected') },
      { keys: 'N', description: t('shortcuts.createNew') },
      { keys: 'Del or Backspace', description: t('shortcuts.deleteSelected') },
    ],
  },
  {
    group: t('shortcuts.group.entityDetail'),
    items: [
      { keys: 'E', description: t('shortcuts.editEntity') },
      { keys: 'Del or Backspace', description: t('shortcuts.deleteEntity') },
    ],
  },
  {
    group: t('shortcuts.group.formEditor'),
    items: [
      { keys: `${mod.value} + Enter`, description: t('shortcuts.submit') },
      { keys: 'Esc', description: t('shortcuts.cancelBack') },
    ],
  },
])
</script>

<template>
  <RlModal :open="open" :title="t('shortcuts.title')" size="lg" @close="emit('close')">
    <div class="shortcuts-body">
      <div v-for="section in shortcuts" :key="section.group" class="shortcuts-group">
        <h4>{{ section.group }}</h4>
        <div v-for="item in section.items" :key="item.description" class="shortcut-row">
          <span class="shortcut-description">{{ item.description }}</span>
          <!--
            RlKbd owns the splitting, the key chrome and the separator. It also
            gives the combination an accessible name, which the hand-rolled
            version had no way to do: a screen reader met a bare glyph.
          -->
          <RlKbd class="shortcut-keys" :keys="item.keys" :separator="separatorFor(item.keys)" />
        </div>
      </div>
    </div>
  </RlModal>
</template>

<style scoped>
/* The overlay, panel, title row and close button are all RlModal's now. */

.shortcuts-body {
  /* RlModal's body already pads and scrolls; this only sets the columns. */
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 24px;
}

.shortcuts-group h4 {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--rl-color-text-muted);
  margin: 0 0 12px;
}

.shortcut-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 0;
  gap: 16px;
}

.shortcut-description {
  font-size: 13px;
  color: var(--rl-color-text);
}

/* RlKbd lays out its own keys and styles both them and the separator; only
   the refusal to shrink beside a long description is this dialog's. */
.shortcut-keys {
  flex-shrink: 0;
}
</style>
