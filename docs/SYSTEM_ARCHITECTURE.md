# SynapseAI — Full System Architecture

## 1. Overview

SynapseAI is an AI-powered adaptive learning platform built on a **microservices architecture**. Students upload study documents (PDFs, text files), the platform automatically extracts the content, sends it to a large-language model (Groq / LLaMA-3), and generates contextual multiple-choice quizzes. Students take quizzes, receive scored results with explanations, and their progress is tracked over time.

---

## 2. High-Level Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────────┐
│                          Browser (Next.js 14)                         │
│   /login  /register  /upload  /dashboard  /quizzes  /quizzes/[id]    │
└────────────────────────────┬─────────────────────────────────────────┘
                             │ HTTP (direct to services, port 80 via nginx)
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    API Gateway — Nginx (port 80)                      │
│  /api/auth/*  →  auth-service:8001                                   │
│  /api/content/* → content-service:8003                               │
│  /api/quiz/*  →  quiz-service:8005                                   │
│  Rate limiting: auth 5r/s · general 10r/s                            │
└──────┬──────────────────┬──────────────────┬───────────────────────-┘
       │                  │                  │
       ▼                  ▼                  ▼
┌────────────┐  ┌─────────────────┐  ┌───────────────┐
│auth-service│  │ content-service │  │  quiz-service │
│  :8001 HTTP│  │   :8003 HTTP    │  │   :8005 HTTP  │
│  :9001 gRPC│  │   :9003 gRPC    │  │   :9005 gRPC  │
└─────┬──────┘  └────────┬────────┘  └──────┬────────┘
      │                  │                   │
      ▼                  │  content.uploaded  │  quiz.generated
 postgres-auth     ──────┴────────────────►──┘
 postgres-user                │
                       RabbitMQ Exchange (topic: synapseai)
                              │
                    ┌─────────┴──────────┐
                    │                    │
                    ▼                    ▼
             ai-service           quiz-service
             :8004 HTTP            (quiz_generated_queue)
             :9004 gRPC             saves to postgres-quiz
             reads file from
             shared volume,
             calls Groq API,
             publishes quiz.generated
```

---

## 3. Services

### 3.1 Frontend — Next.js 14 (`frontend/web`)

| Property | Value |
|---|---|
| Framework | Next.js 14 (App Router) |
| Styling | Tailwind CSS |
| State management | Zustand (`authStore`) |
| HTTP client | Axios |
| Build mode | `output: standalone` |
| Start command | `node .next/standalone/server.js` |
| Port | 3000 |

**Pages:**
| Route | Purpose |
|---|---|
| `/login` | JWT login form |
| `/register` | Account creation |
| `/upload` | Document upload (multipart/form-data) |
| `/dashboard` | Overview |
| `/quizzes` | Lists all uploaded documents + their quizzes |
| `/quizzes/[quiz_id]` | Full quiz-taking UI with scoring and explanations |

**API base URLs (env-configurable):**
- `AUTH_API_URL` → `localhost:8001`
- `CONTENT_API_URL` → `localhost:8003`
- `QUIZ_API_URL` → `localhost:8005`
- `AI_API_URL` → `localhost:8004`
- `PROGRESS_API_URL` → `localhost:8006`

---

### 3.2 API Gateway — Nginx (`gateway/nginx/nginx.conf`)

| Feature | Detail |
|---|---|
| Container | `synapseai-gateway` |
| Port | `80` |
| Role | Reverse proxy + rate limiting |

**Route map:**
```
/api/auth/*     →  auth-service:8001    (rate: 5r/s, burst 10)
/api/content/*  →  content-service:8003 (100MB upload limit)
/api/quiz/*     →  quiz-service:8005    (rate: 10r/s, burst 15)
```

> **Note:** The frontend currently calls services directly (bypassing Nginx) for simplicity during development. Nginx is production-ready for unified routing.

---

### 3.3 Auth Service (`services/auth`)

| Property | Value |
|---|---|
| HTTP port | `8001` |
| gRPC port | `9001` |
| Database | `postgres-auth` → `auth_db` |
| Cache | Redis (token blacklist / refresh) |

**Endpoints:**
```
POST /register   — create account (bcrypt password hash)
POST /login      — returns access_token (JWT) + refresh_token
POST /logout     — invalidates token via Redis
POST /refresh    — issue new access_token from refresh_token
GET  /validate   — validate JWT (used by middleware)
GET  /health     — health check
```

**JWT claims:** `user_id`, `email`, `exp`  
**JWT secret:** configured via `JWT_SECRET` env var  
**JWT expiry:** 24h (configurable)

---

### 3.4 User Service (`services/user`)

| Property | Value |
|---|---|
| HTTP port | `8002` |
| gRPC port | `9002` |
| Database | `postgres-user` → `user_db` |

Manages user profiles, preferences, and avatar data independently from auth credentials.

---

### 3.5 Content Service (`services/content`)

| Property | Value |
|---|---|
| HTTP port | `8003` |
| gRPC port | `9003` |
| Database | `postgres-content` → `content_db` |
| File storage | Docker named volume `content-uploads` (`/root/uploads` inside container) |

**Endpoints:**
```
POST /upload     — multipart upload; saves file to volume; publishes content.uploaded event
GET  /list       — list user's documents (JWT required)
GET  /:id        — get document metadata
DELETE /:id      — delete document
```

**File naming convention:** `{content_id}_{original_filename}` stored in shared volume.

**RabbitMQ event published on upload:**
```json
{
  "content_id": "uuid",
  "user_id":    "uuid",
  "title":      "Lecture 3",
  "file_path":  "uploads/uuid_Lecture 3.pdf",
  "file_type":  "application/pdf",
  "timestamp":  "..."
}
```

---

### 3.6 AI Service (`services/ai`)

| Property | Value |
|---|---|
| HTTP port | `8004` |
| gRPC port | `9004` |
| LLM Provider | [Groq API](https://groq.com) |
| Model | `llama-3.3-70b-versatile` |
| API key env var | `OPENAI_API_KEY` (holds the Groq key) |
| Shared volume | `content-uploads` (same as content-service) |

**Flow triggered by `content.uploaded` event:**
1. Receive event from `ai_content_queue`
2. Resolve file path — try event path first, then scan `/root/uploads/{content_id}_*`
3. **PDF extraction** — `ledongthuc/pdf` library extracts plain text from PDF pages
4. Send extracted text + document title to Groq API (`/v1/chat/completions`)
5. Parse JSON response into 5 quiz questions
6. Publish `quiz.generated` event to RabbitMQ

**Groq prompt rules:**
- Questions must be based on specific facts in the document
- Options must be meaningful phrases — generic labels like "Option A" are forbidden
- `correct_option` is the 0-based index of the correct answer

**HTTP endpoints:**
```
GET  /health     — health check
POST /retrigger  — manually regenerate quiz for existing content
                   Body: { content_id, user_id, file_path?, title }
```

**Fallback:** If file is unreadable and content text is < 50 chars, placeholder questions are returned (no generic options — answers prompt user to review the document).

---

### 3.7 Quiz Service (`services/quiz`)

| Property | Value |
|---|---|
| HTTP port | `8005` |
| gRPC port | `9005` |
| Database | `postgres-quiz` → `quiz_db` |

**Database schema:**
```sql
quizzes    (quiz_id, content_id, title, difficulty, created_at)
questions  (question_id, quiz_id, question_text, options JSON, correct_option, explanation)
attempts   (attempt_id, quiz_id, user_id, score, percentage, passed, attempted_at)
```

**HTTP endpoints:**
```
GET  /health                 — health check
GET  /list?content_id=...    — list quizzes for a document
GET  /:quiz_id               — get full quiz with all questions + options
POST /:quiz_id/submit        — submit answers → returns { score, total, correct, results[] }
```

**Scoring logic:**
- Score = `(correct / total) * 100`
- Pass threshold = 70%
- XP earned = `(correct × 10) + 50 bonus if passed`

**Consumes:** `quiz_generated_queue` (routing key: `quiz.generated`)  
**Publishes:** `quiz.completed` event on each attempt submission

---

### 3.8 Progress Service (`services/progress`)

| Property | Value |
|---|---|
| gRPC port | `9006` |
| Database | `postgres-progress` → `progress_db` |

Consumes `quiz.completed` events to update streaks, XP totals, and leaderboard rankings.

---

### 3.9 Notification Service (`services/notification`)

| Property | Value |
|---|---|
| HTTP port | `8007` |

Consumes events from RabbitMQ and dispatches notifications (email, in-app) based on milestones or quiz results.

---

## 4. Infrastructure

### 4.1 Message Broker — RabbitMQ

| Property | Value |
|---|---|
| Container | `synapseai-rabbitmq` |
| AMQP port | `5672` |
| Management UI | `15672` (guest/guest) |
| Exchange | `synapseai` (topic type, durable) |

**Routing keys and queues:**

| Routing Key | Queue | Producer | Consumer |
|---|---|---|---|
| `content.uploaded` | `ai_content_queue` | content-service | ai-service |
| `quiz.generated` | `quiz_generated_queue` | ai-service | quiz-service |
| `quiz.completed` | *(progress/notification)* | quiz-service | progress-service, notification-service |

---

### 4.2 Databases — PostgreSQL 15

Each service has its own dedicated PostgreSQL instance (database-per-service pattern):

| Container | Database | Used by |
|---|---|---|
| `postgres-auth` | `auth_db` | auth-service |
| `postgres-user` | `user_db` | user-service |
| `postgres-content` | `content_db` | content-service |
| `postgres-quiz` | `quiz_db` | quiz-service |
| `postgres-progress` | `progress_db` | progress-service |

All use `lib/pq` Go driver. Authentication mode: **md5** (enforced via `docker/init-db.sh`).

---

### 4.3 Cache — Redis 7

| Container | `synapseai-redis` |
|---|---|
| Port | `6379` |
| Used by | auth-service (token blacklist, refresh token storage) |

---

### 4.4 File Storage

| Volume | `content-uploads` (Docker named volume) |
|---|---|
| Mounted at | `/root/uploads` (content-service and ai-service) |
| Naming | `{content_id}_{original_filename}` |
| Purpose | PDFs/text files shared between upload and AI extraction |

---

## 5. Communication Patterns

```
Synchronous (HTTP/gRPC):
  Browser  ──HTTP──►  auth-service
  Browser  ──HTTP──►  content-service
  Browser  ──HTTP──►  quiz-service
  Browser  ──HTTP──►  ai-service (/retrigger)

Asynchronous (RabbitMQ topic exchange):
  content-service  ──content.uploaded──►  ai-service
  ai-service       ──quiz.generated───►  quiz-service
  quiz-service     ──quiz.completed───►  progress-service
  quiz-service     ──quiz.completed───►  notification-service
```

---

## 6. Authentication & Security

- **JWT tokens** issued by auth-service, verified by each service independently via shared `JWT_SECRET`
- **Middleware** (`pkg/middleware/auth.go`) extracts and validates Bearer token on every protected route
- **CORS** — all services set `Access-Control-Allow-Origin: *` with explicit allowed methods/headers
- **Password hashing** — bcrypt in auth-service
- **Rate limiting** — Nginx enforces 5r/s on auth routes, 10r/s on general routes

---

## 7. Technology Stack

| Layer | Technology |
|---|---|
| Frontend | Next.js 14, React, TypeScript, Tailwind CSS, Zustand, Axios |
| Backend services | Go 1.24 |
| API framework | gorilla/mux (HTTP), google.golang.org/grpc (gRPC) |
| Service-to-service contracts | Protocol Buffers (proto3) |
| Database driver | lib/pq (PostgreSQL) |
| Message broker client | amqp091-go |
| CORS middleware | rs/cors |
| PDF extraction | ledongthuc/pdf |
| Logging | uber-go/zap |
| Config | joho/godotenv |
| LLM | Groq API — `llama-3.3-70b-versatile` (free tier) |
| Containerisation | Docker Compose (14 containers) |
| API Gateway | Nginx (alpine) |

---

## 8. Container Inventory

| Container | Image | Ports |
|---|---|---|
| `synapseai-gateway` | nginx:alpine | 80 |
| `auth-service` | custom Go | 8001, 9001 |
| `user-service` | custom Go | 8002, 9002 |
| `content-service` | custom Go | 8003, 9003 |
| `ai-service` | custom Go | 8004, 9004 |
| `quiz-service` | custom Go | 8005, 9005 |
| `progress-service` | custom Go | 9006 |
| `notification-service` | custom Go | 8007 |
| `postgres-auth` | postgres:15-alpine | 5432 |
| `postgres-user` | postgres:15-alpine | — |
| `postgres-content` | postgres:15-alpine | — |
| `postgres-quiz` | postgres:15-alpine | — |
| `postgres-progress` | postgres:15-alpine | — |
| `synapseai-redis` | redis:7-alpine | 6379 |
| `synapseai-rabbitmq` | rabbitmq:3-management-alpine | 5672, 15672 |

---

## 9. End-to-End User Flow

```
1. User registers / logs in
      → auth-service issues JWT

2. User uploads a PDF
      → content-service saves file to shared volume
      → publishes content.uploaded to RabbitMQ

3. AI pipeline triggers (async, ~3-5 seconds)
      → ai-service receives event
      → scans shared volume for file by content_id prefix
      → ledongthuc/pdf extracts text from all PDF pages
      → sends text + title to Groq API (llama-3.3-70b-versatile)
      → Groq returns 5 JSON-structured questions with options + explanations
      → ai-service publishes quiz.generated to RabbitMQ

4. Quiz is persisted
      → quiz-service receives quiz.generated event
      → stores quiz + questions (with JSON options) in postgres-quiz

5. User visits /quizzes
      → frontend lists all uploaded documents
      → fetches quiz count per document from quiz-service
      → shows Take Quiz button when quiz exists

6. User takes quiz at /quizzes/[quiz_id]
      → frontend fetches full quiz (questions + shuffled options)
      → user selects answers for all questions
      → POST /:quiz_id/submit → quiz-service scores answers
      → returns { score, total, correct, results[] with explanations }
      → score banner shown; correct/wrong highlighted

7. Progress updated
      → quiz-service publishes quiz.completed
      → progress-service updates XP, streak, leaderboard
```

---

---

# Challenges Encountered

## Infrastructure & Startup

| # | Challenge | Root Cause | Resolution |
|---|---|---|---|
| 1 | Auth service crashing on startup | Missing `.env` file — config loader called `log.Fatal` when env vars absent | Created `.env` with all required variables |
| 2 | PostgreSQL authentication failure | PostgreSQL 15 defaults to `scram-sha-256`; Go's `lib/pq` driver only supports `md5` | Created `docker/init-db.sh` that rewrites `pg_hba.conf` to use `md5` on container init |
| 3 | Services failing to wait for dependencies | `docker-compose up` starts containers in parallel; services launched before DBs were ready | Added `healthcheck` + `depends_on: condition: service_healthy` for all DB-dependent services |

---

## Routing

| # | Challenge | Root Cause | Resolution |
|---|---|---|---|
| 4 | `GET /list` returning 404 on content-service | `gorilla/mux.PathPrefix("/").Subrouter()` creates a double-slash match conflict | Changed to `router.NewRoute().Subrouter()` on content-service and progress-service |
| 5 | All quiz API calls blocked by CORS | quiz-service used `cors.Default()` which restricts origins | Replaced with explicit `cors.New(cors.Options{AllowedOrigins: ["*"], ...})` |
| 6 | AI service `/retrigger` blocked by CORS | ai-service had no CORS middleware at all | Added inline CORS handler wrapping the `http.ServeMux` |

---

## AI & Quiz Generation

| # | Challenge | Root Cause | Resolution |
|---|---|---|---|
| 7 | Questions were generic mocks not based on content | AI service used hardcoded fallback questions with no LLM integration | Completely rewrote ai-service with real Groq HTTP client |
| 8 | Paid xAI Grok API required | Original implementation targeted xAI's Grok API (paid) | Switched to free Groq API (`groq.com`) with `llama-3.3-70b-versatile` model |
| 9 | Groq returning generic "Option A / B / C / D" labels | Prompt did not forbid generic option text | Rewrote prompt with explicit rule: options must be meaningful specific phrases |
| 10 | PDF files producing garbled binary content | `os.ReadFile()` on a `.pdf` returns raw binary bytes that Groq can't process | Integrated `ledongthuc/pdf` pure-Go library to extract structured text page by page |
| 11 | Retrigger producing fallback questions for existing docs | `/retrigger` call from frontend sent no `file_path`; empty string passed to `readFileContent()` | Added `findFileByContentID()` which scans the uploads directory for `{content_id}_*` filename prefix |
| 12 | Quiz title stored as `"Quiz for <uuid>"` | `GenerateQuiz` hardcoded `fmt.Sprintf("Quiz for %s", req.ContentId)` | Refactored into `generateAndPublish()` that accepts a human-readable `title` parameter, passed from the content event's `Title` field |

---

## Frontend

| # | Challenge | Root Cause | Resolution |
|---|---|---|---|
| 13 | Quizzes page showed 0 quizzes from 5 documents | `quizzes/page.tsx` had a duplicate `export default` — old component body appended below the new one; old component gated display on `totalQuizzes === 0` | Removed entire duplicate trailing component section |
| 14 | "Take Quiz" button returning 404 | `/quizzes/[quiz_id]` dynamic route page did not exist | Created full quiz-taking page at `app/quizzes/[quiz_id]/page.tsx` |
| 15 | Quiz detail page still 404 after page creation | Next.js was running via `next start` which does not re-read the filesystem after standalone build | Switched to `node .next/standalone/server.js` with manual static asset copy |
| 16 | No API endpoint to score quiz submissions | quiz-service had no HTTP submit handler (only gRPC) | Added `POST /{quiz_id}/submit` HTTP handler to quiz-service with full scoring logic |
| 17 | Quiz option letters white/invisible on quiz cards | Option buttons had no explicit text color — inherited white in dark mode from parent | Added `text-gray-900 dark:text-gray-100` to option buttons, question text, and card titles |

---

## Development Environment

| # | Challenge | Root Cause | Resolution |
|---|---|---|---|
| 18 | Go workspace module resolution errors | Multi-module Go workspace with `go.work`; services reference shared `pkg/` via `replace` directive | Maintained `replace github.com/synapseai/platform => ../..` in each service's `go.mod` |
| 19 | Docker build pulling stale cached layers | Code changes not picked up after editing | Used `docker-compose build <service>` followed by `docker-compose up -d <service>` to force rebuild |
| 20 | Frontend static assets 404 after standalone build | `output: standalone` copies server code but not static files | Required manual `Copy-Item .next/static .next/standalone/.next/static` after every build |

Challenges — 20 total across 5 categories:

Infrastructure (3) — missing .env, PostgreSQL scram-sha-256 vs lib/pq, startup race conditions
Routing (3) — gorilla/mux double-slash bug, CORS on quiz-service, missing CORS on ai-service
AI & Quiz Generation (6) — mock questions, paid xAI → free Groq, generic option labels, PDF binary bytes, retrigger file path empty, quiz title showing UUID
Frontend (5) — duplicate export, missing dynamic route, standalone mode mismatch, no submit endpoint, white text in dark mode
Dev Environment (3) — Go workspace modules, Docker cache, Next.js static asset copy after standalone build