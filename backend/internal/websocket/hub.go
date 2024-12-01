package websocket

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"messager/internal/models"
	"messager/internal/db"
)

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	userID   int64
	username string
}

type Hub struct {
	clients    map[*Client]bool
	Broadcast  chan []byte
	Register   chan *Client
	Unregister chan *Client
	userMap    map[int64]*Client
	mu         sync.RWMutex
	logger     *log.Logger
	db         *db.DB
}

func NewHub(database *db.DB) *Hub {
	return &Hub{
		Broadcast:  make(chan []byte),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		userMap:    make(map[int64]*Client),
		logger:     log.New(os.Stdout, "[WEBSOCKET] ", log.LstdFlags|log.Lshortfile),
		db:         database,
	}
}

func (h *Hub) Run() {
	h.logger.Println("WebSocket hub started")
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.clients[client] = true
			h.userMap[client.userID] = client
			h.mu.Unlock()
			h.logger.Printf("Client connected: %s (ID: %d), total clients: %d", 
				client.username, client.userID, len(h.clients))

			// Send welcome message
			welcomeMsg := models.WebSocketMessage{
				Type: "system",
				Payload: map[string]interface{}{
					"message": "Connected to chat server",
				},
			}
			if data, err := json.Marshal(welcomeMsg); err == nil {
				client.send <- data
			}

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				delete(h.userMap, client.userID)
				close(client.send)
				h.logger.Printf("Client disconnected: %s (ID: %d), remaining clients: %d", 
					client.username, client.userID, len(h.clients))
			}
			h.mu.Unlock()

		case message := <-h.Broadcast:
			h.logger.Printf("Broadcasting message to %d clients", len(h.clients))
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
					h.logger.Printf("Message sent to client: %s", client.username)
				default:
					h.logger.Printf("Failed to send message to client: %s, removing client", client.username)
					h.mu.RUnlock()
					h.mu.Lock()
					close(client.send)
					delete(h.clients, client)
					delete(h.userMap, client.userID)
					h.mu.Unlock()
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) SendToUser(userID int64, message interface{}) error {
	h.mu.RLock()
	client, ok := h.userMap[userID]
	h.mu.RUnlock()

	if !ok {
		h.logger.Printf("User not connected: %d", userID)
		return nil // User not connected
	}

	data, err := json.Marshal(message)
	if err != nil {
		h.logger.Printf("Failed to marshal message: %v", err)
		return err
	}

	select {
	case client.send <- data:
		h.logger.Printf("Message sent to user: %d", userID)
	default:
		h.logger.Printf("Failed to send message to user: %d, removing client", userID)
		h.mu.Lock()
		close(client.send)
		delete(h.clients, client)
		delete(h.userMap, client.userID)
		h.mu.Unlock()
	}

	return nil
}

func (h *Hub) SendToConversation(conversationID int64, message interface{}, participants []int64) error {
	data, err := json.Marshal(message)
	if err != nil {
		h.logger.Printf("Failed to marshal conversation message: %v", err)
		return err
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, userID := range participants {
		if client, ok := h.userMap[userID]; ok {
			select {
			case client.send <- data:
				h.logger.Printf("Message sent to participant: %d in conversation: %d", userID, conversationID)
			default:
				h.logger.Printf("Failed to send message to participant: %d in conversation: %d", userID, conversationID)
				continue
			}
		}
	}

	return nil
}

func (h *Hub) BroadcastMessage(message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		h.logger.Printf("Failed to marshal broadcast message: %v", err)
		return err
	}

	h.Broadcast <- data
	h.logger.Println("Message queued for broadcast")
	return nil
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister <- c
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		var wsMessage models.WebSocketMessage
		if err := json.Unmarshal(message, &wsMessage); err != nil {
			log.Printf("error unmarshaling message: %v", err)
			continue
		}

		// Handle different message types
		switch wsMessage.Type {
		case "message":
			if msg, ok := wsMessage.Payload.(map[string]interface{}); ok {
				conversationID := int64(msg["conversation_id"].(float64))
				content := msg["content"].(string)

				// Create and save the message to the database
				newMessage := &models.Message{
					ConversationID: conversationID,
					SenderID:      c.userID,
					Content:       content,
					CreatedAt:     time.Now(),
				}

				// Save message to database
				savedMessage, err := c.hub.db.SaveMessage(newMessage)
				if err != nil {
					log.Printf("Failed to save message: %v", err)
					continue
				}

				// Create response message with saved message data
				response := models.WebSocketMessage{
					Type:    "message",
					Payload: savedMessage,
				}

				// Send to all participants in the conversation
				participants, err := c.hub.db.GetConversationParticipantIDs(conversationID)
				if err != nil {
					log.Printf("Failed to get conversation participants: %v", err)
					continue
				}

				if err := c.hub.SendToConversation(conversationID, response, participants); err != nil {
					log.Printf("Failed to broadcast message: %v", err)
				}
			}
		case "typing":
			if typing, ok := wsMessage.Payload.(map[string]interface{}); ok {
				response := models.WebSocketMessage{
					Type: "typing",
					Payload: map[string]interface{}{
						"user_id":         c.userID,
						"conversation_id": typing["conversation_id"],
						"is_typing":       typing["is_typing"],
					},
				}
				c.hub.BroadcastMessage(response)
			}
		}
	}
}

func (c *Client) WritePump() {
	defer func() {
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		}
	}
} 