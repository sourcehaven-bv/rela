<script setup lang="ts">
import { computed, ref } from 'vue'

import type { ScriptError } from '../../types/scriptError'

const props = defineProps<{ error: ScriptError }>()

const stackOpen = ref(false)
const outputOpen = ref(false)
const argsOpen = ref(false)

const surfaceLabel = computed((): string => {
  switch (props.error.script.surface) {
    case 'action':
      return 'Action'
    case 'document':
      return 'Document'
    case 'automation':
      return 'Automation'
    case 'lua_run':
      return 'Lua Run'
    case 'lua_eval':
      return 'Lua Eval'
    case 'validation':
      return 'Validation'
    default:
      return props.error.script.surface
  }
})

const argsEntries = computed((): [string, unknown][] => {
  const args = props.error.script.args
  if (!args) return []
  return Object.entries(args)
})

async function copyCorrelationId(): Promise<void> {
  if (!props.error.correlation_id) return
  try {
    await navigator.clipboard.writeText(props.error.correlation_id)
  } catch {
    // Older browsers / insecure contexts: leave it visible for manual copy.
  }
}
</script>

<template>
  <div class="script-error-panel">
    <header class="se-header">
      <span class="se-surface">{{ surfaceLabel }}</span>
      <code class="se-path">{{ error.script.path }}</code>
      <span v-if="error.script.entity_id" class="se-entity">{{ error.script.entity_id }}</span>
    </header>

    <p class="se-message">
      <strong v-if="error.lua.line">line {{ error.lua.line }}:</strong>
      {{ error.lua.message }}
    </p>

    <div v-if="error.source && error.source.length > 0" class="se-source">
      <pre><code><span
        v-for="line in error.source"
        :key="line.n"
        :class="{ 'se-line': true, 'se-highlight': line.highlight }"
      ><span class="se-lineno">{{ line.n }}</span>{{ line.text }}
</span></code></pre>
    </div>

    <details v-if="error.stack && error.stack.length > 0" :open="stackOpen" @toggle="stackOpen = !stackOpen">
      <summary>Stack ({{ error.stack.length }} frame{{ error.stack.length === 1 ? '' : 's' }})</summary>
      <ol class="se-stack">
        <li v-for="(frame, i) in error.stack" :key="i">
          <code v-if="frame.path">{{ frame.path }}<span v-if="frame.line">:{{ frame.line }}</span></code>
          <span v-if="frame.func" class="se-func">in {{ frame.func }}</span>
        </li>
      </ol>
    </details>

    <details v-if="error.captured_output" :open="outputOpen" @toggle="outputOpen = !outputOpen">
      <summary>Output before error</summary>
      <pre class="se-output">{{ error.captured_output }}</pre>
    </details>

    <details v-if="argsEntries.length > 0" :open="argsOpen" @toggle="argsOpen = !argsOpen">
      <summary>Args</summary>
      <dl class="se-args">
        <template v-for="[k, v] in argsEntries" :key="k">
          <dt>{{ k }}</dt>
          <dd>{{ typeof v === 'string' ? v : JSON.stringify(v) }}</dd>
        </template>
      </dl>
    </details>

    <footer v-if="error.correlation_id" class="se-footer">
      <span class="se-corr-label">Correlation ID:</span>
      <code class="se-corr">{{ error.correlation_id }}</code>
      <button type="button" class="se-copy" @click="copyCorrelationId">Copy</button>
    </footer>
  </div>
</template>

<style scoped>
.script-error-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  font-family: var(--font-family, system-ui, sans-serif);
  color: var(--text-color);
}

.se-header {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.se-surface {
  display: inline-block;
  padding: 2px 8px;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 999px;
  font-size: 12px;
  color: var(--muted-text);
}

.se-path {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 13px;
}

.se-entity {
  padding: 2px 8px;
  background: var(--card-bg);
  border-radius: 4px;
  font-size: 12px;
  color: var(--muted-text);
}

.se-message {
  margin: 0;
  padding: 8px 12px;
  background: rgba(220, 50, 50, 0.08);
  border-left: 3px solid #d33;
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 13px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.se-source {
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 4px;
  overflow: auto;
  max-height: 240px;
}

.se-source pre {
  margin: 0;
  padding: 8px 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  line-height: 1.5;
}

.se-line {
  display: block;
  padding: 0 12px;
}

.se-highlight {
  background: rgba(220, 50, 50, 0.12);
}

.se-lineno {
  display: inline-block;
  width: 32px;
  margin-right: 12px;
  color: var(--muted-text);
  text-align: right;
  user-select: none;
}

.se-stack {
  margin: 8px 0 0;
  padding-left: 24px;
  font-size: 13px;
}

.se-stack li {
  margin: 4px 0;
}

.se-stack code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.se-func {
  margin-left: 8px;
  color: var(--muted-text);
  font-size: 12px;
}

.se-output {
  margin: 8px 0 0;
  padding: 8px 12px;
  max-height: 200px;
  overflow: auto;
  background: var(--card-bg);
  border: 1px solid var(--border-color);
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  white-space: pre-wrap;
}

.se-args {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 4px 12px;
  margin: 8px 0 0;
  font-size: 13px;
}

.se-args dt {
  color: var(--muted-text);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.se-args dd {
  margin: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  word-break: break-all;
}

.se-footer {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--muted-text);
}

.se-corr {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.se-copy {
  padding: 2px 8px;
  border: 1px solid var(--border-color);
  background: var(--card-bg);
  border-radius: 4px;
  font-size: 11px;
  cursor: pointer;
}

.se-copy:hover {
  background: var(--hover-bg, var(--border-color));
}

details summary {
  cursor: pointer;
  font-size: 13px;
  color: var(--muted-text);
}
</style>
