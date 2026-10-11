<script setup lang="ts">
import { useId } from 'vue'
import RlText from 'rela-components/components/common/RlText.vue'

/**
 * The frame around every preview in the Configure space: a sunken panel
 * headed "Preview", so what the draft will look like never reads as more
 * controls to edit. Copied from the mockups' fixture; the library has no
 * component for it yet.
 */
withDefaults(
  defineProps<{
    /** What is previewed, after the word "Preview". */
    subject: string
    /** Content runs edge to edge, for a preview that draws its own panel. */
    flush?: boolean
    /** Stays in view while the editor beside it scrolls. */
    sticky?: boolean
  }>(),
  { flush: false, sticky: false }
)

const labelId = useId()
</script>

<template>
  <section
    class="preview"
    :class="{ 'preview--flush': flush, 'preview--sticky': sticky }"
    :aria-labelledby="labelId"
    data-testid="config-preview"
  >
    <div class="preview__label">
      <RlText :id="labelId" size="xs" tone="subtle" weight="medium" class="preview__word"
        >Preview</RlText
      >
      <RlText size="xs" tone="subtle">{{ subject }}</RlText>
    </div>
    <div class="preview__body"><slot /></div>
  </section>
</template>

<style scoped>
.preview {
  display: flex;
  flex-direction: column;
  min-width: 0;
  border: 1px solid var(--rl-color-border);
  border-radius: var(--rl-radius-lg);
  background: var(--rl-color-bg-sunken);
  overflow: hidden;
}

.preview--sticky {
  position: sticky;
  top: var(--rl-space-4);
}

.preview__label {
  display: flex;
  align-items: baseline;
  gap: var(--rl-space-2);
  padding: var(--rl-space-3) var(--rl-space-5);
  border-bottom: 1px solid var(--rl-color-border);
}

.preview__word {
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.preview__body {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-3);
  flex: 1;
  min-height: 0;
  padding: var(--rl-space-5);
}

.preview--flush .preview__body {
  padding: 0;
}
</style>
