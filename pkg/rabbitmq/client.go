package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Client struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

// NewClient creates a new RabbitMQ client
func NewClient(url string) (*Client, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// Declare topic exchange
	err = channel.ExchangeDeclare(
		"synapseai", // exchange name
		"topic",     // exchange type
		true,        // durable
		false,       // auto-deleted
		false,       // internal
		false,       // no-wait
		nil,         // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	return &Client{
		conn:    conn,
		channel: channel,
	}, nil
}

// Publish publishes a message to a routing key
func (c *Client) Publish(routingKey string, message interface{}) error {
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = c.channel.PublishWithContext(
		ctx,
		"synapseai", // exchange
		routingKey,  // routing key
		false,       // mandatory
		false,       // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	log.Printf("Published message to routing key: %s", routingKey)
	return nil
}

// Subscribe subscribes to messages with a routing key pattern
func (c *Client) Subscribe(queueName, routingKey string, handler func([]byte) error) error {
	// Declare queue
	queue, err := c.channel.QueueDeclare(
		queueName, // queue name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind queue to exchange with routing key
	err = c.channel.QueueBind(
		queue.Name,
		routingKey,
		"synapseai",
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	// Consume messages
	messages, err := c.channel.Consume(
		queue.Name,
		"",    // consumer
		false, // auto-ack (manual ack for reliability)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	// Process messages
	go func() {
		for msg := range messages {
			log.Printf("Received message from: %s", routingKey)

			err := handler(msg.Body)
			if err != nil {
				log.Printf("Error handling message: %v", err)
				msg.Nack(false, true) // Requeue on error
			} else {
				msg.Ack(false) // Acknowledge successful processing
			}
		}
	}()

	log.Printf("Subscribed to routing key: %s (queue: %s)", routingKey, queueName)
	return nil
}

// Close closes the RabbitMQ connection
func (c *Client) Close() error {
	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Event types and structures
const (
	// Routing keys
	ContentUploadedKey = "content.uploaded"
	QuizGeneratedKey   = "quiz.generated"
	QuizCompletedKey   = "quiz.completed"
)

type ContentUploadedEvent struct {
	ContentID string    `json:"content_id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	FilePath  string    `json:"file_path"`
	FileType  string    `json:"file_type"`
	Timestamp time.Time `json:"timestamp"`
}

type QuizGeneratedEvent struct {
	QuizID       string         `json:"quiz_id"`
	ContentID    string         `json:"content_id"`
	UserID       string         `json:"user_id"`
	Title        string         `json:"title"`
	Difficulty   string         `json:"difficulty"`
	NumQuestions int            `json:"num_questions"`
	Questions    []QuizQuestion `json:"questions"`
	Timestamp    time.Time      `json:"timestamp"`
}

type QuizQuestion struct {
	Question      string   `json:"question"`
	Options       []string `json:"options"`
	CorrectOption int32    `json:"correct_option"`
	Explanation   string   `json:"explanation"`
}

type QuizCompletedEvent struct {
	AttemptID  string    `json:"attempt_id"`
	QuizID     string    `json:"quiz_id"`
	UserID     string    `json:"user_id"`
	Score      int       `json:"score"`
	Percentage float64   `json:"percentage"`
	Passed     bool      `json:"passed"`
	XPEarned   int       `json:"xp_earned"`
	Timestamp  time.Time `json:"timestamp"`
}
