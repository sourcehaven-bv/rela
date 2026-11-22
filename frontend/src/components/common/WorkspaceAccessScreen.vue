<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlStatusRegion from 'rela-components/components/feedback/RlStatusRegion.vue'
import type { AccessRefusal } from '@/api/workspaceAccess'

/**
 * What the app shows when the server refuses the session or the workspace
 * (see `api/workspaceAccess.ts`).
 *
 * An ended session is a dialog over the app, not a replacement for it: the
 * page underneath stays mounted, so a half-filled form keeps its input and its
 * "leave this page?" guard. "Sign in again" is a full page navigation, to the
 * sign-in page the server named or else a reload, which the edge sends to
 * sign-in.
 *
 * Every other refusal replaces the app. A suspended or archived workspace
 * offers nothing to click, because no retry helps until someone changes its
 * state; it says who to ask. An unknown workspace and a server that cannot
 * identify anyone offer a reload: a workspace that is being created, or a
 * server whose operator fixes its settings, can work a moment later.
 */
const props = defineProps<{
  refusal: AccessRefusal
  loginUrl?: string | null
  detail?: string | null
  /** Replaces `window.location`, for tests. */
  location?: Pick<Location, 'assign' | 'reload'>
}>()

const { t } = useI18n()

const text = computed(() => {
  switch (props.refusal) {
    case 'session_ended':
      return {
        title: t('shell.access.sessionEndedTitle'),
        body: t('shell.access.sessionEndedBody'),
      }
    case 'no_identity':
      return { title: t('shell.access.noIdentityTitle'), body: props.detail ?? '' }
    case 'suspended':
      return { title: t('shell.access.suspendedTitle'), body: t('shell.access.suspendedBody') }
    case 'archived':
      return { title: t('shell.access.archivedTitle'), body: t('shell.access.archivedBody') }
    default:
      return { title: t('shell.access.unknownTitle'), body: t('shell.access.unknownBody') }
  }
})

const showContact = computed(() =>
  ['suspended', 'archived', 'unknown_workspace'].includes(props.refusal)
)
const offersReload = computed(
  () => props.refusal === 'unknown_workspace' || props.refusal === 'no_identity'
)

const loc = () => props.location ?? window.location

function signIn() {
  // A reload, not assign(href): assigning the same URL with a #fragment is a
  // jump within the page and never reaches the edge.
  if (props.loginUrl) loc().assign(props.loginUrl)
  else loc().reload()
}

function reload() {
  loc().reload()
}

const signInButton = ref<InstanceType<typeof RlButton> | null>(null)
onMounted(() => {
  if (props.refusal !== 'session_ended') return
  ;(signInButton.value?.$el as HTMLElement | undefined)?.focus()
})
</script>

<template>
  <div
    v-if="refusal === 'session_ended'"
    class="access-overlay"
    role="alertdialog"
    aria-modal="true"
    aria-labelledby="access-title"
    data-testid="workspace-access-screen"
    :data-refusal="refusal"
  >
    <div class="access-dialog">
      <h1 id="access-title" class="access-title">{{ text.title }}</h1>
      <p class="access-body">{{ text.body }}</p>
      <RlButton ref="signInButton" variant="primary" @click="signIn">
        {{ t('shell.access.signInAgain') }}
      </RlButton>
    </div>
  </div>

  <div v-else class="boot-screen" data-testid="workspace-access-screen" :data-refusal="refusal">
    <RlStatusRegion tone="error">
      <h1 class="access-title">{{ text.title }}</h1>
      <p class="access-body">{{ text.body }}</p>
      <p v-if="showContact" class="access-contact">{{ t('shell.access.contact') }}</p>
      <template v-if="offersReload" #actions>
        <RlButton variant="secondary" @click="reload">{{ t('shell.access.tryAgain') }}</RlButton>
      </template>
    </RlStatusRegion>
  </div>
</template>

<style scoped>
.boot-screen {
  display: flex;
  min-height: 100vh;
}

.access-overlay {
  position: fixed;
  inset: 0;
  z-index: calc(var(--rl-z-tooltip) + 1);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-md);
  background: color-mix(in srgb, var(--rl-color-bg) 70%, transparent);
}

.access-dialog {
  max-width: 28rem;
  padding: var(--space-lg);
  background: var(--rl-color-bg-raised);
  border: 1px solid var(--rl-color-border);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-lg);
}

.access-title {
  margin: 0 0 var(--space-xs);
  font-size: var(--font-size-lg);
  font-weight: 600;
  color: var(--rl-color-text);
}

.access-body,
.access-contact {
  margin: 0 0 var(--space-md);
}

.access-contact {
  font-size: var(--font-size-sm);
  color: var(--rl-color-text-muted);
}
</style>
