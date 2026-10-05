<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { navigationPending } from '@/router'
import { useSchemaStore, useUIStore } from '@/stores'
import { getErrorMessage } from '@/api'
import {
  useKeyboardShortcuts,
  useShellCommands,
  shortcutsModalOpen,
  paletteOpen,
  useEvents,
  useVisualViewportOffset,
} from '@/composables'
import { useConfirmHost } from '@/composables/useConfirm'
import { useDetailPanelOutlet } from '@/composables/useDetailPanel'
import { usePageHeaderOutlet } from '@/composables/usePageHeader'
import { useBackTarget } from '@/composables/useBackTarget'
import { unknownWorldQuery } from '@/composables/useWorld'
import ActivityBar from '@/components/common/ActivityBar.vue'
import RlAppShell from 'rela-components/components/layout/RlAppShell.vue'
import RlPageHeader from 'rela-components/components/layout/RlPageHeader.vue'
import RlViewTabs from 'rela-components/components/layout/RlViewTabs.vue'
import SpaceCreateMenu from '@/components/common/SpaceCreateMenu.vue'
import { useSpaceStore } from '@/stores/space'
import { SIDEBAR_DEFAULT_WIDTH } from '@/stores/ui'
import Sidebar from '@/components/common/Sidebar.vue'
import SidebarFlyout from '@/components/flyout/SidebarFlyout.vue'
import RlToastHost from 'rela-components/components/feedback/RlToastHost.vue'
import { useToasts } from 'rela-components/components/feedback/useToasts'
import ScriptErrorDialog from '@/components/common/ScriptErrorDialog.vue'
import KeyboardShortcutsModal from '@/components/ui/KeyboardShortcutsModal.vue'
import CommandPaletteModal from '@/components/ui/CommandPaletteModal.vue'
import ConfirmModal from '@/components/ui/ConfirmModal.vue'
import RlButton from 'rela-components/components/common/RlButton.vue'
import RlStatusRegion from 'rela-components/components/feedback/RlStatusRegion.vue'
import RlIconButton from 'rela-components/components/common/RlIconButton.vue'

// Hamburger only shows on "top-level" screens — those routed directly from
// the sidebar. Detail/edit/form/document/view screens render their own Back
// button (or, for forms, a Cancel button) so the hamburger would be
// redundant and overlap.
//
// Gate 1 — back target: any view that has ?return_to= or ?from= is by
// definition not top-level, so hide the hamburger and let the view's Back
// button serve as the primary nav affordance.
//
// Gate 2 — route name: forms always have a Cancel that navigates back, so
// even a "top-level" form (linked from the sidebar with no back-target
// query) should hide the hamburger.
const route = useRoute()
const backTarget = useBackTarget()
const NON_TOP_LEVEL_ROUTES = new Set(['form-create', 'form-edit'])
/*
 * The page header band. Hoisted out of the views so it spans the content and
 * the panel both; the routed view still decides what goes in it. A screen
 * that sets none renders no band at all, which is how every screen not yet
 * migrated keeps its own in-view header.
 */
const pageHeader = usePageHeaderOutlet()
const pageTabs = computed(() => {
  const frame = pageHeader.frame.value
  return frame && frame.tabs.length > 1 ? frame : null
})
// Gate 3 — hoisted header: RlPageHeader renders its own nav toggle, so a
// screen that fills the header slot already has one and this button would
// be a second hamburger sitting above it.
const showHamburger = computed(
  () =>
    backTarget.value === null &&
    !NON_TOP_LEVEL_ROUTES.has(route.name as string) &&
    !pageHeader.present.value,
)

const schemaStore = useSchemaStore()
const uiStore = useUIStore()
// The one toast host. Every toast, whether raised through uiStore or straight
// on the library queue, lands in this queue.
const { toasts, dismiss: dismissToast } = useToasts()
const spaceStore = useSpaceStore()

/*
 * The shell's drawer, bridged onto the store so every existing caller —
 * Sidebar's own close, the keyboard shortcuts — still drives one piece of
 * state. RlAppShell owns the scrim, the Escape handler and the scroll lock
 * that rela used to hand-roll.
 */
const navOpen = computed({
  get: () => uiStore.sidebarMobileOpen,
  set: (open) => (open ? uiStore.openMobileSidebar() : uiStore.closeMobileSidebar()),
})
/*
 * The detail panel beside a list. The routed view decides what goes in it
 * (see useDetailPanel); the shell only renders what it is given, so a screen
 * with no panel costs nothing here.
 */
const detailPanel = useDetailPanelOutlet()

/*
 * The dragged sidebar width. Unset on the collapsed rail, which has a fixed
 * width and no handle. The frame carries the same token so the flyout layer
 * starts where the dragged sidebar ends.
 */
