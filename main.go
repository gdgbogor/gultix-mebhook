package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

type PretixWebhook struct {
	NotificationID int    `json:"notification_id"`
	Organizer      string `json:"organizer"`
	Event          string `json:"event"`
	Code           string `json:"code"` // Order code is at top level
	Action         string `json:"action"`
	// Additional fields that might come in different webhook types
	Status string `json:"status,omitempty"` // Sometimes present
	Email  string `json:"email,omitempty"`  // Sometimes present
	Total  string `json:"total,omitempty"`  // Sometimes present
	Secret string `json:"secret,omitempty"` // Sometimes present
}

type Config struct {
	Port                  string
	FCMServiceAccountPath string
	FCMProjectID          string
	FCMTopic              string
}

var (
	config    Config
	fcmClient *messaging.Client
)

func loadConfig() {
	godotenv.Load()

	config = Config{
		Port:                  getEnvOrDefault("PORT", "8080"),
		FCMServiceAccountPath: os.Getenv("FCM_SERVICE_ACCOUNT_PATH"),
		FCMProjectID:          os.Getenv("FCM_PROJECT_ID"),
		FCMTopic:              getEnvOrDefault("FCM_TOPIC", "pretix-orders"),
	}

	if config.FCMServiceAccountPath == "" {
		log.Fatal("FCM_SERVICE_ACCOUNT_PATH environment variable is required")
	}
	if config.FCMProjectID == "" {
		log.Fatal("FCM_PROJECT_ID environment variable is required")
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func initFCM() error {
	ctx := context.Background()

	opt := option.WithCredentialsFile(config.FCMServiceAccountPath)
	app, err := firebase.NewApp(ctx, &firebase.Config{
		ProjectID: config.FCMProjectID,
	}, opt)
	if err != nil {
		return fmt.Errorf("error initializing firebase app: %v", err)
	}

	fcmClient, err = app.Messaging(ctx)
	if err != nil {
		return fmt.Errorf("error getting messaging client: %v", err)
	}

	return nil
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var webhook PretixWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		log.Printf("Error parsing webhook payload: %v", err)
		http.Error(w, "Error parsing payload", http.StatusBadRequest)
		return
	}

	log.Printf("Received webhook: organizer=%s, event=%s, action=%s, order=%s, status=%s",
		webhook.Organizer, webhook.Event, webhook.Action, webhook.Code, webhook.Status)

	if err := sendFCMNotification(webhook); err != nil {
		log.Printf("Error sending FCM notification: %v", err)
		http.Error(w, "Error processing webhook", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Webhook processed successfully"))
}

func sendFCMNotification(webhook PretixWebhook) error {
	ctx := context.Background()

	title := fmt.Sprintf("Order %s", formatAction(webhook.Action))
	body := fmt.Sprintf("Order %s from %s", webhook.Code, webhook.Event)
	if webhook.Status != "" {
		body += fmt.Sprintf(" - %s", webhook.Status)
	}
	if webhook.Total != "" {
		body += fmt.Sprintf(" (Total: %s)", webhook.Total)
	}

	message := &messaging.Message{
		Topic: config.FCMTopic,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: map[string]string{
			"notification_id": fmt.Sprintf("%d", webhook.NotificationID),
			"organizer":       webhook.Organizer,
			"event":           webhook.Event,
			"action":          webhook.Action,
			"order_code":      webhook.Code,
			"status":          webhook.Status,
			"total":           webhook.Total,
			"email":           webhook.Email,
		},
	}

	response, err := fcmClient.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("error sending FCM message: %v", err)
	}

	log.Printf("FCM message sent successfully: %s", response)
	return nil
}

func formatAction(action string) string {
	parts := strings.Split(action, ".")
	if len(parts) > 0 {
		lastPart := strings.ReplaceAll(parts[len(parts)-1], "_", " ")
		// Capitalize first letter of each word
		words := strings.Fields(lastPart)
		for i, word := range words {
			if len(word) > 0 {
				words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
			}
		}
		return strings.Join(words, " ")
	}
	return action
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func testFCMToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse device token from request body
	var request struct {
		Token   string `json:"token"`
		Title   string `json:"title,omitempty"`
		Message string `json:"message,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if request.Token == "" {
		http.Error(w, "Device token is required", http.StatusBadRequest)
		return
	}

	// Set default test message if not provided
	title := request.Title
	if title == "" {
		title = "Test FCM Message"
	}
	messageBody := request.Message
	if messageBody == "" {
		messageBody = "This is a test message from your webhook service"
	}

	// Create FCM message for direct device token
	ctx := context.Background()
	message := &messaging.Message{
		Token: request.Token,
		Notification: &messaging.Notification{
			Title: title,
			Body:  messageBody,
		},
		Data: map[string]string{
			"test":      "true",
			"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
			"source":    "webhook-test-endpoint",
		},
	}

	// Send the message
	response, err := fcmClient.Send(ctx, message)
	if err != nil {
		log.Printf("Error sending test FCM message: %v", err)
		http.Error(w, fmt.Sprintf("Failed to send message: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Test FCM message sent successfully to token: %s, response: %s",
		request.Token[:10]+"...", response)

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "success",
		"message_id": response,
		"message":    "Test message sent successfully",
	})
}

func main() {
	loadConfig()

	if err := initFCM(); err != nil {
		log.Fatalf("Failed to initialize FCM: %v", err)
	}

	http.HandleFunc("/webhook", handleWebhook)
	http.HandleFunc("/health", healthCheck)
	http.HandleFunc("/test-fcm", testFCMToken)

	log.Printf("Server starting on port %s", config.Port)
	log.Printf("Available endpoints:")
	log.Printf("  POST /webhook - Pretix webhook handler")
	log.Printf("  GET  /health - Health check")
	log.Printf("  POST /test-fcm - Test FCM with device token")
	log.Fatal(http.ListenAndServe(":"+config.Port, nil))
}
