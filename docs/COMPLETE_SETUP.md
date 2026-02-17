# Complete Setup Guide - SynapseAI Platform

This guide walks you through setting up the entire SynapseAI platform from scratch.

## Prerequisites

- **Windows 10/11** with PowerShell
- **Docker Desktop** installed and running
- **Go 1.21+** installed
- **Node.js 18+** and npm installed
- **Git** installed
- **Protocol Buffers compiler (protoc)** installed

### Install Protocol Buffers Compiler

Download from: https://github.com/protocolbuffers/protobuf/releases

Or use Chocolatey:
```powershell
choco install protoc
```

---

## Step 1: Clone and Navigate to Project

```powershell
cd c:\ACA\Projects\SynapseAI
```

---

## Step 2: Backend Setup

### 2.1 Install Go Protobuf Plugins

```powershell
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### 2.2 Download Go Dependencies

```powershell
go work sync
go mod download
```

This will download all required Go packages:
- gRPC
- Protocol Buffers
- PostgreSQL driver
- JWT library
- RabbitMQ client
- Zap logger
- And more...

### 2.3 Compile Protocol Buffer Files

```powershell
.\scripts\compile-proto.ps1
```

Or using Make:
```powershell
make proto
```

This generates Go code from `.proto` files for all 6 gRPC services.

---

## Step 3: Infrastructure Setup

### 3.1 Start Docker Services

```powershell
docker-compose up -d
```

This starts:
- **PostgreSQL** (5 databases for services)
- **Redis** (caching)
- **RabbitMQ** (message broker)
- **Nginx** (API gateway)

### 3.2 Verify Services are Running

```powershell
docker-compose ps
```

All services should show as "Up".

### 3.3 Check Service Health

- RabbitMQ Management UI: http://localhost:15672 (guest/guest)
- PostgreSQL: `localhost:5432`
- Redis: `localhost:6379`
- Nginx Gateway: http://localhost:80

---

## Step 4: Database Initialization

The databases are automatically created by Docker Compose:
- `auth_db` - Authentication data
- `user_db` - User profiles and preferences
- `content_db` - Uploaded content metadata
- `quiz_db` - Quizzes and attempts
- `progress_db` - XP, levels, streaks

Tables will be created automatically on first service startup.

---

## Step 5: Start Backend Services

### Option A: Using PowerShell Script (Recommended)

```powershell
.\scripts\start-services.ps1
```

This opens a terminal window for each service for easy monitoring.

### Option B: Manual Start (for debugging)

Open 7 PowerShell windows and run:

**Window 1 - Auth Service:**
```powershell
cd services\auth\cmd
go run main.go
```

**Window 2 - User Service:**
```powershell
cd services\user\cmd
go run main.go
```

**Window 3 - Content Service:**
```powershell
cd services\content\cmd
go run main.go
```

**Window 4 - AI Service:**
```powershell
cd services\ai\cmd
go run main.go
```

**Window 5 - Quiz Service:**
```powershell
cd services\quiz\cmd
go run main.go
```

**Window 6 - Progress Service:**
```powershell
cd services\progress\cmd
go run main.go
```

**Window 7 - Notification Service:**
```powershell
cd services\notification\cmd
go run main.go
```

### Verify Services Started

Each service should log:
```
✓ Database connected
✓ gRPC server listening on port XXXX
✓ HTTP server listening on port XXXX (if applicable)
✓ Connected to RabbitMQ
```

---

## Step 6: Frontend Setup

### 6.1 Navigate to Frontend Directory

```powershell
cd frontend\web
```

### 6.2 Install Node Dependencies

```powershell
npm install
```

This installs:
- Next.js 14
- React 18
- TypeScript
- Tailwind CSS
- Axios
- Zustand

### 6.3 Start Development Server

```powershell
npm run dev
```

Frontend will be available at: **http://localhost:3000**

---

## Step 7: Test the Platform

### 7.1 Open Frontend

Navigate to: http://localhost:3000

### 7.2 Register a New User

1. Click "Get Started" or "Sign Up"
2. Fill in the registration form:
   - Name: Test User
   - Email: test@example.com
   - Password: password123
3. Click "Create Account"

You should be automatically logged in and redirected to the dashboard.

### 7.3 Upload Content

1. Click "Upload Content" from dashboard
2. Enter a title: "Sample Document"
3. Select a PDF file
4. Click "Upload Content"

The AI service will automatically:
- Generate a summary
- Create quiz questions
- Generate flashcards

### 7.4 Check Progress

Go to Dashboard to see:
- Content uploaded count
- XP earned
- Streaks
- Quizzes completed

---

## Step 8: Verify Event-Driven Communication

### 8.1 Check RabbitMQ

Open: http://localhost:15672

Login with: guest/guest

Navigate to "Queues" tab to see:
- `content_uploaded_queue`
- `quiz_generated_queue`
- `quiz_completed_queue`

### 8.2 Monitor Service Logs

Watch the terminal windows to see event processing:

**Content Service**: Published `ContentUploaded` event
**AI Service**: Consumed `ContentUploaded` event, Published `QuizGenerated` event
**Notification Service**: Consumed `QuizGenerated` event, sent email

---

## Architecture Overview

```
┌─────────────────┐
│   Frontend      │
│   Next.js       │
│   Port 3000     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   Nginx Gateway │
│   Port 80       │
└────────┬────────┘
         │
    ┌────┴─────┬──────────┬──────────┬──────────┬──────────┬──────────┐
    ▼          ▼          ▼          ▼          ▼          ▼          ▼
