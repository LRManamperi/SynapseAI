package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
	"github.com/synapseai/platform/pkg/config"
	"github.com/synapseai/platform/pkg/logger"
	"github.com/synapseai/platform/pkg/rabbitmq"
	pb "github.com/synapseai/platform/proto/ai"
	"google.golang.org/grpc"
)

const (
	aiBaseURL  = "https://api.groq.com/openai/v1"
	aiModel    = "llama-3.3-70b-versatile"
	uploadsDir = "/root/uploads"
)

// Groq API types (OpenAI-compatible)
type grokMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type grokRequest struct {
	Model       string        `json:"model"`
	Messages    []grokMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type grokResponse struct {
	Choices []struct {
		Message grokMessage `json:"message"`
	} `json:"choices"`
	Error json.RawMessage `json:"error,omitempty"`
}

type generatedQuestion struct {
	Question      string   `json:"question"`
	Options       []string `json:"options"`
	CorrectOption int      `json:"correct_option"`
	Explanation   string   `json:"explanation"`
}

type aiServer struct {
	pb.UnimplementedAIServiceServer
	apiKey string
	rmq    *rabbitmq.Client
}

// callAI sends a prompt to the Groq API and returns the text response
func (s *aiServer) callAI(prompt string) (string, error) {
	reqBody := grokRequest{
		Model:       aiModel,
		Messages:    []grokMessage{{Role: "user", Content: prompt}},
		Temperature: 0.7,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", aiBaseURL+"/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Grok API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read Grok response: %w", err)
	}

	var grokResp grokResponse
	if err := json.Unmarshal(respBodyBytes, &grokResp); err != nil {
		return "", fmt.Errorf("failed to decode Grok response (status %d): %w", resp.StatusCode, err)
	}

	if len(grokResp.Error) > 0 && string(grokResp.Error) != "null" {
		// error can be a string or object
		var errMsg string
		var errObj struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(grokResp.Error, &errObj) == nil && errObj.Message != "" {
			errMsg = errObj.Message
		} else {
			json.Unmarshal(grokResp.Error, &errMsg)
		}
		return "", fmt.Errorf("Grok API error (HTTP %d): %s | body: %s", resp.StatusCode, errMsg, string(respBodyBytes))
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("Grok API HTTP %d: %s", resp.StatusCode, string(respBodyBytes))
	}
	if len(grokResp.Choices) == 0 {
		return "", fmt.Errorf("empty response from Grok API (body: %s)", string(respBodyBytes))
	}

	return grokResp.Choices[0].Message.Content, nil
}

// generateQuestionsFromAI calls Groq to produce quiz questions from content text
func (s *aiServer) generateQuestionsFromAI(contentText, title string, numQuestions int, difficulty string) ([]generatedQuestion, error) {
	// Truncate very large content to avoid token limits
	maxLen := 8000
	if len(contentText) > maxLen {
		contentText = contentText[:maxLen] + "..."
	}

	prompt := fmt.Sprintf(`You are an expert educational quiz generator. Your job is to create high-quality, specific quiz questions strictly based on the actual content provided below.

Document Title: %s
Difficulty: %s

Document Content:
%s

Generate exactly %d multiple-choice questions. Return ONLY a valid JSON array with no markdown, no code blocks, no extra text.

Required JSON structure:
[
  {
    "question": "A specific question about a concept, fact, or idea found in the document?",
    "options": ["Specific answer text", "Specific wrong answer", "Another plausible wrong answer", "Another wrong answer"],
    "correct_option": 0,
    "explanation": "Brief explanation citing where in the content this answer is found"
  }
]

CRITICAL RULES:
- Every question MUST be based on a specific fact, concept, term, process, or idea found in the document
- Options MUST be meaningful, specific phrases — NEVER generic labels like "Option A", "Option B", "Choice 1", etc.
- All 4 options must be plausible but only one correct
- correct_option is the 0-based index (0, 1, 2, or 3) of the correct answer
- Questions should test real comprehension, not just recall of obvious words
- Return ONLY the JSON array`, title, difficulty, contentText, numQuestions)

	response, err := s.callAI(prompt)
	if err != nil {
		return nil, err
	}

	// Extract JSON array from response (strip markdown code blocks if present)
	response = strings.TrimSpace(response)
	if idx := strings.Index(response, "["); idx != -1 {
		response = response[idx:]
	}
	if idx := strings.LastIndex(response, "]"); idx != -1 {
		response = response[:idx+1]
	}

	var questions []generatedQuestion
	if err := json.Unmarshal([]byte(response), &questions); err != nil {
		return nil, fmt.Errorf("failed to parse Grok response as JSON: %w\nResponse was: %s", err, response)
	}

	return questions, nil
}

// findFileByContentID scans the uploads directory for a file whose name begins with
// "<contentID>_", which is the naming convention used by the content service.
func findFileByContentID(contentID string) string {
	entries, err := os.ReadDir(uploadsDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), contentID+"_") {
			return filepath.Join(uploadsDir, e.Name())
		}
	}
	// Also try working-directory-relative path (content service uses "./uploads")
	entries2, err := os.ReadDir("uploads")
	if err == nil {
		for _, e := range entries2 {
			if !e.IsDir() && strings.HasPrefix(e.Name(), contentID+"_") {
				return filepath.Join("uploads", e.Name())
			}
		}
	}
	return ""
}

