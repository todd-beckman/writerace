package main

import (
	"log"
	"net/http"
	"os"

	"github.com/todd-beckman/writerace/server/internal/handler"
	"github.com/todd-beckman/writerace/server/internal/ratelimit"
	"github.com/todd-beckman/writerace/server/internal/session"
	"github.com/todd-beckman/writerace/server/internal/static"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	allowedOrigin := os.Getenv("ALLOWED_ORIGIN")
	if allowedOrigin == "" {
		allowedOrigin = "http://localhost:5173"
	}

	mgr := session.NewManager()
	limiter := ratelimit.NewLimiter()
	addr := ":" + port
	log.Println("Server listening on", addr)
	log.Fatal(http.ListenAndServe(addr, handler.SecureHeaders(handler.Cors(allowedOrigin, handler.NewMux(allowedOrigin, mgr, limiter, static.Content)))))
}
