#!/usr/bin/env pwsh
# Frontend Setup Script

Write-Host "Setting up SynapseAI Frontend..." -ForegroundColor Green

# Check if Node.js is installed
if (-not (Get-Command node -ErrorAction SilentlyContinue)) {
    Write-Host "❌ Node.js is not installed. Please install Node.js 18+ first." -ForegroundColor Red
    exit 1
}

Write-Host "✓ Node.js version: $(node --version)" -ForegroundColor Cyan

# Check if npm is installed
if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    Write-Host "❌ npm is not installed." -ForegroundColor Red
    exit 1
}

Write-Host "✓ npm version: $(npm --version)" -ForegroundColor Cyan

# Navigate to frontend directory
Set-Location -Path "frontend\web"

# Install dependencies
Write-Host "`nInstalling dependencies..." -ForegroundColor Yellow
npm install

if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Failed to install dependencies" -ForegroundColor Red
    exit 1
}

Write-Host "✓ Dependencies installed successfully!" -ForegroundColor Green

# Check if .env.local exists
if (-not (Test-Path ".env.local")) {
    Write-Host "`n⚠ Warning: .env.local not found" -ForegroundColor Yellow
    Write-Host "Creating .env.local..." -ForegroundColor Yellow
    @"
NEXT_PUBLIC_API_URL=http://localhost:80

# Development
NODE_ENV=development
"@ | Out-File -FilePath ".env.local" -Encoding utf8
    Write-Host "✓ Created .env.local" -ForegroundColor Green
}

# Build check (optional)
Write-Host "`nChecking if build works..." -ForegroundColor Yellow
npm run build

if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ Build successful!" -ForegroundColor Green
} else {
    Write-Host "⚠ Build failed - this may be okay if backend is not running yet" -ForegroundColor Yellow
}

Write-Host "`n✅ Frontend setup complete!" -ForegroundColor Green
Write-Host "`nTo start the development server, run:" -ForegroundColor Cyan
Write-Host "  cd frontend\web" -ForegroundColor White
Write-Host "  npm run dev" -ForegroundColor White
Write-Host "`nThe frontend will be available at: http://localhost:3000" -ForegroundColor Cyan
