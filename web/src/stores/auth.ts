import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { resetTokenVerified } from '@/router'

export interface UserInfo {
  id: number
  username: string
  role: 'admin' | 'guest'
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')
  const user = ref<UserInfo | null>(JSON.parse(localStorage.getItem('user') || 'null'))

  const isLoggedIn = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.role === 'admin')

  function setAuth(t: string, u: UserInfo) {
    token.value = t
    user.value = u
    localStorage.setItem('token', t)
    localStorage.setItem('user', JSON.stringify(u))
  }

  function logout() {
    token.value = ''
    user.value = null
    localStorage.removeItem('token')
    localStorage.removeItem('user')
    resetTokenVerified()
  }

  return { token, user, isLoggedIn, isAdmin, setAuth, logout }
})
