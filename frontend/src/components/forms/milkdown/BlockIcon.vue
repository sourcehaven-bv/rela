<script setup lang="ts">
/**
 * Inline SVG glyphs for the editor toolbar and `/` menu.
 *
 * Inline paths rather than an icon font, matching the reasoning already
 * recorded on the entity-reference button: the editor ships no font at all,
 * which matters for the sandboxed app editor where every byte is inlined.
 *
 * Drawn on a 24x24 grid with `currentColor` so a single glyph works in both
 * themes and inherits the button's hover and active colours. Geometry is
 * hand-written rather than copied from an icon set, so there is no licence to
 * carry.
 */
const props = defineProps<{ name: string }>()
</script>

<template>
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    width="16"
    height="16"
    fill="none"
    stroke="currentColor"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <!-- Bold: a B built from two stacked bowls. -->
    <template v-if="props.name === 'strong'">
      <path d="M7 5h6a3.5 3.5 0 0 1 0 7H7z" />
      <path d="M7 12h7a3.5 3.5 0 0 1 0 7H7z" />
    </template>

    <!-- Italic: slanted stroke with serifs. -->
    <template v-else-if="props.name === 'emphasis'">
      <path d="M15 5h-5M14 19H9M14 5l-4 14" />
    </template>

    <!-- Strikethrough: an S-ish stroke crossed through the middle. -->
    <template v-else-if="props.name === 'strikethrough'">
      <path d="M4 12h16" />
      <path d="M16 7a4 4 0 0 0-4-2c-2.2 0-4 1.2-4 3 0 1.3.8 2.2 2 2.8" />
      <path d="M8 17a4 4 0 0 0 4 2c2.2 0 4-1.2 4-3 0-.6-.2-1.2-.6-1.6" />
    </template>

    <!-- Inline code: angle brackets. -->
    <template v-else-if="props.name === 'inlineCode'">
      <path d="m9 8-4 4 4 4M15 8l4 4-4 4" />
    </template>

    <!-- Headings: an H with the level as a numeral. -->
    <template v-else-if="props.name === 'h1' || props.name === 'h2' || props.name === 'h3'">
      <path d="M5 6v12M13 6v12M5 12h8" />
      <text
        x="16.5"
        y="18"
        font-size="10"
        font-family="inherit"
        font-weight="700"
        fill="currentColor"
        stroke="none"
      >
        {{ props.name.slice(1) }}
      </text>
    </template>

    <!-- Bullet list: dots plus rules. -->
    <template v-else-if="props.name === 'bulletList'">
      <path d="M9 6h11M9 12h11M9 18h11" />
      <circle cx="4.5" cy="6" r="1.4" fill="currentColor" stroke="none" />
      <circle cx="4.5" cy="12" r="1.4" fill="currentColor" stroke="none" />
      <circle cx="4.5" cy="18" r="1.4" fill="currentColor" stroke="none" />
    </template>

    <!-- Numbered list: numerals plus rules. -->
    <template v-else-if="props.name === 'orderedList'">
      <path d="M10 6h10M10 12h10M10 18h10" />
      <text x="2" y="8" font-size="7" font-family="inherit" fill="currentColor" stroke="none">
        1
      </text>
      <text x="2" y="14" font-size="7" font-family="inherit" fill="currentColor" stroke="none">
        2
      </text>
      <text x="2" y="20" font-size="7" font-family="inherit" fill="currentColor" stroke="none">
        3
      </text>
    </template>

    <!-- Quote: a bar with indented rules. -->
    <template v-else-if="props.name === 'blockquote'">
      <path d="M5 5v14" stroke-width="2.5" />
      <path d="M10 8h9M10 12h9M10 16h6" />
    </template>

    <!-- Code block: brackets inside a frame. -->
    <template v-else-if="props.name === 'codeBlock'">
      <rect x="3" y="4" width="18" height="16" rx="2" />
      <path d="m10 10-2 2 2 2M14 10l2 2-2 2" />
    </template>

    <!-- Table: a grid with a header row. -->
    <template v-else-if="props.name === 'table'">
      <rect x="3" y="4" width="18" height="16" rx="2" />
      <path d="M3 9h18M9 9v11M15 9v11" />
    </template>

    <!-- Table row/column operations. Each shows the grid with the affected
         band highlighted and a +/x marking what happens to it. -->
    <template v-else-if="props.name === 'addRowBefore'">
      <rect x="3" y="10" width="18" height="11" rx="1.5" />
      <path d="M3 15.5h18" />
      <path d="M12 3v5M9.5 5.5h5" />
    </template>

    <template v-else-if="props.name === 'addRowAfter'">
      <rect x="3" y="3" width="18" height="11" rx="1.5" />
      <path d="M3 8.5h18" />
      <path d="M12 16v5M9.5 18.5h5" />
    </template>

    <template v-else-if="props.name === 'addColBefore'">
      <rect x="10" y="3" width="11" height="18" rx="1.5" />
      <path d="M15.5 3v18" />
      <path d="M3 12h5M5.5 9.5v5" />
    </template>

    <template v-else-if="props.name === 'addColAfter'">
      <rect x="3" y="3" width="11" height="18" rx="1.5" />
      <path d="M8.5 3v18" />
      <path d="M16 12h5M18.5 9.5v5" />
    </template>

    <template v-else-if="props.name === 'deleteRow'">
      <rect x="3" y="4" width="18" height="16" rx="1.5" />
      <path d="M3 9.5h18M3 14.5h18" />
      <path d="M8 12h8" stroke-width="2.5" />
    </template>

    <template v-else-if="props.name === 'deleteColumn'">
      <rect x="4" y="3" width="16" height="18" rx="1.5" />
      <path d="M9.5 3v18M14.5 3v18" />
      <path d="M12 8v8" stroke-width="2.5" />
    </template>

    <template v-else-if="props.name === 'deleteTable'">
      <rect x="3" y="4" width="18" height="16" rx="1.5" />
      <path d="M3 9h18M9 9v11" />
      <path d="m14 13 5 5M19 13l-5 5" />
    </template>

    <!-- Entity reference: connected nodes, the graph-edge idea. -->
    <template v-else-if="props.name === 'entityRef'">
      <circle cx="5" cy="6" r="2" />
      <circle cx="19" cy="9" r="2" />
      <circle cx="12" cy="19" r="2" />
      <path d="m6.8 7 10.4 1.4M17.6 10.7 13.2 17.2M10.6 17.4 6 7.9" />
    </template>
  </svg>
</template>
