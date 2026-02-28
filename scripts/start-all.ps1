# Complete Startup Script for SynapseAI Platform
# This script starts everything: infrastructure, backend services, and frontend

Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "   SynapseAI Platform - Complete Startup" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host ""

# Check if Docker is running
Write-Host "[1/4] Checking Docker..." -ForegroundColor Yellow
try {
    docker ps | Out-Null
    Write-Host "      ✓ Docker is running" -ForegroundColor Green
} catch {
    Write-Host "      ✗ Docker is not running. Please start Docker Desktop first." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "[2/4] Starting infrastructure services..." -ForegroundColor Yellow
Write-Host "      - PostgreSQL databases (5 instances)" -ForegroundColor Gray
Write-Host "      - Redis cache" -ForegroundColor Gray
Write-Host "      - RabbitMQ message broker" -ForegroundColor Gray

# Start infrastructure
docker-compose up -d postgres-auth postgres-user postgres-content postgres-quiz postgres-progress redis rabbitmq 2>&1 | Out-Null

Write-Host "      Waiting for services to initialize..." -ForegroundColor Gray
Start-Sleep -Seconds 15

Write-Host "      ✓ Infrastructure started" -ForegroundColor Green

Write-Host ""
Write-Host "[3/4] Starting backend microservices..." -ForegroundColor Yellow

# Start all microservices using docker-compose
docker-compose up -d auth-service user-service content-service ai-service quiz-service progress-service notification-service 2>&1 | Out-Null

Write-Host "      Waiting for services to initialize..." -ForegroundColor Gray
Start-Sleep -Seconds 10

Write-Host "      ✓ Backend services started" -ForegroundColor Green

Write-Host ""
Write-Host "[4/4] Starting frontend..." -ForegroundColor Yellow
Write-Host "      Change to frontend directory and run: npm run dev" -ForegroundColor Gray
Write-Host ""

# Show status
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "   Checking service status..." -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host ""

docker-compose ps

Write-Host ""
Write-Host "==================================================" -ForegroundColor Green
Write-Host "   Platform is running!" -ForegroundColor Green
Write-Host "==================================================" -ForegroundColor Green
Write-Host ""
Write-Host "Service Endpoints:" -ForegroundColor Yellow
Write-Host "  Auth Service:         http://localhost:8001" -ForegroundColor White
Write-Host "  User Service:         http://localhost:8002" -ForegroundColor White
Write-Host "  Content Service:      http://localhost:8003" -ForegroundColor White
Write-Host "  Quiz Service:         http://localhost:8005" -ForegroundColor White
Write-Host "  API Gateway (Nginx):  http://localhost:80" -ForegroundColor White
Write-Host ""
Write-Host "Infrastructure:" -ForegroundColor Yellow
Write-Host "  PostgreSQL:           localhost:5432" -ForegroundColor White
Write-Host "  Redis:                localhost:6379" -ForegroundColor White
Write-Host "  RabbitMQ Management:  http://localhost:15672" -ForegroundColor White
Write-Host ""
Write-Host "Next Steps:" -ForegroundColor Yellow
Write-Host "  1. Start frontend: cd frontend\web; npm run dev" -ForegroundColor White
Write-Host "  2. Access frontend: http://localhost:3000" -ForegroundColor White
Write-Host "  3. View logs: docker-compose logs -f [service-name]" -ForegroundColor White
Write-Host "  4. Stop all: docker-compose down" -ForegroundColor White
Write-Host ""
