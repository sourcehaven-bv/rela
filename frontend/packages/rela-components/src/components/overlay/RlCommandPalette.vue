<script setup lang="ts">
/**
 * The Ctrl+K dialog: type to narrow a list of commands, arrow to one, Enter
 * to run it.
 *
 * Built on `RlModal` with `align="top"`, which is where a dialog the user
 * types into belongs: centred, the list grows in both directions and the
 * eye has to chase it. The modal already owns the scrim, the scroll lock,
 * the focus trap and the Escape, so this adds only the part that is a
 * palette: the filter, the cursor and the grouping.
 *
 * Filtering is the caller's when `filter` is false. A palette over anything
 * real searches a server, and matching on the label here would quietly
 * disagree with what the server returned.
 *
 * It holds the cursor but never the open state. Which key opens a palette is
 * the app's, and an app that binds Ctrl+K also wants to open it from a menu
 * item and from a button, so the state has to live above all three.
 */
import { computed, nextTick, ref, watch } from 'vue'
import RlModal from './RlModal.vue'
import RlIcon from '../common/RlIcon.vue'
import RlKbd from '../data/RlKbd.vue'
import RlEmptyState from '../feedback/RlEmptyState.vue'
import type { IconName } from '../common/icons'

export interface Command {
  id: string
  label: string
  /** A line under the label, for what the command does or where it goes. */
  description?: string
  icon?: IconName
  /** The shortcut that runs it directly, drawn at the end of the row. */
  keys?: string
  /** Heading this command sits under. Commands with none come first. */
  group?: string
  disabled?: boolean
}

const props = withDefaults(
  defineProps<{
    open: boolean
    commands: Command[]
    placeholder?: string
    /**
     * Whether the palette narrows the list itself. On for a list held in
     * memory; off where the caller is searching a server and `commands` is
     * already the answer.
     */
    filter?: boolean
    /** Shown when nothing matches. */
    emptyLabel?: string
    /** Names the dialog for a screen reader; the title is never drawn. */
    title?: string
  }>(),
  {
    placeholder: 'Type a command or search',
    filter: true,
    emptyLabel: 'No matching commands',
    title: 'Command palette',
  },
)

const emit = defineEmits<{
  'update:open': [value: boolean]
  /** A command was chosen. The palette closes itself first. */
  run: [command: Command]
  /** The query changed, for a caller filtering on a server. */
  'update:query': [value: string]
}>()

const query = ref('')
const cursor = ref(0)
const listRef = ref<HTMLElement | null>(null)

const matches = computed(() => {
  const enabled = props.commands
  if (!props.filter) return enabled
  const needle = query.value.trim().toLowerCase()
  if (!needle) return enabled
  return enabled.filter((command) =>
    `${command.label} ${command.description ?? ''}`.toLowerCase().includes(needle),
  )
})

/*
 * Grouped for display but kept flat for the cursor, so arrowing down crosses
 * a heading without stopping on it. A heading is not a destination.
 */
const groups = computed(() => {
  const result: { name: string | null; commands: Command[] }[] = []
  for (const command of matches.value) {
    const name = command.group ?? null
    const last = result[result.length - 1]
    if (last && last.name === name) last.commands.push(command)
    else result.push({ name, commands: [command] })
  }
  return result
})

/** Where each command sits in the flat order, which is what the cursor counts. */
function indexOf(command: Command) {
  return matches.value.indexOf(command)
}

const active = computed(() => matches.value[cursor.value])

/* A new query means a new list, so the cursor goes back to the top of it. */
watch(
  () => [query.value, props.commands] as const,
  () => {
    cursor.value = 0
    emit('update:query', query.value)
  },
)

/* Reopening starts clean: a palette that remembered the last query would
 * make the common case, typing something new, begin with a deletion. */
watch(
  () => props.open,
  (open) => {
    if (!open) return
    query.value = ''
    cursor.value = 0
  },
)

function move(delta: number) {
  const total = matches.value.length
  if (!total) return
  /* Wraps, because a list this short is faster to cycle than to reverse. */
  cursor.value = (cursor.value + delta + total) % total
  void scrollCursorIntoView()
}

async function scrollCursorIntoView() {
  await nextTick()
  listRef.value
    ?.querySelector('[data-active="true"]')
    ?.scrollIntoView({ block: 'nearest' })
}

function run(command: Command) {
  if (command.disabled) return
  emit('update:open', false)
  emit('run', command)
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    move(1)
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    move(-1)
  } else if (event.key === 'Enter') {
    /*
     * Prevented whether or not something is active, so Enter on an empty
     * result never submits a form the palette happens to be inside.
     */
    event.preventDefault()
    if (active.value) run(active.value)
  } else if (event.key === 'Home') {
    event.preventDefault()
    cursor.value = 0
    void scrollCursorIntoView()
  } else if (event.key === 'End') {
    event.preventDefault()
    cursor.value = Math.max(0, matches.value.length - 1)
    void scrollCursorIntoView()
  }
}
</script>

