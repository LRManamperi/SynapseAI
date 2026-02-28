package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/rs/cors"
	"github.com/synapseai/platform/pkg/config"
	"github.com/synapseai/platform/pkg/jwt"
	"github.com/synapseai/platform/pkg/logger"
	"github.com/synapseai/platform/pkg/middleware"
	"github.com/synapseai/platform/pkg/rabbitmq"
	pb "github.com/synapseai/platform/proto/progress"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type progressServer struct {
	pb.UnimplementedProgressServiceServer
	db  *sql.DB
	rmq *rabbitmq.Client
}

func (s *progressServer) GetUserProgress(ctx context.Context, req *pb.GetUserProgressRequest) (*pb.GetUserProgressResponse, error) {
	var progress pb.GetUserProgressResponse
	query := `
		SELECT user_id, total_xp, level, current_streak, longest_streak, quizzes_completed, content_uploaded, last_activity
		FROM user_progress WHERE user_id = $1
	`

	err := s.db.QueryRow(query, req.UserId).Scan(
		&progress.UserId, &progress.TotalXp, &progress.Level, &progress.CurrentStreak,
		&progress.LongestStreak, &progress.QuizzesCompleted, &progress.ContentUploaded, &progress.LastActivity,
	)

	if err == sql.ErrNoRows {
		// Create new progress record
		return s.createUserProgress(req.UserId)
	}

	if err != nil {
		return nil, err
	}

	return &progress, nil
}

func (s *progressServer) createUserProgress(userID string) (*pb.GetUserProgressResponse, error) {
	query := `
		INSERT INTO user_progress (user_id, total_xp, level, current_streak, longest_streak, quizzes_completed, content_uploaded, last_activity)
		VALUES ($1, 0, 1, 0, 0, 0, 0, NOW())
	`
	_, err := s.db.Exec(query, userID)
	if err != nil {
		return nil, err
	}

	return &pb.GetUserProgressResponse{
		UserId:           userID,
		TotalXp:          0,
		Level:            1,
		CurrentStreak:    0,
		LongestStreak:    0,
		QuizzesCompleted: 0,
		ContentUploaded:  0,
		LastActivity:     time.Now().Format(time.RFC3339),
	}, nil
}

func (s *progressServer) AddXP(ctx context.Context, req *pb.AddXPRequest) (*pb.AddXPResponse, error) {
	// Calculate new level
	var currentXP, currentLevel int32
	s.db.QueryRow(`SELECT total_xp, level FROM user_progress WHERE user_id = $1`, req.UserId).Scan(&currentXP, &currentLevel)

	newXP := currentXP + req.XpAmount
	newLevel := calculateLevel(newXP)
	leveledUp := newLevel > currentLevel

	// Update progress
	query := `
		UPDATE user_progress 
		SET total_xp = $1, level = $2, last_activity = NOW()
		WHERE user_id = $3
	`
	_, err := s.db.Exec(query, newXP, newLevel, req.UserId)
	if err != nil {
		return &pb.AddXPResponse{Success: false, Message: err.Error()}, nil
	}

	// Log XP
	logQuery := `INSERT INTO xp_logs (user_id, xp_amount, activity_type, reference_id, created_at) VALUES ($1, $2, $3, $4, NOW())`
	s.db.Exec(logQuery, req.UserId, req.XpAmount, req.ActivityType, req.ReferenceId)

	message := fmt.Sprintf("Added %d XP", req.XpAmount)
	if leveledUp {
		message = fmt.Sprintf("Level up! You are now level %d", newLevel)
	}

	return &pb.AddXPResponse{
		Success:    true,
		NewTotalXp: newXP,
		NewLevel:   newLevel,
		LevelUp:    leveledUp,
		Message:    message,
	}, nil
}

