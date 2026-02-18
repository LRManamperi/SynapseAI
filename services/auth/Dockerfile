FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

# Set GOTOOLCHAIN to auto to allow using newer Go versions
ENV GOTOOLCHAIN=auto

RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOTOOLCHAIN=auto go build -o /auth-service ./services/auth/cmd/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /auth-service .

EXPOSE 8001 9001

CMD ["./auth-service"]
