package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/synapseai/platform/pkg/config"
	"github.com/synapseai/platform/pkg/logger"
	"github.com/synapseai/platform/pkg/middleware"
	"github.com/synapseai/platform/pkg/rabbitmq"
	pb "github.com/synapseai/platform/proto/content"
	"google.golang.org/grpc"
)

type contentServer struct {
	pb.UnimplementedContentServiceServer
	db     *sql.DB
	rmq    *rabbitmq.Client
}

func (s *contentServer) GetContent(ctx context.Context, req *pb.GetContentRequest) (*pb.GetContentResponse, error) {
	var content pb.GetContentResponse
	query := `SELECT content_id, user_id, title, file_path, file_type, file_size, description, uploaded_at FROM content WHERE content_id = $1 AND user_id = $2`
	
	err := s.db.QueryRow(query, req.ContentId, req.UserId).Scan(
		&content.ContentId, &content.UserId, &content.Title, &content.FilePath,
		&content.FileType, &content.FileSize, &content.Description, &content.UploadedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &content, nil
}

func (s *contentServer) ListContent(ctx context.Context, req *pb.ListContentRequest) (*pb.ListContentResponse, error) {
	query := `SELECT content_id, title, file_type, file_size, uploaded_at FROM content WHERE user_id = $1 ORDER BY uploaded_at DESC LIMIT $2 OFFSET $3`
	
	offset := (req.Page - 1) * req.Limit
	rows, err := s.db.Query(query, req.UserId, req.Limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var items []*pb.ContentItem
	for rows.Next() {
		var item pb.ContentItem
		err := rows.Scan(&item.ContentId, &item.Title, &item.FileType, &item.FileSize, &item.UploadedAt)
		if err != nil {
			continue
		}
		items = append(items, &item)
	}
	
	var total int32
	s.db.QueryRow(`SELECT COUNT(*) FROM content WHERE user_id = $1`, req.UserId).Scan(&total)
	
	return &pb.ListContentResponse{Items: items, Total: total, Page: req.Page, Limit: req.Limit}, nil
}

func (s *contentServer) DeleteContent(ctx context.Context, req *pb.DeleteContentRequest) (*pb.DeleteContentResponse, error) {
	_, err := s.db.Exec(`DELETE FROM content WHERE content_id = $1 AND user_id = $2`, req.ContentId, req.UserId)
	if err != nil {
		return &pb.DeleteContentResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.DeleteContentResponse{Success: true, Message: "Content deleted"}, nil
}

func (s *contentServer) GetContentMetadata(ctx context.Context, req *pb.GetContentMetadataRequest) (*pb.GetContentMetadataResponse, error) {
	var meta pb.GetContentMetadataResponse
	query := `SELECT content_id, title, description FROM content WHERE content_id = $1`
	err := s.db.QueryRow(query, req.ContentId).Scan(&meta.ContentId, &meta.Title, &meta.Description)
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

type httpHandler struct {
	db  *sql.DB
	rmq *rabbitmq.Client
}

func (h *httpHandler) uploadContent(w http.ResponseWriter, r *http.Request) {
	userCtx, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse multipart form
	err := r.ParseMultipartForm(10 << 20) // 10 MB max
	if err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "failed to get file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Create uploads directory
	uploadDir := "./uploads"
	os.MkdirAll(uploadDir, os.ModePerm)

	// Save file
	contentID := uuid.New().String()
	filename := fmt.Sprintf("%s_%s", contentID, header.Filename)
	filePath := filepath.Join(uploadDir, filename)

	dst, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "failed to save file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		http.Error(w, "failed to save file", http.StatusInternalServerError)
		return
	}

	// Save to database
	title := r.FormValue("title")
	if title == "" {
		title = header.Filename
	}

	query := `INSERT INTO content (content_id, user_id, title, file_path, file_type, file_size, uploaded_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err = h.db.Exec(query, contentID, userCtx.UserID, title, filePath, header.Header.Get("Content-Type"), header.Size, time.Now())
	if err != nil {
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	// Publish event
	event := rabbitmq.ContentUploadedEvent{
		ContentID: contentID,
		UserID:    userCtx.UserID,
		Title:     title,
		FilePath:  filePath,
		FileType:  header.Header.Get("Content-Type"),
		Timestamp: time.Now(),
	}
	h.rmq.Publish(rabbitmq.ContentUploadedKey, event)

	json.NewEncoder(w).Encode(map[string]string{
		"content_id": contentID,
		"message":    "Content uploaded successfully",
	})
}

func initDB(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS content (
			content_id VARCHAR(36) PRIMARY KEY,
			user_id VARCHAR(36) NOT NULL,
			title VARCHAR(255) NOT NULL,
			file_path TEXT NOT NULL,
			file_type VARCHAR(100),
			file_size BIGINT,
			description TEXT,
			uploaded_at TIMESTAMP NOT NULL,
			INDEX idx_user_id (user_id)
		);
	`
	_, err := db.Exec(schema)
	return err
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	err = logger.Init("content-service", cfg.Environment)
	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}
	defer logger.Sync()

	db, err := sql.Open("postgres", cfg.DatabaseDSN())
	if err != nil {
		logger.Fatal("Failed to connect to database")
	}
	defer db.Close()

	if err := initDB(db); err != nil {
		logger.Fatal("Failed to initialize database")
	}

	rmqClient, err := rabbitmq.NewClient(cfg.RabbitMQURL)
	if err != nil {
		logger.Fatal("Failed to connect to RabbitMQ")
	}
	defer rmqClient.Close()

	// Start gRPC server
	go func() {
		lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
		if err != nil {
			logger.Fatal("Failed to listen")
		}

		grpcServer := grpc.NewServer()
		pb.RegisterContentServiceServer(grpcServer, &contentServer{db: db, rmq: rmqClient})

		logger.Info(fmt.Sprintf("Content Service gRPC listening on port %s", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal("Failed to serve gRPC")
		}
	}()

	// Start HTTP server
	r := mux.NewRouter()
	handler := &httpHandler{db: db, rmq: rmqClient}
	
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}).Methods("GET")
	
	r.HandleFunc("/upload", handler.uploadContent).Methods("POST")

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	})

	logger.Info(fmt.Sprintf("Content Service HTTP listening on port %s", cfg.ServerPort))
	if err := http.ListenAndServe(":"+cfg.ServerPort, c.Handler(r)); err != nil {
		logger.Fatal("Failed to serve HTTP")
	}
}
