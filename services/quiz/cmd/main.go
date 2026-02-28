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
	_ "github.com/lib/pq"
	"github.com/rs/cors"
	"github.com/synapseai/platform/pkg/config"
	"github.com/synapseai/platform/pkg/logger"
	"github.com/synapseai/platform/pkg/rabbitmq"
	pb "github.com/synapseai/platform/proto/quiz"
	"go.uber.org/zap"
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
			created_at TIMESTAMP NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_content_id ON quizzes(content_id);

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
			attempted_at TIMESTAMP NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_user_quiz ON attempts(user_id, quiz_id);
	`
	_, err := db.Exec(schema)
	return err
}

func subscribeToEvents(rmq *rabbitmq.Client, server *quizServer) {
	// Subscribe to QuizGenerated events
	rmq.Subscribe("quiz_generated_queue", rabbitmq.QuizGeneratedKey, func(body []byte) error {
		var event rabbitmq.QuizGeneratedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return fmt.Errorf("failed to unmarshal event: %w", err)
		}

		logger.Info(fmt.Sprintf("Processing quiz generation: %s for content: %s", event.QuizID, event.ContentID))

		// Insert quiz record
		quizQuery := `INSERT INTO quizzes (quiz_id, content_id, title, difficulty, created_at) 
		              VALUES ($1, $2, $3, $4, $5)`
		_, err := server.db.Exec(quizQuery, event.QuizID, event.ContentID, event.Title, event.Difficulty, event.Timestamp)
		if err != nil {
			return fmt.Errorf("failed to insert quiz: %w", err)
		}

		// Insert questions
		questionQuery := `INSERT INTO questions (question_id, quiz_id, question_text, options, correct_option, explanation) 
		                  VALUES ($1, $2, $3, $4, $5, $6)`

		for i, q := range event.Questions {
			questionID := fmt.Sprintf("%s_q%d", event.QuizID, i+1)

			// Convert options to JSON
			optionsJSON, err := json.Marshal(q.Options)
			if err != nil {
				logger.Error("Failed to marshal options", zap.Error(err))
				continue
			}

			_, err = server.db.Exec(questionQuery, questionID, event.QuizID, q.Question, optionsJSON, q.CorrectOption, q.Explanation)
			if err != nil {
				logger.Error("Failed to insert question", zap.Error(err), zap.String("questionID", questionID))
				continue
			}
		}

		logger.Info(fmt.Sprintf("Successfully saved quiz: %s with %d questions", event.QuizID, len(event.Questions)))
		return nil
	})
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

	rmqClient, err := rabbitmq.NewClient(cfg.RabbitMQURL)
	if err != nil {
		logger.Fatal("Failed to connect to RabbitMQ")
	}
	defer rmqClient.Close()

	server := &quizServer{db: db, rmq: rmqClient}

	// Subscribe to events
	subscribeToEvents(rmqClient, server)

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

	// List quizzes for a content
	r.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		contentID := r.URL.Query().Get("content_id")
		if contentID == "" {
			http.Error(w, "content_id is required", http.StatusBadRequest)
			return
		}

		query := `SELECT quiz_id, title, difficulty, created_at, 
		          (SELECT COUNT(*) FROM questions WHERE questions.quiz_id = quizzes.quiz_id) as question_count
		          FROM quizzes WHERE content_id = $1 ORDER BY created_at DESC LIMIT 10`

		rows, err := server.db.Query(query, contentID)
		if err != nil {
			logger.Error("Failed to query quizzes", zap.Error(err))
			http.Error(w, "Failed to fetch quizzes", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type QuizItem struct {
			QuizID        string `json:"quiz_id"`
			Title         string `json:"title"`
			Difficulty    string `json:"difficulty"`
			CreatedAt     string `json:"created_at"`
			QuestionCount int    `json:"question_count"`
		}

		var items []QuizItem
		for rows.Next() {
			var item QuizItem
			var createdAt time.Time
			err := rows.Scan(&item.QuizID, &item.Title, &item.Difficulty, &createdAt, &item.QuestionCount)
			if err != nil {
				logger.Error("Failed to scan quiz", zap.Error(err))
				continue
			}
			item.CreatedAt = createdAt.Format(time.RFC3339)
			items = append(items, item)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": items,
			"total": len(items),
		})
	}).Methods("GET")

	// Get quiz details
	r.HandleFunc("/{quiz_id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		quizID := vars["quiz_id"]

		type Question struct {
			QuestionID    string   `json:"question_id"`
			QuestionText  string   `json:"question_text"`
			Options       []string `json:"options"`
			CorrectOption int32    `json:"correct_option"`
			Explanation   string   `json:"explanation"`
		}

		type QuizResponse struct {
			QuizID     string     `json:"quiz_id"`
			ContentID  string     `json:"content_id"`
			Title      string     `json:"title"`
			Difficulty string     `json:"difficulty"`
			CreatedAt  string     `json:"created_at"`
			Questions  []Question `json:"questions"`
		}

		var quiz QuizResponse
		var createdAt time.Time
		query := `SELECT quiz_id, content_id, title, difficulty, created_at FROM quizzes WHERE quiz_id = $1`

		err := server.db.QueryRow(query, quizID).Scan(&quiz.QuizID, &quiz.ContentID, &quiz.Title, &quiz.Difficulty, &createdAt)
		if err != nil {
			logger.Error("Failed to query quiz", zap.Error(err))
			http.Error(w, "Quiz not found", http.StatusNotFound)
			return
		}
		quiz.CreatedAt = createdAt.Format(time.RFC3339)

		// Get questions
		questionsQuery := `SELECT question_id, question_text, options, correct_option, explanation FROM questions WHERE quiz_id = $1`
		rows, err := server.db.Query(questionsQuery, quizID)
		if err != nil {
			logger.Error("Failed to query questions", zap.Error(err))
			http.Error(w, "Failed to fetch questions", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var q Question
			var optionsJSON []byte
			err := rows.Scan(&q.QuestionID, &q.QuestionText, &optionsJSON, &q.CorrectOption, &q.Explanation)
			if err != nil {
				logger.Error("Failed to scan question", zap.Error(err))
				continue
			}

			// Parse options JSON
			if err := json.Unmarshal(optionsJSON, &q.Options); err != nil {
				logger.Error("Failed to unmarshal options", zap.Error(err))
				q.Options = []string{"Option A", "Option B", "Option C", "Option D"}
			}

			quiz.Questions = append(quiz.Questions, q)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(quiz)
	}).Methods("GET")

	// Submit quiz answers
	r.HandleFunc("/{quiz_id}/submit", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		quizID := vars["quiz_id"]

		type AnswerInput struct {
			QuestionID     string `json:"question_id"`
			SelectedOption int32  `json:"selected_option"`
		}
		type SubmitRequest struct {
			Answers []AnswerInput `json:"answers"`
		}
		type QuestionResult struct {
			QuestionID    string `json:"question_id"`
			Correct       bool   `json:"correct"`
			CorrectOption int32  `json:"correct_option"`
			Explanation   string `json:"explanation"`
		}
		type SubmitResponse struct {
			Score   int              `json:"score"`
			Total   int              `json:"total"`
			Correct int              `json:"correct"`
			Results []QuestionResult `json:"results"`
		}

		var req SubmitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		// Fetch correct answers from DB
		rows, err := server.db.Query(
			`SELECT question_id, correct_option, explanation FROM questions WHERE quiz_id = $1`,
			quizID,
		)
		if err != nil {
			http.Error(w, "failed to fetch questions", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type dbQuestion struct {
			ID            string
			CorrectOption int32
			Explanation   string
		}
		dbQuestions := map[string]dbQuestion{}
		for rows.Next() {
			var q dbQuestion
			rows.Scan(&q.ID, &q.CorrectOption, &q.Explanation)
			dbQuestions[q.ID] = q
		}

		// Build answer map for quick lookup
		answerMap := map[string]int32{}
		for _, a := range req.Answers {
			answerMap[a.QuestionID] = a.SelectedOption
		}

		// Score
		var results []QuestionResult
		correct := 0
		for _, dbQ := range dbQuestions {
			selected, answered := answerMap[dbQ.ID]
			isCorrect := answered && selected == dbQ.CorrectOption
			if isCorrect {
				correct++
			}
			results = append(results, QuestionResult{
				QuestionID:    dbQ.ID,
				Correct:       isCorrect,
				CorrectOption: dbQ.CorrectOption,
				Explanation:   dbQ.Explanation,
			})
		}

		total := len(dbQuestions)
		score := 0
		if total > 0 {
			score = (correct * 100) / total
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SubmitResponse{
			Score:   score,
			Total:   total,
			Correct: correct,
			Results: results,
		})
	}).Methods("POST")

	// User stats across all their attempts
	r.HandleFunc("/user-stats", func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			http.Error(w, "user_id is required", http.StatusBadRequest)
			return
		}

		type UserStats struct {
			TotalAttempts int     `json:"total_attempts"`
			Passed        int     `json:"passed"`
			AvgScore      float64 `json:"avg_score"`
			XP            int     `json:"xp"`
			DaysActive    int     `json:"days_active"`
			UniqueQuizzes int     `json:"unique_quizzes"`
		}

		var stats UserStats
		err := server.db.QueryRow(`
			SELECT
				COUNT(*) AS total_attempts,
				COALESCE(SUM(CASE WHEN passed THEN 1 ELSE 0 END), 0) AS passed,
				COALESCE(AVG(score), 0) AS avg_score,
				COUNT(DISTINCT DATE(attempted_at)) AS days_active,
				COUNT(DISTINCT quiz_id) AS unique_quizzes
			FROM attempts WHERE user_id = $1`, userID,
		).Scan(&stats.TotalAttempts, &stats.Passed, &stats.AvgScore, &stats.DaysActive, &stats.UniqueQuizzes)
		if err != nil {
			stats = UserStats{}
		}
		stats.XP = stats.TotalAttempts*10 + stats.Passed*50

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}).Methods("GET")

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	})
	logger.Info(fmt.Sprintf("Quiz Service HTTP listening on port %s", cfg.ServerPort))
	if err := http.ListenAndServe(":"+cfg.ServerPort, c.Handler(r)); err != nil {
		logger.Fatal("Failed to serve HTTP")
	}
}
