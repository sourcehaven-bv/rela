<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import type { ScriptError } from '../../types/scriptError'
import RlButton from 'rela-components/components/common/RlButton.vue'

const props = defineProps<{ error: ScriptError }>()

const { t } = useI18n()

const stackOpen = ref(false)
const outputOpen = ref(false)
const argsOpen = ref(false)

const surfaceLabel = computed((): string => {
  switch (props.error.script.surface) {
    case 'action':
      return t('scriptError.surface.action')
    case 'document':
      return t('scriptError.surface.document')
    case 'automation':
      return t('scriptError.surface.automation')
    case 'lua_run':
      return t('scriptError.surface.luaRun')
    case 'lua_eval':
      return t('scriptError.surface.luaEval')
    case 'validation':
      return t('scriptError.surface.validation')
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
      <strong v-if="error.lua.line && error.lua.file">{{ t('scriptError.fileLine', { file: error.lua.file, line: error.lua.line }) }}</strong>
      <strong v-else-if="error.lua.line">{{ t('scriptError.line', { line: error.lua.line }) }}</strong>
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
      <summary>{{ t('scriptError.stack', { n: error.stack.length }, error.stack.length) }}</summary>
      <ol class="se-stack">
        <li v-for="(frame, i) in error.stack" :key="i">
          <code v-if="frame.path">{{ frame.path }}<span v-if="frame.line">:{{ frame.line }}</span></code>
          <span v-if="frame.func" class="se-func">{{ t('scriptError.inFunc', { func: frame.func }) }}</span>
        </li>
      </ol>
    </details>

    <details v-if="error.captured_output" :open="outputOpen" @toggle="outputOpen = !outputOpen">
      <summary>{{ t('scriptError.output') }}</summary>
      <pre class="se-output">{{ error.captured_output }}</pre>
    </details>

    <details v-if="argsEntries.length > 0" :open="argsOpen" @toggle="argsOpen = !argsOpen">
      <summary>{{ t('scriptError.args') }}</summary>
      <dl class="se-args">
        <template v-for="[k, v] in argsEntries" :key="k">
          <dt>{{ k }}</dt>
          <dd>{{ typeof v === 'string' ? v : JSON.stringify(v) }}</dd>
        </template>
      </dl>
    </details>

    <footer v-if="error.correlation_id" class="se-footer">
      <span class="se-corr-label">{{ t('scriptError.correlationId') }}</span>
      <code class="se-corr">{{ error.correlation_id }}</code>
      <RlButton variant="secondary" size="sm" @click="copyCorrelationId">{{ t('common.copy') }}</RlButton>
    </footer>
  </div>
</template>

<style scoped>
.script-error-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  font-family: var(--font-family, system-ui, sans-serif);
  color: var(--rl-color-text);
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
  background: var(--rl-color-bg-raised);
  border: 1px solid var(--rl-color-border);
  border-radius: 999px;
  font-size: 12px;
  color: var(--rl-color-text-muted);
}

.se-path {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 13px;
}

.se-entity {
  padding: 2px 8px;
  background: var(--rl-color-bg-raised);
  border-radius: 4px;
  font-size: 12px;
  color: var(--rl-color-text-muted);
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
  background: var(--rl-color-bg-raised);
  border: 1px solid var(--rl-color-border);
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
  color: var(--rl-color-text-muted);
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
  color: var(--rl-color-text-muted);
  font-size: 12px;
}

.se-output {
  margin: 8px 0 0;
  padding: 8px 12px;
  max-height: 200px;
  overflow: auto;
  background: var(--rl-color-bg-raised);
  border: 1px solid var(--rl-color-border);
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
  color: var(--rl-color-text-muted);
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
  color: var(--rl-color-text-muted);
}

.se-corr {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

details summary {
  cursor: pointer;
  font-size: 13px;
  color: var(--rl-color-text-muted);
}
</style>
