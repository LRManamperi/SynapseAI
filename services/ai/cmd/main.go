package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/synapseai/platform/pkg/config"
	"github.com/synapseai/platform/pkg/logger"
	"github.com/synapseai/platform/pkg/rabbitmq"
	pb "github.com/synapseai/platform/proto/ai"
	"google.golang.org/grpc"
)

type aiServer struct {
	pb.UnimplementedAIServiceServer
	apiKey string
	rmq    *rabbitmq.Client
}

func (s *aiServer) GenerateSummary(ctx context.Context, req *pb.GenerateSummaryRequest) (*pb.GenerateSummaryResponse, error) {
	// Mock OpenAI API call - In production, use actual OpenAI SDK
	summary := fmt.Sprintf("Summary of content: %s (Mock implementation - integrate OpenAI SDK for production)", req.ContentText[:min(100, len(req.ContentText))])
	
	return &pb.GenerateSummaryResponse{
		Summary:   summary,
		WordCount: 50,
	}, nil
}

func (s *aiServer) GenerateQuiz(ctx context.Context, req *pb.GenerateQuizRequest) (*pb.GenerateQuizResponse, error) {
	// Mock quiz generation - In production implement OpenAI API
	quizID := fmt.Sprintf("quiz_%d", time.Now().Unix())
	
	questions := []*pb.QuizQuestion{
		{
			Question:      "What is the main topic of this content?",
			Options:       []string{"Option A", "Option B", "Option C", "Option D"},
			CorrectOption: 0,
			Explanation:   "This is the correct answer based on the content analysis.",
		},
		{
			Question:      "Which concept is emphasized in the content?",
			Options:       []string{"Concept 1", "Concept 2", "Concept 3", "Concept 4"},
			CorrectOption: 1,
			Explanation:   "The content focuses heavily on this concept.",
		},
	}

	// Convert questions for event
	var eventQuestions []rabbitmq.QuizQuestion
	for _, q := range questions {
		eventQuestions = append(eventQuestions, rabbitmq.QuizQuestion{
			Question:      q.Question,
			Options:       q.Options,
			CorrectOption: q.CorrectOption,
			Explanation:   q.Explanation,
		})
	}

	// Publish QuizGenerated event
	event := rabbitmq.QuizGeneratedEvent{
		QuizID:       quizID,
		ContentID:    req.ContentId,
		UserID:       req.UserId,
		Title:        "Generated Quiz",
		Difficulty:   req.Difficulty,
		NumQuestions: len(questions),
		Questions:    eventQuestions,
		Timestamp:    time.Now(),
	}
	s.rmq.Publish(rabbitmq.QuizGeneratedKey, event)

	return &pb.GenerateQuizResponse{
		QuizId:    quizID,
		Questions: questions,
	}, nil
}

func (s *aiServer) GenerateFlashcards(ctx context.Context, req *pb.GenerateFlashcardsRequest) (*pb.GenerateFlashcardsResponse, error) {
	// Mock flashcard generation
	flashcards := []*pb.Flashcard{
		{Front: "What is microservices architecture?", Back: "An architectural style that structures an application as a collection of loosely coupled services."},
		{Front: "What is gRPC?", Back: "A high-performance RPC framework that uses Protocol Buffers for serialization."},
		{Front: "What is Clean Architecture?", Back: "A software design philosophy that separates concerns into layers with dependencies pointing inward."},
	}

	return &pb.GenerateFlashcardsResponse{Flashcards: flashcards}, nil
}

func (s *aiServer) AnalyzeContent(ctx context.Context, req *pb.AnalyzeContentRequest) (*pb.AnalyzeContentResponse, error) {
	// Mock content analysis
	return &pb.AnalyzeContentResponse{
		DifficultyLevel:      "intermediate",
		Topics:               []string{"microservices", "architecture", "backend"},
		Keywords:             []string{"Go", "gRPC", "PostgreSQL", "Docker"},
		EstimatedReadingTime: 15,
	}, nil
}

func subscribeToEvents(rmq *rabbitmq.Client, server *aiServer) {
	// Subscribe to ContentUploaded events
	rmq.Subscribe("ai_content_queue", rabbitmq.ContentUploadedKey, func(body []byte) error {
		var event rabbitmq.ContentUploadedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return err
		}

		logger.Info(fmt.Sprintf("Processing content: %s", event.ContentID))

		// Auto-generate quiz for uploaded content
		// In production, read actual file content and process it
		ctx := context.Background()
		_, err := server.GenerateQuiz(ctx, &pb.GenerateQuizRequest{
			ContentId:    event.ContentID,
			ContentText:  "Sample content text", // Read from file in production
			UserId:       event.UserID,
			NumQuestions: 5,
			Difficulty:   "intermediate",
		})

		return err
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	err = logger.Init("ai-service", cfg.Environment)
	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}
	defer logger.Sync()

	rmqClient, err := rabbitmq.NewClient(cfg.RabbitMQURL)
	if err != nil {
		logger.Fatal("Failed to connect to RabbitMQ")
	}
	defer rmqClient.Close()

	server := &aiServer{
		apiKey: cfg.OpenAIAPIKey,
		rmq:    rmqClient,
	}

	// Subscribe to events
	subscribeToEvents(rmqClient, server)

	// Start gRPC server
	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		logger.Fatal("Failed to listen")
	}

	grpcServer := grpc.NewServer()
	pb.RegisterAIServiceServer(grpcServer, server)

	logger.Info(fmt.Sprintf("AI Service listening on port %s", cfg.GRPCPort))
	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatal("Failed to serve")
	}
}
