import type { Meta, StoryObj } from '@storybook/vue3-vite'
import type { Section, Task } from '../types'
import { computed, defineComponent, h, ref } from 'vue'

import RlAppShell from '../components/layout/RlAppShell.vue'
import RlSidebar from '../components/layout/RlSidebar.vue'
import RlPageHeader from '../components/layout/RlPageHeader.vue'
import RlViewTabs from '../components/layout/RlViewTabs.vue'
import type { ViewTab } from '../components/layout/RlViewTabs.vue'
import RlButton from '../components/common/RlButton.vue'
import RlIcon from '../components/common/RlIcon.vue'
import RlIconButton from '../components/common/RlIconButton.vue'
import RlStatusPill from '../components/common/RlStatusPill.vue'
import RlBoard from '../fixtures/RlTaskBoard.vue'
import RlMyTasksPanels from '../fixtures/RlMyTasksPanels.vue'
import RlTable from '../fixtures/RlTaskTable.vue'
import RlTaskDetail from '../components/task/RlTaskDetail.vue'
import RlDetailPanel from '../components/task/RlDetailPanel.vue'
import RlComment from '../components/task/RlComment.vue'
import RlCommentComposer from '../components/task/RlCommentComposer.vue'
import RlDocEditor from '../components/doc/RlDocEditor.vue'
import RlDocToolbar from '../components/doc/RlDocToolbar.vue'

import {
  navGroups,
  meetingNavGroups,
  boardSections,
  tableSections,
  tableColumns,
  detailFields,
  detailDescription,
  detailAttachments,
  detailSubtasks,
  detailComments,
} from '../fixtures'

const viewTabs = [
  { id: 'board', label: 'Board', icon: 'kanban' as const },
  { id: 'table', label: 'Table', icon: 'table' as const },
  { id: 'timeline', label: 'Timeline', icon: 'chart-no-axes-gantt' as const },
]

/**
 * The same tabs, with the panel behaviour each view needs. A timeline is read
 * across its full width, so the panel floats above it instead of taking a
 * share of the row.
 */
const panelTabs: ViewTab[] = [
  { id: 'board', label: 'Board', icon: 'kanban', panelMode: 'inline' },
  { id: 'table', label: 'Table', icon: 'table', panelMode: 'inline' },
  { id: 'timeline', label: 'Timeline', icon: 'chart-no-axes-gantt', panelMode: 'overlay' },
]

/*
 * Board and table fixtures are separate datasets, so a task selected in one
 * has no counterpart in the other. The three-pane story needs one dataset
 * across all its views for selection to carry, so it derives its board from
 * the table's sections.
 */
const sharedBoardSections: Section[] = tableSections.map((section) => ({
  ...section,
  collapsed: false,
}))

/** Stand-in for the timeline view, which this library does not ship yet. */
const RlTimelinePlaceholder = defineComponent({
  name: 'RlTimelinePlaceholder',
  setup: () => () =>
    h(
      'div',
      {
        style: {
          flex: '1',
          display: 'grid',
          placeItems: 'center',
          margin: '16px',
          border: '1px dashed var(--rl-color-border-strong)',
          borderRadius: 'var(--rl-radius-lg)',
          color: 'var(--rl-color-text-subtle)',
        },
      },
      'Timeline view',
    ),
})

const meta: Meta = {
  title: 'Pages',
  parameters: { layout: 'fullscreen' },
}
export default meta

/** Screen 1: initiative board with a collapsed Postpone column. */
export const InitiativeBoard: StoryObj = {
  render: () => ({
    components: {
      RlAppShell, RlSidebar, RlMyTasksPanels, RlPageHeader, RlViewTabs, RlBoard, RlButton, RlIcon,
    },
    setup: () => ({ navGroups, boardSections, viewTabs, view: ref('board'), navOpen: ref(false), sidebarWidth: ref(260), myTasksOpen: ref(false) }),
    template: `
      <RlMyTasksPanels v-model:open="myTasksOpen" :sidebar-width="sidebarWidth">
        <RlAppShell v-model:nav-open="navOpen" v-model:sidebar-width="sidebarWidth" :sidebar-default-width="260">
          <template #sidebar>
            <RlSidebar
              workspace-name="Atlas Projects"
              :groups="navGroups"
              active-id="atlas"
              :flyout-id="myTasksOpen ? 'my-tasks' : null"
              @select="myTasksOpen = $event.id === 'my-tasks' ? !myTasksOpen : myTasksOpen"
              @close="navOpen = false"
            />
          </template>

          <RlPageHeader
            title="Atlas for Sroucehaven"
            status-label="On track"
            starred
            @open-nav="navOpen = true"
          >
            <template #actions>
              <RlButton variant="primary">
                Create
                <template #trailing><RlIcon name="chevron-down" :size="14" /></template>
              </RlButton>
            </template>
            <template #tabs><RlViewTabs v-model="view" :tabs="viewTabs" /></template>
            <template #tools>
              <RlButton size="sm"><template #icon><RlIcon name="search" :size="15" /></template>Search</RlButton>
              <RlButton size="sm"><template #icon><RlIcon name="filter" :size="15" /></template>Filter</RlButton>
              <RlButton size="sm"><template #icon><RlIcon name="arrow-up-down" :size="15" /></template>Sort</RlButton>
            </template>
          </RlPageHeader>

          <RlBoard :sections="boardSections" />
        </RlAppShell>
      </RlMyTasksPanels>
    `,
  }),
}

