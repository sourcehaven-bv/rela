<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { navigationPending } from '@/router'
import { useSchemaStore, useUIStore } from '@/stores'
import { provideMessages } from 'rela-components/composables/useMessages'
import { localeMessages } from 'rela-components/composables/localeMessages'
import { resolvedLanguage } from '@/i18n'
import { getErrorMessage } from '@/api'
import { tenantProvisioning } from '@/api/tenantProvisioning'
import { REFUSAL, accessRefusal, loginUrl, refusalDetail } from '@/api/workspaceAccess'
import { loadWorkspaceReadOnly, workspaceReadOnly } from '@/api/workspaceReadOnly'
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
import { useDocumentTitle } from '@/composables/useDocumentTitle'
import { useShellAppConfig } from '@/composables/useShellAppConfig'
import { bootMeta } from '@/i18n/bootMeta'
import { usePresentationOutlet } from '@/composables/usePresentation'
import { useBackTarget } from '@/composables/useBackTarget'
import { unknownWorldQuery } from '@/composables/useWorld'
import { isPageHidden } from '@/composables/visibleRefetch'
import AsOfBanner from '@/components/common/AsOfBanner.vue'
import ReadOnlyBanner from '@/components/common/ReadOnlyBanner.vue'
import WorkspaceAccessScreen from '@/components/common/WorkspaceAccessScreen.vue'
import ActivityBar from '@/components/common/ActivityBar.vue'
import RlAppShell from 'rela-components/components/layout/RlAppShell.vue'
import NextActionCard from '@/components/NextActionCard.vue'
import RlPageHeader from 'rela-components/components/layout/RlPageHeader.vue'
import RlViewTabs from 'rela-components/components/layout/RlViewTabs.vue'
import SpaceCreateMenu from '@/components/common/SpaceCreateMenu.vue'
import { useSpaceStore } from '@/stores/space'
import { SIDEBAR_DEFAULT_WIDTH } from '@/stores/ui'
import Sidebar from '@/components/common/Sidebar.vue'
import ConfigureSidebar from '@/components/configure/ConfigureSidebar.vue'
import { isConfigurePath } from '@/configure/routes'
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
const { t } = useI18n()
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
// A presented steps page (TKT-35BY3PE) hides the sidebar, the hamburger and
// the suggestion card, so the room sees only the step.
const presentation = usePresentationOutlet()
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
// Also feeds app.language to the language resolution.
const { appName } = useShellAppConfig(schemaStore)
useDocumentTitle(appName)

// The library's own strings follow the interface language (TKT-ZL1QAO).
provideMessages(localeMessages(() => resolvedLanguage.value.language))
const uiStore = useUIStore()
// The Configure space has its own sidebar and no space create menu.
const configuring = computed(() => isConfigurePath(route.path))
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
    error.value = getErrorMessage(err, t('shell.loadFailed'))
    uiStore.error(error.value)
  } finally {
    loading.value = false
  }
})

