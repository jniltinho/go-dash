import { createRouter, createWebHistory } from 'vue-router'

import { basePath } from './lib/basePath'
import { useAuthStore } from './stores/app'
import AdminDisabledView from './views/AdminDisabledView.vue'
import DashboardPage from './views/DashboardPage.vue'

export const router = createRouter({
  // Under a reverse-proxy prefix the server injects <base href>; routes stay "/...".
  history: createWebHistory(basePath()),
  routes: [
    { path: '/', name: 'dashboard', component: DashboardPage, props: { slug: 'default' } },
    // Same slug rule as the server; anything else falls through to the catch-all.
    { path: '/dash/:slug([a-z0-9-]+)', name: 'named-dashboard', component: DashboardPage, props: true },
    { path: '/admin', name: 'admin', component: () => import('./views/AdminView.vue') },
    { path: '/admin/login', name: 'login', component: () => import('./views/LoginView.vue') },
    { path: '/admin/disabled', name: 'admin-disabled', component: AdminDisabledView },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async (to) => {
  if (!to.path.startsWith('/admin') || to.name === 'admin-disabled') return true

  const auth = useAuthStore()
  let status
  try {
    status = await auth.refresh()
  } catch {
    return { name: 'dashboard' }
  }
  if (!status.admin_enabled) return { name: 'admin-disabled' }
  if (to.name === 'login') return status.authenticated ? { name: 'admin' } : true
  return status.authenticated ? true : { name: 'login' }
})
