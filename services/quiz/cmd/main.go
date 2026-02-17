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

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"github.com/synapseai/platform/pkg/config"
	"github.com/synapseai/platform/pkg/logger"
	"github.com/synapseai/platform/pkg/rabbitmq"
	pb "github.com/synapseai/platform/proto/quiz"
	"google.golang.org/grpc"
)

type quizServer struct {
	pb.UnimplementedQuizServiceServer
	db  *sql.DB
	rmq *rabbitmq.Client
}

func (s *quizServer) GetQuiz(ctx context.Context, req *pb.GetQuizRequest) (*pb.GetQuizResponse, error) {
	var quiz pb.GetQuizResponse
	query := `SELECT quiz_id, content_id, title, difficulty, created_at FROM quizzes WHERE quiz_id = $1`
	
	err := s.db.QueryRow(query, req.QuizId).Scan(&quiz.QuizId, &quiz.ContentId, &quiz.Title, &quiz.Difficulty, &quiz.CreatedAt)
	if err != nil {
		return nil, err
	}

	// Get questions
	questionsQuery := `SELECT question_id, question_text, correct_option, explanation FROM questions WHERE quiz_id = $1`
	rows, err := s.db.Query(questionsQuery, req.QuizId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var q pb.Question
		err := rows.Scan(&q.QuestionId, &q.QuestionText, &q.CorrectOption, &q.Explanation)
		if err != nil {
			continue
		}
		// Get options (simplified - stored as JSON in production)
		q.Options = []string{"Option A", "Option B", "Option C", "Option D"}
		quiz.Questions = append(quiz.Questions, &q)
	}

	return &quiz, nil
}

