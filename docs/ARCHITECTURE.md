# Architecture Deep Dive

## Clean Architecture Pattern

Each service follows Clean Architecture principles:

```
services/[service-name]/
├── cmd/
│   └── main.go              # Entry point
├── internal/
│   ├── domain/              # Business entities
│   │   └── models.go
│   ├── repository/          # Data access layer
│   │   └── repository.go
│   ├── service/             # Business logic
│   │   └── service.go
│   └── transport/           # Delivery layer
│       ├── http/
│       │   └── handler.go
│       └── grpc/
│           └── server.go
├── Dockerfile
└── .env.example
```

### Dependency Flow

```
Transport Layer (HTTP/gRPC)
    ↓
Service Layer (Business Logic)
    ↓
Repository Layer (Data Access)
    ↓
Database
```

**Key Principles:**
- Dependencies point inward
- Business logic is independent of frameworks
- Easy to test and mock
- Flexible to change databases or transport

## Communication Patterns

### 1. Synchronous (gRPC)

Used for:
- Service-to-service calls
- When immediate response needed
- Strong typing required

Example:
```go
// Auth service validates token
conn, _ := grpc.Dial("auth-service:9001")
client := authpb.NewAuthServiceClient(conn)
resp, _ := client.ValidateToken(ctx, &authpb.ValidateTokenRequest{
    Token: token,
})
```

### 2. Asynchronous (RabbitMQ)

Used for:
- Event-driven workflows
- Decoupled services
- Fire-and-forget operations

Example:
```go
// Content service publishes event
event := rabbitmq.ContentUploadedEvent{
    ContentID: "123",
    UserID: "456",
}
rmq.Publish(rabbitmq.ContentUploadedKey, event)

// AI service subscribes
rmq.Subscribe("ai_queue", rabbitmq.ContentUploadedKey, func(body []byte) error {
    var event rabbitmq.ContentUploadedEvent
    json.Unmarshal(body, &event)
    // Process event...
})
```

### 3. REST APIs (HTTP)

Used for:
- Frontend communication
- Public APIs
- Simple CRUD operations

## Database Design

### Per-Service Databases

Each microservice owns its database following the Database-per-Service pattern:

**Benefits:**
- Independent scaling
- Technology flexibility
- Fault isolation
- No shared schema coupling

### Schema Examples

**Auth DB:**
```sql
CREATE TABLE users (
    id VARCHAR(36) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    role VARCHAR(50) DEFAULT 'user',
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE refresh_tokens (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    token TEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL
);
```

**Content DB:**
```sql
CREATE TABLE content (
    content_id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    title VARCHAR(255) NOT NULL,
    file_path TEXT NOT NULL,
    file_type VARCHAR(100),
    file_size BIGINT,
    description TEXT,
    uploaded_at TIMESTAMP NOT NULL
);
```

**Quiz DB:**
```sql
CREATE TABLE quizzes (
    quiz_id VARCHAR(36) PRIMARY KEY,
    content_id VARCHAR(36) NOT NULL,
    title VARCHAR(255) NOT NULL,
    difficulty VARCHAR(50),
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE questions (
    question_id VARCHAR(36) PRIMARY KEY,
    quiz_id VARCHAR(36) NOT NULL,
    question_text TEXT NOT NULL,
    options JSON,
    correct_option INT NOT NULL,
    explanation TEXT
);

CREATE TABLE attempts (
    attempt_id VARCHAR(36) PRIMARY KEY,
    quiz_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    score INT NOT NULL,
    percentage FLOAT NOT NULL,
    passed BOOLEAN NOT NULL,
    attempted_at TIMESTAMP NOT NULL
);
```

**Progress DB:**
```sql
CREATE TABLE user_progress (
    user_id VARCHAR(36) PRIMARY KEY,
    total_xp INT DEFAULT 0,
    level INT DEFAULT 1,
    current_streak INT DEFAULT 0,
    longest_streak INT DEFAULT 0,
    quizzes_completed INT DEFAULT 0,
    content_uploaded INT DEFAULT 0,
    last_activity TIMESTAMP DEFAULT NOW()
);

CREATE TABLE xp_logs (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    xp_amount INT NOT NULL,
    activity_type VARCHAR(50),
    reference_id VARCHAR(36),
    created_at TIMESTAMP NOT NULL
);
```

## Event-Driven Architecture

### RabbitMQ Setup

**Exchange Type:** Topic
**Exchange Name:** synapseai

### Event Definitions

