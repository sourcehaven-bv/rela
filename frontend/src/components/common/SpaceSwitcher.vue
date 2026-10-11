<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import RlMenu from 'rela-components/components/overlay/RlMenu.vue'
import RlMenuItem from 'rela-components/components/overlay/RlMenuItem.vue'
import RlMenuSeparator from 'rela-components/components/overlay/RlMenuSeparator.vue'
import RlWorkspaceSwitcher from 'rela-components/components/layout/RlWorkspaceSwitcher.vue'
import { relaBase } from '@/api/base'
import { isIconName, type IconName } from 'rela-components/components/common/icons'
import { useSpaceStore, withSpace } from '@/stores/space'
import { useMe } from '@/queries/me'
import { configureRoute } from '@/configure/routes'
import { shouldDeferToBrowser } from '@/utils/openIntent'
import type { SidebarSpace } from '@/types'

/**
 * The sidebar header: the current space, and a menu of the others
 * (TKT-GNKR5H).
 *
 * Composed from library parts; a space is a rela concept, so there is no
 * library space switcher. With fewer than two spaces the name is a plain
 * label, and without spaces it is the app name.
 *
 * A principal who may configure the app (TKT-F5NGMG) also gets a Configure
 * row, so the menu is there even with one space. Inside the Configure space
 * (`configuring`) the header names it, and the rows lead back out.
 */
const props = defineProps<{ appName: string; configuring?: boolean }>()

const space = useSpaceStore()
const router = useRouter()
const { data: me } = useMe()

const canConfigure = computed(() => me.value?.can_configure === true)
const name = computed(() =>
  props.configuring ? 'Configure' : space.currentSpace?.label || props.appName
)
const hasMenu = computed(() => space.spaces.length > 1 || canConfigure.value || props.configuring)

function homeOf(s: SidebarSpace): string {
  return withSpace(s.home, s.id)
}

// The rows are real links, so a modified click opens a space in a new tab.
// A plain click stays in the SPA.
function open(s: SidebarSpace, event: MouseEvent) {
  if (shouldDeferToBrowser(event)) return
  event.preventDefault()
  void router.push(homeOf(s))
}

// The config names an icon; one the library does not know renders none.
function iconOf(s: SidebarSpace): IconName | undefined {
  return s.icon && isIconName(s.icon) ? s.icon : undefined
}

// RlMenuItem renders a raw <a>, which the router base does not reach.
function hrefOf(path: string): string {
  const base = relaBase()
  return base === '/' ? path : base + path.slice(1)
}

function go(path: string, event: MouseEvent) {
  if (shouldDeferToBrowser(event)) return
  event.preventDefault()
  void router.push(path)
}
</script>

<template>
  <RlMenu v-if="hasMenu" align="start">
    <template #trigger="{ toggle, attrs }">
      <RlWorkspaceSwitcher :name="name" v-bind="attrs" @click="toggle">
        <template v-if="$slots.logo" #logo><slot name="logo" /></template>
      </RlWorkspaceSwitcher>
    </template>
    <RlMenuItem
      v-for="s in space.spaces"
      :key="s.id"
      :icon="iconOf(s)"
      :href="hrefOf(homeOf(s))"
      :current="!configuring && s.id === space.current"
      @click="(e: MouseEvent) => open(s, e)"
    >{{ s.label }}</RlMenuItem>
    <RlMenuItem
      v-if="configuring && space.spaces.length === 0"
      :href="hrefOf('/')"
      @click="(e: MouseEvent) => go('/', e)"
    >{{ appName }}</RlMenuItem>
    <template v-if="canConfigure || configuring">
      <RlMenuSeparator />
      <RlMenuItem
        icon="sliders"
        :href="hrefOf(configureRoute.home())"
        :current="configuring"
        @click="(e: MouseEvent) => go(configureRoute.home(), e)"
      >Configure</RlMenuItem>
    </template>
  </RlMenu>
  <RlWorkspaceSwitcher v-else :name="name" :menu="false">
    <template v-if="$slots.logo" #logo><slot name="logo" /></template>
  </RlWorkspaceSwitcher>
</template>
