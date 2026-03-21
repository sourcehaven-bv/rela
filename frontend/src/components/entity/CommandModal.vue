<script setup lang="ts">
import { ref } from 'vue'
import type { Command } from '@/types'

const props = defineProps<{
  entityId: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

// State
const showModal = ref(false)
const activeCommand = ref<Command | null>(null)
const running = ref(false)
const output = ref<Array<{ type: 'text' | 'file'; text?: string; path?: string; label?: string }>>([])
const success = ref<boolean | null>(null)

async function runCommand(cmd: Command) {
  if (cmd.confirm && !confirm(cmd.confirm)) {
    return
  }

  activeCommand.value = cmd
  running.value = true
  output.value = []
  success.value = null
  showModal.value = true

  const params = new URLSearchParams()
  params.set('entity_id', props.entityId)

  const url = `/api/command/${cmd.id}?${params.toString()}`

  try {
    const response = await fetch(url)
    if (!response.ok) {
      const text = await response.text()
      throw new Error(text || response.statusText)
    }

    const reader = response.body?.getReader()
    if (!reader) throw new Error('No response body')

    const decoder = new TextDecoder()
    let buffer = ''
    let currentEvent = 'message'

    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      for (const line of lines) {
        if (line.startsWith('event: ')) {
          currentEvent = line.substring(7).trim()
        } else if (line.startsWith('data: ')) {
          const data = line.substring(6)
          processSSEEvent(currentEvent, data, cmd)
          currentEvent = 'message'
        }
      }
    }

    // Stream ended without done event
    if (success.value === null) {
      success.value = true
      running.value = false
    }
  } catch (err) {
    output.value.push({ type: 'text', text: `Error: ${err instanceof Error ? err.message : 'Connection failed'}` })
    success.value = false
    running.value = false
  }
}

function processSSEEvent(eventType: string, rawData: string, cmd: Command) {
  try {
    const data = JSON.parse(rawData)
    switch (eventType) {
      case 'message':
      case 'log':
        output.value.push({ type: 'text', text: data.text || '' })
        break
      case 'file':
        output.value.push({
          type: 'file',
          path: data.path,
          label: data.label || data.path.split('/').pop() || 'File',
        })
        if (cmd.auto_open !== false && data.path) {
          fetch(`/api/open-file?path=${encodeURIComponent(data.path)}&action=open`, { method: 'POST' })
        }
        break
      case 'error':
        output.value.push({ type: 'text', text: `Error: ${data.text || 'Command error'}` })
        success.value = false
        running.value = false
        break
      case 'done':
        success.value = !!data.success
        running.value = false
        break
    }
  } catch {
    // Ignore parse errors
  }
}

function openFile(path: string) {
  fetch(`/api/open-file?path=${encodeURIComponent(path)}&action=open`, { method: 'POST' })
}

function revealFile(path: string) {
  fetch(`/api/open-file?path=${encodeURIComponent(path)}&action=reveal`, { method: 'POST' })
}

function close() {
  showModal.value = false
  activeCommand.value = null
  running.value = false
  emit('close')
}

// Expose run method to parent
defineExpose({ runCommand })
</script>

<template>
  <div v-if="showModal" class="modal-overlay" @click.self="!running && close()">
    <div class="modal command-modal">
      <div class="command-header">
        <h3>{{ activeCommand?.label }}</h3>
        <span v-if="running" class="command-status running">Running...</span>
        <span v-else-if="success === true" class="command-status success">Completed</span>
        <span v-else-if="success === false" class="command-status error">Failed</span>
      </div>
      <div class="command-output">
        <template v-if="output.length === 0">
          <div class="output-line">Starting...</div>
        </template>
        <template v-for="(item, idx) in output" :key="idx">
          <div v-if="item.type === 'text'" class="output-line">{{ item.text }}</div>
          <div v-else-if="item.type === 'file'" class="output-file">
            <span class="file-icon">📄</span>
            <span class="file-label">{{ item.label }}</span>
            <button class="file-btn" @click="openFile(item.path!)">Open</button>
            <button class="file-btn" @click="revealFile(item.path!)">Reveal</button>
          </div>
        </template>
      </div>
      <div class="modal-actions">
        <button
          class="btn btn-secondary"
          :disabled="running"
          @click="close"
        >
          Close
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Uses global .modal-overlay, .modal, .modal-actions from App.vue */

.command-modal {
  max-width: 600px;
  width: 90%;
}

.command-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.command-header h3 {
  margin: 0;
  flex: 1;
}

.command-status {
  font-size: 12px;
  font-weight: 600;
  padding: 4px 8px;
  border-radius: 4px;
}

.command-status.running {
  background: color-mix(in srgb, var(--warning-color, #f59e0b) 20%, transparent);
  color: var(--warning-color, #f59e0b);
}

.command-status.success {
  background: color-mix(in srgb, var(--success-color, #10b981) 20%, transparent);
  color: var(--success-color, #10b981);
}

.command-status.error {
  background: color-mix(in srgb, var(--error-color, #ef4444) 20%, transparent);
  color: var(--error-color, #ef4444);
}

.command-output {
  background: #1e293b;
  border-radius: 6px;
  padding: 16px;
  max-height: 400px;
  overflow: auto;
  margin-bottom: 16px;
}

.output-line {
  color: #e2e8f0;
  font-size: 13px;
  line-height: 1.6;
  font-family: monospace;
  white-space: pre-wrap;
  word-break: break-word;
}

.output-file {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  margin: 4px 0;
  background: #334155;
  border-radius: 4px;
}

.file-icon {
  font-size: 14px;
}

.file-label {
  flex: 1;
  color: #e2e8f0;
  font-size: 13px;
  font-family: monospace;
}

.file-btn {
  padding: 4px 10px;
  background: #475569;
  color: #e2e8f0;
  border: none;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
  transition: background 0.15s;
}

.file-btn:hover {
  background: #64748b;
}
</style>