const sidebarWidth = computed(() => (uiStore.sidebarCollapsed ? undefined : uiStore.sidebarWidth))
const frameStyle = computed(() =>
  sidebarWidth.value === undefined ? undefined : { '--rl-sidebar-width': `${sidebarWidth.value}px` },
)

const loading = ref(true)
const error = ref<string | null>(null)

// Initialize global keyboard shortcuts
useKeyboardShortcuts()

// Commands from a native shell's menu bar (the desktop app).
useShellCommands()

// Initialize SSE connection for real-time updates
useEvents()

// Single global confirm modal — driven by useConfirm() from anywhere.
const { state: confirmState, onConfirmEvent, onCancelEvent } = useConfirmHost()

// useConfirmHost re-throws errors from onConfirm callbacks so the modal stays
// open with busy cleared (caller has already surfaced the error via toast).
// Don't let those become unhandled-rejection warnings at the modal boundary.
function handleConfirm() {
  onConfirmEvent().catch(() => {})
}

// Mirror visualViewport.offsetTop onto --vv-offset-top so sticky topbars
// follow the iOS keyboard. See useVisualViewportOffset for the rationale.
useVisualViewportOffset()

onMounted(async () => {
  try {
    await schemaStore.load()
  } catch (err) {
    error.value = getErrorMessage(err, 'Failed to load application')
    uiStore.error(error.value)
  } finally {
    loading.value = false
  }
})

// A `?world=` naming no served world is a 400 on every API route, so drop
// it and land in the default world. See unknownWorldQuery.
const router = useRouter()
watch(
  [() => schemaStore.loaded, () => route.query.world],
  ([loaded]) => {
    if (!loaded) return
    const next = unknownWorldQuery(route.query, schemaStore.worlds)
    if (next) router.replace({ query: next })
  },
  { immediate: true },
)

// Apply palette CSS variables when schema loads, theme toggles, or
// the saved palette changes (e.g. after the user clicks Save Palette
// in Settings — schemaStore.reload() rewrites paletteLight/paletteDark/
// darkDisabled, this watch picks it up and re-applies inline styles
// to <html> so the change is visible immediately on the current
// screen).
//
// When the project palette has dark disabled (Regular mode), we
// always render the light palette regardless of the user's global
// dark toggle, AND we strip the `dark` class from <html> so any
// dark-mode CSS rules don't apply. The toggle button is hidden in
// the status bar in this case.
watch(
  [
    () => schemaStore.loaded,
    () => uiStore.darkMode,
    () => schemaStore.paletteLight,
    () => schemaStore.paletteDark,
    () => schemaStore.darkDisabled,
  ],
  () => {
    if (!schemaStore.loaded) return

    // Regular-mode project: force-render as light, no html.dark class.
    if (schemaStore.darkDisabled) {
      document.documentElement.classList.remove('dark')
      if (Object.keys(schemaStore.paletteLight).length > 0) {
        uiStore.applyPalette(schemaStore.paletteLight)
      }
      return
    }

    // Light+Dark project: respect the user's global toggle.
    const palette = uiStore.darkMode ? schemaStore.paletteDark : schemaStore.paletteLight
    if (Object.keys(palette).length > 0) {
      uiStore.applyPalette(palette)
    }
  },
  { immediate: true }
)
</script>

