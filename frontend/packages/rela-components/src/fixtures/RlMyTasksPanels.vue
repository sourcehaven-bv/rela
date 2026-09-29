<script setup lang="ts">
/**
 * The "My tasks" slide-out, wired once so every demo screen can offer it.
 *
 * The nav item is in every nav fixture, so the panel belongs on every screen
 * that shows a sidebar rather than on one of them. Wiring it per story would
 * mean repeating the task list, the open state and the close rules in each,
 * and the next screen added would quietly be the one without it.
 *
 * Wraps the whole `RlAppShell` rather than sitting inside it, which is what
 * makes the panel start in the same place on every screen. The stack fills
 * its containing block, so a wrapper placed inside the shell would start
 * below whatever that page happens to put above it: a page header, a doc
 * toolbar, a header plus view tabs. Those differ per screen, and the panel
 * would visibly start lower on some pages than others. Wrapping the shell
 * gives one containing block the size of the window instead.
 *
 * The panels are then inset from the left by the sidebar's width, so they
 * slide out of the sidebar and cover the content beside it, header included.
 * Read from a value rather than measured, so the inset cannot settle a frame
 * after the panel has already slid.
 *
 * That width has to be passed in. This wraps the shell, so it is an ancestor
 * of the sidebar and no amount of CSS inheritance carries the width upwards:
 * reading `--rl-sidebar-width` here finds whatever `:root` says, which is the
 * default and not the dragged width. The panel then anchors at 260px while
 * the sidebar sits elsewhere, and slides out from underneath the nav.
 *
 * This is a fixture for the demo stories and pages, not part of the
 * library's surface. An app would hold `open` in its route instead.
 */
import { computed, ref } from 'vue'
import RlSlidePanelStack, { type SlidePanel } from '../components/layout/RlSlidePanelStack.vue'
import RlPanelListRow from '../components/layout/RlPanelListRow.vue'
import RlSectionHeading from '../components/layout/RlSectionHeading.vue'
import RlText from '../components/common/RlText.vue'
import { myTaskGroups } from './index'

const props = withDefaults(
  defineProps<{
    /**
     * Width of the sidebar the panels slide out of, in pixels. Pass the same
     * value given to `RlAppShell`; left unset it falls back to the token,
     * which is right only while the sidebar is not resizable.
     */
    sidebarWidth?: number
  }>(),
  {},
)

const open = defineModel<boolean>('open', { default: false })

const layerStyle = computed(() => ({
  '--rl-my-tasks-inset':
    props.sidebarWidth === undefined ? 'var(--rl-sidebar-width)' : `${props.sidebarWidth}px`,
}))

const openTaskId = ref<string | null>(null)
const done = ref<Record<string, boolean>>({})

const allRows = computed(() => myTaskGroups.flatMap((group) => group.rows))
const openTask = computed(() => allRows.value.find((row) => row.id === openTaskId.value))

const panels = computed<SlidePanel[]>(() => {
  const result: SlidePanel[] = []
  if (open.value) result.push({ id: 'my-tasks', title: 'My tasks', size: 'md' })
  if (openTask.value) result.push({ id: 'task', title: openTask.value.label, size: 'lg' })
  return result
})

/* Closing a panel closes what it opened, so the stack cannot outlive its root. */
function onClose(id: string) {
  if (id === 'my-tasks') {
    open.value = false
    openTaskId.value = null
  } else {
    openTaskId.value = null
  }
}
</script>

<template>
  <div class="rl-my-tasks-panels">
    <slot />

    <!--
      Held in its own layer rather than beside the shell, so the shell keeps
      the height it sets for itself and the panels measure against the window.
    -->
    <div class="rl-my-tasks-panels__layer" :style="layerStyle">
      <RlSlidePanelStack :panels="panels" @close="onClose">
        <template #panel="{ panel }">
          <template v-if="panel.id === 'my-tasks'">
            <div v-for="group in myTaskGroups" :key="group.title">
              <RlSectionHeading :title="group.title" :count="group.rows.length" />
              <RlPanelListRow
                v-for="row in group.rows"
                :key="row.id"
                :label="row.label"
                :meta="row.meta"
                checkable
                :checked="done[row.id] ?? false"
                :selected="openTaskId === row.id"
                @update:checked="done[row.id] = $event"
                @select="openTaskId = row.id"
              />
            </div>
          </template>

          <div v-else class="rl-my-tasks-panels__detail">
            <RlText as="p" tone="subtle">Due {{ openTask?.meta }}</RlText>
            <RlText as="p">The page behind stays usable while this is open.</RlText>
          </div>
        </template>
      </RlSlidePanelStack>
    </div>
  </div>
</template>

<style scoped>
.rl-my-tasks-panels {
  position: relative;
  height: 100vh;
  height: 100dvh;
}

/*
 * Starts where the sidebar ends, so the panels slide out of the nav that
 * opened them and cover the content beside it from its very top edge.
 * Pointer-transparent, because the stack inside decides what is clickable:
 * this layer only says where the panels may reach.
 */
.rl-my-tasks-panels__layer {
  position: absolute;
  inset: 0 0 0 var(--rl-my-tasks-inset);
  pointer-events: none;
}

/*
 * Below 768px the sidebar is an off-canvas drawer, so there is no rail to
 * start after and the panels take the screen.
 */
@media (max-width: 767px) {
  .rl-my-tasks-panels__layer { left: 0; }
}

.rl-my-tasks-panels__detail {
  padding: var(--rl-space-5);
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-3);
}
</style>
