package websocket

import (
	"github.com/gorilla/websocket"
)

func NewClient(hub *Hub, conn *websocket.Conn, userID int64, username string) *Client {
	return &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, 256),
		userID:   userID,
		username: username,
	}
} 