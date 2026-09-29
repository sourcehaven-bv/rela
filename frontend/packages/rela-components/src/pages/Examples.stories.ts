/**
 * Worked examples: whole screens assembled from the library, to show how the
 * pieces fit together rather than what each one looks like on its own.
 *
 * Each screen is a real interaction. The form saves, the list filters, the
 * settings page raises a toast. What they do not do is talk to a server, so
 * the delays are simulated and nothing is stored.
 */
import type { Meta, StoryObj } from '@storybook/vue3-vite'
import { computed, nextTick, onUnmounted, ref } from 'vue'

import RlAppShell from '../components/layout/RlAppShell.vue'
import RlSidebar from '../components/layout/RlSidebar.vue'
import RlPageHeader from '../components/layout/RlPageHeader.vue'
import RlSectionHeading from '../components/layout/RlSectionHeading.vue'

import RlButton from '../components/common/RlButton.vue'
import RlButtonGroup from '../components/common/RlButtonGroup.vue'
import RlConfirmDialog from '../components/common/RlConfirmDialog.vue'
import RlHeading from '../components/common/RlHeading.vue'
import RlIconButton from '../components/common/RlIconButton.vue'
import RlText from '../components/common/RlText.vue'

import RlTextField from '../components/form/RlTextField.vue'
import RlTextarea from '../components/form/RlTextarea.vue'
import RlNumberField from '../components/form/RlNumberField.vue'
import RlSelect from '../components/form/RlSelect.vue'
import RlMultiSelect from '../components/form/RlMultiSelect.vue'
import RlCheckbox from '../components/form/RlCheckbox.vue'
import RlDateField from '../components/form/RlDateField.vue'
import RlFileField from '../components/form/RlFileField.vue'

import RlModal from '../components/overlay/RlModal.vue'
import RlDrawer from '../components/overlay/RlDrawer.vue'
import RlMenu from '../components/overlay/RlMenu.vue'
import RlMenuItem from '../components/overlay/RlMenuItem.vue'
import RlMenuSeparator from '../components/overlay/RlMenuSeparator.vue'
import RlTooltip from '../components/overlay/RlTooltip.vue'
import RlPopover from '../components/overlay/RlPopover.vue'

import RlBanner from '../components/feedback/RlBanner.vue'
import RlCallout from '../components/feedback/RlCallout.vue'
import RlToastHost from '../components/feedback/RlToastHost.vue'
import RlEmptyState from '../components/feedback/RlEmptyState.vue'
import RlSkeleton from '../components/feedback/RlSkeleton.vue'
import RlProgressBar from '../components/feedback/RlProgressBar.vue'
import RlActivityBar from '../components/feedback/RlActivityBar.vue'
import RlAutoSaveIndicator from '../components/feedback/RlAutoSaveIndicator.vue'
import { useToasts } from '../components/feedback/useToasts'

import RlSearchBox from '../components/data/RlSearchBox.vue'
import RlFilterChip from '../components/data/RlFilterChip.vue'
import RlPagination from '../components/data/RlPagination.vue'
import RlAvatar from '../components/data/RlAvatar.vue'
import RlAvatarGroup from '../components/data/RlAvatarGroup.vue'
import RlAccordion from '../components/data/RlAccordion.vue'
import RlSegmentedControl from '../components/data/RlSegmentedControl.vue'
import RlKbd from '../components/data/RlKbd.vue'

import RlInlineEdit from '../components/common/RlInlineEdit.vue'
import RlOptionSelect from '../components/form/RlOptionSelect.vue'
import RlMarkdownEditor from '../components/editor/RlMarkdownEditor.vue'
import RlCommentIndicator from '../components/comment/RlCommentIndicator.vue'
import type { AnchoredComment } from '../components/comment/types'
import RlStatusDot from '../components/common/RlStatusDot.vue'
import RlTable from '../fixtures/RlTaskTable.vue'
import type { TableColumn } from '../components/table/types'
import RlTaskDetail from '../components/task/RlTaskDetail.vue'
import RlDetailPanel from '../components/task/RlDetailPanel.vue'

import RlBoard from '../fixtures/RlTaskBoard.vue'
import RlMyTasksPanels from '../fixtures/RlMyTasksPanels.vue'
import RlViewTabs from '../components/layout/RlViewTabs.vue'
import RlIcon from '../components/common/RlIcon.vue'
import RlComment from '../components/task/RlComment.vue'
import RlCommentComposer from '../components/task/RlCommentComposer.vue'

import { navGroups, statusOptions, detailFields, boardSections } from '../fixtures'
import {
  bulkAttachments,
  bulkBoardSections,
  bulkComments,
  bulkDescription,
  bulkDetailFields,
  bulkDetailTitle,
  bulkNavGroups,
  bulkSubtasks,
  crowdedNavGroups,
  bulkTableColumns,
  bulkTableSections,
} from '../fixtures/bulk'
import type { DetailField, Task } from '../types'

const meta: Meta = {
  title: 'Examples',
  parameters: { layout: 'fullscreen' },
}
export default meta

const statuses = [
  { value: 'todo', label: 'To do' },
  { value: 'doing', label: 'In progress' },
  { value: 'review', label: 'In review' },
  { value: 'done', label: 'Done' },
]

const labelOptions = [
  { value: 'design', label: 'Design' },
  { value: 'frontend', label: 'Frontend' },
  { value: 'backend', label: 'Backend' },
  { value: 'research', label: 'Research' },
]

/**
 * A create form: every input type, validated on submit, with the three
 * pending indicators in their proper places.
 *
 * The submit button swaps its own label while it works, which is the
 * explicit-action indicator. The draft above it saves in the background,
 * which is the ambient one. Neither is a page-level spinner.
 */