<template>
  <!--
    Outside the loading/error/app branches on purpose: a navigation can be
    in flight in any of them, and re-mounting the bar per branch would
    restart its fade. Fixed-position, so it costs no layout in any state.
  -->
  <ActivityBar :active="navigationPending.isNavigating.value" />

  <!--
    The boot screens stand in for the whole app, so they get their own
    full-viewport wrapper: RlStatusRegion grows to fill its parent, and at boot
    there is no parent pane for it to fill.
  -->
  <div v-if="loading" class="boot-screen">
    <RlStatusRegion>Loading...</RlStatusRegion>
  </div>

  <div v-else-if="error" class="boot-screen">
    <RlStatusRegion tone="error">
      {{ error }}
      <template #actions>
        <RlButton variant="primary" @click="schemaStore.reload()">Retry</RlButton>
      </template>
    </RlStatusRegion>
  </div>

  <!--
    The frame, around the shell rather than inside it, so the flyout layer
    measures against the window: a layer inside the content would start
    below whatever header the page puts above it, and the flyout would open
    at a different height on each screen.
  -->
  <div
    v-else
    class="app-frame"
    :class="{ 'app-frame--sidebar-collapsed': uiStore.sidebarCollapsed }"
    :style="frameStyle"
  >
    <RlAppShell
      v-model:nav-open="navOpen"
      class="app-layout"
      :sidebar-width="sidebarWidth"
      :sidebar-default-width="SIDEBAR_DEFAULT_WIDTH"
      :panel-open="detailPanel.open.value"
      :panel-mode="detailPanel.mode.value"
      @update:sidebar-width="uiStore.setSidebarWidth"
    >
      <template #sidebar><Sidebar /></template>

      <!--
        The view supplies the title and the slot content; the shell decides how
        the band looks and where it sits. Star and overflow menu are off because
        rela has neither.
      -->
      <template v-if="pageHeader.present.value" #header>
        <RlPageHeader
          :title="pageHeader.title.value"
          :show-star="false"
          :show-menu="false"
          @open-nav="navOpen = true"
        >
          <template v-if="pageHeader.menu.value" #menu>
            <component :is="pageHeader.menu.value" />
          </template>
          <template v-if="pageHeader.badge.value" #status>
            <component :is="pageHeader.badge.value" />
          </template>
          <!-- A page's tab bar (TKT-ITQ0HL). One tab left needs no bar. -->
          <template v-if="pageTabs" #tabs>
            <RlViewTabs
              :tabs="pageTabs.tabs"
              :model-value="pageTabs.active"
              :show-add="false"
              @update:model-value="pageTabs.select"
            />
          </template>
          <template v-if="pageHeader.content.value?.actions || spaceStore.create.length" #actions>
            <component :is="pageHeader.content.value.actions" v-if="pageHeader.content.value?.actions" />
            <SpaceCreateMenu />
          </template>
          <template v-if="pageHeader.content.value?.tools" #tools>
            <component :is="pageHeader.content.value.tools" />
          </template>
        </RlPageHeader>
      </template>

      <template v-if="detailPanel.content.value" #panel>
        <component
          :is="detailPanel.content.value.component"
          v-bind="detailPanel.content.value.props"
        />
      </template>
      <!--
        The hamburger sits inside the shell's main pane rather than fixed to the
        viewport: the shell owns the drawer, so the trigger belongs to the pane
        the drawer covers.
      -->
      <RlIconButton
        v-if="showHamburger"
        class="mobile-menu-btn"
        icon="menu"
        label="Toggle navigation"
        :aria-expanded="navOpen"
        aria-controls="main-sidebar"
        @click="navOpen = !navOpen"
      />
      <RouterView />
      <RlToastHost :toasts="toasts" @dismiss="dismissToast" />
      <ScriptErrorDialog />
      <KeyboardShortcutsModal
        :open="shortcutsModalOpen"
        @close="shortcutsModalOpen = false"
      />
    </RlAppShell>

    <div class="app-frame__flyout">
      <SidebarFlyout />
    </div>
  </div>

  <!-- Mounted unconditionally so Cmd+K works during schema loading and on
       the error screen, mirroring the ConfirmModal hoist below. -->
  <CommandPaletteModal :open="paletteOpen" @close="paletteOpen = false" />

  <!-- Mounted unconditionally (outside the loading/error/loaded branches) so
       any caller of useConfirm() resolves to a rendered modal even during
       schema loading or on the error screen. Without this hoist, callers
       would deadlock on a forever-pending promise. -->
  <ConfirmModal
    :open="confirmState.open"
    :title="confirmState.title"
    :message="confirmState.message"
    :confirm-label="confirmState.confirmLabel"
    :cancel-label="confirmState.cancelLabel"
    :busy="confirmState.busy"
    :danger="confirmState.danger"
    @confirm="handleConfirm"
    @cancel="onCancelEvent"
  />
</template>

<style>
/* The palette is rela-components', in rl/styles/tokens.css + dark.css. rela
   no longer keeps one of its own: two palettes drifted, and the shell painted
   light on a dark page. See src/styles/tokens.css. */

* {
  box-sizing: border-box;
  margin: 0;
  padding: 0;
}

/* Matches the library's own body rule (base.css), which rela does not import
   because that file also carries a reset. Reading the same tokens is what
   keeps the page behind a component the same colour as the component. */
body {
  font-family: var(--rl-font-family);
  font-size: var(--rl-font-size-md);
  background: var(--rl-color-bg);
  line-height: var(--rl-line-height-normal);
  transition: background-color 0.2s ease, color 0.2s ease;
}

/*
 * RlAppShell draws the frame: sidebar, drawer, scrim and the panes. It is
 * `100dvh` with each pane scrolling internally, so the margin-left that used
 * to reserve space beside a fixed sidebar is gone — the sidebar is a flex
 * child now and takes its own width, collapsed or not.
 *
 * What remains here is rela's page padding and the iOS safe-area insets,
 * which the library has no notion of. No `:deep` needed: this block is
 * unscoped, so a plain class reaches the shell's element.
 */
