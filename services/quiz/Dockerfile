FROM golang:1.23-alpine AS builder
ENV GOTOOLCHAIN=auto
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOTOOLCHAIN=auto go build -o /quiz-service ./services/quiz/cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /quiz-service .
EXPOSE 8005 9005
CMD ["./quiz-service"]
