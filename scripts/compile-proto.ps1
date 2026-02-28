# Compile Protocol Buffer Files
Write-Host "Compiling protocol buffer files..." -ForegroundColor Green

$protoFiles = @(
    "auth",
    "user",
    "content",
    "ai",
    "quiz",
    "progress"
)

foreach ($proto in $protoFiles) {
    Write-Host "Compiling $proto.proto..." -ForegroundColor Cyan
    protoc --go_out=. --go_opt=paths=source_relative `
           --go-grpc_out=. --go-grpc_opt=paths=source_relative `
           proto/$proto/$proto.proto
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "  Success: $proto.proto compiled successfully" -ForegroundColor Green
    } else {
        Write-Host "  Error: Failed to compile $proto.proto" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "Proto compilation complete!" -ForegroundColor Green
