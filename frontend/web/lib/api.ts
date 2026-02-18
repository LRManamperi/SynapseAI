import axios from 'axios'

const AUTH_API_URL = process.env.NEXT_PUBLIC_AUTH_API_URL || 'http://localhost:8001'
const CONTENT_API_URL = process.env.NEXT_PUBLIC_CONTENT_API_URL || 'http://localhost:8003'
const QUIZ_API_URL = process.env.NEXT_PUBLIC_QUIZ_API_URL || 'http://localhost:8005'
const PROGRESS_API_URL = process.env.NEXT_PUBLIC_PROGRESS_API_URL || 'http://localhost:8006'

// Auth API instance
const authApi = axios.create({
  baseURL: AUTH_API_URL,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Content API instance
const contentApi = axios.create({
  baseURL: CONTENT_API_URL,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Quiz API instance
const quizApi = axios.create({
  baseURL: QUIZ_API_URL,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Progress API instance
const progressApi = axios.create({
  baseURL: PROGRESS_API_URL,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Add auth token to all instances
const addAuthToken = (config: any) => {
  if (typeof window !== 'undefined') {
    const token = localStorage.getItem('access_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
  }
  return config
}

authApi.interceptors.request.use(addAuthToken, (error) => Promise.reject(error))
contentApi.interceptors.request.use(addAuthToken, (error) => Promise.reject(error))
quizApi.interceptors.request.use(addAuthToken, (error) => Promise.reject(error))
progressApi.interceptors.request.use(addAuthToken, (error) => Promise.reject(error))

// Handle 401 errors for all instances
const handleAuthError = (error: any) => {
  if (error.response?.status === 401 && typeof window !== 'undefined') {
    localStorage.removeItem('access_token')
    localStorage.removeItem('refresh_token')
    window.location.href = '/login'
  }
  return Promise.reject(error)
}

authApi.interceptors.response.use((response) => response, handleAuthError)
contentApi.interceptors.response.use((response) => response, handleAuthError)
quizApi.interceptors.response.use((response) => response, handleAuthError)
progressApi.interceptors.response.use((response) => response, handleAuthError)

// Auth API
export const authAPI = {
  register: (data: { email: string; password: string; name: string }) =>
    authApi.post('/register', data),
  
  login: (data: { email: string; password: string }) =>
    authApi.post('/login', data),
  
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
    contentApi.post('/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    }),
  
  list: () =>
    contentApi.get('/list'),
  
  get: (contentId: string) =>
    contentApi.get(`/${contentId}`),
  
  delete: (contentId: string) =>
    contentApi.delete(`/${contentId}`),
}

// Quiz API
export const quizAPI = {
  get: (quizId: string) =>
    quizApi.get(`/${quizId}`),
  
  list: (contentId: string, page = 1, limit = 10) =>
    quizApi.get(`/list?content_id=${contentId}&page=${page}&limit=${limit}`),
  
  listAll: () =>
    quizApi.get('/list'),
  
  submit: (quizId: string, answers: Array<{ question_id: string; selected_option: number }>) =>
    quizApi.post(`/${quizId}/submit`, { answers }),
  
  history: (quizId: string, limit = 10) =>
    quizApi.get(`/${quizId}/history?limit=${limit}`),
  
  stats: (quizId: string) =>
    quizApi.get(`/${quizId}/stats`),
}

// Progress API
export const progressAPI = {
  getProgress: () =>
    progressApi.get('/progress'),
  
  getStreak: () =>
    progressApi.get('/streak'),
  
  getLeaderboard: () =>
    progressApi.get('/leaderboard'),
}
