<script setup lang="ts">
import { computed, ref, watch, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import RlButton from 'rela-components/components/common/RlButton.vue'
import { getChanges, describeSubject, type ChangeRecord } from '@/api/audit'
import { getErrorMessage, shouldDropHeldContent } from '@/api/errors'
import { isCancelledFetch } from '@/composables/usePageData'
import { useWorld } from '@/composables/useWorld'
import { useSchemaStore } from '@/stores/schema'
import { formatInstant } from '@/components/flows/flowView'

// The audit records of the entity face on screen (TKT-X1ZFO04), newest
// first. The server gates every record, so this renders what it is sent.
// Self-gating: absent unless the sidebar says a database audit log is wired.

const props = defineProps<{
  entityType: string
  /** The address on screen (`POL-1@draft`). */
  entityId: string
}>()

const { t } = useI18n()
const { worldParam } = useWorld()
const schemaStore = useSchemaStore()

const enabled = computed(() => schemaStore.auditChanges)
const changes = ref<ChangeRecord[]>([])
const nextBefore = ref<number | undefined>()
const loaded = ref(false)
const loadingMore = ref(false)
const error = ref('')
let controller: AbortController | null = null

/** The bare id on screen, for naming the other end of a relation. */
const selfId = computed(() => props.entityId.split('@')[0])

function reset() {
  changes.value = []
  nextBefore.value = undefined
  loaded.value = false
  error.value = ''
}

async function load(before?: number) {
  controller?.abort()
  const ctl = new AbortController()
  controller = ctl
  try {
    const page = await getChanges(
      props.entityType,
      props.entityId,
      { before, world: worldParam.value },
      ctl.signal
    )
    const got = page.changes ?? []
    changes.value = before ? [...changes.value, ...got] : got
    nextBefore.value = page.next_before || undefined
    loaded.value = true
    error.value = ''
  } catch (err) {
    if (isCancelledFetch(err)) return
    // A 404 means there is no change list; a refusal must not leave the
    // previous list on screen.
    if (shouldDropHeldContent(err)) {
      reset()
      return
    }
    error.value = getErrorMessage(err)
  }
}

async function loadMore() {
  loadingMore.value = true
  try {
    await load(nextBefore.value)
  } finally {
    loadingMore.value = false
  }
}

function actor(c: ChangeRecord): string {
  return c.user || c.tool || ''
}

watch(
  () => [enabled.value, props.entityType, props.entityId, worldParam.value],
  () => {
    controller?.abort()
    reset()
    if (enabled.value) void load()
  },
  { immediate: true }
)

onBeforeUnmount(() => controller?.abort())
</script>

<template>
  <section
    v-if="enabled && (loaded || error)"
    class="changes-panel"
    aria-labelledby="changes-heading"
  >
    <h2 id="changes-heading" class="panel-heading">{{ t('audit.changes.title') }}</h2>
    <p v-if="error" class="panel-error" role="alert">{{ error }}</p>
    <p v-else-if="changes.length === 0" class="panel-note">{{ t('audit.changes.empty') }}</p>
    <ol v-if="changes.length > 0" class="change-list">
      <li v-for="c in changes" :key="c.seq" class="change-item" data-testid="change-row">
        <div class="change-line">
          <time :datetime="c.time" class="change-time">{{ formatInstant(c.time) }}</time>
          <span class="change-user">{{ actor(c) }}</span>
          <span class="change-op">{{ c.op }}</span>
          <span class="change-subject">{{ describeSubject(c, selfId) }}</span>
        </div>
        <p v-if="c.summary" class="change-summary">{{ c.summary }}</p>
        <p v-if="c.trigger" class="change-trigger">
          {{ t('audit.triggeredBy', { trigger: c.trigger }) }}
        </p>
      </li>
    </ol>
    <div v-if="nextBefore" class="panel-more">
      <RlButton
        variant="secondary"
        size="sm"
        :loading="loadingMore"
        :pending-label="t('common.loading')"
        @click="loadMore"
      >
        {{ t('audit.loadMore') }}
      </RlButton>
    </div>
  </section>
</template>

<style scoped>
.changes-panel {
  background: var(--rl-color-bg-raised);
  border: 1px solid var(--rl-color-border);
  border-radius: 8px;
  padding: 16px 24px;
}

.panel-heading {
  margin: 0 0 12px;
  font-size: 18px;
  color: var(--rl-color-text);
}

.change-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.change-line {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: baseline;
}

.change-time,
.change-trigger {
  font-size: var(--font-size-sm);
  color: var(--rl-color-text-muted);
}

.change-op {
  font-weight: 600;
}

.change-summary,
.change-trigger {
  margin: 2px 0 0;
}

.panel-note,
.panel-error {
  margin: 0;
  font-size: 14px;
  color: var(--rl-color-text-muted);
}

.panel-error {
  color: var(--rl-color-danger, #c00);
}

.panel-more {
  margin-top: 12px;
}
</style>
