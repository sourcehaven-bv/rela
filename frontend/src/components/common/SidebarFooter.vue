<script setup lang="ts">
/**
 * The sidebar's footer band: app chrome, not navigation.
 *
 * This replaces the old fixed-position StatusBar, a 24px strip pinned across
 * the bottom of the viewport. The strip predates the app shell and fought it
 * on two counts: it painted over the shell's own bottom edge (sidebar footer
 * included), and its theme toggle and Settings link duplicated the ones
 * RlSidebar already renders — two System/Light/Dark controls on one screen.
 *
 * Everything the strip carried lives here instead, except the theme toggle:
 * that one was pure duplication, and Sidebar.vue already binds the library's
 * picker to the same stored choice. The order is by how often a row is wanted
 * — the git/next-action signals read top-down as status, then the chrome links.
 *
 * The two overlays stay teleported. The sidebar is a scroll container and a
 * drawer on narrow viewports, so a popover anchored inside it would clip.
 */
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useGitStore, useSchemaStore } from '@/stores'
import { shortcutsModalOpen } from '@/composables/useKeyboardShortcuts'
import { useNextAction } from '@/composables/useNextAction'
import NextActionOffers from '@/components/NextActionOffers.vue'
import RlSidebarFooterLink from 'rela-components/components/layout/RlSidebarFooterLink.vue'
import RlModal from 'rela-components/components/overlay/RlModal.vue'
import RlPopover from 'rela-components/components/overlay/RlPopover.vue'
import RlTag from 'rela-components/components/common/RlTag.vue'
import RlStatusDot from 'rela-components/components/common/RlStatusDot.vue'
import RlText from 'rela-components/components/common/RlText.vue'
import RlKbd from 'rela-components/components/data/RlKbd.vue'
import { renderMarkdown } from '@/utils/markdown'
import { stripSpace } from '@/stores/space'

const gitStore = useGitStore()
const schemaStore = useSchemaStore()
const route = useRoute()
const router = useRouter()

// Global "About" help: the deployment description (data-entry app.description,
// falling back to the metamodel's top-level description on the server). The
// button is shown only when there is a description to show (TKT-DUQBD0).
const aboutOpen = ref(false)

// The quietest prominence tier: a chip that says something exists, expanding
// on click. Present without ever being in the way — for suggestions that are
// true most of the time and urgent none of it.
const {
  suggestion: naSuggestion,
  bandLabel: naBandLabel,
  isStatusBar: naInStatusBar,
  markShown: naMarkShown,
  expanded: naExpanded,
  loadOnce: naLoadOnce,
} = useNextAction()

const appName = computed(() => schemaStore.app?.name || 'rela')
const appDescription = computed(() => schemaStore.aboutDescription?.trim() || '')
// The description is authored as markdown (in data-entry.yaml or the metamodel);
// render it so *emphasis*, lists, etc. display. renderMarkdown sanitizes.
const appDescriptionHtml = computed(() => renderMarkdown(appDescription.value))

// Initial fetch - SSE handles subsequent updates
onMounted(() => {
  gitStore.fetchStatus().catch(() => {
    // Errors are already handled by the store
  })
  // Resolve once per session. The composable is a singleton, so this and the
  // page-level card share one suggestion rather than racing for two.
  void naLoadOnce()
})

// The chip IS the render for the statusbar tier, so it reports its own
// impression — the page-level card never sees these suggestions.
watch(
  naInStatusBar,
  (shown) => {
    if (shown) void naMarkShown()
  },
  { immediate: true },
)

/*
 * RlPopover owns its own open state, so the composable's flag is pushed into
 * it. The composable has to stay authoritative: it clears the flag on a route
 * change and after an offer is acted on, neither of which the popover can see.
 */
const naPopover = ref<{ open: () => void; close: () => void } | null>(null)
watch(naExpanded, (want) => {
  if (want) naPopover.value?.open()
  else naPopover.value?.close()
})

async function handleSync() {
  try {
    const result = await gitStore.sync()
    if (result.conflict_files && result.conflict_files.length > 0) {
      router.push('/conflicts')
    }
  } catch {
    // Error is already captured in store
  }
}
</script>

