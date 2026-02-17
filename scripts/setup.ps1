# Build and Setup Scripts

## setup.ps1 - Initial project setup
Write-Host "Setting up SynapseAI Platform..." -ForegroundColor Green

# Download Go dependencies
Write-Host "`nDownloading Go dependencies..." -ForegroundColor Yellow
go work sync
go mod download

# Install protoc-gen-go if not installed
Write-Host "`nChecking for protoc plugins..." -ForegroundColor Yellow
if (-not (Get-Command protoc-gen-go -ErrorAction SilentlyContinue)) {
    Write-Host "Installing protoc-gen-go..." -ForegroundColor Cyan
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
}
if (-not (Get-Command protoc-gen-go-grpc -ErrorAction SilentlyContinue)) {
    Write-Host "Installing protoc-gen-go-grpc..." -ForegroundColor Cyan
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
}

# Compile proto files
Write-Host "`nCompiling protocol buffer files..." -ForegroundColor Yellow
.\scripts\compile-proto.ps1

Write-Host "`nSetup complete!" -ForegroundColor Green
Write-Host "Next steps:" -ForegroundColor Cyan
Write-Host "1. Run 'docker-compose up -d' to start infrastructure"
Write-Host "2. Run '.\scripts\start-services.ps1' to start all services"
