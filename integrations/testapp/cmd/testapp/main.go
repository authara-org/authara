package main

import (
	"encoding/base64"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/alexlup06-authgate/authgate-go/authgate"
	"github.com/alexlup06-authgate/authgate-testapp/handlers"
)

func main() {
	r := chi.NewRouter()

	appSdk, err := authgate.New(authgate.Config{
		Audience: "app",
		Issuer:   "authgate",
		Keys: map[string][]byte{
			"key-2026-01": mustKey("VZp2u1sYz0g2nF2vY8q8dP7cZQpL5cRrXn0k7FZ0xkE="),
			"key-2025-09": mustKey("Qk8K6E3XrV6mF4T9yZcA2p9xYbDqZpM0JwH3uZ8sL1E="),
		},
		AuthGateBaseURL: "http://authgate:8080",
	})

	if err != nil {
		log.Fatalln("sdk not starting")
	}

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/", handlers.Home)

	r.Group(func(r chi.Router) {
		r.Use(appSdk.RequireAuthWithRefresh)
		r.Get("/private", handlers.Private)
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
