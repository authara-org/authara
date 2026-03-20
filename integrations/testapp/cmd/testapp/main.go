package main

import (
	"encoding/base64"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/authara-org/authara-go/authara"
	"github.com/authara-org/authara-testapp/handlers"
)

func main() {
	r := chi.NewRouter()

	appSdk, err := authara.New(authara.Config{
		Audience: "app",
		Issuer:   "authara",
		Keys: map[string][]byte{
			"key-2026-01": mustKey("VZp2u1sYz0g2nF2vY8q8dP7cZQpL5cRrXn0k7FZ0xkE="),
			"key-2025-09": mustKey("Qk8K6E3XrV6mF4T9yZcA2p9xYbDqZpM0JwH3uZ8sL1E="),
		},
		AutharaBaseURL: "http://authara:8080",
	})
	if err != nil {
		log.Fatalln("sdk not starting")
	}

	webhookSecret := os.Getenv("AUTHARA_WEBHOOK_SECRET")
	if webhookSecret == "" {
		log.Println("warning: AUTHARA_WEBHOOK_SECRET is empty, /webhooks/authara will reject requests")
	}

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	r.Get("/", handlers.Home)

	r.Group(func(r chi.Router) {
		r.Use(appSdk.RequireAuthWithRefresh)
		r.Get("/private", handlers.Private)
	})

	r.Post("/webhooks/authara", func(w http.ResponseWriter, r *http.Request) {
		handler := &authara.WebhookHandler{
			Secret: webhookSecret,
		}

		evt, err := handler.Handle(w, r)
		if err != nil {
			// Handle already wrote the HTTP error response.
			log.Printf("webhook rejected: %v", err)
			return
		}

		log.Printf(
			"authara webhook received: id=%s type=%s created_at=%s data=%s",
			evt.ID,
			evt.Type,
			evt.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			string(evt.Data),
		)

		switch evt.Type {
		case authara.WebhookEventUserCreated:
			data, err := authara.DecodeWebhookData[authara.UserCreatedData](evt)
			if err != nil {
				http.Error(w, "invalid user.created payload", http.StatusBadRequest)
				return
			}
			log.Printf("user.created: user_id=%s", data.UserID)

		case authara.WebhookEventUserDeleted:
			data, err := authara.DecodeWebhookData[authara.UserDeletedData](evt)
			if err != nil {
				http.Error(w, "invalid user.deleted payload", http.StatusBadRequest)
				return
			}
			log.Printf("user.deleted: user_id=%s", data.UserID)

		default:
			log.Printf("unknown webhook event type: %s", evt.Type)
		}

		w.WriteHeader(http.StatusNoContent)
	})

	log.Println("testapp listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func mustKey(b64 string) []byte {
	key, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		panic(err)
	}
	return key
}
