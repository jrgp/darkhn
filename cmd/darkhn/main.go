package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/jrgp/darkhn/internal/handler"
	"github.com/jrgp/darkhn/internal/middleware"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := chi.NewRouter()

	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.BotProtection)
	r.Use(middleware.RateLimit(10, 20)) // 10 req/s sustained, burst of 20

	// Serve the injected dark-mode CSS from the inject/ directory.
	r.Get("/inject.css", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "inject/inject.css")
	})

	proxy := handler.NewProxy()

	r.Get("/*", proxy.Handle)

	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Unsupported HTTP Method "+r.Method, http.StatusMethodNotAllowed)
	})

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Server started on port %s", port)
	log.Fatal(srv.ListenAndServe())
}