<template>
  <div class="sidebar-footer status-bar">
    <!--
      Status signals. Both are conditional, so the footer collapses to the
      three chrome links on a project with no git repo and no suggestion.
    -->
    <div v-if="gitStore.isAvailable" class="git-status" :class="gitStore.statusClass">
      <button
        type="button"
        class="footer-row git-row status-item"
        :title="gitStore.syncing ? 'Syncing...' : 'Click to sync'"
        @click="handleSync"
      >
        <span class="git-dot" />
        <span class="git-branch">{{ gitStore.branch }}</span>
        <span class="git-status-text">{{ gitStore.statusText }}</span>
      </button>
      <RouterLink
        v-if="gitStore.hasConflicts"
        to="/conflicts"
        class="footer-row status-warning status-item"
        title="Resolve conflicts"
      >
        Resolve Conflicts
      </RouterLink>
    </div>

    <!--
      statusbar prominence: a chip, expanding into a popover. RlPopover owns the
      anchoring, the outside-click and Escape dismissal and the overlay stack.
      RlMenu could not serve this: the panel holds prose and several controls,
      so `menuitem` misdescribes it, and closing on the first click inside would
      dismiss the panel before an offer could act.

      `naExpanded` stays the source of truth because the composable closes the
      panel on a route change and after an action; the popover is driven from it
      rather than mirroring it.
    -->
    <RlPopover
      v-if="naInStatusBar && naSuggestion"
      ref="naPopover"
      title="Suggested next action"
      align="start"
      placement="top"
      @close="naExpanded = false"
    >
      <template #trigger="{ open, attrs }">
        <button
          type="button"
          class="footer-row na-chip rela-na-chip status-item"
          :class="{ 'na-chip--open': open }"
          :data-band="naSuggestion.band"
          :data-source="naSuggestion.source"
          :title="naSuggestion.message"
          v-bind="attrs"
          @click="naExpanded = !naExpanded"
        >
          <RlStatusDot color="blue" />
          <span class="na-chip__text">{{ naBandLabel || 'Suggestion' }}</span>
        </button>
      </template>

      <div
        class="rela-na"
        :data-band="naSuggestion.band"
        data-prominence="statusbar"
        :data-source="naSuggestion.source"
        :data-entity-id="naSuggestion.entity_id"
      >
        <!-- Same tier-1 hook as the page-level surface, so one custom.js
             definition serves both without branching on where it rendered. -->
        <rela-slot name="companion" :data-band="naSuggestion.band" data-prominence="statusbar" />
        <RlTag v-if="naBandLabel" :label="naBandLabel" class="na-band rela-na-band" />
        <RlText as="p" class="na-message rela-na-message">{{ naSuggestion.message }}</RlText>
        <NextActionOffers
          :offers="naSuggestion.actions || []"
          :entity-id="naSuggestion.entity_id"
          :pick-options="naSuggestion.pick_options"
        />
      </div>
    </RlPopover>

    <RlSidebarFooterLink
      label="Settings"
      icon="settings"
      :class="{ active: stripSpace(route.path) === '/settings' }"
      @click="router.push('/settings')"
    />
    <RlSidebarFooterLink v-if="appDescription" label="About" icon="help" @click="aboutOpen = true" />
    <button type="button" class="footer-row shortcuts-btn status-item" title="Keyboard shortcuts" @click="shortcutsModalOpen = true">
      <RlKbd keys="?" />
      <span class="shortcuts-text">Shortcuts</span>
    </button>

    <!-- About: the deployment description. RlModal owns the scrim, the focus
         trap, Escape and the overlay stack. -->
    <RlModal :open="aboutOpen" :title="appName" @close="aboutOpen = false">
      <!-- eslint-disable-next-line vue/no-v-html -->
      <div class="about-body md-body" v-html="appDescriptionHtml" />
    </RlModal>
  </div>
</template>

<style scoped>
.sidebar-footer {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

/*
 * The rows that are not RlSidebarFooterLink still have to look like it, so
 * they share its box. Kept here rather than reaching into the library's class
 * from outside: these are rela's rows, and the library owns its own.
 */
.footer-row {
  display: flex;
  align-items: center;
  gap: var(--rl-space-3);
  width: 100%;
  padding: 6px var(--rl-space-2);
  border: none;
  border-radius: var(--rl-radius-md);
  background: transparent;
  font-family: inherit;
  font-size: var(--rl-font-size-md);
  color: var(--rl-color-text-muted);
  text-align: left;
  text-decoration: none;
  cursor: pointer;
}

.footer-row:hover {
  background: var(--rl-color-bg-hover);
}

.footer-row:focus-visible {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: -2px;
}

/* The branch name is the identity; the status text is the detail, so it
   yields first when the rail is narrow. */
.git-branch {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.git-status-text {
  margin-left: auto;
  flex-shrink: 0;
  opacity: 0.7;
}

.git-dot {
  width: 8px;
  height: 8px;
  flex-shrink: 0;
  border-radius: var(--rl-radius-pill);
  background: currentcolor;
}

.git-status.synced .git-dot {
  background: var(--rl-color-status-green);
}

.git-status.changes .git-dot {
  background: var(--rl-color-status-amber);
}

.git-status.conflict .git-dot {
  background: var(--rl-color-danger);
}

.status-warning {
  color: var(--rl-color-status-amber);
}

.na-chip--open {
  background: var(--rl-color-bg-hover);
}

/* The panel, its anchoring and its shadow are RlPopover's; the band chip is
   RlTag. Only the uppercase treatment is rela's own label convention, shared
   with .cmdk-type. */
.na-band {
  margin-bottom: var(--rl-space-2);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

/* The operator hook must not introduce a box of its own. */
.rela-na :deep(rela-slot) {
  display: contents;
}

.na-message {
  margin: 0 0 var(--rl-space-3);
}






/* Only the prose rhythm is rela's; the panel, header and close button are
   RlModal's. `md-body` carries the shared markdown typography. */
.about-body {
  line-height: 1.6;
}

.about-body :deep(p) {
  margin: 0 0 10px;
}

.about-body :deep(p:last-child) {
  margin-bottom: 0;
}
</style>