<template>
  <RlModal
    :open="open"
    :title="title"
    title-hidden
    align="top"
    size="md"
    :show-close="false"
    @update:open="emit('update:open', $event)"
  >
    <div class="rl-command-palette">
      <div class="rl-command-palette__search">
        <RlIcon name="search" :size="16" aria-hidden="true" />
        <!--
          `combobox` rather than a plain field: the input keeps focus while
          the cursor moves through the list, so the list has to be named as
          the thing being controlled and the active row pointed at.
        -->
        <input
          v-model="query"
          type="text"
          class="rl-command-palette__input"
          role="combobox"
          :placeholder="placeholder"
          :aria-expanded="matches.length > 0"
          aria-controls="rl-command-palette-list"
          aria-autocomplete="list"
          :aria-activedescendant="active ? `rl-command-${active.id}` : undefined"
          @keydown="onKeydown"
        />
      </div>

      <div
        v-if="matches.length"
        id="rl-command-palette-list"
        ref="listRef"
        class="rl-command-palette__list"
        role="listbox"
        :aria-label="title"
      >
        <template v-for="group in groups" :key="group.name ?? '_'">
          <p v-if="group.name" class="rl-command-palette__group" aria-hidden="true">
            {{ group.name }}
          </p>

          <!--
            A div with `option`, not a button: inside a listbox the rows are
            options, and focus stays in the input throughout.
          -->
          <div
            v-for="command in group.commands"
            :id="`rl-command-${command.id}`"
            :key="command.id"
            class="rl-command-palette__item"
            :class="{ 'rl-command-palette__item--disabled': command.disabled }"
            role="option"
            :aria-selected="indexOf(command) === cursor"
            :aria-disabled="command.disabled || undefined"
            :data-active="indexOf(command) === cursor"
            @click="run(command)"
            @mousemove="cursor = indexOf(command)"
          >
            <RlIcon v-if="command.icon" :name="command.icon" :size="16" class="rl-command-palette__icon" />

            <span class="rl-command-palette__text">
              <span class="rl-command-palette__label">{{ command.label }}</span>
              <span v-if="command.description" class="rl-command-palette__description">
                {{ command.description }}
              </span>
            </span>

            <RlKbd v-if="command.keys" :keys="command.keys" />
          </div>
        </template>
      </div>

      <RlEmptyState v-else icon="search" :title="emptyLabel" class="rl-command-palette__empty" />
    </div>
  </RlModal>
</template>

<style scoped>
.rl-command-palette {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.rl-command-palette__search {
  display: flex;
  align-items: center;
  gap: var(--rl-space-3);
  flex: none;
  padding-bottom: var(--rl-space-3);
  border-bottom: 1px solid var(--rl-color-border);
  color: var(--rl-color-text-subtle);
}

.rl-command-palette__input {
  flex: 1;
  min-width: 0;
  border: none;
  background: none;
  font: inherit;
  font-size: var(--rl-font-size-lg);
  color: var(--rl-color-text);
}

/* The field is the dialog here, so its own ring would box in what is already framed. */
.rl-command-palette__input:focus { outline: none; }
.rl-command-palette__input::placeholder { color: var(--rl-color-text-subtle); }

.rl-command-palette__list {
  flex: 1;
  min-height: 0;
  /* Caps at roughly eight rows, so the dialog stays put as the list changes. */
  max-height: 336px;
  overflow-y: auto;
  padding-top: var(--rl-space-2);
  margin: 0 calc(-1 * var(--rl-space-2));
}

.rl-command-palette__group {
  margin: var(--rl-space-3) var(--rl-space-2) var(--rl-space-1);
  font-size: var(--rl-font-size-xs);
  font-weight: var(--rl-font-weight-medium);
  color: var(--rl-color-text-subtle);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.rl-command-palette__item {
  display: flex;
  align-items: center;
  gap: var(--rl-space-3);
  padding: var(--rl-space-2) var(--rl-space-2);
  border-radius: var(--rl-radius-md);
  cursor: pointer;
  color: var(--rl-color-text);
}

/*
 * One highlight, driven by the cursor rather than by hover, so the mouse and
 * the keyboard cannot each show a different row as the one Enter will run.
 * `mousemove` moves the cursor instead, which keeps them agreeing.
 */
.rl-command-palette__item[data-active='true'] {
  background: var(--rl-color-bg-selected);
}

.rl-command-palette__item--disabled {
  cursor: default;
  color: var(--rl-color-text-subtle);
}

.rl-command-palette__icon {
  flex: none;
  color: var(--rl-color-text-subtle);
}

.rl-command-palette__text {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.rl-command-palette__label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rl-command-palette__description {
  font-size: var(--rl-font-size-sm);
  color: var(--rl-color-text-subtle);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rl-command-palette__empty { padding: var(--rl-space-6) 0; }
</style>
