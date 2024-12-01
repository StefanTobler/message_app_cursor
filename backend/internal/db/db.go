package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"messager/internal/models"
)

type DB struct {
	*sql.DB
}

func NewDB(dbPath string) (*DB, error) {
	// Create the database directory if it doesn't exist
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("error creating database directory: %v", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error connecting to the database: %v", err)
	}

	// Initialize database schema
	if err := initSchema(db); err != nil {
		return nil, fmt.Errorf("error initializing schema: %v", err)
	}

	return &DB{db}, nil
}

func initSchema(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			avatar TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS conversations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS conversation_participants (
			conversation_id INTEGER,
			user_id INTEGER,
			joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (conversation_id, user_id),
			FOREIGN KEY (conversation_id) REFERENCES conversations(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id INTEGER,
			sender_id INTEGER,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (conversation_id) REFERENCES conversations(id),
			FOREIGN KEY (sender_id) REFERENCES users(id)
		)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute schema query: %v", err)
		}
	}

	return nil
}

// User methods
func (db *DB) CreateUser(username, password, avatar string) (*models.User, error) {
	result, err := db.Exec(
		"INSERT INTO users (username, password, avatar, created_at) VALUES (?, ?, ?, ?)",
		username, password, avatar, time.Now(),
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &models.User{
		ID:        id,
		Username:  username,
		Avatar:    avatar,
		CreatedAt: time.Now(),
	}, nil
}

func (db *DB) GetUserByUsername(username string) (*models.User, error) {
	log.Printf("Looking up user by username: %s", username)
	
	user := &models.User{}
	err := db.DB.QueryRow(`
		SELECT id, username, password, avatar, created_at 
		FROM users 
		WHERE username = ?
	`, username).Scan(&user.ID, &user.Username, &user.Password, &user.Avatar, &user.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("No user found with username: %s", username)
			return nil, fmt.Errorf("user not found")
		}
		log.Printf("Database error looking up user %s: %v", username, err)
		return nil, fmt.Errorf("database error: %v", err)
	}

	log.Printf("Successfully found user: %s (ID: %d)", username, user.ID)
	return user, nil
}

