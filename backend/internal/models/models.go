package models

import "time"

type User struct {
	ID        int64     `json:"id" db:"id"`
	Username  string    `json:"username" db:"username"`
	Password  string    `json:"-" db:"password"`
	Avatar    string    `json:"avatar" db:"avatar"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type Conversation struct {
	ID        int64     `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Type      string    `json:"type" db:"type"` // "direct" or "group"
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type ConversationParticipant struct {
	ConversationID int64     `json:"conversation_id" db:"conversation_id"`
	UserID         int64     `json:"user_id" db:"user_id"`
	JoinedAt       time.Time `json:"joined_at" db:"joined_at"`
}

type Message struct {
	ID             int64     `json:"id" db:"id"`
	ConversationID int64     `json:"conversation_id" db:"conversation_id"`
	SenderID       int64     `json:"sender_id" db:"sender_id"`
	Content        string    `json:"content" db:"content"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

// Request/Response structures
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Avatar   string `json:"avatar"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type CreateConversationRequest struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Participants []int64 `json:"participants"`
}

type SendMessageRequest struct {
	ConversationID int64  `json:"conversation_id"`
	Content        string `json:"content"`
}

type WebSocketMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
} 