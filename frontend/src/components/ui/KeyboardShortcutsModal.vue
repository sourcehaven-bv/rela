<script setup lang="ts">
import { computed } from 'vue'

defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

const isMac = computed(() => /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent))
const mod = computed(() => (isMac.value ? '\u2318' : 'Ctrl'))

const shortcuts = computed(() => [
  {
    group: 'Global',
    items: [
      { keys: '?', description: 'Show keyboard shortcuts' },
      { keys: '/', description: 'Focus search' },
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
    ],
  },
  {
    group: 'Entity Detail',
    items: [{ keys: 'E', description: 'Edit entity' }],
  },
  {
    group: 'Form / Editor',
    items: [
      { keys: `${mod.value} + Enter`, description: 'Save / submit' },
      { keys: 'Esc', description: 'Cancel and go back' },
    ],
  },
])

function handleOverlayClick(e: MouseEvent) {
  if (e.target === e.currentTarget) {
    emit('close')
  }
}
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="shortcuts-overlay" @click="handleOverlayClick">
      <div class="shortcuts-modal">
        <div class="shortcuts-header">
          <h3>Keyboard Shortcuts</h3>
          <button class="close-btn" @click="emit('close')">&times;</button>
        </div>
        <div class="shortcuts-body">
          <div v-for="section in shortcuts" :key="section.group" class="shortcuts-group">
            <h4>{{ section.group }}</h4>
            <div v-for="item in section.items" :key="item.description" class="shortcut-row">
              <span class="shortcut-description">{{ item.description }}</span>
              <div class="shortcut-keys">
                <template v-for="(part, idx) in item.keys.split(' ')" :key="idx">
                  <span v-if="part === 'or' || part === 'then' || part === '+'" class="key-separator">
                    {{ part }}
                  </span>
                  <kbd v-else>{{ part }}</kbd>
                </template>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.shortcuts-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.shortcuts-modal {
  background: white;
  border-radius: 12px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.2);
  max-width: 600px;
  width: 90%;
  max-height: 80vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.shortcuts-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-color, #e2e8f0);
}

.shortcuts-header h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}

.close-btn {
  background: none;
  border: none;
  font-size: 24px;
  color: #64748b;
  cursor: pointer;
  padding: 0;
  line-height: 1;
}

.close-btn:hover {
  color: #1e293b;
}

.shortcuts-body {
  padding: 20px;
  overflow-y: auto;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 24px;
}

.shortcuts-group h4 {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: #64748b;
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
  color: #374151;
}

.shortcut-keys {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.shortcut-keys kbd {
  display: inline-block;
  padding: 2px 6px;
  font-size: 11px;
  font-family: ui-monospace, monospace;
  background: #f1f5f9;
  border: 1px solid #e2e8f0;
  border-radius: 4px;
  color: #475569;
}

.key-separator {
  font-size: 11px;
  color: #94a3b8;
  margin: 0 2px;
}
</style>
