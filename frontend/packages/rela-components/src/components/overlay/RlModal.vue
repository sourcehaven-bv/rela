<script setup lang="ts">
/**
 * Modal dialog: a scrim, a focus trap and a labelled panel.
 *
 * This is the base every blocking overlay in the library sits on, so the
 * scroll lock, the Escape handling and the focus restore are written once.
 * Registering with the shared overlay stack is what stops two open dialogs
 * both reacting to one Escape press.
 */
import { nextTick, ref, watch } from 'vue'
import {
  FOCUSABLE_SELECTOR,
  trapTab,
  useFocusRestore,
  useOverlayStack,
} from '../../composables/useOverlayStack'
import RlHeading from '../common/RlHeading.vue'
import RlIconButton from '../common/RlIconButton.vue'

/*
 * The root is a Teleport, so Vue has no single element to fall attributes
 * through to and drops them with a warning. Taking them by hand puts them on
 * the panel, which is the element a caller means when it passes a class or an
 * id. Listeners come along too, so a caller's @keydown sees any key
 * pressed inside the dialog.
 */
defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    open: boolean
    title: string
    description?: string
    size?: 'sm' | 'md' | 'lg'
    /**
     * `alertdialog` interrupts to report something or ask for a decision that
     * cannot be deferred; `dialog` is everything else.
     */
    role?: 'dialog' | 'alertdialog'
    /** Hides the title visually; it still names the dialog. */
    titleHidden?: boolean
    /**
     * Blocks the ways out that do not require a decision. For a dialog that
     * is mid-submit, where closing would leave the work half done.
     */
    persistent?: boolean
    showClose?: boolean
    /**
     * Where the panel sits on the vertical axis. A dialog that answers the
     * page belongs in the middle; one the user types into, such as a command
     * palette or a picker, belongs on the reading line near the top.
     */
    align?: 'center' | 'top'
    /**
     * Overrides the z-index of the whole overlay, for the two cases the
     * modal tier cannot express: sitting under a confirm dialog this modal
     * itself raises, and sitting over a third-party editor that picks its
     * own z-index.
     */
    layer?: number | string
    /** Goes on the panel, for a width or a scoped override one caller needs. */
    panelClass?: string
  }>(),
  {
    size: 'md',
    role: 'dialog',
    titleHidden: false,
    persistent: false,
    showClose: true,
    align: 'center',
    layer: undefined,
    panelClass: undefined,
  },
)

const emit = defineEmits<{ close: [] }>()

const panel = ref<HTMLElement | null>(null)
const { push, pop, isTop } = useOverlayStack()
const focus = useFocusRestore()

function close() {
  if (props.persistent) return
  emit('close')
}

function onKeydown(event: KeyboardEvent) {
  // Only the topmost overlay acts, so Escape closes one dialog at a time.
  if (!isTop()) return
  if (event.key === 'Escape') {
    event.stopPropagation()
    close()
  } else if (event.key === 'Tab') {
    trapTab(panel.value, event)
  }
}

watch(
  () => props.open,
  async (open) => {
    if (typeof document === 'undefined') return

    if (open) {
      focus.capture()
      push()
      document.addEventListener('keydown', onKeydown, true)
      await nextTick()
      /*
       * Focus the first control rather than the panel itself, so the user is
       * already on something actionable. The panel is the fallback for a
       * dialog that is purely informational.
       */
      const first = panel.value?.querySelector<HTMLElement>(FOCUSABLE_SELECTOR)
      ;(first ?? panel.value)?.focus()
    } else {
      document.removeEventListener('keydown', onKeydown, true)
      pop()
      focus.restore()
    }
  },
  { immediate: true },
)
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="rl-modal__scrim"
      :class="`rl-modal__scrim--${align}`"
      :style="layer === undefined ? undefined : { zIndex: layer }"
      @click.self="close"
    >
      <div
        ref="panel"
        class="rl-modal"
        :class="[`rl-modal--${size}`, panelClass]"
        v-bind="$attrs"
        :role="role"
        aria-modal="true"
        :aria-label="title"
        tabindex="-1"
      >
        <header v-if="!titleHidden || showClose" class="rl-modal__header">
          <RlHeading v-if="!titleHidden" :level="2" size="lg">{{ title }}</RlHeading>
          <RlIconButton
            v-if="showClose && !persistent"
            icon="x"
            label="Close dialog"
            class="rl-modal__close"
            @click="close"
          />
        </header>

        <div class="rl-modal__body"><slot /></div>

        <footer v-if="$slots.actions" class="rl-modal__footer">
          <slot name="actions" />
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.rl-modal__scrim {
  position: fixed;
  inset: 0;
  z-index: var(--rl-z-modal);
  display: flex;
  justify-content: center;
  /*
   * The scrim itself covers the safe area, so the tint reaches the screen
   * edge, but its padding keeps the dialog out of it.
   */
  padding: calc(var(--rl-space-4) + var(--rl-safe-inset-top))
           calc(var(--rl-space-4) + var(--rl-safe-inset-right))
           calc(var(--rl-space-4) + var(--rl-safe-inset-bottom))
           calc(var(--rl-space-4) + var(--rl-safe-inset-left));
  background: var(--rl-color-scrim);
}

