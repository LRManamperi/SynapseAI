import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface User {
  id: string
  email: string
  name: string
  role: string
}

interface AuthState {
  user: User | null
  accessToken: string | null
  refreshToken: string | null
  setAuth: (user: User, accessToken: string, refreshToken: string) => void
  clearAuth: () => void
  isAuthenticated: () => boolean
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,
      accessToken: null,
      refreshToken: null,
      
      setAuth: (user, accessToken, refreshToken) => {
        set({ user, accessToken, refreshToken })
        // Sync with localStorage for API interceptor
        if (typeof window !== 'undefined') {
          localStorage.setItem('access_token', accessToken)
          localStorage.setItem('refresh_token', refreshToken)
        }
      },
      
      clearAuth: () => {
        set({ user: null, accessToken: null, refreshToken: null })
        // Clear localStorage for API interceptor
        if (typeof window !== 'undefined') {
          localStorage.removeItem('access_token')
          localStorage.removeItem('refresh_token')
        }
      },
      
      isAuthenticated: () => {
        return !!get().accessToken
      },
    }),
    {
      name: 'auth-storage',
    }
  )
)
