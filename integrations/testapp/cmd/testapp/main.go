package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/alexlup06/authgate-testapp/handlers"
	appmw "github.com/alexlup06/authgate-testapp/middleware"
)

func main() {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// Public
	r.Get("/", handlers.Home)

	// Protected
	r.Group(func(r chi.Router) {
		r.Use(appmw.RequireAuth)
		r.Get("/private", handlers.Private)
	})

	log.Println("testapp listening on :3000")
	log.Fatal(http.ListenAndServe(":3000", r))
}
