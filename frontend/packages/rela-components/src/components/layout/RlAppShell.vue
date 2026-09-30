<script setup lang="ts">
/**
 * Two- or three-pane frame: sidebar, main content, optional right panel.
 *
 * The `header` slot spans the full width above both the content and the
 * panel, so the panel reads as a sibling of the view rather than as part of
 * it. `panelMode` decides how the panel sits against the content:
 *
 * - `inline`  — beside the content, sharing the row.
 * - `overlay` — above the content, anchored right, with a shadow.
 *
 * Views that need their full width (a timeline, a calendar) ask for
 * `overlay`. Below 1080px the shell forces `overlay` regardless, because a
 * list and a 720px panel cannot share that space.
 *
 * `sidebarWidth` makes the sidebar draggable. It is opt-in rather than on by
 * default because the width then has somewhere to live: the shell reports it
 * and the app decides whether it survives a reload. Left unset, the sidebar
 * takes `--rl-sidebar-width` as before and no handle is drawn.
 */
import { computed, onBeforeUnmount, watch } from 'vue'
import RlResizer from './RlResizer.vue'

const props = withDefaults(
  defineProps<{
    navOpen?: boolean
    panelOpen?: boolean
    panelMode?: 'inline' | 'overlay'
    /**
     * Current sidebar width in pixels. Set it to make the sidebar draggable;
     * leave it unset and the sidebar keeps taking `--rl-sidebar-width` and no
     * handle is drawn.
     *
     * The shell reports changes through `update:sidebarWidth` and stores
     * nothing. Clamp the stored value at render rather than keeping one width
     * per breakpoint; see `RlResizer`.
     */
    sidebarWidth?: number
    /** Narrowest the sidebar may be dragged. */
    sidebarMinWidth?: number
    /** Widest the sidebar may be dragged. */
    sidebarMaxWidth?: number
    /** Width a double-click or Enter returns the sidebar to. */
    sidebarDefaultWidth?: number
  }>(),
  {
    navOpen: false,
    panelOpen: false,
    panelMode: 'inline',
    sidebarMinWidth: 200,
    sidebarMaxWidth: 420,
  },
)

const emit = defineEmits<{
  'update:navOpen': [value: boolean]
  closePanel: []
  'update:sidebarWidth': [value: number]
}>()

const resizable = computed(() => props.sidebarWidth !== undefined)

/*
 * The shell binds the width it was given, so one rule decides how wide the
 * sidebar is whether or not it is draggable. Clamped here as well as in the
 * resizer, because a caller is free to pass a stored width that predates a
 * change to the bounds.
 */
const sidebarStyle = computed(() =>
  resizable.value
    ? {
        '--rl-sidebar-width': `${Math.min(
          props.sidebarMaxWidth,
          Math.max(props.sidebarMinWidth, props.sidebarWidth as number),
        )}px`,
      }
    : undefined,
)

function close() {
  emit('update:navOpen', false)
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') close()
}

// Escape closes the drawer, and the page behind it must not scroll.
watch(
  () => props.navOpen,
  (open) => {
    if (typeof document === 'undefined') return
    document.body.style.overflow = open ? 'hidden' : ''
    if (open) document.addEventListener('keydown', onKeydown)
    else document.removeEventListener('keydown', onKeydown)
  },
)

