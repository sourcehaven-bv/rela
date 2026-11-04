<script setup lang="ts">
/*
 * Mockup: editing owned and related items inline on the parent's page.
 *
 * Three patterns, one per kind of child:
 *
 * - `table`: owned items of one type (`owning: quote-line`, `orderable:`).
 *   An `RlTable` whose cells edit in place.
 * - `blocks`: owned items of several types (a page built from blocks). Each
 *   block is an accordion holding its own type's fields; "Add block" picks
 *   the type.
 * - `related`: entities the parent links to but does not own. Edits say who
 *   else sees them, and removing one unlinks it rather than deleting it.
 *
 * Edits save through rela's existing autosave and its indicator, which the
 * mockup leaves out. Removing an owned item deletes it, with Undo in a toast
 * (rela's soft-delete window). Nothing is stored.
 */
import { computed, ref } from 'vue'

import RlIconButton from '../components/common/RlIconButton.vue'
import RlStatusPill from '../components/common/RlStatusPill.vue'
import RlAddButton from '../components/common/RlAddButton.vue'
import RlInlineEdit from '../components/common/RlInlineEdit.vue'
import RlText from '../components/common/RlText.vue'
import RlMarkdownEditor from '../components/editor/RlMarkdownEditor.vue'
import RlSectionHeading from '../components/layout/RlSectionHeading.vue'
import RlOptionSelect from '../components/form/RlOptionSelect.vue'
import RlTable from '../components/table/RlTable.vue'
import RlSortableList from '../components/data/RlSortableList.vue'
import RlAccordion from '../components/data/RlAccordion.vue'
import RlRelatedList from '../components/data/RlRelatedList.vue'
import RlTotals from '../components/data/RlTotals.vue'
import RlDetailField from '../components/task/RlDetailField.vue'
import RlMenu from '../components/overlay/RlMenu.vue'
import RlMenuItem from '../components/overlay/RlMenuItem.vue'
import RlMenuSeparator from '../components/overlay/RlMenuSeparator.vue'
import RlToastHost from '../components/feedback/RlToastHost.vue'
import { useToasts } from '../components/feedback/useToasts'
import RlCallout from '../components/feedback/RlCallout.vue'
import type { IconName } from '../components/common/icons'
import type { TableColumn, RowMove } from '../components/table/types'
import type { DetailField, StatusOption } from '../types'

const props = withDefaults(defineProps<{ pattern?: 'all' | 'table' | 'blocks' | 'related' }>(), {
  pattern: 'all',
})
const show = (p: 'table' | 'blocks' | 'related') => props.pattern === 'all' || props.pattern === p

const { toasts, dismiss, show: toast } = useToasts()
/** Deletes `item` from `list` at once and offers to put it back. */
function removeWithUndo<T extends { id: string }>(list: { value: T[] }, item: T, what: string) {
  const index = list.value.findIndex((x) => x.id === item.id)
  list.value = list.value.filter((x) => x.id !== item.id)
  toast({
    title: `${what} deleted`,
    action: {
      label: 'Undo',
      onAction: () => {
        list.value.splice(index, 0, item)
      },
    },
  })
}

/** Applies a table reorder to a copy of `list`. */
function applyMove<T extends { id: string }>(list: T[], move: RowMove): T[] {
  const from = list.findIndex((x) => x.id === move.itemId)
  if (from === -1) return list
  const next = [...list]
  const [item] = next.splice(from, 1)
  let to: number
  if ('step' in move) to = Math.max(0, Math.min(next.length, from + move.step))
  else {
    const target = next.findIndex((x) => x.id === move.targetId)
    to = move.placement === 'before' ? target : target + 1
  }
  next.splice(to, 0, item)
  return next
}

/* ---------------------------------------------------------------- table -- */

interface Line {
  id: string
  title: string
  qty: string
  price: string
  vat: string
}

const initialLines = (): Line[] => [
  { id: 'QL-1', title: 'Discovery workshop', qty: '1', price: '1450.00', vat: '21%' },
  { id: 'QL-2', title: 'Schema design', qty: '12', price: '115.00', vat: '21%' },
  { id: 'QL-3', title: 'Hosting (per month)', qty: '12', price: '49.50', vat: '21%' },
  { id: 'QL-4', title: 'Training material', qty: '1', price: '320.00', vat: '9%' },
]
const lines = ref<Line[]>(initialLines())
let nextLine = 5

const lineColumns: TableColumn[] = [
  { key: 'qty', header: 'Qty', width: 72, align: 'end' },
  { key: 'price', header: 'Unit price', width: 110, align: 'end' },
  { key: 'vat', header: 'VAT', width: 84 },
  { key: 'total', header: 'Total', width: 110, align: 'end' },
  { key: 'actions', header: '', width: 48 },
]

