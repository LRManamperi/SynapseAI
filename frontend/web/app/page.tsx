import Link from 'next/link'

export default function Home() {
  return (
    <main className="min-h-screen bg-gradient-to-br from-primary-50 to-blue-100 dark:from-gray-900 dark:to-gray-800">
      <div className="container mx-auto px-4 py-16">
        {/* Hero Section */}
        <div className="text-center mb-16">
          <h1 className="text-6xl font-bold text-gray-900 dark:text-white mb-4">
            Welcome to <span className="text-primary-600">SynapseAI</span>
          </h1>
          <p className="text-xl text-gray-600 dark:text-gray-300 mb-8">
            Transform your learning with AI-powered quizzes and personalized insights
          </p>
          <div className="flex gap-4 justify-center">
            <Link href="/register" className="btn-primary text-lg px-8 py-3">
              Get Started
            </Link>
            <Link href="/login" className="btn-secondary text-lg px-8 py-3">
              Sign In
            </Link>
          </div>
        </div>

        {/* Features */}
        <div className="grid md:grid-cols-3 gap-8 mt-16">
          <div className="card text-center">
            <div className="text-4xl mb-4">📚</div>
            <h3 className="text-xl font-semibold mb-2">Upload Content</h3>
            <p className="text-gray-600 dark:text-gray-400">
              Upload PDFs and documents to generate personalized learning materials
            </p>
          </div>

          <div className="card text-center">
            <div className="text-4xl mb-4">🤖</div>
            <h3 className="text-xl font-semibold mb-2">AI-Powered Quizzes</h3>
            <p className="text-gray-600 dark:text-gray-400">
              Automatically generate quizzes and flashcards from your content
            </p>
          </div>

          <div className="card text-center">
            <div className="text-4xl mb-4">📊</div>
            <h3 className="text-xl font-semibold mb-2">Track Progress</h3>
            <p className="text-gray-600 dark:text-gray-400">
              Monitor your learning journey with XP, streaks, and achievements
            </p>
          </div>
        </div>

        {/* Stats */}
        <div className="mt-16 grid grid-cols-3 gap-8 text-center">
          <div>
            <div className="text-4xl font-bold text-primary-600">10K+</div>
            <div className="text-gray-600 dark:text-gray-400">Active Learners</div>
          </div>
          <div>
            <div className="text-4xl font-bold text-primary-600">50K+</div>
            <div className="text-gray-600 dark:text-gray-400">Quizzes Generated</div>
          </div>
          <div>
            <div className="text-4xl font-bold text-primary-600">95%</div>
            <div className="text-gray-600 dark:text-gray-400">Success Rate</div>
          </div>
        </div>
      </div>
    </main>
  )
}
