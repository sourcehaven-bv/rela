<script setup lang="ts">
/**
 * A markdown body with suggested changes drawn inline: added text underlined
 * in green, removed text struck in red, formatting changes dotted. Story-only.
 */
import { inline, type Block } from './useTrackedChanges'

defineProps<{
  blocks: Block[]
  /** The change to highlight, such as the one whose card is hovered. */
  active?: string | null
  /** Smaller type, for the description section of a detail panel. */
  dense?: boolean
}>()

const emit = defineEmits<{
  /** A change was clicked; the event's target is the clicked mark. */
  pick: [id: string, event: MouseEvent]
  hover: [id: string]
}>()
</script>

<template>
  <article class="tc__doc" :class="{ 'tc__doc--dense': dense }">
    <template v-for="(block, b) in blocks" :key="b">
      <ul v-if="block.kind === 'ul'">
        <li
          v-for="(line, l) in block.lines"
          :key="l"
          :class="[
            line.whole && `tc__line--${line.whole.op}`,
            line.kind === 'task' && 'tc__task',
            line.whole && active === line.whole.change && 'tc__line--active',
          ]"
        >
          <input v-if="line.kind === 'task'" type="checkbox" :checked="line.checked" disabled />
          <template v-for="(run, r) in line.runs" :key="r">
            <component
              :is="run.mark ?? 'span'"
              :class="
                run.mark && [
                  'tc__mark',
                  `tc__mark--${run.mark}`,
                  active === run.change && 'tc__mark--active',
                ]
              "
              @mouseenter="run.mark && run.change && emit('hover', run.change)"
              @click="run.mark && run.change && emit('pick', run.change, $event)"
            >
              <template v-for="(piece, p) in inline(run.text)" :key="p">
                <strong v-if="piece.fmt === 'b'">{{ piece.text }}</strong>
                <em v-else-if="piece.fmt === 'i'">{{ piece.text }}</em>
                <code v-else-if="piece.fmt === 'code'">{{ piece.text }}</code>
                <a v-else-if="piece.fmt === 'a'" href="#" @click.prevent>{{ piece.text }}</a>
                <template v-else>{{ piece.text }}</template>
              </template>
            </component>
          </template>
        </li>
      </ul>
      <component
        :is="block.kind === 'quote' ? 'blockquote' : block.kind"
        v-else
        :class="[
          block.lines[0].whole && `tc__line--${block.lines[0].whole.op}`,
          block.lines[0].whole && active === block.lines[0].whole.change && 'tc__line--active',
        ]"
      >
        <template v-for="(line, l) in block.lines" :key="l">
          <template v-if="l > 0">{{ ' ' }}</template>
          <template v-for="(run, r) in line.runs" :key="r">
            <component
              :is="run.mark ?? 'span'"
              :class="
                run.mark && [
                  'tc__mark',
                  `tc__mark--${run.mark}`,
                  active === run.change && 'tc__mark--active',
                ]
              "
              @mouseenter="run.mark && run.change && emit('hover', run.change)"
              @click="run.mark && run.change && emit('pick', run.change, $event)"
            >
              <template v-for="(piece, p) in inline(run.text)" :key="p">
                <strong v-if="piece.fmt === 'b'">{{ piece.text }}</strong>
                <em v-else-if="piece.fmt === 'i'">{{ piece.text }}</em>
                <code v-else-if="piece.fmt === 'code'">{{ piece.text }}</code>
                <a v-else-if="piece.fmt === 'a'" href="#" @click.prevent>{{ piece.text }}</a>
                <template v-else>{{ piece.text }}</template>
              </template>
            </component>
          </template>
        </template>
      </component>
    </template>
  </article>
</template>

<style scoped>
.tc__doc {
  padding: var(--rl-space-5) var(--rl-space-6);
  line-height: 1.6;
  font-size: var(--rl-font-size-md);
}
.tc__doc :is(h1, h2, h3) {
  margin: var(--rl-space-4) 0 var(--rl-space-2);
  line-height: 1.3;
}
.tc__doc h1 {
  font-size: var(--rl-font-size-2xl);
  margin-top: 0;
}
.tc__doc h2 {
  font-size: var(--rl-font-size-xl);
}
.tc__doc h3 {
  font-size: var(--rl-font-size-lg);
}
.tc__doc p,
.tc__doc ul,
.tc__doc blockquote {
  margin: 0 0 var(--rl-space-3);
}
.tc__doc ul {
  padding-left: var(--rl-space-5);
}
.tc__doc blockquote {
  padding-left: var(--rl-space-3);
  border-left: 3px solid var(--rl-color-border-strong);
  color: var(--rl-color-text-muted);
}
.tc__doc code {
  font-family: var(--rl-font-family-mono);
  font-size: 0.9em;
  padding: 0 4px;
  border-radius: var(--rl-radius-sm);
  background: var(--rl-color-bg-hover);
}
.tc__task {
  list-style: none;
  margin-left: calc(-1 * var(--rl-space-5));
}
.tc__task input {
  margin-right: var(--rl-space-2);
}

/* Tracked changes: inserted text underlined in green, removed text struck in red. */
.tc__mark {
  cursor: pointer;
  border-radius: 2px;
  text-decoration-thickness: 2px;
}
.tc__mark--ins {
  text-decoration-line: underline;
  text-decoration-color: var(--rl-color-status-green);
  background: var(--rl-color-success-bg);
}
.tc__mark--del {
  text-decoration-line: line-through;
  text-decoration-color: var(--rl-color-danger);
  color: var(--rl-color-text-muted);
  background: var(--rl-color-danger-bg);
}
.tc__mark--fmt {
  text-decoration-line: underline;
  text-decoration-style: dotted;
  text-decoration-color: var(--rl-color-status-blue);
}
.tc__mark--active {
  outline: 2px solid var(--rl-color-focus);
  outline-offset: 1px;
}
.tc__line--ins,
.tc__line--del {
  position: relative;
}
.tc__line--ins::before,
.tc__line--del::before {
  content: '';
  position: absolute;
  left: calc(-1 * var(--rl-space-5) + 2px);
  top: 2px;
  bottom: 2px;
  width: 3px;
  border-radius: 2px;
  background: var(--rl-color-status-green);
}
.tc__line--del::before {
  background: var(--rl-color-danger);
}
li.tc__line--ins::before,
li.tc__line--del::before {
  left: calc(-1 * var(--rl-space-5) - var(--rl-space-3));
}

.tc__doc--dense {
  padding: 0;
  font-size: var(--rl-font-size-sm);
}
.tc__doc--dense h1 {
  font-size: var(--rl-font-size-lg);
}
.tc__doc--dense :is(h2, h3) {
  font-size: var(--rl-font-size-md);
}
</style>