/** Screen 4: the same initiative in table view with configured columns. */
export const InitiativeTable: StoryObj = {
  render: () => ({
    components: {
      RlAppShell, RlSidebar, RlMyTasksPanels, RlPageHeader, RlViewTabs, RlTable, RlButton, RlIcon,
    },
    setup: () => ({
      navGroups, tableSections, tableColumns, viewTabs,
      view: ref('board'), navOpen: ref(false), sidebarWidth: ref(260), myTasksOpen: ref(false),
    }),
    template: `
      <RlMyTasksPanels v-model:open="myTasksOpen" :sidebar-width="sidebarWidth">
        <RlAppShell v-model:nav-open="navOpen" v-model:sidebar-width="sidebarWidth" :sidebar-default-width="260">
          <template #sidebar>
            <RlSidebar
              workspace-name="Atlas Projects"
              :groups="navGroups"
              active-id="atlas"
              :flyout-id="myTasksOpen ? 'my-tasks' : null"
              @select="myTasksOpen = $event.id === 'my-tasks' ? !myTasksOpen : myTasksOpen"
              @close="navOpen = false"
            />
          </template>

          <RlPageHeader
            title="Atlas for Sroucehaven"
            status-label="On track"
            starred
            @open-nav="navOpen = true"
          >
            <template #actions>
              <RlButton variant="primary">
                Create
                <template #trailing><RlIcon name="chevron-down" :size="14" /></template>
              </RlButton>
            </template>
            <template #tabs><RlViewTabs v-model="view" :tabs="viewTabs" /></template>
            <template #tools>
              <RlButton size="sm"><template #icon><RlIcon name="search" :size="15" /></template>Search</RlButton>
              <RlButton size="sm"><template #icon><RlIcon name="filter" :size="15" /></template>Filter</RlButton>
              <RlButton size="sm"><template #icon><RlIcon name="arrow-up-down" :size="15" /></template>Sort</RlButton>
            </template>
          </RlPageHeader>

          <RlTable :sections="tableSections" :columns="tableColumns" />
        </RlAppShell>
      </RlMyTasksPanels>
    `,
  }),
}

/** Screen 2: task detail as a full page, with subtasks. */
export const TaskDetailPage: StoryObj = {
  render: () => ({
    components: { RlAppShell, RlSidebar, RlMyTasksPanels, RlDetailPanel, RlTaskDetail, RlIconButton },
    setup: () => ({
      navGroups, detailFields, detailDescription, detailSubtasks, navOpen: ref(false), sidebarWidth: ref(260), myTasksOpen: ref(false),
    }),
    template: `
      <RlMyTasksPanels v-model:open="myTasksOpen" :sidebar-width="sidebarWidth">
        <RlAppShell v-model:nav-open="navOpen" v-model:sidebar-width="sidebarWidth" :sidebar-default-width="260">
          <template #sidebar>
            <RlSidebar
              workspace-name="Atlas Projects"
              :groups="navGroups"
              active-id="atlas"
              :flyout-id="myTasksOpen ? 'my-tasks' : null"
              @select="myTasksOpen = $event.id === 'my-tasks' ? !myTasksOpen : myTasksOpen"
              @close="navOpen = false"
            />
          </template>

          <RlDetailPanel variant="page" :show-navigation="false" :show-expand="false" show-link>
            <template #leading>
              <RlIconButton class="rl-compact-only" icon="menu" label="Open navigation" @click="navOpen = true" />
            </template>
            <div style="max-width:900px">
              <RlTaskDetail
                title="Create awesome UX for Atlas Projects"
                :fields="detailFields"
                :description="detailDescription"
                :subtasks="detailSubtasks"
                :attachments="[]"
              />
            </div>
          </RlDetailPanel>
        </RlAppShell>
      </RlMyTasksPanels>
    `,
  }),
}

/**
 * Screen 3: three panes — sidebar, task list, detail with comments.
 *
 * The header spans list and panel both, so the panel reads as a sibling of
 * the view rather than as a column of the table. Board and Table keep the
 * panel inline beside the list; Timeline declares `panelMode: 'overlay'`, so
 * the panel floats above it and the full width stays visible. The selected
 * task is marked in every view, so switching tabs keeps its origin visible.
 */
