<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue'
import { RouterLink, type RouteLocationRaw } from 'vue-router'
import RlButton from 'rela-components/components/common/RlButton.vue'
import { getMentionedBy, type MentionSource } from '@/api/mentions'
import { getErrorMessage, shouldDropHeldContent } from '@/api/errors'
import { isCancelledFetch } from '@/composables/usePageData'
import { useWorld } from '@/composables/useWorld'
import { entityDetailHref } from '@/utils/entityRoute'

// The entities whose body mentions this one (TKT-Q6AKUN). The server gates,
// ranks and redacts every source, so this renders what it is sent. The panel
// stays out of the page while nothing mentions the entity.

const props = defineProps<{
  entityType: string
  /** The address on screen (`POL-1@draft`). */
  entityId: string
}>()

const { worldParam } = useWorld()

const items = ref<MentionSource[]>([])
const nextCursor = ref<string | undefined>()
const truncated = ref(false)
const loadingMore = ref(false)
const error = ref('')
let controller: AbortController | null = null

async function load(cursor?: string) {
  controller?.abort()
  const ctl = new AbortController()
  controller = ctl
  try {
    const page = await getMentionedBy(
      props.entityType,
      props.entityId,
      { cursor, world: worldParam.value },
      ctl.signal
    )
    // A response without a list reads as an empty page, not a crash.
    const got = page.items ?? []
    items.value = cursor ? [...items.value, ...got] : got
    nextCursor.value = page.next_cursor
    truncated.value = page.truncated === true
    error.value = ''
  } catch (err) {
    if (isCancelledFetch(err)) return
    // A refused refetch must not leave the previous list on screen.
    if (shouldDropHeldContent(err)) {
      items.value = []
      nextCursor.value = undefined
      truncated.value = false
      error.value = ''
      return
    }
    error.value = getErrorMessage(err)
  }
}

async function loadMore() {
  loadingMore.value = true
  try {
    await load(nextCursor.value)
  } finally {
    loadingMore.value = false
  }
}

function sourceAddress(s: MentionSource): string {
  return s.face ? `${s.id}@${s.face}` : s.id
}

function sourceTarget(s: MentionSource): RouteLocationRaw {
  const path = entityDetailHref({ type: s.type, id: sourceAddress(s) })
  return worldParam.value ? { path, query: { world: worldParam.value } } : path
}

watch(
  () => [props.entityType, props.entityId, worldParam.value],
  () => {
    items.value = []
    nextCursor.value = undefined
    truncated.value = false
    void load()
  },
  { immediate: true }
)

onBeforeUnmount(() => controller?.abort())
</script>

<template>
  <section
    v-if="items.length > 0 || truncated || error"
    class="mentioned-by-panel"
    aria-labelledby="mentioned-by-heading"
  >
    <h2 id="mentioned-by-heading" class="panel-heading">Mentioned by</h2>
    <p v-if="error" class="panel-error" role="alert">{{ error }}</p>
    <ul v-if="items.length > 0" class="mention-list">
      <li v-for="s in items" :key="sourceAddress(s)" :data-entity-id="s.id">
        <RouterLink :to="sourceTarget(s)" class="mention-link">
          <span class="entity-title">{{ s.title }}</span>
          <span class="entity-id">{{ sourceAddress(s) }}</span>
        </RouterLink>
      </li>
    </ul>
    <p v-if="truncated" class="panel-note">
      Some entities that mention this one could not be listed.
    </p>
    <div v-if="nextCursor" class="panel-more">
      <RlButton
        variant="secondary"
        size="sm"
        :loading="loadingMore"
        pending-label="Loading…"
        @click="loadMore"
      >
        Show more
      </RlButton>
    </div>
  </section>
</template>

<style scoped>
.mentioned-by-panel {
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

.mention-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.mention-link {
  display: inline-flex;
  gap: 8px;
  align-items: baseline;
  color: var(--rl-color-text);
  text-decoration: none;
}

.mention-link:hover .entity-title {
  text-decoration: underline;
}

.entity-id {
  font-size: var(--font-size-sm);
  color: var(--rl-color-text-muted);
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
