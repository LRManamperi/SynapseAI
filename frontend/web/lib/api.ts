import axios from 'axios'

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:80'

// Create axios instance
const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Request interceptor to add auth token
api.interceptors.request.use(
  (config) => {
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem('access_token')
      if (token) {
        config.headers.Authorization = `Bearer ${token}`
      }
    }
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// Response interceptor to handle errors
api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config

    // If 401 and hasn't retried, try to refresh token
    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true

      try {
        const refreshToken = typeof window !== 'undefined' ? localStorage.getItem('refresh_token') : null
        if (!refreshToken) {
          throw new Error('No refresh token available')
        }

        const response = await axios.post(`${API_BASE_URL}/api/auth/refresh`, {
          refresh_token: refreshToken,
        })

        const { access_token } = response.data
        if (typeof window !== 'undefined') {
          localStorage.setItem('access_token', access_token)
        }

        originalRequest.headers.Authorization = `Bearer ${access_token}`
        return api(originalRequest)
      } catch (refreshError) {
        // Refresh failed, redirect to login
        if (typeof window !== 'undefined') {
          localStorage.removeItem('access_token')
          localStorage.removeItem('refresh_token')
          window.location.href = '/login'
        }
        return Promise.reject(refreshError)
      }
    }

    return Promise.reject(error)
  }
)

// Auth API
export const authAPI = {
  register: (data: { email: string; password: string; name: string }) =>
    api.post('/api/auth/register', data),
  
  login: (data: { email: string; password: string }) =>
    api.post('/api/auth/login', data),
  
  logout: () => {
    if (typeof window !== 'undefined') {
      localStorage.removeItem('access_token')
      localStorage.removeItem('refresh_token')
    }
  },
}

// Content API
export const contentAPI = {
  upload: (formData: FormData) =>
    api.post('/api/content/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    }),
  
  list: (page = 1, limit = 10) =>
    api.get(`/api/content/list?page=${page}&limit=${limit}`),
  
  get: (contentId: string) =>
    api.get(`/api/content/${contentId}`),
  
  delete: (contentId: string) =>
    api.delete(`/api/content/${contentId}`),
}

// Quiz API
export const quizAPI = {
  get: (quizId: string) =>
    api.get(`/api/quiz/${quizId}`),
  
  list: (contentId: string, page = 1, limit = 10) =>
    api.get(`/api/quiz/list?content_id=${contentId}&page=${page}&limit=${limit}`),
  
  submit: (quizId: string, answers: Array<{ question_id: string; selected_option: number }>) =>
    api.post(`/api/quiz/${quizId}/submit`, { answers }),
  
  history: (quizId: string, limit = 10) =>
    api.get(`/api/quiz/${quizId}/history?limit=${limit}`),
  
  stats: (quizId: string) =>
    api.get(`/api/quiz/${quizId}/stats`),
}

export default api