onBeforeUnmount(() => {
  if (typeof document === 'undefined') return
  document.body.style.overflow = ''
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div
    class="rl-app-shell"
    :class="[
      { 'rl-app-shell--panel-open': panelOpen },
      `rl-app-shell--panel-${panelMode}`,
    ]"
  >
    <div
      v-if="navOpen"
      class="rl-app-shell__scrim"
      @click="close"
    />

    <div
      class="rl-app-shell__sidebar"
      :class="{ 'rl-app-shell__sidebar--open': navOpen }"
      :style="sidebarStyle"
    >
      <slot name="sidebar" />
    </div>

    <!--
      Beside the sidebar, not inside it, so `RlSidebar` keeps taking its width
      from the token and stays unaware it can be dragged. Hidden below the
      drawer breakpoint, where the sidebar is a fixed overlay the width of the
      screen and there is no edge between two panes to move.
    -->
    <RlResizer
      v-if="resizable"
      class="rl-app-shell__resizer"
      label="Resize sidebar"
      :model-value="sidebarWidth as number"
      :min="sidebarMinWidth"
      :max="sidebarMaxWidth"
      :default-value="sidebarDefaultWidth"
      @update:model-value="emit('update:sidebarWidth', $event)"
    />

    <div class="rl-app-shell__frame">
      <!--
        Header spans content and panel both. Without the slot the frame is a
        plain row, so existing two-pane screens are unaffected.
      -->
      <div v-if="$slots.header" class="rl-app-shell__header">
        <slot name="header" />
      </div>

      <div class="rl-app-shell__row">
        <main class="rl-app-shell__main">
          <slot />
        </main>

        <div class="rl-app-shell__panel">
          <slot name="panel" />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.rl-app-shell {
  display: flex;
  height: 100vh;
  /* Dynamic viewport unit avoids the mobile browser chrome cropping content. */
  height: 100dvh;
  background: var(--rl-color-bg);
}

.rl-app-shell__sidebar { display: flex; }

/*
 * The handle belongs to the row, not to the drawer: below the breakpoint the
 * sidebar leaves the flow and an edge between two panes stops existing.
 */
@media (max-width: 767px) {
  .rl-app-shell__resizer { display: none; }
}

/*
 * The shell fills the viewport, which on a notched phone reaches under the
 * home indicator, so the frame stops short of it. The top edge is left to
 * the header and the sides to the page gutters, each of which sits closer to
 * the content it protects.
 */
.rl-app-shell__frame {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  padding-bottom: var(--rl-safe-inset-bottom);
}

/*
 * The header is the top band of the frame, so it clears the status bar. On a
 * desktop the inset is zero and this is the plain header it always was.
 */
.rl-app-shell__header {
  flex: none;
  padding-top: var(--rl-safe-inset-top);
}

/*
 * Without a header slot the content is the top band instead, so the inset
 * moves onto the frame. Padding the frame rather than `main` keeps it
 * outside the scroll container, so it cannot scroll away with the content.
 */
.rl-app-shell__frame:not(:has(> .rl-app-shell__header)) {
  padding-top: var(--rl-safe-inset-top);
}

.rl-app-shell__row {
  position: relative;
  display: flex;
  flex: 1;
  min-height: 0;
  min-width: 0;
}

.rl-app-shell__main {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
  overflow: hidden;
}

.rl-app-shell__panel { display: contents; }

/*
 * Overlay: the panel floats above the content, anchored right. No scrim and
 * no dim, because selecting another task from the list behind it should swap
 * the panel's contents rather than dismiss it. The shadow carries the
 * separation on its own.
 */
.rl-app-shell--panel-overlay .rl-app-shell__panel {
  display: block;
  position: absolute;
  inset: 0 0 0 auto;
  z-index: var(--rl-z-shell-panel);
  max-width: 100%;
  box-shadow: var(--rl-shadow-lg);
}

/*
 * An open overlay hides the right of the content, and the content's own
 * scroll ends at the edge of the row, so whatever is under the panel cannot
 * be scrolled out from under it. The shell publishes the width the panel took
 * as the overlay inset, and the right page gutter adds it. That reaches every
 * scroll container in the library at once, including the board's horizontal
 * track, which padding on `main` alone could never reach.
 *
 * The gutter is restated here rather than left to the root, because a custom
 * property is resolved where it is declared: the root's copy substituted the
 * inset while it was still zero, so the value that travels down to the board
 * has to be built at the same place the inset is set.
 *
 * The width is declared rather than measured, from the same token and the
 * same breakpoint the panel uses, so the two cannot drift apart by a frame.
 */
.rl-app-shell__main {
  --rl-page-gutter-right: calc(
    var(--rl-page-gutter) + var(--rl-safe-inset-right) + var(--rl-overlay-inset)
  );
}

.rl-app-shell--panel-overlay.rl-app-shell--panel-open .rl-app-shell__main {
  --rl-overlay-inset: var(--rl-detail-panel-width);
}

@media (max-width: 1079px) {
  .rl-app-shell--panel-overlay.rl-app-shell--panel-open .rl-app-shell__main {
    --rl-overlay-inset: clamp(320px, 72vw, var(--rl-detail-panel-width));
  }
}

/*
 * Below 1080px a list and a full-width panel cannot share the row, so the
 * panel always overlays and the sidebar steps aside for it.
 */
@media (min-width: 768px) and (max-width: 1079px) {
  .rl-app-shell--panel-open .rl-app-shell__panel {
    display: block;
    position: absolute;
    inset: 0 0 0 auto;
    z-index: var(--rl-z-shell-panel);
    max-width: 100%;
    box-shadow: var(--rl-shadow-lg);
  }

  /* Forced overlay here too, so the content owes the same trailing space. */
  .rl-app-shell--panel-open .rl-app-shell__main {
    --rl-overlay-inset: clamp(320px, 72vw, var(--rl-detail-panel-width));
  }

  .rl-app-shell--panel-open .rl-app-shell__sidebar {
    position: fixed;
    inset: 0 auto 0 0;
    z-index: var(--rl-z-shell-nav);
    transform: translateX(-100%);
    transition: transform var(--rl-duration-slow) var(--rl-ease);
    box-shadow: var(--rl-shadow-lg);
  }

  .rl-app-shell--panel-open .rl-app-shell__sidebar--open { transform: translateX(0); }

  /* A drawer here too, so it carries the same insets. */
  .rl-app-shell--panel-open .rl-app-shell__sidebar {
    box-sizing: border-box;
    left: var(--rl-safe-inset-left);
    padding-top: var(--rl-safe-inset-top);
    padding-bottom: var(--rl-safe-inset-bottom);
    --rl-sidebar-safe-top: 0px;
    --rl-sidebar-safe-bottom: 0px;
    --rl-sidebar-safe-left: 0px;
  }

  .rl-app-shell--panel-open .rl-app-shell__scrim {
    position: fixed;
    inset: 0;
    z-index: var(--rl-z-shell-nav-scrim);
    background: var(--rl-color-scrim);
  }

  /* Reveal the header's drawer trigger while the sidebar is off-canvas. */
  .rl-app-shell--panel-open { --rl-nav-toggle-display: flex; }
}

/* ---- Compact: drawer nav, full-screen detail ---- */
@media (max-width: 767px) {
  .rl-app-shell__sidebar {
    position: fixed;
    inset: 0 auto 0 0;
    z-index: var(--rl-z-shell-nav);
    transform: translateX(-100%);
    transition: transform var(--rl-duration-slow) var(--rl-ease);
    box-shadow: var(--rl-shadow-lg);
  }

  .rl-app-shell__sidebar--open { transform: translateX(0); }

  /*
   * As a drawer the sidebar stands against the screen edge itself, so the
   * shell insets it here rather than leaving it to whatever is in the slot:
   * a consumer's own markup gets the same protection as `RlSidebar` does.
   *
   * The child is told the work is done, so a nested `RlSidebar` adds nothing
   * on top of this and the inset is applied once.
   */
  .rl-app-shell__sidebar {
    /*
     * The side inset shifts the drawer clear of the notch rather than padding
     * it, so the drawer keeps exactly the width its contents asked for; the
     * top and bottom are padding, which the full-height drawer absorbs.
     */
    box-sizing: border-box;
    left: var(--rl-safe-inset-left);
    padding-top: var(--rl-safe-inset-top);
    padding-bottom: var(--rl-safe-inset-bottom);
    --rl-sidebar-safe-top: 0px;
    --rl-sidebar-safe-bottom: 0px;
    --rl-sidebar-safe-left: 0px;
  }


  .rl-app-shell { --rl-nav-toggle-display: flex; }

  .rl-app-shell__scrim {
    position: fixed;
    inset: 0;
    z-index: var(--rl-z-shell-nav-scrim);
    background: var(--rl-color-scrim);
  }

  /*
   * The detail pane covers the whole screen, header included: on a phone the
   * task is the screen, not a region within the view.
   */
  .rl-app-shell--panel-open .rl-app-shell__panel {
    display: block;
    position: fixed;
    inset: 0;
    z-index: var(--rl-z-shell-panel-full);
    max-width: none;
    background: var(--rl-color-bg);
    box-shadow: none;
    /*
     * The panel covers the screen edge to edge, so its background reaches
     * under the status bar and the home indicator while its padding keeps
     * the content itself out of both. The side insets are left to the page
     * gutters inside, which already carry them.
     */
    padding-top: var(--rl-safe-inset-top);
    padding-bottom: var(--rl-safe-inset-bottom);
  }

  .rl-app-shell--panel-open .rl-app-shell__main,
  .rl-app-shell--panel-open .rl-app-shell__header { display: none; }

  /*
   * The panel covers the screen here rather than part of it, so there is no
   * content beside it to keep reachable and no trailing space to owe. Written
   * with both classes to outrank the overlay rule above, which sets the inset
   * at the same specificity and would otherwise win on order alone.
   */
  .rl-app-shell--panel-overlay.rl-app-shell--panel-open .rl-app-shell__main,
  .rl-app-shell--panel-open.rl-app-shell--panel-open .rl-app-shell__main {
    --rl-overlay-inset: 0px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .rl-app-shell__sidebar { transition: none; }
}
</style>
