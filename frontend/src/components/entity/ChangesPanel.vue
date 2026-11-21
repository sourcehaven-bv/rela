<script setup lang="ts">
import { computed, ref, watch, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import RlButton from 'rela-components/components/common/RlButton.vue'
import { getChanges, type ChangeRecord } from '@/api/audit'
import { getErrorMessage, shouldDropHeldContent } from '@/api/errors'
import { isCancelledFetch } from '@/composables/usePageData'
import { useWorld } from '@/composables/useWorld'
import { useSchemaStore } from '@/stores/schema'
import { formatInstant } from '@/components/flows/flowView'
import {
  actorText,
  opText,
  subjectText,
  summaryText,
  triggerText,
  type ChangeSchema,
} from './changeText'

// The audit records of the entity face on screen (TKT-X1ZFO04), newest
// first. The server gates every record, so this renders what it is sent.
// Self-gating: absent unless the sidebar says a database audit log is wired,
// and absent where `header_buttons.history` hides the history (BUG-XR1DUQA).
// It names fields, relations and entities by label and title, as the rest of
// the page does; the raw audit values stay in the operators' audit log.

const props = defineProps<{
  entityType: string
  /** The address on screen (`POL-1@draft`). */
  entityId: string
}>()

const { t } = useI18n()
const { worldParam } = useWorld()
const schemaStore = useSchemaStore()

const enabled = computed(() => schemaStore.auditChanges && schemaStore.headerButtons.history)
const titles = ref<Record<string, string>>({})
const schema: ChangeSchema = {
  entityType: (name) => schemaStore.getEntityType(name),
  relationType: (name) => schemaStore.getRelationType(name),
}
const changes = ref<ChangeRecord[]>([])
const nextBefore = ref<number | undefined>()
const loaded = ref(false)
const loadingMore = ref(false)
const error = ref('')
let controller: AbortController | null = null

/** The bare id on screen, for naming the other end of a relation. */
const selfId = computed(() => props.entityId.split('@')[0])

/** Each change as the reader sees it, worked out once per render. */
const rows = computed(() =>
  changes.value.map((c) => ({
    seq: c.seq,
    time: c.time,
    actor: actorText(c, titles.value),
    op: opText(c.op),
    subject: subjectText(c, selfId.value, titles.value, schema),
    fields: summaryText(c, schema),
    trigger: triggerText(c),
  }))
)

function reset() {
  changes.value = []
  titles.value = {}
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
    titles.value = before ? { ...titles.value, ...page.titles } : (page.titles ?? {})
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
    <p v-else-if="changes.length === 0 && !nextBefore" class="panel-note">
      {{ t('audit.changes.empty') }}
    </p>
    <ol v-if="changes.length > 0" class="change-list">
      <li v-for="r in rows" :key="r.seq" class="change-item" data-testid="change-row">
        <div class="change-line">
          <time :datetime="r.time" class="change-time">{{ formatInstant(r.time) }}</time>
          <span class="change-user">{{ r.actor }}</span>
          <span class="change-op">{{ r.op }}</span>
          <span class="change-subject">{{ r.subject }}</span>
        </div>
        <p v-if="r.fields" class="change-summary">{{ r.fields }}</p>
        <p v-if="r.trigger" class="change-trigger">{{ r.trigger }}</p>
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
