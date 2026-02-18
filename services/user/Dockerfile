FROM golang:1.23-alpine AS builder
ENV GOTOOLCHAIN=auto
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOTOOLCHAIN=auto go build -o /user-service ./services/user/cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /user-service .
EXPOSE 8002 9002
CMD ["./user-service"]