const cents = (v: string) => Math.round((Number.parseFloat(v.replace(',', '.')) || 0) * 100)
/** In cents. */
const lineTotal = (l: Line) => Math.round((cents(l.qty) * cents(l.price)) / 100)
const euro = (c: number) =>
  new Intl.NumberFormat('nl-NL', { style: 'currency', currency: 'EUR' }).format(c / 100)

const subtotal = computed(() => lines.value.reduce((s, l) => s + lineTotal(l), 0))
const vat = computed(() =>
  lines.value.reduce((s, l) => s + Math.round((lineTotal(l) * Number.parseInt(l.vat)) / 100), 0),
)
const totals = computed(() => [
  { label: 'Subtotal', value: euro(subtotal.value) },
  { label: 'VAT', value: euro(vat.value) },
  { label: 'Total', value: euro(subtotal.value + vat.value), emphasis: true },
])
const lineSections = computed(() => [
  { id: 'lines', title: 'Lines', count: lines.value.length, items: lines.value },
])

/** The value being edited. One field edits at a time. */
const draft = ref('')
function commitLine(l: Line, key: 'title' | 'qty' | 'price') {
  if (draft.value === l[key]) return
  l[key] = draft.value
}
function setVat(l: Line, v: string) {
  l.vat = v
}
function addLine() {
  lines.value.push({ id: `QL-new-${nextLine++}`, title: '', qty: '1', price: '0.00', vat: '21%' })
}
function removeLine(l: Line) {
  removeWithUndo(lines, l, l.title ? `"${l.title}"` : 'Line')
}
function duplicateLine(l: Line) {
  const i = lines.value.indexOf(l)
  lines.value.splice(i + 1, 0, { ...l, id: `QL-new-${nextLine++}` })
}
function onLineReorder(move: RowMove) {
  lines.value = applyMove(lines.value, move)
}

/* --------------------------------------------------------------- blocks -- */

const blockTypes: Record<string, { label: string; icon: IconName; fields: [string, string][] }> = {
  hero: { label: 'Hero', icon: 'image', fields: [['heading', 'Heading'], ['image', 'Image']] },
  text: { label: 'Text', icon: 'document', fields: [['body', 'Body']] },
  cta: { label: 'Call to action', icon: 'target', fields: [['label', 'Button label'], ['target', 'Links to']] },
  faq: { label: 'FAQ item', icon: 'help', fields: [['question', 'Question'], ['answer', 'Answer']] },
}
/** Fields holding markdown prose, edited in the markdown editor. */
const proseFields = new Set(['body', 'answer'])

interface Block {
  id: string
  title: string
  type: string
  values: Record<string, string>
  open: boolean
}

const initialBlocks = (): Block[] => [
  {
    id: 'BLK-1',
    title: 'Hero',
    type: 'hero',
    values: { heading: 'Model your domain, not your database', image: 'hero-graph.png' },
    open: false,
  },
  {
    id: 'BLK-2',
    title: 'Text',
    type: 'text',
    values: {
      body: 'rela turns a YAML schema into **typed entities**, relations and a web app.\n\n- Your data stays in plain markdown files\n- Every change is in the audit log',
    },
    open: true,
  },
  {
    id: 'BLK-3',
    title: 'Call to action',
    type: 'cta',
    values: { label: 'Start a project', target: 'GUIDE-getting-started' },
    open: false,
  },
]
const blocks = ref<Block[]>(initialBlocks())
let nextBlock = 4

const blockFields = (b: Block): DetailField[] =>
  blockTypes[b.type].fields.map(([id, label]) => ({ id, label, type: 'text', value: b.values[id] }))
