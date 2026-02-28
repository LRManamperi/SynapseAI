# Quick Start Script for SynapseAI Platform
# This script starts the infrastructure and services properly

Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "   SynapseAI Platform - Quick Start" -ForegroundColor Cyan
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host ""

# Check if Docker is running
Write-Host "Checking Docker..." -ForegroundColor Yellow
try {
    docker ps | Out-Null
    Write-Host "✓ Docker is running" -ForegroundColor Green
} catch {
    Write-Host "✗ Docker is not running. Please start Docker Desktop first." -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "Starting infrastructure services..." -ForegroundColor Yellow
Write-Host "  - PostgreSQL databases" -ForegroundColor Gray
Write-Host "  - Redis cache" -ForegroundColor Gray
Write-Host "  - RabbitMQ message broker" -ForegroundColor Gray
Write-Host ""

# Start only infrastructure services (not the microservices yet)
docker-compose up -d postgres-auth postgres-user postgres-content postgres-quiz postgres-progress redis rabbitmq

Write-Host ""
Write-Host "Waiting for services to be healthy..." -ForegroundColor Yellow
Start-Sleep -Seconds 10

# Check infrastructure health
Write-Host ""
Write-Host "Checking infrastructure health:" -ForegroundColor Yellow

$healthy = $true

# Check PostgreSQL
try {
    docker exec postgres-auth pg_isready -U synapseai | Out-Null
    Write-Host "  ✓ PostgreSQL (Auth DB) - Ready" -ForegroundColor Green
} catch {
    Write-Host "  ✗ PostgreSQL (Auth DB) - Not Ready" -ForegroundColor Red
    $healthy = $false
}

# Check Redis
try {
    docker exec synapseai-redis redis-cli ping | Out-Null
    Write-Host "  ✓ Redis - Ready" -ForegroundColor Green
} catch {
    Write-Host "  ✗ Redis - Not Ready" -ForegroundColor Red
    $healthy = $false
}

# Check RabbitMQ
try {
    docker exec synapseai-rabbitmq rabbitmq-diagnostics ping | Out-Null
    Write-Host "  ✓ RabbitMQ - Ready" -ForegroundColor Green
} catch {
    Write-Host "  ✗ RabbitMQ - Not Ready" -ForegroundColor Red
    $healthy = $false
}

if (-not $healthy) {
    Write-Host ""
    Write-Host "Some services are not healthy. Waiting longer..." -ForegroundColor Yellow
    Start-Sleep -Seconds 15
}

Write-Host ""
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host "   Infrastructure is ready!" -ForegroundColor Green
Write-Host "==================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "You can now:" -ForegroundColor Yellow
Write-Host "  1. Run individual services with: cd services/auth/cmd; go run main.go" -ForegroundColor White
Write-Host "  2. Or start all services with Docker: docker-compose up -d" -ForegroundColor White
Write-Host "  3. Or use the start-services.ps1 script" -ForegroundColor White
Write-Host ""
Write-Host "Infrastructure endpoints:" -ForegroundColor Yellow
Write-Host "  PostgreSQL: localhost:5432 (user: synapseai, pass: synapseai123)" -ForegroundColor White
Write-Host "  Redis:      localhost:6379" -ForegroundColor White
Write-Host "  RabbitMQ:   localhost:15672 (http://localhost:15672)" -ForegroundColor White
Write-Host ""
Write-Host "To stop infrastructure: docker-compose down" -ForegroundColor Gray
Write-Host ""
