// Composables
import Home from '@/views/Home.vue'
import Mail from '@/views/Mail.vue'
import VKLogin from '@/views/VKLogin.vue'
import Users from '@/views/Users.vue'
import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/',
    component: Home,
  },
  {
    path: '/users',
    component: Users
  },
  {
    path: '/mail',
    component: Mail
  },
  {
    path: '/auth',
    component: VKLogin
  }
]

const router = createRouter({
  history: createWebHistory(process.env.BASE_URL),
  routes,
})

export default router
