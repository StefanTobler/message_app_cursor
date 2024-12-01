package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"messager/internal/api"
	"messager/internal/config"
	"messager/internal/db"
	"messager/internal/websocket"
)

func setupLogger() *log.Logger {
	return log.New(os.Stdout, "[SERVER] ", log.LstdFlags|log.Lshortfile)
}

func main() {
	// Parse command line flags
	isLoadTest := flag.Bool("loadtest", false, "Run server with load testing configuration")
	flag.Parse()

	logger := setupLogger()
	logger.Println("Starting server...")

	// Load configuration
	cfg := config.Load()

	// Modify database path for load testing
	if *isLoadTest {
		// Create loadtest directory next to the regular database
		cwd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		loadTestDir := filepath.Join(cwd, "loadtest")
		if err := os.MkdirAll(loadTestDir, 0755); err != nil {
			logger.Fatalf("Failed to create loadtest directory: %v", err)
		}

		// Update the database path to use the loadtest directory
		loadTestPath := filepath.Join(loadTestDir, "loadtest.db")
		cfg.UpdateDatabasePath(loadTestPath)
		logger.Printf("Using load testing database: %s", loadTestPath)
	}

	logger.Printf("Loaded configuration: %+v\n", cfg)

	// Initialize database with clean path
	database, err := db.NewDB(cfg.CleanDatabasePath())
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()
	logger.Println("Database connection established")

	// Initialize WebSocket hub
	hub := websocket.NewHub(database)
	go hub.Run()
	logger.Println("WebSocket hub initialized")

	// Initialize API handlers
	handlers := api.NewHandlers(database, hub)
	logger.Println("API handlers initialized")

	// Set up HTTP routes
	mux := http.NewServeMux()

	// WebSocket endpoint - handle separately without logging middleware
	mux.HandleFunc("/ws", handlers.HandleWebSocket)

	// Auth endpoints
	mux.HandleFunc("/api/auth/register", logRequest(logger, handlers.HandleRegister))
	mux.HandleFunc("/api/auth/login", logRequest(logger, handlers.HandleLogin))
	mux.HandleFunc("/api/auth/verify", logRequest(logger, handlers.HandleVerify))
	mux.HandleFunc("/api/auth/logout", logRequest(logger, handlers.HandleLogout))

	// Conversation endpoints
	mux.HandleFunc("/api/conversations", logRequest(logger, handlers.HandleConversations))
	mux.HandleFunc("/api/conversations/create", logRequest(logger, handlers.HandleCreateConversation))
	mux.HandleFunc("/api/conversations/messages", logRequest(logger, handlers.HandleMessages))

	// User endpoints
	mux.HandleFunc("/api/users", logRequest(logger, handlers.HandleUsers))

	// Create a wrapped handler that skips CORS for WebSocket
	wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws" {
			handlers.HandleWebSocket(w, r)
			return
		}
		handlers.WithCORS(handlers.WithAuth(mux)).ServeHTTP(w, r)
	})

	// Start server
	server := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: wrappedHandler,
	}

	// Start server in a goroutine
	go func() {
		logger.Printf("Server starting on %s", cfg.ServerAddress)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Printf("Received signal: %v", sig)

	logger.Println("Server shutting down...")
}

func logRequest(logger *log.Logger, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		logger.Printf("Started %s %s", r.Method, r.URL.Path)
		
		// Create a custom response writer to capture the status code
		lrw := newLoggingResponseWriter(w)
		
		next.ServeHTTP(lrw, r)
		
		logger.Printf("Completed %s %s %d %s in %v",
			r.Method, r.URL.Path, lrw.statusCode,
			http.StatusText(lrw.statusCode),
			time.Since(start))
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
} 