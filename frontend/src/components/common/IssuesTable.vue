<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import type { AnalyzeIssue } from '@/types'
import { useSchemaStore } from '@/stores'
import { useScriptErrorStore } from '@/stores/scriptError'

// Renders one analysis check's issues as a table. The row's click
// targets are split (TKT-IL499B): the entity-title cell navigates to the
// entity; the message cell reveals "why did this fire" detail — either
// the ScriptErrorDialog (Lua failures) or an expandable detail row
// listing structured detail (e.g. missing headers). Neither cell is
// interactive when it has nothing to do.
const props = defineProps<{ issues: AnalyzeIssue[] }>()

const router = useRouter()
const schemaStore = useSchemaStore()
const scriptErrorStore = useScriptErrorStore()

interface IssueRow {
  key: string
  issue: AnalyzeIssue
}

// A stable per-row key so expand state survives re-render. The index is
// part of the key because two distinct rules with the same Description
// can be violated by the same entity, yielding identical
// entityId+message rows; without the index those would collide (Vue
// treats duplicate keys as one node, cross-linking their expand state).
// The list only re-renders on a fresh analyze load, so index-based keys
// are stable within a render and resetting expand state on reload is
// acceptable.
function rowsFor(issues: AnalyzeIssue[]): IssueRow[] {
  return issues.map((issue, i) => ({
    key: `${i}:${issue.checkType}:${issue.entityType}:${issue.entityId}`,
    issue,
  }))
}

function getEntityTitle(issue: AnalyzeIssue): string {
  return issue.title?.trim() || issue.entityId
}

function getEntityTypeLabel(type: string): string {
  return schemaStore.entityTypes.get(type)?.label || type
}

const expandedRows = ref<Set<string>>(new Set())
function isExpanded(key: string): boolean {
  return expandedRows.value.has(key)
}

// The entity-title cell navigates. Inert when the issue has no entity
// (e.g. script-error / load-error rows).
function canNavigate(issue: AnalyzeIssue): boolean {
  return Boolean(issue.entityId && issue.entityType)
}
function onEntityClick(issue: AnalyzeIssue) {
  if (canNavigate(issue)) {
    router.push(`/entity/${issue.entityType}/${issue.entityId}`)
  }
}

// The message cell reveals detail: script-error rows open the dialog;
// rows with structured detail toggle the expandable detail row.
function hasDetailReveal(issue: AnalyzeIssue): boolean {
  return Boolean(issue.scriptError) || Boolean(issue.detail?.length)
}
function onMessageClick(key: string, issue: AnalyzeIssue, ev: Event) {
  if (issue.scriptError) {
    const trigger = ev.currentTarget instanceof HTMLElement ? ev.currentTarget : null
    scriptErrorStore.show(issue.scriptError, trigger)
    return
  }
  if (issue.detail?.length) {
    const next = new Set(expandedRows.value)
    if (next.has(key)) next.delete(key)
    else next.add(key)
    expandedRows.value = next
  }
}
</script>

