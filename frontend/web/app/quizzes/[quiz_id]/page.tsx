'use client'

import { useEffect, useState } from 'react'
import { useRouter, useParams } from 'next/navigation'
import Link from 'next/link'
import { useAuthStore } from '@/store/authStore'
import { quizAPI } from '@/lib/api'

interface Question {
  question_id: string
  question_text: string
  options: string[]
  correct_option: number
  explanation: string
}

interface Quiz {
  quiz_id: string
  content_id: string
  title: string
  difficulty: string
  created_at: string
  questions: Question[]
}

interface SubmitResult {
  score: number
  total: number
  correct: number
  results: Array<{
    question_id: string
    correct: boolean
    correct_option: number
    explanation: string
  }>
}

export default function TakeQuizPage() {
  const router = useRouter()
  const params = useParams()
  const quizId = params.quiz_id as string
  const { user, isAuthenticated } = useAuthStore()

  const [quiz, setQuiz] = useState<Quiz | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [selected, setSelected] = useState<Record<string, number>>({})
  const [submitting, setSubmitting] = useState(false)
  const [result, setResult] = useState<SubmitResult | null>(null)

  useEffect(() => {
    if (!isAuthenticated()) {
      router.push('/login')
      return
    }
    fetchQuiz()
  }, [quizId])

  const fetchQuiz = async () => {
    try {
      const resp = await quizAPI.get(quizId)
      setQuiz(resp.data)
    } catch (err: any) {
      setError(err.response?.data || err.message || 'Failed to load quiz')
    } finally {
      setLoading(false)
    }
  }

  const handleSelect = (questionId: string, optionIndex: number) => {
    if (result) return // locked after submit
    setSelected((prev) => ({ ...prev, [questionId]: optionIndex }))
  }

  const handleSubmit = async () => {
    if (!quiz) return
    const unanswered = quiz.questions.filter((q) => selected[q.question_id] === undefined)
    if (unanswered.length > 0) {
      alert(`Please answer all questions (${unanswered.length} remaining)`)
      return
    }
    setSubmitting(true)
    try {
      const answers = quiz.questions.map((q) => ({
        question_id: q.question_id,
        selected_option: selected[q.question_id],
      }))
      const resp = await quizAPI.submit(quizId, answers)
      setResult(resp.data)
    } catch (err: any) {
      // If submit endpoint fails, calculate locally
      const correct = quiz.questions.filter(
        (q) => selected[q.question_id] === q.correct_option
      ).length
      setResult({
        score: Math.round((correct / quiz.questions.length) * 100),
        total: quiz.questions.length,
        correct,
        results: quiz.questions.map((q) => ({
          question_id: q.question_id,
          correct: selected[q.question_id] === q.correct_option,
          correct_option: q.correct_option,
          explanation: q.explanation,
        })),
      })
    } finally {
      setSubmitting(false)
    }
  }

  if (!user) return null

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-900 flex items-center justify-center">
        <p className="text-gray-600 dark:text-gray-400">Loading quiz…</p>
      </div>
    )
  }

  if (error || !quiz) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-900 flex items-center justify-center">
        <div className="text-center">
          <p className="text-red-600 dark:text-red-400 mb-4">{error || 'Quiz not found'}</p>
          <Link href="/quizzes" className="btn-primary">Back to Quizzes</Link>
        </div>
      </div>
    )
  }

  const answeredCount = Object.keys(selected).length
  const totalQ = quiz.questions?.length || 0

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      <nav className="bg-white dark:bg-gray-800 shadow">
        <div className="container mx-auto px-4 py-4 flex justify-between items-center">
          <Link href="/dashboard" className="text-2xl font-bold text-primary-600">
            SynapseAI
          </Link>
          <Link href="/quizzes" className="text-gray-700 dark:text-gray-300 hover:text-primary-600">
            ← Back to Quizzes
          </Link>
        </div>
      </nav>

      <div className="container mx-auto px-4 py-8 max-w-3xl">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-3xl font-bold mb-2">{quiz.title}</h1>
          <div className="flex items-center gap-4 text-sm text-gray-600 dark:text-gray-400">
            <span>🎯 {totalQ} Questions</span>
            <span>📊 {quiz.difficulty || 'Intermediate'}</span>
            {!result && (
              <span className={answeredCount === totalQ ? 'text-green-600' : ''}>
                ✏️ {answeredCount}/{totalQ} answered
              </span>
            )}
          </div>
        </div>

        {/* Score banner */}
        {result && (
          <div className={`card mb-8 text-center ${result.score >= 70 ? 'border-green-500' : 'border-red-400'} border-2`}>
            <div className="text-5xl mb-2">{result.score >= 90 ? '🏆' : result.score >= 70 ? '🎉' : '📚'}</div>
            <h2 className="text-2xl font-bold mb-1">
              {result.score >= 90 ? 'Excellent!' : result.score >= 70 ? 'Well done!' : 'Keep studying!'}
            </h2>
            <p className="text-4xl font-bold text-primary-600 mb-2">{result.score}%</p>
            <p className="text-gray-600 dark:text-gray-400">
              {result.correct} correct out of {result.total} questions
            </p>
            <div className="flex justify-center gap-3 mt-4">
              <button
                onClick={() => { setResult(null); setSelected({}) }}
                className="btn-secondary"
              >
                🔄 Retake Quiz
              </button>
              <Link href="/quizzes" className="btn-primary">Back to Quizzes</Link>
            </div>
          </div>
        )}

        {/* Questions */}
        <div className="space-y-6">
          {(quiz.questions || []).map((question, qIndex) => {
            const userAnswer = selected[question.question_id]
            const qResult = result?.results?.find((r) => r.question_id === question.question_id)
            const isCorrect = qResult?.correct

            return (
              <div
                key={question.question_id}
                className={`card ${
                  result
                    ? isCorrect
                      ? 'border-l-4 border-green-500'
                      : 'border-l-4 border-red-400'
                    : ''
                }`}
              >
                <div className="flex items-start gap-3 mb-4">
                  <span className="bg-primary-100 text-primary-800 dark:bg-primary-900 dark:text-primary-200 rounded-full w-8 h-8 flex items-center justify-center text-sm font-bold flex-shrink-0">
                    {qIndex + 1}
                  </span>
                  <p className="font-medium text-lg leading-snug text-gray-900 dark:text-gray-100">{question.question_text}</p>
                </div>

                <div className="space-y-2 ml-11">
                  {(question.options || []).map((option, optIndex) => {
                    const isSelected = userAnswer === optIndex
                    const isCorrectOpt = result && optIndex === qResult?.correct_option
                    const isWrongSelected = result && isSelected && !isCorrect

                    let optClass =
                      'w-full text-left px-4 py-3 rounded-lg border transition-colors text-sm '
                    if (isCorrectOpt) {
                      optClass += 'bg-green-100 border-green-500 text-green-800 dark:bg-green-900 dark:text-green-200'
                    } else if (isWrongSelected) {
                      optClass += 'bg-red-100 border-red-400 text-red-800 dark:bg-red-900 dark:text-red-200'
                    } else if (isSelected) {
                      optClass += 'bg-primary-100 border-primary-500 text-primary-800 dark:bg-primary-900 dark:text-primary-200'
                    } else {
                      optClass +=
                        'bg-white dark:bg-gray-800 border-gray-200 dark:border-gray-700 text-gray-900 dark:text-gray-100 hover:border-primary-400 hover:bg-primary-50 dark:hover:bg-gray-700'
                    }

                    return (
                      <button
                        key={optIndex}
                        onClick={() => handleSelect(question.question_id, optIndex)}
                        className={optClass}
                        disabled={!!result}
                      >
                        <span className="font-semibold mr-2">
                          {String.fromCharCode(65 + optIndex)}.
                        </span>
                        {option}
                        {isCorrectOpt && ' ✓'}
                        {isWrongSelected && ' ✗'}
                      </button>
                    )
                  })}
                </div>

                {/* Explanation after submit */}
                {result && qResult?.explanation && (
                  <div className="mt-4 ml-11 p-3 bg-blue-50 dark:bg-blue-900/30 rounded-lg text-sm text-blue-800 dark:text-blue-200">
                    <span className="font-semibold">💡 Explanation: </span>
                    {qResult.explanation}
                  </div>
                )}
              </div>
            )
          })}
        </div>

        {/* Submit button */}
        {!result && totalQ > 0 && (
          <div className="mt-8 flex justify-end">
            <button
              onClick={handleSubmit}
              disabled={submitting || answeredCount < totalQ}
              className="btn-primary text-lg px-8 py-3 disabled:opacity-50"
            >
              {submitting
                ? 'Submitting…'
                : answeredCount < totalQ
                ? `Answer ${totalQ - answeredCount} more question${totalQ - answeredCount !== 1 ? 's' : ''}`
                : 'Submit Quiz ✓'}
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
