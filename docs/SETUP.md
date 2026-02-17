# SynapseAI - Quick Start Guide

## Prerequisites Installation

### 1. Install Go (1.21+)

**Windows:**
```powershell
winget install GoLang.Go
```

**Verify:**
```bash
go version
```

### 2. Install Protocol Buffers Compiler

**Windows:**
```powershell
# Download from https://github.com/protocolbuffers/protobuf/releases
# Extract and add to PATH
```

**Install Go plugins:**
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### 3. Install Docker Desktop

Download from: https://www.docker.com/products/docker-desktop/

### 4. Install Node.js (18+)

**Windows:**
```powershell
winget install OpenJS.NodeJS
```

## Project Setup

### Step 1: Clone and Navigate

```bash
cd c:\ACA\Projects\SynapseAI
```

### Step 2: Initialize Go Modules

```bash
go mod download
go mod tidy
```

### Step 3: Generate Proto Files

```bash
make proto
```

Or manually:
```bash
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/auth/*.proto
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/user/*.proto
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/content/*.proto
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/ai/*.proto
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/quiz/*.proto
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto/progress/*.proto
```

### Step 4: Start Infrastructure with Docker

```bash
docker-compose up --build -d
```

This will start:
- 5 PostgreSQL databases (one per service)
- Redis
- RabbitMQ
- All 7 microservices
- Nginx API Gateway

### Step 5: Verify Services

```bash
# Check all containers are running
docker ps

# Check service health
curl http://localhost/health
curl http://localhost:8001/health  # Auth service
curl http://localhost:8003/health  # Content service
curl http://localhost:8005/health  # Quiz service

# Check RabbitMQ
# Open browser: http://localhost:15672 (guest/guest)
```

### Step 6: Setup Frontend

```bash
cd frontend/web
npm install
npm run dev
```

Frontend will be available at: http://localhost:3000

## Testing the System

### 1. Register a User

```bash
curl -X POST http://localhost/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "Test123456!",
    "name": "Test User"
  }'
```

**Response:**
```json
{
  "access_token": "eyJhbGc...",
  "refresh_token": "eyJhbGc...",
  "user": {
    "id": "uuid-here",
    "email": "test@example.com",
    "name": "Test User",
    "role": "user"
  }
}
```

### 2. Login

```bash
curl -X POST http://localhost/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "Test123456!"
  }'
```

### 3. Upload Content

```bash
curl -X POST http://localhost/api/content/upload \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -F "file=@test.pdf" \
  -F "title=Introduction to Microservices"
```

This will:
- Upload the file
- Store metadata in database
- Publish `ContentUploaded` event to RabbitMQ
- AI Service will auto-generate a quiz
- Notification Service will send an email

## Service Ports Reference

| Service | HTTP | gRPC | Database |
|---------|------|------|----------|
| Auth | 8001 | 9001 | postgres-auth:5432 |
| User | 8002 | 9002 | postgres-user:5432 |
| Content | 8003 | 9003 | postgres-content:5432 |
| AI | - | 9004 | - |
| Quiz | 8005 | 9005 | postgres-quiz:5432 |
| Progress | - | 9006 | postgres-progress:5432 |
| Notification | 8007 | - | - |

**Infrastructure:**
- API Gateway: http://localhost:80
- Redis: localhost:6379
- RabbitMQ: localhost:5672
- RabbitMQ Management UI: http://localhost:15672

## Development Workflow

### Run Individual Service (Development)

```bash
cd services/auth
go run cmd/main.go
```

### View Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f auth-service
docker-compose logs -f rabbitmq
```

### Restart a Service

```bash
docker-compose restart auth-service
```

### Rebuild a Service

```bash
docker-compose up -d --build auth-service
```

### Stop All Services

```bash
docker-compose down
```

### Clean Everything (including volumes)

```bash
docker-compose down -v
```

## Event Flow Example

1. **User uploads content** → Content Service
2. Content Service publishes `ContentUploaded` event
3. **AI Service receives event** → Generates quiz
4. AI Service publishes `QuizGenerated` event
5. **Notification Service receives event** → Sends email
6. **User takes quiz** → Quiz Service
7. Quiz Service publishes `QuizCompleted` event
8. **Progress Service receives event** → Updates XP and streak
9. **Notification Service receives event** → Sends completion email

## Troubleshooting

### Proto Files Not Generated

```bash
# Check if protoc is installed
protoc --version

# Check if Go plugins are in PATH
which protoc-gen-go
which protoc-gen-go-grpc

# On Windows, add to PATH:
# %GOPATH%\bin or %USERPROFILE%\go\bin
```

### Service Won't Start

```bash
# Check database is ready
docker-compose logs postgres-auth

# Check RabbitMQ is ready
docker-compose logs rabbitmq

# Restart service
docker-compose restart auth-service
```

### Database Connection Error

```bash
# Verify environment variables
docker-compose exec auth-service env | grep DB_

# Check database is accessible
docker-compose exec postgres-auth psql -U synapseai -d auth_db -c "SELECT 1;"
```

### RabbitMQ Events Not Working

```bash
# Check RabbitMQ status
docker-compose exec rabbitmq rabbitmqctl status

# View queues
# Open http://localhost:15672 and check Queues tab
```

## Production Deployment Checklist

- [ ] Change all default passwords
- [ ] Set strong JWT_SECRET
- [ ] Configure proper CORS origins
- [ ] Enable HTTPS/TLS
- [ ] Set up proper logging (ELK/Loki)
- [ ] Configure monitoring (Prometheus/Grafana)
- [ ] Set up distributed tracing
- [ ] Implement rate limiting
- [ ] Configure auto-scaling
- [ ] Set up CI/CD pipeline
- [ ] Configure database backups
- [ ] Use secrets management (Vault)
- [ ] Implement circuit breakers
- [ ] Add health checks to load balancer
- [ ] Configure proper PostgreSQL connection pooling
- [ ] Set up Redis persistence
- [ ] Configure RabbitMQ clustering

## API Documentation

See [README.md](../README.md) for full API documentation and examples.

## Architecture Diagrams

```
┌─────────────┐
│   Next.js   │
│   Frontend  │
└──────┬──────┘
       │ HTTP/REST
       ↓
┌─────────────┐
│    Nginx    │
│   Gateway   │
└──────┬──────┘
       │
       ├→ Auth Service (REST/gRPC)
       ├→ Content Service (REST/gRPC)
       └→ Quiz Service (REST/gRPC)
       
┌──────────────────────────┐
│      RabbitMQ Events     │
├──────────────────────────┤
│ ContentUploaded          │
│   ↓                      │
│ AI Service               │
│   ↓                      │
│ QuizGenerated            │
│   ↓                      │
│ Notification Service     │
│                          │
│ QuizCompleted            │
│   ↓                      │
│ Progress Service         │
│ Notification Service     │
└──────────────────────────┘
```

## Support

For issues or questions, check the logs first:
```bash
docker-compose logs -f [service-name]
```
