<script setup lang="ts">
/**
 * The step bar of a page with `mode: steps` (TKT-35BY3PE): the steps as a
 * numbered list of links, Back and Next, the controls that enter and leave
 * present mode, and the current step's prompt.
 *
 * The page owns the URLs; this component only draws them. Each step is a
 * real link, so a step can be opened in a new tab and the browser's back
 * button returns to the previous step.
 */
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import RlButton from 'rela-components/components/common/RlButton.vue'
import type { SidebarPageTab } from '@/types'

const props = defineProps<{
  steps: SidebarPageTab[]
  active: string
  presenting: boolean
  /** The URL of a step, keeping present mode as it is. */
  href: (id: string) => string
}>()

const emit = defineEmits<{
  go: [id: string]
  present: [on: boolean]
}>()

const { t } = useI18n()

const index = computed(() => props.steps.findIndex((s) => s.id === props.active))
const prev = computed(() => props.steps[index.value - 1])
const next = computed(() => props.steps[index.value + 1])
const prompt = computed(() => props.steps[index.value]?.prompt)

/*
 * Full screen is the browser's, not the app's: Escape leaves it without
 * telling us, so the label follows the document's state. A refusal (no user
 * gesture, a policy, an iframe) leaves the page as it was.
 */
const fullscreen = ref(!!document.fullscreenElement)
function onFullscreenChange() {
  fullscreen.value = !!document.fullscreenElement
}
document.addEventListener('fullscreenchange', onFullscreenChange)
onBeforeUnmount(() => document.removeEventListener('fullscreenchange', onFullscreenChange))
const canFullscreen = typeof document.documentElement.requestFullscreen === 'function'

/*
 * Full screen belongs to present mode, which has the only control to leave
 * it. Leaving present mode by any route (Exit, browser back) leaves full
 * screen too, so the normal app is never stuck in it.
 */
watch(
  () => props.presenting,
  (on) => {
    if (!on && document.fullscreenElement) void document.exitFullscreen().catch(() => {})
  }
)

async function toggleFullscreen() {
  try {
    if (document.fullscreenElement) await document.exitFullscreen()
    else await document.documentElement.requestFullscreen()
  } catch {
    // Refused by the browser; nothing to undo.
  }
}
</script>

<template>
  <div class="page-steps" :class="{ 'page-steps--presenting': presenting }">
    <div class="page-steps__bar">
      <nav class="page-steps__nav" :aria-label="t('page.steps.label')">
        <ol class="page-steps__list">
          <li v-for="(step, i) in steps" :key="step.id">
            <RouterLink
              :to="href(step.id)"
              class="page-steps__step rl-focus-ring"
              :class="{
                'page-steps__step--active': step.id === active,
                'page-steps__step--done': i < index,
              }"
              :aria-current="step.id === active ? 'step' : undefined"
            >
              <span class="page-steps__number">{{ i + 1 }}</span>
              <span class="page-steps__label">{{ step.label }}</span>
            </RouterLink>
          </li>
        </ol>
      </nav>
      <div class="page-steps__actions">
        <RlButton
          variant="secondary"
          size="sm"
          icon="arrow-left"
          :disabled="!prev"
          @click="prev && emit('go', prev.id)"
        >
          {{ t('page.steps.back') }}
        </RlButton>
        <RlButton
          variant="primary"
          size="sm"
          :disabled="!next"
          @click="next && emit('go', next.id)"
        >
          {{ t('page.steps.next') }}
        </RlButton>
        <RlButton
          v-if="presenting && canFullscreen"
          variant="ghost"
          size="sm"
          icon="expand"
          @click="toggleFullscreen"
        >
          {{ fullscreen ? t('page.steps.exitFullscreen') : t('page.steps.fullscreen') }}
        </RlButton>
        <RlButton
          variant="ghost"
          size="sm"
          :icon="presenting ? 'close' : 'play'"
          @click="emit('present', !presenting)"
        >
          {{ presenting ? t('page.steps.exitPresent') : t('page.steps.present') }}
        </RlButton>
      </div>
    </div>
    <h2 v-if="prompt" class="page-steps__prompt">{{ prompt }}</h2>
  </div>
</template>

<style scoped>
.page-steps {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
  padding: var(--space-sm) var(--space-lg);
  border-bottom: 1px solid var(--rl-color-border);
}

.page-steps__bar {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  flex-wrap: wrap;
}

.page-steps__nav {
  flex: 1;
  min-width: 0;
}

.page-steps__list {
  display: flex;
  gap: var(--space-xs);
  margin: 0;
  padding: 0;
  list-style: none;
  overflow-x: auto;
}

.page-steps__step {
  display: inline-flex;
  align-items: center;
  gap: var(--space-xs);
  padding: var(--space-xs) var(--space-sm);
  border-radius: var(--radius-md);
  color: var(--rl-color-text-muted);
  text-decoration: none;
  white-space: nowrap;
}

.page-steps__step:hover {
  background: var(--rl-color-bg-hover);
}

.page-steps__number {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 1.5em;
  height: 1.5em;
  border-radius: 999px;
  border: 1px solid var(--rl-color-border);
  font-size: var(--font-size-sm);
}

.page-steps__step--done .page-steps__number {
  border-color: var(--rl-color-accent);
  color: var(--rl-color-accent);
}

.page-steps__step--active {
  color: var(--rl-color-text);
  font-weight: 600;
}

.page-steps__step--active .page-steps__number {
  background: var(--rl-color-accent);
  border-color: var(--rl-color-accent);
  color: var(--rl-color-text-inverse);
}

.page-steps__actions {
  display: flex;
  gap: var(--space-xs);
}

.page-steps__prompt {
  margin: 0;
  font-size: var(--font-size-xl);
  font-weight: 600;
  color: var(--rl-color-text);
}

.page-steps--presenting .page-steps__prompt {
  font-size: 2rem;
}
</style>
