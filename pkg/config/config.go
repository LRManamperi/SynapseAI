package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// Redis
	RedisHost string
	RedisPort string

	// RabbitMQ
	RabbitMQURL string

	// JWT
	JWTSecret string
	JWTExpiry time.Duration

	// Server
	ServerPort string
	GRPCPort   string

	// Service specific
	ServiceName string
	Environment string

	// OpenAI (for AI service)
	OpenAIAPIKey string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Try to load .env file (ignore error if not found)
	_ = godotenv.Load()

	cfg := &Config{
		// Database
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "synapseai"),
		DBPassword: getEnv("DB_PASSWORD", "synapseai123"),
		DBName:     getEnv("DB_NAME", "synapseai_db"),

		// Redis
		RedisHost: getEnv("REDIS_HOST", "localhost"),
		RedisPort: getEnv("REDIS_PORT", "6379"),

		// RabbitMQ
		RabbitMQURL: getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),

		// JWT
		JWTSecret: getEnv("JWT_SECRET", "your-super-secret-jwt-key-change-in-production"),
		JWTExpiry: getEnvDuration("JWT_EXPIRY", 24*time.Hour),

		// Server
		ServerPort: getEnv("SERVER_PORT", "8080"),
		GRPCPort:   getEnv("GRPC_PORT", "9090"),

		// Service
		ServiceName: getEnv("SERVICE_NAME", "unknown"),
		Environment: getEnv("ENVIRONMENT", "development"),

		// OpenAI
		OpenAIAPIKey: getEnv("OPENAI_API_KEY", ""),
	}

	return cfg, nil
}

// DatabaseDSN returns PostgreSQL connection string
func (c *Config) DatabaseDSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName,
	)
}

// RedisAddr returns Redis address
func (c *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}

	return duration
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intValue
}