```go
// ContentUploaded
{
    "content_id": "uuid",
    "user_id": "uuid",
    "title": "string",
    "file_path": "string",
    "file_type": "string",
    "timestamp": "RFC3339"
}

// QuizGenerated
{
    "quiz_id": "uuid",
    "content_id": "uuid",
    "user_id": "uuid",
    "title": "string",
    "num_questions": int,
    "timestamp": "RFC3339"
}

// QuizCompleted
{
    "attempt_id": "uuid",
    "quiz_id": "uuid",
    "user_id": "uuid",
    "score": int,
    "percentage": float,
    "passed": bool,
    "xp_earned": int,
    "timestamp": "RFC3339"
}
```

### Routing Keys

- `content.uploaded` - Content upload events
- `quiz.generated` - Quiz generation events
- `quiz.completed` - Quiz completion events

### Consumer Pattern

```go
func subscribeToEvents(rmq *rabbitmq.Client, handler Handler) {
    rmq.Subscribe("service_queue", "routing.key", func(body []byte) error {
        var event EventType
        if err := json.Unmarshal(body, &event); err != nil {
            return err
        }
        
        // Process event
        err := handler.Process(event)
        
        // Return error to requeue, nil to acknowledge
        return err
    })
}
```

## Security

### JWT Authentication

**Token Generation:**
```go
claims := &jwt.Claims{
    UserID: "user-id",
    Email: "user@example.com",
    Role: "user",
    RegisteredClaims: jwt.RegisteredClaims{
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(24*time.Hour)),
        IssuedAt: jwt.NewNumericDate(time.Now()),
    },
}
token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
tokenString, _ := token.SignedString([]byte(secretKey))
```

**Token Validation:**
```go
token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
    return []byte(secretKey), nil
})
```

**Middleware:**
```go
func AuthMiddleware(jwtManager *jwt.Manager) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            token := strings.TrimPrefix(authHeader, "Bearer ")
            
            claims, err := jwtManager.ValidateToken(token)
            if err != nil {
                http.Error(w, "unauthorized", http.StatusUnauthorized)
                return
            }
            
            ctx := context.WithValue(r.Context(), "user", claims)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### Password Hashing

```go
// Hash password
hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

// Verify password
err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
```

## Performance Optimization

### Redis Caching

```go
// Cache user data
func (s *UserService) GetUser(userID string) (*User, error) {
    // Try cache first
    cached, err := s.redis.Get(ctx, "user:"+userID).Result()
    if err == nil {
        var user User
        json.Unmarshal([]byte(cached), &user)
        return &user, nil
    }
    
    // Cache miss, fetch from DB
    user, err := s.repo.FindByID(userID)
    if err != nil {
        return nil, err
    }
    
    // Store in cache
    data, _ := json.Marshal(user)
    s.redis.Set(ctx, "user:"+userID, data, 1*time.Hour)
    
    return user, nil
}
```

### Connection Pooling

```go
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```

### Rate Limiting (Nginx)

```nginx
limit_req_zone $binary_remote_addr zone=auth:10m rate=5r/s;
limit_req_zone $binary_remote_addr zone=general:10m rate=10r/s;

location /api/auth/ {
    limit_req zone=auth burst=10 nodelay;
    # ...
}
```

## Monitoring & Observability

### Structured Logging

```go
logger.Info("User registered",
    zap.String("user_id", userID),
    zap.String("email", email),
)

logger.Error("Database connection failed",
    zap.Error(err),
    zap.String("host", dbHost),
)
```

### Health Checks

```go
func healthCheck(w http.ResponseWriter, r *http.Request) {
    // Check database
    if err := db.Ping(); err != nil {
        w.WriteHeader(http.StatusServiceUnavailable)
        return
    }
    
    // Check Redis
    if err := redis.Ping(ctx).Err(); err != nil {
        w.WriteHeader(http.StatusServiceUnavailable)
        return
    }
    
    json.NewEncoder(w).Encode(map[string]string{
        "status": "healthy",
        "service": serviceName,
    })
}
```

## Scaling Strategies

### Horizontal Scaling

```yaml
# docker-compose.yml
auth-service:
  build: ./services/auth
  deploy:
    replicas: 3
  # ...
```

### Load Balancing (Nginx)

```nginx
upstream auth_service {
    server auth-service-1:8001;
    server auth-service-2:8001;
    server auth-service-3:8001;
}
```

### Database Read Replicas

```go
// Master for writes
masterDB, _ := sql.Open("postgres", masterDSN)

// Slaves for reads
slaveDB1, _ := sql.Open("postgres", slave1DSN)
```

## Best Practices

1. **Always use context for cancellation**
2. **Implement graceful shutdown**
3. **Use prepared statements**
4. **Validate all inputs**
5. **Handle errors properly**
6. **Log with correlation IDs**
7. **Implement circuit breakers**
8. **Use timeouts everywhere**
9. **Monitor resource usage**
10. **Keep services loosely coupled**
