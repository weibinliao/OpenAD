package main

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/weibinliao/OpenAD/internal/scanservice"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var scanProgressUpgrader = websocket.Upgrader{
	CheckOrigin: func(_ *http.Request) bool {
		return true
	},
}

type scanProgressHub struct {
	mutex   sync.RWMutex
	clients map[string]map[*scanProgressClient]struct{}
}

type scanProgressClient struct {
	connection *websocket.Conn
	mutex      sync.Mutex
}

func newScanProgressHub() *scanProgressHub {
	return &scanProgressHub{
		clients: make(map[string]map[*scanProgressClient]struct{}),
	}
}

func (application *application) handleScanProgressWebSocket(context *gin.Context) {
	scanID := strings.TrimSpace(context.Query("scan_id"))
	if scanID == "" {
		context.JSON(http.StatusBadRequest, gin.H{"error": "scan_id is required"})
		return
	}

	connection, err := scanProgressUpgrader.Upgrade(context.Writer, context.Request, nil)
	if err != nil {
		log.Printf("failed to upgrade scan progress websocket: %v", err)
		return
	}

	client := application.progressHub.subscribe(scanID, connection)

	if err := client.write(scanservice.ProgressEvent{
		ScanID: scanID,
		Status: "connected",
	}); err != nil {
		application.progressHub.unsubscribe(scanID, client)
		return
	}

	for {
		if _, _, err := connection.ReadMessage(); err != nil {
			application.progressHub.unsubscribe(scanID, client)
			return
		}
	}
}

func (application *application) buildScanProgressCallback(scanID string) scanservice.ProgressCallback {
	if scanID == "" {
		return nil
	}

	return func(event scanservice.ProgressEvent) {
		application.progressHub.publish(scanID, event)
	}
}

func (hub *scanProgressHub) subscribe(scanID string, connection *websocket.Conn) *scanProgressClient {
	client := &scanProgressClient{
		connection: connection,
	}

	hub.mutex.Lock()
	defer hub.mutex.Unlock()

	if _, ok := hub.clients[scanID]; !ok {
		hub.clients[scanID] = make(map[*scanProgressClient]struct{})
	}

	hub.clients[scanID][client] = struct{}{}

	return client
}

func (hub *scanProgressHub) unsubscribe(scanID string, client *scanProgressClient) {
	hub.mutex.Lock()
	defer hub.mutex.Unlock()

	clients, ok := hub.clients[scanID]
	if !ok {
		return
	}

	if _, ok := clients[client]; !ok {
		return
	}

	delete(clients, client)
	client.connection.Close()

	if len(clients) == 0 {
		delete(hub.clients, scanID)
	}
}

func (hub *scanProgressHub) publish(scanID string, event scanservice.ProgressEvent) {
	hub.mutex.RLock()
	clients := hub.clients[scanID]
	listeners := make([]*scanProgressClient, 0, len(clients))
	for client := range clients {
		listeners = append(listeners, client)
	}
	hub.mutex.RUnlock()

	if event.ScanID == "" {
		event.ScanID = scanID
	}

	for _, client := range listeners {
		if err := client.write(event); err != nil {
			hub.unsubscribe(scanID, client)
		}
	}
}

func (client *scanProgressClient) write(event scanservice.ProgressEvent) error {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	if err := client.connection.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}

	return client.connection.WriteJSON(event)
}
