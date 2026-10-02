<script setup lang="ts">
/**
 * One glyph from the icon set.
 *
 * The body is vendored markup rather than a component per icon, because a
 * Lucide glyph is not always a single `<path>`: a search icon is a path and a
 * circle, a calendar is a rect and ten segments. It is generated, static and
 * never interpolated from user input, which is what makes `v-html` safe here —
 * `name` is a key into a closed map, and an unknown one renders nothing.
 */
import { computed } from 'vue'
import { icons, type IconName } from './icons'

const props = withDefaults(
  defineProps<{
    name: IconName
    size?: number
    /**
     * A decorative icon beside a label is hidden from screen readers, which is
     * the common case. Give a label where the icon is the only thing carrying
     * the meaning, and it becomes an `img` with that name.
     */
    label?: string
  }>(),
  { size: 16 },
)

const body = computed(() => icons[props.name] ?? '')
</script>

<template>
  <svg
    class="rl-icon"
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
    :aria-hidden="label ? undefined : true"
    :role="label ? 'img' : undefined"
    :aria-label="label"
    v-html="body"
  />
</template>

<style scoped>
.rl-icon {
  display: block;
  flex: none;
}
</style>
