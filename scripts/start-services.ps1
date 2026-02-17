# Start all microservices
Write-Host "Starting SynapseAI microservices..." -ForegroundColor Green

$services = @(
    @{Name="Auth"; Path="services/auth/cmd"; Port=8001},
    @{Name="User"; Path="services/user/cmd"; Port=9002},
    @{Name="Content"; Path="services/content/cmd"; Port=8003},
    @{Name="AI"; Path="services/ai/cmd"; Port=9004},
    @{Name="Quiz"; Path="services/quiz/cmd"; Port=8005},
    @{Name="Progress"; Path="services/progress/cmd"; Port=9006},
    @{Name="Notification"; Path="services/notification/cmd"; Port=8007}
)

foreach ($service in $services) {
    Write-Host "`nStarting $($service.Name) service..." -ForegroundColor Cyan
    Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd $($service.Path); go run main.go" -WindowStyle Normal
    Start-Sleep -Seconds 1
}

Write-Host "`n✓ All services started!" -ForegroundColor Green
Write-Host "Check individual terminal windows for service logs" -ForegroundColor Yellow
