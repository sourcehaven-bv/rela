<script setup lang="ts">
/**
 * PageLayout — canonical screen chrome wrapper.
 *
 * Owns the sticky topbar, optional scope-nav strip, optional bottom
 * actionbar, and the safe-area / visual-viewport math that makes them
 * behave on iOS.
 *
 * Slot map:
 *   #topbar     (required) — title + actions row. Caller composes
 *                            <PageTitle> + arbitrary content.
 *   #actions    (optional) — right-aligned action cluster in topbar
 *                            (e.g. <HelpButton>, Refresh). Pulled out
 *                            of #topbar so the title row stays simple.
 *   #scope-nav  (optional) — back-button strip above the topbar.
 *                            Sticks together with the topbar.
 *   default     — page content.
 *   #actionbar  (optional) — bottom action bar on mobile. Inline
 *                            after content on desktop.
 *
 * Hamburger contract:
 *   App.vue renders the fixed-position hamburger button (.mobile-menu-btn)
 *   above this component. PageLayout always reserves 64px on the left of
 *   the topbar so the hamburger lands on top of empty space — never over
 *   content. On screens where App.vue chooses not to render the
 *   hamburger (forms, detail), the reserved space is harmless: the
 *   topbar's own content (back button, title) fills it.
 *
 * Page-padding contract:
 *   Negative horizontal margins use var(--page-padding-x), set on
 *   .main-content by App.vue (16px <=768px, 12px <=480px). Update this
 *   variable in App.vue if the breakpoints shift; PageLayout follows.
 */
defineProps<{
  /**
   * When true, the actionbar is `position: fixed` (pinned to the
   * viewport). Use for pages that can be much taller than the
   * viewport — sticky would scroll off with the container.
   * Default: sticky.
   */
  actionbarFixed?: boolean
}>()

defineSlots<{
  topbar(): unknown
  actions?(): unknown
  'scope-nav'?(): unknown
  default?(): unknown
  actionbar?(): unknown
}>()
</script>

<template>
  <div class="page-layout">
    <div class="page-layout__sticky">
      <div v-if="$slots['scope-nav']" class="page-layout__scope-nav">
        <slot name="scope-nav" />
      </div>
      <header class="page-layout__topbar">
        <div class="page-layout__topbar-main">
          <slot name="topbar" />
        </div>
        <div v-if="$slots.actions" class="page-layout__topbar-actions">
          <slot name="actions" />
        </div>
      </header>
    </div>

    <div class="page-layout__content">
      <slot />
    </div>

    <div
      v-if="$slots.actionbar"
      class="page-layout__actionbar"
      :class="{ 'page-layout__actionbar--fixed': actionbarFixed }"
    >
      <slot name="actionbar" />
    </div>
  </div>
</template>

<style scoped>
.page-layout {
  display: flex;
  flex-direction: column;
}

/* On desktop, topbar is in normal flow. The negative margins / sticky
   behaviour all live in the mobile breakpoint below. */
.page-layout__sticky {
  display: flex;
  flex-direction: column;
}

.page-layout__topbar {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 24px;
}

.page-layout__topbar-main {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  flex: 1;
  min-width: 0;
}

.page-layout__topbar-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.page-layout__scope-nav {
  margin-bottom: 8px;
}

/* Bottom action bar — desktop: inline after content, normal flow. */
.page-layout__actionbar {
  display: flex;
  gap: 12px;
  margin-top: 24px;
}

/* Mobile chrome: sticky topbar + safe-area + viewport-edge bleed. */
@media (max-width: 768px) {
  .page-layout__sticky {
    position: sticky;
    top: 0;
    z-index: 102;
    background: var(--bg-color);
    /* Pull the sticky stack up under .main-content's 60px padding-top
       plus the safe-area inset, so its background fills the status-bar
       area. The negative horizontal margin matches .main-content's
       horizontal padding (--page-padding-x), set by App.vue per
       breakpoint. */
    margin:
      calc(-60px - env(safe-area-inset-top, 0px))
      calc(0px - var(--page-padding-x, 16px))
      16px
      calc(0px - var(--page-padding-x, 16px));
    padding-top: calc(10px + env(safe-area-inset-top, 0px));
    padding-bottom: 10px;
    border-bottom: 1px solid var(--border-color);
    /* iOS WebKit anchors sticky to the layout viewport. When the
       keyboard opens, the visual viewport shifts up — translate the
       bar to follow it so it doesn't slide under the status bar.
       App.vue's useVisualViewportOffset keeps --vv-offset-top in sync. */
    transform: translateY(var(--vv-offset-top, 0px));
  }

  /* The topbar always reserves 64px on the left for the hamburger
     button rendered by App.vue (44px wide + 8+8 inset). On screens
     without a hamburger this space is filled by the topbar content
     (back button, title) so it isn't visually wasted. */
  .page-layout__topbar {
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin: 0;
    padding: 0 var(--page-padding-x, 16px) 0 64px;
  }

  .page-layout__topbar-main {
    align-items: center;
  }

  .page-layout__topbar-actions {
    flex-wrap: wrap;
    gap: 8px;
  }

  .page-layout__scope-nav {
    margin-bottom: 0;
    padding: 8px var(--page-padding-x, 16px);
  }

  /* Mobile bottom action bar. Sticky by default — scrolls off with
     the page only when the page is short. --fixed pins it to the
     viewport regardless of scroll. */
  .page-layout__actionbar {
    position: sticky;
    bottom: 0;
    z-index: 10;
    background: var(--bg-color);
    margin:
      0
      calc(0px - var(--page-padding-x, 16px))
      calc(0px - var(--page-padding-x, 16px))
      calc(0px - var(--page-padding-x, 16px));
    padding: 12px var(--page-padding-x, 16px);
    padding-bottom: calc(12px + env(safe-area-inset-bottom, 0px));
    border-top: 1px solid var(--border-color);
    box-shadow: 0 -2px 8px rgba(0, 0, 0, 0.08);
  }

  .page-layout__actionbar--fixed {
    position: fixed;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 50;
    margin: 0;
    /* Inside .main-content the viewport-edge bleed is automatic via
       margin; once fixed-positioned we lose that, so re-add safe-area
       insets to keep content off the home indicator and screen edges. */
    padding-left: calc(var(--page-padding-x, 16px) + env(safe-area-inset-left, 0px));
    padding-right: calc(var(--page-padding-x, 16px) + env(safe-area-inset-right, 0px));
  }
}
</style>