.app-layout .rl-app-shell__main {
  /* --page-padding-x exposes the horizontal padding to PageLayout so
     its sticky topbar / actionbar can bleed full-width via negative
     margin without each view re-asserting the value. Stays in sync
     across breakpoints below.

     Taken from the library's page gutter rather than a number of our own:
     the header band is painted by the shell and pads itself with the same
     token, so any other value here leaves the content misaligned with the
     title above it. The token already carries its own breakpoints and the
     iOS safe-area inset. */
  --page-padding-x: var(--rl-page-gutter-left);
  padding: var(--rl-space-5) var(--rl-page-gutter-right) var(--rl-space-5) var(--rl-page-gutter-left);
  padding-bottom: 48px; /* Account for status bar */
  /* The pane is the scroll container, so PageLayout's sticky bars stick
     against it rather than against the document. */
  overflow-y: auto;
}

.app-frame {
  position: relative;
}

/*
 * Starts where the sidebar ends, so a flyout slides out of the nav that
 * opened it and covers the content from its top edge. Pointer-transparent:
 * the stack inside decides what is clickable, this layer only says where the
 * panels may reach.
 */
.app-frame__flyout {
  position: absolute;
  inset: 0 0 0 var(--rl-sidebar-width);
  pointer-events: none;
}

/* The collapsed rail; the same width Sidebar.vue gives it. */
.app-frame--sidebar-collapsed .app-frame__flyout {
  left: 60px;
}

/* The sidebar is an off-canvas drawer here, so there is no rail to clear. */
@media (max-width: 767px) {
  .app-frame__flyout,
  .app-frame--sidebar-collapsed .app-frame__flyout {
    left: 0;
  }
}

.boot-screen {
  display: flex;
  min-height: 100vh;
}

/* Placement only: RlIconButton draws the control. The shell owns the drawer,
   so its trigger belongs to the pane the drawer covers rather than to the
   viewport. This is the fallback for views that do not yet fill the header
   slot — RlPageHeader renders its own toggle for those that do.

   Three classes so the hiding outranks RlIconButton's scoped
   `.rl-icon-button[data-v]` `display: flex` on specificity alone. A bare
   class lost to it, which left an invisible full-width button over the top
   of every desktop page; two classes would tie and depend on bundle order. */
.app-layout .mobile-menu-btn.rl-icon-button {
  /* The shell sets this token whenever its sidebar is off-canvas: below
     768px, and up to 1079px while a detail panel is open. Following it
     keeps this button in step with the shell's own breakpoints. */
  display: var(--rl-nav-toggle-display, none);
  align-items: center;
  justify-content: center;
  /* Sticky, not fixed: the button scrolls with its pane's padding box and
     stays put as the content moves under it. Fixed would pin it to the
     viewport, where it would sit over the drawer the shell slides in. */
  position: sticky;
  float: left;
  /* No safe-area term: the shell pads the frame, so this button's containing
     block already starts below the notch. */
  top: 8px;
  margin-left: calc(0px - var(--page-padding-x, 16px) + 8px);
  z-index: 101;
}

@media (max-width: 768px) {
  /* The iOS safe-area insets are RlAppShell's, not ours. With no header slot
     it pads the FRAME, which sits outside the scroll container — so the inset
     cannot scroll away with the content. Adding `env()` again here would
     stack a second inset on a notched device, and no test would catch it: the
     mobile assertions are lower bounds and Pixel 7 reports no inset at all. */
  .app-layout .rl-app-shell__main {
    /* Horizontal padding is not restated: --rl-page-gutter drops to 16px
       below 768px on its own, and restating it here would let the content
       drift from the header band the next time the library retunes it.
       Space for the hamburger button (only present on top-level screens).
       On detail/edit screens the hamburger is hidden, but we still need
       breathing room, so keep the same top padding regardless. */
    padding-top: 60px;
    /* Extra bottom padding so the last card's rounded corners aren't flush
       against the screen edge. */
    padding-bottom: 24px;
  }

  /* Hide keyboard shortcut hints on mobile: a touch device has no keys to
     press. !important because RlKbd sets display: inline-flex in its own
     scoped style, which this has to beat. */
  .rl-kbd {
    display: none !important;
  }

  /* EasyMDE toolbar responsive */
  .EasyMDEContainer .editor-toolbar {
    overflow-x: auto;
    flex-wrap: nowrap;
  }
}

@media (max-width: 480px) {
  .app-layout .rl-app-shell__main {
    /* Same as above: the gutter token owns the horizontal padding. */
    padding-top: 56px;
    padding-bottom: 24px;
  }
}



/* `.page-header` and `.header-actions` are gone — RlPageHeader owns the page
   header now and the app shell renders it. */
</style>
