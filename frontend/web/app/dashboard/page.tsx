'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { useAuthStore } from '@/store/authStore'
import { contentAPI } from '@/lib/api'

interface Content {
  content_id: string
  user_id: string
  title: string
  file_type: string
  file_path: string
  uploaded_at: string
}

export default function DashboardPage() {
  const router = useRouter()
  const { user, isAuthenticated, clearAuth } = useAuthStore()
  const [contents, setContents] = useState<Content[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!isAuthenticated()) {
      router.push('/login')
      return
    }

    loadContents()
  }, [isAuthenticated, router])

  const loadContents = async () => {
    try {
      setError(null)
      const response = await contentAPI.list(1, 10)
      setContents(response.data.items || [])
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to load contents'
      setError(errorMessage)
      console.error('Failed to load contents:', err)
    } finally {
      setLoading(false)
    }
  }

  const handleLogout = () => {
    clearAuth()
    router.push('/')
  }

  // Handle loading and unauthenticated states
  if (loading && !user) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-900 flex items-center justify-center">
        <div className="text-gray-600 dark:text-gray-400">Loading...</div>
      </div>
    )
  }

  if (!user) {
    return null
  }

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      {/* Navigation */}
      <nav className="bg-white dark:bg-gray-800 shadow">
        <div className="container mx-auto px-4 py-4 flex justify-between items-center">
          <h1 className="text-2xl font-bold text-primary-600">SynapseAI</h1>
          <div className="flex items-center gap-4">
            <span className="text-gray-700 dark:text-gray-300">
              Welcome, {user.name}
            </span>
            <button onClick={handleLogout} className="btn-secondary">
              Logout
            </button>
          </div>
        </div>
      </nav>

      {/* Main Content */}
      <div className="container mx-auto px-4 py-8">
        {/* Stats Cards */}
        <div className="grid md:grid-cols-4 gap-6 mb-8">
          <div className="card">
            <div className="text-3xl mb-2">📚</div>
            <div className="text-2xl font-bold text-primary-600">
              {contents.length}
            </div>
            <div className="text-gray-600 dark:text-gray-400">Content Uploaded</div>
          </div>

          <div className="card">
            <div className="text-3xl mb-2">🎯</div>
            <div className="text-2xl font-bold text-primary-600">0</div>
            <div className="text-gray-600 dark:text-gray-400">Quizzes Completed</div>
          </div>

          <div className="card">
            <div className="text-3xl mb-2">⚡</div>
            <div className="text-2xl font-bold text-primary-600">0 XP</div>
            <div className="text-gray-600 dark:text-gray-400">Total Experience</div>
          </div>

          <div className="card">
            <div className="text-3xl mb-2">🔥</div>
            <div className="text-2xl font-bold text-primary-600">0 Days</div>
            <div className="text-gray-600 dark:text-gray-400">Current Streak</div>
          </div>
        </div>

        {/* Quick Actions */}
        <div className="card mb-8">
          <h2 className="text-xl font-semibold mb-4">Quick Actions</h2>
          <div className="flex gap-4">
            <Link href="/upload" className="btn-primary">
              📤 Upload Content
            </Link>
            <Link href="/dashboard" className="btn-secondary">
              📊 View Progress
            </Link>
          </div>
        </div>

        {/* Recent Content */}
        <div className="card">
          <h2 className="text-xl font-semibold mb-4">Recent Content</h2>
          {loading ? (
            <p className="text-gray-600 dark:text-gray-400">Loading...</p>
          ) : error ? (
            <div className="text-center py-8">
              <p className="text-red-600 dark:text-red-400 mb-4">{error}</p>
              <button onClick={loadContents} className="btn-primary">
                Retry
              </button>
            </div>
          ) : contents.length === 0 ? (
            <div className="text-center py-8 text-gray-600 dark:text-gray-400">
              <p className="mb-4">No content uploaded yet</p>
              <Link href="/upload" className="btn-primary">
                Upload Your First Document
              </Link>
            </div>
          ) : (
            <div className="space-y-4">
              {contents.map((content) => (
                <div
                  key={content.content_id}
                  className="border border-gray-200 dark:border-gray-700 rounded-lg p-4 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
                >
                  <h3 className="font-semibold mb-2">{content.title}</h3>
                  <div className="flex justify-between items-center text-sm text-gray-600 dark:text-gray-400">
                    <span>{content.file_type}</span>
                    <span>{new Date(content.uploaded_at).toLocaleDateString()}</span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
