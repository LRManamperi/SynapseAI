package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/synapseai/platform/pkg/config"
	"github.com/synapseai/platform/pkg/logger"
	"github.com/synapseai/platform/pkg/rabbitmq"
)

type NotificationService struct {
	rmq *rabbitmq.Client
}

func (ns *NotificationService) sendEmail(to, subject, body string) error {
	// Mock SMTP implementation
	logger.Info(fmt.Sprintf("📧 Sending email to: %s", to))
	logger.Info(fmt.Sprintf("Subject: %s", subject))
	logger.Info(fmt.Sprintf("Body: %s", body))
	
	// In production, use SMTP library like gomail or net/smtp
	return nil
}

func (ns *NotificationService) subscribeToEvents() {
	// Subscribe to QuizGenerated events
	ns.rmq.Subscribe("notification_quiz_queue", rabbitmq.QuizGeneratedKey, func(body []byte) error {
		var event rabbitmq.QuizGeneratedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return err
		}

		logger.Info(fmt.Sprintf("Quiz generated notification for user: %s", event.UserID))
		
		subject := "New Quiz Available!"
		emailBody := fmt.Sprintf(
			"Hello!\n\nA new quiz '%s' has been generated from your content.\n"+
				"It contains %d questions and is ready for you to take.\n\n"+
				"Happy learning!",
			event.Title, event.NumQuestions,
		)
		
		return ns.sendEmail(fmt.Sprintf("user_%s@example.com", event.UserID), subject, emailBody)
	})

	// Subscribe to QuizCompleted events
	ns.rmq.Subscribe("notification_completion_queue", rabbitmq.QuizCompletedKey, func(body []byte) error {
		var event rabbitmq.QuizCompletedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return err
		}

		logger.Info(fmt.Sprintf("Quiz completed notification for user: %s", event.UserID))

		var subject string
		var emailBody string
		
		if event.Passed {
			subject = "🎉 Congratulations! You passed the quiz!"
			emailBody = fmt.Sprintf(
				"Great job!\n\n"+
					"You scored %.2f%% on your quiz.\n"+
					"You earned %d XP points!\n\n"+
					"Keep up the excellent work!",
				event.Percentage, event.XPEarned,
			)
		} else {
			subject = "Quiz Attempt Summary"
			emailBody = fmt.Sprintf(
				"Thanks for completing the quiz!\n\n"+
					"You scored %.2f%%. Don't worry, you can try again!\n"+
					"Review the material and give it another shot.\n\n"+
					"You've got this!",
				event.Percentage,
			)
		}
		
		return ns.sendEmail(fmt.Sprintf("user_%s@example.com", event.UserID), subject, emailBody)
	})

	logger.Info("Subscribed to all notification events")
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	err = logger.Init("notification-service", cfg.Environment)
	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}
	defer logger.Sync()

	rmqClient, err := rabbitmq.NewClient(cfg.RabbitMQURL)
	if err != nil {
		logger.Fatal("Failed to connect to RabbitMQ")
	}
	defer rmqClient.Close()

	service := &NotificationService{rmq: rmqClient}
	
	// Subscribe to events
	service.subscribeToEvents()

	// Simple HTTP server for health checks
	r := mux.NewRouter()
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "notification"})
	}).Methods("GET")

	logger.Info("Notification Service started and listening for events")
	logger.Info("Health endpoint available on port 8007")
	
	if err := http.ListenAndServe(":8007", r); err != nil {
		logger.Fatal("Failed to start HTTP server")
	}
}