/** The first line of the first filled field, as plain text: a summary, not a render. */
const blockSummary = (b: Block) => {
  const first = Object.values(b.values).find(Boolean)
  if (!first) return 'Empty'
  const line = first.split('\n').find((l) => l.trim()) ?? ''
  return line.replace(/^\s*(#+|[-*+]|\d+\.)\s+/, '').replace(/[*_`]/g, '')
}

function setBlockValue(b: Block, id: string, value: string) {
  if (b.values[id] === value) return
  b.values[id] = value
}
function addBlock(type: string) {
  blocks.value.forEach((b) => (b.open = false))
  const values = Object.fromEntries(blockTypes[type].fields.map(([id]) => [id, '']))
  blocks.value.push({
    id: `BLK-new-${nextBlock++}`,
    title: blockTypes[type].label,
    type,
    values,
    open: true,
  })
}
function removeBlock(b: Block) {
  removeWithUndo(blocks, b, `${blockTypes[b.type].label} block`)
}
function moveBlock(item: Block, to: number) {
  const rest = blocks.value.filter((b) => b.id !== item.id)
  rest.splice(to, 0, item)
  blocks.value = rest
}

/* -------------------------------------------------------------- related -- */

const productStatuses: StatusOption[] = [
  { value: 'Draft', label: 'Draft', status: 'amber' },
  { value: 'Active', label: 'Active', status: 'green' },
  { value: 'Retired', label: 'Retired', status: 'grey' },
]

interface Product {
  id: string
  title: string
  fields: DetailField[]
  sharedWith: number
  open: boolean
}
const products = ref<Product[]>([
  {
    id: 'PRD-007',
    title: 'Hosted rela (Postgres)',
    sharedWith: 14,
    open: false,
    fields: [
      { id: 'title', label: 'Title', type: 'text', value: 'Hosted rela (Postgres)' },
      { id: 'status', label: 'Status', type: 'status', value: 'Active', status: 'green' },
      { id: 'owner', label: 'Owner', type: 'text', value: 'Sanne' },
    ],
  },
  {
    id: 'PRD-012',
    title: 'Onboarding package',
    sharedWith: 2,
    open: true,
    fields: [
      { id: 'title', label: 'Title', type: 'text', value: 'Onboarding package' },
      { id: 'status', label: 'Status', type: 'status', value: 'Draft', status: 'amber' },
      { id: 'owner', label: 'Owner', type: 'text', value: 'Ruben' },
    ],
  },
])
const statusOf = (p: Product) => p.fields.find((f) => f.id === 'status')!
function setProductField(p: Product, f: DetailField) {
  p.fields = p.fields.map((x) => (x.id === f.id ? f : x))
  if (f.id === 'title') p.title = f.value ?? ''
}
function unlink(p: Product) {
  products.value = products.value.filter((x) => x.id !== p.id)
}

</script>

<template>
  <div class="owned">

    <!-- ------------------------------------------------------- table -->
    <section v-if="show('table')" class="owned__section">
      <RlTable
        :sections="lineSections"
        :columns="lineColumns"
        name-label="Description"
        add-label="Add line"
        :reorderable="() => true"
        @add="addLine"
        @reorder="onLineReorder"
      >
        <template #name="{ item: l }">
          <RlInlineEdit
            label="Description"
            :empty="!l.title"
            placeholder="Describe the line"
            @edit="draft = l.title"
            @commit="commitLine(l, 'title')"
          >
            <template #read>{{ l.title }}</template>
            <template #edit>
              <input v-model="draft" class="rl-control rl-control--sm" aria-label="Description" />
            </template>
          </RlInlineEdit>
        </template>

        <template #cell-qty="{ item: l }">
          <RlInlineEdit
            label="Quantity"
            numeric
            @edit="draft = l.qty"
            @commit="commitLine(l, 'qty')"
          >
            <template #read>{{ l.qty }}</template>
            <template #edit>
              <input v-model="draft" class="rl-control rl-control--sm" inputmode="decimal" aria-label="Quantity" />
            </template>
          </RlInlineEdit>
        </template>

        <template #cell-price="{ item: l }">
          <RlInlineEdit
            label="Unit price"
            numeric
            @edit="draft = l.price"
            @commit="commitLine(l, 'price')"
          >
            <template #read>{{ l.price }}</template>
            <template #edit>
              <input v-model="draft" class="rl-control rl-control--sm" inputmode="decimal" aria-label="Unit price" />
            </template>
          </RlInlineEdit>
        </template>

        <template #cell-vat="{ item: l }">
          <RlOptionSelect
            variant="inline"
            label="VAT"
            :model-value="l.vat"
            :options="['21%', '9%', '0%']"
            @update:model-value="setVat(l, $event)"
          >
            <template #option="{ value }">{{ value }}</template>
          </RlOptionSelect>
        </template>

        <template #cell-total="{ item: l }">
          <RlText tone="muted" numeric>{{ euro(lineTotal(l)) }}</RlText>
        </template>

        <template #cell-actions="{ item: l }">
          <RlMenu align="end">
            <template #trigger="{ toggle, attrs }">
              <RlIconButton icon="ellipsis" :label="`Options for ${l.title || 'new line'}`" v-bind="attrs" @click="toggle" />
            </template>
            <RlMenuItem icon="external">Open as page</RlMenuItem>
            <RlMenuItem icon="copy" @click="duplicateLine(l)">Duplicate</RlMenuItem>
            <RlMenuSeparator />
            <RlMenuItem icon="delete" tone="danger" @click="removeLine(l)">Delete line</RlMenuItem>
          </RlMenu>
        </template>
      </RlTable>

      <RlTotals class="owned__totals" label="Quote totals" :rows="totals" />
    </section>

    <!-- ------------------------------------------------------ blocks -->
    <section v-if="show('blocks')" class="owned__section">
      <RlSectionHeading title="Page blocks" :count="blocks.length" size="sm" :level="2" />

      <RlSortableList :items="blocks" label="Page blocks" @move="moveBlock">
        <template #item="{ item: b }">
          <RlAccordion
            :divider="false"
            :title="blockTypes[b.type].label"
            :meta="b.open ? undefined : blockSummary(b)"
            :open="b.open"
            @toggle="b.open = $event"
          >
            <dl class="owned__fields">
              <template v-for="f in blockFields(b)" :key="f.id">
                <!-- Prose is a block's own markdown body: the same Milkdown
                     editor as an entity body, read-only until clicked. -->
                <RlDetailField v-if="proseFields.has(f.id)" :field="f" stacked>
                  <template #value>
                    <RlInlineEdit block :label="f.label" :empty="!f.value" @edit="draft = f.value ?? ''">
                      <template #read>
                        <RlMarkdownEditor :model-value="f.value" readonly />
                      </template>
                      <template #edit>
                        <RlMarkdownEditor :model-value="draft" @update:model-value="setBlockValue(b, f.id, $event)" />
                      </template>
                    </RlInlineEdit>
                  </template>
                </RlDetailField>
                <RlDetailField
                  v-else
                  :field="f"
                  editable
                  @update:field="setBlockValue(b, f.id, $event.value ?? '')"
                />
              </template>
            </dl>

            <template #actions>
              <RlMenu align="end">
                <template #trigger="{ toggle, attrs }">
                  <RlIconButton
                    icon="ellipsis"
                    :label="`Options for ${blockTypes[b.type].label} block`"
                    v-bind="attrs"
                    @click="toggle"
                  />
                </template>
                <RlMenuItem icon="copy">Duplicate</RlMenuItem>
                <RlMenuItem icon="refresh">Change type…</RlMenuItem>
                <RlMenuSeparator />
                <RlMenuItem icon="delete" tone="danger" @click="removeBlock(b)">Delete block</RlMenuItem>
              </RlMenu>
            </template>
          </RlAccordion>
        </template>
      </RlSortableList>

      <RlMenu class="owned__add-menu">
        <template #trigger="{ toggle, attrs }">
          <RlAddButton label="Add block" v-bind="attrs" @click="toggle" />
        </template>
        <RlMenuItem v-for="(t, key) in blockTypes" :key="key" :icon="t.icon" @click="addBlock(key)">
          {{ t.label }}
        </RlMenuItem>
      </RlMenu>
    </section>

    <!-- ----------------------------------------------------- related -->
    <section v-if="show('related')" class="owned__section">
      <RlRelatedList title="Products" :items="products" add-label="Link product" :level="2">
        <template #row="{ item: p }">
          <RlAccordion :divider="false" :title="p.title" :meta="p.id" :open="p.open" @toggle="p.open = $event">
            <template #summaryExtra>
              <RlStatusPill :label="statusOf(p).value ?? ''" :color="statusOf(p).status" />
            </template>

            <RlCallout tone="warning">
              {{ p.id }} is linked from {{ p.sharedWith }} other entities. Changes here show there too.
            </RlCallout>
            <dl class="owned__fields">
              <RlDetailField
                v-for="f in p.fields"
                :key="f.id"
                :field="f"
                editable
                :options="productStatuses"
                @update:field="setProductField(p, $event)"
              />
            </dl>

            <template #actions>
              <RlMenu align="end">
                <template #trigger="{ toggle, attrs }">
                  <RlIconButton icon="ellipsis" :label="`Options for ${p.title}`" v-bind="attrs" @click="toggle" />
                </template>
                <RlMenuItem icon="external">Open {{ p.id }}</RlMenuItem>
                <RlMenuItem icon="refresh">Replace with…</RlMenuItem>
                <RlMenuSeparator />
                <RlMenuItem icon="link" tone="danger" @click="unlink(p)">Unlink from quote</RlMenuItem>
              </RlMenu>
            </template>
          </RlAccordion>
        </template>
      </RlRelatedList>
    </section>

    <RlToastHost :toasts="toasts" @dismiss="dismiss" />
  </div>
</template>

<style scoped>
.owned {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-8);
  max-width: 920px;
}
.owned__section {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-3);
}
.owned__add-menu {
  align-self: flex-start;
}
/* Pushed under the Total column; the space is the actions column's width. */
.owned__totals {
  margin: 0 calc(var(--rl-space-8) + var(--rl-space-4)) 0 auto;
}

.owned__fields {
  display: flex;
  flex-direction: column;
  gap: var(--rl-space-3);
  margin: var(--rl-space-3) 0;
}
</style>
