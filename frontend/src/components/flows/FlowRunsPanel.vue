<script setup lang="ts">
/**
 * The flow runs on the entity page (TKT-ZAXRXND): every run on this entity
 * the reader can see, newest first, and a Start button per manual flow on
 * this type.
 *
 * Self-gating: it renders nothing unless a flow runs on this type, so a
 * project without flows (where `/_flows` answers 404) sees the page it
 * always saw. A refused read drops the runs on screen (BUG-E8XE7I).
 */
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import RlButton from 'rela-components/components/common/RlButton.vue'
import { listSubjectRuns, startFlowRun, type Flow, type FlowRun } from '@/api/flows'
import { getErrorMessage, shouldDropHeldContent } from '@/api/errors'
import { isCancelledFetch } from '@/composables/usePageData'
import { useEvents } from '@/composables/useEvents'
import { useFlowList } from '@/composables/useFlowList'
import { useUIStore } from '@/stores'
import FlowRunCard from './FlowRunCard.vue'

/** The entity type of a run header; a run's moves change it. */
const FLOW_RUN_TYPE = '_flow-run'

const props = defineProps<{
  entityType: string
  /** The address on screen (`PER-1` or `POL-1@draft`). */
  entityId: string
}>()

const { t } = useI18n()
const ui = useUIStore()
const { flows } = useFlowList()
const { on } = useEvents()

const runs = ref<FlowRun[]>([])
const error = ref('')
const starting = ref('')
let controller: AbortController | null = null

/** The flows that run on this type. */
const typeFlows = computed<Flow[]>(() => flows.value.filter((f) => f.subject === props.entityType))
const startable = computed(() => typeFlows.value.filter((f) => f.manual))

async function load() {
  if (typeFlows.value.length === 0 || !props.entityId) {
    runs.value = []
    return
  }
  controller?.abort()
  const ctl = new AbortController()
  controller = ctl
  try {
    runs.value = await listSubjectRuns(props.entityId, ctl.signal)
    error.value = ''
  } catch (err) {
    if (isCancelledFetch(err)) return
    if (shouldDropHeldContent(err)) {
      runs.value = []
      error.value = ''
      return
    }
    error.value = getErrorMessage(err, t('flows.panel.loadFailed'))
  }
}

watch(
  () => [props.entityType, props.entityId, typeFlows.value.length],
  () => {
    runs.value = []
    error.value = ''
    void load()
  },
  { immediate: true }
)

// A step, a timer or another reader moves a run; its header and the subject
// change with it. The event names no subject, so a burst of run changes
// anywhere is folded into one reload.
let reloadTimer: ReturnType<typeof setTimeout> | undefined
function reloadSoon() {
  clearTimeout(reloadTimer)
  reloadTimer = setTimeout(() => void load(), 300)
}
on('entity:changed', (data) => {
  if (data?.type === FLOW_RUN_TYPE || data?.type === props.entityType) reloadSoon()
})
on('refresh', () => void load())

onBeforeUnmount(() => {
  clearTimeout(reloadTimer)
  controller?.abort()
})

async function start(flow: Flow) {
  if (starting.value) return
  starting.value = flow.name
  try {
    await startFlowRun(flow.name, props.entityId)
    ui.success(t('flows.panel.started', { flow: flow.label || flow.name }))
    await load()
  } catch (err) {
    ui.error(getErrorMessage(err, t('flows.panel.startFailed')))
  } finally {
    starting.value = ''
  }
}

/**
 * Shown when there is a run, a flow to start or an error. A start the
 * server refuses (a missing permission, or `per_subject: one` with a run
 * already here) answers with a message that says why.
 */
const visible = computed(
  () =>
    typeFlows.value.length > 0 &&
    (runs.value.length > 0 || startable.value.length > 0 || !!error.value)
)
</script>

<template>
  <section
    v-if="visible"
    class="flow-runs-panel"
    aria-labelledby="flow-runs-heading"
    data-testid="flow-runs-panel"
  >
    <header class="flow-runs-panel__header">
      <h2 id="flow-runs-heading" class="flow-runs-panel__heading">{{ t('flows.panel.title') }}</h2>
      <div v-if="startable.length > 0" class="flow-runs-panel__start">
        <RlButton
          v-for="f in startable"
          :key="f.name"
          variant="secondary"
          size="sm"
          icon="workflow"
          :loading="starting === f.name"
          :pending-label="t('flows.panel.starting')"
          :data-testid="`flow-start-${f.name}`"
          @click="start(f)"
        >
          {{ t('flows.panel.start', { flow: f.label || f.name }) }}
        </RlButton>
      </div>
    </header>
    <p v-if="error" class="flow-runs-panel__error" role="alert">{{ error }}</p>
    <p v-if="runs.length === 0 && !error" class="flow-runs-panel__empty">
      {{ t('flows.panel.empty') }}
    </p>
    <div class="flow-runs-panel__runs">
      <FlowRunCard
        v-for="r in runs"
        :key="r.id"
        :run="r"
        :subject-type="entityType"
        link-title
        @changed="load"
        @gone="load"
      />
    </div>
  </section>
</template>

<style scoped>
.flow-runs-panel {
  background: var(--rl-color-bg-raised);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--radius-lg);
  padding: var(--space-md) var(--space-xl);
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.flow-runs-panel__header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-sm);
}

.flow-runs-panel__heading {
  margin: 0;
  font-size: 18px;
  color: var(--rl-color-text);
}

.flow-runs-panel__start {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-sm);
}

.flow-runs-panel__error {
  margin: 0;
  color: var(--rl-color-danger);
}

.flow-runs-panel__empty {
  margin: 0;
  font-size: var(--font-size-sm);
  color: var(--rl-color-text-muted);
}

.flow-runs-panel__runs {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}
</style>
