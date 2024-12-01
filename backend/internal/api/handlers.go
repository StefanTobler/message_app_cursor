package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt"
	gorilla "github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"

	"messager/internal/db"
	"messager/internal/models"
	"messager/internal/websocket"
)

type contextKey string

const (
	userContextKey contextKey = "user"
)

type Handlers struct {
	db  *db.DB
	hub *websocket.Hub
}

var upgrader = gorilla.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "http://localhost:3000"
	},
}

func NewHandlers(db *db.DB, hub *websocket.Hub) *Handlers {
	return &Handlers{db: db, hub: hub}
}

// Middleware
func (h *Handlers) WithAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for login, register, and verify endpoints
		if r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/auth/register" || r.URL.Path == "/api/auth/verify" {
			next.ServeHTTP(w, r)
			return
		}

		// Get token from cookie
		cookie, err := r.Cookie("auth_token")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Parse and validate token
		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(cookie.Value, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte("your-secret-key"), nil // TODO: Use config
		})

		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Check token expiration
		exp, ok := claims["exp"].(float64)
		if !ok || int64(exp) < time.Now().Unix() {
			http.Error(w, "Token expired", http.StatusUnauthorized)
			return
		}

		// Get user ID from claims
		userID, ok := claims["user_id"].(float64)
		if !ok {
			http.Error(w, "Invalid user ID in token", http.StatusUnauthorized)
			return
		}

		// Get user from database
		user, err := h.db.GetUserByID(int64(userID))
		if err != nil {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		// Add user to request context
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handlers) WithCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip CORS for WebSocket connections
		if r.Header.Get("Upgrade") == "websocket" {
			next.ServeHTTP(w, r)
			return
		}

		// Allow requests from your frontend domain in development
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Auth handlers
func (h *Handlers) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	user, err := h.db.CreateUser(req.Username, string(hashedPassword), req.Avatar)
	if err != nil {
		http.Error(w, "Username already exists", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

func (h *Handlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.db.GetUserByUsername(req.Username)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(time.Hour * 24 * 30).Unix(), // 30 days
	})

	tokenString, err := token.SignedString([]byte("your-secret-key")) // TODO: Use config
	if err != nil {
		http.Error(w, "Failed to create token", http.StatusInternalServerError)
		return
	}

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    tokenString,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   60 * 60 * 24 * 30, // 30 days in seconds
	})

	// Return user data and token
	user.Password = "" // Don't send password back
	response := models.LoginResponse{
		Token: tokenString,
		User:  *user,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handlers) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Clear the auth cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		MaxAge:   -1,    // Delete the cookie
	})

	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) HandleVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get token from cookie
	cookie, err := r.Cookie("auth_token")
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse and validate token
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(cookie.Value, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte("your-secret-key"), nil // TODO: Use config
	})

	if err != nil || !token.Valid {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Check token expiration
	exp, ok := claims["exp"].(float64)
	if !ok || int64(exp) < time.Now().Unix() {
		http.Error(w, "Token expired", http.StatusUnauthorized)
		return
	}

	// Get user ID from claims
	userID, ok := claims["user_id"].(float64)
	if !ok {
		http.Error(w, "Invalid user ID in token", http.StatusUnauthorized)
		return
	}

	// Get user from database
	user, err := h.db.GetUserByID(int64(userID))
	if err != nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	// Don't send password back
	user.Password = ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// Conversation handlers
func (h *Handlers) HandleCreateConversation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user from context
	user := r.Context().Value(userContextKey).(*models.User)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req models.CreateConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// For direct messages, check if conversation already exists
	if req.Type == "direct" && len(req.Participants) == 1 {
		otherUserID := req.Participants[0]
		existingConv, err := h.db.GetExistingDirectConversation(user.ID, otherUserID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to check existing conversation: %v", err), http.StatusInternalServerError)
			return
		}
		if existingConv != nil {
			// Return the existing conversation
			json.NewEncoder(w).Encode(existingConv)
			return
		}
	}

	// For direct messages, ensure the conversation name is set to the sender's name
	if req.Type == "direct" {
		req.Name = user.Username
	}

	// Add the current user to participants if not already included
	hasCurrentUser := false
	for _, participantID := range req.Participants {
		if participantID == user.ID {
			hasCurrentUser = true
			break
		}
	}
	if !hasCurrentUser {
		req.Participants = append(req.Participants, user.ID)
	}

	conversation, err := h.db.CreateConversation(req.Name, req.Type, req.Participants)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create conversation: %v", err), http.StatusInternalServerError)
		return
	}

	// For direct messages, create a second conversation for the other user
	if req.Type == "direct" && len(req.Participants) == 2 {
		otherUserID := req.Participants[0]
		if otherUserID == user.ID {
			otherUserID = req.Participants[1]
		}

		// Create a conversation for the other user with the current user's name
		_, err = h.db.CreateConversation(user.Username, req.Type, req.Participants)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create reciprocal conversation: %v", err), http.StatusInternalServerError)
			return
		}
	}

	json.NewEncoder(w).Encode(conversation)
}

