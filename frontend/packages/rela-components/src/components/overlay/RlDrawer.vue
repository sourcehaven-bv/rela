<script setup lang="ts">
/**
 * Panel that slides in from an edge, for a form or detail view that should
 * not take the user away from the list behind it.
 *
 * Modal by default, because a drawer usually holds a task to finish. Set
 * `modal` to false for an inspector that the user reads while still working
 * in the page behind it; then there is no scrim and no focus trap.
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

const props = withDefaults(
  defineProps<{
    open: boolean
    title: string
    side?: 'left' | 'right'
    size?: 'sm' | 'md' | 'lg'
    modal?: boolean
    titleHidden?: boolean
  }>(),
  { side: 'right', size: 'md', modal: true, titleHidden: false },
)

const emit = defineEmits<{ close: [] }>()

const panel = ref<HTMLElement | null>(null)
const { push, pop, isTop } = useOverlayStack({ scrollLock: props.modal })
const focus = useFocusRestore()

function onKeydown(event: KeyboardEvent) {
  if (!isTop()) return
  if (event.key === 'Escape') {
    event.stopPropagation()
    emit('close')
  } else if (event.key === 'Tab' && props.modal) {
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
    <div v-if="open" class="rl-drawer__layer" :class="{ 'rl-drawer__layer--modal': modal }">
      <div v-if="modal" class="rl-drawer__scrim" @click="emit('close')" />

      <aside
        ref="panel"
        class="rl-drawer"
        :class="[`rl-drawer--${side}`, `rl-drawer--${size}`]"
        :role="modal ? 'dialog' : 'complementary'"
        :aria-modal="modal || undefined"
        :aria-label="title"
        tabindex="-1"
      >
        <header class="rl-drawer__header">
          <RlHeading v-if="!titleHidden" :level="2" size="md">{{ title }}</RlHeading>
          <RlIconButton
            icon="x"
            label="Close panel"
            class="rl-drawer__close"
            @click="emit('close')"
          />
        </header>

        <div class="rl-drawer__body"><slot /></div>

        <footer v-if="$slots.actions" class="rl-drawer__footer">
          <slot name="actions" />
        </footer>
      </aside>
    </div>
  </Teleport>
</template>

<style scoped>
.rl-drawer__layer {
  position: fixed;
  inset: 0;
  z-index: var(--rl-z-drawer);
  /*
   * A non-modal drawer must not swallow clicks meant for the page behind it,
   * so the layer is transparent to the pointer and the panel takes it back.
   */
  pointer-events: none;
}
.rl-drawer__layer--modal { pointer-events: auto; }

.rl-drawer__scrim {
  position: absolute;
  inset: 0;
  background: var(--rl-color-scrim);
}

.rl-drawer {
  position: absolute;
  top: 0;
  bottom: 0;
  display: flex;
  flex-direction: column;
  width: 100%;
  background: var(--rl-color-bg-raised);
  box-shadow: var(--rl-shadow-lg);
  pointer-events: auto;
}

/*
 * The edge faces the page, so it is on the inner side only. The token is
 * transparent in the light theme, where the scrim and the shadow already
 * separate the drawer, and coloured in the dark one, where a black shadow
 * over a black page cannot.
 */
.rl-drawer--left {
  left: 0;
  border-right: 1px solid var(--rl-color-border-scrimmed);
}
.rl-drawer--right {
  right: 0;
  border-left: 1px solid var(--rl-color-border-scrimmed);
}

.rl-drawer--sm { max-width: 320px; }
.rl-drawer--md { max-width: 480px; }
.rl-drawer--lg { max-width: var(--rl-detail-panel-width); }

/*
 * The drawer spans the full height and reaches the side edge, so each of its
 * three bands carries the inset on the edges it actually touches: the header
 * the status bar, the footer the home indicator, and all three the notch on
 * whichever side the drawer is anchored to.
 */
.rl-drawer__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--rl-space-4);
  padding: calc(var(--rl-space-4) + var(--rl-safe-inset-top))
           var(--rl-space-5) var(--rl-space-4);
  border-bottom: 1px solid var(--rl-color-border);
}

.rl-drawer__close { margin-left: auto; }

.rl-drawer__body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: var(--rl-space-5);
}

.rl-drawer__footer {
  padding: var(--rl-space-4) var(--rl-space-5)
           calc(var(--rl-space-4) + var(--rl-safe-inset-bottom));
  border-top: 1px solid var(--rl-color-border);
}

/*
 * Without a footer the body is the bottom band, so it clears the home
 * indicator instead.
 */
.rl-drawer__body:last-child {
  padding-bottom: calc(var(--rl-space-5) + var(--rl-safe-inset-bottom));
}

/*
 * All three bands share one horizontal padding, so the notch is added to it
 * on the side the drawer is anchored to. The opposite side faces the page,
 * where there is no inset to clear.
 */
.rl-drawer--left > * {
  padding-left: calc(var(--rl-space-5) + var(--rl-safe-inset-left));
}
.rl-drawer--right > * {
  padding-right: calc(var(--rl-space-5) + var(--rl-safe-inset-right));
}

@media (max-width: 767px) {
  /* A phone has no room for a side panel, so it becomes a full-screen view. */
  .rl-drawer { max-width: none; }
}
</style>
