import { createRouter, createWebHistory } from 'vue-router'

const RouteHost = { template: '<div />' }

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/tasks' },
    { path: '/login', name: 'login', component: RouteHost, meta: { public: true } },
    { path: '/register', name: 'register', component: RouteHost, meta: { public: true } },
    { path: '/tasks', name: 'tasks', component: RouteHost },
    { path: '/workers', name: 'workers', component: RouteHost },
    { path: '/planning', name: 'planning', component: RouteHost },
    { path: '/:pathMatch(.*)*', redirect: '/tasks' }
  ]
})

router.beforeEach((to) => {
  const hasToken = Boolean(localStorage.getItem('token'))
  if (!to.meta.public && !hasToken) return { name: 'login' }
  if ((to.name === 'login' || to.name === 'register') && hasToken) return { name: 'tasks' }
  return true
})

export default router
