'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { useAuthStore } from '@/store/authStore'
import { contentAPI, quizAPI, aiAPI } from '@/lib/api'

interface Content {
  content_id: string
  title: string
  uploaded_at: string
}

interface Quiz {
  quiz_id: string
  content_id: string
  title: string
  difficulty: string
  created_at: string
  question_count: number
}

export default function QuizzesPage() {
  const router = useRouter()
  const { user, isAuthenticated } = useAuthStore()
  const [contents, setContents] = useState<Content[]>([])
  const [quizzesByContent, setQuizzesByContent] = useState<Record<string, Quiz[]>>({})
  const [loading, setLoading] = useState(true)
  const [generating, setGenerating] = useState<Record<string, boolean>>({})
  const [generateMsg, setGenerateMsg] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!isAuthenticated()) {
      router.push('/login')
      return
    }
    loadData()
  }, [isAuthenticated, router])

  const loadData = async () => {
    try {
      setError(null)
      const contentResponse = await contentAPI.list()
      const contentItems = contentResponse.data.items || []
      setContents(contentItems)

      const quizzesMap: Record<string, Quiz[]> = {}
      for (const content of contentItems) {
        try {
          const quizResponse = await quizAPI.list(content.content_id, 1, 10)
          if (quizResponse.data.items && quizResponse.data.items.length > 0) {
            quizzesMap[content.content_id] = quizResponse.data.items
          }
        } catch (err) {
          console.error(`Failed to load quizzes for content ${content.content_id}:`, err)
        }
      }
      setQuizzesByContent(quizzesMap)
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to load data'
      setError(errorMessage)
    } finally {
      setLoading(false)
    }
  }

  const handleGenerateQuiz = async (content: Content) => {
    setGenerating((prev) => ({ ...prev, [content.content_id]: true }))
    setGenerateMsg((prev) => ({ ...prev, [content.content_id]: '' }))
    try {
      await aiAPI.retrigger({
        content_id: content.content_id,
        user_id: user?.id || '',
        title: content.title,
      })
      setGenerateMsg((prev) => ({
        ...prev,
        [content.content_id]: '✅ Quiz generation started! Refresh in a few seconds.',
      }))
      // Auto-refresh quizzes for this content after 4 seconds
      setTimeout(async () => {
        try {
          const quizResponse = await quizAPI.list(content.content_id, 1, 10)
          if (quizResponse.data.items && quizResponse.data.items.length > 0) {
            setQuizzesByContent((prev) => ({
              ...prev,
              [content.content_id]: quizResponse.data.items,
            }))
            setGenerateMsg((prev) => ({ ...prev, [content.content_id]: '' }))
          }
        } catch {}
      }, 4000)
    } catch (err: any) {
      setGenerateMsg((prev) => ({
        ...prev,
        [content.content_id]: `❌ ${err.response?.data || err.message || 'Generation failed'}`,
      }))
    } finally {
      setGenerating((prev) => ({ ...prev, [content.content_id]: false }))
    }
  }

  if (!user) return null

  const totalQuizzes = Object.values(quizzesByContent).reduce((sum, q) => sum + q.length, 0)

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      <nav className="bg-white dark:bg-gray-800 shadow">
        <div className="container mx-auto px-4 py-4 flex justify-between items-center">
          <Link href="/dashboard" className="text-2xl font-bold text-primary-600">
            SynapseAI
          </Link>
          <div className="flex items-center gap-4">
            <Link href="/dashboard" className="text-gray-700 dark:text-gray-300 hover:text-primary-600">
              Dashboard
            </Link>
            <Link href="/upload" className="text-gray-700 dark:text-gray-300 hover:text-primary-600">
              Upload
            </Link>
          </div>
        </div>
      </nav>

      <div className="container mx-auto px-4 py-8">
        <div className="mb-8 flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold mb-2">📚 Your Quizzes</h1>
            <p className="text-gray-600 dark:text-gray-400">
              {totalQuizzes} quiz{totalQuizzes !== 1 ? 'zes' : ''} from {contents.length} document{contents.length !== 1 ? 's' : ''}
            </p>
          </div>
          <button onClick={loadData} className="btn-secondary text-sm">
            🔄 Refresh
          </button>
        </div>

        {loading ? (
          <div className="text-center py-12">
            <p className="text-gray-600 dark:text-gray-400">Loading quizzes...</p>
          </div>
        ) : error ? (
          <div className="text-center py-12">
            <p className="text-red-600 dark:text-red-400 mb-4">{error}</p>
            <button onClick={loadData} className="btn-primary">Retry</button>
          </div>
        ) : contents.length === 0 ? (
          <div className="text-center py-12">
            <div className="text-6xl mb-4">📝</div>
            <h2 className="text-xl font-semibold mb-2">No Documents Yet</h2>
            <p className="text-gray-600 dark:text-gray-400 mb-6">
              Upload a document to generate AI-powered quizzes
            </p>
            <Link href="/upload" className="btn-primary">Upload Document</Link>
          </div>
        ) : (
          <div className="space-y-6">
            {contents.map((content) => {
              const quizzes = quizzesByContent[content.content_id] || []
              const isGen = generating[content.content_id]
              const msg = generateMsg[content.content_id]

              return (
                <div key={content.content_id} className="card">
                  <div className="flex items-start justify-between mb-4">
                    <div>
                      <h2 className="text-xl font-semibold mb-1">{content.title}</h2>
                      <p className="text-sm text-gray-600 dark:text-gray-400">
                        Uploaded {new Date(content.uploaded_at).toLocaleDateString()}
                      </p>
                    </div>
                    <div className="flex items-center gap-3">
                      {quizzes.length > 0 && (
                        <span className="bg-primary-100 text-primary-800 dark:bg-primary-900 dark:text-primary-200 px-3 py-1 rounded-full text-sm">
                          {quizzes.length} Quiz{quizzes.length !== 1 ? 'zes' : ''}
                        </span>
                      )}
                      <button
                        onClick={() => handleGenerateQuiz(content)}
                        disabled={isGen}
                        className="btn-primary text-sm disabled:opacity-60"
                      >
                        {isGen ? '⏳ Generating...' : '✨ Generate Quiz'}
                      </button>
                    </div>
                  </div>

                  {msg && (
                    <p className="text-sm mb-4 text-gray-700 dark:text-gray-300">{msg}</p>
                  )}

                  {quizzes.length === 0 ? (
                    <p className="text-gray-500 dark:text-gray-400 text-sm">
                      No quizzes yet — click "Generate Quiz" to create one with AI.
                    </p>
                  ) : (
                    <div className="grid md:grid-cols-2 gap-4">
                      {quizzes.map((quiz) => (
                        <div
                          key={quiz.quiz_id}
                          className="border border-gray-200 dark:border-gray-700 rounded-lg p-4 hover:border-primary-500 transition-colors"
                        >
                          <div className="flex items-start justify-between mb-3">
                            <div>
                              <h3 className="font-semibold mb-1 text-gray-900 dark:text-gray-100">{quiz.title}</h3>
                              <p className="text-sm text-gray-600 dark:text-gray-400">
                                {quiz.question_count || 5} Questions • {quiz.difficulty || 'Intermediate'}
                              </p>
                            </div>
                            <span className="text-2xl">🎯</span>
                          </div>
                          <div className="flex gap-2">
                            <Link
                              href={`/quizzes/${quiz.quiz_id}`}
                              className="btn-primary text-sm flex-1 text-center"
                            >
                              Take Quiz
                            </Link>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
