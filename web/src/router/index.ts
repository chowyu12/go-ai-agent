import { createRouter, createWebHistory } from 'vue-router'
import { setupApi, type SetupStatus } from '@/api/setup'
import { authApi } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'

let _setupStatus: SetupStatus | null = null
let _tokenVerified = false

async function checkSetupStatus(): Promise<SetupStatus> {
  if (_setupStatus) return _setupStatus
  try {
    const res: any = await setupApi.check()
    _setupStatus = res.data
    return _setupStatus!
  } catch {
    return { database_configured: true, initialized: true }
  }
}

async function verifyToken(): Promise<boolean> {
  if (_tokenVerified) return true
  try {
    const res: any = await authApi.me()
    const authStore = useAuthStore()
    authStore.setAuth(authStore.token, res.data)
    _tokenVerified = true
    return true
  } catch {
    const authStore = useAuthStore()
    authStore.logout()
    _tokenVerified = false
    return false
  }
}

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/setup/database',
      name: 'SetupDatabase',
      component: () => import('../views/setup/Database.vue'),
      meta: { public: true },
    },
    {
      path: '/setup',
      name: 'Setup',
      component: () => import('../views/setup/Index.vue'),
      meta: { public: true },
    },
    {
      path: '/login',
      name: 'Login',
      component: () => import('../views/login/Index.vue'),
      meta: { public: true },
    },
    {
      path: '/',
      component: () => import('../components/layout/AppLayout.vue'),
      children: [
        { path: '', name: 'Dashboard', component: () => import('../views/dashboard/Index.vue') },
        { path: 'providers', name: 'Providers', component: () => import('../views/provider/Index.vue') },
        { path: 'agents', name: 'Agents', component: () => import('../views/agent/Index.vue') },
        { path: 'agents/create', name: 'AgentCreate', component: () => import('../views/agent/Form.vue') },
        { path: 'agents/:id/edit', name: 'AgentEdit', component: () => import('../views/agent/Form.vue') },
        { path: 'tools', name: 'Tools', component: () => import('../views/tool/Index.vue') },
        { path: 'tools/create', name: 'ToolCreate', component: () => import('../views/tool/Form.vue') },
        { path: 'tools/:id/edit', name: 'ToolEdit', component: () => import('../views/tool/Form.vue') },
        { path: 'skills', name: 'Skills', component: () => import('../views/skill/Index.vue') },
        { path: 'skills/create', name: 'SkillCreate', component: () => import('../views/skill/Form.vue') },
        { path: 'skills/:id/edit', name: 'SkillEdit', component: () => import('../views/skill/Form.vue') },
        { path: 'mcp-servers', name: 'McpServers', component: () => import('../views/mcp/Index.vue') },
        { path: 'mcp-servers/create', name: 'McpCreate', component: () => import('../views/mcp/Form.vue') },
        { path: 'mcp-servers/:id/edit', name: 'McpEdit', component: () => import('../views/mcp/Form.vue') },
        { path: 'chat', name: 'Chat', component: () => import('../views/chat/Index.vue') },
        { path: 'logs', name: 'Logs', component: () => import('../views/log/Index.vue') },
        { path: 'users', name: 'Users', component: () => import('../views/user/Index.vue'), meta: { adminOnly: true } },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  const status = await checkSetupStatus()

  if (!status.database_configured && to.path !== '/setup/database') {
    return '/setup/database'
  }

  if (status.database_configured && !status.initialized && to.path !== '/setup') {
    return '/setup'
  }

  if (status.database_configured && status.initialized) {
    if (to.path === '/setup/database' || to.path === '/setup') {
      return '/login'
    }
  }

  const token = localStorage.getItem('token')
  if (!to.meta.public && !token) {
    return '/login'
  }

  if (!to.meta.public && token && !_tokenVerified) {
    const valid = await verifyToken()
    if (!valid) {
      return '/login'
    }
  }

  if (to.path === '/login' && token) {
    return '/'
  }

  if (to.meta.adminOnly) {
    const authStore = useAuthStore()
    if (!authStore.isAdmin) {
      return '/'
    }
  }
})

export function resetSetupStatus() {
  _setupStatus = null
}

export function resetTokenVerified() {
  _tokenVerified = false
}

export default router