// Whether the workspace is read-only, once the app is up (also after the boot
// screen's Retry) and again whenever the tab comes back into view, so a state
// an operator changed meanwhile shows without a reload.
watch(
  () => schemaStore.loaded,
  (loaded) => {
    if (loaded) void loadWorkspaceReadOnly()
  },
  { immediate: true }
)
function refreshReadOnlyOnReturn() {
  if (!isPageHidden() && schemaStore.loaded) void loadWorkspaceReadOnly()
}
onMounted(() => document.addEventListener('visibilitychange', refreshReadOnlyOnReturn))
onBeforeUnmount(() => document.removeEventListener('visibilitychange', refreshReadOnlyOnReturn))

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
  <!--
    The workspace is suspended, archived or unknown, or the server cannot tell
    who the user is: nothing below can work, so this replaces the app. An
    ended session is a dialog over the app instead; see the end of the file.
  -->
  <WorkspaceAccessScreen
    v-if="accessRefusal && accessRefusal !== REFUSAL.sessionEnded"
    :refusal="accessRefusal"
    :detail="refusalDetail"
    :app-name="appName()"
    :logo="bootMeta('rela-app-logo')"
    :sign-out-url="bootMeta('rela-sign-out')"
    :account-url="bootMeta('rela-account-url')"
    :contact="bootMeta('rela-contact')"
  />

  <!-- A multi-tenant server is still creating this organization's tenant. -->
  <div v-else-if="tenantProvisioning !== 'idle'" class="boot-screen">
    <RlStatusRegion :tone="tenantProvisioning === 'failed' ? 'error' : undefined">
      {{ tenantProvisioning === 'failed' ? t('shell.provisioningFailed') : t('shell.provisioning') }}
    </RlStatusRegion>
  </div>

  <div v-else-if="loading" class="boot-screen">
    <RlStatusRegion>{{ t('shell.loading') }}</RlStatusRegion>
  </div>

  <div v-else-if="error" class="boot-screen">
    <RlStatusRegion tone="error">
      {{ error }}
      <template #actions>
        <RlButton variant="primary" @click="schemaStore.reload()">{{ t('shell.retry') }}</RlButton>
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
      :presenting="presentation.presenting.value"
      :sidebar-width="sidebarWidth"
      :sidebar-default-width="SIDEBAR_DEFAULT_WIDTH"
      :panel-open="detailPanel.open.value"
      :panel-mode="detailPanel.mode.value"
      @update:sidebar-width="uiStore.setSidebarWidth"
    >
      <template #sidebar>
        <ConfigureSidebar v-if="configuring" />
        <Sidebar v-else />
      </template>

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
          <template
            v-if="pageHeader.content.value?.actions || (spaceStore.create.length && !configuring)"
            #actions
          >
            <component :is="pageHeader.content.value.actions" v-if="pageHeader.content.value?.actions" />
            <SpaceCreateMenu v-if="!configuring" />
          </template>
          <template v-if="pageHeader.content.value?.tools" #tools>
            <component :is="pageHeader.content.value.tools" />
          </template>
        </RlPageHeader>
      </template>

      <!-- Full width below the header, over the content and the panel both:
           in the content pane a sticky table header covered it, and an open
           panel squeezed it. -->
      <template v-if="workspaceReadOnly && !presentation.presenting.value" #banner>
        <ReadOnlyBanner />
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
        the drawer covers. It sits in a bar of its own, so content scrolls
        under the bar rather than past a floating button. Not `raised`: the
        bar or the page's topbar is the surface, and a raised border and
        shadow drew a box around the button that crossed the topbar's rule.
      -->
      <div
        v-if="showHamburger && !navOpen && !presentation.presenting.value"
        class="mobile-menu-bar"
      >
        <RlIconButton
          class="mobile-menu-btn"
          icon="menu"
          :label="t('shell.toggleNav')"
          :aria-expanded="navOpen"
          aria-controls="main-sidebar"
          @click="navOpen = !navOpen"
        />
      </div>
      <!-- One suggestion slot for every page, above the routed view. The
           server applies the operator's placement for the page, so the card
           is always mounted and simply renders nothing where none is owed. -->
      <NextActionCard v-if="!presentation.presenting.value" />
      <AsOfBanner v-if="!presentation.presenting.value" />
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

  <!-- Over the app, so a half-filled form keeps its input until the user signs in again. -->
  <WorkspaceAccessScreen
    v-if="accessRefusal === REFUSAL.sessionEnded"
    refusal="session_ended"
    :login-url="loginUrl"
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
  /* The top padding as a variable, so the hamburger bar can pull itself up
     over it and stick at the pane's edge (see .mobile-menu-bar). */
  --pane-padding-top: var(--rl-space-5);
  padding: var(--pane-padding-top) var(--rl-page-gutter-right) var(--rl-space-5) var(--rl-page-gutter-left);
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
   slot; RlPageHeader renders its own toggle for those that do.

   The button sits in a full-width bar with the page's own background. A
   floating button let content scroll past under it: headings and table
   headers showed around the button and under its corner. */
