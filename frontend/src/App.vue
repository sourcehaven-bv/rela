<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useSchemaStore, useUIStore } from '@/stores'
import Sidebar from '@/components/common/Sidebar.vue'
import Toast from '@/components/common/Toast.vue'

const schemaStore = useSchemaStore()
const uiStore = useUIStore()
const loading = ref(true)
const error = ref<string | null>(null)

onMounted(async () => {
  try {
    await schemaStore.load()
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load application'
    uiStore.error(error.value)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div v-if="loading" class="loading-screen">
    <div class="spinner"></div>
    <p>Loading...</p>
  </div>

  <div v-else-if="error" class="error-screen">
    <h1>Error</h1>
    <p>{{ error }}</p>
    <button @click="schemaStore.reload()">Retry</button>
  </div>

  <div v-else class="app-layout">
    <Sidebar />
    <main class="main-content" :class="{ 'sidebar-collapsed': uiStore.sidebarCollapsed }">
      <RouterView />
    </main>
    <Toast />
  </div>
</template>

<style>
:root {
  --sidebar-bg: #1a1a2e;
  --sidebar-text: #e8e8e8;
  --accent-color: #6366f1;
  --bg-color: #f8fafc;
  --text-color: #1e293b;
  --border-color: #e2e8f0;
  --success-color: #10b981;
  --error-color: #ef4444;
  --warning-color: #f59e0b;
  --info-color: #3b82f6;
}

* {
  box-sizing: border-box;
  margin: 0;
  padding: 0;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
  background: var(--bg-color);
  color: var(--text-color);
  line-height: 1.5;
}

.app-layout {
  display: flex;
  min-height: 100vh;
}

.main-content {
  flex: 1;
  margin-left: 240px;
  padding: 24px;
  transition: margin-left 0.2s ease;
}

.main-content.sidebar-collapsed {
  margin-left: 60px;
}

.loading-screen,
.error-screen {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  gap: 16px;
}

.spinner {
  width: 40px;
  height: 40px;
  border: 3px solid var(--border-color);
  border-top-color: var(--accent-color);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.error-screen h1 {
  color: var(--error-color);
}

.error-screen button {
  padding: 8px 16px;
  background: var(--accent-color);
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

@media (max-width: 768px) {
  .main-content {
    margin-left: 0;
  }
}
</style>