func (s *quizServer) ListQuizzes(ctx context.Context, req *pb.ListQuizzesRequest) (*pb.ListQuizzesResponse, error) {
	query := `SELECT quiz_id, title, difficulty, created_at FROM quizzes WHERE content_id = $1 ORDER BY created_at DESC LIMIT $2`
	
	rows, err := s.db.Query(query, req.ContentId, req.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*pb.QuizItem
	for rows.Next() {
		var item pb.QuizItem
		err := rows.Scan(&item.QuizId, &item.Title, &item.Difficulty, &item.CreatedAt)
		if err != nil {
			continue
		}
		item.QuestionCount = 5 // Get actual count in production
		items = append(items, &item)
	}

	var total int32
	s.db.QueryRow(`SELECT COUNT(*) FROM quizzes WHERE content_id = $1`, req.ContentId).Scan(&total)

	return &pb.ListQuizzesResponse{Items: items, Total: total}, nil
}

func (s *quizServer) SubmitAttempt(ctx context.Context, req *pb.SubmitAttemptRequest) (*pb.SubmitAttemptResponse, error) {
	attemptID := uuid.New().String()
	
	// Get quiz questions to evaluate
	questionsQuery := `SELECT question_id, correct_option, explanation FROM questions WHERE quiz_id = $1`
	rows, err := s.db.Query(questionsQuery, req.QuizId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	correctAnswers := make(map[string]int32)
	explanations := make(map[string]string)
	
	for rows.Next() {
		var qID string
		var correctOpt int32
		var explanation string
		rows.Scan(&qID, &correctOpt, &explanation)
		correctAnswers[qID] = correctOpt
		explanations[qID] = explanation
	}

	// Evaluate answers
	var results []*pb.QuestionResult
	correctCount := 0
	totalQuestions := len(correctAnswers)

	for _, answer := range req.Answers {
		correct := correctAnswers[answer.QuestionId] == answer.SelectedOption
		if correct {
			correctCount++
		}
		results = append(results, &pb.QuestionResult{
			QuestionId:     answer.QuestionId,
			Correct:        correct,
			SelectedOption: answer.SelectedOption,
			CorrectOption:  correctAnswers[answer.QuestionId],
			Explanation:    explanations[answer.QuestionId],
		})
	}

	score := int32((correctCount * 100) / totalQuestions)
	percentage := float64(correctCount) / float64(totalQuestions) * 100
	passed := percentage >= 70.0

	// Calculate XP
	xpEarned := correctCount * 10
	if passed {
		xpEarned += 50 // Bonus for passing
	}

	// Save attempt
	query := `INSERT INTO attempts (attempt_id, quiz_id, user_id, score, percentage, passed, attempted_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err = s.db.Exec(query, attemptID, req.QuizId, req.UserId, score, percentage, passed, time.Now())
	if err != nil {
		return nil, err
	}

	// Publish QuizCompleted event
	event := rabbitmq.QuizCompletedEvent{
		AttemptID:  attemptID,
		QuizID:     req.QuizId,
		UserID:     req.UserId,
		Score:      int(score),
		Percentage: percentage,
		Passed:     passed,
		XPEarned:   int(xpEarned),
		Timestamp:  time.Now(),
	}
	s.rmq.Publish(rabbitmq.QuizCompletedKey, event)

	return &pb.SubmitAttemptResponse{
		AttemptId:      attemptID,
		Score:          score,
		TotalQuestions: int32(totalQuestions),
		CorrectAnswers: int32(correctCount),
		Percentage:     percentage,
		Passed:         passed,
		Results:        results,
	}, nil
}

func (s *quizServer) GetAttemptHistory(ctx context.Context, req *pb.GetAttemptHistoryRequest) (*pb.GetAttemptHistoryResponse, error) {
	query := `SELECT attempt_id, quiz_id, score, percentage, passed, attempted_at FROM attempts WHERE user_id = $1 AND quiz_id = $2 ORDER BY attempted_at DESC LIMIT $3`
	
	rows, err := s.db.Query(query, req.UserId, req.QuizId, req.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attempts []*pb.AttemptSummary
	for rows.Next() {
		var attempt pb.AttemptSummary
		err := rows.Scan(&attempt.AttemptId, &attempt.QuizId, &attempt.Score, &attempt.Percentage, &attempt.Passed, &attempt.AttemptedAt)
		if err != nil {
			continue
		}
		attempts = append(attempts, &attempt)
	}

	return &pb.GetAttemptHistoryResponse{Attempts: attempts}, nil
}

func (s *quizServer) GetQuizStats(ctx context.Context, req *pb.GetQuizStatsRequest) (*pb.GetQuizStatsResponse, error) {
	var stats pb.GetQuizStatsResponse
	query := `
		SELECT 
			COUNT(*) as total_attempts,
			AVG(score) as average_score,
			SUM(CASE WHEN passed THEN 1 ELSE 0 END)::FLOAT / COUNT(*) * 100 as pass_rate,
			COUNT(DISTINCT user_id) as unique_users
		FROM attempts WHERE quiz_id = $1
	`
	
	err := s.db.QueryRow(query, req.QuizId).Scan(&stats.TotalAttempts, &stats.AverageScore, &stats.PassRate, &stats.UniqueUsers)
	if err != nil {
		return &pb.GetQuizStatsResponse{}, nil
	}

	return &stats, nil
}

func initDB(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS quizzes (
			quiz_id VARCHAR(36) PRIMARY KEY,
			content_id VARCHAR(36) NOT NULL,
			title VARCHAR(255) NOT NULL,
			difficulty VARCHAR(50),
			created_at TIMESTAMP NOT NULL,
			INDEX idx_content_id (content_id)
		);

		CREATE TABLE IF NOT EXISTS questions (
			question_id VARCHAR(36) PRIMARY KEY,
			quiz_id VARCHAR(36) NOT NULL,
			question_text TEXT NOT NULL,
			options JSON,
			correct_option INT NOT NULL,
			explanation TEXT,
			FOREIGN KEY (quiz_id) REFERENCES quizzes(quiz_id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS attempts (
			attempt_id VARCHAR(36) PRIMARY KEY,
			quiz_id VARCHAR(36) NOT NULL,
			user_id VARCHAR(36) NOT NULL,
			score INT NOT NULL,
			percentage FLOAT NOT NULL,
			passed BOOLEAN NOT NULL,
			attempted_at TIMESTAMP NOT NULL,
			INDEX idx_user_quiz (user_id, quiz_id)
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

	err = logger.Init("quiz-service", cfg.Environment)
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

	server := &quizServer{db: db, rmq: rmqClient}

	// Start gRPC server
	go func() {
		lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
		if err != nil {
			logger.Fatal("Failed to listen")
		}

		grpcServer := grpc.NewServer()
		pb.RegisterQuizServiceServer(grpcServer, server)

		logger.Info(fmt.Sprintf("Quiz Service gRPC listening on port %s", cfg.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal("Failed to serve gRPC")
		}
	}()

	// Start HTTP server
	r := mux.NewRouter()
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}).Methods("GET")

	c := cors.Default()
	logger.Info(fmt.Sprintf("Quiz Service HTTP listening on port %s", cfg.ServerPort))
	if err := http.ListenAndServe(":"+cfg.ServerPort, c.Handler(r)); err != nil {
		logger.Fatal("Failed to serve HTTP")
	}
}