func (s *progressServer) GetStreak(ctx context.Context, req *pb.GetStreakRequest) (*pb.GetStreakResponse, error) {
	var streak pb.GetStreakResponse
	query := `SELECT current_streak, longest_streak, last_activity FROM user_progress WHERE user_id = $1`

	var lastActivity string
	err := s.db.QueryRow(query, req.UserId).Scan(&streak.CurrentStreak, &streak.LongestStreak, &lastActivity)
	if err != nil {
		return nil, err
	}

	streak.LastActivityDate = lastActivity

	// Check if active today
	lastActivityTime, _ := time.Parse(time.RFC3339, lastActivity)
	today := time.Now().Truncate(24 * time.Hour)
	streak.ActiveToday = lastActivityTime.After(today)

	return &streak, nil
}

func (s *progressServer) GetLeaderboard(ctx context.Context, req *pb.GetLeaderboardRequest) (*pb.GetLeaderboardResponse, error) {
	query := `
		SELECT user_id, total_xp, level 
		FROM user_progress 
		ORDER BY total_xp DESC 
		LIMIT $1
	`

	rows, err := s.db.Query(query, req.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*pb.LeaderboardEntry
	rank := int32(1)

	for rows.Next() {
		var entry pb.LeaderboardEntry
		err := rows.Scan(&entry.UserId, &entry.Xp, &entry.Level)
		if err != nil {
			continue
		}
		entry.Rank = rank
		entry.Username = fmt.Sprintf("User_%s", entry.UserId[:8])
		entries = append(entries, &entry)
		rank++
	}

	return &pb.GetLeaderboardResponse{Entries: entries}, nil
}

func (s *progressServer) GetAchievements(ctx context.Context, req *pb.GetAchievementsRequest) (*pb.GetAchievementsResponse, error) {
	// Mock achievements - implement full system in production
	achievements := []*pb.Achievement{
		{AchievementId: "first_quiz", Title: "First Quiz", Description: "Complete your first quiz", Unlocked: true, XpReward: 50},
		{AchievementId: "quiz_master", Title: "Quiz Master", Description: "Complete 10 quizzes", Unlocked: false, XpReward: 200},
	}

	return &pb.GetAchievementsResponse{Achievements: achievements, TotalUnlocked: 1}, nil
}

func calculateLevel(xp int32) int32 {
	// Simple level calculation - 100 XP per level
	return (xp / 100) + 1
}

type httpHandler struct {
	server *progressServer
}

func (h *httpHandler) getProgress(w http.ResponseWriter, r *http.Request) {
	userCtx, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	progress, err := h.server.GetUserProgress(context.Background(), &pb.GetUserProgressRequest{UserId: userCtx.UserID})
	if err != nil {
		http.Error(w, "failed to fetch progress", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user_id":           progress.UserId,
		"total_xp":          progress.TotalXp,
		"level":             progress.Level,
		"current_streak":    progress.CurrentStreak,
		"longest_streak":    progress.LongestStreak,
		"quizzes_completed": progress.QuizzesCompleted,
		"content_uploaded":  progress.ContentUploaded,
		"last_activity":     progress.LastActivity,
	})
}

func (h *httpHandler) getStreak(w http.ResponseWriter, r *http.Request) {
	userCtx, ok := middleware.GetUserFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	streak, err := h.server.GetStreak(context.Background(), &pb.GetStreakRequest{UserId: userCtx.UserID})
	if err != nil {
		http.Error(w, "failed to fetch streak", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"current_streak":     streak.CurrentStreak,
		"longest_streak":     streak.LongestStreak,
		"last_activity_date": streak.LastActivityDate,
		"active_today":       streak.ActiveToday,
	})
}

func (h *httpHandler) getLeaderboard(w http.ResponseWriter, r *http.Request) {
	leaderboard, err := h.server.GetLeaderboard(context.Background(), &pb.GetLeaderboardRequest{Limit: 50})
	if err != nil {
		http.Error(w, "failed to fetch leaderboard", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries": leaderboard.Entries,
	})
}

func subscribeToEvents(rmq *rabbitmq.Client, server *progressServer) {
	rmq.Subscribe("progress_quiz_queue", rabbitmq.QuizCompletedKey, func(body []byte) error {
		var event rabbitmq.QuizCompletedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return err
		}

		logger.Info(fmt.Sprintf("Processing quiz completion for user: %s", event.UserID))

		// Add XP
		ctx := context.Background()
		_, err := server.AddXP(ctx, &pb.AddXPRequest{
			UserId:       event.UserID,
			XpAmount:     int32(event.XPEarned),
			ActivityType: "quiz_completed",
			ReferenceId:  event.QuizID,
		})

		// Update streak
		if err == nil {
			updateStreakQuery := `
				UPDATE user_progress 
				SET quizzes_completed = quizzes_completed + 1,
				    current_streak = current_streak + 1,
				    longest_streak = GREATEST(longest_streak, current_streak + 1)
				WHERE user_id = $1
			`
			server.db.Exec(updateStreakQuery, event.UserID)
		}

		return err
	})
}

