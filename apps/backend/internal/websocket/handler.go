package websocket

import (
	"log"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: CheckOrigin,
}

type ProgressEvent struct {
	Type            string `json:"type"`
	SessionID       string `json:"session_id"`
	ItemsScanned    int    `json:"items_scanned"`
	PermissionCount int    `json:"permission_count"`
	CurrentPath     string `json:"current_path"`
	Status          string `json:"status"`
	Message         string `json:"message,omitempty"`
}

func HandleConnection(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	sessionID := c.Query("session_id")
	if sessionID == "" {
		conn.WriteJSON(map[string]string{"error": "session_id required"})
		return
	}

	// Register connection for progress updates
	registerConnection(sessionID, conn)
	defer unregisterConnection(sessionID, conn)

	// Keep connection alive
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

var (
	connections = make(map[string][]*websocket.Conn)
	connMutex   sync.RWMutex
)

func registerConnection(sessionID string, conn *websocket.Conn) {
	connMutex.Lock()
	defer connMutex.Unlock()
	connections[sessionID] = append(connections[sessionID], conn)
}

func unregisterConnection(sessionID string, conn *websocket.Conn) {
	connMutex.Lock()
	defer connMutex.Unlock()
	conns := connections[sessionID]
	for i, c := range conns {
		if c == conn {
			connections[sessionID] = append(conns[:i], conns[i+1:]...)
			break
		}
	}
}

func BroadcastProgress(event ProgressEvent) {
	connMutex.RLock()
	conns := make([]*websocket.Conn, len(connections[event.SessionID]))
	copy(conns, connections[event.SessionID])
	connMutex.RUnlock()

	for _, conn := range conns {
		if err := conn.WriteJSON(event); err != nil {
			log.Printf("Failed to send progress: %v", err)
		}
	}
}
