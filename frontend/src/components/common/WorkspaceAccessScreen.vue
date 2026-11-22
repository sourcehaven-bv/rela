<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlStatusRegion from 'rela-components/components/feedback/RlStatusRegion.vue'
import { REFUSAL, type AccessRefusal } from '@/api/workspaceAccess'

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
 * offers no retry, because none helps until someone changes its state; it
 * says who to ask. An unknown workspace and a server that cannot identify
 * anyone offer a reload: a workspace that is being created, or a server whose
 * operator fixes its settings, can work a moment later.
 *
 * The pages that replace the app show the app's logo or name, and offer the
 * account pages and sign out when the operator configured them, so the user
 * is not stuck on a page with no way out. The operator can also replace the
 * suspended and archived explanations and the contact text, and add a
 * contact address. The values come from the shell
 * (`bootMeta`), because the workspace refuses its config.
 */
const props = defineProps<{
  refusal: AccessRefusal
  loginUrl?: string | null
  detail?: string | null
  appName?: string | null
  /** `app.logo`: an image URL, shown instead of the name. */
  logo?: string | null
  /** `account.sign_out`. */
  signOutUrl?: string | null
  /** `account.account`: the account portal. */
  accountUrl?: string | null
  /** `account.contact`: replaces the default "who to ask" text. */
  contact?: string | null
  /** `account.contact_email`: shown after the contact text as a mail link. */
  contactEmail?: string | null
  /** `account.suspended_text`: replaces the suspended page's explanation. */
  suspendedText?: string | null
  /** `account.archived_text`: replaces the archived page's explanation. */
  archivedText?: string | null
  /** Replaces `window.location`, for tests. */
  location?: Pick<Location, 'assign' | 'reload'>
}>()

const { t } = useI18n()

const text = computed(() => {
  switch (props.refusal) {
    case REFUSAL.sessionEnded:
      return {
        title: t('shell.access.sessionEndedTitle'),
        body: t('shell.access.sessionEndedBody'),
      }
    case REFUSAL.noIdentity:
      return { title: t('shell.access.noIdentityTitle'), body: props.detail ?? '' }
    case REFUSAL.suspended:
      return {
        title: t('shell.access.suspendedTitle'),
        body: props.suspendedText || t('shell.access.suspendedBody'),
      }
    case REFUSAL.archived:
      return {
        title: t('shell.access.archivedTitle'),
        body: props.archivedText || t('shell.access.archivedBody'),
      }
    default:
      return { title: t('shell.access.unknownTitle'), body: t('shell.access.unknownBody') }
  }
})

const CONTACT_REFUSALS: readonly AccessRefusal[] = [
  REFUSAL.suspended,
  REFUSAL.archived,
  REFUSAL.unknownWorkspace,
]
const showContact = computed(() => CONTACT_REFUSALS.includes(props.refusal))
const contactText = computed(() => props.contact || t('shell.access.contact'))

// The server only writes a bare address (dataentryconfig.ValidContactEmail);
// the same shape is checked here so a hand-edited shell cannot add mailto
// headers or a second recipient.
const BARE_EMAIL = /^[^\s@?&,;<>"\\]+@[^\s@?&,;<>"\\]+\.[^\s@?&,;<>"\\]+$/
const contactEmail = computed(() =>
  props.contactEmail && BARE_EMAIL.test(props.contactEmail) ? props.contactEmail : null
)
const offersExit = computed(() => props.refusal !== REFUSAL.noIdentity)
const offersReload = computed(
  () => props.refusal === REFUSAL.unknownWorkspace || props.refusal === REFUSAL.noIdentity
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
  if (props.refusal !== REFUSAL.sessionEnded) return
  ;(signInButton.value?.$el as HTMLElement | undefined)?.focus()
})
</script>

<template>
  <div
    v-if="refusal === REFUSAL.sessionEnded"
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
    <header v-if="logo || appName" class="access-brand">
      <img v-if="logo" :src="logo" :alt="appName ?? ''" class="access-logo" />
      <span v-else class="access-app-name">{{ appName }}</span>
    </header>
    <RlStatusRegion tone="error">
      <h1 class="access-title">{{ text.title }}</h1>
      <p class="access-body">{{ text.body }}</p>
      <p v-if="showContact" class="access-contact">
        {{ contactText }}
        <a v-if="contactEmail" :href="`mailto:${contactEmail}`" class="access-contact-email">{{
          contactEmail
        }}</a>
      </p>
      <template v-if="offersReload || (offersExit && (accountUrl || signOutUrl))" #actions>
        <RlButton v-if="offersReload" variant="secondary" @click="reload">
          {{ t('shell.access.tryAgain') }}
        </RlButton>
        <RlButton v-if="offersExit && accountUrl" as="a" :href="accountUrl" variant="secondary">
          {{ t('shell.access.openAccount') }}
        </RlButton>
        <RlButton v-if="offersExit && signOutUrl" as="a" :href="signOutUrl" variant="ghost">
          {{ t('account.signOut') }}
        </RlButton>
      </template>
    </RlStatusRegion>
  </div>
</template>

<style scoped>
.boot-screen {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
}

.access-brand {
  display: flex;
  align-items: center;
  padding: var(--space-md) var(--space-lg);
}

.access-logo {
  display: block;
  max-height: 2rem;
  max-width: 12rem;
}

.access-app-name {
  font-size: var(--font-size-lg);
  font-weight: 600;
  color: var(--rl-color-text);
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
