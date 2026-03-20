package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/authara-org/authara-go/authara"
	"github.com/authara-org/authara-testapp/handlers"
)

func main() {
	r := chi.NewRouter()

	// --- SDK config from env ---
	cfg, err := authara.ConfigFromEnv()
	if err != nil {
		log.Fatalf("authara config failed: %v", err)
	}

	appSdk, err := authara.New(cfg)
	if err != nil {
		log.Fatalf("sdk not starting: %v", err)
	}

	// --- Webhook handler from env ---
	webhookHandler, err := authara.RequireWebhookHandlerFromEnv()
	if err != nil {
		log.Fatalf("webhook handler not staritng. Check AUTHARA_WEBHOOK_SECRET : %v", err)
	}

	// --- Middleware ---
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	// --- Public routes ---
	r.Get("/", handlers.Home)

	// --- Protected routes ---
	r.Group(func(r chi.Router) {
		r.Use(appSdk.RequireAuthWithRefresh)
		r.Get("/private", handlers.Private)
	})

	// --- Webhook endpoint ---
	r.Post("/webhooks/authara", func(w http.ResponseWriter, r *http.Request) {
		evt, err := webhookHandler.Handle(w, r)
		if err != nil {
			// Handle already wrote HTTP response
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