.app-layout .mobile-menu-bar {
  /* The shell sets this token whenever its sidebar is off-canvas: below
     768px, and up to 1079px while a detail panel is open. Following it
     keeps the bar in step with the shell's own breakpoints. */
  display: var(--rl-nav-toggle-display, none);
  flex: none;
  /* Sticky, not fixed: the bar scrolls with its pane and stays put as the
     content moves under it. Fixed would pin it to the viewport, where it
     would sit over the drawer the shell slides in.

     A sticky box stops at the pane's padding edge, not its border, so the
     bar is pulled up over the top padding (margin) and sticks there (top).
     Without both, content would show through the padding above the bar. */
  position: sticky;
  top: calc(0px - var(--pane-padding-top, 0px));
  z-index: 101;
  /* Full width: out to the pane's edges on both sides. The button keeps
     its 8px inset from the left edge. No safe-area term: the shell pads
     the frame, so the pane already starts below the notch. */
  margin: calc(0px - var(--pane-padding-top, 0px)) calc(0px - var(--rl-page-gutter-right, 16px))
    var(--rl-space-2, 8px) calc(0px - var(--page-padding-x, 16px));
  padding: 8px;
  background: var(--rl-color-bg);
}

/* A 44px touch target. Two classes so the size outranks RlIconButton's own
   scoped rule on specificity alone.

   The button is flat, in the page's background: no border, no shadow. Where
   the bar is see-through (below) it still covers what scrolls under it. The
   hover fill applies only where a pointer can hover: a phone keeps :hover
   after a tap, which left a grey box around the button once the drawer
   closed. Three classes, so this outranks the library's scoped :hover. */
.app-layout .mobile-menu-bar .mobile-menu-btn.rl-icon-button {
  width: 44px;
  height: 44px;
  align-items: center;
  justify-content: center;
  background: var(--rl-color-bg);
}

@media (hover: hover) {
  .app-layout .mobile-menu-bar .mobile-menu-btn.rl-icon-button:hover {
    background: var(--rl-color-bg-hover);
  }
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
    --pane-padding-top: 60px;
    /* Extra bottom padding so the last card's rounded corners aren't flush
       against the screen edge. */
    padding-bottom: 24px;
  }

  /* A screen that fills the shell's header slot (a list, a board) has the
     drawer toggle in that header, and App.vue renders no hamburger bar.
     The 60px reservation would be an empty band under the header, so the
     pane keeps a small gap instead. */
  .app-layout .rl-app-shell__frame:has(> .rl-app-shell__header) .rl-app-shell__main {
    --pane-padding-top: var(--rl-space-3);
  }

  /* A sticky topbar has its own safe-area and top spacing. Keep the shell's
     60px reservation only on pages without one; otherwise the sticky bar
     starts a full header-slot below the viewport edge. */
  .app-layout .rl-app-shell__main:has(.mobile-topbar, .page-layout__sticky) {
    --pane-padding-top: 0px;
  }

  /* A sticky topbar leaves 64px on its left for the button, so there the bar
     is see-through and lies over the topbar instead of taking a row. Only
     the button takes taps. */
  .app-layout .rl-app-shell__main:has(.mobile-topbar, .page-layout__sticky) > .mobile-menu-bar {
    /* 2px rather than 8px at the top: the button then sits inside the
       topbar, level with its title, instead of hanging over the topbar's
       bottom rule. The negative margin is the bar's own height (2px + 44px +
       8px), so the topbar starts at the top of the pane. */
    padding-top: 2px;
    margin-bottom: -54px;
    background: none;
    pointer-events: none;
  }

  .app-layout .rl-app-shell__main:has(.mobile-topbar, .page-layout__sticky) > .mobile-menu-bar > * {
    pointer-events: auto;
  }

  /* Except when a next-action card or the as-of banner comes first: they
     sit above the topbar, so the bar keeps its row and they start below
     the button rather than under it. */
  .app-layout
    .rl-app-shell__main:has(.mobile-topbar, .page-layout__sticky):has(> .na, > .as-of-banner)
    > .mobile-menu-bar {
    margin-bottom: var(--rl-space-2, 8px);
    padding-top: 8px;
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
    --pane-padding-top: 56px;
    padding-bottom: 24px;
  }
}



/* `.page-header` and `.header-actions` are gone — RlPageHeader owns the page
   header now and the app shell renders it. */
</style>