// extractPDFText extracts readable text from a PDF file using the ledongthuc/pdf library
func extractPDFText(filePath string) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("pdf.Open: %w", err)
	}
	defer f.Close()

	var sb strings.Builder
	for pageNum := 1; pageNum <= r.NumPage(); pageNum++ {
		page := r.Page(pageNum)
		if page.V.IsNull() {
			continue
		}
		texts := page.Content().Text
		for _, t := range texts {
			sb.WriteString(t.S)
			sb.WriteString(" ")
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// readFileContent reads an uploaded file's text content, with PDF support
func readFileContent(filePath string) string {
	// Build candidate paths: original path + uploads dir fallback
	candidates := []string{filePath, filepath.Join(uploadsDir, filepath.Base(filePath))}

	for _, path := range candidates {
		if _, err := os.Stat(path); err != nil {
			continue
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".pdf" {
			text, err := extractPDFText(path)
			if err != nil {
				logger.Info(fmt.Sprintf("PDF extraction failed for %s: %v", path, err))
				continue
			}
			text = strings.TrimSpace(text)
			if len(text) > 50 {
				logger.Info(fmt.Sprintf("Extracted PDF text from %s (%d chars)", path, len(text)))
				return text
			}
			logger.Info(fmt.Sprintf("PDF extraction yielded too little text from %s", path))
			continue
		}

		// Text-based file
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		logger.Info(fmt.Sprintf("Read text file %s (%d bytes)", path, len(data)))
		return string(data)
	}

	logger.Info(fmt.Sprintf("File not readable: %s", filePath))
	return ""
}

func (s *aiServer) GenerateSummary(ctx context.Context, req *pb.GenerateSummaryRequest) (*pb.GenerateSummaryResponse, error) {
	if s.apiKey == "" {
		summary := fmt.Sprintf("Summary of: %s (API key not configured)", req.ContentText[:min(100, len(req.ContentText))])
		return &pb.GenerateSummaryResponse{Summary: summary, WordCount: 50}, nil
	}
	prompt := fmt.Sprintf("Summarize the following content in 150 words or less:\n\n%s", req.ContentText)
	summary, err := s.callAI(prompt)
	if err != nil {
		return nil, err
	}
	return &pb.GenerateSummaryResponse{Summary: summary, WordCount: int32(len(strings.Fields(summary)))}, nil
}

// generateAndPublish is the central helper that generates questions via Groq and publishes the quiz event.
// title should be the human-readable document name.
func (s *aiServer) generateAndPublish(ctx context.Context, contentID, title, userID, contentText string, numQ int, difficulty string) (string, []*pb.QuizQuestion, error) {
	quizID := fmt.Sprintf("quiz_%d", time.Now().UnixNano())

	if numQ <= 0 {
		numQ = 5
	}
	if difficulty == "" {
		difficulty = "intermediate"
	}
	if title == "" {
		title = contentID
	}

	var pbQuestions []*pb.QuizQuestion
	var eventQuestions []rabbitmq.QuizQuestion

	// Use Groq if API key present and content is non-trivial
	if s.apiKey != "" && len(strings.TrimSpace(contentText)) > 50 {
		logger.Info(fmt.Sprintf("Groq: generating %d questions for '%s'", numQ, title))
		generated, err := s.generateQuestionsFromAI(contentText, title, numQ, difficulty)
		if err != nil {
			logger.Info(fmt.Sprintf("Groq failed: %v — using fallback", err))
		} else {
			for _, q := range generated {
				pbQuestions = append(pbQuestions, &pb.QuizQuestion{
					Question:      q.Question,
					Options:       q.Options,
					CorrectOption: int32(q.CorrectOption),
					Explanation:   q.Explanation,
				})
				eventQuestions = append(eventQuestions, rabbitmq.QuizQuestion{
					Question:      q.Question,
					Options:       q.Options,
					CorrectOption: int32(q.CorrectOption),
					Explanation:   q.Explanation,
				})
			}
			logger.Info(fmt.Sprintf("Groq generated %d questions for '%s'", len(generated), title))
		}
	} else if s.apiKey == "" {
		logger.Info("Groq API key not set — using fallback questions")
	} else {
		logger.Info(fmt.Sprintf("Content too short for '%s' (%d chars) — using fallback", title, len(strings.TrimSpace(contentText))))
	}

	// Fallback if Groq failed, not configured, or returned nothing
	if len(pbQuestions) == 0 {
		logger.Info("Using fallback placeholder questions")
		fallback := []struct{ q, e string }{
			{"What is the main topic discussed in this document?", "Refer to the document's introduction."},
			{"Which key concept is central to this material?", "This concept is a core theme of the document."},
			{"What is the primary purpose of this document?", "The document states its purpose at the beginning."},
			{"Which approach is described in this content?", "The content describes this approach in detail."},
			{"What conclusion can be drawn from this material?", "The document leads to this conclusion."},
		}
		opts := []string{"Review the document", "Consult additional sources", "Ask an expert", "Not covered in this document"}
		for i, f := range fallback {
			if i >= numQ {
				break
			}
			pbQuestions = append(pbQuestions, &pb.QuizQuestion{
				Question: f.q, Options: opts, CorrectOption: 0, Explanation: f.e,
			})
			eventQuestions = append(eventQuestions, rabbitmq.QuizQuestion{
				Question: f.q, Options: opts, CorrectOption: 0, Explanation: f.e,
			})
		}
	}

	// Publish quiz event with the real document title
	event := rabbitmq.QuizGeneratedEvent{
		QuizID:       quizID,
		ContentID:    contentID,
		UserID:       userID,
		Title:        title,
		Difficulty:   difficulty,
		NumQuestions: len(eventQuestions),
		Questions:    eventQuestions,
		Timestamp:    time.Now(),
	}
	if err := s.rmq.Publish(rabbitmq.QuizGeneratedKey, event); err != nil {
		logger.Info(fmt.Sprintf("Failed to publish QuizGenerated event: %v", err))
		return quizID, pbQuestions, fmt.Errorf("publish failed: %w", err)
	}

	return quizID, pbQuestions, nil
}

func (s *aiServer) GenerateQuiz(ctx context.Context, req *pb.GenerateQuizRequest) (*pb.GenerateQuizResponse, error) {
	// Use content_id as a fallback title when none is available
	quizID, pbQuestions, err := s.generateAndPublish(
		ctx, req.ContentId, req.ContentId, req.UserId, req.ContentText,
		int(req.NumQuestions), req.Difficulty,
	)
	if err != nil {
		return nil, err
	}
	return &pb.GenerateQuizResponse{QuizId: quizID, Questions: pbQuestions}, nil
}

func (s *aiServer) GenerateFlashcards(ctx context.Context, req *pb.GenerateFlashcardsRequest) (*pb.GenerateFlashcardsResponse, error) {
	flashcards := []*pb.Flashcard{
		{Front: "What is microservices architecture?", Back: "An architectural style that structures an application as a collection of loosely coupled services."},
		{Front: "What is gRPC?", Back: "A high-performance RPC framework that uses Protocol Buffers for serialization."},
		{Front: "What is Clean Architecture?", Back: "A software design philosophy that separates concerns into layers with dependencies pointing inward."},
	}
	return &pb.GenerateFlashcardsResponse{Flashcards: flashcards}, nil
}

func (s *aiServer) AnalyzeContent(ctx context.Context, req *pb.AnalyzeContentRequest) (*pb.AnalyzeContentResponse, error) {
	return &pb.AnalyzeContentResponse{
		DifficultyLevel:      "intermediate",
		Topics:               []string{"microservices", "architecture", "backend"},
		Keywords:             []string{"Go", "gRPC", "PostgreSQL", "Docker"},
		EstimatedReadingTime: 15,
	}, nil
}

func subscribeToEvents(rmq *rabbitmq.Client, server *aiServer) {
	rmq.Subscribe("ai_content_queue", rabbitmq.ContentUploadedKey, func(body []byte) error {
		var event rabbitmq.ContentUploadedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return err
		}

		logger.Info(fmt.Sprintf("Received ContentUploaded: id=%s title=%s file=%s", event.ContentID, event.Title, event.FilePath))

		// Read and extract the actual document content (handles PDFs)
		contentText := readFileContent(event.FilePath)
		if contentText == "" {
			// Path from event failed — scan uploads dir by content_id prefix
			if found := findFileByContentID(event.ContentID); found != "" {
				logger.Info(fmt.Sprintf("Found file by content_id scan: %s", found))
				contentText = readFileContent(found)
			}
		}
		if contentText == "" {
			contentText = fmt.Sprintf("Document titled: %s. File type: %s.", event.Title, event.FileType)
			logger.Info(fmt.Sprintf("File unreadable, using title as fallback context: %s", event.ContentID))
		}

		ctx := context.Background()
		// Pass event.Title so the quiz is named after the actual document
		_, _, err := server.generateAndPublish(ctx, event.ContentID, event.Title, event.UserID, contentText, 5, "intermediate")
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

	if server.apiKey == "" {
		logger.Info("WARNING: OPENAI_API_KEY not set — Groq API disabled, fallback questions will be used")
	} else {
		logger.Info(fmt.Sprintf("Groq API enabled (model: %s)", aiModel))
	}

	subscribeToEvents(rmqClient, server)

	// HTTP server: health check + manual quiz retrigger
	httpMux := http.NewServeMux()

	httpMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "ai-service"})
	})

	// POST /retrigger — manually trigger quiz generation for existing content
	// Body: {"content_id":"...","user_id":"...","file_path":"...","title":"..."}
	httpMux.HandleFunc("/retrigger", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			ContentID string `json:"content_id"`
			UserID    string `json:"user_id"`
			FilePath  string `json:"file_path"`
			Title     string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		if body.ContentID == "" {
			http.Error(w, "content_id is required", http.StatusBadRequest)
			return
		}

		contentText := readFileContent(body.FilePath)
		if contentText == "" {
			// File path not supplied or invalid — find file by content_id prefix
			if found := findFileByContentID(body.ContentID); found != "" {
				logger.Info(fmt.Sprintf("Retrigger: found file by scan: %s", found))
				contentText = readFileContent(found)
			}
		}
		if contentText == "" {
			contentText = fmt.Sprintf("Document titled: %s.", body.Title)
		}

		ctx := context.Background()
		// Use body.Title so retrigger also stores the real document name
		quizID, questions, err := server.generateAndPublish(ctx, body.ContentID, body.Title, body.UserID, contentText, 5, "intermediate")
		if err != nil {
			logger.Info(fmt.Sprintf("Retrigger failed for %s: %v", body.ContentID, err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":       "quiz generation triggered",
			"content_id":    body.ContentID,
			"quiz_id":       quizID,
			"num_questions": len(questions),
		})
	})

	// Wrap with CORS
	corsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		httpMux.ServeHTTP(w, r)
	})

	// Start HTTP server in background
	httpPort := "8004"
	go func() {
		logger.Info(fmt.Sprintf("AI Service HTTP listening on port %s", httpPort))
		if err := http.ListenAndServe(":"+httpPort, corsHandler); err != nil {
			logger.Info(fmt.Sprintf("HTTP server error: %v", err))
		}
	}()

	// Start gRPC server
	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		logger.Fatal("Failed to listen")
	}

	grpcServer := grpc.NewServer()
	pb.RegisterAIServiceServer(grpcServer, server)

	logger.Info(fmt.Sprintf("AI Service gRPC listening on port %s", cfg.GRPCPort))
	if err := grpcServer.Serve(lis); err != nil {
		logger.Fatal("Failed to serve")
	}
}