func (db *DB) GetUserByID(id int64) (*models.User, error) {
	var user models.User
	err := db.QueryRow(
		"SELECT id, username, password, avatar, created_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Username, &user.Password, &user.Avatar, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Conversation methods
func (db *DB) CreateConversation(name string, convType string, participants []int64) (*models.Conversation, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Create conversation
	result, err := tx.Exec(`
		INSERT INTO conversations (name, type)
		VALUES (?, ?)
	`, name, convType)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %v", err)
	}

	conversationID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation ID: %v", err)
	}

	// Add participants
	for _, userID := range participants {
		_, err = tx.Exec(`
			INSERT INTO conversation_participants (conversation_id, user_id)
			VALUES (?, ?)
		`, conversationID, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to add participant %d: %v", userID, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	// Fetch the created conversation
	conversation := &models.Conversation{}
	err = db.DB.QueryRow(`
		SELECT id, name, type, created_at
		FROM conversations
		WHERE id = ?
	`, conversationID).Scan(&conversation.ID, &conversation.Name, &conversation.Type, &conversation.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created conversation: %v", err)
	}

	return conversation, nil
}

func (db *DB) GetUserConversations(userID int64) ([]*models.Conversation, error) {
	rows, err := db.DB.Query(`
		SELECT DISTINCT c.id, c.name, c.type, c.created_at
		FROM conversations c
		JOIN conversation_participants cp ON c.id = cp.conversation_id
		WHERE cp.user_id = ?
		ORDER BY c.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query conversations: %v", err)
	}
	defer rows.Close()

	var conversations []*models.Conversation
	for rows.Next() {
		conv := &models.Conversation{}
		err := rows.Scan(&conv.ID, &conv.Name, &conv.Type, &conv.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %v", err)
		}
		conversations = append(conversations, conv)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating conversations: %v", err)
	}

	return conversations, nil
}

// Message methods
func (db *DB) CreateMessage(conversationID, senderID int64, content string) (*models.Message, error) {
	result, err := db.Exec(
		"INSERT INTO messages (conversation_id, sender_id, content, created_at) VALUES (?, ?, ?, ?)",
		conversationID, senderID, content, time.Now(),
	)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &models.Message{
		ID:             id,
		ConversationID: conversationID,
		SenderID:       senderID,
		Content:        content,
		CreatedAt:      time.Now(),
	}, nil
}

func (db *DB) GetConversationMessages(conversationID int64, limit, offset int) ([]models.Message, error) {
	rows, err := db.Query(`
		SELECT id, conversation_id, sender_id, content, created_at
		FROM messages
		WHERE conversation_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, conversationID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var msg models.Message
		if err := rows.Scan(&msg.ID, &msg.ConversationID, &msg.SenderID, &msg.Content, &msg.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

func (db *DB) GetConversationParticipants(conversationID int64) ([]models.User, error) {
	rows, err := db.Query(`
		SELECT u.id, u.username, u.avatar, u.created_at
		FROM users u
		JOIN conversation_participants cp ON u.id = cp.user_id
		WHERE cp.conversation_id = ?
	`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var participants []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Username, &user.Avatar, &user.CreatedAt); err != nil {
			return nil, err
		}
		participants = append(participants, user)
	}
	return participants, nil
}

// GetAllUsers returns all users in the database
func (db *DB) GetAllUsers() ([]*models.User, error) {
	rows, err := db.DB.Query(`
		SELECT id, username, password, avatar, created_at 
		FROM users 
		ORDER BY username
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		err := rows.Scan(&user.ID, &user.Username, &user.Password, &user.Avatar, &user.CreatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

// SearchUsers searches for users by username with case-insensitive partial matching
func (db *DB) SearchUsers(query string) ([]*models.User, error) {
	// Use LIKE with case-insensitive matching and limit results
	rows, err := db.DB.Query(`
		SELECT id, username, avatar, created_at 
		FROM users 
		WHERE username LIKE ? COLLATE NOCASE
		ORDER BY 
			CASE 
				WHEN username LIKE ? COLLATE NOCASE THEN 1  -- Exact match
				WHEN username LIKE ? COLLATE NOCASE THEN 2  -- Starts with
				ELSE 3                                      -- Contains
			END,
			username COLLATE NOCASE
		LIMIT 10
	`, "%"+query+"%", query, query+"%")
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %v", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		err := rows.Scan(&user.ID, &user.Username, &user.Avatar, &user.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %v", err)
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %v", err)
	}

	return users, nil
}

// SaveMessage saves a new message to the database
func (db *DB) SaveMessage(message *models.Message) (*models.Message, error) {
	result, err := db.DB.Exec(`
		INSERT INTO messages (conversation_id, sender_id, content, created_at)
		VALUES (?, ?, ?, ?)
	`, message.ConversationID, message.SenderID, message.Content, message.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to save message: %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get message ID: %v", err)
	}

	message.ID = id
	return message, nil
}

// GetConversationParticipantIDs returns all participant IDs for a conversation
func (db *DB) GetConversationParticipantIDs(conversationID int64) ([]int64, error) {
	rows, err := db.DB.Query(`
		SELECT user_id
		FROM conversation_participants
		WHERE conversation_id = ?
	`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %v", err)
	}
	defer rows.Close()

	var participantIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan participant ID: %v", err)
		}
		participantIDs = append(participantIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating participants: %v", err)
	}

	return participantIDs, nil
}

// GetExistingDirectConversation checks if a direct conversation exists between two users
func (db *DB) GetExistingDirectConversation(userID1, userID2 int64) (*models.Conversation, error) {
	// Find conversations where both users are participants
	rows, err := db.DB.Query(`
		SELECT DISTINCT c.id, c.name, c.type, c.created_at
		FROM conversations c
		JOIN conversation_participants cp1 ON c.id = cp1.conversation_id
		JOIN conversation_participants cp2 ON c.id = cp2.conversation_id
		WHERE c.type = 'direct'
		AND cp1.user_id = ?
		AND cp2.user_id = ?
	`, userID1, userID2)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing conversation: %v", err)
	}
	defer rows.Close()

	// There should be at most one such conversation
	if rows.Next() {
		conv := &models.Conversation{}
		err := rows.Scan(&conv.ID, &conv.Name, &conv.Type, &conv.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %v", err)
		}
		return conv, nil
	}

	return nil, nil
} 