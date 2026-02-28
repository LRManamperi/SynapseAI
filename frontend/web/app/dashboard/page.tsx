'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { useAuthStore } from '@/store/authStore'
import { contentAPI, quizAPI } from '@/lib/api'

interface Content {
  content_id: string
  title: string
  file_type: string
  file_size: number
  uploaded_at: string
}

interface UserStats {
  total_attempts: number
  passed: number
  avg_score: number
  xp: number
  days_active: number
  unique_quizzes: number
}

export default function DashboardPage() {
  const router = useRouter()
  const { user, isAuthenticated, clearAuth } = useAuthStore()
  const [contents, setContents] = useState<Content[]>([])
  const [stats, setStats] = useState<UserStats>({
    total_attempts: 0, passed: 0, avg_score: 0,
    xp: 0, days_active: 0, unique_quizzes: 0,
  })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!isAuthenticated()) {
      router.push('/login')
      return
    }
    loadData()
  }, [])

  const loadData = async () => {
    try {
      setError(null)
      const [contentRes, statsRes] = await Promise.allSettled([
        contentAPI.list(),
        quizAPI.userStats(user?.id || ''),
      ])
      if (contentRes.status === 'fulfilled') setContents(contentRes.value.data.items || [])
      if (statsRes.status === 'fulfilled') setStats(statsRes.value.data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load data')
    } finally {
      setLoading(false)
    }
  }

  const handleLogout = () => { clearAuth(); router.push('/') }

  const formatFileSize = (bytes: number) => {
    if (!bytes) return '—'
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  }

  const formatFileType = (type: string) => {
    if (!type) return 'Unknown'
    if (type.includes('pdf')) return 'PDF'
    if (type.includes('text')) return 'TXT'
    if (type.includes('word') || type.includes('docx')) return 'Word'
    return type.split('/').pop()?.toUpperCase() || type
  }

  const passRate = stats.total_attempts > 0
    ? Math.round((stats.passed / stats.total_attempts) * 100) : 0

  if (loading && !user) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-900 flex items-center justify-center">
        <div className="text-gray-600 dark:text-gray-400">Loading...</div>
      </div>
    )
  }
  if (!user) return null

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      {/* Navigation */}
      <nav className="bg-white dark:bg-gray-800 shadow">
        <div className="container mx-auto px-4 py-4 flex justify-between items-center">
          <h1 className="text-2xl font-bold text-primary-600">SynapseAI</h1>
          <div className="flex items-center gap-4">
            <span className="text-gray-700 dark:text-gray-300">Welcome, {user.name}</span>
            <button onClick={handleLogout} className="btn-secondary">Logout</button>
          </div>
        </div>
      </nav>

      <div className="container mx-auto px-4 py-8">

        {/* Stats Cards */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
          {[
            { icon: '📚', value: contents.length, label: 'Documents' },
            { icon: '🎯', value: stats.total_attempts, label: 'Quizzes Taken' },
            { icon: '⚡', value: `${stats.xp} XP`, label: 'Total XP' },
            { icon: '🔥', value: stats.days_active, label: 'Days Active' },
          ].map(({ icon, value, label }) => (
            <div key={label} className="card text-center">
              <div className="text-3xl mb-1">{icon}</div>
              <div className="text-2xl font-bold text-primary-600">{value}</div>
              <div className="text-sm text-gray-600 dark:text-gray-400 mt-1">{label}</div>
            </div>
          ))}
        </div>

        <div className="grid md:grid-cols-3 gap-6 mb-8">

          {/* Progress Panel */}
          <div className="card md:col-span-2">
            <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100 mb-5">Progress</h2>
            {stats.total_attempts === 0 ? (
              <div className="text-center py-8 text-gray-500 dark:text-gray-400">
                <div className="text-4xl mb-3">📝</div>
                <p className="mb-4">No quizzes taken yet. Take your first quiz to see progress!</p>
                <Link href="/quizzes" className="btn-primary">Go to Quizzes →</Link>
              </div>
            ) : (
              <div className="space-y-5">
                <div>
                  <div className="flex justify-between mb-1">
                    <span className="text-sm font-medium text-gray-700 dark:text-gray-300">Average Score</span>
                    <span className="text-sm font-bold text-gray-900 dark:text-gray-100">{Math.round(stats.avg_score)}%</span>
                  </div>
                  <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-3">
                    <div className="h-3 rounded-full transition-all duration-500"
                      style={{
                        width: `${Math.round(stats.avg_score)}%`,
                        backgroundColor: stats.avg_score >= 70 ? '#16a34a' : stats.avg_score >= 50 ? '#d97706' : '#dc2626',
                      }} />
                  </div>
                </div>
                <div>
                  <div className="flex justify-between mb-1">
                    <span className="text-sm font-medium text-gray-700 dark:text-gray-300">Pass Rate</span>
                    <span className="text-sm font-bold text-gray-900 dark:text-gray-100">{passRate}%</span>
                  </div>
                  <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-3">
                    <div className="h-3 rounded-full bg-blue-500 transition-all duration-500"
                      style={{ width: `${passRate}%` }} />
                  </div>
                </div>
                <div className="grid grid-cols-3 gap-3 pt-1">
                  <div className="bg-green-50 dark:bg-green-900/20 rounded-lg p-3 text-center">
                    <div className="text-xl font-bold text-green-700 dark:text-green-400">{stats.passed}</div>
                    <div className="text-xs text-green-600 dark:text-green-500">Passed</div>
                  </div>
                  <div className="bg-red-50 dark:bg-red-900/20 rounded-lg p-3 text-center">
                    <div className="text-xl font-bold text-red-700 dark:text-red-400">{stats.total_attempts - stats.passed}</div>
                    <div className="text-xs text-red-600 dark:text-red-500">Failed</div>
                  </div>
                  <div className="bg-purple-50 dark:bg-purple-900/20 rounded-lg p-3 text-center">
                    <div className="text-xl font-bold text-purple-700 dark:text-purple-400">{stats.unique_quizzes}</div>
                    <div className="text-xs text-purple-600 dark:text-purple-500">Unique Quizzes</div>
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* Quick Actions + Achievements */}
          <div className="card">
            <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100 mb-4">Quick Actions</h2>
            <div className="flex flex-col gap-3 mb-6">
              <Link href="/upload" className="btn-primary text-center">📤 Upload Document</Link>
              <Link href="/quizzes" className="btn-secondary text-center">🎯 View Quizzes</Link>
            </div>
            <div className="border-t border-gray-200 dark:border-gray-700 pt-4">
              <div className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Achievements</div>
              <div className="space-y-2 text-sm text-gray-600 dark:text-gray-400">
                {contents.length >= 1 ? <div>📂 First Document Uploaded</div> : <div className="opacity-40">📂 Upload a document</div>}
                {stats.total_attempts >= 1 ? <div>🏅 First Quiz Taken</div> : <div className="opacity-40">🏅 Take a quiz</div>}
                {stats.passed >= 1 ? <div>✅ First Quiz Passed</div> : <div className="opacity-40">✅ Pass a quiz</div>}
                {stats.total_attempts >= 5 ? <div>🎖️ Quiz Enthusiast (5+)</div> : <div className="opacity-40">🎖️ Take 5 quizzes ({stats.total_attempts}/5)</div>}
                {stats.xp >= 100 ? <div>⚡ 100 XP Club</div> : <div className="opacity-40">⚡ Earn 100 XP ({stats.xp}/100)</div>}
                {stats.days_active >= 3 ? <div>🔥 3-Day Streak</div> : <div className="opacity-40">🔥 3 active days ({stats.days_active}/3)</div>}
              </div>
            </div>
          </div>
        </div>

        {/* Documents Table */}
        <div className="card">
          <div className="flex justify-between items-center mb-4">
            <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100">Uploaded Documents</h2>
            <Link href="/upload" className="text-sm text-primary-600 hover:underline">+ Add New</Link>
          </div>

          {loading ? (
            <p className="text-gray-600 dark:text-gray-400 py-4">Loading…</p>
          ) : error ? (
            <div className="text-center py-8">
              <p className="text-red-600 dark:text-red-400 mb-4">{error}</p>
              <button onClick={loadData} className="btn-primary">Retry</button>
            </div>
          ) : contents.length === 0 ? (
            <div className="text-center py-10 text-gray-500 dark:text-gray-400">
              <div className="text-4xl mb-3">📂</div>
              <p className="mb-4">No documents uploaded yet</p>
              <Link href="/upload" className="btn-primary">Upload Your First Document</Link>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-gray-200 dark:border-gray-700">
                    {['Title', 'Type', 'Size', 'Uploaded', 'Action'].map((h) => (
                      <th key={h} className="text-left py-2 px-3 text-gray-600 dark:text-gray-400 font-medium">{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {contents.map((c) => (
                    <tr key={c.content_id}
                      className="border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors">
                      <td className="py-3 px-3 font-medium text-gray-900 dark:text-gray-100">{c.title}</td>
                      <td className="py-3 px-3">
                        <span className="px-2 py-0.5 rounded text-xs font-medium bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300">
                          {formatFileType(c.file_type)}
                        </span>
                      </td>
                      <td className="py-3 px-3 text-gray-600 dark:text-gray-400">{formatFileSize(c.file_size)}</td>
                      <td className="py-3 px-3 text-gray-600 dark:text-gray-400">
                        {new Date(c.uploaded_at).toLocaleDateString()}
                      </td>
                      <td className="py-3 px-3">
                        <Link href="/quizzes" className="text-primary-600 hover:text-primary-500 text-xs font-medium">
                          View Quizzes →
                        </Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
