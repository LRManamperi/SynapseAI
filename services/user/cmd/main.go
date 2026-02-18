package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"

	_ "github.com/lib/pq"
	"github.com/synapseai/platform/pkg/config"
	"github.com/synapseai/platform/pkg/logger"
	pb "github.com/synapseai/platform/proto/user"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type server struct {
	pb.UnimplementedUserServiceServer
	db *sql.DB
}

func (s *server) GetProfile(ctx context.Context, req *pb.GetProfileRequest) (*pb.GetProfileResponse, error) {
	var profile pb.GetProfileResponse
	query := `SELECT user_id, name, email, avatar_url, bio, created_at, updated_at FROM profiles WHERE user_id = $1`
	
	err := s.db.QueryRow(query, req.UserId).Scan(
		&profile.UserId, &profile.Name, &profile.Email, 
		&profile.AvatarUrl, &profile.Bio, &profile.CreatedAt, &profile.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &profile, nil
}

func (s *server) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.UpdateProfileResponse, error) {
	query := `
		INSERT INTO profiles (user_id, name, bio, avatar_url, updated_at) 
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (user_id) 
		DO UPDATE SET name = $2, bio = $3, avatar_url = $4, updated_at = NOW()
	`
	
	_, err := s.db.Exec(query, req.UserId, req.Name, req.Bio, req.AvatarUrl)
	if err != nil {
		return &pb.UpdateProfileResponse{Success: false, Message: err.Error()}, nil
	}
	
	return &pb.UpdateProfileResponse{Success: true, Message: "Profile updated successfully"}, nil
}

func (s *server) GetPreferences(ctx context.Context, req *pb.GetPreferencesRequest) (*pb.GetPreferencesResponse, error) {
	var prefs pb.GetPreferencesResponse
	query := `SELECT user_id, language, difficulty_level, email_notifications, push_notifications FROM preferences WHERE user_id = $1`
	
	err := s.db.QueryRow(query, req.UserId).Scan(
		&prefs.UserId, &prefs.Language, &prefs.DifficultyLevel, 
		&prefs.EmailNotifications, &prefs.PushNotifications,
	)
	if err != nil {
		// Return defaults if not found
		return &pb.GetPreferencesResponse{
			UserId: req.UserId,
			Language: "en",
			DifficultyLevel: "intermediate",
			EmailNotifications: true,
			PushNotifications: false,
		}, nil
	}
	
	return &prefs, nil
}

func (s *server) UpdatePreferences(ctx context.Context, req *pb.UpdatePreferencesRequest) (*pb.UpdatePreferencesResponse, error) {
	query := `
		INSERT INTO preferences (user_id, language, difficulty_level, email_notifications, push_notifications)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id)
		DO UPDATE SET language = $2, difficulty_level = $3, email_notifications = $4, push_notifications = $5
	`
	
	_, err := s.db.Exec(query, req.UserId, req.Language, req.DifficultyLevel, req.EmailNotifications, req.PushNotifications)
	if err != nil {
		return &pb.UpdatePreferencesResponse{Success: false, Message: err.Error()}, nil
	}
	
	return &pb.UpdatePreferencesResponse{Success: true, Message: "Preferences updated"}, nil
}

func (s *server) GetLearningGoals(ctx context.Context, req *pb.GetLearningGoalsRequest) (*pb.GetLearningGoalsResponse, error) {
	query := `SELECT goal_id, title, description, target_date, progress, status FROM learning_goals WHERE user_id = $1`
	rows, err := s.db.Query(query, req.UserId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var goals []*pb.LearningGoal
	for rows.Next() {
		var goal pb.LearningGoal
		err := rows.Scan(&goal.GoalId, &goal.Title, &goal.Description, &goal.TargetDate, &goal.Progress, &goal.Status)
		if err != nil {
			continue
		}
		goals = append(goals, &goal)
	}
	
	return &pb.GetLearningGoalsResponse{Goals: goals}, nil
}

func (s *server) UpdateLearningGoals(ctx context.Context, req *pb.UpdateLearningGoalsRequest) (*pb.UpdateLearningGoalsResponse, error) {
	// Simplified: delete and re-insert all goals
	_, err := s.db.Exec(`DELETE FROM learning_goals WHERE user_id = $1`, req.UserId)
	if err != nil {
		return &pb.UpdateLearningGoalsResponse{Success: false, Message: err.Error()}, nil
	}
	
	for _, goal := range req.Goals {
		query := `INSERT INTO learning_goals (goal_id, user_id, title, description, target_date, progress, status) VALUES ($1, $2, $3, $4, $5, $6, $7)`
		_, err := s.db.Exec(query, goal.GoalId, req.UserId, goal.Title, goal.Description, goal.TargetDate, goal.Progress, goal.Status)
		if err != nil {
			continue
		}
	}
	
	return &pb.UpdateLearningGoalsResponse{Success: true, Message: "Goals updated"}, nil
}

func initDB(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS profiles (
			user_id VARCHAR(36) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL,
			avatar_url TEXT,
			bio TEXT,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS preferences (
			user_id VARCHAR(36) PRIMARY KEY,
			language VARCHAR(10) DEFAULT 'en',
			difficulty_level VARCHAR(50) DEFAULT 'intermediate',
			email_notifications BOOLEAN DEFAULT true,
			push_notifications BOOLEAN DEFAULT false
		);

		CREATE TABLE IF NOT EXISTS learning_goals (
			goal_id VARCHAR(36) PRIMARY KEY,
			user_id VARCHAR(36) NOT NULL,
			title VARCHAR(255) NOT NULL,
			description TEXT,
			target_date VARCHAR(50),
			progress INT DEFAULT 0,
			status VARCHAR(50) DEFAULT 'in_progress'
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

	err = logger.Init("user-service", cfg.Environment)
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

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		logger.Fatal("Failed to listen")
	}

	grpcServer := grpc.NewServer()
	pb.RegisterUserServiceServer(grpcServer, &server{db: db})
	
	// Register health check
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

	logger.Info(fmt.Sprintf("User Service listening on port %s", cfg.GRPCPort))
	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatal("Failed to serve")
	}
}
