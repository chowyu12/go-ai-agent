import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      component: () => import('../components/layout/AppLayout.vue'),
      children: [
        { path: '', name: 'Dashboard', component: () => import('../views/dashboard/Index.vue') },
        { path: 'providers', name: 'Providers', component: () => import('../views/provider/Index.vue') },
        { path: 'agents', name: 'Agents', component: () => import('../views/agent/Index.vue') },
        { path: 'tools', name: 'Tools', component: () => import('../views/tool/Index.vue') },
        { path: 'skills', name: 'Skills', component: () => import('../views/skill/Index.vue') },
        { path: 'chat', name: 'Chat', component: () => import('../views/chat/Index.vue') },
        { path: 'logs', name: 'Logs', component: () => import('../views/log/Index.vue') },
      ],
    },
  ],
})

export default router
