<script setup lang="ts">
import { computed, toRef } from 'vue'
import { useModalStack } from '@/composables/modalStack'
// RlModal brings Escape, the focus trap, focus restore and the scroll lock,
// none of which this dialog had — it even listed "Esc  Close modal" in its own
// table without implementing it.
import RlModal from 'rela-components/components/overlay/RlModal.vue'
import RlKbd from 'rela-components/components/data/RlKbd.vue'

const props = defineProps<{
  open: boolean
}>()

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
    group: 'Global',
    items: [
      { keys: '?', description: 'Show keyboard shortcuts' },
      { keys: '/', description: 'Focus search' },
      { keys: `${mod.value} + K`, description: 'Quick jump' },
      { keys: 'Esc', description: 'Close modal / cancel' },
    ],
  },
  {
    group: 'Navigation',
    items: [
      { keys: 'G then D', description: 'Go to Dashboard' },
      { keys: 'G then G', description: 'Go to Graph' },
      { keys: 'G then S', description: 'Go to Search' },
      { keys: 'G then A', description: 'Go to Analyze' },
    ],
  },
  {
    group: 'List View',
    items: [
      { keys: 'J or \u2193', description: 'Move selection down' },
      { keys: 'K or \u2191', description: 'Move selection up' },
      { keys: 'Enter or O', description: 'Open selected entity' },
      { keys: 'E', description: 'Edit selected entity' },
      { keys: 'N', description: 'Create new entity' },
      { keys: 'Del or Backspace', description: 'Delete selected entity' },
    ],
  },
  {
    group: 'Entity Detail',
    items: [
      { keys: 'E', description: 'Edit entity' },
      { keys: 'Del or Backspace', description: 'Delete entity' },
    ],
  },
  {
    group: 'Form / Editor',
    items: [
      { keys: `${mod.value} + Enter`, description: 'Save / submit' },
      { keys: 'Esc', description: 'Cancel and go back' },
    ],
  },
])
</script>

<template>
  <RlModal :open="open" title="Keyboard Shortcuts" size="lg" @close="emit('close')">
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