func (h *Handlers) HandleConversations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}



	// Get user from context as *models.User
    user, ok := r.Context().Value(userContextKey).(*models.User)
    if !ok {
        log.Printf("Failed to get user from context")
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    log.Printf("Fetching conversations for user: %d", user.ID)
    conversations, err := h.db.GetUserConversations(user.ID)
    if err != nil {
        log.Printf("Failed to fetch conversations: %v", err)
        http.Error(w, "Failed to fetch conversations", http.StatusInternalServerError)
        return
    }

	log.Printf("Found %d conversations for user %d", len(conversations), user.ID)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(conversations); err != nil {
		log.Printf("Failed to encode conversations: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *Handlers) HandleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	conversationID, err := strconv.ParseInt(r.URL.Query().Get("conversation_id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid conversation ID", http.StatusBadRequest)
		return
	}

	limit := 50 // Default limit
	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		offset, _ = strconv.Atoi(offsetStr)
	}

	messages, err := h.db.GetConversationMessages(conversationID, limit, offset)
	if err != nil {
		http.Error(w, "Failed to fetch messages", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(messages)
}

// User handlers
func (h *Handlers) HandleUsers(w http.ResponseWriter, r *http.Request) {
	// Only allow GET method
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get search query from URL parameters
	query := r.URL.Query().Get("search")

	var users []*models.User
	var err error

	if query != "" {
		// If search query is provided, search users
		users, err = h.db.SearchUsers(query)
	} else {
		// If no search query, get all users
		users, err = h.db.GetAllUsers()
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get users: %v", err), http.StatusInternalServerError)
		return
	}

	// Filter out sensitive information and prepare response
	type UserResponse struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Avatar   string `json:"avatar"`
	}

	response := make([]UserResponse, 0, len(users))
	for _, user := range users {
		response = append(response, UserResponse{
			ID:       user.ID,
			Username: user.Username,
			Avatar:   user.Avatar,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}
}

// WebSocket handler
func (h *Handlers) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Printf("WebSocket connection attempt from %s", r.RemoteAddr)

	// Get auth cookie
	cookie, err := r.Cookie("auth_token")
	if err != nil {
		log.Printf("No auth cookie found: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate token
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(cookie.Value, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte("your-secret-key"), nil // Use config.JWTSecret in production
	})

	if err != nil || !token.Valid {
		log.Printf("Invalid token: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userIDFloat, _ := claims["user_id"].(float64)
	userID := int64(userIDFloat)
	user, err := h.db.GetUserByID(userID)
	if err != nil {
		log.Printf("User not found: %v", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Upgrade connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	log.Printf("WebSocket authenticated for user: %s (ID: %d)", user.Username, user.ID)

	client := websocket.NewClient(h.hub, conn, userID, user.Username)
	h.hub.Register <- client

	go client.WritePump()
	go client.ReadPump()
} 