<template>
  <div class="issues-table-wrapper">
    <table class="issues-table">
      <thead>
        <tr>
          <th>Entity</th>
          <th>Type</th>
          <th>Message</th>
          <th>Severity</th>
        </tr>
      </thead>
      <tbody>
        <template v-for="row in rowsFor(props.issues)" :key="row.key">
          <tr class="issue-row">
            <td class="entity-cell">
              <template v-if="row.issue.entityId">
                <span
                  class="entity-title"
                  :class="{ clickable: canNavigate(row.issue) }"
                  :role="canNavigate(row.issue) ? 'button' : undefined"
                  :tabindex="canNavigate(row.issue) ? 0 : undefined"
                  @click="onEntityClick(row.issue)"
                  @keydown.enter="onEntityClick(row.issue)"
                  @keydown.space.prevent="onEntityClick(row.issue)"
                  >{{ getEntityTitle(row.issue) }}</span
                >
                <span class="entity-id">{{ row.issue.entityId }}</span>
              </template>
              <span v-else class="entity-empty">&mdash;</span>
            </td>
            <td>
              <span v-if="row.issue.entityType" class="type-badge">{{
                getEntityTypeLabel(row.issue.entityType)
              }}</span>
              <span v-else class="entity-empty">&mdash;</span>
            </td>
            <td class="message-cell">
              <span
                v-if="hasDetailReveal(row.issue)"
                class="message-toggle"
                role="button"
                tabindex="0"
                :aria-expanded="row.issue.detail?.length ? isExpanded(row.key) : undefined"
                @click="onMessageClick(row.key, row.issue, $event)"
                @keydown.enter="onMessageClick(row.key, row.issue, $event)"
                @keydown.space.prevent="onMessageClick(row.key, row.issue, $event)"
              >
                <span
                  v-if="row.issue.detail?.length"
                  class="disclosure"
                  :class="{ open: isExpanded(row.key) }"
                  aria-hidden="true"
                  >&#9656;</span
                >
                {{ row.issue.message }}
              </span>
              <span v-else>{{ row.issue.message }}</span>
            </td>
            <td>
              <span class="severity-badge" :class="row.issue.severity">
                {{ row.issue.severity.toUpperCase() }}
              </span>
            </td>
          </tr>
          <tr v-if="row.issue.detail?.length && isExpanded(row.key)" class="issue-detail-row">
            <td colspan="4">
              <div class="detail-label">Missing required headers:</div>
              <ul class="detail-list">
                <li v-for="(d, i) in row.issue.detail" :key="i">{{ d }}</li>
              </ul>
            </td>
          </tr>
        </template>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.issues-table {
  width: 100%;
  border-collapse: collapse;
}

.issues-table th {
  text-align: left;
  padding: 10px 16px;
  background: var(--hover-bg);
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--muted-text);
  border-bottom: 1px solid var(--border-color);
}

.issues-table td {
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-color);
  font-size: 14px;
}

.issue-row {
  transition: background 0.15s;
}

.issue-row:last-child td {
  border-bottom: none;
}

/* `display: flex` on the <td> collapses the cell box so its border
 * doesn't span the row height (visible discontinuity between rows).
 * Keep the td as a normal table-cell and stack the two spans with
 * block display + margin instead. */
.entity-title {
  display: block;
  color: var(--accent-color, #6366f1);
  font-weight: 500;
}

/* Split click targets (TKT-IL499B): the entity title navigates, the
 * message toggles detail. Each is an independently focusable button. */
.entity-title.clickable {
  cursor: pointer;
}

.entity-title.clickable:hover {
  text-decoration: underline;
}

.entity-title.clickable:focus-visible {
  outline: 2px solid var(--accent-color, #6366f1);
  outline-offset: 2px;
  border-radius: 2px;
}

.message-toggle {
  cursor: pointer;
  display: inline;
}

.message-toggle:focus-visible {
  outline: 2px solid var(--accent-color, #6366f1);
  outline-offset: 2px;
  border-radius: 2px;
}

.disclosure {
  display: inline-block;
  margin-right: 4px;
  font-size: 11px;
  color: var(--muted-text);
  transition: transform 0.15s;
}

.disclosure.open {
  transform: rotate(90deg);
}

.issue-detail-row td {
  background: var(--hover-bg);
  padding: 8px 20px 12px;
}

.detail-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--muted-text);
  margin-bottom: 4px;
}

.detail-list {
  margin: 0;
  padding-left: 20px;
}

.detail-list li {
  font-family: monospace;
  font-size: 13px;
  color: var(--text-color);
}

.entity-id {
  display: block;
  margin-top: 2px;
  font-family: monospace;
  font-size: 12px;
  color: var(--muted-text);
}

.entity-empty {
  color: var(--muted-text);
  font-size: 14px;
}

.type-badge {
  display: inline-block;
  padding: 4px 8px;
  background: var(--hover-bg);
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  color: var(--muted-text);
}

.message-cell {
  color: var(--text-color);
  overflow-wrap: anywhere;
  word-break: break-word;
}

.severity-badge {
  display: inline-block;
  padding: 4px 10px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
}

.severity-badge.error {
  background: color-mix(in srgb, var(--error-color) 15%, transparent);
  color: var(--error-color);
}

.severity-badge.warning {
  background: color-mix(in srgb, var(--warning-color) 15%, transparent);
  color: var(--warning-color);
}

.issues-table-wrapper {
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}

@media (max-width: 768px) {
  .issues-table th,
  .issues-table td {
    padding: 8px 10px;
    font-size: 12px;
  }
}
</style>