export const TaskForm: StoryObj = {
  render: () => ({
    components: {
      RlAppShell, RlSidebar, RlMyTasksPanels, RlPageHeader, RlSectionHeading,
      RlButton, RlButtonGroup, RlText, RlHeading,
      RlTextField, RlTextarea, RlNumberField, RlSelect, RlMultiSelect,
      RlCheckbox, RlDateField, RlFileField,
      RlBanner, RlCallout, RlToastHost, RlAutoSaveIndicator,
    },
    setup() {
      const { toasts, dismiss, success } = useToasts()

      const form = ref({
        name: '',
        description: '',
        status: 'todo',
        labels: [] as string[],
        due: '',
        estimate: undefined as number | undefined,
        notify: true,
        files: [] as File[],
      })

      const errors = ref<Record<string, string>>({})
      const submitting = ref(false)
      const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
      let draftTimer: ReturnType<typeof setTimeout> | undefined

      // The draft saves itself as the user types, quietly.
      function touch() {
        saveState.value = 'saving'
        clearTimeout(draftTimer)
        draftTimer = setTimeout(() => (saveState.value = 'saved'), 900)
      }

      function submit() {
        const found: Record<string, string> = {}
        if (!form.value.name.trim()) found.name = 'Give the task a name.'
        if (!form.value.due) found.due = 'Pick a due date.'
        errors.value = found
        if (Object.keys(found).length) return

        submitting.value = true
        setTimeout(() => {
          submitting.value = false
          success('Task created', `${form.value.name} is on the board.`)
        }, 1200)
      }

      return { navGroups, statuses, labelOptions, form, errors, submitting, saveState, touch, submit, toasts, dismiss, navOpen: ref(false), sidebarWidth: ref(260), myTasksOpen: ref(false) }
    },
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

          <RlPageHeader title="New task" @open-nav="navOpen = true" />

          <div style="max-width:640px; padding:24px var(--rl-page-gutter) 64px">
            <RlBanner tone="info" title="Draft" style="margin-bottom:24px">
              This task is a draft until you create it. Nobody else can see it yet.
            </RlBanner>

            <!--
              novalidate: the fields keep their required attribute so screen
              readers announce them, but the browser's own bubble is suppressed.
              That bubble shows one field at a time, is not styled with the page
              and vanishes on the next click, so the form validates itself and
              reports every problem at once in the fields.
            -->
            <form novalidate @submit.prevent="submit" style="display:flex; flex-direction:column; gap:20px">
              <div v-if="saveState !== 'idle'" style="display:flex; justify-content:flex-end">
                <RlAutoSaveIndicator :state="saveState" />
              </div>

              <RlTextField
                v-model="form.name"
                label="Task name"
                placeholder="Name this task"
                required
                :error="errors.name"
                hint="Shown on the board and in search results."
                @update:model-value="touch"
              />

              <RlTextarea
                v-model="form.description"
                label="Description"
                placeholder="What needs to happen?"
                auto-grow
                :rows="3"
                :maxlength="500"
                @update:model-value="touch"
              />

              <div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(200px, 1fr)); gap:20px">
                <RlSelect v-model="form.status" label="Status" :options="statuses" @update:model-value="touch" />
                <RlDateField v-model="form.due" label="Due date" required :error="errors.due" @update:model-value="touch" />
              </div>

              <div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(200px, 1fr)); gap:20px">
                <RlMultiSelect v-model="form.labels" label="Labels" :options="labelOptions" @update:model-value="touch" />
                <RlNumberField v-model="form.estimate" label="Estimate" suffix="hours" :min="0" :step="0.5" @update:model-value="touch" />
              </div>

              <RlFileField v-model="form.files" label="Attachments" multiple hint="PDF, PNG or JPG, up to 10 MB each." />

              <RlCheckbox v-model="form.notify" label="Notify the assignee" hint="They get an email as soon as this is created." />

              <RlCallout tone="info" title="Tip">
                Subtasks roll up to their parent, so closing every subtask closes the parent too.
              </RlCallout>

              <RlButtonGroup>
                <RlButton variant="secondary" type="button">Cancel</RlButton>
                <RlButton
                  variant="primary"
                  type="submit"
                  :loading="submitting"
                  pending-label="Creating"
                  loading-label="Creating the task"
                >
                  Create task
                </RlButton>
              </RlButtonGroup>
            </form>
          </div>

          <RlToastHost :toasts="toasts" @dismiss="dismiss" />
        </RlAppShell>
      </RlMyTasksPanels>
    `,
  }),
}

/**
 * A list screen: search, filters, pagination, an overflow menu with a guarded
 * delete, and the loading and empty states the same list has to handle.
 *
 * Search for something that matches nothing to see the empty state, and use
 * the "..." on a row to reach the confirmation dialog.
 */
export const TaskList: StoryObj = {
  render: () => ({
    components: {
      RlAppShell, RlSidebar, RlPageHeader,
      RlButton, RlIconButton, RlText, RlConfirmDialog,
      RlSearchBox, RlFilterChip, RlPagination, RlAvatarGroup, RlSegmentedControl,
      RlMenu, RlMenuItem, RlMenuSeparator, RlTooltip,
      RlEmptyState, RlSkeleton, RlProgressBar, RlToastHost, RlActivityBar,
    },
    setup() {
      const { toasts, dismiss, success } = useToasts()

      const allTasks = ref([
        { id: 1, name: 'Atlas mobile redesign', status: 'In progress', done: 5, total: 8, people: [{ name: 'Hanry Fonda' }, { name: 'Ada Lovelace' }] },
        { id: 2, name: 'Payment provider migration', status: 'Blocked', done: 3, total: 12, people: [{ name: 'Grace Hopper' }] },
        { id: 3, name: 'Onboarding survey', status: 'Done', done: 6, total: 6, people: [{ name: 'Alan Turing' }, { name: 'Katherine Johnson' }, { name: 'Ada Lovelace' }] },
        { id: 4, name: 'Design token audit', status: 'To do', done: 0, total: 4, people: [{ name: 'Hanry Fonda' }] },
        { id: 5, name: 'Mobile navigation spike', status: 'In progress', done: 2, total: 5, people: [{ name: 'Grace Hopper' }, { name: 'Alan Turing' }] },
      ])

      const query = ref('')
      const loading = ref(false)
      const navigating = ref(false)
      const page = ref(1)
      const density = ref('comfortable')
      const pendingDelete = ref<{ id: number; name: string } | null>(null)

      const filters = ref([
        { id: 'status', label: 'Status', value: 'Open', icon: 'done' as const },
        { id: 'assignee', label: 'Assignee', value: 'Hanry Fonda', icon: 'user' as const },
      ])

      const matches = computed(() =>
        allTasks.value.filter((task) => task.name.toLowerCase().includes(query.value.toLowerCase())),
      )

      // A search that takes long enough to be worth showing a skeleton for.
      function onSearch() {
        loading.value = true
        setTimeout(() => (loading.value = false), 500)
      }

      function confirmDelete() {
        const target = pendingDelete.value
        if (!target) return
        allTasks.value = allTasks.value.filter((task) => task.id !== target.id)
        pendingDelete.value = null
        success('Task deleted', `${target.name} was removed.`)
      }

      return {
        navGroups, query, matches, loading, navigating, page, density, filters,
        pendingDelete, onSearch, confirmDelete, toasts, dismiss, navOpen: ref(false), sidebarWidth: ref(260),
        densityOptions: [
          { value: 'compact', label: 'Compact' },
          { value: 'comfortable', label: 'Comfortable' },
        ],
      }
    },
    template: `
      <RlAppShell v-model:nav-open="navOpen" v-model:sidebar-width="sidebarWidth" :sidebar-default-width="260">
        <template #sidebar>
          <RlSidebar workspace-name="Atlas Projects" :groups="navGroups" active-id="my-tasks" @close="navOpen = false" />
        </template>

        <RlActivityBar :active="navigating" />

        <RlPageHeader title="My tasks" @open-nav="navOpen = true">
          <template #actions>
            <RlButton variant="primary" icon="plus">New task</RlButton>
          </template>
        </RlPageHeader>

        <div style="padding:16px var(--rl-page-gutter) 48px">
          <div style="display:flex; flex-wrap:wrap; align-items:center; gap:12px; margin-bottom:12px">
            <RlSearchBox
              v-model="query"
              placeholder="Search tasks"
              :result-text="matches.length + ' results'"
              style="flex:1; min-width:200px; max-width:320px"
              @search="onSearch"
            />
            <RlSegmentedControl v-model="density" label="Row density" :options="densityOptions" size="sm" />
            <RlTooltip text="Keyboard shortcuts">
              <template #default="{ describedBy }">
                <RlIconButton icon="help" label="Keyboard shortcuts" :aria-describedby="describedBy" />
              </template>
            </RlTooltip>
          </div>

          <div v-if="filters.length" style="display:flex; flex-wrap:wrap; align-items:center; gap:8px; margin-bottom:16px">
            <RlFilterChip
              v-for="filter in filters"
              :key="filter.id"
              :label="filter.label"
              :value="filter.value"
              :icon="filter.icon"
              @remove="filters = filters.filter((entry) => entry.id !== filter.id)"
            />
            <RlButton size="sm" variant="ghost" @click="filters = []">Clear all</RlButton>
          </div>

          <!-- Loading: the shape of the rows that are coming, not a spinner. -->
          <div v-if="loading" aria-busy="true" style="display:flex; flex-direction:column; gap:16px">
            <div v-for="row in 4" :key="row" style="display:flex; align-items:center; gap:16px; padding:12px 0">
              <div style="flex:1"><RlSkeleton variant="text" width="45%" /></div>
              <RlSkeleton variant="text" width="80px" />
              <RlSkeleton variant="circle" />
            </div>
          </div>

          <RlEmptyState
            v-else-if="!matches.length"
            icon="search"
            title="No tasks match these filters"
            description="Try removing a filter or searching for something broader."
          >
            <template #actions>
              <RlButton variant="secondary" @click="query = ''; filters = []">Clear filters</RlButton>
            </template>
          </RlEmptyState>

          <ul v-else style="margin:0; padding:0; list-style:none">
            <li
              v-for="task in matches"
              :key="task.id"
              style="display:flex; align-items:center; gap:16px; padding:12px 0; border-bottom:1px solid var(--rl-color-border)"
            >
              <div style="flex:1; min-width:0">
                <RlText size="md" weight="medium" truncate>{{ task.name }}</RlText>
                <RlProgressBar
                  :segments="[
                    { label: 'done', value: task.done, tone: 'success' },
                    { label: 'remaining', value: task.total - task.done, tone: 'neutral' },
                  ]"
                  :aria-label="task.name + ' progress'"
                  size="sm"
                  style="max-width:160px; margin-top:6px"
                />
              </div>

              <RlText size="sm" tone="muted">{{ task.status }}</RlText>
              <RlAvatarGroup :people="task.people" :max="2" size="sm" />

              <RlMenu>
                <template #trigger="{ toggle, attrs }">
                  <RlIconButton icon="ellipsis" :label="'Options for ' + task.name" v-bind="attrs" @click="toggle" />
                </template>
                <RlMenuItem icon="link" shortcut="C">Copy link</RlMenuItem>
                <RlMenuItem icon="file">Duplicate</RlMenuItem>
                <RlMenuSeparator />
                <RlMenuItem icon="trash-2" tone="danger" @click="pendingDelete = task">Delete task</RlMenuItem>
              </RlMenu>
            </li>
          </ul>

          <RlPagination
            v-if="matches.length && !loading"
            v-model:page="page"
            :page-count="8"
            :summary="matches.length + ' of 189'"
            style="margin-top:20px"
          />
        </div>

        <RlConfirmDialog
          :open="Boolean(pendingDelete)"
          title="Delete this task?"
          :description="pendingDelete ? pendingDelete.name + ' and its subtasks will be removed. This cannot be undone.' : ''"
          @cancel="pendingDelete = null"
          @confirm="confirmDelete"
        />

        <RlToastHost :toasts="toasts" @dismiss="dismiss" />
      </RlAppShell>
    `,
  }),
}

/**
 * The "My tasks" slide-out, which every screen with that nav item offers:
 * clicking it slides a list out over the page, and clicking a row in the
 * list slides the task out beside it. Closing either uncovers what was under
 * it, and the page stays usable throughout.
 *
 * This story is the reference case, shown against a board because a board is
 * the busiest thing the panels have to sit over. The panels are not dimming
 * or blocking it, because a list of today's tasks is something you read
 * against the work rather than instead of it. Escape closes one panel, so a
 * two-panel stack takes two presses rather than collapsing at once.
 *
 * The third panel would be one more entry in the array, not a new component.
 */
export const MyTasksFromTheNav: StoryObj = {
  render: () => ({
    components: {
      RlAppShell, RlSidebar, RlPageHeader, RlBoard, RlMyTasksPanels, RlButton,
    },
    setup: () => ({
      navGroups, boardSections, navOpen: ref(false), sidebarWidth: ref(260), myTasksOpen: ref(false),
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

          <RlPageHeader title="Atlas for Sourcehaven" status-label="On track" @open-nav="navOpen = true">
            <template #actions>
              <RlButton variant="primary" icon="plus">New task</RlButton>
            </template>
          </RlPageHeader>

          <RlBoard :sections="boardSections" />
        </RlAppShell>
      </RlMyTasksPanels>
    `,
  }),
}


