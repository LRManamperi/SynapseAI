package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/rs/cors"
	"github.com/synapseai/platform/pkg/config"
	"github.com/synapseai/platform/pkg/jwt"
	"github.com/synapseai/platform/pkg/logger"
	pb "github.com/synapseai/platform/proto/auth"
	"github.com/synapseai/platform/services/auth/internal/repository"
	"github.com/synapseai/platform/services/auth/internal/service"
	grpcTransport "github.com/synapseai/platform/services/auth/internal/transport/grpc"
	httpTransport "github.com/synapseai/platform/services/auth/internal/transport/http"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Initialize logger
	err = logger.Init("auth-service", cfg.Environment)
	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}
	defer logger.Sync()

	// Connect to database
	db, err := sql.Open("postgres", cfg.DatabaseDSN())
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		logger.Fatal("Failed to ping database", zap.Error(err), zap.String("dsn", cfg.DatabaseDSN()))
	}

	// Initialize repository
	userRepo := repository.NewUserRepository(db)
	
	// Initialize database schema
	if err := userRepo.InitSchema(); err != nil {
		logger.Fatal("Failed to initialize database schema", zap.Error(err))
	}

	// Initialize JWT manager
	jwtManager := jwt.NewManager(cfg.JWTSecret, cfg.JWTExpiry)

	// Initialize service
	authService := service.NewAuthService(userRepo, jwtManager)

	// Start gRPC server
	go startGRPCServer(cfg.GRPCPort, authService)

	// Start HTTP server
	startHTTPServer(cfg.ServerPort, authService)
}

func startGRPCServer(port string, authService *service.AuthService) {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Fatal("Failed to listen for gRPC")
	}

	grpcServer := grpc.NewServer()
	pb.RegisterAuthServiceServer(grpcServer, grpcTransport.NewServer(authService))

	logger.Info(fmt.Sprintf("gRPC server listening on port %s", port))
	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatal("Failed to serve gRPC")
	}
}

func startHTTPServer(port string, authService *service.AuthService) {
	r := mux.NewRouter()
	handler := httpTransport.NewHandler(authService)
	handler.SetupRoutes(r)

	// CORS middleware
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	logger.Info(fmt.Sprintf("HTTP server listening on port %s", port))
	if err := http.ListenAndServe(":"+port, c.Handler(r)); err != nil {
		logger.Fatal("Failed to serve HTTP")
	}
}