export const TaskDetailBesideList: StoryObj = {
  render: () => ({
    components: {
      RlAppShell, RlSidebar, RlMyTasksPanels, RlPageHeader, RlViewTabs, RlTable, RlBoard,
      RlDetailPanel, RlTaskDetail, RlComment, RlCommentComposer, RlStatusPill,
      RlIconButton, RlTimelinePlaceholder,
    },
    setup: () => {
      const view = ref('table')
      const panelMode = computed(
        () => panelTabs.find((t) => t.id === view.value)?.panelMode ?? 'inline',
      )
      const selectedTaskId = ref('i1')
      const panelOpen = ref(true)

      function select(task: Task) {
        selectedTaskId.value = task.id
        panelOpen.value = true
      }

      return {
        navGroups, tableSections, sharedBoardSections, detailFields, detailDescription,
        detailAttachments, detailComments, panelTabs,
        view, panelMode, selectedTaskId, panelOpen, select,
        navOpen: ref(false), sidebarWidth: ref(260), myTasksOpen: ref(false),
      }
    },
    template: `
      <RlMyTasksPanels v-model:open="myTasksOpen" :sidebar-width="sidebarWidth">
        <RlAppShell
          v-model:nav-open="navOpen"
          v-model:sidebar-width="sidebarWidth"
          :sidebar-default-width="260"
          :panel-open="panelOpen"
          :panel-mode="panelMode"
        >
          <template #sidebar>
            <RlSidebar
              workspace-name="Atlas Projects"
              :groups="navGroups"
              active-id="atlas"
              :flyout-id="myTasksOpen ? 'my-tasks' : null"
              @select="myTasksOpen = $event.id === 'my-tasks' ? !myTasksOpen : myTasksOpen"
              @close="navOpen = false"
            />
          </template>

          <template #header>
            <RlPageHeader
              title="Atlas for Sourcehaven"
              status-label="On track"
              :show-star="false"
              @open-nav="navOpen = true"
            >
              <template #tabs><RlViewTabs v-model="view" :tabs="panelTabs" :show-add="false" /></template>
            </RlPageHeader>
          </template>

          <RlTable
            v-if="view === 'table'"
            :sections="tableSections"
            :columns="[]"
            :selected-id="selectedTaskId"
            @select="select"
          />
          <RlBoard
            v-else-if="view === 'board'"
            :sections="sharedBoardSections"
            :show-add-section="false"
            :selected-id="selectedTaskId"
            @select="select"
          />
          <RlTimelinePlaceholder v-else />

          <template #panel>
            <RlDetailPanel v-if="panelOpen" show-close @close="panelOpen = false">
              <template #leading>
                <RlIconButton
                  class="rl-phone-only"
                  icon="arrow-left"
                  label="Back to list"
                  @click="panelOpen = false"
                />
              </template>
              <RlTaskDetail
                title="Create awesome UX for Atlas Projects"
                :fields="detailFields"
                :description="detailDescription"
                :attachments="detailAttachments"
              />
              <div class="rl-detail-panel-bleed" style="border-top:1px solid var(--rl-color-border); padding-block:24px">
                <RlComment v-for="c in detailComments" :key="c.id" :comment="c" />
              </div>
              <template #footer><RlCommentComposer /></template>
            </RlDetailPanel>
          </template>
        </RlAppShell>
      </RlMyTasksPanels>
    `,
  }),
}

/** Screen 5: empty meeting-notes document. */
export const MeetingNotes: StoryObj = {
  render: () => ({
    components: { RlAppShell, RlSidebar, RlMyTasksPanels, RlDocToolbar, RlDocEditor, RlIconButton },
    setup: () => ({ meetingNavGroups, navOpen: ref(false), sidebarWidth: ref(260), myTasksOpen: ref(false) }),
    template: `
      <RlMyTasksPanels v-model:open="myTasksOpen" :sidebar-width="sidebarWidth">
        <RlAppShell v-model:nav-open="navOpen" v-model:sidebar-width="sidebarWidth" :sidebar-default-width="260">
          <template #sidebar>
            <RlSidebar
              workspace-name="Atlas Projects"
              :groups="meetingNavGroups"
              active-id="mt-2"
              :flyout-id="myTasksOpen ? 'my-tasks' : null"
              @select="myTasksOpen = $event.id === 'my-tasks' ? !myTasksOpen : myTasksOpen"
              @close="navOpen = false"
            />
          </template>

          <RlDocToolbar @open-nav="navOpen = true" />
          <div style="flex:1; overflow-y:auto"><RlDocEditor /></div>
        </RlAppShell>
      </RlMyTasksPanels>
    `,
  }),
}