/**
 * A settings screen: grouped sections in accordions, a drawer for a secondary
 * form, a modal, and a destructive action guarded by a confirmation.
 *
 * It also shows the shortcut sheet, which is the one place a keyboard key
 * belongs in a page rather than a menu.
 */
export const Settings: StoryObj = {
  render: () => ({
    components: {
      RlAppShell, RlSidebar, RlMyTasksPanels, RlPageHeader, RlSectionHeading,
      RlButton, RlButtonGroup, RlText, RlHeading, RlIconButton, RlConfirmDialog,
      RlTextField, RlSelect, RlCheckbox,
      RlModal, RlDrawer, RlMenu, RlMenuItem, RlMenuSeparator,
      RlAccordion, RlAvatar, RlKbd, RlBanner, RlCallout,
      RlToastHost,
    },
    setup() {
      const { toasts, dismiss, success } = useToasts()

      const name = ref('Atlas for Sourcehaven')
      const visibility = ref('team')
      const weeklyDigest = ref(true)
      const saving = ref(false)

      const shortcutsOpen = ref(false)
      const memberDrawerOpen = ref(false)
      const deleteOpen = ref(false)
      const deleting = ref(false)
      const inviteEmail = ref('')

      function save() {
        saving.value = true
        setTimeout(() => {
          saving.value = false
          success('Settings saved')
        }, 900)
      }

      function invite() {
        memberDrawerOpen.value = false
        success('Invitation sent', inviteEmail.value + ' will get an email.')
        inviteEmail.value = ''
      }

      // A destructive action that is pending keeps the dialog open and locked.
      function destroy() {
        deleting.value = true
        setTimeout(() => {
          deleting.value = false
          deleteOpen.value = false
          success('Project deleted')
        }, 1200)
      }

      return {
        navGroups, name, visibility, weeklyDigest, saving, save,
        shortcutsOpen, memberDrawerOpen, deleteOpen, deleting, destroy,
        inviteEmail, invite, toasts, dismiss, navOpen: ref(false), sidebarWidth: ref(260), myTasksOpen: ref(false),
        visibilityOptions: [
          { value: 'private', label: 'Private' },
          { value: 'team', label: 'Everyone in the team' },
          { value: 'public', label: 'Anyone with the link' },
        ],
        members: [
          { name: 'Hanry Fonda', role: 'Owner' },
          { name: 'Ada Lovelace', role: 'Editor' },
          { name: 'Grace Hopper', role: 'Viewer' },
        ],
        shortcuts: [
          { action: 'Open the command palette', keys: 'Ctrl+K', separator: '+' },
          { action: 'Focus search', keys: '/', separator: '+' },
          { action: 'Go to dashboard', keys: 'G then D', separator: 'then' },
          { action: 'Next task', keys: 'J or Down', separator: 'or' },
        ],
      }
    },
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

          <RlPageHeader title="Project settings" @open-nav="navOpen = true">
            <template #actions>
              <RlButton variant="secondary" @click="shortcutsOpen = true">Shortcuts</RlButton>
              <RlMenu>
                <template #trigger="{ toggle, attrs }">
                  <RlIconButton icon="ellipsis" label="More options" v-bind="attrs" @click="toggle" />
                </template>
                <RlMenuItem icon="link">Copy project link</RlMenuItem>
                <RlMenuItem icon="file">Export as CSV</RlMenuItem>
                <RlMenuSeparator />
                <RlMenuItem icon="trash-2" tone="danger" @click="deleteOpen = true">Delete project</RlMenuItem>
              </RlMenu>
            </template>
          </RlPageHeader>

          <div style="max-width:720px; padding:24px var(--rl-page-gutter) 64px">
            <RlAccordion title="General" :open="true" :level="2">
              <div style="display:flex; flex-direction:column; gap:20px; padding-top:8px">
                <RlTextField v-model="name" label="Project name" required />
                <RlSelect v-model="visibility" label="Visibility" :options="visibilityOptions"
                  hint="Controls who can open this project without an invitation." />
                <RlCheckbox v-model="weeklyDigest" label="Send a weekly digest" hint="Every Monday at 09:00." />
                <RlButtonGroup align="start">
                  <RlButton variant="primary" :loading="saving" pending-label="Saving" @click="save">
                    Save changes
                  </RlButton>
                </RlButtonGroup>
              </div>
            </RlAccordion>

            <RlAccordion title="Members" :count="3" :level="2">
              <div style="padding-top:8px">
                <ul style="margin:0 0 16px; padding:0; list-style:none; display:flex; flex-direction:column; gap:12px">
                  <li v-for="member in members" :key="member.name" style="display:flex; align-items:center; gap:12px">
                    <RlAvatar :name="member.name" size="sm" decorative />
                    <RlText size="md" style="flex:1">{{ member.name }}</RlText>
                    <RlText size="sm" tone="muted">{{ member.role }}</RlText>
                  </li>
                </ul>
                <RlButton variant="secondary" icon="plus" @click="memberDrawerOpen = true">Invite someone</RlButton>
              </div>
            </RlAccordion>

            <RlAccordion title="Danger zone" :level="2">
              <div style="padding-top:8px; display:flex; flex-direction:column; gap:16px">
                <RlCallout tone="danger" title="Cannot be undone">
                  Deleting this project deletes its tasks, comments and attachments.
                </RlCallout>
                <RlButtonGroup align="start">
                  <RlButton variant="secondary" tone="danger" icon="trash-2" @click="deleteOpen = true">
                    Delete this project
                  </RlButton>
                </RlButtonGroup>
              </div>
            </RlAccordion>
          </div>

          <!-- A modal for reference material the user reads and dismisses. -->
          <RlModal title="Keyboard shortcuts" :open="shortcutsOpen" @close="shortcutsOpen = false">
            <dl style="display:grid; grid-template-columns:1fr auto; gap:12px 24px; margin:0">
              <template v-for="shortcut in shortcuts" :key="shortcut.action">
                <dt style="font-size:14px">{{ shortcut.action }}</dt>
                <dd style="margin:0"><RlKbd :keys="shortcut.keys" :separator="shortcut.separator" /></dd>
              </template>
            </dl>
          </RlModal>

          <!-- A drawer for a secondary form, so the settings stay visible behind it. -->
          <RlDrawer title="Invite someone" size="sm" :open="memberDrawerOpen" @close="memberDrawerOpen = false">
            <RlTextField v-model="inviteEmail" label="Email address" type="email" placeholder="name@example.com" required />
            <template #actions>
              <RlButtonGroup>
                <RlButton variant="secondary" @click="memberDrawerOpen = false">Cancel</RlButton>
                <RlButton variant="primary" @click="invite">Send invitation</RlButton>
              </RlButtonGroup>
            </template>
          </RlDrawer>

          <RlConfirmDialog
            :open="deleteOpen"
            :confirming="deleting"
            title="Delete this project?"
            description="Atlas for Sourcehaven, its 189 tasks and every comment on them will be removed. This cannot be undone."
            confirm-label="Delete project"
            @cancel="deleteOpen = false"
            @confirm="destroy"
          />

          <RlToastHost :toasts="toasts" @dismiss="dismiss" />
        </RlAppShell>
      </RlMyTasksPanels>
    `,
  }),
}


/**
 * Inline editing: a screen where the values themselves are the controls.
 *
 * The same component does all three of these. A status in the detail panel, a
 * status in a table cell and the task's own title and description differ only
 * in what they show when idle and what control they swap in. None of them is
 * a special case of the others.
 *
 * The rule the screen follows is that an edit is kept when the user moves on,
 * by pressing Enter or clicking elsewhere, and dropped on Escape. A status
 * closes the moment a choice is made, because with a fixed list of five there
 * is nothing left to confirm.
 */
export const InlineEditing: StoryObj = {
  render: () => ({
    components: {
      RlAppShell, RlSidebar, RlPageHeader, RlDetailPanel, RlTaskDetail, RlTable,
      RlInlineEdit, RlStatusDot, RlHeading, RlText, RlToastHost,
      RlOptionSelect, RlMarkdownEditor, RlAutoSaveIndicator, RlCommentIndicator,
    },
    setup() {
      const { toasts, dismiss, success } = useToasts()

      // The detail panel's own fields, edited through RlTaskDetail.
      const fields = ref<DetailField[]>(detailFields.map((field) => ({ ...field })))

      function updateField(next: DetailField) {
        const index = fields.value.findIndex((field) => field.id === next.id)
        if (index === -1) return
        fields.value[index] = next
        success(`${next.label} set to ${next.value ?? next.tags?.[0]?.label}`)
      }

      const priorityOptions = [
        { id: 'low', label: 'Low', color: 'grey' as const },
        { id: 'medium', label: 'Medium', color: 'blue' as const },
        { id: 'high', label: 'High', color: 'green' as const },
        { id: 'urgent', label: 'Urgent', color: 'red' as const },
      ]

      /*
       * A comment marker per field, beside the label rather than the value: a
       * field is a column on a grid, and the end of a value moves with the
       * value's length. Only fields with a thread are shown here with one,
       * since the marker is quiet until the row is hovered anyway.
       */
      const fieldAnchor = (field: DetailField) => ({
        kind: 'property' as const,
        ref: field.id,
        label: field.label,
      })

      const fieldComments: Record<string, AnchoredComment[]> = {}
      const assignee = fields.value.find((field) => field.type === 'text')
      if (assignee) {
        fieldComments[assignee.id] = [
          {
            id: 'f1',
            author: 'Rowdy van Looy',
            timestamp: '22 Sep, 09:40',
            body: 'Should this be the whole team rather than one person?',
            anchor: fieldAnchor(assignee),
          },
        ]
      }

      // The task body. The title and the description are values too, so they
      // edit in place rather than behind an "edit task" mode. The description
      // is prose, so it gets the markdown editor rather than a textarea.
      const title = ref('Create awesome UX for Atlas Projects')
      const description = ref(
        [
          'Atlas needs a **detail view** that reads as a document, not a form.',
          '',
          '- Every value edits where it sits',
          '- Nothing opens a modal to change one field',
          '',
          'See the [design notes](https://example.com) for the full brief.',
        ].join('\n'),
      )
      const titleDraft = ref(title.value)
      const descriptionDraft = ref(description.value)

      const descriptionAnchor = {
        kind: 'property' as const,
        ref: 'description',
        label: 'Description',
      }

      const descriptionComments = [
        {
          id: 'd1',
          author: 'Rowdy van Looy',
          timestamp: '22 Sep, 11:02',
          body: 'Can we name the two flows this covers?',
          anchor: descriptionAnchor,
        },
        {
          id: 'd2',
          author: 'Hanna de Vries',
          timestamp: '22 Sep, 11:20',
          body: 'Added them as bullets.',
          anchor: descriptionAnchor,
        },
      ]

      /*
       * The description saves on change, debounced, so the swap carries no
       * commit semantics: there is no blur to interpret, and no need to treat
       * the toolbar or a comment popover as "still editing". Leaving simply
       * goes back to the read view, and what is on screen is already saved.
       */
      function onDescriptionInput(value: string) {
        descriptionDraft.value = value
        description.value = value
        queueSave()
      }

      /*
       * Autosave, debounced so a burst of typing is one save rather than one
       * per keystroke. A real app would send the request here; this only
       * moves the indicator through its states.
       */
      const saveState = ref<'idle' | 'saving' | 'saved' | 'error'>('idle')
      let saveTimer: ReturnType<typeof setTimeout> | undefined
      let settleTimer: ReturnType<typeof setTimeout> | undefined

      function queueSave() {
        clearTimeout(saveTimer)
        clearTimeout(settleTimer)
        saveState.value = 'saving'
        saveTimer = setTimeout(() => {
          saveState.value = 'saved'
          // Back to idle, because a permanent "Saved" stops being read.
          settleTimer = setTimeout(() => (saveState.value = 'idle'), 2000)
        }, 700)
      }

      onUnmounted(() => {
        clearTimeout(saveTimer)
        clearTimeout(settleTimer)
      })

      /*
       * The list. Each row carries its own status, so the table needs a column
       * for it and a cell slot that renders the picker instead of plain text.
       */
      const columns: TableColumn[] = [
        { key: 'status', header: 'Status', field: 'status', width: 190 },
        { key: 'assignee', header: 'Assignee', field: 'assignee', width: 110 },
      ]

      const rows = ref<Task[]>([
        { id: 'r1', title: 'Draft the onboarding flow', status: 'In progress', assignee: 'Rowdy', dueDate: '18 sep' },
        { id: 'r2', title: 'Review the colour tokens', status: 'At risk', assignee: 'Hanna', dueDate: '22 sep' },
        { id: 'r3', title: 'Write the migration notes', status: 'Not started', assignee: 'Alex', dueDate: '2 oct' },
        { id: 'r4', title: 'Ship the dark theme', status: 'Done', assignee: 'Jeroen', dueDate: '9 sep' },
      ])

      const sections = computed(() => [
        { id: 'open', title: 'This week', color: 'green' as const, count: rows.value.length, items: rows.value },
      ])

      function colorOf(status?: string) {
        return statusOptions.find((option) => option.value === status)?.status ?? 'grey'
      }

      function setRowStatus(task: Task, status: string) {
        const row = rows.value.find((entry) => entry.id === task.id)
        if (!row || row.status === status) return
        row.status = status
        success(`${row.title} is now ${row.status}`)
      }

      /*
       * A task only moves forward, so the states behind the one it is in are
       * shown greyed rather than dropped: a list that silently loses a choice
       * leaves the user hunting for it. This stands in for the transition
       * rules and the permission checks a real app would ask.
       */
      const statusOrder = statusOptions.map((option) => option.value)

      function isStatusClosed(task: Task, status: string) {
        return statusOrder.indexOf(status) < statusOrder.indexOf(task.status ?? '')
      }

      return {
        navGroups, toasts, dismiss, fields, updateField, statusOptions, priorityOptions,
        title, description, descriptionDraft, titleDraft, saveState, queueSave, onDescriptionInput,
        descriptionAnchor, descriptionComments, fieldAnchor, fieldComments,
        columns, sections, colorOf, setRowStatus, isStatusClosed,
      }
    },
    template: `
      <RlAppShell>
        <template #sidebar><RlSidebar :groups="navGroups" active-id="projects" /></template>

        <RlPageHeader title="Atlas Projects" subtitle="Click any value to change it" />

        <div style="display:flex; min-height:0; flex:1">
          <div style="flex:1; min-width:0; display:flex; flex-direction:column">
            <!--
              A status in a table cell. The cell slot replaces the plain text
              the table would otherwise print for the field.
            -->
            <RlTable :sections="sections" :columns="columns">
              <template #cell-status="{ item: task }">
                <RlOptionSelect
                  variant="inline"
                  :model-value="task.status"
                  :options="statusOptions.map(o => o.value)"
                  :is-option-disabled="status => isStatusClosed(task, status)"
                  :label="'Status of ' + task.title"
                  @update:model-value="setRowStatus(task, $event)"
                >
                  <template #option="{ value }">
                    <RlStatusDot :color="colorOf(value)" />
                    <span style="white-space:nowrap">{{ value }}</span>
                  </template>
                </RlOptionSelect>
              </template>
            </RlTable>
          </div>

          <RlDetailPanel :show-navigation="false" :show-attach="false">
            <RlTaskDetail
              :title="title"
              :fields="fields"
              editable-fields
              :status-options="statusOptions"
              :tag-options="priorityOptions"
              :show-empty-fields-toggle="false"
              @update-field="updateField"
            >
              <!-- A comment marker per field, beside the label. -->
              <template #field-label-trailing="{ field }">
                <RlCommentIndicator
                  :anchor="fieldAnchor(field)"
                  :comments="fieldComments[field.id] ?? []"
                />
              </template>
              <!-- The title is a value too, so it edits where it sits. -->
              <template #title>
                <RlInlineEdit
                  label="Title"
                  class="rl-task-detail__title"
                  @edit="titleDraft = title"
                  @commit="title = titleDraft"
                >
                  <template #read>
                    <RlHeading :level="1" size="2xl" weight="normal">{{ title }}</RlHeading>
                  </template>
                  <template #edit>
                    <input v-model="titleDraft" type="text" class="rl-control" aria-label="Title" style="min-width:320px" />
                  </template>
                </RlInlineEdit>
              </template>

              <!--
                The body. A real swap, because the two states differ in what
                they contain and not only in their chrome: reading shows the
                comment threads anchored to the prose, editing hides them,
                since a marker anchored to text being rewritten has nothing
                stable to hold on to. A class toggle cannot express that.

                The swap carries no commit semantics, though. Saving happens
                on change with a debounce, so there is no blur to interpret
                and no need to treat the toolbar or a comment popover as
                "still editing". Leaving returns to the read view, and what
                is on screen is already saved.
              -->
              <template #default>
                <section style="margin-top:24px">
                  <div style="display:flex; align-items:center; gap:12px; margin-bottom:12px">
                    <RlHeading :level="2" size="md" weight="normal" tone="muted">
                      Description
                    </RlHeading>
                    <RlAutoSaveIndicator :state="saveState" />
                  </div>

                  <RlInlineEdit
                    block
                    label="Description"
                    placeholder="Add a description"
                    :empty="!description"
                    @edit="descriptionDraft = description"
                  >
                    <template #read>
                      <div style="width:100%">
                        <RlMarkdownEditor :key="description" :model-value="description" readonly />
                        <!-- Anchored to the prose, so it belongs to the read
                             view and disappears while the text is in flux. -->
                        <div style="display:flex; align-items:center; gap:8px; margin-top:8px">
                          <RlCommentIndicator
                            :anchor="descriptionAnchor"
                            :comments="descriptionComments"
                          />
                          <RlText size="sm" tone="subtle">
                            {{ descriptionComments.length }} comments on this description
                          </RlText>
                        </div>
                      </div>
                    </template>

                    <template #edit>
                      <div style="width:100%" @keydown.enter.stop>
                        <RlMarkdownEditor
                          :model-value="descriptionDraft"
                          @update:model-value="onDescriptionInput"
                        />
                      </div>
                    </template>
                  </RlInlineEdit>
                </section>
              </template>
            </RlTaskDetail>
          </RlDetailPanel>
        </div>

        <RlToastHost :toasts="toasts" @dismiss="dismiss" />
      </RlAppShell>
    `,
  }),
}

/*
 * The three screens below are the Pages compositions at the size a real
 * workspace reaches. Nothing about them is new; only the data is larger.
 *
 * They exist because a screen that reads well with four tasks can fail with
 * four hundred, and the failures are not subtle: a column that scrolls the
 * page instead of itself, a title that pushes its row's meta off the edge, a
 * sticky header that stops being sticky, a comment thread that scrolls the
 * composer out of reach. These are the fixtures to check that against.
 */

const scaleViewTabs = [
  { id: 'board', label: 'Board', icon: 'kanban' as const },
  { id: 'table', label: 'Table', icon: 'table' as const },
  { id: 'timeline', label: 'Timeline', icon: 'chart-no-axes-gantt' as const },
]

/**
 * The initiative table with 131 open tasks across six sections, plus two
 * collapsed ones standing in for the quarters already closed.
 *
 * What to look at: the section headers while scrolling, rows whose title runs
 * past its column, and the sidebar, which has enough initiatives to scroll
 * independently of the list.
 *
 * The three tools are live. Search reveals a field and narrows the list as
 * you type, showing what a table this size does when a query matches two rows
 * and when it matches none. Filter and Sort open panels: filters land as
 * removable chips above the table, and the sort is applied within each
 * section rather than across them, since the sections are the grouping.
 */
export const InitiativeTableAtScale: StoryObj = {
  render: () => ({
    components: {
      RlAppShell, RlSidebar, RlMyTasksPanels, RlPageHeader, RlViewTabs, RlTable, RlButton, RlIcon,
      RlPopover, RlSearchBox, RlFilterChip, RlCheckbox, RlSegmentedControl,
      RlEmptyState,
    },
    setup() {
      const view = ref('table')
      const navOpen = ref(false)

      /*
       * Search is a disclosure rather than a permanent field. The bar already
       * carries the view tabs, and a table is read far more often than it is
       * searched, so the field is summoned and then given focus.
       */
      const searchOpen = ref(false)
      const query = ref('')
      const searchBox = ref<InstanceType<typeof RlSearchBox> | null>(null)

      async function toggleSearch() {
        searchOpen.value = !searchOpen.value
        if (searchOpen.value) {
          await nextTick()
          searchBox.value?.focus()
        } else {
          // Closing the field clears it, or the list stays narrowed by
          // something the user can no longer see.
          query.value = ''
        }
      }

      /* Which statuses are in play. Every box ticked is the same as none. */
      const statuses = bulkTableSections.map((section) => section.title)
      const statusFilter = ref<string[]>([])

      function toggleStatus(title: string, on: boolean) {
        statusFilter.value = on
          ? [...statusFilter.value, title]
          : statusFilter.value.filter((entry) => entry !== title)
      }

      const assigneeOnly = ref(false)

      /* The chips are the filter state read back, so removing one clears it. */
      const chips = computed(() => [
        ...statusFilter.value.map((title) => ({
          id: `status:${title}`,
          label: 'Status',
          value: title,
          icon: 'done' as const,
        })),
        ...(assigneeOnly.value
          ? [{ id: 'mine', label: 'Assignee', value: 'Me', icon: 'user' as const }]
          : []),
      ])

      function removeChip(id: string) {
        if (id === 'mine') assigneeOnly.value = false
        else statusFilter.value = statusFilter.value.filter((t) => `status:${t}` !== id)
      }

      function clearFilters() {
        statusFilter.value = []
        assigneeOnly.value = false
      }

      const sortKey = ref('none')
      const sortDir = ref('asc')
      const sortKeys = [
        { value: 'none', label: 'Manual' },
        { value: 'title', label: 'Title' },
        { value: 'assignee', label: 'Assignee' },
        { value: 'dueDate', label: 'Due date' },
      ]
      const sortDirs = [
        { value: 'asc', label: 'Ascending' },
        { value: 'desc', label: 'Descending' },
      ]

      /*
       * Sorting happens inside each section rather than across them: the
       * sections are the grouping the table is built on, and reordering rows
       * between them would dissolve it.
       *
       * Filtering drops a whole section once nothing in it survives, so the
       * list does not fill with empty headings.
       */
      const sections = computed(() => {
        const text = query.value.trim().toLowerCase()

        return bulkTableSections
          .filter((section) => !statusFilter.value.length || statusFilter.value.includes(section.title))
          .map((section) => {
            let items = section.items
            if (text) items = items.filter((item) => item.title.toLowerCase().includes(text))
            if (assigneeOnly.value) items = items.filter((item) => item.assignee === 'Rowdy')

            if (sortKey.value !== 'none') {
              const key = sortKey.value as 'title' | 'assignee' | 'dueDate'
              const factor = sortDir.value === 'asc' ? 1 : -1
              items = [...items].sort(
                (a, b) => String(a[key] ?? '').localeCompare(String(b[key] ?? '')) * factor,
              )
            }

            /*
             * The count follows the rows once a filter is on. Left at the
             * fixture's own number it would claim 31 beside four rows.
             */
            const filtered = text || assigneeOnly.value || statusFilter.value.length
            return { ...section, items, count: filtered ? items.length : section.count }
          })
          .filter((section) => section.items.length > 0)
      })

      const total = computed(() =>
        sections.value.reduce((sum, section) => sum + section.items.length, 0),
      )

      return {
        bulkNavGroups, bulkTableColumns, viewTabs: scaleViewTabs, view, navOpen, sidebarWidth: ref(260), myTasksOpen: ref(false),
        searchOpen, query, searchBox, toggleSearch,
        statuses, statusFilter, toggleStatus, assigneeOnly,
        chips, removeChip, clearFilters,
        sortKey, sortDir, sortKeys, sortDirs, sections, total,
      }
    },
    template: `
      <RlMyTasksPanels v-model:open="myTasksOpen" :sidebar-width="sidebarWidth">
        <RlAppShell v-model:nav-open="navOpen" v-model:sidebar-width="sidebarWidth" :sidebar-default-width="260">
          <template #sidebar>
            <RlSidebar
              workspace-name="Atlas Projects"
              :groups="bulkNavGroups"
              active-id="legacy"
              :flyout-id="myTasksOpen ? 'my-tasks' : null"
              @select="myTasksOpen = $event.id === 'my-tasks' ? !myTasksOpen : myTasksOpen"
              @close="navOpen = false"
            />
          </template>

          <RlPageHeader
            title="Legacy schema migration"
            status-label="At risk"
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
              <!--
                The field takes the toolbar's place while it is open, rather
                than sitting beside three buttons in a bar that also carries the
                view tabs. A search is a mode: you are looking for something,
                and the other two tools can wait until you are done.
              -->
              <RlSearchBox
                v-if="searchOpen"
                ref="searchBox"
                v-model="query"
                size="sm"
                placeholder="Search tasks"
                :result-text="total + ' results'"
                style="width:240px"
              />
              <RlButton
                size="sm"
                :aria-expanded="searchOpen"
                :aria-label="searchOpen ? 'Close search' : 'Search'"
                @click="toggleSearch"
              >
                <template #icon><RlIcon :name="searchOpen ? 'x' : 'search'" :size="15" /></template>
                {{ searchOpen ? 'Close' : 'Search' }}
              </RlButton>

              <!--
                A popover, not a menu: a menu closes on any click inside it, so
                ticking one box would dismiss the panel before a second could be
                ticked. Filters are chosen several at a time.
              -->
              <RlPopover v-if="!searchOpen" title="Filter tasks" align="end">
                <template #trigger="{ toggle, attrs }">
                  <RlButton size="sm" v-bind="attrs" @click="toggle">
                    <template #icon><RlIcon name="filter" :size="15" /></template>
                    Filter<template v-if="chips.length"> ({{ chips.length }})</template>
                  </RlButton>
                </template>

                <template #default="{ close }">
                  <div style="display:flex; flex-direction:column; gap:var(--rl-space-3); min-width:200px">
                    <strong style="font-size:var(--rl-font-size-sm)">Status</strong>
                    <RlCheckbox
                      v-for="status in statuses"
                      :key="status"
                      :label="status"
                      :model-value="statusFilter.includes(status)"
                      @update:model-value="toggleStatus(status, $event)"
                    />
                    <hr style="margin:0; border:none; border-top:1px solid var(--rl-color-border)" />
                    <RlCheckbox
                      label="Assigned to me"
                      :model-value="assigneeOnly"
                      @update:model-value="assigneeOnly = $event"
                    />
                    <div style="display:flex; gap:var(--rl-space-2); justify-content:flex-end">
                      <RlButton size="sm" variant="ghost" @click="clearFilters">Clear</RlButton>
                      <RlButton size="sm" variant="primary" @click="close">Done</RlButton>
                    </div>
                  </div>
                </template>
              </RlPopover>

              <RlPopover v-if="!searchOpen" title="Sort tasks" align="end">
                <template #trigger="{ toggle, attrs }">
                  <RlButton size="sm" v-bind="attrs" @click="toggle">
                    <template #icon><RlIcon name="arrow-up-down" :size="15" /></template>
                    Sort
                  </RlButton>
                </template>

                <template #default="{ close }">
                  <div style="display:flex; flex-direction:column; gap:var(--rl-space-3); min-width:200px">
                    <strong style="font-size:var(--rl-font-size-sm)">Sort by</strong>
                    <label
                      v-for="option in sortKeys"
                      :key="option.value"
                      style="display:flex; gap:var(--rl-space-2); align-items:center; font-size:var(--rl-font-size-sm)"
                    >
                      <input type="radio" name="sort-key" :value="option.value" :checked="sortKey === option.value" @change="sortKey = option.value" />
                      {{ option.label }}
                    </label>
                    <RlSegmentedControl
                      v-if="sortKey !== 'none'"
                      :model-value="sortDir"
                      label="Sort direction"
                      size="sm"
                      :options="sortDirs"
                      @update:model-value="sortDir = $event"
                    />
                    <div style="display:flex; justify-content:flex-end">
                      <RlButton size="sm" variant="primary" @click="close">Done</RlButton>
                    </div>
                  </div>
                </template>
              </RlPopover>
            </template>
          </RlPageHeader>

          <!--
            The filters read back, so what is narrowing the list is visible
            without reopening the panel that set it.
          -->
          <div
            v-if="chips.length"
            style="display:flex; flex-wrap:wrap; align-items:center; gap:var(--rl-space-2); padding:var(--rl-space-3) var(--rl-page-gutter-right) 0 var(--rl-page-gutter-left)"
          >
            <RlFilterChip
              v-for="chip in chips"
              :key="chip.id"
              :label="chip.label"
              :value="chip.value"
              :icon="chip.icon"
              @remove="removeChip(chip.id)"
            />
            <RlButton size="sm" variant="ghost" @click="clearFilters">Clear all</RlButton>
          </div>

          <!--
            A query matching nothing is the state worth showing at this scale:
            131 rows and none of them the one you meant.
          -->
          <RlEmptyState
            v-if="!sections.length"
            icon="search"
            title="No tasks match"
            description="Try a shorter query, or clear the filters."
          >
            <template #actions>
              <RlButton variant="secondary" @click="query = ''; clearFilters()">Clear everything</RlButton>
            </template>
          </RlEmptyState>

          <!--
            Capped, so every section heading stays reachable: without it the
            first section alone runs for six screens and buries the rest.
          -->
          <RlTable v-else :sections="sections" :columns="bulkTableColumns" :page-size="15" />
        </RlAppShell>
      </RlMyTasksPanels>
    `,
  }),
}

/**
 * The same 131 tasks as a board: six open columns, wider than the viewport,
 * each taller than it.
 *
 * What to look at: whether a column scrolls inside itself or drags the board
 * with it, how a card with three tags and a long title reflows, and what the
 * horizontal scroll does to the column headers.
 */
export const InitiativeBoardAtScale: StoryObj = {
  render: () => ({
    components: {
      RlAppShell, RlSidebar, RlMyTasksPanels, RlPageHeader, RlViewTabs, RlBoard, RlButton, RlIcon,
    },
    setup: () => ({
      bulkNavGroups, bulkBoardSections,
      viewTabs: scaleViewTabs, view: ref('board'), navOpen: ref(false), sidebarWidth: ref(260), myTasksOpen: ref(false),
    }),
    template: `
      <RlMyTasksPanels v-model:open="myTasksOpen" :sidebar-width="sidebarWidth">
        <RlAppShell v-model:nav-open="navOpen" v-model:sidebar-width="sidebarWidth" :sidebar-default-width="260">
          <template #sidebar>
            <RlSidebar
              workspace-name="Atlas Projects"
              :groups="bulkNavGroups"
              active-id="legacy"
              :flyout-id="myTasksOpen ? 'my-tasks' : null"
              @select="myTasksOpen = $event.id === 'my-tasks' ? !myTasksOpen : myTasksOpen"
              @close="navOpen = false"
            />
          </template>

          <RlPageHeader
            title="Legacy schema migration"
            status-label="At risk"
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

          <RlBoard :sections="bulkBoardSections" :selected-id="'in-progress-3'" />
        </RlAppShell>
      </RlMyTasksPanels>
    `,
  }),
}

/**
 * List and detail side by side, with both panes loaded: 131 tasks in the
 * list, and a task carrying twelve fields, twelve attachments, twenty-four
 * subtasks, twelve paragraphs and a twenty-eight-comment thread.
 *
 * What to look at: the two panes scroll independently, the composer stays at
 * the foot of the panel however long the thread runs, and selecting a task
 * from the list does not reset the panel's scroll for the wrong reason.
 */
export const TaskDetailBesideListAtScale: StoryObj = {
  render: () => ({
    components: {
      RlAppShell, RlSidebar, RlMyTasksPanels, RlPageHeader, RlViewTabs, RlTable, RlBoard,
      RlDetailPanel, RlTaskDetail, RlComment, RlCommentComposer, RlIconButton,
    },
    setup() {
      const view = ref('table')
      const selectedTaskId = ref('in-progress-3')
      const panelOpen = ref(true)

      function select(task: Task) {
        selectedTaskId.value = task.id
        panelOpen.value = true
      }

      return {
        bulkNavGroups, bulkTableSections, bulkBoardSections, bulkDetailFields,
        bulkDetailTitle, bulkDescription, bulkAttachments, bulkSubtasks, bulkComments,
        viewTabs: scaleViewTabs, view, selectedTaskId, panelOpen, select,
        navOpen: ref(false), sidebarWidth: ref(260), myTasksOpen: ref(false),
      }
    },
    template: `
      <RlMyTasksPanels v-model:open="myTasksOpen" :sidebar-width="sidebarWidth">
        <RlAppShell v-model:nav-open="navOpen" v-model:sidebar-width="sidebarWidth" :sidebar-default-width="260" :panel-open="panelOpen" panel-mode="inline">
          <template #sidebar>
            <RlSidebar
              workspace-name="Atlas Projects"
              :groups="bulkNavGroups"
              active-id="legacy"
              :flyout-id="myTasksOpen ? 'my-tasks' : null"
              @select="myTasksOpen = $event.id === 'my-tasks' ? !myTasksOpen : myTasksOpen"
              @close="navOpen = false"
            />
          </template>

          <template #header>
            <RlPageHeader
              title="Legacy schema migration"
              status-label="At risk"
              :show-star="false"
              @open-nav="navOpen = true"
            >
              <template #tabs><RlViewTabs v-model="view" :tabs="viewTabs" :show-add="false" /></template>
            </RlPageHeader>
          </template>

          <RlTable
            v-if="view === 'table'"
            :sections="bulkTableSections"
            :columns="[]"
            :page-size="15"
            :selected-id="selectedTaskId"
            @select="select"
          />
          <RlBoard
            v-else
            :sections="bulkBoardSections"
            :show-add-section="false"
            :selected-id="selectedTaskId"
            @select="select"
          />

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
                :title="bulkDetailTitle"
                :fields="bulkDetailFields"
                :description="bulkDescription"
                :attachments="bulkAttachments"
                :subtasks="bulkSubtasks"
              />
              <div class="rl-detail-panel-bleed" style="border-top:1px solid var(--rl-color-border); padding-block:24px">
                <RlComment v-for="c in bulkComments" :key="c.id" :comment="c" />
              </div>
              <template #footer><RlCommentComposer /></template>
            </RlDetailPanel>
          </template>
        </RlAppShell>
      </RlMyTasksPanels>
    `,
  }),
}

/**
 * A workspace with more initiatives than fit: the sidebar past the point
 * where reading down the list stops working.
 *
 * The sidebar adjusts itself rather than being configured. Past a count of
 * visible items it grows a filter, which is why the smaller screens above
 * show no such control. Its group header sticks while its own items scroll
 * beneath it, and the open initiative is scrolled into view on load, so the
 * one item that says where you are is never the one below the fold.
 *
 * The names are close together on purpose. Several begin "Billing" and three
 * name ISO27001, which is what makes a list this long hard to scan rather
 * than merely long, and what the filter is there to answer.
 */
export const CrowdedSidebar: StoryObj = {
  render: () => ({
    components: {
      RlAppShell, RlSidebar, RlMyTasksPanels, RlPageHeader, RlViewTabs, RlTable, RlButton, RlIcon,
    },
    setup: () => ({
      crowdedNavGroups, bulkTableSections, bulkTableColumns,
      viewTabs: scaleViewTabs, view: ref('table'), navOpen: ref(false), sidebarWidth: ref(260), myTasksOpen: ref(false),
    }),
    template: `
      <RlMyTasksPanels v-model:open="myTasksOpen" :sidebar-width="sidebarWidth">
        <RlAppShell v-model:nav-open="navOpen" v-model:sidebar-width="sidebarWidth" :sidebar-default-width="260">
          <template #sidebar>
            <!-- Deep in the list, so the reveal-on-load has something to do. -->
            <RlSidebar
              workspace-name="Atlas Projects"
              :groups="crowdedNavGroups"
              active-id="seat-count"
              :flyout-id="myTasksOpen ? 'my-tasks' : null"
              @select="myTasksOpen = $event.id === 'my-tasks' ? !myTasksOpen : myTasksOpen"
              @close="navOpen = false"
            />
          </template>

          <RlPageHeader
            title="Seat-count reporting"
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

          <RlTable :sections="bulkTableSections" :columns="bulkTableColumns" :page-size="15" />
        </RlAppShell>
      </RlMyTasksPanels>
    `,
  }),
}