┌───────┐  ┌───────┐  ┌───────┐  ┌───────┐  ┌───────┐  ┌──────┐  ┌────────┐
│ Auth  │  │ User  │  │Content│  │  AI   │  │ Quiz  │  │Progress│ │Notify  │
│ :8001 │  │ :9002 │  │ :8003 │  │ :9004 │  │ :8005 │  │ :9006 │  │ :8007  │
└───┬───┘  └───┬───┘  └───┬───┘  └───┬───┘  └───┬───┘  └───┬───┘  └───┬────┘
    │          │          │          │          │          │          │
    └──────────┴──────────┴──────────┴──────────┴──────────┴──────────┘
                          │                     │
    ┌─────────────────────┴─────────────────────┴─────────────────────┐
    │                                                                   │
    ▼                          ▼                          ▼            ▼
┌────────┐              ┌──────────┐              ┌───────┐      ┌────────┐
│Postgres│              │RabbitMQ  │              │ Redis │      │  Logs  │
│5 DBs   │              │Topic Exch│              │Cache  │      │  Zap   │
└────────┘              └──────────┘              └───────┘      └────────┘
```

---

## Environment Variables

Each service uses environment variables from `.env` or Docker Compose.

**Key Variables:**
- `DB_HOST` - Database host
- `DB_NAME` - Database name
- `RABBITMQ_URL` - RabbitMQ connection string
- `REDIS_HOST` - Redis host
- `JWT_SECRET` - JWT signing key
- `OPENAI_API_KEY` - OpenAI API key (for AI service)

---

## Common Issues and Solutions

### Issue: "Cannot find module" errors in frontend

**Solution:**
```powershell
cd frontend\web
rm -rf node_modules package-lock.json
npm install
```

### Issue: Proto compilation fails

**Solution:**
```powershell
# Verify protoc is installed
protoc --version

# Reinstall plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Add Go bin to PATH
$env:PATH += ";$env:USERPROFILE\go\bin"
```

### Issue: Service won't start - "port already in use"

**Solution:**
```powershell
# Find process using port (e.g., 8001)
netstat -ano | findstr :8001

# Kill the process
taskkill /PID <PID> /F
```

### Issue: Docker containers won't start

**Solution:**
```powershell
docker-compose down -v
docker-compose up -d --force-recreate
```

### Issue: "could not import" errors in Go

**Solution:**
```powershell
go work sync
go mod download
go mod tidy
```

### Issue: RabbitMQ connection refused

**Solution:**
```powershell
# Restart RabbitMQ container
docker-compose restart rabbitmq

# Wait for RabbitMQ to be ready (check logs)
docker-compose logs -f rabbitmq
```

---

## Stopping the Platform

### Stop Frontend
Press `Ctrl+C` in the terminal running `npm run dev`

### Stop Backend Services
Press `Ctrl+C` in each service terminal window

Or close the PowerShell windows

### Stop Infrastructure
```powershell
docker-compose down
```

To remove volumes (databases):
```powershell
docker-compose down -v
```

---

## Production Deployment

For production, you'll need:

1. **Update Environment Variables**
   - Change all default passwords
   - Set strong JWT secret
   - Configure real SMTP for notifications
   - Add OpenAI API key

2. **Build Binaries**
   ```powershell
   make build
   ```

3. **Use Production Database**
   - PostgreSQL cluster
   - Backup strategy
   - Connection pooling

4. **Configure HTTPS**
   - SSL certificates
   - Update Nginx config

5. **Deploy to Cloud**
   - Kubernetes for orchestration
   - AWS / GCP / Azure
   - CI/CD pipelines

6. **Monitoring**
   - Prometheus + Grafana
   - Log aggregation (ELK)
   - Distributed tracing (Jaeger)

---

## Next Steps

1. ✅ Platform is running
2. 📚 Read [ARCHITECTURE.md](ARCHITECTURE.md) for design details
3. 🔧 Read [API.md](API.md) for endpoint documentation
4. 🐛 Report issues on GitHub
5. 🚀 Start building features!

---

## Support

- **Documentation**: `/docs` folder
- **Backend Bugs**: See [BUGFIXES.md](BUGFIXES.md)
- **Frontend Issues**: See [FRONTEND_FIXES.md](FRONTEND_FIXES.md)
- **Architecture**: See [ARCHITECTURE.md](ARCHITECTURE.md)

---

**Congratulations! Your SynapseAI platform is now running! 🎉**
