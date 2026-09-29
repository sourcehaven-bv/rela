<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useQuery } from '@pinia/colada'
import RlMenu from 'rela-components/components/overlay/RlMenu.vue'
import RlMenuItem from 'rela-components/components/overlay/RlMenuItem.vue'
import RlMenuSection from 'rela-components/components/overlay/RlMenuSection.vue'
import RlMenuSeparator from 'rela-components/components/overlay/RlMenuSeparator.vue'
import RlAvatar from 'rela-components/components/data/RlAvatar.vue'
import RlIcon from 'rela-components/components/common/RlIcon.vue'
import { getMe } from '@/api'
import { apiUrl } from '@/api/base'
import { useSpaceStore } from '@/stores/space'
import { shouldDeferToBrowser } from '@/utils/openIntent'

/**
 * The account menu in the sidebar footer (TKT-MJTD12).
 *
 * Composed from library parts after rela-components' account-menu recipe.
 * Who you are comes from rela: the name and avatar are the person entity's.
 * What you can do about it comes from the login proxy: every proxy row is a
 * plain link to a page the proxy owns, opened as a full page load, so rela
 * never calls the proxy and never holds its CSRF token.
 *
 * Hidden while `_me` has not answered, and when it fails: a menu with no
 * identity in it says nothing.
 */
const router = useRouter()
const space = useSpaceStore()

// Identity changes only with a new login, which is a page load.
const { data: me } = useQuery({
  key: ['me'],
  query: getMe,
  staleTime: Infinity,
})

const name = computed(() => me.value?.person?.title || me.value?.email || me.value?.user || '')
const org = computed(() => me.value?.org?.name || me.value?.org?.slug || '')

// The quieter lines under the name, without repeating the name itself.
const lines = computed(() =>
  [me.value?.email, org.value].filter((line): line is string => !!line && line !== name.value),
)

const profilePath = computed(() => {
  const person = me.value?.person
  if (!person) return undefined
  return space.href(`/entity/${encodeURIComponent(person.type)}/${encodeURIComponent(person.id)}`)
})

// RlMenuItem renders a raw <a>, so the history base has to be applied here.
const profileHref = computed(() => (profilePath.value ? router.resolve(profilePath.value).href : undefined))

function openProfile(event: MouseEvent) {
  if (!profilePath.value || shouldDeferToBrowser(event)) return
  event.preventDefault()
  void router.push(profilePath.value)
}

// A root-relative avatar is an /api/v1 attachment path, so it takes the
// project base; an https URL is used as given.
const avatar = computed(() => {
  const src = me.value?.person?.avatar
  if (!src) return undefined
  return src.startsWith('/') ? apiUrl(src) : src
})

const links = computed(() => me.value?.links ?? {})
const hasProxyRows = computed(() => !!(links.value.account || links.value.switch_org || links.value.admin))
</script>

<template>
  <RlMenu v-if="me" align="start" placement="top" class="account-menu">
    <template #trigger="{ toggle, attrs }">
      <button
        type="button"
        class="account-trigger"
        data-testid="account-menu"
        v-bind="attrs"
        @click="toggle"
      >
        <RlAvatar :name="name" :src="avatar" size="sm" decorative />
        <span class="account-trigger__text">
          <span class="account-trigger__name">{{ name }}</span>
          <span v-if="org" class="account-trigger__org">{{ org }}</span>
        </span>
        <RlIcon name="chevron-down" :size="14" class="account-trigger__chevron" aria-hidden="true" />
      </button>
    </template>

    <RlMenuSection :label="name" :lines="lines" />
    <template v-if="profileHref || hasProxyRows">
      <RlMenuSeparator />
      <RlMenuItem v-if="profileHref" icon="user" :href="profileHref" @click="openProfile">Profile</RlMenuItem>
      <RlMenuItem v-if="links.account" icon="settings" :href="links.account">Account</RlMenuItem>
      <RlMenuItem v-if="links.switch_org" icon="organization" :href="links.switch_org">Switch org</RlMenuItem>
      <RlMenuItem v-if="links.admin" icon="shield" :href="links.admin">Admin</RlMenuItem>
    </template>
    <template v-if="links.sign_out">
      <RlMenuSeparator />
      <RlMenuItem icon="sign-out" :href="links.sign_out">Sign out</RlMenuItem>
    </template>
  </RlMenu>
</template>

<style scoped>
.account-menu {
  display: block;
}

/* The same box as the footer rows above it. */
.account-trigger {
  display: flex;
  align-items: center;
  gap: var(--rl-space-2);
  width: 100%;
  padding: 6px var(--rl-space-2);
  border: none;
  border-radius: var(--rl-radius-md);
  background: transparent;
  font-family: inherit;
  text-align: left;
  color: var(--rl-color-text);
  cursor: pointer;
}

.account-trigger:hover {
  background: var(--rl-color-bg-hover);
}

.account-trigger__text {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
}

.account-trigger__name,
.account-trigger__org {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.account-trigger__name {
  font-size: var(--font-size-dense);
}

.account-trigger__org {
  font-size: var(--rl-font-size-xs);
  color: var(--rl-color-text-subtle);
}

.account-trigger__chevron {
  flex: none;
  color: var(--rl-color-text-muted);
}

/*
 * The collapsed rail. The text is visually hidden rather than removed, so the
 * button keeps the name as its accessible name while only the avatar shows.
 */
@container rl-sidebar (max-width: 120px) {
  .account-trigger__text {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }

  .account-trigger__chevron {
    display: none;
  }
}
</style>