func initDB(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS user_progress (
			user_id VARCHAR(36) PRIMARY KEY,
			total_xp INT DEFAULT 0,
			level INT DEFAULT 1,
			current_streak INT DEFAULT 0,
			longest_streak INT DEFAULT 0,
			quizzes_completed INT DEFAULT 0,
			content_uploaded INT DEFAULT 0,
			last_activity TIMESTAMP DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS xp_logs (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(36) NOT NULL,
			xp_amount INT NOT NULL,
			activity_type VARCHAR(50),
			reference_id VARCHAR(36),
			created_at TIMESTAMP NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_user_id ON xp_logs(user_id);

		CREATE TABLE IF NOT EXISTS streaks (
			user_id VARCHAR(36) PRIMARY KEY,
			current_streak INT DEFAULT 0,
			longest_streak INT DEFAULT 0,
			last_activity_date DATE
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

	err = logger.Init("progress-service", cfg.Environment)
	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}
	defer logger.Sync()

	db, err := sql.Open("postgres", cfg.DatabaseDSN())
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err), zap.String("dsn", cfg.DatabaseDSN()))
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		logger.Fatal("Failed to ping database", zap.Error(err), zap.String("dsn", cfg.DatabaseDSN()))
	}

	if err := initDB(db); err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}

	// Initialize JWT manager
	jwtManager := jwt.NewManager(cfg.JWTSecret, cfg.JWTExpiry)

	rmqClient, err := rabbitmq.NewClient(cfg.RabbitMQURL)
	if err != nil {
		logger.Fatal("Failed to connect to RabbitMQ")
	}
	defer rmqClient.Close()

	server := &progressServer{db: db, rmq: rmqClient}

	// Subscribe to events
	subscribeToEvents(rmqClient, server)

	// Start gRPC server
	go func() {
		lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
		if err != nil {
			logger.Fatal("Failed to listen")
		}

		grpcServer := grpc.NewServer()
		pb.RegisterProgressServiceServer(grpcServer, server)

		logger.Info(fmt.Sprintf("Progress Service gRPC listening on port %s", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal("Failed to serve gRPC")
		}
	}()

	// Start HTTP server
	r := mux.NewRouter()
	httpHandler := &httpHandler{server: server}

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}).Methods("GET")

	// Protected routes
	protected := r.NewRoute().Subrouter()
	protected.Use(middleware.AuthMiddleware(jwtManager))
	protected.HandleFunc("/progress", httpHandler.getProgress).Methods("GET")
	protected.HandleFunc("/streak", httpHandler.getStreak).Methods("GET")
	protected.HandleFunc("/leaderboard", httpHandler.getLeaderboard).Methods("GET")

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	})

	// Get ServerPort from config (default to 8006 if not set)
	serverPort := "8006"
	if cfg.ServerPort != "" {
		serverPort = cfg.ServerPort
	}

	logger.Info(fmt.Sprintf("Progress Service HTTP listening on port %s", serverPort))
	if err := http.ListenAndServe(":"+serverPort, c.Handler(r)); err != nil {
		logger.Fatal("Failed to serve HTTP")
	}
}