.rl-modal__scrim--center { align-items: center; }

/*
 * Not flex-start: a panel pinned to the very top reads as part of the
 * chrome. This keeps it on roughly the same line as the page's own heading.
 */
.rl-modal__scrim--top {
  align-items: flex-start;
  padding-top: calc(12vh + var(--rl-safe-inset-top));
}

/* The panel starts lower, so it has that much less room to grow into. */
.rl-modal__scrim--top .rl-modal { max-height: calc(88vh - var(--rl-space-8)); }

.rl-modal {
  display: flex;
  flex-direction: column;
  width: 100%;
  /* Never taller than the viewport, so the actions stay reachable. */
  max-height: calc(100vh - var(--rl-space-8));
  padding: var(--rl-space-6);
  /*
   * Transparent in the light theme, where the scrim and the shadow already
   * separate the dialog from the page. The dark theme colours it, because a
   * black shadow over a black page cannot.
   */
  border: 1px solid var(--rl-color-border-scrimmed);
  border-radius: var(--rl-radius-lg);
  background: var(--rl-color-bg-raised);
  box-shadow: var(--rl-shadow-lg);
}

.rl-modal--sm { max-width: 400px; }
.rl-modal--md { max-width: 560px; }
.rl-modal--lg { max-width: 800px; }

.rl-modal__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--rl-space-4);
  margin-bottom: var(--rl-space-4);
}

.rl-modal__close { margin: -4px -4px 0 auto; }

/*
 * Only the body scrolls, so the title and the actions stay put. A scrolling
 * box clips what is drawn outside it, which would cut the focus ring off a
 * full-width control. The padding makes room for the ring and the negative
 * margin takes it back, so the content keeps its place.
 */
.rl-modal__body {
  --rl-modal-ring-room: calc(var(--rl-focus-ring-gap) + var(--rl-focus-ring-width));
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  margin: calc(-1 * var(--rl-modal-ring-room));
  padding: var(--rl-modal-ring-room);
}

.rl-modal__footer { margin-top: var(--rl-space-6); }

@media (max-width: 767px) {
  /* A sheet from the bottom, whichever alignment the caller asked for. */
  .rl-modal__scrim { padding: 0; }
  .rl-modal__scrim--center,
  .rl-modal__scrim--top { align-items: flex-end; padding-top: 0; }
  .rl-modal,
  .rl-modal__scrim--top .rl-modal {
    max-width: none;
    max-height: 90vh;
    /* Sheet from the bottom: the top corners round, the bottom ones do not. */
    border-radius: var(--rl-radius-lg) var(--rl-radius-lg) 0 0;
    /*
     * The sheet reaches the bottom edge, so its own padding carries the home
     * indicator rather than a gap under it: a gap would show the page through
     * the sheet's own background.
     */
    padding-bottom: calc(var(--rl-space-6) + var(--rl-safe-inset-bottom));
    padding-left: calc(var(--rl-space-6) + var(--rl-safe-inset-left));
    padding-right: calc(var(--rl-space-6) + var(--rl-safe-inset-right));
  }
}
</style>
