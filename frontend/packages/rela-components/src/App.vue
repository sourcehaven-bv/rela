<script setup lang="ts">
import RlAppShell from './components/layout/RlAppShell.vue'
import RlSidebar from './components/layout/RlSidebar.vue'
import RlPageHeader from './components/layout/RlPageHeader.vue'
import RlViewTabs from './components/layout/RlViewTabs.vue'
import RlBoard from './components/board/RlBoard.vue'
import { navGroups, boardSections } from './fixtures'
import { ref } from 'vue'

const view = ref('board')
</script>

<template>
  <RlAppShell>
    <template #sidebar>
      <RlSidebar workspace-name="Atlas Projects" :groups="navGroups" active-id="atlas" />
    </template>

    <RlPageHeader title="Atlas for Sroucehaven" status-label="On track" starred>
      <template #tabs>
        <RlViewTabs
          v-model="view"
          :tabs="[
            { id: 'board', label: 'Board', icon: 'kanban' },
            { id: 'table', label: 'Table', icon: 'table' },
            { id: 'timeline', label: 'Timeline', icon: 'chart-no-axes-gantt' },
          ]"
        />
      </template>
    </RlPageHeader>

    <RlBoard :sections="boardSections" />
  </RlAppShell>
</template>
