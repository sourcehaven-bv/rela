import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    redirect: '/dashboard',
  },
  {
    path: '/dashboard',
    name: 'dashboard',
    component: () => import('@/views/DashboardView.vue'),
  },
  {
    path: '/list/:id',
    name: 'list',
    component: () => import('@/views/ListView.vue'),
    props: true,
  },
  {
    path: '/form/:id',
    name: 'form-create',
    component: () => import('@/views/FormView.vue'),
    props: true,
  },
  {
    path: '/form/:id/:entityId',
    name: 'form-edit',
    component: () => import('@/views/FormView.vue'),
    props: true,
  },
  {
    path: '/entity/:type/:id',
    name: 'entity',
    component: () => import('@/views/EntityView.vue'),
    props: true,
  },
  {
    path: '/view/:id/:entityId',
    name: 'view',
    component: () => import('@/views/CustomView.vue'),
    props: true,
  },
  {
    path: '/kanban/:id',
    name: 'kanban',
    component: () => import('@/views/KanbanView.vue'),
    props: true,
  },
  {
    path: '/search',
    name: 'search',
    component: () => import('@/views/SearchView.vue'),
  },
  {
    path: '/analyze',
    name: 'analyze',
    component: () => import('@/views/AnalyzeView.vue'),
  },
  {
    path: '/settings',
    name: 'settings',
    component: () => import('@/views/SettingsView.vue'),
  },
  {
    path: '/graph',
    name: 'graph',
    component: () => import('@/views/GraphView.vue'),
  },
]

const router = createRouter({
  history: createWebHistory('/v2/'),
  routes,
})

export default router
